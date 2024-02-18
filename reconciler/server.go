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
	router.POST("/statusRequest", func(ctx *gin.Context) { handleStatusRequest(ctx, config) })
	router.POST("/actionResponse", func(ctx *gin.Context) { handleActionResponse(ctx, config) })
	if err := router.Run(":8081"); err != nil {
		logger.Error("Failed to start server", zap.Error(err))
	}
}

// handles request from WebEx bot for execution status of the recipe
func handleStatusRequest(c *gin.Context, config *Config) {

	var data map[string]interface{}

	if err := c.BindJSON(&data); err != nil {
		logger.Error("Failed to parse JSON", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON for Action response"})
		return
	}

	// Log the alert data
	logger.Info("Status Request received", zap.Any("request", data))

	// var requestType RequestType = StatusRequest
	// handleRequest(requestType, &requestData)
	var jobStatuses []JobStatus
	jobStatuses, err := getJobStatus(&data)
	if err != nil {
		logger.Error("Error Getting Job Status", zap.Error(err))
	}
	err = postStatusToWebexBot(jobStatuses, config.WebexBotAddress)
	if err != nil {
		logger.Error("Failed to send status to Bot", zap.Error(err))
	}

	c.JSON(http.StatusOK, gin.H{"message": "Status Request received and processed"})
}

// Gets the job status
func getJobStatus(message *map[string]interface{}) ([]JobStatus, error) {
	jobList, err := clientset.BatchV1().Jobs(jobNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logger.Error("Failed to get clienset form K8", zap.Error(err))
		return nil, err
	}

	uuid := (*message)["uuid"]
	fmt.Printf(uuid.(string))

	var allJobStatuses []JobStatus

	for _, job := range jobList.Items {
		var jobStatus JobStatus
		if value, ok := job.Labels["uuid"]; ok && value == uuid {
			jobStatus = JobStatus{
				Name:      job.Name,
				StartTime: job.CreationTimestamp.Time.Format(time.RFC3339),
				Labels:    job.Labels,
				//Get description of recipe from  from Configmap
				Description: "Hi! I am the description for the recipe",
			}
			if job.Status.Active > 0 {
				jobStatus.Status = "Active"
			} else if job.Status.Succeeded > 0 {
				jobStatus.Status = "Completed"
			} else if job.Status.Failed > 0 {
				jobStatus.Status = "Failed"
			}

		}

		allJobStatuses = append(allJobStatuses, jobStatus)
	}
	logger.Info("All Job Statuses", zap.Any("statuses", allJobStatuses))

	return allJobStatuses, nil
}

// Post status message to Webex Bot.
func postStatusToWebexBot(message []JobStatus, webexBotAddress string) error {
	// Convert the messages to JSON
	jsonData, err := json.Marshal(message)
	if err != nil {
		return err
	}

	// Send the POST request
	url := fmt.Sprintf("%s/api/actions", webexBotAddress)
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

//-------------------------------------------------------------

// handles response from WebEx Bot to execute actions
func handleActionResponse(c *gin.Context, config *Config) {

	var data map[string]interface{}

	if err := c.BindJSON(&data); err != nil {
		logger.Error("Failed to parse JSON", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON for Action response"})
		return
	}

	// Log the alert data
	logger.Info("Action response received", zap.Any("request", data))
	//Perform the action
	var requestType RequestType = Actions
	go StartRecipeExecutor(c, config, &data, requestType)

	c.JSON(http.StatusOK, gin.H{"message": "Response Request received and processed"})
}
