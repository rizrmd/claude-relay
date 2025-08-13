# Claude Relay

A Go library for communicating with Claude CLI, providing isolated installations and process management.

## What This Library Does

The library provides **only** Claude CLI communication functionality:

- **Isolated Claude CLI Installation**: Manages portable Bun and Claude CLI installations
- **Process Management**: Spawns and controls Claude CLI processes with conversation state
- **Authentication Management**: Handles Claude CLI browser-based authentication
- **No Network Components**: Pure library for local Claude CLI interaction

## Installation

```bash
go get github.com/rizrmd/claude-relay
```

## Library Usage

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    clauderelay "github.com/rizrmd/claude-relay"
)

func main() {
    // Initialize Claude setup
    setup, err := clauderelay.NewClaudeSetup()
    if err != nil {
        log.Fatal(err)
    }
    
    // Install Claude CLI if needed
    if !setup.IsInstalled() {
        if err := setup.Setup(); err != nil {
            log.Fatal(err)
        }
    }
    
    // Check and handle authentication
    if authenticated, _ := setup.CheckAuthentication(); !authenticated {
        fmt.Println("Please authenticate with Claude:")
        if err := setup.RunClaudeLogin(); err != nil {
            log.Fatal(err)
        }
    }
    
    // Create Claude process
    process, err := clauderelay.NewClaudeProcess(setup)
    if err != nil {
        log.Fatal(err)
    }
    defer process.Stop()
    
    // Send message to Claude
    ctx := context.Background()
    response, err := process.SendMessage(ctx, "Hello Claude!")
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Println("Claude:", response)
}
```

## Core API

### Setup and Installation

```go
// Create setup manager
setup, err := clauderelay.NewClaudeSetup()
setup, err := clauderelay.NewClaudeSetupWithBaseDir("./my-claude")

// Install Claude CLI and dependencies
err := setup.Setup()

// Check if installed
installed := setup.IsInstalled()

// Get installation paths
claudePath := setup.GetClaudePath()
claudeHome := setup.GetClaudeHome()
```

### Authentication

```go
// Check authentication status
authenticated, err := setup.CheckAuthentication()

// Run interactive authentication (requires terminal and browser)
err := setup.RunClaudeLogin()

// Copy authentication from another installation
err := setup.CopyAuthFrom("/path/to/auth/dir")
```

### Process Management

```go
// Create Claude process
process, err := clauderelay.NewClaudeProcess(setup)

// Send message and get response
response, err := process.SendMessage(ctx, "Your message here")

// Send message with conversation history
response, err := process.SendMessageWithHistory(ctx, "Message", conversationHistory)

// Stop process
err := process.Stop()
```

## CLI Tool

The repository includes a simple CLI tool for direct interaction:

```bash
# Install the CLI
go install github.com/rizrmd/claude-relay/cmd/claude-relay@latest

# Install Claude CLI
claude-relay -setup

# Authenticate
claude-relay -auth

# Send a message
claude-relay -message "What is 2+2?"

# Show status
claude-relay
```

## Examples

### Basic Usage
See [examples/basic](examples/basic) - Simple message sending to Claude CLI

### WebSocket Server
See [examples/websocket-server](examples/websocket-server) - Full WebSocket server with web UI

### Other Examples
See [examples/auth-preconfig](examples/auth-preconfig) - Using pre-configured authentication
See [examples/docker-deployment](examples/docker-deployment) - Docker deployment example
See [examples/webservice](examples/webservice) - Web service integration

## WebSocket Server Example

The [examples/websocket-server](examples/websocket-server) directory contains a complete WebSocket server implementation:

```bash
# Run the WebSocket server
cd examples/websocket-server
go run .

# Or with custom port
go run . 8081

# Access the web UI
open http://localhost:8080
```

Features:
- WebSocket endpoint for real-time communication
- Web UI with conversation management
- Undo/restore functionality
- Progress indicators

## Directory Structure

```
claude-relay/
├── setup.go              # Claude CLI installation and auth (library)
├── process.go            # Claude process management (library)
├── config.go             # Configuration structures (library)
├── doc.go                # Package documentation (library)
├── cmd/
│   └── claude-relay/     # Simple CLI tool
│       └── main.go
└── examples/
    ├── basic/            # Basic usage example
    ├── websocket-server/ # Complete WebSocket server application
    │   ├── main.go       # Server application
    │   ├── websocket.go  # WebSocket handler
    │   └── client.html   # Web UI
    ├── auth-preconfig/   # Pre-configured auth example
    ├── docker-deployment/# Docker deployment example
    └── webservice/       # Web service integration example
```

## Authentication

Claude CLI uses browser-based OAuth authentication (NOT API keys):

1. Run `/login` command in Claude CLI
2. Browser opens for Anthropic account login
3. Session token saved in `.claude-home/.config/claude/auth.json`

### Interactive Authentication

```go
// Run Claude CLI interactively for authentication
err := setup.RunClaudeLogin()
// User types /login in Claude CLI
// Browser opens for authentication
// User completes OAuth flow
```

Or use the helper for better UX:
```go
err := setup.RunInteractiveAuth()
// Guides user through the authentication process
```

### Non-Interactive Authentication

For servers and automated deployments:

```go
// Step 1: Get authentication URL
authURL, sessionID, err := setup.StartNonInteractiveAuth()
fmt.Printf("Visit: %s\n", authURL)

// Step 2: User completes auth in browser and gets session token

// Step 3: Complete authentication with token
err = setup.CompleteNonInteractiveAuth(sessionToken)
```

Or use environment variable:
```bash
export CLAUDE_SESSION_TOKEN="your-session-token"
# Server will auto-authenticate on startup
```

### Pre-configured Authentication

For headless environments:

```bash
# On development machine with browser
claude-relay -auth

# Backup auth files
tar -czf claude-auth.tar.gz .claude-home/.config/claude/

# On production machine
tar -xzf claude-auth.tar.gz
```

## Requirements

- Go 1.19 or higher
- Internet connection for initial Claude CLI setup
- macOS or Linux (Windows not tested)
- Browser access for initial authentication

## License

MIT License - see [LICENSE](LICENSE) file

Copyright (c) 2024 rizrmd