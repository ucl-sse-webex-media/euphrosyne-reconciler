package main

import (
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// unit test
// not finished yet
func Test_CollectRecipeResult(t *testing.T){
    connectRedis()
	var wg sync.WaitGroup
    wg.Add(3)

    c, _ := gin.CreateTestContext(httptest.NewRecorder())

    defer deleteTestConfigmap()

	err := createTestConfigmap()
	assert.Nil(t,err)

	recipes,err := getRecipesFromConfigMap()
    assert.NotNil(t,recipes)
	assert.Nil(t,err)
    
	r,err := NewAlertReconciler(c,alertData,recipes)
    assert.NotNil(t,r)
    assert.Nil(t,err)

	go func() {
        defer wg.Done()
		collectRecipeResult(r)
    }()

	go func() {
        defer wg.Done()
        time.Sleep(time.Second)
    }()

    go func() {
        defer wg.Done()
        time.Sleep(time.Second)
    }()

    wg.Wait()
}

