// Package clauderelay provides a WebSocket relay server for Claude Code CLI with
// isolated, portable installation support.
//
// This package enables you to programmatically create and manage multiple isolated
// Claude instances, each with its own configuration and authentication, without
// requiring system-wide installation.
//
// # Features
//
// • Isolated Installation: Each instance has its own Bun and Claude installation
// • Multiple Instances: Run multiple Claude relays on different ports
// • Auto-Setup: Automatically downloads and installs dependencies
// • Authentication Management: Handle Claude authentication programmatically
// • WebSocket Interface: Connect to Claude via WebSocket protocol
// • Conversation State: Maintains context across messages
// • Undo/Restore: Support for undoing and restoring conversations
//
// # Basic Usage
//
// Create a simple relay server:
//
//	import "github.com/rizrmd/claude-relay"
//
//	func main() {
//		// Create relay with automatic setup
//		relay, err := clauderelay.New(clauderelay.Options{
//			Port:          "8081",
//			BaseDir:       "./claude-instance",
//			AutoSetup:     true,
//			EnableLogging: true,
//		})
//		if err != nil {
//			log.Fatal(err)
//		}
//		defer relay.Close()
//
//		// Check authentication
//		if authenticated, _ := relay.IsAuthenticated(); !authenticated {
//			if err := relay.Authenticate(); err != nil {
//				log.Fatal(err)
//			}
//		}
//
//		// Start the server
//		if err := relay.Start(); err != nil {
//			log.Fatal(err)
//		}
//
//		// Wait for shutdown
//		select {}
//	}
//
// # Multiple Instances
//
// Run multiple isolated Claude instances:
//
//	// Create instance 1
//	relay1, _ := clauderelay.New(clauderelay.Options{
//		Port:    "8081",
//		BaseDir: "./claude-1",
//	})
//	relay1.Start()
//
//	// Create instance 2
//	relay2, _ := clauderelay.New(clauderelay.Options{
//		Port:    "8082",
//		BaseDir: "./claude-2",
//	})
//	relay2.Start()
//
// # Embedding in Existing Server
//
// Integrate Claude relay into your existing HTTP server:
//
//	relay, _ := clauderelay.New(clauderelay.Options{
//		Port: "8081",
//	})
//	relay.Start()
//
//	// Use relay.GetWebSocketURL() to get the WebSocket endpoint
//	// Integrate with your existing routes
//
// # WebSocket Protocol
//
// The relay server exposes a WebSocket endpoint at /ws that accepts text messages
// and returns Claude's responses.
//
// Client to Server:
// • Plain text messages to send to Claude
// • /undo-from:N - Undo from message index N
// • /restore - Restore last undone conversation
//
// Server to Client:
// • Claude's responses as plain text
// • Progress indicators during processing
// • Error messages prefixed with ❌
//
// # Directory Structure
//
// Each Claude instance creates the following structure in its BaseDir:
//
//	BaseDir/
//	├── .bun/                  // Portable Bun installation
//	│   └── bin/
//	│       ├── bun            // Bun executable
//	│       └── claude         // Claude CLI
//	├── .claude-home/          // Claude configuration
//	│   └── .config/
//	│       └── claude/
//	│           └── auth.json  // Authentication token
//	└── claude-relay-*/        // Temporary workspaces (runtime)
//
// # Authentication
//
// Claude requires authentication on first use. The library provides multiple authentication methods
// to suit different deployment scenarios:
//
// ## Authentication Methods
//
// Claude Code CLI uses browser-based authentication, NOT API keys.
//
// ### 1. Authenticate() - Interactive/Terminal-Based
//
// Standard authentication flow - requires terminal and browser access.
//
//	err := relay.Authenticate()
//	// User will:
//	// 1. See Claude interface in terminal
//	// 2. Choose a theme (press 1 for dark)
//	// 3. Type /login
//	// 4. Complete browser authentication
//
// ### 2. CopyAuthFrom() - Reuse Existing Authentication
//
// Copy authentication from another machine or backup.
//
//	// First, authenticate on a machine with browser access
//	// Then copy .claude-home/.config/claude/ directory
//	err := relay.CopyAuthFrom("/path/to/claude/config")
//
// ### 3. Options.PreAuthDirectory - Pre-authenticated Setup
//
// Provide pre-authenticated config during initialization.
//
//	relay, _ := clauderelay.New(clauderelay.Options{
//		PreAuthDirectory: "/backup/claude-auth",
//	})
//
// ## When to Use Each Method
//
// Authenticate() - Use when:
// • Local development with terminal
// • Initial setup with browser access
// • One-time authentication
//
// CopyAuthFrom() - Use when:
// • Deploying to servers without browser
// • Docker containers
// • CI/CD pipelines
// • Reusing auth across instances
//
// ## Authentication in Production
//
// For production deployments without browser access:
//
// 1. Authenticate once on a development machine:
//
//	relay.Authenticate() // Interactive login
//	authPath := relay.GetAuthConfigPath()
//	// Save the files in authPath
//
// 2. Copy auth files to production:
//
//	relay.CopyAuthFrom("/deployed/auth/backup")
//
// ## Checking Authentication Status
//
//	// Simple check
//	authenticated, err := relay.IsAuthenticated()
//
//	// Detailed status
//	authenticated, message, err := relay.GetAuthStatus()
//	// Returns: (true, "Authenticated", nil) or (false, "No authentication file found", nil)
//
// # Thread Safety
//
// The Relay type is thread-safe and can be safely accessed from multiple goroutines.
// Multiple relay instances can run concurrently without interference.
package clauderelay