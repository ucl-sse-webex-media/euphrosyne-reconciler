from collections import Counter
import logging
import re

from sdk.errors import DataAggregatorHTTPError, ApiResError
from sdk.incident import Incident
from sdk.recipe import Recipe, RecipeStatus

logger = logging.getLogger(__name__)


def handler(incident: Incident, recipe: Recipe):
    """HTTP Errors Recipe."""
    logger.info("Received input:", incident)

    results, aggregator = recipe.results, recipe.aggregator

    # query for grafana
    try:
        grafana_result = aggregator.get_grafana_info_from_incident(incident)
    except (DataAggregatorHTTPError, ApiResError) as e:
        results.log(str(e))
        results.status = RecipeStatus.FAILED
        raise

    # query for influxdb
    firing_time = aggregator.get_firing_time(incident)
    start_time = aggregator.calculate_query_start_time(grafana_result, firing_time)

    influxdb_query = {
        "bucket": aggregator.get_influxdb_bucket(grafana_result),
        "measurement": aggregator.get_influxdb_measurement(grafana_result),
        "startTime": start_time,
        "stopTime": firing_time,
    }

    try:
        influxdb_records = aggregator.get_influxdb_records(incident, influxdb_query)
    except (DataAggregatorHTTPError, ApiResError) as e:
        results.log(str(e))
        results.status = RecipeStatus.FAILED
        raise

    # _field for error type
    # _value for count
    count_key = "_value"
    error_num = sum(item[count_key] for item in influxdb_records)

    # count how many different error code like 500,501
    error_code_count = aggregator.count_metric(influxdb_records, "_field", count_key)

    # find the largest percentage of region(environment)
    region_count = aggregator.count_metric(influxdb_records, "environment", count_key)
    max_region_name = max(region_count, key=region_count.get)
    max_region_count = region_count[max_region_name]

    uri_count = {}
    confluence_uri_count = 0
    # {uri:{method:count}}
    # find is there any confluence uri with method post
    for item in influxdb_records:
        uri = item["uri"]
        method = item["method"]
        count = item[count_key]
        if method == "POST" and re.search(r"/calliope/api/v2/venues/.+?/confluences", uri):
            uri = "calliope/api/v2/venues/param/confluences"
            confluence_uri_count += count
        if uri not in uri_count:
            uri_count[uri] = {}
        if method not in uri_count[uri]:
            uri_count[uri][method] = 0
        uri_count[uri][method] += count

    # find the lagest percent of uri
    max_uri_count = 0
    max_uri = ""
    max_method = ""
    for uri, methods in uri_count.items():
        for method, count in methods.items():
            if count > max_uri_count:
                max_uri_count = count
                max_uri = uri
                max_method = method

    # query for opensearch
    influxdb_webextrackingid_name = "webextrackingID"
    webex_tracking_id_list = list(
        {record[influxdb_webextrackingid_name] for record in influxdb_records}
    )
    
    index_pattern_url = aggregator.get_opensearch_index_pattern_url(grafana_result)
    if index_pattern_url == "":
        index_pattern_url = "541ca530-d1c5-11ee-b437-abf99369aba1"
    opensearch_query = {
        "webextrackingID": webex_tracking_id_list,
        "index_pattern": index_pattern_url,
    }
    try:
        opensearch_records = aggregator.get_opensearch_records(incident, opensearch_query)
    except (DataAggregatorHTTPError, ApiResError) as e:
        results.log(str(e))
        results.status = RecipeStatus.FAILED
        raise

    user_id_list = []
    opensearch_records_len = 0
    for _, record_list in opensearch_records.items():
        opensearch_records_len += len(record_list)
        for item in record_list:
            user_id_list.append(item["fields"].get("USER_ID", ""))

    # find the largest percent of user_id
    user_id_count = Counter(user_id_list)
    max_userid = max(user_id_count, key=user_id_count.get)
    max_userid_count = user_id_count[max_userid]

    # find the example error
    if confluence_uri_count > 0:
        for item in influxdb_records:
            if (
                re.search(r"/calliope/api/v2/venues/.+?/confluences", item["uri"])
                and method == "POST"
            ):
                example_influxdb = item
                break
    else:
        # find an example that has max mertric
        max_dict = {
            "environment": max_region_count,
            "uri": max_uri_count,
            "USER_ID": max_userid_count,
        }
        max_metric = max(max_dict, key=max_dict.get)
        if max_metric == "environment":
            for item in influxdb_records:
                if item["environment"] == max_region_name:
                    example_influxdb = item
                    break
        elif max_metric == "uri":
            for item in influxdb_records:
                if item["uri"] == max_uri:
                    example_influxdb = item
                    break
        elif max_metric == "USER_ID":
            for _, record_list in opensearch_records.items():
                for record in record_list:
                    if item["fields"]["USER_ID"] == max_userid:
                        example_opensearch = record
                        break
    if example_influxdb is not None:
        example_opensearch = opensearch_records[example_influxdb[influxdb_webextrackingid_name]][0]

    # format analysis
    analysis = f"From {start_time} To {firing_time}, there are:\n"

    for error_code, count in error_code_count.items():
        analysis += f"{count} pieces of http {error_code} errors \n"

    if len(error_code_count) > 1:
        analysis += f"Total of {sum(error_code_count.values())} errors\n"

    analysis += f"Largest percent of region is '{max_region_name}', occur in {max_region_count} ({max_region_count/(error_num)*100}%) of errors \n"

    analysis += f"Largest percent of uri is '{max_uri}' with method '{max_method}', occur in {max_uri_count} ({max_uri_count/(error_num)*100}%) of errors \n"

    if max_uri != "calliope/api/v2/venues/param/confluences" and confluence_uri_count > 0:
        analysis += f"IMPORTANT! {confluence_uri_count} ({confluence_uri_count/(error_num)*100} %) errors occur in uri 'calliope/api/v2/venues/param/confluences' with method 'POST' \n"

    analysis += f"{len(user_id_count)} unique USER ID in opensearch logs\n"
    analysis += f"Largest percent of USER ID is '{max_userid}', occur in {max_userid_count} ({max_userid_count/opensearch_records_len*100}%) of opensearch logs"

    analysis += "\n"
    analysis += "Example Error Info: \n"
    example_openseach_field = example_opensearch["fields"]
    analysis += f"WEBEXTRACKING_ID:{example_openseach_field['webextrackingID']}\nregion:{example_opensearch['environment']}\noperation: {example_openseach_field['operation_key']}\nUSER_ID:{example_openseach_field['USER_ID']}\n"

    analysis += f"message: {example_opensearch['message']}\n"
    if example_openseach_field.get("stack_trace") is not None:
        stack_trace = example_openseach_field["stack_trace"].split("\n")
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
