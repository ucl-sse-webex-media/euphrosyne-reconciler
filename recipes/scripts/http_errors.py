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
        grafana_info = aggregator.get_grafana_info_from_incident(incident)   
    except DataAggregatorHTTPError as e:
        results.log(str(e))
        results.status = RecipeStatus.FAILED

    firing_time = aggregator.get_firing_time(incident)
    alert_rule = grafana_info["alertRule"]
    
    try:
        start_time = aggregator.calculate_query_start_time(alert_rule,firing_time)
    except DataAggregatorHTTPError as e:
        results.log(str(e))
        results.status = RecipeStatus.FAILED
        
    query = {
        "measurement" : "HTTPlogs",
        "start_time" : start_time,
        "stop_time" : firing_time
    }
    
    influxdb_records = aggregator.get_influxdb_records(incident,query)
    #continue analysising the influxdb_records here
    
    results.status = RecipeStatus.SUCCESSFUL


def main():
    Recipe("http-errors", handler).run()


if __name__ == "__main__":
    main()
