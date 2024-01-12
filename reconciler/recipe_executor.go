package main

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"

	"go.uber.org/zap"
)

const (
	configMapNamespace = "default"
	configMapName      = "orpheus-operator-recipes"
	jobNamespace       = "default"
)

type RecipeConfig struct {
	Image      string `yaml:"image"`
	Entrypoint string `yaml:"entrypoint"`
	Params     []struct {
		Name  string `yaml:"name"`
		Value string `yaml:"value"`
	} `yaml:"params"`
}

// Initialise and run the recipe executor.
func StartRecipeExecutor(alertData *map[string]interface{}, clientset *kubernetes.Clientset, logger *zap.Logger) {

	// Retrieve recipes from ConfigMap
	recipes, err := getRecipesFromConfigMap(clientset)
	if err != nil {
		logger.Error("Failed to retrieve recipes from ConfigMap", zap.Error(err))
		return
	}

	// Create a Job for each recipe
	for recipeName, recipeConfig := range recipes {
		err := createJob(clientset, recipeName, recipeConfig, alertData)
		if err != nil {
			logger.Error("Failed to create Job", zap.Error(err))
			// FIXME: Handle the error as needed
		}
	}

	logger.Info("Recipe executor initialisation completed")
}

// Retrieve recipes from ConfigMap.
func getRecipesFromConfigMap(clientset *kubernetes.Clientset) (map[string]RecipeConfig, error) {
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
func createJob(
	clientset *kubernetes.Clientset, recipeName string, recipeConfig RecipeConfig, alertData *map[string]interface{},
) error {
	jobClient := clientset.BatchV1().Jobs(jobNamespace)

	// Define the Job object
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%v-", recipeName),
			Labels:       map[string]string{"app": "euphrosyne", "recipe": recipeName},
			Namespace:    jobNamespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "euphrosyne", "recipe": recipeName},
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

	_, err := jobClient.Create(context.TODO(), job, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	logger.Info("Job created successfully", zap.String("jobName", job.Name))
	return nil
}

// Build Recipe command
func buildRecipeCommand(recipeConfig RecipeConfig, alertData *map[string]interface{}) string {
	var alertDataString string
	for _, value := range *alertData {
		alertDataString += fmt.Sprintf("%v ", value)
	}

	var recipeCommand string
	recipeCommand += fmt.Sprintf("%v ", recipeConfig.Entrypoint)
	for _, param := range recipeConfig.Params {
		recipeCommand += fmt.Sprintf("--%v '%v'", param.Name, alertDataString)
	}
	return recipeCommand
}
