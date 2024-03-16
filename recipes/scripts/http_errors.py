import logging
import re
from collections import Counter
from datetime import datetime

import numpy as np
from sklearn.linear_model import LinearRegression

from sdk.errors import ApiResError, DataAggregatorHTTPError
from sdk.incident import Incident
from sdk.recipe import Recipe, RecipeStatus

logger = logging.getLogger(__name__)

count_key = "_value"


def precise_timestamp_to_second(timestamp_str):
    return timestamp_str[:19] + "Z"


def get_influxdb_start_and_end_time(influxdb_records):
    """get the start and end time of accurate influxdb records"""
    start_time = influxdb_records[0]["_time"]
    end_time = influxdb_records[-1]["_time"]
    return precise_timestamp_to_second(start_time), precise_timestamp_to_second(end_time)


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
    """find the largest percentage of region(environment)"""
    region_count = count_metric_by_key(influxdb_records, "environment", count_key)
    max_region_name = max(region_count, key=region_count.get)
    max_region_count = region_count[max_region_name]
    return max_region_name, max_region_count


def analysis_max_url(influxdb_records):
    """analysis max percent url and method, and find the confluence uri count"""
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
    """analysis max percent user id in opensearch records"""
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
    """analysis confluence log"""
    confluence_log_list = []
    for _, record_list in opensearch_records.items():
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
                # agent full name in org.group.xxx format
                first_dot = agent_name.find(".")
                second_dot = agent_name.find(".", first_dot + 1)
                agent_org_group = agent_name[:second_dot]
                agent_org_group_list.append(agent_org_group)

    agent_full_name_count = Counter(agent_full_name_list)
    sorted_agent_full_name_count = sorted(
        agent_full_name_count.items(), key=lambda x: x[1], reverse=True
    )
    agent_org_group_count = Counter(agent_org_group_list)
    sorted_agent_org_group_count = sorted(
        agent_org_group_count.items(), key=lambda x: x[1], reverse=True
    )
    return sorted_agent_full_name_count, sorted_agent_org_group_count, len(agent_full_name_list)


