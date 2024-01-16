import argparse
import json
import redis
import requests


WEBEX_BOT_ENDPOINT = "https://4863-144-82-8-42.ngrok-free.app/api/sendToRoom"


def get_headers():
    return {
        "Content-Type": "application/json",
    }


def get_data(data):
    return {
        "roomId": "Y2lzY29zcGFyazovL3VzL1JPT00vZmU2MjY4NzAtYjU0ZC0xMWViLWE2ZjEtMjUyYjUxZjY0ZjQ2",
        "message": data,
    }


def main():
    parser = argparse.ArgumentParser(description="A dummy recipe that logs input to stdout.")
    parser.add_argument("--data", type=str, help="Input data to be logged")

    args = parser.parse_args()

    if args.data:
        print("Received input:", args.data)
        alert = json.loads(args.data)
        redisClient = redis.Redis(host='redis-service', port=6379)
        redisClient.publish(alert["alertId"],"message 2")
        # response = requests.post(
        #     WEBEX_BOT_ENDPOINT, headers=get_headers(), data=get_data(args.data)
        # )
        # if response.status_code == 200:
        #     print("Data forwarded successfully to Webex Bot")
        # else:
        #     print("Failed to forward data to Webex Bot:", response.text)
    else:
        print("No input provided")


if __name__ == "__main__":
    main()
