package main

import (
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	AggregatorAddress = "localhost:8080"
	RedisAddress      = "localhost:6379"
	WebexBotAddress   = "localhost:7001"
	RecipeTimeout     = 300
)

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
	fs.Parse(args)

	// Bind command-line flags to v keys
	v.BindPFlags(fs)

	config := Config{
		AggregatorAddress: v.GetString("aggregator-address"),
		RedisAddress:      v.GetString("redis-address"),
		WebexBotAddress:   v.GetString("webex-bot-address"),
		RecipeTimeout:     v.GetInt("recipe-timeout"),
	}

	return config
}
