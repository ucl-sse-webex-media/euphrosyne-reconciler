package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
)

func StartAlertHandler(clientset *kubernetes.Clientset, logger *zap.Logger) {
	router := gin.Default()
	router.POST("/webhook", func(ctx *gin.Context) { handleWebhook(ctx, clientset, logger) })

	if err := router.Run(":8080"); err != nil {
		logger.Error("Failed to start server", zap.Error(err))
	}
}

func handleWebhook(c *gin.Context, clientset *kubernetes.Clientset, logger *zap.Logger) {
	var alertData map[string]interface{}

	if err := c.BindJSON(&alertData); err != nil {
		logger.Error("Failed to parse JSON", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	// Log the alert data
	logger.Info("Alert received", zap.Any("alert", alertData))

	// Start the recipe executor
	go StartRecipeExecutor(&alertData, clientset, logger)

	c.JSON(http.StatusOK, gin.H{"message": "Alert received and processed"})
}
