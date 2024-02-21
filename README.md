# Euphrosyne Reconciler

## Setup

In order to setup the Euphrosyne Reconciler you will need a working Kubernetes cluster and
`kubectl` configured to communicate with the API Server. An easy way to get started is `microk8s`.

To apply the Kubernetes manifests responsible for setting up the Reconciler on Kubernetes, run the
following (recursively applying all YAML files inside the `manifests` directory):

```bash
kubectl apply -f reconciler/manifests -R
```

You will also need to apply the ConfigMap containing the list of available recipes:

```bash
kubectl apply -f recipes/kubernetes/orpheus-operator-recipes.yaml
```

In order for the Euphrosyne Reconciler to be able to interact with external services, we load the
corresponding credentials from Kubernetes secrets. Please run the following command, providing your
own credentials for accessing Jira:

```bash
kubectl create secret generic euphrosyne-keys \
  --from-literal=jira-url=<your Jira server URL> \
  --from-literal=jira-user=<your Jira username> \
  --from-literal=jira-token=<your Jira token>
```
