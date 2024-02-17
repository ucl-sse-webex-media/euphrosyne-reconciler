import logging

from sdk.errors import DataAggregatorHTTPError
from sdk.incident import Incident
from sdk.recipe import Recipe, RecipeStatus

logger = logging.getLogger(__name__)


def handler(incident: Incident, recipe: Recipe):
    """HTTP Errors Recipe."""
    logger.info("Received input:", incident)

    results, aggregator = recipe.results, recipe.aggregator

    try:
        aggregator.get_grafana_dashboard_from_incident(incident)
    except DataAggregatorHTTPError as e:
        results.log(str(e))
        results.status = RecipeStatus.FAILED

    results.status = RecipeStatus.SUCCESSFUL


def main():
    Recipe("http-errors", handler).run()


if __name__ == "__main__":
    main()
