import logging

from sdk.errors import JiraHTTPError, JiraParsingError
from sdk.incident import Incident
from sdk.recipe import Recipe, RecipeStatus
from sdk.services import Jira

logger = logging.getLogger(__name__)


def handler(incident: Incident, recipe: Recipe):
    """Create Jira Issue Recipe."""
    logger.info("Create Jira issue")
    
    jira = Jira()
    results = recipe.results
    try:
        issue = jira.create_issue(incident.data)
        print(issue)
        key, summary, url = issue["key"], issue["summary"], issue["url"]
        results.log(f"Jira issue '{summary}' with key '{key}' created successfully: {url}")
        results.status = RecipeStatus.SUCCESSFUL
    except (JiraHTTPError, JiraParsingError) as e:
        results.log(f"Failed to create Jira issue: {e}")
        results.status = RecipeStatus.FAILED


def main():
    Recipe("jira", handler).run()


if __name__ == "__main__":
    main()
