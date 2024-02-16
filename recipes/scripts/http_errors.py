import logging

from sdk.errors import DataAggregatorHTTPError
from sdk.incident import Incident
from sdk.recipe import Recipe, RecipeResults, RecipeStatus

logger = logging.getLogger(__name__)


def handler(incident: Incident, results: RecipeResults, recipe: Recipe):
    """HTTP Errors Recipe."""
    logger.info("Received input:", incident)

    aggregator = recipe.aggregator

    try:
        aggregator.get_grafana_dashboard_from_incident(incident)
    except DataAggregatorHTTPError as e:
        results.log(str(e))
        results.status = RecipeStatus.FAILED
        return results

    results.status = RecipeStatus.SUCCESSFUL
    return results


def main():
    Recipe("http-errors", handler).run()


if __name__ == "__main__":
    main()
