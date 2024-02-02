import logging

from sdk.recipe import Recipe, RecipeResults, RecipeStatus

logger = logging.getLogger(__name__)


def handler(data: dict, results: RecipeResults):
    """Dummy Recipe."""
    logger.info("Received input:", data)
    results.log("Tough luck mate!")
    results.status = RecipeStatus.SUCCESSFUL
    return results


def main():
    Recipe("dummy", handler).run()


if __name__ == "__main__":
    main()
