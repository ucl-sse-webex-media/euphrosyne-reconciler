package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type JobStatus struct {
	Name        string            `json:"name"`
	StartTime   string            `json:"startTime"`
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels"`
	Description string            `json:"description"`
}

func StartServer(config *Config) {
	router := gin.Default()
	router.POST("/api/status", func(ctx *gin.Context) { handleStatusRequest(ctx, config) })
	router.POST("/api/actions", func(ctx *gin.Context) { handleActionsRequest(ctx, config) })
	if err := router.Run(":8081"); err != nil {
		logger.Error("Failed to start server", zap.Error(err))
	}
}

// Handle request from Webex Bot for execution status of the submitted recipes.
func handleStatusRequest(c *gin.Context, config *Config) {

	var data map[string]interface{}

	if err := c.BindJSON(&data); err != nil {
		logger.Error("Failed to parse JSON", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON for Action response"})
		return
	}

	// Log the alert data
	logger.Info("Status Request received", zap.Any("request", data))

	var jobStatuses []JobStatus
	jobStatuses, err := getJobStatus(&data, config.RecipeNamespace)
	if err != nil {
		logger.Error("Error Getting Job Status", zap.Error(err))
	}
	err = postStatusToWebexBot(jobStatuses, config.WebexBotAddress)
	if err != nil {
		logger.Error("Failed to send status to Bot", zap.Error(err))
	}

	c.JSON(http.StatusOK, gin.H{"message": "Status Request received and processed"})
}

// Get the list of Job statuses for a specific UUID.
func getJobStatus(message *map[string]interface{}, namespace string) ([]JobStatus, error) {
	listOptions := metav1.ListOptions{
		LabelSelector: "app=euphrosyne",
	}
	if _, ok := (*message)["uuid"]; ok {
		listOptions.LabelSelector += fmt.Sprintf(",uuid=%s", (*message)["uuid"])
	}
	jobList, err := clientset.BatchV1().Jobs(namespace).List(context.TODO(), listOptions)
	if err != nil {
		logger.Error("Failed to list K8s Jobs", zap.Error(err))
		return nil, err
	}

	jobStatuses := []JobStatus{}

	for _, job := range jobList.Items {
		jobStatus := JobStatus{
			Name:        job.Name,
			StartTime:   job.CreationTimestamp.Time.Format(time.RFC3339),
			Labels:      job.Labels,
			Description: job.Annotations["description"],
		}
		if job.Status.Active > 0 {
			jobStatus.Status = "Active"
		} else if job.Status.Succeeded > 0 {
			jobStatus.Status = "Completed"
		} else if job.Status.Failed > 0 {
			jobStatus.Status = "Failed"
		}

		jobStatuses = append(jobStatuses, jobStatus)
	}
	logger.Info("Euphrosyne Reconciler Jobs", zap.Any("jobs", jobStatuses))

	return jobStatuses, nil
}

// Post status message to Webex Bot.
func postStatusToWebexBot(message []JobStatus, webexBotAddress string) error {
	// Convert the messages to JSON
	jsonData, err := json.Marshal(message)
	if err != nil {
		return err
	}

	// Send the POST request
	url := fmt.Sprintf("%s/api/analysis", webexBotAddress)
	resp, err := httpc.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check the response status code
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Unexpected response status: %s", resp.Status)
	}

	return nil

}

// Handle request from Webex Bot to execute actions.
func handleActionsRequest(c *gin.Context, config *Config) {

	var data map[string]interface{}

	if err := c.BindJSON(&data); err != nil {
		logger.Error("Failed to parse JSON", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON for Action response"})
		return
	}

	logger.Info("Action response received", zap.Any("request", data))
	go StartRecipeExecutor(c, config, &data, Actions)

	c.JSON(http.StatusOK, gin.H{"message": "Response Request received and processed"})
}
