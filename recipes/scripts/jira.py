import logging
import requests
import json
import base64
import os

from sdk.recipe import Recipe, RecipeResults, RecipeStatus
logger = logging.getLogger(__name__)

def handler(data: dict, results: RecipeResults):
    # email = 'bambooliulala@gmail.com'
    # token = 'ATATT3xFfGF0Ofo12ijF8Js4pmEnX7pDp-FcpEhIawOAqxgR7NmYOgGn1EYe_7G_-44bdEGLQ3RsLrgpq59jC0W0xqz0gCLOSpz4AmXLN_M3md_JptK-amGAjyz7rH7idqZIDZFZTUPKcaUlmnHOiG4CwpssqafHXNhEZJBA2995D3-8X-wohvc=7894AA6C'
    # credentials = f'{email}:{token}'
    logger.info("Create jira ticket")
    jira_credentials = os.getenv('JIRA_CREDENTIALS')

    if jira_credentials is not None:
        print("JIRA Credentials:", jira_credentials)
    else:
        print("JIRA_CREDENTIALS environment variable is not set.")
        logger.info("JIRA_CREDENTIALS environment variable is not set.")
    encoded_credentials = base64.b64encode(jira_credentials.encode()).decode()

    url = 'https://bambooliulala.atlassian.net/rest/api/3/issue'

    headers = {
        'Authorization': f'Basic {encoded_credentials}',
        'Accept': 'application/json',
        'Content-Type': 'application/json'
    }

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
