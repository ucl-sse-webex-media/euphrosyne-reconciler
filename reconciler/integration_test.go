package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_500Alert(t *testing.T){

	assert.Equal(t,os.Getenv("ddd"),"")
}