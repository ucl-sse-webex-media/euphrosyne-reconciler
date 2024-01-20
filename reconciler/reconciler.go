package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Reconciler struct {
	uuid      string
	alertData *map[string]interface{}
	pubsub    *redis.PubSub
	recipes   map[string]RecipeConfig
}

type Recipe struct {
	Status  string `json:"status"`
	Results string `json:"results"`
}

// Initialise a reconciler for a specific alert.
func NewAlertReconciler(
	c *gin.Context, alertData *map[string]interface{}, recipes map[string]RecipeConfig,
) (*Reconciler, error) {
	uuid := (*alertData)["uuid"].(string)

	// Subscribe to a new redis channel
	pubsub := rdb.Subscribe(c, uuid)

	_, err := pubsub.Receive(c)

	if err != nil {
		logger.Error("Failed to subscribe to channel", zap.Error(err))
		return nil, err
	}

	return &Reconciler{
		uuid:      uuid,
		alertData: alertData,
		pubsub:    pubsub,
		recipes:   recipes,
	}, nil
}

// Run the reconciler to monitor the subscribed Redis channel for the outcome of each recipe.
func (r *Reconciler) Run() {
	defer r.Cleanup()
	ch := r.pubsub.Channel()

	messageCount := 0

	timeoutDuration := time.Duration(recipeTimeout) * time.Second
	timeout := time.NewTimer(timeoutDuration)
	shouldBreak := false

	var receivedMessages []string

	for {
		select {
		case msg := <-ch:
			// Parse the recipe results from the Redis message
			recipe, err := r.parseRecipeResults(msg.Payload)
			if err != nil {
				logger.Error("Failed to parse recipe results", zap.Error(err))
			}
			logger.Info(
				"Received message from channel",
				zap.String("channel", msg.Channel),
				zap.Any("payload", recipe),
			)
			receivedMessages = append(receivedMessages, recipe.Results)
			messageCount++
			if messageCount == len(r.recipes) {
				shouldBreak = true
			}

		// Close channel after timeout to protect against recipes that end up in error state
		// Recipes might not complete if there are errors during runtime
		case <-timeout.C:
			shouldBreak = true
			logger.Warn(
				fmt.Sprintf(
					"Recipes failed to complete in %d seconds, closing channel", recipeTimeout,
				),
			)
		}

		if shouldBreak {
			break
		}
	}

	// Send received messages to Webex Bot
	err := r.postMessageToWebexBot(receivedMessages)
	if err != nil {
		logger.Error("Failed to forward message to Webex Bot", zap.Error(err))
		// FIXME: Handle the error as needed
	}

	err = r.pubsub.Close()
	if err != nil {
		logger.Error("Failed to close channel", zap.Error(err))
		return
	}
}

// Parse recipe results from Redis message.
func (r *Reconciler) parseRecipeResults(message string) (Recipe, error) {
	var recipe Recipe
	err := json.Unmarshal([]byte(message), &recipe)
	if err != nil {
		return Recipe{}, err
	}

	return recipe, nil
}

// Post message to Webex Bot.
func (r *Reconciler) postMessageToWebexBot(messages []string) error {
	// Convert the messages to JSON
	jsonData, err := json.Marshal(messages)
	if err != nil {
		return err
	}

	// Send the POST request
	time.Sleep(10 * time.Second)
	resp, err := http.Post(webexBotAddress, "application/json", bytes.NewBuffer(jsonData))
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

// Cleanup at the end of the reconciler execution.
func (r *Reconciler) Cleanup() {
	logger.Info("Cleaning up created resources")

	// Delete the completed recipe Jobs
	labels := map[string]string{
		"app":  "euphrosyne",
		"uuid": r.uuid,
	}
	err := r.deleteCompletedJobsWithLabels(labels)
	if err != nil {
		logger.Error("Failed to delete completed Jobs", zap.Error(err))
	}
}

// Delete completed Kubernetes Jobs with the specified labels.
func (r *Reconciler) deleteCompletedJobsWithLabels(labels map[string]string) error {
	jobClient := clientset.BatchV1().Jobs(jobNamespace)

	labelSelector := metav1.FormatLabelSelector(&metav1.LabelSelector{MatchLabels: labels})
	fieldSelector := "status.successful=1"

	logger.Info(
		"Deleting completed recipe Jobs with the following conditions",
		zap.String("labelSelector", labelSelector),
		zap.String("fieldSelector", fieldSelector),
	)

	propagationPolicy := metav1.DeletePropagationBackground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	}
	listOptions := metav1.ListOptions{
		LabelSelector: labelSelector,
		FieldSelector: fieldSelector,
	}
	err := jobClient.DeleteCollection(context.TODO(), deleteOptions, listOptions)
	if err != nil {
		return err
	}

	return nil
}