// Example of non-interactive authentication using API key.
package main

import (
	"fmt"
	"log"
	"os"

	"claude-relay"
)

func main() {
	// Get API key from environment variable
	apiKey := os.Getenv("CLAUDE_API_KEY")
	if apiKey == "" {
		log.Fatal("Please set CLAUDE_API_KEY environment variable")
	}

	// Create relay with API key - no interactive authentication needed
	relay, err := clauderelay.New(clauderelay.Options{
		Port:          "8081",
		BaseDir:       "./claude-apikey",
		AutoSetup:     true,
		EnableLogging: true,
		APIKey:        apiKey, // Provide API key directly
	})
	if err != nil {
		log.Fatal("Failed to create relay:", err)
	}
	defer relay.Close()

	// Check authentication status
	authenticated, status, err := relay.GetAuthStatus()
	if err != nil {
		log.Printf("Error checking auth: %v", err)
	} else {
		fmt.Printf("Authentication status: %v (%s)\n", authenticated, status)
	}

	// Start the server
	if err := relay.Start(); err != nil {
		log.Fatal("Failed to start relay:", err)
	}

	fmt.Printf("Claude relay server started on %s\n", relay.GetWebSocketURL())
	fmt.Println("Authentication was handled automatically using the provided API key")
	
	// Keep running
	select {}
}