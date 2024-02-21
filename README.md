# Euphrosyne Reconciler

The Euphrosyne Reconciler is responsible for listening for alerts raised by an alerting system and
orchestrating the process of investigating the detected incident, suggesting mitigation actions and
eventually executing them. The Reconciler is designed as a K8s-native operator that lives inside
the cluster and provides 2 interfaces, one internal to the K8s cluster and one external:
* `/webhook`: an internal interface for receiving alerts from the configured monitoring/alerting
  system
* `/api`: an external interface to expose parts of the internal state, as well as the supported
  actions. More specifically:
  * `/api/status`: provide details about the workloads responsible for debugging/mitigating an
    incident
  * `/api/actions`: execute actions based on the provided data

The basic unit of execution for the Reconciler is a **recipe**. A recipe is essentially a script,
carrying out predefined actions based on its input data. There are 2 types of recipes:
* **Debugging recipes**: series of steps for analysing an incident and suggesting a mitigation
  action. An example recipe could be receiving alert data as input, retrieving metrics from
  multiple sources, looking for patterns (e.g. failing nodes, user errors, networking issues), and
  suggesting mitigation actions (e.g. opening a Jira issue describing the incident, starting a
  Webex discussion, bringing a node offline)
* **Action recipes**: series of steps responsible for carrying out the actions suggested by the
  debugging recipes

Each recipe is executed as a K8s Job on the cluster. The Euphrosyne Reconciler is responsible for
submitting Jobs for execution, waiting for their completion, and aggregating their results. New
recipes can be registered through the configured K8s ConfigMap object.

A common workflow looks like this:
1. The Reconciler receives an alert from the alerting system signifying an incident
2. The Reconciler retrieves the registered debugging recipes dynamically from the configured
   ConfigMap
3. The Reconciler submits each one of the retrieved recipes as a separate K8s Job (i.e. the recipes
   run in parallel)
4. Each recipe goes through its predefined steps to debug the incident and logs its results upon
   completion
   * A recipe might be successful (managed to identify the problem) or not
5. The Reconciler collects the results from the completed recipes and aggregates them
6. The Reconciler sends the analysis and suggested mitigation actions as generated by the recipes
   to a configured Webex Bot
   * The Webex Bot is a possible way of interfacing with human operators
7. A human operator receives a message from the Webex Bot in their chat, inspects the analysis, and
   approves the suggested action(s)
8. The Webex Bot sends the approved action(s) back to the Reconciler through its `/api/actions`
   interface
9. The Reconciler retrieves the registered action recipes dynamically from the configured ConfigMap
10. The Reconciler submits the recipes that correspond to the requested action(s) to be executed as
    K8s Job(s) (i.e. the recipes run in parallel)
11. Each recipe goes through its predefined steps to carry out the intended action and logs its
    results upon completion
12. The Reconciler collects the results from the completed recipes and aggregates them
13. The Reconciler sends the outcome of the actions to the configured Webex Bot

It's worth noting that the collection of the recipe results is implemented using Redis, along with
a Pub/Sub model that allows the Reconciler to await the results of the submitted recipes.

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
