package main

import (
	"crypto/tls"
	"context"
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
	clientset *kubernetes.Clientset
	httpc     *http.Client
	rdb       *redis.Client
	logger    *zap.Logger
	redisAddress string = Redis_Address
	webexBotAddress string = Webex_Bot_Address
	recipeTimeout   int = Recipe_Timeout
)

func init() {
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
	logger.Sync()
}

func getHTTPClient() *http.Client {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	return &http.Client{Transport: tr}
}

func main() {
	httpc = getHTTPClient()

	initLogger()

	rdb = redis.NewClient(&redis.Options{
		Addr:     redisAddress,
		Password: "",
		DB:       0,
	})

	var err error
	_, err = rdb.Ping(context.Background()).Result()
    if err != nil {
		logger.Error("Failed to connect to redis", zap.Error(err))
        return
    }
	logger.Info("Redis connected successfully", zap.String("redisAddress",redisAddress))

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
	logger.Sync()
}
