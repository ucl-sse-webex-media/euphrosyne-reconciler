import logging
import os
import re
from datetime import datetime, timedelta
from urllib.parse import urlparse

import requests
from requests.auth import HTTPBasicAuth

from sdk.errors import ApiResError, DataAggregatorHTTPError, JiraHTTPError, JiraParsingError
from sdk.incident import Incident

logger = logging.getLogger(__name__)


class HTTPService:
    """Interface for an HTTP Service."""

    def __init__(self, url):
        self.session = requests.Session()
        self.url = url

    def get_headers(self):
        """Get HTTP headers."""
        return {
            "Accept": "application/json",
            "Content-Type": "application/json",
        }

    def get(self, url, params=None, auth=None):
        """Send a GET request."""
        try:
            response = self.session.get(
                url,
                params=params,
                headers=self.get_headers(),
                auth=auth,
            )
            response.raise_for_status()
            return response.json()
        except requests.exceptions.RequestException as e:
            logger.error(e)
            raise e

    def post(self, url, params=None, body=None, auth=None):
        """Send a POST request."""
        try:
            response = self.session.post(
                url,
                params=params,
                json=body,
                headers=self.get_headers(),
                auth=auth,
            )
            response.raise_for_status()
            return response.json()
        except requests.exceptions.RequestException as e:
            logger.error(e)
            raise e


class Jira(HTTPService):
    """Interface for Atlassian Jira."""

    ISSUE_DEFAULTS = {
        "issuetype": "Story",
        "project": "SCRUM",
        "description": "This is the issue description.",
    }

    def __init__(self, url=None):
        self._load_environment_variables()
        super().__init__(url or self.url)

    def _load_environment_variables(self):
        """Load environment variables."""
        self.url = os.getenv("JIRA_URL")
        self.user = os.getenv("JIRA_USER")
        self.token = os.getenv("JIRA_TOKEN")
        if not self.url or not self.user or not self.token:
            raise JiraParsingError(
                "JIRA_URL, JIRA_USER, and JIRA_TOKEN environment variables must be set."
            )

    def get_auth(self):
        """Get HTTP Basic Authentication object."""
        return HTTPBasicAuth(self.user, self.token)

    def get_issue_url(self, project_key: str, issue_key: str):
        """Get the URL for a Jira issue."""
        parsed_url = urlparse(self.url)
        base_url = f"{parsed_url.scheme}://{parsed_url.netloc}"

        board_url = f"{base_url}/rest/agile/1.0/board?projectKeyOrId={project_key}"
        board_details = self.get(board_url, auth=self.get_auth())

        board_id = board_details["values"][0]["id"]
        issue = f"?selectedIssue={issue_key}"
        return f"{base_url}/jira/software/projects/{project_key}/boards/{board_id}/{issue}"

    def create_issue(self, data: dict):
        """Create a Jira issue."""
        summary = data.get("summary")
        if not summary:
            raise JiraParsingError("Summary needs to be provided.")
        issuetype = data.get("issuetype") or self.ISSUE_DEFAULTS["issuetype"]
        project = data.get("project") or self.ISSUE_DEFAULTS["project"]
        description = data.get("description") or self.ISSUE_DEFAULTS["description"]

        issue_fields = {
            "fields": {
                "summary": summary,
                "issuetype": {"name": issuetype},
                "project": {"key": project},
                "description": {
                    "type": "doc",
                    "version": 1,
                    "content": [
                        {
                            "type": "paragraph",
                            "content": [{"text": description, "type": "text"}],
                        }
                    ],
                },
            }
        }
        try:
            response = self.post(self.url, body=issue_fields, auth=self.get_auth())
            issue_key = response.get("key")
            detail_url = self.get_issue_url(project, issue_key)
            return {"key": issue_key, "summary": summary, "url": detail_url}
        except requests.exceptions.RequestException as e:
            logger.error("Failed to create Jira issue: ", e)
            raise JiraHTTPError(e)


