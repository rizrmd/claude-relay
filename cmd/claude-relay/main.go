// Simple CLI wrapper for Claude relay library.
// This provides a basic command-line interface for running Claude CLI.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	clauderelay "github.com/rizrmd/claude-relay"
)

func main() {
	var (
		baseDir = flag.String("dir", ".", "Base directory for Claude installation")
		setup   = flag.Bool("setup", false, "Run setup to install Claude CLI")
		auth    = flag.Bool("auth", false, "Run interactive authentication")
		message = flag.String("message", "", "Send a message to Claude")
		help    = flag.Bool("help", false, "Show help")
	)

	flag.Parse()

	if *help {
		fmt.Println("Claude Relay CLI - Communicate with Claude CLI")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  claude-relay [options]")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  -dir string     Base directory for Claude installation (default \".\")")
		fmt.Println("  -setup          Run setup to install Claude CLI")
		fmt.Println("  -auth           Run interactive authentication")
		fmt.Println("  -message string Send a message to Claude")
		fmt.Println("  -help           Show this help message")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  # Install Claude CLI")
		fmt.Println("  claude-relay -setup")
		fmt.Println()
		fmt.Println("  # Authenticate with Claude")
		fmt.Println("  claude-relay -auth")
		fmt.Println()
		fmt.Println("  # Send a message to Claude")
		fmt.Println("  claude-relay -message \"What is 2+2?\"")
		fmt.Println()
		fmt.Println("For WebSocket server, see examples/websocket-server")
		os.Exit(0)
	}

	// Create Claude setup
	claudeSetup, err := clauderelay.NewClaudeSetupWithBaseDir(*baseDir)
	if err != nil {
		log.Fatal("Failed to initialize Claude setup:", err)
	}

	// Run setup if requested
	if *setup {
		fmt.Println("Setting up Claude CLI...")
		if err := claudeSetup.Setup(); err != nil {
			log.Fatal("Setup failed:", err)
		}
		fmt.Println("Claude CLI installed successfully!")
		return
	}

	// Check if Claude is installed
	if !claudeSetup.IsInstalled() {
		fmt.Println("Claude CLI is not installed. Run with -setup flag to install.")
		os.Exit(1)
	}

	// Run authentication if requested
	if *auth {
		fmt.Println("Starting Claude authentication...")
		fmt.Println("When Claude starts, choose a theme (press 1)")
		fmt.Println("Then type /login and follow the instructions")
		if err := claudeSetup.RunClaudeLogin(); err != nil {
			log.Fatal("Authentication failed:", err)
		}
		fmt.Println("Authentication completed!")
		return
	}

	// Check authentication
	authenticated, err := claudeSetup.CheckAuthentication()
	if err != nil {
		log.Printf("Warning: Failed to check authentication: %v", err)
	}

	if !authenticated {
		fmt.Println("Claude is not authenticated. Run with -auth flag to authenticate.")
		os.Exit(1)
	}

	// Send message if provided
	if *message != "" {
		process, err := clauderelay.NewClaudeProcess(claudeSetup)
		if err != nil {
			log.Fatal("Failed to create Claude process:", err)
		}
		defer process.Stop()

		ctx := context.Background()
		response, err := process.SendMessage(ctx, *message)
		if err != nil {
			log.Fatal("Failed to send message:", err)
		}

		fmt.Println(response)
		return
	}

	// If no specific action, show status
	fmt.Println("Claude Relay Status:")
	fmt.Printf("  Installation directory: %s\n", *baseDir)
	fmt.Printf("  Claude installed: %v\n", claudeSetup.IsInstalled())
	fmt.Printf("  Authenticated: %v\n", authenticated)
	fmt.Println()
	fmt.Println("Run with -help for usage information")
}