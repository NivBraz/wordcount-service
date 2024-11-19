package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/NivBraz/wordcount-service/internal/app"
	"github.com/NivBraz/wordcount-service/internal/config"
)

func main() {
	fmt.Println("Word Count Service")
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	fmt.Printf("Configuration loaded")

	// Create context that listens for the interrupt signal from the OS
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Initialize the application
	fmt.Println("Initializing application...")
	application, err := app.New(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}
	fmt.Println("Application initialized")

	// Run the application
	fmt.Println("Running application...")
	results, err := application.Run(ctx)
	if err != nil {
		log.Printf("Erors that occurred during the application run: %v", err)
	}
	fmt.Println("Application completed")

	// Output results as JSON
	output, err := json.MarshalIndent(results, "", "    ")
	if err != nil {
		log.Fatalf("Failed to marshal results: %v", err)
	}

	fmt.Println(string(output))
}
