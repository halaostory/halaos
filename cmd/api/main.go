package main

import (
	"log"

	"github.com/tonypk/aigonhr/internal/app"
	"github.com/tonypk/aigonhr/internal/config"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("config validation failed: %v", err)
	}

	application, err := app.New(cfg)
	if err != nil {
		log.Fatalf("failed to create application: %v", err)
	}

	if err := application.Run(); err != nil {
		log.Fatalf("application error: %v", err)
	}
}
