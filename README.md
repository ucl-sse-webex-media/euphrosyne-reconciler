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

### Setting up Grafana

The Reconciler responds to alerts raised by an external system. Using Grafana for this purpose is
pretty straightforward. The easiest way to get started with Grafana is using the
[kube-prometheus](https://github.com/prometheus-operator/kube-prometheus) project. Installing this
configures Grafana along with some other services (e.g. Prometheus) and some Dashboards and Alerts
already in place. In order for the installation to work seamlessly with our system, we need to
make some slight modifications to the manifests before applying them. More specifically we need to
expose Grafana so that the Aggregator can communicate with it. By default, Grafana is only
accessible by Prometheus:

1. Clone the `kube-prometheus` project locally:

    ```bash
    git clone git@github.com:prometheus-operator/kube-prometheus.git
    cd kube-prometheus
    ```

2. Edit the Grafana service port, so that we don't need to specify it when executing HTTP requests
   using the Service FQDN (i.e. http://grafana.monitoring.svc.cluster.local). More specifically,
   edit [`manifests/grafana-service.yaml`](https://github.com/prometheus-operator/kube-prometheus/blob/main/manifests/grafana-service.yaml) to set the `port` to 80:

    ```yaml
    # replace the following
      ports:
      - name: http
          port: 3000
        targetPort: http
    # with
      ports:
      - name: http
          port: 80
        targetPort: http
    ```

3. Edit the Grafana network policy so that ingress traffic from any application is allowed. By
   default, only traffic coming from Prometheus is permitted. More specifically, edit
   [`manifests/grafana-networkPolicy`](https://github.com/prometheus-operator/kube-prometheus/blob/main/manifests/grafana-networkPolicy.yaml) to allow *all* ingress traffic (in a production setting we would
   keep this more fine-grained):

    ```yaml
    # replace the following
      ingress:
      - from:
        - podSelector:
            matchLabels:
            app.kubernetes.io/name: prometheus
        ports:
        - port: 3000
        protocol: TCP
    # with
      ingress:
      - {}
    ```

4. Finally, apply the manifests, as shown in the upstream guide:

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
kubectl -n monitoring port-forward svc/grafana 3000:80
```

Navigate to `localhost:3000` and to the Alerting panel. Select `Contact Points` and create a new
Contact Point of type `Webhook`.
In the address, provide the incluster address of the Euphrosyne Reconciler service:
```bash
http://euphrosyne-reconciler.default.svc.cluster.local/webhook
```

Click `Test` to test the connection. This will create a test alert that will go through our entire
workflow.
