package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	testConfigMapName = "orpheus-operator-recipes-test"
	testNamespace = "orpheus-test" // use a test namespace
	testJobNamespace = "orpheus-test"
)

var recipe_config_1 = RecipeConfig{
	Image: "maikeee32e/euphrosyne-recipes-test:latest",
	Entrypoint: "test-1-recipe",
	Params: []struct {
		Name  string `yaml:"name"`
		Value string `yaml:"value"`
	}{
		{Name: "data", Value: "dummy"},
	},
}

var recipe_config_2 = RecipeConfig{
	Image: "maikeee32e/euphrosyne-recipes-test:latest",
	Entrypoint: "test-2-recipe",
	Params: []struct {
		Name  string `yaml:"name"`
		Value string `yaml:"value"`
	}{
		{Name: "data", Value: "dummy"},
	},
}

//test-1-recipe contains a single redis publish msg process
//test-2-recipe has some bugs and fails to run
var configMap = map[string]string{
	"test-1-recipe": `image: "maikeee32e/euphrosyne-recipes-test:latest"
entrypoint: "test-1-recipe"
params:
- name: "data"
  value: "dummy"`,

	"test-2-recipe": `image: "maikeee32e/euphrosyne-recipes-test:latest"
entrypoint: "test-2-recipe"
params:
- name: "data"
  value: "dummy"`,
}

var alertData = &map[string]interface{}{
	"uuid": "123",
}

func createTestNamespace() {
	testNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNamespace,
		},
	}
	_, err := clientset.CoreV1().Namespaces().Create(context.TODO(), testNamespace, metav1.CreateOptions{})
	if err != nil {
		panic(err.Error())
	}
}

func createTestConfigmap() error{
	configMapObj := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testConfigMapName,
			Namespace: testNamespace,
		},
		Data: configMap,
	}
	_, err := clientset.CoreV1().ConfigMaps(testNamespace).Create(context.TODO(), configMapObj, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func deleteTestConfigmap() {
	err := clientset.CoreV1().ConfigMaps(testNamespace).Delete(context.TODO(), testConfigMapName, metav1.DeleteOptions{})
    if err != nil {
		panic(err)
    }
}


func init(){
	initLogger()

	configMapNamespace = testNamespace
	configMapName = testConfigMapName
	jobNamespace = testJobNamespace
	var err error
	clientset,err = InitialiseKubernetesClient()
	if err != nil {
		panic(err)
	}
	// check whether the test namespace exists
	_, err = clientset.CoreV1().Namespaces().Get(context.TODO(),testNamespace,metav1.GetOptions{})
	if err != nil {
		createTestNamespace()
	}
}

// unit test 
func Test_GetRecipeConfig(t *testing.T){
	defer deleteTestConfigmap()

	err := createTestConfigmap()
	assert.Nil(t,err)

	recipe,err := getRecipesFromConfigMap()
	assert.Nil(t,err)
	assert.Equal(t,2,len(recipe))

	assert.Equal(t,recipe_config_1,recipe["test-1-recipe"])
	assert.Equal(t,recipe_config_2,recipe["test-2-recipe"])
}

func Test_BuildRecipeCommand(t *testing.T){
	command := buildRecipeCommand(recipe_config_1,alertData)
	expect_command := "test-1-recipe --data '{\"uuid\":123}' --aggregator-base-url 'http://thalia-aggregator.default.svc.cluster.local' --redis-address 'euphrosyne-reconciler-redis:80' "
	assert.Equal(t,expect_command,command)
}

func Test_CreateJob(t *testing.T){
	job, err := createJob("test-1-recipe",recipe_config_1,alertData)
	assert.NotNil(t,job)
	assert.Nil(t,err)
	getJob, err := clientset.BatchV1().Jobs(testNamespace).Get(context.TODO(), job.Name, metav1.GetOptions{})
	assert.NotNil(t,getJob)
	assert.Nil(t,err)
	err = clientset.BatchV1().Jobs(testNamespace).Delete(context.TODO(), job.Name, metav1.DeleteOptions{})
	assert.Nil(t,err)
}


// integration test
func Test_StartRecipeExecutor(t *testing.T){

}