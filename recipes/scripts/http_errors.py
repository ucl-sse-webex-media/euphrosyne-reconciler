from collections import Counter
import logging

from sdk.errors import DataAggregatorHTTPError,ApiResError
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
        if grafana_info.get("error") is not None:
            raise Exception(grafana_info.get("error"))
    except (DataAggregatorHTTPError,ApiResError) as e:
        results.log(str(e))
        results.status = RecipeStatus.FAILED
        raise

    # query for influxdb
    firing_time = aggregator.get_firing_time(incident)
    start_time = aggregator.calculate_query_start_time(grafana_info, firing_time)

    influxdb_query = {
        "bucket": aggregator.get_influxdb_bucket(grafana_info),
        "measurement": aggregator.get_influxdb_measurement(grafana_info),
        "startTime": start_time,
        "stopTime": firing_time,
    }

    try:
        influxdb_records = aggregator.get_influxdb_records(incident, influxdb_query)
    except (DataAggregatorHTTPError,ApiResError) as e:
        results.log(str(e))
        results.status = RecipeStatus.FAILED
        raise
    
    # _field for error type
    # _value for count
    count_key = "_value"
    error_num = sum(item[count_key] for item in influxdb_records)
    
    # count how many different error code like 500,501
    error_code_count = aggregator.count_influxdb_metric(influxdb_records,"_field",count_key)     
    
    # find the largest percentage of region(environment)
    region_count = aggregator.count_influxdb_metric(influxdb_records,"environment",count_key)    
    max_region_name = max(region_count, key=region_count.get)
    max_region_count = region_count[max_region_name]
    
    # find the lagest percent of uri
    uri_count = aggregator.count_influxdb_metric(influxdb_records,"uri",count_key)    
    max_uri_name = max(uri_count, key=uri_count.get)
    max_uri_count = region_count[max_uri_name]
    
    # find is there any confluence uri
    
    
    # find the error with largest count
    sorted_records = sorted(influxdb_records, key=lambda x: x[count_key], reverse=True)
    example_error = sorted_records[0]
    
    # query for opensearch
    # index_pattern_url = aggregator.get_opensearch_index_pattern_url(grafana_info)
    webex_tracking_id = example_error["webextrackingID"]
    opensearch_query = {
        "WEBEX_TRACKINGID": [webex_tracking_id],
        "index_pattern": "541ca530-d1c5-11ee-b437-abf99369aba1",
    }
    try:
        opensearch_records = aggregator.get_opensearch_records(incident, opensearch_query)
    except(DataAggregatorHTTPError,ApiResError) as e:
        results.log(str(e))
        results.status = RecipeStatus.FAILED
        raise

    opensearch_record = opensearch_records[webex_tracking_id][0]
    openseach_field = opensearch_record["fields"]

    # format analysis
    analysis = (
        f"From {start_time} To {firing_time}, there are:"
    )
    
    for error_code,count in error_code_count.items():
        analysis += f"{count} pieces of http {error_code} errors \n"

    analysis += (
        f"{max_region_count} (f{max_region_count/len(error_num)*100} %) errors happens in region:" f" {max_region_name}.\n"
        f"{max_uri_count} (f{max_uri_count/len(error_num)*100} %) errors happens in region:" f" {max_uri_name}.\n"
    )

    analysis += "Example Error Info: \n"
    # can be made as a method later
    analysis += (
        f"uri: {example_error['uri']}\nservicename: {example_error['servicename']}\nenvironment:"
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
