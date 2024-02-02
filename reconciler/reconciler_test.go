package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// unit test
func Test_CollectRecipeResult(t *testing.T) {
	// make sure redis is started
	connectRedis()
	var wg sync.WaitGroup
	wg.Add(2)
	recipeTimeout = 5

	// simulate that there are 2 recipes
	testRecipeMap := map[string]RecipeConfig{
		"test-1-recipe": recipeConfig1,
		"test-2-recipe": recipeConfig2,
	}

	r, err := newAlertReconciler(c, alertData, testRecipeMap)
	assert.NotNil(t, r)
	assert.Nil(t, err)

	// two recipe works normally
	go func() {
		defer wg.Done()
		time.Sleep(time.Second)
		rdb.Publish(c, (*alertData)["uuid"].(string), "{}")
	}()

	go func() {
		defer wg.Done()
		time.Sleep(time.Second)
		rdb.Publish(c, (*alertData)["uuid"].(string), "{}")
	}()

	receivedMessages, err := collectRecipeResult(r)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(receivedMessages))
	wg.Wait()

	// 1 recipe timeout
	wg.Add(2)
	recipeTimeout = 2
	r, err = newAlertReconciler(c, alertData, testRecipeMap)
	assert.NotNil(t, r)
	assert.Nil(t, err)

	go func() {
		defer wg.Done()
		time.Sleep(time.Second)
		rdb.Publish(c, (*alertData)["uuid"].(string), "{}")
	}()

	go func() {
		defer wg.Done()
		time.Sleep(3 * time.Second)
	}()
	receivedMessages, err = collectRecipeResult(r)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(receivedMessages))
	wg.Wait()
}

func Test_Cleanup(t *testing.T) {
	// an easy job that must run successfully
	jobObj := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-job-",
			Labels: map[string]string{
				"app":     "euphrosyne",
				"recipes": "",
				"uuid":    (*alertData)["uuid"].(string),
			},
			Namespace: jobNamespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":     "euphrosyne",
						"recipes": "",
						"uuid":    (*alertData)["uuid"].(string),
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    "test-job",
							Image:   "busybox",
							Command: []string{"echo", "Hello from Kubernetes job"},
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}

	r, err := newAlertReconciler(c, alertData, nil)
	assert.Nil(t, err)

	job, err := clientset.BatchV1().Jobs(testJobNamespace).Create(context.TODO(), jobObj, metav1.CreateOptions{})
	assert.NotNil(t, job)
	assert.Nil(t, err)

	for {
		getJob, err := clientset.BatchV1().Jobs(testNamespace).Get(context.TODO(), job.Name, metav1.GetOptions{})
		assert.NotNil(t, getJob)
		assert.Nil(t, err)
		if getJob.Status.Succeeded > 0 {
			r.Cleanup()
			getJob, err = clientset.BatchV1().Jobs(testNamespace).Get(context.TODO(), job.Name, metav1.GetOptions{})
			assert.Equal(t, true, errors.IsNotFound(err))
			assert.Equal(t, "", getJob.Name)
			break
		}
		time.Sleep(1 * time.Second)
	}
}
