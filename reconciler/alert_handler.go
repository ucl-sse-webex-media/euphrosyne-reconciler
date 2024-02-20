package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func StartAlertHandler(config *Config) {
	router := gin.Default()
	router.POST("/webhook", func(ctx *gin.Context) { handleWebhook(ctx, config) })

	if err := router.Run(":8080"); err != nil {
		logger.Error("Failed to start server", zap.Error(err))
	}
}

func handleWebhook(c *gin.Context, config *Config) {
	var alertData map[string]interface{}

	if err := c.BindJSON(&alertData); err != nil {
		logger.Error("Failed to parse JSON", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	// Log the alert data
	alertData["uuid"] = uuid.New().String()
	logger.Info("Alert received", zap.Any("alert", alertData))

	go StartRecipeExecutor(c, config, &alertData, Alert)

	c.JSON(http.StatusOK, gin.H{"message": "Alert received and processed"})
}
