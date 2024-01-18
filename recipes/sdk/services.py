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

    URL = "http://thalia-aggregator.default.svc.cluster.local:80"
    SOURCES = {"grafana", "prometheus", "influxdb", "opensearch"}

    def __init__(self, url=None):
        super().__init__(url=(url or self.URL))
        self.sources = {source: f"{self.url}/sources/{source}" for source in self.SOURCES}

    def get_source_url(self, source):
        """Get the base URL for a data source."""
        if source not in self.SOURCES:
            raise ValueError(f"Invalid source: '{source}'. Valid sources are: {self.SOURCES}")
        return self.sources[source]

    def get_grafana_dashboard(self, dashboard_id):
        """Get a Grafana dashboard."""
        url = f"{self.get_source_url('grafana')}/dashboards/{dashboard_id}"
        return self.post(url, params={}, body={})
