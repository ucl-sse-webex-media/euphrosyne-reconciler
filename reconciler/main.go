package main

import (
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

var (
	clientset *kubernetes.Clientset
	httpc     *http.Client
	rdb       *redis.Client
	logger    *zap.Logger

	webexBotAddress string
	recipeTimeout   int
)

func parseConfig() {
	flag.StringVar(
		&webexBotAddress,
		"webex-bot-address",
		os.Getenv("WEBEX_BOT_ADDRESS"),
		"HTTP address for the Webex Bot",
	)

	timeout, _ := strconv.Atoi(os.Getenv("RECIPE_TIMEOUT"))
	if timeout == 0 {
		timeout = 300
	}
	flag.IntVar(
		&recipeTimeout,
		"recipe-timeout",
		timeout,
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

func connectRedis(){
	rdb = redis.NewClient(&redis.Options{
		Addr:     "euphrosyne-reconciler-redis.default.svc.cluster.local:80",
		Password: "",
		DB:       0,
	})

}

func main() {
	parseConfig()
	httpc = getHTTPClient()

	initLogger()

	connectRedis()

	// Create a channel for graceful shutdown signal
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM)

	var err error
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
