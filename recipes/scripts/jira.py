import logging

from sdk.incident import Incident
from sdk.recipe import Recipe, RecipeResults, RecipeStatus
from sdk.services import Jira

logger = logging.getLogger(__name__)


def handler(incident: Incident, results: RecipeResults):
    """Create Jira Issue Recipe."""
    logger.info("Create jira ticket")

    jira = Jira()
    response = jira.create_issue(incident.data)

    if response.status_code == 200:
        results.log("create jira ticket successfully" + response.text)
        results.status = RecipeStatus.SUCCESSFUL
    else:
        results.log("fail to create jira ticket" + response.text)
        results.status = RecipeStatus.FAILED
    return results


def main():
    Recipe("jira", handler).run()


if __name__ == "__main__":
    main()
