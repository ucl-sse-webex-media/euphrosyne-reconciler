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
    wg.Add(2)
    recipeTimeout = 20
    c, _ := gin.CreateTestContext(httptest.NewRecorder())

	r,err := NewAlertReconciler(c,alertData,recipeMap)
    assert.NotNil(t,r)
    assert.Nil(t,err)

    // two recipe works normally 

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
    
    receivedMessages,err := collectRecipeResult(r)
    assert.Nil(t,err)
    assert.Equal(t,2,len(receivedMessages))
    wg.Wait()
    
    // simulate time out
    wg.Add(2)
    recipeTimeout = 2

    r,err = NewAlertReconciler(c,alertData,recipeMap)
    assert.NotNil(t,r)
    assert.Nil(t,err)

	go func() {
        defer wg.Done()
        time.Sleep(time.Second)
        rdb.Publish(c,(*alertData)["uuid"].(string),"{}")
    }()

    go func() {
        defer wg.Done()
        time.Sleep(3)
    }()
    
    receivedMessages,err = collectRecipeResult(r)
    assert.Nil(t,err)
    assert.Equal(t,1,len(receivedMessages))

    wg.Wait()    
}

