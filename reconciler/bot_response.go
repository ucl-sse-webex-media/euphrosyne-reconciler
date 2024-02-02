package main

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func StartBotResponseHandler(r *Reconciler) {
	router := gin.Default()
	router.POST("/botResponse", func(ctx *gin.Context) { handleBotResponse(ctx, r) })

	if err := router.Run(":8080"); err != nil {
		logger.Error("Failed to start server", zap.Error(err))
	}
}

func handleBotResponse(c *gin.Context, r *Reconciler) {
	var responseData map[string]interface{}

	if err := c.BindJSON(&responseData); err != nil {
		logger.Error("Failed to parse JSON", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Response received from the WebExBot"})
	logger.Info("Response received", zap.Any("response", responseData))
	if responseData["type"] == "recipe_status_request" {
		jobstatuses, err := r.getJobStatus()
		if err != nil {
			logger.Error("Error getting job status", zap.Error(err))
		}
		message, err := jobStatusesToStringSlice(jobstatuses)
		err = r.postMessageToWebexBot(message)
		if err != nil {
			logger.Error("Failed to forward message to Webex Bot", zap.Error(err))
		}
	}

}

func jobStatusesToStringSlice(jobStatuses []JobStatus) ([]string, error) {
	var stringSlice []string

	for _, jobStatus := range jobStatuses {
		// Convert each JobStatus instance to JSON string
		jsonString, err := json.Marshal(jobStatus)
		if err != nil {
			return nil, err
		}

		// Append the JSON string to the result slice
		stringSlice = append(stringSlice, string(jsonString))
	}

	return stringSlice, nil
}
