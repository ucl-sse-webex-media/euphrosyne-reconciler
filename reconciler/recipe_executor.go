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
func StartRecipeExecutor(c *gin.Context, data *map[string]interface{}, requestType RequestType) {
	// Retrieve recipes from ConfigMap
	recipes, err := getRecipesFromConfigMap()
	if err != nil {
		logger.Error("Failed to retrieve recipes from ConfigMap", zap.Error(err))
		return
	}

	reconciler, err := NewReconciler(c, data, recipes, requestType)
	if err != nil {
		logger.Error("Failed to create reconciler", zap.Error(err))
		return
	}

	if requestType == Actions {
		err = createJobsForActions(recipes, data)
		if err != nil {
			logger.Error("Failed to create jobs for Action", zap.Error(err))
			return
		}
	} else if requestType == Alert {
		err = createJobsForAlert(recipes, data)
		if err != nil {
			logger.Error("Failed to create jobs for Alert", zap.Error(err))
			return
		}
	}

	go reconciler.Run()

	logger.Info("Recipe execution started successfully")
}

// Retrieve recipes from ConfigMap.
func getRecipesFromConfigMap() (map[string]Recipe, error) {
	configMap, err := clientset.CoreV1().ConfigMaps(configMapNamespace).Get(
		context.TODO(), configMapName, metav1.GetOptions{},
	)
	if err != nil {
		return nil, err
	}

	recipes := make(map[string]Recipe)
	for key, value := range configMap.Data {
		// Parse the value as YAML into RecipeConfig
		var recipeConfig RecipeConfig
		err := yaml.Unmarshal([]byte(value), &recipeConfig)
		if err != nil {
			logger.Error("Failed to parse recipe configuration", zap.Error(err))
			// FIXME: Handle the error as needed
			continue
		}
		recipes[key] = Recipe{Config: &recipeConfig}
	}
	return recipes, nil
}

// Create a Kubernetes Job to execute a recipe.
func createJob(
	recipeName string, recipe Recipe, data *map[string]interface{},
) (*batchv1.Job, error) {
	jobClient := clientset.BatchV1().Jobs(jobNamespace)

	// Define the Job object
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%v-", recipeName),
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
								buildRecipeCommand(recipe.Config, data),
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

func createJobsForAlert(recipes map[string]Recipe, data *map[string]interface{}) error {
	// Create a Job for each recipe
	for recipeName, recipe := range recipes {
		_, err := createJob(recipeName, recipe, data)
		logger.Info("Created Job for recipe:" + recipeName)
		if err != nil {
			logger.Error("Failed to create K8s Job", zap.Error(err))
			// FIXME: Handle the error as needed
		}
	}
	return nil
}

func createJobsForActions(recipes map[string]Recipe, data *map[string]interface{}) error {
	var actions []Action
	var err error
	actions, err = parseActionData(data)
	if err != nil {
		logger.Error("Failed to parse response action data", zap.Error(err))
		return err
	}
	// Create a map for quick recipe name lookup
	recipeNames := make(map[string]struct{}, len(recipes))
	for recipeName := range recipes {
		recipeNames[strings.ToLower(recipeName)] = struct{}{}
	}
	// Iterate over actions and create jobs for matching recipes
	for _, action := range actions {
		actionName := strings.ToLower(action.Action)
		_, ok := recipeNames[actionName]
		if ok {
			_, err := createJob(action.Action, recipes[action.Action], data)
			if err != nil {
				logger.Error("Failed to create K8s Job", zap.Error(err))
				// FIXME: Handle the error as needed
			}
		}
	}
	return nil
}

// Build Recipe command.
func buildRecipeCommand(recipeConfig *RecipeConfig, data *map[string]interface{}) string {
	dataStr, err := json.Marshal(data)
	if err != nil {
		logger.Error("Failed to convert alertData to string", zap.Error(err))
	}

	var recipeCommand string
	recipeCommand += fmt.Sprintf("%v ", recipeConfig.Entrypoint)
	for _, param := range recipeConfig.Params {
		recipeCommand += fmt.Sprintf("--%v '%v'", param.Name, string(dataStr))
	}
	return recipeCommand
}

// Returns the list of actions from the message
func parseActionData(data *map[string]interface{}) ([]Action, error) {

	var actions []Action
	// Check if "actions" field exists in data
	if actionsData, ok := (*data)["actions"]; ok {
		switch actionsData := actionsData.(type) {
		case map[string]interface{}:
			// If "actions" is a single object, create a single Action
			action := Action{
				Action:      actionsData["action"].(string),
				Description: actionsData["description"].(string),
			}
			actions = append(actions, action)
		case []interface{}:
			// If "actions" is an array of objects, iterate through each object and create Actions
			for _, actionData := range actionsData {
				action := Action{
					Action:      actionData.(map[string]interface{})["action"].(string),
					Description: actionData.(map[string]interface{})["description"].(string),
				}
				actions = append(actions, action)
			}
		default:
			return nil, fmt.Errorf("error parsing parseActionData")
		}
	}

	return actions, nil
}
