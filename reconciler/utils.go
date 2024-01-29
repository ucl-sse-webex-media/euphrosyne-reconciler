package main

import (
	"fmt"
	"os"

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

func LoadEnvVariable(){
	if os.Getenv("REDIS_ADDRESS")!=""{
		redisAddress = os.Getenv("REDIS_ADDRESS")
	}

	if os.Getenv("WEBEX_BOT_ADDRESS")!=""{
		redisAddress = os.Getenv("WEBEX_BOT_ADDRESS")
	}

	if os.Getenv("RECIPE_TIMEOUT")!=""{
		redisAddress = os.Getenv("RECIPE_TIMEOUT")
	}
}