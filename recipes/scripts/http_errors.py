import logging
import re
from collections import Counter

from sdk.errors import ApiResError, DataAggregatorHTTPError
from sdk.incident import Incident
from sdk.recipe import Recipe, RecipeStatus

from datetime import datetime
from sklearn.linear_model import LinearRegression
import numpy as np

logger = logging.getLogger(__name__)

count_key = "_value"

def get_influxdb_start_and_end_time(influxdb_records):
    """get the start and end time of influxdb records"""
    start_time = influxdb_records[0]["_time"]
    end_time = influxdb_records[-1]["_time"]
    return start_time, end_time


def count_metric_by_key(record_list, metric, count_key):
    """count the number of metric by a key in list"""
    count = {}
    for item in record_list:
        if item[metric] in count:
            count[item[metric]] += item[count_key]
        else:
            count[item[metric]] = item[count_key]
    return count


def analysis_max_region(influxdb_records):
    # find the largest percentage of region(environment)
    region_count = count_metric_by_key(influxdb_records, "environment", count_key)
    max_region_name = max(region_count, key=region_count.get)
    max_region_count = region_count[max_region_name]
    return max_region_name, max_region_count


def analysis_max_url(influxdb_records):
    uri_count = {}
    confluence_uri_count = 0
    # {uri:{method:count}}
    # find is there any confluence uri with method post
    for item in influxdb_records:
        uri = item["uri"]
        method = item["method"]
        count = item[count_key]
        if method == "POST" and re.search(r"/calliope/api/v2/venues/.+?/confluences", uri):
            uri = "calliope/api/v2/venues/{venueId}/confluences"
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

    return max_uri, max_method, max_uri_count, confluence_uri_count


def analysis_max_user_id(opensearch_records):
    user_id_list = []
    for _, record_list in opensearch_records.items():
        for record in record_list:
            user_id_list.append(record["fields"].get("USER_ID", ""))

    # find the largest percent of user_id
    user_id_count = Counter(user_id_list)
    max_userid = max(user_id_count, key=user_id_count.get)
    max_userid_count = user_id_count[max_userid]
    return max_userid, max_userid_count, user_id_count


def analysis_confluence_log(opensearch_records):
    confluence_log_list = []
    for _, record_list in opensearch_records:
        for record in record_list:
            operation_key = record["fields"]["operation_key"]
            method = operation_key.split(" ")[0]
            uri = operation_key.split(" ")[1]
            if method == "POST" and re.search(r"/calliope/api/v2/venues/.+?/confluences", uri):
                message = record.get("message", "")
                if "Confluence Create Broker Summary" in message:
                    confluence_log_list.append(message)

    agent_full_name_list = []
    agent_org_group_list = []
    for log in confluence_log_list:
        summarys = re.split(r"\n\t(?!\t)", log)[1:]
        for item in summarys:
            status = re.split(r"\n\t\t\t(?!\t)", item)[1]
            if re.match(r"^- error", status):
                agent_name = re.split(r"\n\t\t(?!\t)", item)[0]
                left_bracket_index = agent_name.find("(")
                agent_full_name = agent_name[:left_bracket_index]
                agent_full_name_list.append(agent_full_name)

                first_dot = agent_name.find(".")
                second_dot = agent_name.find(".", first_dot + 1)
                agent_org_group = agent_name[:second_dot]
                agent_org_group_list.append(agent_org_group)

    agent_full_name_count = Counter(agent_full_name_list)
    sorted_agent_full_name_count = sorted(
        agent_full_name_count.items(), key=lambda x: x[1], reverse=True
    )
    agent_org_group_coount = Counter(agent_org_group_list)
    sorted_agent_org_group_count = sorted(
        agent_org_group_coount.items(), key=lambda x: x[1], reverse=True
    )
    return sorted_agent_full_name_count, sorted_agent_org_group_count

