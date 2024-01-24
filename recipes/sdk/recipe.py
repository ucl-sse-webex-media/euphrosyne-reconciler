import functools
import json
import logging
from .util import parse_args
import redis
from tenacity import retry, stop_after_attempt, wait_exponential

logger = logging.getLogger(__name__)


class Recipe():
    """Euphrosyne Reconciler Recipe."""

    def __init__(self, name, handler):
        self.name = name
        self.handler = handler
        self.redisClient = redis.Redis(host="euphrosyne-reconciler-redis", port=80)

    def parse_input_data(func):
        """A decorator for parsing command-line arguments."""
        @functools.wraps(func)
        def wrapper(self, *args, **kwargs):
            parsed_args = parse_args()
            if parsed_args.data:
                try:
                    data = json.loads(parsed_args.data)
                except json.JSONDecodeError:
                    logger.error("Invalid input provided. Please provide valid JSON input.")
                return func(self, data, *args, **kwargs)
            else:
                logger.error("No input provided. Please provide input using the --data option.")

        return wrapper

    def _get_redis_channel(self, data):
        """Get a Redis channel name to publish the recipe results."""
        return data.get("uuid")

    @retry(
        wait=wait_exponential(multiplier=2, min=1, max=10),
        stop=stop_after_attempt(3),
        reraise=True,
    )
    def publish_results(self, channel, results):
        """Publish recipe results to Redis."""
        try:
            self.redisClient.publish(channel, json.dumps(results))
        except redis.exceptions.ConnectionError:
            logger.error("Could not connect to Redis. Please ensure that the service is running.")

    @parse_input_data
    def run(self, data):
        """Run the recipe."""
        results = self.handler(data)
        self.publish_results(self._get_redis_channel(data), results)
