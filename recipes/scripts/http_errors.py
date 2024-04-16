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

# the key name in InfluxDB that is used to count the number of errors
count_key = "_value"


def precise_timestamp_to_second(timestamp_str):
    return timestamp_str[:19] + "Z"


def get_influxdb_start_and_end_time(influxdb_records):
    """Get the start and end time of accurate InfluxDB records."""
    start_time = influxdb_records[0]["_time"]
    end_time = influxdb_records[-1]["_time"]
    return precise_timestamp_to_second(start_time), precise_timestamp_to_second(end_time)


def count_metric_by_key(record_list, metric, count_key):
    """Count the number of metric by a key in list."""
    count = {}
    for item in record_list:
        if item[metric] in count:
            count[item[metric]] += item[count_key]
        else:
            count[item[metric]] = item[count_key]
    return count


def analyse_max_region(influxdb_records):
    """Find the largest percentage of region (environment)."""
    region_count = count_metric_by_key(influxdb_records, "environment", count_key)
    max_region_name = max(region_count, key=region_count.get)
    max_region_count = region_count[max_region_name]
    return max_region_name, max_region_count


def analyse_max_url(influxdb_records):
    """Analyse max percent url and method, and find the Confluence uri count."""
    uri_count = {}
    confluence_uri_count = 0
    # {uri:{method:count}}
    # find is there any Confluence uri with method post
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

    # find the largest percent of uri
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


def analyse_max_user_id(opensearch_records):
    """Analyse max percent user id in OpenSearch records."""
    user_id_list = []
    for record in opensearch_records:
        user_id_list.append(record["fields"].get("USER_ID", ""))

    # find the largest percent of user_id
    user_id_count = Counter(user_id_list)
    max_userid = max(user_id_count, key=user_id_count.get)
    max_userid_count = user_id_count[max_userid]
    return max_userid, max_userid_count, user_id_count


def analyse_confluence_log(opensearch_records):
    """Analyse Confluence log."""
    confluence_log_list = []
    for record in opensearch_records:
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


def analyse_trend(influxdb_records, max_region):
    """Analyse error trend in the max region."""
    # Filter records for the specified max_region
    max_region_records = [
        record for record in influxdb_records if record["environment"] == max_region
    ]

    # Ensure there are records to analyse after filtering
    if not max_region_records:
        return "No records to analyse for the specified region."

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


def get_total_opensearch_records_num(opensearch_records):
    """
    Get the total number of OpenSearch records.
    """
    return len(opensearch_records)


