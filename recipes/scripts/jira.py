import logging
import requests
import json

from sdk.incident import Incident
from sdk.recipe import Recipe, RecipeResults, RecipeStatus
from sdk.services import Jira

logger = logging.getLogger(__name__)

def handler(incident: Incident, results: RecipeResults):
    """Create Jira Issue Recipe."""
    logger.info("Create jira ticket")
    
    jira = Jira()
    jira.create_issue(incident.data)

    
    

    # fileds: summary, issuetype, parent, description, customfield_10020, project, customfield_10021, 
    # reporter, customfield_10000, labels, customfield_10016, customfield_10019, attachment, issuelinks, assignee
    # TODO: get fields from bot
    summary_text = "Summit 2019 is awesome!"
    description_text = "from reconciler"
    data = {
        "fields": {
            "summary": summary_text,
            "issuetype": {
                "id": "10001"
            },
            "project": {
                "key": "SCRUM"
            },
            "description": {
                "type": "doc",
                "version": 1,
                "content": [
                    {
                        "type": "paragraph",
                        "content": [
                            {
                                "text": description_text,
                                "type": "text"
                            }
                        ]
                    }
                ]
            }
        }
    }

    response = requests.post(url, headers=headers, data=json.dumps(data))
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
