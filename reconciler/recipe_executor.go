package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

var (
	configMapNamespace = "default"
	configMapName      = "orpheus-operator-recipes"
	jobNamespace       = "default"
)

// Initialise and run the recipe executor.
func StartRecipeExecutor(
	c *gin.Context, config *Config, data *map[string]interface{}, requestType RequestType,
) {
	// Retrieve recipes from ConfigMap
	recipes, err := getRecipesFromConfigMap(requestType, true)
	if err != nil {
		logger.Error("Failed to retrieve recipes from ConfigMap", zap.Error(err))
		return
	}
	logger.Info("Retrieved recipes from ConfigMap", zap.Any("recipes", recipes))

	reconciler, err := NewReconciler(c, config, data, recipes, requestType)
	if err != nil {
		logger.Error("Failed to create reconciler", zap.Error(err))
		return
	}

	if requestType == Actions {
		err = createJobsForActions(recipes, data, config)
		if err != nil {
			logger.Error("Failed to create jobs for Action", zap.Error(err))
			return
		}
	} else if requestType == Alert {
		err = createJobsForAlert(recipes, data, config)
		if err != nil {
			logger.Error("Failed to create jobs for Alert", zap.Error(err))
			return
		}
	}

	go reconciler.Run()

	logger.Info("Recipe execution started successfully")
}

// Retrieve recipes from ConfigMap, optionally filtering by enabled status.
func getRecipesFromConfigMap(
	requestType RequestType, filterEnabled bool,
) (map[string]Recipe, error) {
	configMap, err := clientset.CoreV1().ConfigMaps(configMapNamespace).Get(
		context.TODO(), configMapName, metav1.GetOptions{},
	)
	if err != nil {
		return nil, err
	}

	var recipeConfigMap map[string]RecipeConfig
	if requestType == Actions {
		err = yaml.Unmarshal([]byte(configMap.Data["actions"]), &recipeConfigMap)
	} else {
		err = yaml.Unmarshal([]byte(configMap.Data["debugging"]), &recipeConfigMap)
	}
	if err != nil {
		return nil, err
	}

	recipeMap := make(map[string]Recipe)
	for recipeName, recipeConfig := range recipeConfigMap {
		recipeConfigCopy := recipeConfig
		if recipeConfigCopy.Enabled || !filterEnabled {
			recipeMap[recipeName] = Recipe{Config: &recipeConfigCopy}
		}
	}

	return recipeMap, nil
}

// Create a Kubernetes Job to execute a recipe.
func createJob(
	recipeName string, recipe Recipe, data *map[string]interface{}, config *Config,
) (*batchv1.Job, error) {
	jobClient := clientset.BatchV1().Jobs(jobNamespace)

	// Define the Job object
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%v-", recipeName),
			Annotations: map[string]string{
				"description": recipe.Config.Description,
			},
			Labels: map[string]string{
				"app":    "euphrosyne",
				"recipe": recipeName,
				"uuid":   (*data)["uuid"].(string),
			},
			Namespace: jobNamespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":    "euphrosyne",
						"recipe": recipeName,
						"uuid":   (*data)["uuid"].(string),
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "recipe-container",
							Image: recipe.Config.Image,
							Command: []string{
								"/bin/sh",
								"-c",
								buildRecipeCommand(recipe.Config, config, data),
							},
							Env: []corev1.EnvVar{
								{
									Name: "JIRA_URL",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "euphrosyne-keys",
											},
											Key: "jira-url",
										},
									},
								},
								{
									Name: "JIRA_USER",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "euphrosyne-keys",
											},
											Key: "jira-user",
										},
									},
								},
								{
									Name: "JIRA_TOKEN",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "euphrosyne-keys",
											},
											Key: "jira-token",
										},
									},
								},
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
			BackoffLimit: int32Ptr(0),
		},
	}

	job, err := jobClient.Create(context.TODO(), job, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	logger.Info("Job created successfully", zap.String("jobName", job.Name))

	return job, nil
}

// Create Jobs to execute a list of recipes.
func createJobsForAlert(
	recipes map[string]Recipe, data *map[string]interface{}, config *Config,
) error {
	// Create a Job for each recipe
	for recipeName, recipe := range recipes {
		_, err := createJob(recipeName, recipe, data, config)
		if err != nil {
			logger.Error("Failed to create K8s Job", zap.Error(err))
			// FIXME: Handle the error as needed
		}
	}
	return nil
}

// Create Jobs for the actions in the Webex Bot request.
func createJobsForActions(
	recipes map[string]Recipe, data *map[string]interface{}, config *Config,
) error {
	var actions []Action
	var err error
	actions, err = parseActionData(data)
	if err != nil {
		logger.Error("Failed to parse actions", zap.Error(err))
		return err
	}

	for _, action := range actions {
		_, ok := recipes[action.Name]
		if ok {
			wrappedData := map[string]interface{}{
				"data": action.Data,
				"uuid": (*data)["uuid"].(string),
			}
			_, err := createJob(action.Name, recipes[action.Name], &wrappedData, config)
			if err != nil {
				logger.Error("Failed to create K8s Job", zap.Error(err))
				// FIXME: Handle the error as needed
			}
		}
	}
	return nil
}

// Build Recipe command.
func buildRecipeCommand(
	recipeConfig *RecipeConfig, config *Config, data *map[string]interface{},
) string {
	dataStr, err := json.Marshal(data)
	if err != nil {
		logger.Error("Failed to convert input data to string", zap.Error(err))
	}

	// Escape the quotes inside the JSON string
	escapedDataStr := strings.ReplaceAll(string(dataStr), `"`, `\"`)

	var recipeCommand string
	recipeCommand += fmt.Sprintf("%v ", recipeConfig.Entrypoint)
	recipeCommand += fmt.Sprintf("--aggregator-address '%v' ", config.AggregatorAddress)
	recipeCommand += fmt.Sprintf("--redis-address '%v' ", config.RedisAddress)
	recipeCommand += fmt.Sprintf("--data \"%v\" ", escapedDataStr)
	return recipeCommand
}

// Parse the action data from the Webex Bot request.
func parseActionData(data *map[string]interface{}) ([]Action, error) {
	var actions []Action
	if actionData, ok := (*data)["actions"]; ok {
		for _, item := range actionData.([]interface{}) {
			actionMap, ok := item.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("Expected Action to be a valid JSON object")
			}

			var action Action
			action.Name, ok = actionMap["name"].(string)
			if !ok {
				return nil, fmt.Errorf("Action 'name' field is either missing or not a string")
			}

			data, ok := actionMap["data"].(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf(
					"Action 'data' field is either missing or not valid JSON object",
				)
			}
			action.Data = data

			actions = append(actions, action)
		}
	}
	return actions, nil
}
