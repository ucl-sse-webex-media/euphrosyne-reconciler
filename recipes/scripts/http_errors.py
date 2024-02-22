import logging

from sdk.errors import DataAggregatorHTTPError
from sdk.incident import Incident
from sdk.recipe import Recipe, RecipeStatus

logger = logging.getLogger(__name__)


def handler(incident: Incident, recipe: Recipe):
    """HTTP Errors Recipe."""
    logger.info("Received input:", incident)

    results, aggregator = recipe.results, recipe.aggregator

    # query for grafana
    try:
        grafana_info = aggregator.get_grafana_info_from_incident(incident)
    except DataAggregatorHTTPError as e:
        results.log(str(e))
        results.status = RecipeStatus.FAILED
        return

    # query for influxdb
    firing_time = aggregator.get_firing_time(incident)
    start_time = aggregator.calculate_query_start_time(grafana_info, firing_time)

    influxdb_query = {
        "bucket": aggregator.get_influxdb_bucket(grafana_info),
        "measurement": aggregator.get_influxdb_measurement(grafana_info),
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
    # find the error with largest count
    main_error = sorted_records[0]

    # query for opensearch
    # index_pattern_url = aggregator.get_opensearch_index_pattern_url(grafana_info)
    webex_tracking_id = main_error["webextrackingID"]
    opensearch_query = {
        "WEBEX_TRACKINGID": [webex_tracking_id],
        "index_pattern": "afc30730-c54a-11ee-97eb-f78aebb9cc37",
    }
    try:
        opensearch_records = aggregator.get_opensearch_records(incident, opensearch_query)
    except DataAggregatorHTTPError as e:
        results.log(str(e))
        results.status = RecipeStatus.FAILED
        return

    opensearch_record = opensearch_records[webex_tracking_id][0]
    openseach_field = opensearch_record["fields"]

    # find the largest percentage of cluster
    cluster_count = {}
    for item in influxdb_records:
        if item["cluster"] in cluster_count:
            cluster_count[item["cluster"]] += item["_value"]
        else:
            cluster_count[item["cluster"]] = item["_value"]

    percentages = {cluster: (count / error_num) * 100 for cluster, count in cluster_count.items()}
    max_percentage_cluster = max(percentages, key=percentages.get)
    max_percentage_count = cluster_count[max_percentage_cluster]

    # format analysis
    analysis = (
        f"From {start_time} to {firing_time}, there were {error_num} pieces of"
        f" {main_error['_field']} http errors.\n"
    )
    analysis += (
        f"{max_percentage_count} alerts happens in cluster:"
        f" {max_percentage_cluster}.\n"
    )

    analysis += "Info: \n"
    # can be made as a method later
    analysis += (
        f"uri: {main_error['uri']}\nservicename: {main_error['servicename']}\nenvironment:"
        f" {main_error['environment']}\n"
    )

    analysis += f"message: {opensearch_record['message']}\n"

    if openseach_field.get("stack_trace") is not None:
        stack_trace = openseach_field["stack_trace"].split("\n")

        # filter all logs that contain com.cisco.wx2
        filtered_logs = "\n".join([entry for entry in stack_trace if "com.cisco.wx2" in entry])
        analysis += f"logs: {filtered_logs}\n"

    print(analysis)

    results.analysis = analysis
    results.status = RecipeStatus.SUCCESSFUL


def main():
    Recipe("http-errors", handler).run()


if __name__ == "__main__":
    main()
