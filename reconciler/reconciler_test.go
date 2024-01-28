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
func Test_CollectRecipeResult(t *testing.T){
    // make sure redis is started 
    connectRedis()
	var wg sync.WaitGroup
    wg.Add(3)

    c, _ := gin.CreateTestContext(httptest.NewRecorder())

	r,err := NewAlertReconciler(c,alertData,recipeMap)
    assert.NotNil(t,r)
    assert.Nil(t,err)

    // two recipe works normally 
	go func() {
        defer wg.Done()
		receivedMessages,err := collectRecipeResult(r)
        assert.Nil(t,err)
        assert.Equal(t,2,len(receivedMessages))
    }()

	go func() {
        defer wg.Done()
        time.Sleep(time.Second)
        rdb.Publish(c,(*alertData)["uuid"].(string),"{}")
    }()

    go func() {
        defer wg.Done()
        time.Sleep(time.Second)
        rdb.Publish(c,(*alertData)["uuid"].(string),"{}")
    }()

    wg.Wait()
    
    // simulate time out
    wg.Add(3)
    recipeTimeout = 2
    go func() {
        defer wg.Done()
		receivedMessages,err := collectRecipeResult(r)
        assert.Nil(t,err)
        assert.Equal(t,1,len(receivedMessages))
    }()

	go func() {
        defer wg.Done()
        time.Sleep(time.Second)
        rdb.Publish(c,(*alertData)["uuid"].(string),"{}")
    }()

    go func() {
        defer wg.Done()
        time.Sleep(3)
    }()

    
}

