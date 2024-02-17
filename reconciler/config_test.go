package main

import (
	"os"
	"testing"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func init() {
	viper.Reset()
	pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
}

func TestParseConfig(t *testing.T) {
	testCases := []struct {
		name     string
		envVars  map[string]string
		flagArgs []string
		expected Config
	}{
		{
			name:     "DefaultValues",
			envVars:  map[string]string{},
			flagArgs: []string{},
			expected: Config{
				AggregatorAddress: "localhost:8080",
				RedisAddress:      "localhost:6379",
				WebexBotAddress:   "localhost:7001",
				RecipeTimeout:     300,
			},
		},
		{
			name: "EnvironmentVariables",
			envVars: map[string]string{
				"AGGREGATOR_ADDRESS": "localhost:8081",
				"REDIS_ADDRESS":      "localhost:6380",
				"WEBEX_BOT_ADDRESS":  "localhost:7002",
				"RECIPE_TIMEOUT":     "400",
			},
			flagArgs: []string{},
			expected: Config{
				AggregatorAddress: "localhost:8081",
				RedisAddress:      "localhost:6380",
				WebexBotAddress:   "localhost:7002",
				RecipeTimeout:     400,
			},
		},
		{
			name:    "CommandLineArguments",
			envVars: map[string]string{},
			flagArgs: []string{
				"--aggregator-address=localhost:8082",
				"--redis-address=localhost:6381",
				"--webex-bot-address=localhost:7003",
				"--recipe-timeout=500",
			},
			expected: Config{
				AggregatorAddress: "localhost:8082",
				RedisAddress:      "localhost:6381",
				WebexBotAddress:   "localhost:7003",
				RecipeTimeout:     500,
			},
		}, {
			name: "CommandLineArgsOverrideEnvVars",
			envVars: map[string]string{
				"AGGREGATOR_ADDRESS": "localhost:8083",
				"REDIS_ADDRESS":      "localhost:6382",
				"WEBEX_BOT_ADDRESS":  "localhost:7004",
				"RECIPE_TIMEOUT":     "600",
			},
			flagArgs: []string{
				// Omit WebexBotAddress and RecipeTimeout to test partial overrides
				"--aggregator-address=localhost:8084",
				"--redis-address=localhost:6383",
			},
			expected: Config{
				AggregatorAddress: "localhost:8084", // Expect command-line argument value
				RedisAddress:      "localhost:6383", // Expect command-line argument value
				WebexBotAddress:   "localhost:7004", // Expect environment variable value
				RecipeTimeout:     600,              // Expect environment variable value
			},
		},
		{
			name: "MixedSources",
			envVars: map[string]string{
				// Omit WebexBotAddress to use default
				"AGGREGATOR_ADDRESS": "localhost:8085",
				"REDIS_ADDRESS":      "localhost:6384",
			},
			flagArgs: []string{
				"--redis-address=localhost:6385",
				"--webex-bot-address=localhost:7003",
			},
			expected: Config{
				AggregatorAddress: "localhost:8085", // Expect environment variable value
				RedisAddress:      "localhost:6385", // Expect command-line argument value
				WebexBotAddress:   "localhost:7003", // Expect command-line argument value
				RecipeTimeout:     300,              // Expect default value
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set up environment variables
			for key, value := range tc.envVars {
				os.Setenv(key, value)
			}

			actual := ParseConfig(tc.flagArgs)

			assert.Equal(t, tc.expected, actual)

			// Clean up environment variables
			for key := range tc.envVars {
				os.Unsetenv(key)
			}

			// Reset viper and pflag to clean state
			viper.Reset()
			pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
		})
	}
}
