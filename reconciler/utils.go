package main

import (
	"fmt"
	"os"
	"strconv"

	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// Convert a pointer to an int32.
func int32Ptr(i int32) *int32 { return &i }

// Return the path to the kubeconfig file.
func getKubeconfigPath() string {
	home := homedir.HomeDir()
	return fmt.Sprintf("%s/.kube/config", home)
}

// Initialise a Kubernetes client.
func InitialiseKubernetesClient() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		config, err = clientcmd.BuildConfigFromFlags("", getKubeconfigPath())
		if err != nil {
			logger.Error("Failed to build Kubernetes config", zap.Error(err))
			return nil, err
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Error("Failed to create Kubernetes client", zap.Error(err))
		return nil, err
	}

	return clientset, nil
}

// load local environment variables
func loadEnvVariables(){
	if os.Getenv("REDIS_ADDRESS")!=""{
		redisAddress = os.Getenv("REDIS_ADDRESS")
	}

	if os.Getenv("WEBEX_BOT_ADDRESS")!=""{
		webexBotAddress = os.Getenv("WEBEX_BOT_ADDRESS")
	}

	if os.Getenv("RECIPE_TIMEOUT")!=""{
		recipeTimeout,_ = strconv.Atoi(os.Getenv("RECIPE_TIMEOUT"))
	}
}