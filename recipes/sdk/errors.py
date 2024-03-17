import requests

class ServiceError(requests.exceptions.RequestException):
    """Base class for external service errors."""
    def __init__(self, original_exception):
        self.original_exception = original_exception

    def to_dict(self):
        return {
            "error": {
                "type": type(self.original_exception).__name__,
                "message": str(self.original_exception),
            }
        }

class DataAggregatorHTTPError(ServiceError):
    pass

class SpaceHTTPError(ServiceError):
    pass

class JiraHTTPError(ServiceError):
    pass


class SpaceParsingError(ValueError):
    """Error when parsing a Space creation request."""

    pass
class JiraParsingError(ValueError):
    """Error when parsing a Jira issue creation request."""

    pass


class IncidentParsingError(ValueError):
    """Error when parsing an incident."""

    pass