class DataAggregator(HTTPService):
    """Interface for the Thalia Data Aggregator."""

    URL = "http://localhost:8080"
    SOURCES = {"grafana", "prometheus", "influxdb", "opensearch"}

    def __init__(self, aggregator_address):
        super().__init__(url=(aggregator_address or self.URL))
        self.sources = {source: f"{self.url}/api/sources/{source}" for source in self.SOURCES}

    def get_source_url(self, source):
        """Get the base URL for a data source."""
        if source not in self.SOURCES:
            raise ValueError(f"Invalid source: '{source}'. Valid sources are: {self.SOURCES}")
        return self.sources[source]

    def post(self, args, **kwargs):
        """Send a POST request to the Data Aggregator service and handle errors."""
        try:
            res = super().post(args, **kwargs)
        except requests.exceptions.RequestException as e:
            raise DataAggregatorHTTPError(e)
        self._check_api_res_error(res)
        return res["data"]

    def _check_api_res_error(self, res):
        """Check if the API response contains an error and raise an exception if it does."""
        if res.get("error") is not None:
            raise ApiResError(res.get("error"))

    def _get_grafana_dashboard_from_url(self, url: str):
        """Get a Grafana dashboard ID from a URL."""
        return url.rsplit("/", 1)[-1].split("?")[0]

    def _get_grafana_panel_from_url(self, url: str):
        """Get a Grafana panel ID from a URL."""
        return url.rsplit("=", 1)[-1]

    def _get_alert_rule_from_url(self, url: str):
        """Get alert rule from a URL."""
        return url.split("/")[-2]

    def _get_grafana_config_ids(self, data: dict):
        """Get the Grafana dashboard, alert panel and alert rule ids from the alert."""
        alert = data.get("alert")
        dashboard_id = self._get_grafana_dashboard_from_url(alert["dashboardURL"])
        panel_id = self._get_grafana_panel_from_url(alert["panelURL"])
        alert_rule_id = self._get_alert_rule_from_url(alert["generatorURL"])

        return dashboard_id, panel_id, alert_rule_id

    def get_grafana_info_from_incident(self, incident: Incident, force_latest=False):
        """
        Get alert rule, InfluxDB and OpenSearch config on grafana.

        Parameters:
        incident (Incident): The incident object.
        force_latest (bool): Whether to fetch the latest data.

        Returns:
        dict: The response of garfana query API.
        """
        dashboard_id, panel_id, alert_rule_id = self._get_grafana_config_ids(incident.data)
        url = self.get_source_url("grafana")
        body = {
            "uuid": incident.uuid,
            "params": {
                "dashboard_id": dashboard_id,
                "panel_id": panel_id,
                "alert_rule_id": alert_rule_id,
                "force_latest": force_latest,
            },
        }
        return self.post(url, body=body)

    def get_firing_time_from_incident(self, incident: Incident):
        """Get alert firing time.

        Parameters:
        incident (Incident): The incident object.

        Returns:
        str: The firing time of the alert.
        """
        alert = incident.data.get("alert")
        # The startsAt in grafana alert only represents the firing time (stop time of query)
        return alert["startsAt"]

    def calculate_query_start_time(self, grafana_result, firing_time):
        """
        Calculate query start time by grafana_result.

        Start time = firing time - pending time - querying duration - querying interval

        Parameters:
        grafana_result (dict): response from "get_grafana_info_from_incident" method.
        firing_time (str): The firing time of the alert.

        Returns:
        str: The start time of the query.
        """
        alert_rule = grafana_result["alertRule"]
        fmt_firing_time = datetime.strptime(firing_time, "%Y-%m-%dT%H:%M:%SZ")
        alert_query = alert_rule["data"][0]
        query_time_range = (
            alert_query["relativeTimeRange"]["from"] - alert_query["relativeTimeRange"]["to"]
        )
        query_interval = alert_query["model"]["intervalMs"]
        # alert_rule["for"] is the pending time, initially is like "10s" format
        pending_time = int(re.findall(r"\d+", alert_rule["for"])[0])
        fmt_start_time = (
            fmt_firing_time
            - timedelta(seconds=pending_time)
            - timedelta(seconds=query_time_range)
            - timedelta(milliseconds=query_interval)
        )
        return fmt_start_time.strftime("%Y-%m-%dT%H:%M:%SZ")

    def get_influxdb_bucket_from_grafana(self, grafana_result):
        """
        Get InfluxDB bucket from grafana_result.

        Parameters:
        grafana_result (dict): response from "get_grafana_info_from_incident" method.

        Returns:
        str: The InfluxDB bucket name.
        """
        return grafana_result["influxdb"]["dbName"]

    def get_influxdb_measurement_from_grafana(self, grafana_result):
        """
        Get InfluxDB measurement from grafana_result.

        Parameters:
        grafana_result (dict): response from "get_grafana_info_from_incident" method.

        Returns:
        str: The InfluxDB measurement name.
        """
        alert_rule = grafana_result["alertRule"]
        model = alert_rule["data"][0]["model"]
        # alert query configured in default editor mode
        if "measurement" in model and model["measurement"] != "":
            return model["measurement"]

        # alert query configured in query mode
        query = model["query"]
        match = re.search(r'FROM\s+"([^"]+)"\s+WHERE', query)
        if match:
            result = match.group(1)
        else:
            result = ""
        return result

    def _parse_tags(self, tag_str):
        """
        Parse tags and values by operator.

        String before operator is the tag name

        String After operator is the tag value
        """
        pattern = r"!=|<>|=~|!~|>=|<=|[><=]"
        match = re.search(pattern, tag_str)
        if match:
            start_pos = match.start()
            key = tag_str[:start_pos].strip().replace('"', "").replace("'", "")
            # bracket right after "where"
            if key[0] == "(":
                key = key[1:]
            key = key.strip()
            end_pos = match.end()
            value = tag_str[end_pos:].strip()
            # last bracket that wrap the "where" content
            if value[-1] == ")":
                value = value[:-1].strip()
            # value is string,remove qutation mark
            if value[0] == "'" or value[0] == '"':
                value = value[1:-1]
            # value is int or boolean,remove extra space
            else:
                value = value.strip()
            operator = tag_str[start_pos:end_pos]

        return key, value, operator

    def get_influxdb_tags_from_grafana(self, grafana_result):
        """
        Get InfluxDB tags from grafana_result.

        Parameters:
        grafana_result (dict): response from "get_grafana_info_from_incident" method.

        Rertuns:
        list of dict: The list of InfluxDB tags.
        {
            "key": str,
            "value": str,
            "operator": str
        }
        """
        alert_rule = grafana_result["alertRule"]
        model = alert_rule["data"][0]["model"]
        if "tags" in model and len(model["tags"]) != 0:
            return model["tags"]
        query = model["query"]
        result = []
        if "WHERE" not in query:
            return result
        # alert query configured in query mode
        # extract content after "where"
        where_index = query.find("WHERE")
        query = query[where_index + 6 :]
        # remove the possible query str after "where" content
        if "$timeFilter" in query:
            index = query.find("$timeFilter")
            query = query[:index]
        elif "GROUP BY" in query.upper():
            index = query.upper().find("GROUP BY")
            query = query[:index]
        query = query.replace("and ", "AND ").replace("or ", "OR ")
        # split by "AND", each split item contains tag name, operator and value
        parts = query.split("AND ")
        for part in parts:
            if part.strip() == "":
                continue
            if "OR " in part:
                # Handle OR conditions within an AND segment
                or_parts = part.split("OR ")
                for or_part in or_parts:
                    key, value, operator = self._parse_tags(or_part)
                    result.append({"key": key, "value": value, "operator": operator})
            else:
                key, value, operator = self._parse_tags(part)
                result.append({"key": key, "value": value, "operator": operator})
        return result

    def get_influxdb_records(self, incident: Incident, influxdb_query, force_latest=False):
        """
        Get InfluxDB records.

        Parameters:
        incident (Incident): The incident object.
        influxdb_query (dict): The InfluxDB query.
        {
            "bucket": str,
            "measurement": str,
            "tagSets": list of dict
            [
                {
                tagkeyName:tagvalue
                }
            ],
            "start_time": str,
            "end_time": str
        }
        force_latest (bool): Whether to fetch the latest data.

        Returns:
        dict: The response of InfluxDB query API.
        """
        url = self.get_source_url("influxdb")
        if influxdb_query.get("force_latest") is None:
            influxdb_query["force_latest"] = force_latest
        body = {"uuid": incident.uuid, "params": influxdb_query}
        return self.post(url, body=body)

    def get_grafana_opensearch_config_list(self, grafana_result):
        return grafana_result["opensearch"]

    def get_opensearch_records(self, incident: Incident, opensearch_query, force_latest=False):
        """
        Get OpenSearch records.

        Parameters:
        incident (Incident): The incident object.
        force_latest (bool): Whether to fetch the latest data.

        Returns:
        dict: The response of OpenSearch query API.
        """
        url = self.get_source_url("opensearch")
        if opensearch_query.get("force_latest") is None:
            opensearch_query["force_latest"] = force_latest
        body = {"uuid": incident.uuid, "params": opensearch_query}
        return self.post(url, body=body)

    def get_opensearch_index_pattern_from_link(self, opensearch_dashboard_link):
        """
        Get index pattern from OpenSearch link.

        Parameters:
        opensearch_dashboard_link (str): The OpenSearch dashboard link.

        Returns:
        str: The index pattern.
        """
        start_str = "indexPattern:'"
        start_index = opensearch_dashboard_link.find(start_str) + len(start_str)
        end_index = opensearch_dashboard_link.find("'", start_index)
        index_pattern_url = opensearch_dashboard_link[start_index:end_index]
        return index_pattern_url

    def generate_opensearch_filter_link_is_one_of(
        self, opensearch_dashboard_link, filter_key, filter_data_list, start_time="", end_time=""
    ):
        """
        Generate OpenSearch dashboard filter link

        Filter OpenSearch records that the values of filter_key is one of the values in filter_data_list.

        e.g. filter_key is "WEBEX_TRACKINGID", filter_data_list is ["123456", "234567"], the link can filter all records that the value of "WEBEX_TRACKINGID" is "123456" or "234567".

        For item in filter_data_list, if it starts with an digit, it should be wrapped with single quote in the 'params' and 'should' varibales.

        Parameters:
        opensearch_dashboard_link (str): The OpenSearch dashboard link.
        filter_key (str): The filter key name like "webextrackingID".
        filter_data_list (list): The filter data list.
        start_time (str): The start time of the query.
        end_time (str): The end time of the query.

        Returns:
        str: The OpenSearch filter link.
        """
        if start_time != "":
            opensearch_dashboard_link = re.sub(
                r"from:[^,)]+", f"from:'{start_time}'", opensearch_dashboard_link
            )
        if end_time != "":
            opensearch_dashboard_link = re.sub(
                r"to:[^,)]+", f"to:{end_time}", opensearch_dashboard_link
            )
        index_pattern = self.get_opensearch_index_pattern_from_link(opensearch_dashboard_link)
        template_index = "(match_phrase:({filter_key}:{filter_data}))"
        template_index_with_digit = "(match_phrase:({filter_key}:'{filter_data}'))"
        formatted_items = []
        for filter_data in filter_data_list:
            if filter_data[0].isdigit():
                formatted_items.append(
                    template_index_with_digit.format(
                        filter_key=filter_key, filter_data=filter_data
                    )
                )
            else:
                formatted_items.append(
                    template_index.format(filter_key=filter_key, filter_data=filter_data)
                )
        # fixed pattern of OpenSearch dashboard link string generation
        params = ",".join([f"'{s}'" if s[0].isdigit() else s for s in filter_data_list])
        value = ",%20".join(filter_data_list)
        should = ",".join(formatted_items)
        query_string = (
            "(filters:!(('$state':(store:appState),meta:(alias:!n,disabled:!f,"
            f"index:'{index_pattern}',key:{filter_key},negate:!f,params:!({params}),"
            f"type:phrases,value:'{value}'),query:(bool:(minimum_should_match:1,"
            f"should:!({should}))))),query:(language:kuery,query:''))"
        )
        filter_link = re.sub(r"(&_q).*", f"&_q={query_string}", opensearch_dashboard_link)
        return filter_link

    def shorten_opensearch_dashboard_link(self, url):
        """
        Shorten OpenSearch url.

        Parameters:
        url (str): The OpenSearch dashboard link after "_dashboards/" part.

        Returns:
        str : full shortern url.
        """
        url = self.get_source_url("opensearch") + "/shortern_url"
        return self.post(url, body={url: url})
