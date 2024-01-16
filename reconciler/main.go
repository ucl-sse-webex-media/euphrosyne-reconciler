package main

import (
	"os"
	"os/signal"
	"syscall"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger *zap.Logger

func initLogger() {
	logger = createLogger()
	defer logger.Sync()
}

func createLogger() *zap.Logger {
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

	return zap.Must(config.Build())
}

func main() {
	rdb := redis.NewClient(&redis.Options{
        Addr: "localhost:6379", 
        Password: "", 
        DB:       0, 
    })

	initLogger()

	// Create a channel for graceful shutdown signal
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM)

	clientset, err := InitialiseKubernetesClient()
	if err != nil {
		logger.Error("Failed to initialise Kubernetes client", zap.Error(err))
		return
	}

	go StartAlertHandler(clientset, logger,rdb)

	<-shutdownChan
	logger.Info("Shutting down...")
	logger.Sync()
}
