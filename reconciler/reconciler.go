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
	recipes   map[string]Recipe
}

// Initialise a reconciler for a specific alert.
func NewAlertReconciler(
	c *gin.Context, alertData *map[string]interface{}, recipes map[string]Recipe,
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
	completedRecipes, err := collectRecipeResult(r)
	if err != nil {
		logger.Error("Failed to collect recipe results", zap.Error(err))
	}

	// Send received messages to Webex Bot
	botMessage := IncidentBotMessage{
		UUID:     r.uuid,
		Analysis: r.getIncidentAnalysis(completedRecipes),
		Actions:  "",
	}
	err = r.postMessageToWebexBot(botMessage)
	if err != nil {
		logger.Error("Failed to forward message to Webex Bot", zap.Error(err))
		// FIXME: Handle the error as needed
	}
}

func collectRecipeResult(r *Reconciler) ([]Recipe, error) {
	var completedRecipes []Recipe
	defer func() {
		r.Cleanup(completedRecipes)
	}()
	ch := r.pubsub.Channel()

	messageCount := 0

	timeoutDuration := time.Duration(recipeTimeout) * time.Second
	timeout := time.NewTimer(timeoutDuration)
	shouldBreak := false

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
			// Update the Reconciler recipe with the execution results
			recipe.Config = r.recipes[recipe.Execution.Name].Config
			r.recipes[recipe.Execution.Name] = recipe

			completedRecipes = append(completedRecipes, recipe)
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

	err := r.pubsub.Close()
	if err != nil {
		logger.Error("Failed to close channel", zap.Error(err))
		return nil, err
	}

	return completedRecipes, nil
}

// Aggregate the results of all recipes.
func (r *Reconciler) getIncidentAnalysis(completedRecipes []Recipe) string {
	var incidentAnalysis string
	for _, recipe := range completedRecipes {
		if recipe.Execution.Status == "successful" {
			message := fmt.Sprintf(
				"Recipe '%s' completed successfully in response to incident '%s': %s",
				recipe.Execution.Name,
				recipe.Execution.Incident,
				recipe.Execution.Results.Analysis,
			)
			incidentAnalysis += message + " "
		}
	}
	return incidentAnalysis
}

// Parse recipe results from Redis message.
func (r *Reconciler) parseRecipeResults(message string) (Recipe, error) {
	var recipe Recipe
	err := json.Unmarshal([]byte(message), &recipe.Execution)
	if err != nil {
		return Recipe{}, err
	}
	return recipe, nil
}

// Post message to Webex Bot.
func (r *Reconciler) postMessageToWebexBot(message IncidentBotMessage) error {
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

// Cleanup at the end of the reconciler execution.
func (r *Reconciler) Cleanup(completedRecipes []Recipe) {
	logger.Info("Cleaning up created resources")

	// Delete the completed recipe Jobs
	labels := map[string]string{
		"app":  "euphrosyne",
		"uuid": r.uuid,
	}
	err := r.deleteCompletedJobsWithLabels(completedRecipes, labels)
	if err != nil {
		logger.Error("Failed to delete completed Jobs", zap.Error(err))
	}
}

// Delete completed Kubernetes Jobs with the specified labels.
func (r *Reconciler) deleteCompletedJobsWithLabels(
	completedRecipes []Recipe, labels map[string]string,
) error {
	jobClient := clientset.BatchV1().Jobs(jobNamespace)

	propagationPolicy := metav1.DeletePropagationBackground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	}

	for _, recipe := range completedRecipes {
		labels["recipe"] = recipe.Execution.Name
		labelSelector := metav1.FormatLabelSelector(&metav1.LabelSelector{MatchLabels: labels})

		logger.Info(
			"Deleting completed recipe Job with the following labels",
			zap.String("labelSelector", labelSelector),
		)
		err := jobClient.DeleteCollection(
			context.TODO(), deleteOptions, metav1.ListOptions{LabelSelector: labelSelector},
		)
		if err != nil {

			return err
		}
	}

	return nil
}
