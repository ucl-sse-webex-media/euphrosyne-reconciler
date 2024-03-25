from kubernetes import client, config
import requests, base64

# Load the in-cluster config, or use kubeconfig from local development environment
try:
    config.load_incluster_config()
except config.config_exception.ConfigException:
    config.load_kube_config()

def get_secret(namespace, secret_name):
    v1 = client.CoreV1Api()
    secret = v1.read_namespaced_secret(secret_name, namespace)
    data = {k: base64.b64decode(v).decode('utf-8') for k, v in secret.data.items()}
    return data

def refresh_token(url, client_id, client_secret, refresh_token):
    payload = {
        "grant_type": "refresh_token",
        "client_id": client_id,
        "client_secret": client_secret,
        "refresh_token": refresh_token,
    }
    response = requests.post(url, data=payload)
    return response.json()['access_token']

def update_secret(namespace, secret_name, access_token):
    v1 = client.CoreV1Api()
    body = client.V1Secret(
        metadata=client.V1ObjectMeta(name=secret_name),
        string_data={'webex-token': access_token}
    )
    v1.patch_namespaced_secret(secret_name, namespace, body)

def main():
    namespace = 'default'
    secret_name = 'euphrosyne-keys'
    auth_url = 'https://webexapis.com/v1/access_token'

    secret = get_secret(namespace, secret_name)

    refreshed_token = refresh_token(
        url=auth_url,
        client_id=secret['client-id'],
        client_secret=secret['client-secret'],
        refresh_token=secret['refresh-token']
    )

    update_secret(namespace, secret_name, refreshed_token)
    print("Token refreshed successfully.")

if __name__ == "__main__":
    main()