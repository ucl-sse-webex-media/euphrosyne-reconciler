from .config import (Aggregator_Base_Url)
from .util import parse_args
import logging

import requests

logger = logging.getLogger(__name__)


class HTTPService():
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
            response = self.session.post(url, params=params, json=body, headers=self.get_headers())
            response.raise_for_status()
            return response.json()
        except requests.exceptions.RequestException as e:
            logger.error(e)
            raise e


class DataAggregator(HTTPService):
    """Interface for the Thalia Data Aggregator."""

    SOURCES = {"grafana", "prometheus", "influxdb", "opensearch"}

    def __init__(self):
        super().__init__()
        self.url = self.parse_base_url()
        print(self.url)
        self.sources = {source: f"{self.url}/api/sources/{source}" for source in self.SOURCES}
    
    def parse_base_url(self):
        parsed_args = parse_args()
        if parsed_args.aggregator_base_url:
            return parsed_args.aggregator_base_url
        else:
            return Aggregator_Base_Url

    def get_source_url(self, source):
        """Get the base URL for a data source."""
        if source not in self.SOURCES:
            raise ValueError(f"Invalid source: '{source}'. Valid sources are: {self.SOURCES}")
        return self.sources[source]

    def get_grafana_dashboard(self, uuid, dashboard_id, panel_id):
        """Get a Grafana dashboard."""
        url = self.get_source_url("grafana")
        body = {
            "uuid": uuid,
            "params": {
                "dashboard_id": dashboard_id,
                "panel_id": panel_id,
            },
        }
        return self.post(url, params={}, body=body)
