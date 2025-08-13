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
//	import "github.com/yourusername/claude-relay"
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
// ### 1. SetAuthToken() - Non-Interactive/Programmatic
//
// Use when you already have an API key. Perfect for servers, containers, and headless environments.
//
//	apiKey := os.Getenv("CLAUDE_API_KEY")
//	err := relay.SetAuthToken(apiKey)
//
// ### 2. Authenticate() - Interactive/Terminal-Based
//
// Use when running in a terminal with user interaction. Launches Claude's interactive login.
//
//	err := relay.Authenticate()
//	// User will see Claude interface, choose theme, type /login
//
// ### 3. Options.APIKey - Automatic Authentication
//
// Provide API key during initialization for automatic setup.
//
//	relay, _ := clauderelay.New(clauderelay.Options{
//		APIKey: "sk-ant-...",  // Automatically authenticates
//	})
//
// ### 4. Options.AuthCallback - Custom Authentication Flow
//
// Implement your own authentication UI/flow.
//
//	relay, _ := clauderelay.New(clauderelay.Options{
//		AuthCallback: func(authURL string) (string, error) {
//			// Show authURL to user in your UI
//			// Return API key from your source
//			return getUserAPIKey(authURL), nil
//		},
//	})
//
// ## When to Use Each Method
//
// SetAuthToken() - Use when:
// • Running as a web service or API
// • Deploying in Docker containers
// • Running in CI/CD pipelines
// • Serverless functions (AWS Lambda, etc.)
// • Background jobs or workers
// • Any environment without terminal access
//
// Authenticate() - Use when:
// • Building CLI tools
// • Local development with terminal
// • One-time setup scripts
// • User manages their own authentication
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
// ## Getting the Auth URL
//
// For custom authentication flows:
//
//	url, _ := relay.GetAuthURL()
//	// Returns: "https://console.anthropic.com/settings/keys"
//
// # Thread Safety
//
// The Relay type is thread-safe and can be safely accessed from multiple goroutines.
// Multiple relay instances can run concurrently without interference.
package clauderelay