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
func StartRecipeExecutor(c *gin.Context, config *Config, alertData *map[string]interface{}) {
	// Retrieve recipes from ConfigMap
	recipes, err := getRecipesFromConfigMap()
	if err != nil {
		logger.Error("Failed to retrieve recipes from ConfigMap", zap.Error(err))
		return
	}

	reconciler, err := NewAlertReconciler(c, config, alertData, recipes)
	if err != nil {
		logger.Error("Failed to create reconciler", zap.Error(err))
		return
	}

	// Create a Job for each recipe
	for recipeName, recipe := range recipes {
		_, err := createJob(recipeName, recipe, alertData, config)
		if err != nil {
			logger.Error("Failed to create K8s Job", zap.Error(err))
			// FIXME: Handle the error as needed
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
	recipeName string, recipe Recipe, alertData *map[string]interface{}, config *Config,
) (*batchv1.Job, error) {
	jobClient := clientset.BatchV1().Jobs(jobNamespace)

	// Define the Job object
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%v-", recipeName),
			Labels: map[string]string{
				"app":    "euphrosyne",
				"recipe": recipeName,
				"uuid":   (*alertData)["uuid"].(string),
			},
			Namespace: jobNamespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":    "euphrosyne",
						"recipe": recipeName,
						"uuid":   (*alertData)["uuid"].(string),
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
								buildRecipeCommand(recipe.Config, config, alertData),
							},
							// Add the environment variable from the Secret
                            // Env: []corev1.EnvVar{
							// 	{
							// 		Name: "JIRA_USER",
							// 		ValueFrom: &corev1.EnvVarSource{
							// 			SecretKeyRef: &corev1.SecretKeySelector{
							// 				LocalObjectReference: corev1.LocalObjectReference{
							// 					Name: "euphrosyne-keys",
							// 				},
							// 				Key: "jira-user",
							// 			},
							// 		},
							// 	},
							// 	{
							// 		Name: "JIRA_TOKEN",
							// 		ValueFrom: &corev1.EnvVarSource{
							// 			SecretKeyRef: &corev1.SecretKeySelector{
							// 				LocalObjectReference: corev1.LocalObjectReference{
							// 					Name: "euphrosyne-keys",
							// 				},
							// 				Key: "jira-token",
							// 			},
							// 		},
							// 	},
							// 	{
							// 		Name: "JIRA_URL",
							// 		ValueFrom: &corev1.EnvVarSource{
							// 			SecretKeyRef: &corev1.SecretKeySelector{
							// 				LocalObjectReference: corev1.LocalObjectReference{
							// 					Name: "euphrosyne-keys",
							// 				},
							// 				Key: "jira-url",
							// 			},
							// 		},
							// 	},
							// },
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

// Build Recipe command.
func buildRecipeCommand(
	recipeConfig *RecipeConfig, config *Config, alertData *map[string]interface{},
) string {
	alertDataStr, err := json.Marshal(alertData)
	if err != nil {
		logger.Error("Failed to convert alertData to string", zap.Error(err))
	}

	var recipeCommand string
	recipeCommand += fmt.Sprintf("%v ", recipeConfig.Entrypoint)
	recipeCommand += fmt.Sprintf("--aggregator-address '%v' ", config.AggregatorAddress)
	recipeCommand += fmt.Sprintf("--redis-address '%v' ", config.RedisAddress)
	for _, param := range recipeConfig.Params {
		recipeCommand += fmt.Sprintf("--%v '%v' ", param.Name, string(alertDataStr))
	}
	return recipeCommand
}