def analysis_trend(influxdb_records, max_region):
    """analysis the trend of the error in the max region"""
    # Filter records for the specified max_region
    max_region_records = [
        record for record in influxdb_records if record["environment"] == max_region
    ]

    # Ensure there are records to analyse after filtering
    if not max_region_records:
        return "No records to analyze for the specified region."

    first_time, _ = get_influxdb_start_and_end_time(max_region_records)

    # Convert _time to datetime objects and sort the filtered records
    sorted_records = sorted(
        max_region_records,
        key=lambda x: datetime.strptime(
            precise_timestamp_to_second(x["_time"]), "%Y-%m-%dT%H:%M:%SZ"
        ),
    )

    # Prepare data for regression model
    start_time = datetime.strptime(
        precise_timestamp_to_second(sorted_records[0]["_time"]), "%Y-%m-%dT%H:%M:%SZ"
    )
    X = [
        (
            datetime.strptime(precise_timestamp_to_second(record["_time"]), "%Y-%m-%dT%H:%M:%SZ")
            - start_time
        ).total_seconds()
        for record in sorted_records
    ]
    X = np.array(X).reshape(-1, 1)  # Reshape for sklearn
    y = np.array([record["_value"] for record in sorted_records])

    # Fit linear regression model
    model = LinearRegression().fit(X, y)
    slope = model.coef_[0]

    # Find the peak and the lowest values
    peak_value = max(y)
    peak_index = np.argmax(y)

    # Calculate time to peak and lowest from the start
    time_to_peak_seconds = int(X[peak_index][0])

    num_data_points = len(y)

    # Interpret the slope for trend analysis
    if slope > 0:
        trend = "increasing"
    elif slope < 0:
        trend = "decreasing"
    else:
        trend = "stable"

    return (
        trend,
        peak_value,
        peak_index,
        time_to_peak_seconds,
        num_data_points,
        first_time,
    )


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

    tags = aggregator.get_influxdb_tags_from_grafana(grafana_result)
    tag_set = []

    for tag in tags:
        key = tag["key"]
        if "::tag" in key:
            key = key[: key.find("::tag")]
        tag_set.append({key: tag["value"]})
    influxdb_query = {
        "bucket": aggregator.get_influxdb_bucket_from_grafana(grafana_result),
        "measurement": aggregator.get_influxdb_measurement_from_grafana(grafana_result),
        "startTime": start_time,
        "stopTime": firing_time,
        "tagSets": tag_set,
    }
    try:
        influxdb_records = aggregator.get_influxdb_records(incident, influxdb_query)
    except (DataAggregatorHTTPError, ApiResError) as e:
        results.log(str(e))
        results.status = RecipeStatus.FAILED
        raise
    # get the start and end time of influxdb records
    first_error_time, last_error_time = get_influxdb_start_and_end_time(influxdb_records)

    # _field for error type
    error_num = sum(item[count_key] for item in influxdb_records)

    # count how many different error code like 500,501
    error_code_count = count_metric_by_key(influxdb_records, "httpStatusCode", count_key)

    max_region_name, max_region_count = analysis_max_region(influxdb_records)

    max_uri, max_method, max_uri_count, confluence_uri_count = analysis_max_url(influxdb_records)

    (
        trend,
        peak_value,
        peak_index,
        time_to_peak_seconds,
        num_data_points,
        max_region_first_time,
    ) = analysis_trend(influxdb_records, max_region_name)

    # query for opensearch
    influxdb_trackingid_name = "WEBEX_TRACKINGID"
    webex_tracking_id_list = list(
        {record[influxdb_trackingid_name] for record in influxdb_records}
    )
    opensearch_link = aggregator.get_opensearch_dashboard_link_from_grafana(grafana_result)
    index_pattern = aggregator.get_opensearch_index_pattern(opensearch_link)
    opensearch_query = {
        "field": {"WEBEX_TRACKINGID": webex_tracking_id_list},
        "index_pattern": index_pattern,
    }
    try:
        opensearch_records = aggregator.get_opensearch_records(incident, opensearch_query)
    except (DataAggregatorHTTPError, ApiResError) as e:
        results.log(str(e))
        results.status = RecipeStatus.FAILED
        raise

    opensearch_records_len = aggregator.get_total_opensearch_records_num(opensearch_records)

    max_userid, max_userid_count, user_id_count = analysis_max_user_id(opensearch_records)

    user_id_tracking_id_list = []
    for _, record_list in opensearch_records.items():
        for record in record_list:
            if record["fields"]["USER_ID"] == max_userid:
                tracking_id = record["fields"]["WEBEX_TRACKINGID"]
                if tracking_id not in user_id_tracking_id_list:
                    user_id_tracking_id_list.append(tracking_id)

    user_id_filter_link = aggregator.generate_opensearch_filter_link_is_one_of(
        opensearch_link, "fields.WEBEX_TRACKINGID", user_id_tracking_id_list, start_time=start_time
    )

    (
        sorted_agent_full_name_count,
        sorted_agent_org_group_count,
        confluence_log_total_count,
    ) = analysis_confluence_log(opensearch_records)

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
                    if record["fields"]["USER_ID"] == max_userid:
                        example_opensearch = record
                        break
        else:
            for _, record_list in opensearch_records.items():
                example_opensearch = record_list[0]
                break

    if example_influxdb is not None:
        example_opensearch = opensearch_records[example_influxdb[influxdb_trackingid_name]][0]

    # get the filter link for id

    # example_opensearch_id = example_opensearch["_id"]
    # id_filter_link = aggregator.generate_opensearch_filter_link_is_one_of(opensearch_link,"_id",[example_opensearch_id])

    results.log(f"From {first_error_time} To {last_error_time}, there are:")

    for error_code, count in error_code_count.items():
        results.log(f"{count} pieces of http {error_code} errors")

    if len(error_code_count) > 1:
        results.log(f"Total of {sum(error_code_count.values())} errors")
    results.log("")
    results.log(
        f"Largest percent of region is '{max_region_name}', occur in {max_region_count} ({(round(max_region_count/(error_num)*100,1))}%) of errors"
    )

    if num_data_points > 1:
        results.log(f"Trend for error occured in the region: {trend}")

    if peak_index == 0:
        results.log(
            f"Peak value for errors in the region: {peak_value}, occured in the first influxdb point at {max_region_first_time}\n"
        )
    else:
        results.log(
            f"Peak value for errors in the region: {peak_value}, occured {time_to_peak_seconds} seconds after the first influxdb point at {max_region_first_time}\n"
        )

    results.log(
        f"Largest percent of uri is '{max_uri}' with method '{max_method}', occur in {max_uri_count} ({round(max_uri_count/(error_num)*100,1)}%) of errors"
    )

    if max_uri != "calliope/api/v2/venues/{venueId}/confluences" and confluence_uri_count > 0:
        results.log(
            f"IMPORTANT! {confluence_uri_count}({round(confluence_uri_count/(error_num)*100),1} %) errors occur in uri 'calliope/api/v2/venues/{{venueId}}/confluences' with method 'POST'\n"
        )

    if len(sorted_agent_full_name_count) > 0:
        results.log("The list of affected media agents is:")
        main_percentage_total = 0
        # extract top 2 agent
        for i in range(2):
            if i < len(sorted_agent_full_name_count):
                agent_full_name, agent_full_name_count = sorted_agent_full_name_count[i]
                full_name_percentage = round(
                    agent_full_name_count / confluence_log_total_count * 100, 1
                )
                main_percentage_total += full_name_percentage
                results.log(f"{agent_full_name}: ({full_name_percentage}%)")
        if main_percentage_total != 100:
            results.log(f"other: ({round(100 - main_percentage_total,1)}%)")

        org_group_name, org_group_count = sorted_agent_org_group_count[0]
        org_group_percentage = round(org_group_count / confluence_log_total_count * 100, 1)
        results.log(
            f"Largest percent agent org and group is '{org_group_name}' ({org_group_percentage}%)\n"
        )

    results.log(f"{len(user_id_count)} unique USER ID in opensearch logs")
    results.log(
        f"Largest percent of USER ID is [{max_userid}]({user_id_filter_link}), occur in {max_userid_count} ({round(max_userid_count/opensearch_records_len*100,1)}%) of opensearch logs\n"
    )

    results.log("Example Error Info: ")
    example_openseach_field = example_opensearch["fields"]
    results.log(
        f"region:{example_opensearch['environment']}\noperation: {example_openseach_field['operation_key']}\nUSER_ID:{example_openseach_field['USER_ID']}"
    )

    if "stack_trace" in example_openseach_field:
        stack_trace = example_openseach_field["stack_trace"].split("\n")
        # filter all logs that contain com.cisco.wx2
        filtered_logs = [entry for entry in stack_trace if "com.cisco.wx2" in entry]
        if len(filtered_logs) > 0:
            results.log(f"logs: {filtered_logs[0]}")
        elif len(stack_trace) > 0:
            results.log(f"logs: {stack_trace[0]}")

    logger.info("analysis:", results.analysis)
    results.status = RecipeStatus.SUCCESSFUL


def main():
    Recipe("http-errors", handler).run()


if __name__ == "__main__":
    main()
