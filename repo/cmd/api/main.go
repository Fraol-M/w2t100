package main

import (
	"log"

	"propertyops/backend/internal/app"
	"propertyops/backend/internal/config"
)

var version = "dev"

func main() {
	log.Printf("PropertyOps API %s starting...", version)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if err := app.Run(cfg); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}
