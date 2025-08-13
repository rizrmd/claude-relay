// Example of non-interactive authentication using a callback function.
package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"claude-relay"
)

func main() {
	// Create relay with authentication callback
	relay, err := clauderelay.New(clauderelay.Options{
		Port:          "8081",
		BaseDir:       "./claude-callback",
		AutoSetup:     true,
		EnableLogging: true,
		AuthCallback: func(authURL string) (string, error) {
			// This callback is called when authentication is needed
			// In a real application, you might:
			// - Show the URL in a web UI
			// - Send it via email
			// - Display a QR code
			// - Get the key from a database or secret manager
			
			fmt.Println("========================================")
			fmt.Println("Authentication Required")
			fmt.Println("========================================")
			fmt.Printf("Please visit: %s\n", authURL)
			fmt.Println("Create an API key and paste it below")
			fmt.Print("API Key: ")
			
			// Read API key from user input
			reader := bufio.NewReader(os.Stdin)
			apiKey, err := reader.ReadString('\n')
			if err != nil {
				return "", fmt.Errorf("failed to read API key: %w", err)
			}
			
			apiKey = strings.TrimSpace(apiKey)
			if apiKey == "" {
				return "", fmt.Errorf("API key cannot be empty")
			}
			
			fmt.Println("========================================")
			return apiKey, nil
		},
	})
	if err != nil {
		log.Fatal("Failed to create relay:", err)
	}
	defer relay.Close()

	// Check final authentication status
	authenticated, status, _ := relay.GetAuthStatus()
	fmt.Printf("Authentication: %v (%s)\n", authenticated, status)

	// Start the server
	if err := relay.Start(); err != nil {
		log.Fatal("Failed to start relay:", err)
	}

	fmt.Printf("Claude relay server started on %s\n", relay.GetWebSocketURL())
	
	// Keep running
	select {}
}