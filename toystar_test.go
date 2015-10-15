package main

import (
	"fmt"
	"rundomizer/server/geo"
	"testing"
	"time"
)

func TestAStar(t *testing.T) {
	start := geo.ClosestNode(55.603765, 13.004886)
	goal := geo.ClosestNode(55.596479, 13.037026)
	fmt.Println("Starting A star")
	startTime := time.Now().UnixNano()
	aStar(start, goal)
	fmt.Println("A star: ", float64(time.Now().UnixNano()-startTime)/1000000000.0)
}
