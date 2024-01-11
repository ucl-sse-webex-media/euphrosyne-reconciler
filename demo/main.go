package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sigs.k8s.io/yaml"
	"github.com/argoproj/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/pointer"

	wfv1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	wfclientset "github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned"
)



func main() {
	// get current user to determine home directory
	usr, err := user.Current()
	checkErr(err)

	yamlContent, err := os.ReadFile("hello-world.yaml")
    checkErr(err)

	// parse the workflow json to wf instance
	var wf wfv1.Workflow
	err = yaml.UnmarshalStrict(yamlContent, &wf)

	checkErr(err)

	// get kubeconfig file location
	kubeconfig := flag.String("kubeconfig", filepath.Join(usr.HomeDir, ".kube", "config"),"")
	flag.Parse()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	checkErr(err)
	namespace := "default"
	
	// create the workflow client
	wfClient := wfclientset.NewForConfigOrDie(config).ArgoprojV1alpha1().Workflows(namespace)
	// submit the hello world workflow
	ctx := context.Background()
	createdWf, err := wfClient.Create(ctx, &wf, metav1.CreateOptions{})
	checkErr(err)
	fmt.Printf("Workflow %s submitted\n", createdWf.Name)

	// wait for the workflow to complete
	fieldSelector := fields.ParseSelectorOrDie(fmt.Sprintf("metadata.name=%s", createdWf.Name))
	watchIf, err := wfClient.Watch(ctx, metav1.ListOptions{FieldSelector: fieldSelector.String(), TimeoutSeconds: pointer.Int64Ptr(180)})
	errors.CheckError(err)
	defer watchIf.Stop()
	for next := range watchIf.ResultChan() {
		wf, ok := next.Object.(*wfv1.Workflow)
		if !ok {
			continue
		}
		if !wf.Status.FinishedAt.IsZero() {
			fmt.Printf("Workflow %s %s at %v. Message: %s.\n", wf.Name, wf.Status.Phase, wf.Status.FinishedAt, wf.Status.Message)
			break
		}
	}
}

func checkErr(err error) {
	if err != nil {
		panic(err.Error())
	}
}
