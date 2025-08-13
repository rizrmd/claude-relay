# Claude Relay

A Go library for communicating with Claude CLI and a WebSocket server application that uses it.

## Library: Claude CLI Communication

The core library provides isolated Claude CLI management without any server components:

```go
import "github.com/rizrmd/claude-relay"
```

### Core Features

- **Isolated Installation**: Manages portable Bun and Claude CLI installations
- **Process Management**: Spawns and controls Claude CLI processes  
- **Authentication**: Handles Claude CLI authentication (browser-based, not API keys)
- **Conversation State**: Maintains conversation history using --print mode

### Library API

```go
// Create Claude setup manager
setup, err := clauderelay.NewClaudeSetup()

// Install Claude CLI and dependencies
err := setup.Setup()

// Check authentication
authenticated, err := setup.CheckAuthentication()

// Run interactive authentication
err := setup.RunClaudeLogin()

// Create and manage Claude process
process := clauderelay.NewClaudeProcess(config, setup)
response, err := process.SendMessage(ctx, "Hello Claude")
```

### What the Library Does NOT Include

- No WebSocket server
- No HTTP endpoints
- No web UI
- No network protocols

The library is purely for managing Claude CLI processes locally.

## Application: WebSocket Server

The repository includes a WebSocket server application built on top of the library:

### Running the Server

```bash
# Build and run
go build -o claude-relay ./cmd/claude-relay
./claude-relay -port 8081

# Or use go run
go run ./cmd/claude-relay/main.go -port 8081
```

### Server Features

- WebSocket endpoint at `/ws` for Claude communication
- Web UI at `client.html`
- Conversation management with undo/restore
- Progress indicators during processing
- Session isolation with temp directories

### WebSocket Protocol

**Client → Server:**
- Plain text messages for Claude
- `/undo-from:N` - Undo from message N
- `/restore` - Restore undone messages

**Server → Client:**
- Claude responses as text
- Progress indicators with emoji prefixes
- `UNDO_SUCCESS:N` - Undo confirmation
- `RESTORE_SUCCESS:[...]` - Restore with messages
- Error messages with ❌ prefix

## Installation

### Using as a Library

```bash
go get github.com/rizrmd/claude-relay
```

```go
package main

import (
    "context"
    "log"
    "github.com/rizrmd/claude-relay"
)

func main() {
    // Setup Claude CLI
    setup, err := clauderelay.NewClaudeSetup()
    if err != nil {
        log.Fatal(err)
    }
    
    // Install if needed
    if !setup.IsInstalled() {
        if err := setup.Setup(); err != nil {
            log.Fatal(err)
        }
    }
    
    // Check authentication
    if authenticated, _ := setup.CheckAuthentication(); !authenticated {
        log.Println("Please authenticate:")
        if err := setup.RunClaudeLogin(); err != nil {
            log.Fatal(err)
        }
    }
    
    // Create process manager
    config := &clauderelay.Config{
        ClaudePath: setup.GetClaudePath(),
    }
    process := clauderelay.NewClaudeProcess(config, setup)
    
    // Send message
    ctx := context.Background()
    response, err := process.SendMessage(ctx, "Hello Claude")
    if err != nil {
        log.Fatal(err)
    }
    log.Println("Claude:", response)
}
```

### Running the WebSocket Server

```bash
# Clone repository
git clone https://github.com/rizrmd/claude-relay
cd claude-relay

# Build server
go build -o claude-relay ./cmd/claude-relay

# First run (will install Claude CLI)
./claude-relay -port 8081

# When prompted, type /login to authenticate
# Then open client.html in browser or visit http://localhost:8081
```

## Directory Structure

```
claude-relay/
├── setup.go              # Claude CLI installation and auth
├── process.go            # Claude process management
├── config.go             # Configuration structures
├── websocket.go          # WebSocket server (application)
├── relay.go              # Server orchestration (application)
├── cmd/
│   └── claude-relay/
│       └── main.go       # CLI application
├── examples/
│   ├── basic/            # Basic library usage
│   ├── multiple/         # Multiple instances
│   └── embedded/         # Embedding in HTTP server
└── client.html           # Web UI for WebSocket server
```

## Authentication

Claude CLI uses browser-based authentication, NOT API keys:

1. User runs `/login` command in Claude CLI
2. Browser opens for Anthropic account login  
3. Token saved in `.claude-home/.config/claude/auth.json`

### Interactive Authentication

```go
setup := clauderelay.NewClaudeSetup()
err := setup.RunClaudeLogin()
// User completes browser auth
```

### Pre-configured Authentication

For headless environments, authenticate on a machine with browser access first:

```bash
# On development machine
./claude-relay -port 8081
# Complete /login process

# Backup auth files
tar -czf claude-auth.tar.gz .claude-home/.config/claude/
```

Then restore on production:

```go
setup := clauderelay.NewClaudeSetup()
err := setup.CopyAuthFrom("/path/to/claude-auth/")
```

## Examples

### Library Usage: Simple Claude CLI Communication

```go
package main

import (
    "context"
    "fmt"
    "github.com/rizrmd/claude-relay"
)

func main() {
    setup, _ := clauderelay.NewClaudeSetup()
    
    if !setup.IsInstalled() {
        setup.Setup()
    }
    
    config := &clauderelay.Config{
        ClaudePath: setup.GetClaudePath(),
    }
    
    process := clauderelay.NewClaudeProcess(config, setup)
    ctx := context.Background()
    
    response, _ := process.SendMessage(ctx, "What is 2+2?")
    fmt.Println("Claude:", response)
}
```

### Server Usage: Multiple Isolated Instances

```go
package main

import (
    "log"
    "github.com/rizrmd/claude-relay"
)

func main() {
    // Each instance has isolated Claude installation
    instances := []struct {
        port string
        dir  string
    }{
        {"8081", "./claude-1"},
        {"8082", "./claude-2"},
    }
    
    for _, inst := range instances {
        relay, _ := clauderelay.New(clauderelay.Options{
            Port:    inst.port,
            BaseDir: inst.dir,
        })
        relay.Start()
        defer relay.Close()
        
        log.Printf("Instance at ws://localhost:%s/ws", inst.port)
    }
    
    select {} // Keep running
}
```

## Development

```bash
# Run tests
go test ./...

# Build library
go build

# Build server application
go build -o claude-relay ./cmd/claude-relay

# Run examples
go run examples/basic/main.go
```

## Troubleshooting

### Reset Authentication
```bash
rm -rf .claude-home
./claude-relay -port 8081
# Complete /login when prompted
```

### Port Already in Use
```bash
lsof -ti :8081 | xargs kill -9
```

### Check Installation
```bash
ls -la .bun/bin/       # Should show bun and claude
ls -la .claude-home/   # Should show .config/claude/
```

## Security Notes

- Authentication tokens stored in `.claude-home/.config/claude/auth.json`
- Each instance isolated in its own directory
- Never commit auth.json to version control
- WebSocket server accepts connections from any origin (customize as needed)

## Requirements

- Go 1.19 or higher
- Internet connection for initial Claude CLI setup
- macOS or Linux (Windows not tested)
- Browser access for initial authentication

## License

MIT License - see [LICENSE](LICENSE) file

Copyright (c) 2024 rizrmd
