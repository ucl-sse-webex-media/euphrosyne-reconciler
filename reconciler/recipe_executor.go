package main

import (
	"context"
	"encoding/json"
	"fmt"

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
	recipes, err := getRecipesFromConfigMap(requestType)
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

// Retrieve recipes from ConfigMap.
func getRecipesFromConfigMap(requestType RequestType) (map[string]Recipe, error) {
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
		recipeMap[recipeName] = Recipe{Config: &recipeConfig}
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

func createJobsForActions(
	recipes map[string]Recipe, data *map[string]interface{}, config *Config,
) error {
	var actions []Action
	var err error
	actions, err = parseActionData(data)
	logger.Info("ParsedActions", zap.Any("parsedActions", actions))
	if err != nil {
		logger.Error("Failed to parse response action data", zap.Error(err))
		return err
	}

	for _, action := range actions {
		_, ok := recipes[action.Action]
		if ok {
			wrappedData := map[string]interface{}{
				"data": action.Data,
				"uuid": (*data)["uuid"].(string),
			}
			_, err := createJob(action.Action, recipes[action.Action], &wrappedData, config)
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

	var recipeCommand string
	recipeCommand += fmt.Sprintf("%v ", recipeConfig.Entrypoint)
	recipeCommand += fmt.Sprintf("--aggregator-address '%v' ", config.AggregatorAddress)
	recipeCommand += fmt.Sprintf("--redis-address '%v' ", config.RedisAddress)
	for _, param := range recipeConfig.Params {
		recipeCommand += fmt.Sprintf("--%v '%v' ", param.Name, string(dataStr))
	}
	return recipeCommand
}

// Returns the list of actions from the message
func parseActionData(data *map[string]interface{}) ([]Action, error) {

	var actions []Action
	// Extract the "action" field from the provided data
	if actionData, ok := (*data)["action"]; ok {
		switch actionData := actionData.(type) {
		case map[string]interface{}:
			// If "action" is a single object, create a single Action
			action := Action{
				Action:      actionData["action"].(string),
				Description: actionData["description"].(string),
				Data:        (*data)["data"].(map[string]interface{}),
			}
			actions = append(actions, action)
		default:
			return nil, fmt.Errorf("error parsing parseActionData")
		}
	}

	return actions, nil
}
