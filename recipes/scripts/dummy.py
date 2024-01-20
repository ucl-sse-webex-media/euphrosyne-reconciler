import json
import logging

from sdk.recipe import Recipe
from sdk.services import DataAggregator

logger = logging.getLogger(__name__)


def handler(data):
    """Dummy Recipe."""

    logger.info("Received input:", data)
    logger.info("Calling the Aggregator...")
    aggregator = DataAggregator()
    uuid = data["uuid"]
    dashboard_id = data["alerts"][0]["dashboardURL"].rsplit("/", 1)[-1].split("?")[0]
    panel_id = data["alerts"][0]["panelURL"].rsplit("=", 1)[-1]
    results = aggregator.get_grafana_dashboard(uuid, dashboard_id, panel_id)
    return {"status": "success", "results": json.dumps(results)}


def main():
    Recipe("dummy", handler).run()


if __name__ == "__main__":
    main()
