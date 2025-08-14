# WebSocket Server Example

A complete WebSocket server implementation using the Claude Relay library, providing real-time communication with Claude CLI through a web interface.

## Features

- 🌐 WebSocket endpoint for real-time Claude communication
- 🔐 Built-in authentication flow through WebSocket
- 💬 Web UI with conversation management
- ↩️ Undo/restore conversation functionality  
- 📊 Progress indicators during Claude processing
- 🔒 Session isolation with temporary directories
- 🧹 Automatic cleanup on disconnect

## Running the Server

```bash
# Run with default port (8080)
go run .

# Run with custom port
go run . 8081

# Or build and run
go build -o websocket-server
./websocket-server 8081
```

## Accessing the Web UI

Open your browser to:
- http://localhost:8080 (default)
- http://localhost:PORT (if using custom port)

## WebSocket Protocol

### Client → Server Messages

- **Plain text**: Send message to Claude
- **`/undo-from:N`**: Undo from message index N
- **`/restore`**: Restore last undone conversation

### Server → Client Messages

- **Plain text**: Claude's response
- **Progress indicators**: Status updates with emoji prefixes
- **`UNDO_SUCCESS:N`**: Undo confirmation
- **`RESTORE_SUCCESS:[...]`**: Restore with message array
- **Error messages**: Prefixed with ❌

## Files

- `main.go` - Server application and relay orchestration
- `websocket.go` - WebSocket connection handler
- `client.html` - Web UI for interacting with Claude

## Architecture

The server creates isolated Claude instances for each WebSocket connection:

1. Client connects via WebSocket
2. Server spawns Claude process with isolated workspace
3. Messages are relayed between client and Claude
4. Conversation history maintained for context
5. Process and resources cleaned up on disconnect

## Authentication

On first run, if Claude is not authenticated:

1. Server will prompt for authentication
2. Choose a theme when Claude starts (press 1)
3. Type `/login` and follow browser instructions
4. Authentication persists for future sessions

## Customization

Edit `main.go` to customize:
- Port and base directory
- Maximum concurrent processes
- Logging verbosity
- Authentication handling

## Building a Standalone Binary

```bash
go build -o claude-websocket-server
./claude-websocket-server
```

## Integration

This example can be adapted for:
- Adding Claude chat to existing applications
- Building custom Claude interfaces
- Creating API endpoints for Claude
- Implementing team chat systems

## License

MIT - See repository LICENSE file