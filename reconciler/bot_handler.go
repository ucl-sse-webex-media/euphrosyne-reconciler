package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RequestType is an enum-like type for different types of requests
type RequestType int

const (
	// StatusRequest represents the status request type
	StatusRequest RequestType = iota + 1

	// ActionResponse represents the action response type
	ActionResponse
)

type JobStatus struct {
	Name      string            `json:"name"`
	StartTime string            `json:"startTime"`
	Status    string            `json:"status"`
	Labels    map[string]string `json:"labels"`
}

type StatusReconciler struct {
	uuid string
}

type ActionReconciler struct {
	uuid     string
	actions  string
	analysis string
}

// Handles the bot request based on type responseAction or statusRequest
func BotHandler(requestType RequestType, message *map[string]interface{}) {

	if requestType == StatusRequest {
		statusReconciler, err := newStatusReconciler(message)
		if err != nil {
			logger.Error("Failed to create request reconciler", zap.Error(err))
			return
		}
		allJobStatuses, err := statusReconciler.getJobStatus()
		if err != nil {
			logger.Error("Failed to send Job status ", zap.Error(err))
			return
		}
		err = statusReconciler.postStatusToWebexBot(allJobStatuses)
		if err != nil {
			logger.Error("Failed to send Job Status to WebexBot", zap.Error(err))
			return
		}
	} else if requestType == ActionResponse {
		reconciler, err := newActionReconciler(message)
		if err != nil {
			logger.Error("Failed to create action reconciler", zap.Error(err))
			return
		}
		err = reconciler.performActions()
		if err != nil {
			logger.Error("Failed to perform action on bot response", zap.Error(err))
			return
		}
	}

}

// Creates new status reconciler to handle statusRequest
func newStatusReconciler(requestData *map[string]interface{}) (*StatusReconciler, error) {
	//Returns a reconiler to handle bot request to show status of the recipes
	uuid := (*requestData)["uuid"].(string)
	return &StatusReconciler{
		uuid: uuid,
	}, nil
}

// Gets the job status
func (r *StatusReconciler) getJobStatus() ([]JobStatus, error) {
	jobClient, err := clientset.BatchV1().Jobs("default").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logger.Error("Failed to get clienset form K8", zap.Error(err))
		return nil, err
	}

	var allJobStatuses []JobStatus

	for _, job := range jobClient.Items {
		jobStatus := JobStatus{
			Name:      job.Name,
			StartTime: job.CreationTimestamp.Time.Format(time.RFC3339),
			Labels:    job.Labels,
		}

		if job.Status.Active > 0 {
			jobStatus.Status = "Active"
		} else if job.Status.Succeeded > 0 {
			jobStatus.Status = "Completed"
		} else if job.Status.Failed > 0 {
			jobStatus.Status = "Failed"
		}
		allJobStatuses = append(allJobStatuses, jobStatus)
	}
	logger.Info("All Job Statuses", zap.Any("statuses", allJobStatuses))

	return allJobStatuses, nil
}

// Post status message to Webex Bot.
func (r *StatusReconciler) postStatusToWebexBot(message []JobStatus) error {
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

// Creates new reconciler to handle responseAction
func newActionReconciler(responseData *map[string]interface{}) (*ActionReconciler, error) {
	uuid := (*responseData)["uuid"].(string)
	actions := (*responseData)["actions"].(string)
	analysis := (*responseData)["analysis"].(string)
	return &ActionReconciler{
		uuid:     uuid,
		actions:  actions,
		analysis: analysis,
	}, nil
}

// Runs jobs to perform action
func (r *ActionReconciler) performActions() error {
	logger.Info("Performing actions based on the response")
	return nil
}
