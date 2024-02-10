import argparse

def parse_args():
    parser = argparse.ArgumentParser(description="A Euphrosyne Reconciler recipe.")
    parser.add_argument("--data", type=str, help="Aggregator data")
    parser.add_argument("--aggregator-base-url", type=str, help="Aggregator base url")
    parser.add_argument("--redis-address", type=str, help="Redis address")
    parsed_args = parser.parse_args()
    return parsed_args
