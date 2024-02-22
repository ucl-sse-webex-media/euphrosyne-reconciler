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

class JiraHTTPError(ServiceError):
    def __init__(self, original_exception):
        self.original_exception = original_exception

    def to_dict(self):
        return {
            "error": {
                "type": type(self.original_exception).__name__,
                "message": str(self.original_exception),
            }
        }



class JiraParsingError(ValueError):
    """Error when parsing a Jira issue creation request."""

    pass


class IncidentParsingError(ValueError):
    """Error when parsing an incident."""

    pass


class ApiResError(ValueError):
    """Error in the api response"""

    pass