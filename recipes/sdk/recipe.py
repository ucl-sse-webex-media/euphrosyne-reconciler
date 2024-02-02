import argparse
import functools
import json
import logging
from enum import Enum

import redis
from tenacity import retry, stop_after_attempt, wait_exponential

logger = logging.getLogger(__name__)


class RecipeStatus(Enum):
    """Euphrosyne Reconciler Recipe Status."""
    SUCCESSFUL = "successful"
    FAILED = "failed"
    UNKNOWN = "unknown"


class RecipeResults():
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

    @staticmethod
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


class Recipe():
    """Euphrosyne Reconciler Recipe."""

    def __init__(self, name, handler):
        self.name = name
        self.handler = handler
        self.redisClient = redis.Redis(host="euphrosyne-reconciler-redis", port=80)

        self.results = RecipeResults(name=self.name)

    @staticmethod
    def parse_input_data(func):
        """A decorator for parsing command-line arguments."""
        @functools.wraps(func)
        def wrapper(self, *args, **kwargs):
            parser = argparse.ArgumentParser(description="A Euphrosyne Reconciler recipe.")
            parser.add_argument("--data", type=str, help="Recipe input data")

            parsed_args = parser.parse_args()

            if parsed_args.data:
                try:
                    data = json.loads(parsed_args.data)
                except json.JSONDecodeError:
                    logger.error("Invalid input provided. Please provide valid JSON input.")
                return func(self, data, *args, **kwargs)
            else:
                logger.error("No input provided. Please provide input using the --data option.")

        return wrapper

    def _get_incident_uuid(self, data: dict):
        """Get the incident UUID from the input data."""
        return data.get("uuid")

    def _get_redis_channel(self, data: dict):
        """Get a Redis channel name to publish the recipe results."""
        return self._get_incident_uuid(data)

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
    def run(self, data: dict):
        """Run the recipe."""
        self.results.incident = self._get_incident_uuid(data)
        results = self.handler(data, self.results)
        self.publish_results(self._get_redis_channel(data), results)
