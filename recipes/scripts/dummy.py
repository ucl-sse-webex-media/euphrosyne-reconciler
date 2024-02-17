import logging

from sdk.incident import Incident
from sdk.recipe import Recipe, RecipeStatus

logger = logging.getLogger(__name__)


def handler(incident: Incident, recipe: Recipe):
    """Dummy Recipe."""
    logger.info("Received input:", incident)
    recipe.results.log("Tough luck mate!")
    recipe.results.status = RecipeStatus.SUCCESSFUL


def main():
    Recipe("dummy", handler).run()


if __name__ == "__main__":
    main()
