// Basic example of using the clauderelay library.
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/yourusername/claude-relay"
)

func main() {
	// Create a new relay instance with default settings
	relay, err := clauderelay.New(clauderelay.Options{
		Port:          "8081",
		BaseDir:       "./claude-instance",
		AutoSetup:     true,
		EnableLogging: true,
	})
	if err != nil {
		log.Fatal("Failed to create relay:", err)
	}
	defer relay.Close()

	// Check if authentication is needed
	authenticated, err := relay.IsAuthenticated()
	if err != nil {
		log.Printf("Warning: Failed to check authentication: %v", err)
	}

	if !authenticated {
		fmt.Println("========================================")
		fmt.Println("Claude needs authentication")
		fmt.Println("Please complete the login process")
		fmt.Println("Type /login when prompted")
		fmt.Println("========================================")
		
		if err := relay.Authenticate(); err != nil {
			log.Fatal("Failed to authenticate:", err)
		}
	}

	// Start the relay server
	if err := relay.Start(); err != nil {
		log.Fatal("Failed to start relay:", err)
	}

	fmt.Printf("Claude relay server started on %s\n", relay.GetWebSocketURL())
	fmt.Println("Open client.html in your browser to connect")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down...")
	if err := relay.Stop(); err != nil {
		log.Printf("Error stopping relay: %v", err)
	}
}