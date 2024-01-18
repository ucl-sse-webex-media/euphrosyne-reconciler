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

### Setting up Grafana

The Reconciler responds to alerts raised by an external system. Using Grafana for this purpose is
pretty straightforward. The easiest way to get started with Grafana is using the
[kube-prometheus](https://github.com/prometheus-operator/kube-prometheus) project. Installing this
configures Grafana along with some other services (e.g. Prometheus) and some Dashboards and Alerts
already in place. Following the instructions from the linked repository:

```bash
kubectl apply --server-side -f manifests/setup
kubectl wait \
	--for condition=Established \
	--all CustomResourceDefinition \
	--namespace=monitoring
kubectl apply -f manifests/
```

You can make Grafana reachable from your localhost using port-forwarding:

```bash
kubectl -n monitoring port-forward svc/grafana 3000;
```

Navigate to `localhost:3000` and to the Alerting panel. Select `Contact Points` and create a new
Contact Point of type `Webhook`.
In the address, provide the incluster address of the Euphrosyne Reconciler service:
```bash
http://euphrosyne-reconciler.default.svc.cluster.local/webhook
```

Click `Test` to test the connection. This will create a test alert that will go through our entire
workflow.
