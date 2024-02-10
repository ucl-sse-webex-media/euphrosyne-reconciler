import logging

from sdk.errors import IncidentParsingError

logger = logging.getLogger(__name__)


class Incident:
    """An Incident detected by the Euphrosyne Reconciler."""

    def __init__(self, uuid: str, data: dict):
        self._uuid = uuid
        self._data = self._flatten_alerts(data)

    def _flatten_alerts(self, data: dict):
        """Flatten the alert list if possible or raise an error."""
        alerts = data.get("alerts")
        if alerts and isinstance(alerts, list):
            if len(alerts) == 1:
                data.pop("alerts")
                data["alert"] = alerts[0]
            else:
                raise IncidentParsingError("Multiple alerts detected!")
        return data

    @property
    def uuid(self):
        return self._uuid

    @property
    def data(self):
        return self._data

    @classmethod
    def from_dict(cls, d: dict):
        """Create an Incident from a dictionary."""
        try:
            uuid = d.pop("uuid")
        except KeyError:
            raise IncidentParsingError("No incident UUID provided!")
        return cls(uuid, d)

    def __str__(self):
        return f"Incident '{self.uuid}': data={self.data})"
