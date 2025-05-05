package main

import (
	"log"
	"os"
	"time"

	"ssl-exporter/metrics"
	"ssl-exporter/server"
)

const (
	domainsFile = "domains.json"
	interval    = 5 * time.Minute
)

func main() {
	domains, err := metrics.LoadDomains(domainsFile)
	if err != nil || len(domains) == 0 {
		log.Fatalf("Error loading domains: %v", err)
		os.Exit(1)
	}

	go server.Start()

	for {
		metrics.Generate(domains)
		time.Sleep(interval)
	}
}
