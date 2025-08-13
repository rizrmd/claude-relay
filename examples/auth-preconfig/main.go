// Example of using pre-configured authentication (copying from existing auth).
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/rizrmd/claude-relay"
)

func main() {
	// Path to pre-authenticated Claude config
	// This would typically be copied from a development machine
	// where you've already done the interactive login
	authBackupDir := os.Getenv("CLAUDE_AUTH_DIR")
	if authBackupDir == "" {
		// Try common locations
		homeDir, _ := os.UserHomeDir()
		authBackupDir = filepath.Join(homeDir, ".claude-backup", "config", "claude")
	}

	// Check if auth backup exists
	if _, err := os.Stat(filepath.Join(authBackupDir, "auth.json")); os.IsNotExist(err) {
		log.Fatal(`No pre-configured authentication found.

To set up authentication for production:
1. On a machine with browser access, run the interactive auth
2. Copy the .claude-home/.config/claude/ directory
3. Set CLAUDE_AUTH_DIR environment variable to point to it
`)
	}

	// Create relay using pre-authenticated config
	relay, err := clauderelay.New(clauderelay.Options{
		Port:             "8081",
		BaseDir:          "./claude-preconfigured",
		AutoSetup:        true,
		EnableLogging:    true,
		PreAuthDirectory: authBackupDir, // Use pre-existing auth
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

	if !authenticated {
		log.Fatal("Pre-configured authentication failed. Check your auth files.")
	}

	// Start the server
	if err := relay.Start(); err != nil {
		log.Fatal("Failed to start relay:", err)
	}

	fmt.Printf("Claude relay server started on %s\n", relay.GetWebSocketURL())
	fmt.Println("Using pre-configured authentication from:", authBackupDir)
	
	// Keep running
	select {}
}