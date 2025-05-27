package main

import (
	"time"

	"ssl-exporter/metrics"
	"ssl-exporter/server"
)

const interval = 5 * time.Minute

func main() {
	// run metrics generation immediately
	metrics.Generate()

	// start HTTP server
	go server.Start()

	// update metrics every 5 minutes
	for {
		time.Sleep(interval)
		metrics.Generate()
	}
}
