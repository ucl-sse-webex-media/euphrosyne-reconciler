package main

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// Convert a pointer to an int32.
func int32Ptr(i int32) *int32 { return &i }

// Return the path to the kubeconfig file.
func getKubeconfigPath() string {
	home := homedir.HomeDir()
	return fmt.Sprintf("%s/.kube/config", home)
}

// Initialise a Kubernetes client.
func InitialiseKubernetesClient() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		config, err = clientcmd.BuildConfigFromFlags("", getKubeconfigPath())
		if err != nil {
			logger.Error("Failed to build Kubernetes config", zap.Error(err))
			return nil, err
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Error("Failed to create Kubernetes client", zap.Error(err))
		return nil, err
	}

	return clientset, nil
}

// Check if the reconciler has the necessary permissions in the specified namespace.
func CheckNamespaceAccess(clientset *kubernetes.Clientset, namespace string) error {
	rules := []Rule{
		{
			APIGroups: []string{""},
			Resources: []string{"configmaps"},
			Verbs:     []string{"create", "deletecollection"},
		},
		{
			APIGroups: []string{"batch"},
			Resources: []string{"jobs"},
			Verbs:     []string{"get", "list", "create", "deletecollection"},
		},
	}

	err := checkAccessForRules(clientset, rules, namespace)
	if err != nil {
		logger.Error(
			"The Reconciler doesn't have the necessary permissions in the target namespace",
			zap.String("namespace", namespace),
			zap.Error(err),
		)
		return err
	}

	logger.Info("Access to the target namespace is granted", zap.String("namespace", namespace))
	return nil
}

// Check if the Reconciler has permissions for a list of rules in the specified namespace.
// Returns false and an error message if at least one of the conditions is not met.
func checkAccessForRules(clientset *kubernetes.Clientset, rules []Rule, namespace string) error {
	var errorMessages []string

	for _, rule := range rules {
		for _, group := range rule.APIGroups {
			for _, resource := range rule.Resources {
				for _, verb := range rule.Verbs {
					sar := &authorizationv1.SelfSubjectAccessReview{
						Spec: authorizationv1.SelfSubjectAccessReviewSpec{
							ResourceAttributes: &authorizationv1.ResourceAttributes{
								Namespace: namespace,
								Verb:      verb,
								Group:     group,
								Resource:  resource,
							},
						},
					}

					response, err := clientset.AuthorizationV1().SelfSubjectAccessReviews().Create(
						context.TODO(), sar, metav1.CreateOptions{},
					)
					if err != nil {
						errorMessages = append(
							errorMessages,
							fmt.Sprintf(
								"Permissions check for %s/%s/%s in namespace %s failed: %s",
								group, resource, verb, namespace, err,
							),
						)
					} else if !response.Status.Allowed {
						errorMessages = append(
							errorMessages,
							fmt.Sprintf(
								"The reconciler does not have the necessary permissions for"+
									" %s/%s/%s in namespace %s",
								group, resource, verb, namespace,
							),
						)
					}
				}
			}
		}
	}

	if len(errorMessages) > 0 {
		return fmt.Errorf(strings.Join(errorMessages, ", "))
	}

	return nil
}
