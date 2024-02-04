package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func StartAlertHandler() {
	router := gin.Default()
	router.POST("/webhook", func(ctx *gin.Context) { handleWebhook(ctx) })
	router.POST("/statusRequest", func(ctx *gin.Context) { handleStatusRequest(ctx) })
	router.POST("/actionResponse", func(ctx *gin.Context) { handleActionResponse(ctx) })
	if err := router.Run(":8080"); err != nil {
		logger.Error("Failed to start server", zap.Error(err))
	}
}

func handleWebhook(c *gin.Context) {
	var alertData map[string]interface{}

	if err := c.BindJSON(&alertData); err != nil {
		logger.Error("Failed to parse JSON", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	// Log the alert data
	alertData["uuid"] = uuid.New().String()
	logger.Info("Alert received", zap.Any("alert", alertData))

	// Start the recipe executor
	go StartRecipeExecutor(c, &alertData)

	c.JSON(http.StatusOK, gin.H{"message": "Alert received and processed"})
}

func handleStatusRequest(c *gin.Context) {

	var requestData map[string]interface{}

	if err := c.BindJSON(&requestData); err != nil {
		logger.Error("Failed to parse JSON", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON for Action response"})
		return
	}

	// Log the alert data
	logger.Info("Status Request received", zap.Any("request", requestData))

	// Start the recipe executor
	//Create a new reconciler that returns the status

	c.JSON(http.StatusOK, gin.H{"message": "Alert received and processed"})
}

func handleActionResponse(c *gin.Context) {

	var actionResponseData map[string]interface{}

	if err := c.BindJSON(&actionResponseData); err != nil {
		logger.Error("Failed to parse JSON", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON for Action response"})
		return
	}

	// Log the alert data
	logger.Info("Action response received", zap.Any("request", actionResponseData))

	// Start the recipe executor
	//Create a new reconciler that returns the runs a new reconciler to run the actions
}
