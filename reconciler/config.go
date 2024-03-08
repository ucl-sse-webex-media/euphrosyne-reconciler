package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	AggregatorAddress = "localhost:8080"
	RedisAddress      = "localhost:6379"
	WebexBotAddress   = "localhost:7001"
	RecipeTimeout     = 300
)

// Rule represents a single rule from a Role or ClusterRole in Kubernetes RBAC.
type Rule struct {
	APIGroups []string
	Resources []string
	Verbs     []string
}

// Parse the Reconciler configuration from environment variables and command-line flags.
func ParseConfig(args []string) Config {
	// Set up Viper
	v := viper.New()
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.SetConfigName("config")
	v.AddConfigPath(".")
	v.SetConfigType("yaml")

	// Set default values
	v.SetDefault("aggregator-address", AggregatorAddress)
	v.SetDefault("redis-address", RedisAddress)
	v.SetDefault("webex-bot-address", WebexBotAddress)
	v.SetDefault("recipe-timeout", RecipeTimeout)

	v.AutomaticEnv()

	// Set up command-line flags
	fs := pflag.NewFlagSet("config", pflag.ContinueOnError)
	fs.String("aggregator-address", v.GetString("aggregator-address"), "Aggregator Address")
	fs.String("redis-address", v.GetString("redis-address"), "Redis Address")
	fs.String("webex-bot-address", v.GetString("webex-bot-address"), "Webex Bot Address")
	fs.Int("recipe-timeout", v.GetInt("recipe-timeout"), "Timeout (s) for recipe execution")
	fs.String("recipe-namespace", v.GetString("recipe-namespace"), "Namespace for recipes")
	fs.Parse(args)

	// Bind command-line flags to v keys
	v.BindPFlags(fs)

	config := Config{
		AggregatorAddress:   v.GetString("aggregator-address"),
		RedisAddress:        v.GetString("redis-address"),
		WebexBotAddress:     v.GetString("webex-bot-address"),
		RecipeTimeout:       v.GetInt("recipe-timeout"),
		RecipeNamespace:     v.GetString("recipe-namespace"),
		ReconcilerNamespace: getReconcilerNamespace(),
	}
	if config.ReconcilerNamespace == "default" {
		fmt.Println("Failed to retreive ReconcilerNamespace, using default namespace")
	}
	return config
}

func getReconcilerNamespace() string {
	// First, try to read from the Kubernetes service account namespace file
	ns, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err == nil {
		return string(ns)
	}

	// If reading the file fails, fallback to checking an environment variable
	envNamespace := os.Getenv("RECONCILER_NAMESPACE")
	if envNamespace != "" {
		return envNamespace
	}

	// If neither method works, fallback to a default namespace
	return "default"
}

func CheckNamespaceAccess(namespace string) bool {
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

	allowed, errMsg := checkAccessForRules(clientset, rules, namespace)
	if !allowed {
		logger.Error("Not all permissions are granted:", zap.String("error", errMsg))
		return false
	} else {
		logger.Info("All permissions are granted")
		return true
	}
}

// checkAccessForRules checks permissions for a list of rules in the specified namespace.
// Returns false and an error message if at least one of the conditions is not met.
func checkAccessForRules(clientset *kubernetes.Clientset, rules []Rule, namespace string) (bool, string) {
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

					response, err := clientset.AuthorizationV1().SelfSubjectAccessReviews().Create(context.TODO(), sar, metav1.CreateOptions{})
					if err != nil || !response.Status.Allowed {
						msg := fmt.Sprintf("Access to %v '%v' in namespace '%v' with verb '%v' DENIED or ERROR: %v", group, resource, namespace, verb, err)
						errorMessages = append(errorMessages, msg)
					}
				}
			}
		}
	}

	if len(errorMessages) > 0 {
		return false, strings.Join(errorMessages, "\n")
	}

	return true, ""
}
