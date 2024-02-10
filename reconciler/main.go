package main

import (
	"context"
	"crypto/tls"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/client-go/kubernetes"
)

var (
	clientset       *kubernetes.Clientset
	httpc           *http.Client
	rdb             *redis.Client
	logger          *zap.Logger
	redisAddress    string = RedisAddress
	webexBotAddress string = WebexBotAddress
	recipeTimeout   int    = RecipeTimeout
)

func parseConfig() {
	// config priority: config.go < env variables < command line
	flag.StringVar(
		&redisAddress,
		"redis-address",
		redisAddress,
		"Redis Address",
	)

	flag.StringVar(
		&webexBotAddress,
		"webex-bot-address",
		webexBotAddress,
		"HTTP address for the Webex Bot",
	)

	flag.IntVar(
		&recipeTimeout,
		"recipe-timeout",
		recipeTimeout,
		"Timeout in seconds for recipe execution",
	)

	loadEnvVariables()

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

func connectRedis() {
	rdb = redis.NewClient(&redis.Options{
		Addr:     redisAddress,
		Password: "",
		DB:       0,
	})
	_, err := rdb.Ping(context.Background()).Result()
	if err != nil {
		panic(err)
	}
	logger.Info("Redis connected successfully", zap.String("redisAddress", redisAddress))
}

func main() {
	parseConfig()
	httpc = getHTTPClient()

	var err error
	_, err = rdb.Ping(context.Background()).Result()
	if err != nil {
		logger.Error("Failed to connect to redis", zap.Error(err))
		return
	}
	logger.Info("Redis connected successfully", zap.String("redisAddress", redisAddress))

	connectRedis()

	// Create a channel for graceful shutdown signal
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM)

	clientset, err = InitialiseKubernetesClient()
	if err != nil {
		logger.Error("Failed to initialise Kubernetes client", zap.Error(err))
		return
	}

	go StartAlertHandler()

	<-shutdownChan
	logger.Info("Shutting down...")
	_ = logger.Sync()
}
