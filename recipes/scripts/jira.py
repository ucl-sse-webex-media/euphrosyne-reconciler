import requests
import json
import base64

email = 'bambooliulala@gmail.com'
token = 'ATATT3xFfGF0Ofo12ijF8Js4pmEnX7pDp-FcpEhIawOAqxgR7NmYOgGn1EYe_7G_-44bdEGLQ3RsLrgpq59jC0W0xqz0gCLOSpz4AmXLN_M3md_JptK-amGAjyz7rH7idqZIDZFZTUPKcaUlmnHOiG4CwpssqafHXNhEZJBA2995D3-8X-wohvc=7894AA6C'

credentials = f'{email}:{token}'
encoded_credentials = base64.b64encode(credentials.encode()).decode()

url = 'https://bambooliulala.atlassian.net/rest/api/3/issue'

headers = {
    'Authorization': f'Basic {encoded_credentials}',
    'Accept': 'application/json',
    'Content-Type': 'application/json'
}

# fileds: summary, issuetype, parent, description, customfield_10020, project, customfield_10021, 
# reporter, customfield_10000, labels, customfield_10016, customfield_10019, attachment, issuelinks, assignee
# TODO: get fields from other recipes
summary_text = "Summit 2019 is awesome!"
description_text = "This is the description."
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

print(response.text)
