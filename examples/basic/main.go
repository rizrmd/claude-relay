// Basic example of using the clauderelay library to communicate with Claude CLI.
package main

import (
	"context"
	"fmt"
	"log"

	clauderelay "github.com/rizrmd/claude-relay"
)

func main() {
	// Create Claude setup manager
	setup, err := clauderelay.NewClaudeSetupWithBaseDir("./claude-instance")
	if err != nil {
		log.Fatal("Failed to create setup:", err)
	}

	// Install Claude CLI if not already installed
	if !setup.IsInstalled() {
		fmt.Println("Installing Claude CLI...")
		if err := setup.Setup(); err != nil {
			log.Fatal("Failed to install Claude:", err)
		}
		fmt.Println("Claude CLI installed successfully!")
	}

	// Check authentication
	authenticated, err := setup.CheckAuthentication()
	if err != nil {
		log.Printf("Warning: Failed to check authentication: %v", err)
	}

	if !authenticated {
		fmt.Println("========================================")
		fmt.Println("Claude needs authentication")
		fmt.Println("Please complete the login process")
		fmt.Println("When Claude starts, choose a theme (press 1)")
		fmt.Println("Then type /login and follow the instructions")
		fmt.Println("========================================")
		
		if err := setup.RunClaudeLogin(); err != nil {
			log.Fatal("Failed to authenticate:", err)
		}
		fmt.Println("Authentication completed!")
	}

	// Create Claude process
	process, err := clauderelay.NewClaudeProcess(setup)
	if err != nil {
		log.Fatal("Failed to create Claude process:", err)
	}
	defer process.Stop()

	// Send some messages to Claude
	ctx := context.Background()
	
	messages := []string{
		"Hello Claude! What's 2+2?",
		"Can you write a haiku about Go programming?",
		"What are the benefits of using Go for backend development?",
	}

	for _, msg := range messages {
		fmt.Printf("\nYou: %s\n", msg)
		
		response, err := process.SendMessage(ctx, msg)
		if err != nil {
			log.Printf("Error sending message: %v", err)
			continue
		}
		
		fmt.Printf("Claude: %s\n", response)
	}

	fmt.Println("\nConversation completed!")
}