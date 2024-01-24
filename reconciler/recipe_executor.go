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

const (
	configMapNamespace = "default"
	configMapName      = "orpheus-operator-recipes"
	jobNamespace       = "default"
)

var recipeProdConfig = map[string]string{
	"aggregator-base-url":"http://thalia-aggregator.default.svc.cluster.local",
	"redis-address" :"euphrosyne-reconciler-redis:80",
}
type RecipeConfig struct {
	Image      string `yaml:"image"`
	Entrypoint string `yaml:"entrypoint"`
	Params     []struct {
		Name  string `yaml:"name"`
		Value string `yaml:"value"`
	} `yaml:"params"`
}

// Initialise and run the recipe executor.
func StartRecipeExecutor(c *gin.Context, alertData *map[string]interface{}) {
	// retrieve recipes from ConfigMap
	recipes, err := getRecipesFromConfigMap()
	if err != nil {
		logger.Error("Failed to retrieve recipes from ConfigMap", zap.Error(err))
		return
	}

	reconciler, err := NewAlertReconciler(c, alertData, recipes)
	if err != nil {
		logger.Error("Failed to create reconciler", zap.Error(err))
		return
	}

	// Create a Job for each recipe
	for recipeName, recipeConfig := range recipes {
		_, err := createJob(recipeName, recipeConfig, alertData)
		if err != nil {
			logger.Error("Failed to create K8s Job", zap.Error(err))
			// FIXME: Handle the error as needed
		}
	}

	go reconciler.Run()

	logger.Info("Recipe execution started successfully")
}

// Retrieve recipes from ConfigMap.
func getRecipesFromConfigMap() (map[string]RecipeConfig, error) {
	configMap, err := clientset.CoreV1().ConfigMaps(configMapNamespace).Get(
		context.TODO(), configMapName, metav1.GetOptions{},
	)
	if err != nil {
		return nil, err
	}

	recipes := make(map[string]RecipeConfig)
	for key, value := range configMap.Data {
		// Parse the value as YAML into RecipeConfig
		var recipeConfig RecipeConfig
		err := yaml.Unmarshal([]byte(value), &recipeConfig)
		if err != nil {
			logger.Error("Failed to parse recipe configuration", zap.Error(err))
			// FIXME: Handle the error as needed
			continue
		}

		recipes[key] = recipeConfig
	}

	return recipes, nil
}

// Create a Kubernetes Job to execute a recipe.
func createJob(recipeName string, recipeConfig RecipeConfig, alertData *map[string]interface{}) (*batchv1.Job, error) {
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
							Image: recipeConfig.Image,
							Command: []string{
								"/bin/sh",
								"-c",
								buildRecipeCommand(recipeConfig, alertData),
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

// Build Recipe command.
func buildRecipeCommand(recipeConfig RecipeConfig, alertData *map[string]interface{}) string {
	alertDataStr, err := json.Marshal(alertData)
	if err != nil {
		logger.Error("Failed to convert alertData to string", zap.Error(err))
	}

	var recipeCommand string
	recipeCommand += fmt.Sprintf("%v ", recipeConfig.Entrypoint)
	for _, param := range recipeConfig.Params {
		recipeCommand += fmt.Sprintf("--%v '%v' ", param.Name, string(alertDataStr))
	}

	for name, value := range recipeProdConfig {
		recipeCommand += fmt.Sprintf("--%v '%v' ", name, value)
	}
	fmt.Println(recipeCommand)
	return recipeCommand
}
