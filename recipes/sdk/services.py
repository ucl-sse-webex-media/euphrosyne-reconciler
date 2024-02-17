import logging
import requests
import os
import json
from requests.auth import HTTPBasicAuth

from sdk.errors import DataAggregatorHTTPError
from sdk.incident import Incident

logger = logging.getLogger(__name__)


class HTTPService:
    """Interface for an HTTP Service."""

    def __init__(self, url=None):
        self.session = requests.Session()
        self.url = url

    def get_headers(self):
        """Get HTTP headers."""
        return {
            "Content-Type": "application/json",
        }

    def post(self, url, params, body):
        """Send a POST request."""
        try:
            response = self.session.post(
                url, params=params, json=body, headers=self.get_headers()
            )
            response.raise_for_status()
            return response.json()
        except requests.exceptions.RequestException as e:
            logger.error(e)
            raise e


class Jira(HTTPService):
    """Interface for Jira recipe"""

    # secret
    URL = os.getenv("JIRA_URL")

    def __init__(self, url=None):
        super().__init__(url=(url or self.URL))
    
    def get_headers(self):
        return {
            "Accept": "application/json",
            "Content-Type": "application/json"
        }
    
    def get_auth(self):
        """Get HTTP headers."""
        jira_user = os.getenv("JIRA_USER")
        jira_token = os.getenv("JIRA_TOKEN")
        auth = HTTPBasicAuth(jira_user, jira_token)
        return auth
    
    # TODO: configuration
    # TODO: process error
    def post(self, url, body):
        auth = self.get_auth()
        try:
            response = requests.post(url, json=body, headers=self.get_headers(), auth=auth)
            response.raise_for_status()
            return response.json()
        except requests.exceptions.RequestException as e:
            logger.error(e)
            raise e

    # TODO: complete creating issue
    def create_issue(self, data: dict):
        data = data["data"]
        summary = data.get("summary")
        if summary is None:
            summary = "This is a summary"
        issuetype = data.get("issuetype")
        if issuetype is None:
            issuetype = "10001"
        project = data.get("project")
        if project is None:
            project = "SCRUM"
        description = data.get("description")
        if description is None:
            description = "description"

        ticket_fields = {
            "fields": {
                "summary": summary,
                "issuetype": {"id": issuetype},
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

        payload = json.dumps(ticket_fields)

        


        # response = requests.request(
        #     "POST",
        #     self.url,
        #     data=payload,
        #     headers=self.get_headers(),
        #     auth=self.get_auth
        # )


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
            raise ValueError(
                f"Invalid source: '{source}'. Valid sources are: {self.SOURCES}"
            )
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
        dashboard_id = alert.get(
            "dahsboard_id"
        ) or self._get_grafana_dashboard_from_url(alert["dashboardURL"])
        panel_id = alert.get("panel_id") or self._get_grafana_panel_from_url(
            alert["panelURL"]
        )
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
        return self.post(url, params={}, body=body)