def handler(incident: Incident, recipe: Recipe):
    """HTTP Errors Recipe."""
    logger.info("Received input:", incident)

    results, aggregator = recipe.results, recipe.aggregator

    # query for Grafana
    try:
        grafana_result = aggregator.get_grafana_info_from_incident(incident, force_latest=True)
    except (DataAggregatorHTTPError, ApiResError) as e:
        results.log(str(e))
        results.status = RecipeStatus.FAILED
        raise

    # query for InfluxDB
    firing_time = aggregator.get_firing_time_from_incident(incident)
    query_start_time = aggregator.calculate_query_start_time(grafana_result, firing_time)

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
        "startTime": query_start_time,
        "stopTime": firing_time,
        "tagSets": tag_set,
    }
    try:
        influxdb_records = aggregator.get_influxdb_records(
            incident, influxdb_query, force_latest=True
        )
    except (DataAggregatorHTTPError, ApiResError) as e:
        results.log(str(e))
        results.status = RecipeStatus.FAILED
        raise
    # get the start and end time of InfluxDB records
    first_error_time, last_error_time = get_influxdb_start_and_end_time(influxdb_records)

    error_num = sum(item[count_key] for item in influxdb_records)

    # count how many different error codes, e.g. 500, 501
    error_code_count = count_metric_by_key(influxdb_records, "httpStatusCode", count_key)

    # analyse max region
    max_region_name, max_region_count = analyse_max_region(influxdb_records)

    # analyse max url
    max_uri, max_method, max_uri_count, confluence_uri_count = analyse_max_url(influxdb_records)

    (
        trend,
        peak_value,
        peak_index,
        time_to_peak_seconds,
        num_data_points,
        max_region_first_time,
    ) = analyse_trend(influxdb_records, max_region_name)

    # query for OpenSearch
    grafana_opensearch_config_list = aggregator.get_grafana_opensearch_config_list(grafana_result)

    # only one datalink is set in grafana for now
    grafana_opensearch_config = grafana_opensearch_config_list[0]
    # the key name in influxDB used to link with OpenSearch
    influxdb_tracking_key_name = grafana_opensearch_config["dataSourceField"]
    webex_tracking_id_list = list(
        {record[influxdb_tracking_key_name] for record in influxdb_records}
    )
    opensearch_link = grafana_opensearch_config["url"]
    index_pattern = grafana_opensearch_config["indexPattern"]

    # the OpenSearch filter key name correponding to key name in influxDB
    opensearch_filter_key_name = grafana_opensearch_config["osFilterKey"]
    opensearch_query = {
        "field": {opensearch_filter_key_name: webex_tracking_id_list},
        "index_pattern": index_pattern,
    }
    try:
        opensearch_records = aggregator.get_opensearch_records(
            incident, opensearch_query, force_latest=True
        )
    except (DataAggregatorHTTPError, ApiResError) as e:
        results.log(str(e))
        results.status = RecipeStatus.FAILED
        raise

    opensearch_records_len = get_total_opensearch_records_num(opensearch_records)

    # analyse max user id
    max_userid, max_userid_count, user_id_count = analyse_max_user_id(opensearch_records)

    user_id_tracking_id_list = []
    # find the corresponding tracking id for the max user id
    for record in opensearch_records:
        if record["fields"]["USER_ID"] == max_userid:
            tracking_id = record["fields"][opensearch_filter_key_name]
            if tracking_id not in user_id_tracking_id_list:
                user_id_tracking_id_list.append(tracking_id)

    user_id_filter_link = aggregator.generate_opensearch_filter_link_is_one_of(
        opensearch_link,
        f"fields.{opensearch_filter_key_name}",
        user_id_tracking_id_list,
        start_time=query_start_time,
    )

    (
        sorted_agent_full_name_count,
        sorted_agent_org_group_count,
        confluence_log_total_count,
    ) = analyse_confluence_log(opensearch_records)

    # find the example error
    # if the Confluence uri is in errors, use error with the uri as the example
    if confluence_uri_count > 0:
        for item in influxdb_records:
            if (
                re.search(r"/calliope/api/v2/venues/.+?/confluences", item["uri"])
                and item["method"] == "POST"
            ):
                example_influxdb_record = item
                break
    else:
        # find an example that has max metric value
        # priority: region > uri > USER_ID
        max_dict = {
            "environment": max_region_count,
            "uri": max_uri_count,
            "USER_ID": max_userid_count,
        }
        max_metric = max(max_dict, key=max_dict.get)
        if max_metric == "environment":
            for item in influxdb_records:
                if item["environment"] == max_region_name:
                    example_influxdb_record = item
                    break
        elif max_metric == "uri":
            for item in influxdb_records:
                if item["uri"] == max_uri:
                    example_influxdb_record = item
                    break
        elif max_metric == "USER_ID":
            for record in opensearch_records:
                if record["fields"]["USER_ID"] == max_userid:
                    example_opensearch = record
                    break

    # example error is found in InfluxDB records, then find the corresponding OpenSearch record
    if example_influxdb_record is not None:
        for record in opensearch_records:
            if (
                record["fields"][opensearch_filter_key_name]
                == example_influxdb_record[influxdb_tracking_key_name]
            ):
                example_opensearch = record
                break

    results.log(f"From {first_error_time} To {last_error_time}, there are:")
    for error_code, count in error_code_count.items():
        results.log(f"* {count} HTTP {error_code} errors")

    if len(error_code_count) > 1:
        results.log(f"Total of {sum(error_code_count.values())} errors")

    results.log(
        f"Largest percent of region is '{max_region_name}', occur in {max_region_count}"
        f" ({round(max_region_count / (error_num) * 100, 1)}%) of errors"
    )

    if num_data_points > 1:
        results.log(f"Trend for error occurred in the region: {trend}")

    if peak_index == 0:
        results.log(
            f"Peak value for errors in the region: {peak_value}, occurred in the first InfluxDB"
            f" point at {max_region_first_time}"
        )
    else:
        results.log(
            f"Peak value for errors in the region: {peak_value}, occurred {time_to_peak_seconds}"
            f" seconds after the first InfluxDB point at {max_region_first_time}"
        )

    results.log(
        f"Largest percent of uri is '{max_uri}' with method '{max_method}', occur in"
        f" {max_uri_count} ({round(max_uri_count/(error_num)*100,1)}%) of errors"
    )

    if max_uri != "calliope/api/v2/venues/{venueId}/confluences" and confluence_uri_count > 0:
        results.log(
            f"IMPORTANT! {confluence_uri_count}("
            f"{round(confluence_uri_count / (error_num) * 100), 1} %) of errors occur for uri"
            f" 'calliope/api/v2/venues/{{venueId}}/confluences' with method 'POST'"
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
            results.log(f"other: ({round(100 - main_percentage_total, 1)}%)")

        org_group_name, org_group_count = sorted_agent_org_group_count[0]
        org_group_percentage = round(org_group_count / confluence_log_total_count * 100, 1)
        results.log(
            f"Largest percent agent org and group is '{org_group_name}' ({org_group_percentage}%)"
        )

    results.log(f"{len(user_id_count)} unique USER ID in OpenSearch logs")
    results.log(
        f"Largest percent of USER ID is [{max_userid}]({user_id_filter_link}), occur in"
        f" {max_userid_count} ({round(max_userid_count/opensearch_records_len*100,1)}%) of"
        " OpenSearch logs"
    )

    results.log("Example Error Info: ")
    example_openseach_field = example_opensearch["fields"]
    results.log(f"region:{example_opensearch['environment']}")
    results.log(f"operation: {example_openseach_field['operation_key']}")
    results.log(f"USER_ID:{example_openseach_field['USER_ID']}")

    if "stack_trace" in example_openseach_field:
        stack_trace = example_openseach_field["stack_trace"].split("\n")
        # filter all logs that contain com.cisco.wx2
        filtered_logs = [entry for entry in stack_trace if "com.cisco.wx2" in entry]
        if len(filtered_logs) > 0:
            results.log(f"stack_trace: {filtered_logs[0]}")
        elif len(stack_trace) > 0:
            results.log(f"stack_trace: {stack_trace[0]}")

    logger.info("analysis:", results.analysis)
    results.status = RecipeStatus.SUCCESSFUL


def main():
    Recipe("http-errors", handler).run()


if __name__ == "__main__":
    main()
