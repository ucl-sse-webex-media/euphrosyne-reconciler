package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap"
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
func ParseConfig(args []string) (Config, error) {
	// Set up Viper
	v := viper.New()
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.SetConfigName("config")
	v.AddConfigPath(".")
	v.SetConfigType("yaml")

	reconcilerNamespace, err := getReconcilerNamespace()
	if err != nil {
		logger.Error("Failed to determine Reconciler namespace", zap.Error(err))
		return Config{}, err
	}
	// Set default values
	v.SetDefault("aggregator-address", AggregatorAddress)
	v.SetDefault("redis-address", RedisAddress)
	v.SetDefault("webex-bot-address", WebexBotAddress)
	v.SetDefault("recipe-timeout", RecipeTimeout)
	v.SetDefault("recipe-namespace", reconcilerNamespace)

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
		ReconcilerNamespace: reconcilerNamespace,
	}
	return config, nil
}

// Get the namespace where the Reconciler is running.
func getReconcilerNamespace() (string, error) {
	// First, try to read from the Kubernetes service account namespace file
	ns, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err == nil {
		return string(ns), nil
	}

	// If reading the file fails, fallback to checking an environment variable
	envNamespace := os.Getenv("RECONCILER_NAMESPACE")
	if envNamespace != "" {
		return envNamespace, nil
	}

	return "", fmt.Errorf(
		"Failed to read Reconciler namespace from service account or environment variable",
	)
}
