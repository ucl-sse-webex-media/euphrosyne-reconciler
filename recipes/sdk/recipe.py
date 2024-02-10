import redis
from tenacity import retry, stop_after_attempt, wait_exponential
import functools
import json
import logging
from enum import Enum
from .config import REDIS_ADDRESS
from .util import parse_args
import redis
from tenacity import retry, stop_after_attempt, wait_exponential

from sdk.errors import IncidentParsingError
from sdk.incident import Incident

logger = logging.getLogger(__name__)


class RecipeStatus(Enum):
    """Euphrosyne Reconciler Recipe Status."""

    SUCCESSFUL = "successful"
    FAILED = "failed"
    UNKNOWN = "unknown"


class RecipeResults:
    """Euphrosyne Reconciler Recipe Results."""

    def __init__(
        self,
        incident: str = None,
        name: str = None,
        status: RecipeStatus = None,
        analysis: str = None,
        json: str = None,
        links: list[str] = None,
    ):
        self.incident = incident or ""
        self.name = name or ""
        self._status = status or RecipeStatus.UNKNOWN
        self.results = {
            "analysis": analysis or "",
            "json": json or "",
            "links": links or [],
        }

    @property
    def status(self):
        return self._status

    @status.setter
    def status(self, value: RecipeStatus):
        self._status = value

    @property
    def json(self):
        return self.results["json"]

    @json.setter
    def json(self, value: str):
        self.results["json"] = value

    @property
    def links(self):
        return self.results["links"]

    def add_link(self, link: str):
        """Add a link to the recipe results."""
        self.results["links"].append(link)

    @property
    def analysis(self):
        return self.results["analysis"]

    @analysis.setter
    def analysis(self, value: str):
        self.results["analysis"] = value

    def log(self, message):
        """Add a log to the recipe analysis."""
        self.analysis = f"{self.analysis} {message}" if self.analysis else message

    @classmethod
    def from_dict(cls, d):
        """Create a RecipeResults object from a dictionary."""
        status = RecipeStatus(d.get("status", RecipeStatus.UNKNOWN.value))
        params = {k: v for k, v in d.items() if k != "status"}
        return cls(status=status, **params)

    def to_dict(self):
        """Convert the recipe results to a dictionary."""
        return {
            "incident": self.incident,
            "name": self.name,
            "status": self.status.value,
            "results": self.results,
        }

    def __str__(self):
        """Convert the recipe results to a string."""
        return json.dumps(self.to_dict())


class Recipe:
    """Euphrosyne Reconciler Recipe."""

    def __init__(self, name, handler):
        self.parsed_args = parse_args()
        self.name = name
        self.handler = handler
        redis_address = self._parse_redis_address()
        self.results = RecipeResults(name=self.name)
        try:
            self.redisClient = redis.Redis(redis_address["host"], redis_address["port"])
            self.redisClient.ping()
        except redis.ConnectionError:
            logger.error("Failed to connect to redis at %s:%s",redis_address["host"], redis_address["port"])
                  
    def _parse_redis_address(self):
        redis_address = self.parsed_args.redis_address if self.parsed_args.redis_address else REDIS_ADDRESS
        split_address = redis_address.split(":")
        return {"host":split_address[0],"port":split_address[1]}
        
    def parse_input_data(func):
        """A decorator for parsing command-line arguments."""

        @functools.wraps(func)
        def wrapper(self, *args, **kwargs):
            if self.parsed_args.data:
                try:
                    data = json.loads(self.parsed_args.data)
                    return func(self, Incident.from_dict(data), *args, **kwargs)
                except json.JSONDecodeError:
                    raise IncidentParsingError(
                        "Invalid input provided. Please provide valid JSON input."
                    )
            else:
                raise IncidentParsingError(
                    "No input provided. Please provide input using the --data option."
                )

        return wrapper

    def _get_redis_channel(self, incident: Incident):
        """Get a Redis channel name to publish the recipe results."""
        return incident.uuid

    @retry(
        wait=wait_exponential(multiplier=2, min=1, max=10),
        stop=stop_after_attempt(3),
        reraise=True,
    )
    def publish_results(self, channel: str, results: RecipeResults):
        """Publish recipe results to Redis."""
        try:
            self.redisClient.publish(channel, str(results))
        except redis.exceptions.ConnectionError:
            logger.error("Could not connect to Redis. Please ensure that the service is running.")

    @parse_input_data
    def run(self, incident: Incident):
        """Run the recipe."""
        self.results.incident = incident.uuid
        results = self.handler(incident, self.results)
        self.publish_results(self._get_redis_channel(incident), results)
