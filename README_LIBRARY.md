# Claude Relay - Go Library

[![Go Reference](https://pkg.go.dev/badge/github.com/yourusername/claude-relay.svg)](https://pkg.go.dev/github.com/yourusername/claude-relay)

A Go library for creating WebSocket relay servers for Claude Code CLI with isolated, portable installations.

## Features

- 🔒 **Isolated Installations** - Each instance has its own Bun and Claude installation
- 🚀 **Multiple Instances** - Run multiple Claude relays concurrently on different ports
- 🔧 **Auto-Setup** - Automatically downloads and installs all dependencies
- 🔑 **Authentication Management** - Handle Claude authentication programmatically
- 🌐 **WebSocket Interface** - Connect to Claude via standard WebSocket protocol
- 💬 **Conversation State** - Maintains context across messages
- ↩️ **Undo/Restore** - Support for undoing and restoring conversations
- 🧵 **Thread-Safe** - Safe for concurrent use

## Installation

```bash
go get github.com/yourusername/claude-relay
```

## Quick Start

```go
package main

import (
    "log"
    clauderelay "github.com/yourusername/claude-relay"
)

func main() {
    // Create a new relay instance
    relay, err := clauderelay.New(clauderelay.Options{
        Port:          "8081",
        BaseDir:       "./claude",
        AutoSetup:     true,
        EnableLogging: true,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer relay.Close()

    // Start the server
    if err := relay.Start(); err != nil {
        log.Fatal(err)
    }

    log.Printf("Claude relay running at %s", relay.GetWebSocketURL())
    
    // Keep running
    select {}
}
```

## API Documentation

### Creating a Relay

```go
relay, err := clauderelay.New(clauderelay.Options{
    Port:             "8081",        // WebSocket server port
    BaseDir:          "./claude",    // Installation directory
    AutoSetup:        true,          // Auto-install Claude and Bun
    MaxProcesses:     100,           // Max concurrent Claude processes
    EnableLogging:    true,          // Enable debug logging
    CustomClaudePath: "",            // Optional custom Claude path
})
```

### Options

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Port` | string | "8080" | Port for WebSocket server |
| `BaseDir` | string | "." | Directory for Claude installation |
| `AutoSetup` | bool | true | Automatically install dependencies |
| `MaxProcesses` | int | 100 | Maximum concurrent Claude processes |
| `EnableLogging` | bool | true | Enable detailed logging |
| `CustomClaudePath` | string | "" | Use custom Claude executable |

### Methods

#### Server Control

```go
// Start the relay server
err := relay.Start()

// Stop the relay server
err := relay.Stop()

// Close and cleanup
err := relay.Close()

// Check if running
isRunning := relay.IsRunning()

// Wait for shutdown
relay.Wait()
```

#### Authentication

```go
// Check if Claude is authenticated
authenticated, err := relay.IsAuthenticated()

// Run interactive authentication
err := relay.Authenticate()
```

#### Information

```go
// Get WebSocket URL
url := relay.GetWebSocketURL() // "ws://localhost:8081/ws"

// Get port
port := relay.GetPort() // "8081"

// Get base directory
dir := relay.GetBaseDir() // "/absolute/path/to/claude"

// Check if installed
installed := relay.IsInstalled()
```

## Examples

### Basic Usage

```go
package main

import (
    "fmt"
    "log"
    "os"
    "os/signal"
    
    clauderelay "github.com/yourusername/claude-relay"
)

func main() {
    relay, err := clauderelay.New(clauderelay.Options{
        Port:    "8081",
        BaseDir: "./my-claude",
    })
    if err != nil {
        log.Fatal(err)
    }
    defer relay.Close()

    // Handle authentication if needed
    if auth, _ := relay.IsAuthenticated(); !auth {
        fmt.Println("Please authenticate Claude:")
        if err := relay.Authenticate(); err != nil {
            log.Fatal(err)
        }
    }

    if err := relay.Start(); err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Server running at %s\n", relay.GetWebSocketURL())

    // Wait for interrupt
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt)
    <-c
}
```

### Multiple Instances

```go
package main

import (
    "log"
    clauderelay "github.com/yourusername/claude-relay"
)

func main() {
    // Create multiple isolated instances
    instances := []struct {
        port string
        dir  string
    }{
        {"8081", "./claude-1"},
        {"8082", "./claude-2"},
        {"8083", "./claude-3"},
    }

    var relays []*clauderelay.Relay

    for _, inst := range instances {
        relay, err := clauderelay.New(clauderelay.Options{
            Port:    inst.port,
            BaseDir: inst.dir,
        })
        if err != nil {
            log.Printf("Failed to create instance: %v", err)
            continue
        }

        if err := relay.Start(); err != nil {
            log.Printf("Failed to start instance: %v", err)
            relay.Close()
            continue
        }

        relays = append(relays, relay)
        log.Printf("Instance running at %s", relay.GetWebSocketURL())
    }

    // Keep running...
    select {}
}
```

### Embedding in HTTP Server

```go
package main

import (
    "encoding/json"
    "net/http"
    
    clauderelay "github.com/yourusername/claude-relay"
)

type Server struct {
    relay *clauderelay.Relay
}

func (s *Server) statusHandler(w http.ResponseWriter, r *http.Request) {
    status := map[string]interface{}{
        "running": s.relay.IsRunning(),
        "url":     s.relay.GetWebSocketURL(),
    }
    json.NewEncoder(w).Encode(status)
}

func main() {
    relay, _ := clauderelay.New(clauderelay.Options{
        Port: "8081",
    })
    relay.Start()
    defer relay.Close()

    server := &Server{relay: relay}
    
    http.HandleFunc("/status", server.statusHandler)
    http.ListenAndServe(":8080", nil)
}
```

## WebSocket Protocol

### Client → Server Messages

- **Plain text**: Send message to Claude
- **`/undo-from:N`**: Undo from message index N
- **`/restore`**: Restore last undone conversation

### Server → Client Messages

- **Plain text**: Claude's response
- **Progress indicators**: Updates during processing
- **`UNDO_SUCCESS:N`**: Undo confirmation
- **`RESTORE_SUCCESS:[...]`**: Restore with message array
- **Error messages**: Prefixed with ❌

## Directory Structure

Each instance creates:

```
BaseDir/
├── .bun/                  # Portable Bun installation
│   └── bin/
│       ├── bun
│       └── claude
├── .claude-home/          # Claude configuration
│   └── .config/
│       └── claude/
│           └── auth.json
└── claude-relay-*/        # Temporary workspaces
```

## Authentication

The library provides multiple authentication methods for different deployment scenarios:

### Authentication Methods

#### 1. **SetAuthToken()** - Non-Interactive/Programmatic
Best for servers, containers, and headless environments:
```go
apiKey := os.Getenv("CLAUDE_API_KEY")
err := relay.SetAuthToken(apiKey)
```

#### 2. **Authenticate()** - Interactive/Terminal
For CLI tools and local development:
```go
err := relay.Authenticate()
// User sees Claude interface, types /login
```

#### 3. **Options.APIKey** - Automatic Setup
Provide API key at initialization:
```go
relay, _ := clauderelay.New(clauderelay.Options{
    Port:   "8081",
    APIKey: "sk-ant-...",  // Auto-authenticates
})
```

#### 4. **Options.AuthCallback** - Custom Flow
Implement your own authentication UI:
```go
relay, _ := clauderelay.New(clauderelay.Options{
    AuthCallback: func(authURL string) (string, error) {
        // Show URL in your UI
        // Get API key from user
        return apiKey, nil
    },
})
```

### When to Use Each Method

| Method | Use When |
|--------|----------|
| **SetAuthToken()** | • Web services/APIs<br>• Docker containers<br>• CI/CD pipelines<br>• Serverless (Lambda)<br>• Background jobs<br>• No terminal access |
| **Authenticate()** | • CLI tools<br>• Local development<br>• Terminal available<br>• User-managed auth |
| **Options.APIKey** | • API key in env/config<br>• Automated deployments<br>• No user interaction |
| **AuthCallback** | • Custom UI needed<br>• Web applications<br>• Mobile apps<br>• Special workflows |

### Checking Authentication

```go
// Simple check
authenticated, _ := relay.IsAuthenticated()

// Detailed status
authenticated, message, _ := relay.GetAuthStatus()
// Returns: (true, "Authenticated", nil)
// Or: (false, "No authentication file found", nil)

// Get auth URL for custom flows
url, _ := relay.GetAuthURL()
// Returns: "https://console.anthropic.com/settings/keys"
```

### Authentication Examples

**Environment Variable:**
```go
relay, _ := clauderelay.New(clauderelay.Options{
    APIKey: os.Getenv("CLAUDE_API_KEY"),
})
```

**Web Application:**
```go
relay, _ := clauderelay.New(clauderelay.Options{
    AuthCallback: func(authURL string) (string, error) {
        // Send authURL to frontend
        websocket.Send(authURL)
        // Wait for API key from frontend
        return <-apiKeyChan, nil
    },
})
```

**Docker/Kubernetes:**
```go
// Get from secret mount
apiKey, _ := ioutil.ReadFile("/secrets/claude-api-key")
relay.SetAuthToken(string(apiKey))
```

### Authentication Storage

Tokens are stored in: `BaseDir/.claude-home/.config/claude/auth.json`

Each instance maintains its own authentication, allowing multiple instances with different credentials.

## Thread Safety

The library is designed to be thread-safe:

- ✅ Multiple relays can run concurrently
- ✅ Safe to call methods from different goroutines
- ✅ Each instance is isolated

## Building from Source

```bash
# Clone the repository
git clone https://github.com/yourusername/claude-relay
cd claude-relay

# Build the library
go build

# Run tests
go test ./...

# Build the CLI tool
go build -o claude-relay ./cmd/claude-relay
```

## CLI Tool

The library includes a CLI tool:

```bash
# Install the CLI
go install github.com/yourusername/claude-relay/cmd/claude-relay@latest

# Run with defaults
claude-relay

# Custom configuration
claude-relay -port 9000 -dir ./my-claude -verbose
```

## Requirements

- Go 1.19 or higher
- Internet connection (for initial setup)
- macOS or Linux (Windows untested)

## Troubleshooting

### Authentication Issues

```go
// Reset authentication
os.RemoveAll(filepath.Join(relay.GetBaseDir(), ".claude-home"))
relay.Authenticate()
```

### Port Conflicts

```go
// Use a different port
relay, _ := clauderelay.New(clauderelay.Options{
    Port: "9090",
})
```

### Complete Reset

```bash
rm -rf ./claude-instance
# Restart your application
```

## License

[Your License]

## Contributing

Contributions welcome! Please read our [Contributing Guide](CONTRIBUTING.md).

## Support

- 📖 [Documentation](https://pkg.go.dev/github.com/yourusername/claude-relay)
- 🐛 [Issue Tracker](https://github.com/yourusername/claude-relay/issues)
- 💬 [Discussions](https://github.com/yourusername/claude-relay/discussions)