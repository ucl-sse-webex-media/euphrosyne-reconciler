package main

import (
	"context"
	"crypto/tls"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/client-go/kubernetes"
)

type Config struct {
	AggregatorAddress string
	RedisAddress      string
	WebexBotAddress   string
	RecipeTimeout     int
}

var (
	clientset *kubernetes.Clientset
	httpc     *http.Client
	rdb       *redis.Client
	logger    *zap.Logger
	config    Config = Config{
		AggregatorAddress: AggregatorAddress,
		RedisAddress:      RedisAddress,
		WebexBotAddress:   WebexBotAddress,
		RecipeTimeout:     RecipeTimeout,
	}
)

func parseConfig(config *Config) {
	if os.Getenv("AGGREGATOR_ADDRESS") != "" {
		config.AggregatorAddress = os.Getenv("AGGREGATOR_ADDRESS")
	}

	if os.Getenv("REDIS_ADDRESS") != "" {
		config.RedisAddress = os.Getenv("REDIS_ADDRESS")
	}

	if os.Getenv("WEBEX_BOT_ADDRESS") != "" {
		config.WebexBotAddress = os.Getenv("WEBEX_BOT_ADDRESS")
	}

	if os.Getenv("RECIPE_TIMEOUT") != "" {
		config.RecipeTimeout, _ = strconv.Atoi(os.Getenv("RECIPE_TIMEOUT"))
	}

	flag.StringVar(
		&config.AggregatorAddress,
		"aggregator-address",
		config.AggregatorAddress,
		"Aggregator Address",
	)

	flag.StringVar(
		&config.RedisAddress,
		"redis-address",
		config.RedisAddress,
		"Redis Address",
	)

	flag.StringVar(
		&config.WebexBotAddress,
		"webex-bot-address",
		config.WebexBotAddress,
		"HTTP address for the Webex Bot",
	)

	flag.IntVar(
		&config.RecipeTimeout,
		"recipe-timeout",
		config.RecipeTimeout,
		"Timeout in seconds for recipe execution",
	)

	flag.Parse()
}

func initLogger() {
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "timestamp"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	config := zap.Config{
		Level:             zap.NewAtomicLevelAt(zap.InfoLevel),
		Development:       false,
		DisableCaller:     false,
		DisableStacktrace: false,
		Sampling:          nil,
		Encoding:          "console",
		EncoderConfig:     encoderCfg,
		OutputPaths: []string{
			"stderr",
		},
		ErrorOutputPaths: []string{
			"stderr",
		},
		InitialFields: map[string]interface{}{
			"pid": os.Getpid(),
		},
	}

	logger = zap.Must(config.Build())
	_ = logger.Sync()
}

func getHTTPClient() *http.Client {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	return &http.Client{Transport: tr}
}

func connectRedis(config *Config) {
	rdb = redis.NewClient(&redis.Options{
		Addr:     config.RedisAddress,
		Password: "",
		DB:       0,
	})
	_, err := rdb.Ping(context.Background()).Result()
	if err != nil {
		panic(err)
	}
	logger.Info("Redis connected successfully", zap.String("redisAddress", config.RedisAddress))
}

func main() {
	parseConfig(&config)
	httpc = getHTTPClient()

	var err error
	initLogger()

	connectRedis(&config)

	// Create a channel for graceful shutdown signal
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM)

	clientset, err = InitialiseKubernetesClient()
	if err != nil {
		logger.Error("Failed to initialise Kubernetes client", zap.Error(err))
		return
	}

	go StartAlertHandler(&config)

	<-shutdownChan
	logger.Info("Shutting down...")
	_ = logger.Sync()
}
