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
        return

    firing_time = aggregator.get_firing_time(incident)
    alert_rule = grafana_info["alertRule"]

    start_time = aggregator.calculate_query_start_time(alert_rule, firing_time)

    influxdb_query = {
        "measurement": "HTTPlogs",
        "start_time": start_time,
        "stop_time": firing_time,
    }

    try:
        influxdb_records = aggregator.get_influxdb_records(incident, influxdb_query)
    except DataAggregatorHTTPError as e:
        results.log(str(e))
        results.status = RecipeStatus.FAILED
        return

    # _field for error type
    # _value for count
    error_num = sum(item["_value"] for item in influxdb_records)

    sorted_records = sorted(influxdb_records, key=lambda x: x["_value"], reverse=True)
    main_error = sorted_records[0]
    analysis = (
        f"From {start_time} to {firing_time}, there were {error_num} pieces of"
        f" {main_error['_field']} http errors.\n"
    )
    analysis += (
        f"{(main_error['_value'] / error_num) * 100} percentage of alerts happens in cluster:"
        f" {main_error['cluster']}.\n"
    )

    analysis += "Info: \n"
    # can be made as a method later
    analysis += (
        f"uri: {main_error['uri']}\nservicename: {main_error['servicename']}\nenvironment:"
        f" {main_error['environment']}\n"
    )

    webex_tracking_id = main_error["webextrackingID"]
    opensearch_query = {"WEBEX_TRACKINGID": [webex_tracking_id]}
    try:
        opensearch_records = aggregator.get_opensearch_records(incident, opensearch_query)
    except DataAggregatorHTTPError as e:
        results.log(str(e))
        results.status = RecipeStatus.FAILED
        return

    opensearch_record = opensearch_records[webex_tracking_id][0]
    openseach_field = opensearch_record["fields"]
    if openseach_field.get("stack_trace") is not None:
        stack_trace = openseach_field["stack_trace"].split("\n")

        # filter all logs that contain com.cisco.wx2
        filtered_logs = "\n".join([entry for entry in stack_trace if "com.cisco.wx2" in entry])

        analysis += f"message: {opensearch_record['message']}\nlog: {filtered_logs}\n"
    print(analysis)
    results.analysis = analysis
    results.status = RecipeStatus.SUCCESSFUL


def main():
    Recipe("http-errors", handler).run()


if __name__ == "__main__":
    main()
