package main

import (
	"context"
	"crypto/tls"
	"fmt"
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
)

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
	config, err := ParseConfig(os.Args[1:])
	if err != nil {
		panic(fmt.Sprintf("Failed to parse config: %s", err))
	}
	httpc = getHTTPClient()
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

	if err := CheckNamespaceAccess(clientset, config.RecipeNamespace); err != nil {
		panic(
			fmt.Sprintf(
				"The Reconciler doesn't have the necessary permissions in the specified recipe"+
					" namespace '%s': %s",
				config.RecipeNamespace, err,
			),
		)
	}

	go StartAlertHandler(&config)
	go StartServer(&config)

	<-shutdownChan
	logger.Info("Shutting down...")
	_ = logger.Sync()
}
