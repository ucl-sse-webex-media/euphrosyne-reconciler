import requests


#TODO: move implementation to the base class
class ServiceError(requests.exceptions.RequestException):
    """Base class for external service errors."""

    pass

#QUESTION: 7. three http errors have the same structure
# maybe we can move all parts in the base class
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

class SpaceHTTPError(ServiceError):
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


class SpaceParsingError(ValueError):
    """Error when parsing a Space creation request."""

    pass
class JiraParsingError(ValueError):
    """Error when parsing a Jira issue creation request."""

    pass


class IncidentParsingError(ValueError):
    """Error when parsing an incident."""

    pass
