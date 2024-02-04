package main

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	testConfigMapName = "orpheus-operator-recipes-test"
	testNamespace     = "orpheus-test" // use a test namespace
	testJobNamespace  = "orpheus-test"
)

var recipe_1 = Recipe{
	Config:&RecipeConfig{
		Image:      "maikeee32e/euphrosyne-recipes-test:latest",
		Entrypoint: "test-1-recipe",
		Params: []struct {
			Name  string `yaml:"name"`
			Value string `yaml:"value"`
		}{
			{Name: "data", Value: "dummy"},
		},
	},
}

var recipe_2 = Recipe{
	Config: &RecipeConfig{
		Image:      "maikeee32e/euphrosyne-recipes-test:latest",
		Entrypoint: "test-2-recipe",
		Params: []struct {
			Name  string `yaml:"name"`
			Value string `yaml:"value"`
		}{
			{Name: "data", Value: "dummy"},
		},
	},
}

// just the literal defination, did not define the real recipe for test
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

var c *gin.Context

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

func createTestConfigmap(cMap map[string]string) error {
	configMapObj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testConfigMapName,
			Namespace: testNamespace,
		},
		Data: cMap,
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

func init() {
	initLogger()

	configMapNamespace = testNamespace
	configMapName = testConfigMapName
	jobNamespace = testJobNamespace
	var err error
	clientset, err = initialiseKubernetesClient()
	if err != nil {
		panic(err)
	}
	// check whether the test namespace exists
	_, err = clientset.CoreV1().Namespaces().Get(context.TODO(), testNamespace, metav1.GetOptions{})
	if err != nil {
		createTestNamespace()
	}

	w := httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)

	// make sure redis is started
	connectRedis()
}

// unit test
func Test_GetRecipeConfig(t *testing.T) {
	defer deleteTestConfigmap()

	testRecipeMap := map[string]Recipe{
		"test-1-recipe": recipe_1,
		"test-2-recipe": recipe_2,
	}

	err := createTestConfigmap(configMap)
	assert.Nil(t, err)

	recipe, err := getRecipesFromConfigMap()
	assert.Nil(t, err)
	assert.Equal(t, len(testRecipeMap), len(recipe))

	assert.Equal(t, testRecipeMap["test-1-recipe"], recipe["test-1-recipe"])
	assert.Equal(t, testRecipeMap["test-2-recipe"], recipe["test-2-recipe"])
}

func Test_CreateJob(t *testing.T) {
	job, err := createJob("test-1-recipe", recipe_1, alertData)
	assert.NotNil(t, job)
	assert.Nil(t, err)
	getJob, err := clientset.BatchV1().Jobs(testNamespace).Get(context.TODO(), job.Name, metav1.GetOptions{})
	assert.NotNil(t, getJob)
	assert.Nil(t, err)
	err = clientset.BatchV1().Jobs(testNamespace).Delete(context.TODO(), job.Name, metav1.DeleteOptions{})
	assert.Nil(t, err)
}
