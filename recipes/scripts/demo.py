import argparse
import json
import redis

def main():
    parser = argparse.ArgumentParser(description="A dummy recipe that logs input to stdout.")
    parser.add_argument("--data", type=str, help="Input data to be logged")
    args = parser.parse_args()
    alert = json.loads(args.data)
    redisClient = redis.Redis(host='redis-service', port=6379)
    redisClient.publish(alert["alertId"],"message 1")
    

if __name__ == "__main__":
    main()
