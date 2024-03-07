import logging
import os
from urllib.parse import urlparse

import requests
from requests.auth import HTTPBasicAuth

from sdk.errors import DataAggregatorHTTPError, JiraHTTPError, JiraParsingError, \
    SpaceParsingError, SpaceHTTPError
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


class Space(HTTPService):
    #TODO: raise error
    '''
    create room: /rooms
    header: personal access token
    create a room, authenticated user is automatically added as a member
    parameters: title, description

    create membership: /memberships
    header: personal access token
    add someone (personId or personEmail) to a room (roomId)
    parameters: roomId, personId, personEmail
    '''
    def __init__(self, url=None):
        self.url = "https://webexapis.com/v1"
        super().__init__(url or self.url)
        self._load_environment_variables()

    def _load_environment_variables(self):
        """Load environment variables."""
        #TODO: add token to environment variables
        self.token = os.getenv("WEBEX_TOKEN")
        self.bot_token = os.getenv("BOT_TOKEN")
        if not self.token or not self.bot_token:
            raise SpaceParsingError("WEBEX_TOKEN and BOT_TOKEN environment variables must be set.")

    def get_headers(self):
        """Get Authentication Header For Admin"""
        return {
            'Authorization': 'Bearer ' + self.token
        }

    def get_bot_headers(self):
        """Get Authentication Header For Bot"""
        return {
            'Authorization': 'Bearer ' + self.bot_token
        }

    def create_room(self, data: dict):
        """Create a Webex Teams Room."""
        title = data.get("title")
        if not title:
            raise SpaceParsingError("Title is required.")
        try:
            # check if there is any exitsting room
            room_id = None
            # QUESTION: which token should be used, bot or admin?
            response = self.get(self.url + "/rooms", headers=self.get_headers())
            for room in response["items"]:
                if room["title"] == title:
                    return room["id"]
        except requests.exceptions.RequestException as e:
            logger.error("Failed to get Webex Teams room: ", e)
            raise SpaceHTTPError(e)

        description = data.get("description") or data.get("uuid")
        # create a new room
        room_fields = {
            "title": title,
            "description": description
        }
        try:
            response = self.post(self.url + "/rooms", body=room_fields, headers=self.get_headers())
            return response["id"]
        except requests.exceptions.RequestException as e:
            logger.error("Failed to create Webex Teams room: ", e)
            raise SpaceHTTPError(e)


    def add_user(self, data: dict, roomId: str):
        """Add a user to a Webex Teams Room."""
        personEmails = data.get("personEmails")
        if not personEmails:
            raise SpaceParsingError("Emails of User is required.")

        #TODO: beautify the response
        add_user_response = ""
        for personEmail in personEmails:
            membership_fields = {
                "roomId": roomId,
                "personId": personEmail,
            }
            try:
                response = self.post(self.url + "/memberships", body=membership_fields, headers=self.get_headers())
                add_user_response += response["personEmail"] + " added to the room\n"
            except requests.exceptions.RequestException as e:
                logger.error("Failed to add user to Webex Teams room: ", e)
                raise SpaceHTTPError(e)

        return add_user_response

    def post_analysis(self, data, roomId: str):
        body = {
            "roomId": roomId,
            "text": data["analysis"]
        }
        try:
            response = self.post(self.url + "/messages", body=body, headers=self.get_bot_headers())
            return response["text"]
        except requests.exceptions.RequestException as e:
            logger.error("Failed to post analysis to Webex Teams room: ", e)
            raise SpaceHTTPError(e)

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
        return res

    def _get_grafana_dashboard_from_url(self, url: str):
        """Get a Grafana dashboard ID from a URL."""
        return url.rsplit("/", 1)[-1].split("?")[0]

    def _get_grafana_panel_from_url(self, url: str):
        """Get a Grafana panel ID from a URL."""
        return url.rsplit("=", 1)[-1]

    def _get_grafana_dashboard_and_panel(self, data: dict):
        """Get the Grafana dashboard and specific panel from the input data."""
        alert = data.get("alert")
        dashboard_id = alert.get("dahsboard_id") or self._get_grafana_dashboard_from_url(
            alert["dashboardURL"]
        )
        panel_id = alert.get("panel_id") or self._get_grafana_panel_from_url(alert["panelURL"])
        return dashboard_id, panel_id

    def get_grafana_dashboard_from_incident(self, incident: Incident):
        """Get a Grafana dashboard."""
        uuid = incident.uuid
        dashboard_id, panel_id = self._get_grafana_dashboard_and_panel(incident.data)
        url = self.get_source_url("grafana")
        body = {
            "uuid": uuid,
            "params": {
                "dashboard_id": dashboard_id,
                "panel_id": panel_id,
            },
        }
        return self.post(url, body=body)
