package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	clauderelay "github.com/rizrmd/claude-relay"
)

func main() {
	// Create Claude setup
	setup, err := clauderelay.New("./claude-instance")
	if err != nil {
		log.Fatal("Failed to create setup:", err)
	}

	// Install Claude CLI if needed
	if !setup.IsInstalled() {
		if err := setup.Setup(); err != nil {
			log.Fatal("Failed to install Claude:", err)
		}
	}

	// Check authentication
	authenticated, _ := setup.CheckAuthentication()
	if !authenticated {
		// Interactive authentication
		reader := bufio.NewReader(os.Stdin)
		if err := setup.Authenticate(reader); err != nil {
			log.Fatal("Authentication failed:", err)
		}
	}

	// Create Claude process
	process, err := clauderelay.NewClaudeProcess(setup)
	if err != nil {
		log.Fatal("Failed to create process:", err)
	}
	defer process.Kill()
	defer process.Cleanup()

	// Read message from stdin or use default
	var message string
	if stat, _ := os.Stdin.Stat(); (stat.Mode() & os.ModeCharDevice) == 0 {
		// Input is piped
		input, err := io.ReadAll(os.Stdin)
		if err != nil {
			log.Fatal("Failed to read input:", err)
		}
		message = strings.TrimSpace(string(input))
	}
	
	if message == "" {
		message = "Hello Claude! What's 2+2?"
	}

	// Send the message
	response, err := process.SendMessage(message)
	if err != nil {
		log.Fatal("Failed to send message:", err)
	}

	fmt.Println("Claude:", response)
}