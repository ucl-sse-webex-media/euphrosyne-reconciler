package main

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	testConfigMapName = "orpheus-operator-recipes-test"
	testNamespace     = "orpheus-test"
	imageName         = "maikeee32e/euphrosyne-recipes-test:latest"
)

var testConfig = Config{
	AggregatorAddress:   "localhost:8080",
	RedisAddress:        "localhost:6379",
	WebexBotAddress:     "localhost:7001",
	RecipeTimeout:       300,
	RecipeNamespace:     testNamespace,
	ReconcilerNamespace: testNamespace,
}

var recipe_1 = Recipe{
	Config: &RecipeConfig{
		Enabled:     false,
		Image:       imageName,
		Entrypoint:  "test-1-recipe",
		Description: "Test 1 Recipe",
	},
}

var recipe_2 = Recipe{
	Config: &RecipeConfig{
		Enabled:     true,
		Image:       imageName,
		Description: "Test 2 Recipe",
		Entrypoint:  "test-2-recipe",
	},
}

var recipe_1_config = fmt.Sprintf(`
test-1-recipe:
  enabled: false
  image: "%s"
  entrypoint: "test-1-recipe"
  description: "Test 1 Recipe"
`, imageName)

var recipe_2_config = fmt.Sprintf(`
test-2-recipe:
  enabled: true
  image: "%s"
  entrypoint: "test-2-recipe"
  description: "Test 2 Recipe"
`, imageName)

var debuggingRecipes = fmt.Sprintf("%s%s", recipe_1_config, recipe_2_config)

var actionsRecipes = fmt.Sprintf("%s%s", recipe_1_config, recipe_2_config)

var configMap = map[string]string{
	"debugging": debuggingRecipes,
	"actions":   actionsRecipes,
}

var incidentUuid = "123"
var alertData = &map[string]interface{}{
	"uuid": incidentUuid,
}

var c *gin.Context
var dataConfigMap *corev1.ConfigMap

func createTestNamespace() {
	testNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNamespace,
		},
	}
	_, err := clientset.CoreV1().Namespaces().Create(
		context.TODO(), testNamespace, metav1.CreateOptions{},
	)
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
	_, err := clientset.CoreV1().ConfigMaps(testNamespace).Create(
		context.TODO(), configMapObj, metav1.CreateOptions{},
	)
	if err != nil {
		return err
	}
	return nil
}

func deleteConfigMap(name string, namespace string) {
	err := clientset.CoreV1().ConfigMaps(namespace).Delete(
		context.TODO(), name, metav1.DeleteOptions{},
	)
	if err != nil {
		panic(err)
	}
}

func deleteJob(name string, namespace string) {
	err := clientset.BatchV1().Jobs(namespace).Delete(
		context.TODO(), name, metav1.DeleteOptions{},
	)
	if err != nil {
		panic(err)
	}
}

func deleteNamespace(name string) {
	propagationPolicy := metav1.DeletePropagationForeground
	err := clientset.CoreV1().Namespaces().Delete(
		context.TODO(), name, metav1.DeleteOptions{
			PropagationPolicy: &propagationPolicy,
		},
	)
	if err != nil {
		panic(err)
	}
}

func init() {
	initLogger()

	// FIXME: This is a hack, since the ConfigMap name is hardcoded in the reconciler
	configMapName = testConfigMapName

	var err error
	clientset, err = InitialiseKubernetesClient()
	if err != nil {
		panic(err)
	}

	w := httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)

	// make sure redis is running
	connectRedis(&testConfig)
}

// Test all recipe executor functions.
func TestRecipeExecutor(t *testing.T) {
	// Check whether the test namespace exists
	_, err := clientset.CoreV1().Namespaces().Get(
		context.TODO(), testNamespace, metav1.GetOptions{},
	)
	if err != nil {
		createTestNamespace()
	}
	// create a data ConfigMap for the test recipes
	dataConfigMap, err = createConfigMap(alertData, incidentUuid, testConfig.RecipeNamespace)
	if err != nil {
		panic(err)
	}
	defer deleteNamespace(testNamespace)
	defer deleteConfigMap(dataConfigMap.Name, testNamespace)

	testGetRecipeConfig(t)

	testCreateConfigMap(t)

	testCreateJob(t)
}

// Test that the recipe executor can retrieve recipes from the ConfigMap.
func testGetRecipeConfig(t *testing.T) {
	testRecipeMap := map[string]Recipe{
		"test-1-recipe": recipe_1,
		"test-2-recipe": recipe_2,
	}

	err := createTestConfigmap(configMap)
	assert.Nil(t, err)

	for _, requestType := range []RequestType{Actions, Alert} {
		recipe, err := getRecipesFromConfigMap(requestType, false, testConfig.ReconcilerNamespace)
		assert.Nil(t, err)
		assert.Equal(t, len(testRecipeMap), len(recipe))

		assert.Equal(t, testRecipeMap["test-1-recipe"], recipe["test-1-recipe"])
		assert.Equal(t, testRecipeMap["test-2-recipe"], recipe["test-2-recipe"])
	}

	// Test that the recipe executor can retrieve only enabled recipes from the ConfigMap.
	for _, requestType := range []RequestType{Actions, Alert} {
		recipe, err := getRecipesFromConfigMap(requestType, true, testConfig.ReconcilerNamespace)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(recipe))

		assert.Equal(t, testRecipeMap["test-2-recipe"], recipe["test-2-recipe"])
	}
}

// Test that the recipe executor can create a ConfigMap for the provided recipe data.
func testCreateConfigMap(t *testing.T) {
	var configMapName string
	defer func() {
		deleteConfigMap(configMapName, testNamespace)
	}()

	configMap, err := createConfigMap(alertData, incidentUuid, testConfig.RecipeNamespace)
	assert.Nil(t, err)

	configMapName = configMap.Name
	getConfigMap, err := clientset.CoreV1().ConfigMaps(testNamespace).Get(
		context.TODO(), configMapName, metav1.GetOptions{},
	)
	assert.NotNil(t, getConfigMap)
	assert.Nil(t, err)
}

// Test that the recipe executor can create a Job for the provided alert data.
func testCreateJob(t *testing.T) {
	var jobName string
	defer func() {
		deleteJob(jobName, testNamespace)
	}()

	job, err := createJob("test-1-recipe", recipe_1, incidentUuid, dataConfigMap.Name, &testConfig)
	assert.NotNil(t, job)
	assert.Nil(t, err)
	jobName = job.Name

	getJob, err := clientset.BatchV1().Jobs(testNamespace).Get(
		context.TODO(), job.Name, metav1.GetOptions{},
	)
	assert.NotNil(t, getJob)
	assert.Nil(t, err)
}
