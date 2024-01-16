package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
)

func StartAlertHandler(clientset *kubernetes.Clientset, logger *zap.Logger, rdb *redis.Client) {
	router := gin.Default()
	router.POST("/webhook", func(ctx *gin.Context) { handleWebhook(ctx, clientset, logger,rdb) })

	if err := router.Run(":8080"); err != nil {
		logger.Error("Failed to start server", zap.Error(err))
	}
}

func handleWebhook(c *gin.Context, clientset *kubernetes.Clientset, logger *zap.Logger,rdb *redis.Client) {
	var alertData map[string]interface{}

	if err := c.BindJSON(&alertData); err != nil {
		logger.Error("Failed to parse JSON", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	// Log the alert data
	logger.Info("Alert received", zap.Any("alert", alertData))

	// Start the recipe executor
	go StartRecipeExecutor(c, &alertData, clientset, logger,rdb)

	c.JSON(http.StatusOK, gin.H{"message": "Alert received and processed"})
}
