import logging

from sdk.recipe import Recipe

logger = logging.getLogger(__name__)


def handler(data):
    """Dummy Recipe."""

    logger.info("Received input:", data)
    return {"status": "success", "results": "Dummy results"}


def main():
    Recipe("dummy", handler).run()


if __name__ == "__main__":
    main()
