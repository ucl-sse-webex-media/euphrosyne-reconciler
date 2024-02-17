import argparse
import functools
import json
import logging
from enum import Enum

import redis
from tenacity import retry, stop_after_attempt, wait_exponential

from sdk.errors import IncidentParsingError
from sdk.incident import Incident
from sdk.services import DataAggregator

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

    REDIS_ADDRESS = "localhost:6379"

    def __init__(self, name, handler):
        self._name = name
        self._handler = handler
        self._redis_client = None

        self.aggregator = None
        self.results = RecipeResults(name=self._name)

    @property
    def name(self):
        return self._name

    @staticmethod
    def _parse_input_data(func):
        """A decorator for parsing command-line arguments."""

        @functools.wraps(func)
        def wrapper(self, *args, **kwargs):
            parser = argparse.ArgumentParser(
                description="A Euphrosyne Reconciler recipe."
            )
            parser.add_argument("--data", type=str, help="Recipe input data")
            parser.add_argument("--aggregator-address", type=str, help="Data Aggregator address")
            parser.add_argument("--redis-address", type=str, help="Redis address")
            parsed_args = parser.parse_args()

            if parsed_args.data:
                try:
                    data = json.loads(parsed_args.data)
                    cli_config = {
                        "aggregator_address": parsed_args.aggregator_address,
                        "redis_address": parsed_args.redis_address,
                    }
                    return func(self, Incident.from_dict(data), cli_config, *args, **kwargs)
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

    def _parse_redis_address(self, redis_address=None):
        redis_address = redis_address or self.REDIS_ADDRESS
        split_address = redis_address.split(":")
        return {"host": split_address[0], "port": split_address[1]}

    def _connect_to_redis(self, redis_address=None):
        """Connect to Redis."""
        redis_address = self._parse_redis_address(redis_address)
        try:
            self._redis_client = redis.Redis(redis_address["host"], redis_address["port"])
            self._redis_client.ping()
        except redis.ConnectionError:
            logger.error(
                "Failed to connect to redis at %s:%s", redis_address["host"], redis_address["port"]
            )
            self.results.status = RecipeStatus.FAILED
            raise

    @retry(
        wait=wait_exponential(multiplier=2, min=1, max=10),
        stop=stop_after_attempt(3),
        reraise=True,
    )
    def _publish_results(self, channel: str):
        """Publish recipe results to Redis."""
        try:
            self._redis_client.publish(channel, str(self.results))
        except redis.exceptions.ConnectionError:
            logger.error("Could not connect to Redis. Please ensure that the service is running.")
            self.results.status = RecipeStatus.FAILED
            raise

    @_parse_input_data
    def run(self, incident: Incident, cli_config: dict):
        """Run the recipe."""
        self._connect_to_redis(cli_config["redis_address"])
        self.aggregator = DataAggregator(cli_config["aggregator_address"])
        self.results.incident = incident.uuid
        try:
            self._handler(incident, self)
        except Exception as e:
            logger.error("An error occurred while running the recipe: %s", e)
            self.results.status = RecipeStatus.FAILED
        self._publish_results(self._get_redis_channel(incident))