def analysis_trend(influxdb_records, max_region):
    def adjust_timestamp_for_parsing(timestamp_str):
        # Truncate or round the timestamp to microsecond precision
        return timestamp_str[:26] + 'Z'  # Truncate to microsecond precision for datetime parsing
    
    # Filter records for the specified max_region
    max_region_records = [record for record in influxdb_records if record['environment'] == max_region]
    
    # Ensure there are records to analyse after filtering
    if not max_region_records:
        return "No records to analyze for the specified region."
    
    print(max_region_records)
    print(len(max_region_records))
    
    first_time, last_time = get_influxdb_start_and_end_time(max_region_records)
    
    # Convert _time to datetime objects and sort the filtered records
    sorted_records = sorted(
        max_region_records, 
        key=lambda x: datetime.strptime(adjust_timestamp_for_parsing(x['_time']), "%Y-%m-%dT%H:%M:%S.%fZ")
    )

    # Prepare data for regression model
    start_time = datetime.strptime(adjust_timestamp_for_parsing(sorted_records[0]['_time']), "%Y-%m-%dT%H:%M:%S.%fZ")
    X = [(datetime.strptime(adjust_timestamp_for_parsing(record['_time']), "%Y-%m-%dT%H:%M:%S.%fZ") - start_time).total_seconds() for record in sorted_records]
    X = np.array(X).reshape(-1, 1)  # Reshape for sklearn
    y = np.array([record['_value'] for record in sorted_records])

    # Fit linear regression model
    model = LinearRegression().fit(X, y)
    slope = model.coef_[0]

    # Find the peak and the lowest values
    peak_value = max(y)
    lowest_value = min(y)
    peak_index = np.argmax(y)
    lowest_index = np.argmin(y)

    print(y)

    # Calculate time to peak and lowest from the start
    time_to_peak_seconds = int(X[peak_index][0])
    time_to_lowest_seconds = int(X[lowest_index][0])

    num_data_points = len(y)

    print(num_data_points)

    # Interpret the slope for trend analysis
    if slope > 0:
        trend = "increasing"
    elif slope < 0:
        trend = "decreasing"
    else:
        trend = "stable"

    return f"The trend is {trend}.", peak_value, lowest_value, peak_index, lowest_index, time_to_peak_seconds, time_to_lowest_seconds, num_data_points, first_time, last_time

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

    # get the start and end time of influxdb records
    first_time, last_time = get_influxdb_start_and_end_time(influxdb_records)

    # _field for error type
    error_num = sum(item[count_key] for item in influxdb_records)

    # count how many different error code like 500,501
    error_code_count = count_metric_by_key(influxdb_records, "_field", count_key)

    max_region_name, max_region_count = analysis_max_region(influxdb_records)

    max_uri, max_method, max_uri_count, confluence_uri_count = analysis_max_url(influxdb_records)

    trend, peak_value, lowest_value, peak_index, lowest_index, time_to_peak_seconds, time_to_lowest_seconds, num_data_points, max_region_first_time, max_region_last_time = analysis_trend(influxdb_records, max_region_name)

    # query for opensearch
    influxdb_trackingid_name = "WEBEX_TRACKINGID"
    webex_tracking_id_list = list(
        {record[influxdb_trackingid_name] for record in influxdb_records}
    )

    index_pattern_url = aggregator.get_opensearch_index_pattern_url(grafana_result)
    opensearch_query = {
        "field": {"WEBEX_TRACKINGID": webex_tracking_id_list},
        "index_pattern": index_pattern_url,
    }
    try:
        opensearch_records = aggregator.get_opensearch_records(incident, opensearch_query)
    except (DataAggregatorHTTPError, ApiResError) as e:
        results.log(str(e))
        results.status = RecipeStatus.FAILED
        raise

    opensearch_records_len = aggregator.get_total_opensearch_records_num(opensearch_records)

    max_userid, max_userid_count, user_id_count = analysis_max_user_id(opensearch_records)
    # find the example error
    # if the confluence uri is in errors, use error with the uri as the example
    if confluence_uri_count > 0:
        for item in influxdb_records:
            if (
                re.search(r"/calliope/api/v2/venues/.+?/confluences", item["uri"])
                and item["method"] == "POST"
            ):
                example_influxdb = item
                break
    else:
        # find an example that has max metric value
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
        else:
            for _, record_list in opensearch_records.items():
                example_opensearch = record_list[0]
                break

    if example_influxdb is not None:
        example_opensearch = opensearch_records[example_influxdb[influxdb_trackingid_name]][0]

    # format analysis
    analysis = f"From {first_time} To {last_time}, there are:\n"

    for error_code, count in error_code_count.items():
        analysis += f"{count} pieces of http {error_code} errors \n"

    if len(error_code_count) > 1:
        analysis += f"Total of {sum(error_code_count.values())} errors\n"

    analysis += f"Largest percent of region is '{max_region_name}', occur in {max_region_count} ({max_region_count/(error_num)*100}%) of errors \n"

    analysis += f"Largest percent of uri is '{max_uri}' with method '{max_method}', occur in {max_uri_count} ({max_uri_count/(error_num)*100}%) of errors \n"

    if max_uri != "calliope/api/v2/venues/{venueId}/confluences" and confluence_uri_count > 0:
        analysis += f"IMPORTANT! {confluence_uri_count}({confluence_uri_count/(error_num)*100} %) errors occur in uri 'calliope/api/v2/venues/{{venueId}}/confluences' with method 'POST' \n"

    analysis += f"{len(user_id_count)} unique USER ID in opensearch logs\n"
    analysis += f"Largest percent of USER ID is '{max_userid}', occur in {max_userid_count} ({max_userid_count/opensearch_records_len*100}%) of opensearch logs\n"

    if num_data_points > 1:
        analysis += f"Trend for 5xx error in max percent region: {trend} \n"

        if peak_index == 0: 
            analysis += f"Peak value is {peak_value} times from the first data point in max region {max_region_first_time} \n"
        else:
            analysis += f"Peak value: {peak_value} times at {time_to_peak_seconds} seconds from the first data point in max region {max_region_first_time} \n"
       
        if lowest_index == 0:
            analysis += f"Lowest value is {lowest_value} times from the first data point in max region {max_region_first_time} \n"
        else:
            analysis += f"Lowest value: {lowest_value} times at {time_to_lowest_seconds} seconds from the first data point in max region {max_region_first_time} \n"

    analysis += "\n"
    analysis += "Example Error Info: \n"
    example_openseach_field = example_opensearch["fields"]
    analysis += f"WEBEXTRACKING_ID:{example_openseach_field['WEBEX_TRACKINGID']}\nregion:{example_opensearch['environment']}\noperation: {example_openseach_field['operation_key']}\nUSER_ID:{example_openseach_field['USER_ID']}\n"

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
