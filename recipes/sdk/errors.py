import requests


class ServiceError(requests.exceptions.RequestException):
    """Base class for external service errors."""

    pass


class DataAggregatorHTTPError(ServiceError):
    def __init__(self, original_exception):
        self.original_exception = original_exception

    def to_dict(self):
        return {
            "error": {
                "type": type(self.original_exception).__name__,
                "message": str(self.original_exception),
            }
        }


class IncidentParsingError(ValueError):
    """Error when parsing an incident."""

    pass
