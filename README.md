# Claude Relay - WebSocket Server

A Go-based WebSocket server that provides an isolated Claude Code CLI environment with portable Bun installation.

## Features

- **Isolated Claude Installation**: Uses a portable Bun installation and isolated Claude instance in the project directory
- **Auto-Setup**: Automatically downloads and installs Bun and Claude on first run
- **Authentication Handling**: Guides you through Claude authentication when needed
- **Conversation Context**: Maintains conversation history using Claude's --print mode
- **Undo/Restore**: Per-message undo functionality with restore capability
- **Progress Indicators**: Shows thinking indicators while Claude processes requests
- **Web UI**: Clean, modern web interface with 120-character width support
- **Session Isolation**: Each connection gets its own temporary directory
- **Automatic Cleanup**: Processes and directories cleaned up on disconnect

## Installation

```bash
go mod download
go build -o claude-relay .
```

## First Time Setup

On first run, the application will:
1. Download and install portable Bun in `.bun/`
2. Install Claude Code CLI locally
3. Guide you through Claude authentication

Run the setup:
```bash
./claude-relay -port 8081
```

When Claude starts and shows the interactive prompt, type `/login` and follow the authentication instructions.

## Usage

After initial setup, simply run:
```bash
./claude-relay -port 8081
```

Then open `client.html` in your browser or navigate to http://localhost:8081

## Directory Structure

After setup, the following directories will be created:
- `.bun/` - Portable Bun installation
- `.claude-home/` - Isolated Claude configuration and authentication
- `claude-relay-*/` - Temporary directories for Claude workspace (created at runtime)

## Architecture

- **main.go**: Entry point, setup initialization, and HTTP server
- **setup.go**: Handles Bun and Claude installation, authentication
- **websocket.go**: WebSocket connection and message handling
- **process.go**: Claude process management and conversation state
- **config.go**: Configuration management
- **client.html**: Modern web UI with undo/restore functionality

## Features in Detail

### Isolated Environment
- No system-wide installation required
- All dependencies contained in project directory
- Separate Claude home directory for configuration
- Uses custom HOME environment for Claude

### Conversation Management
- Maintains conversation history across messages
- Uses Claude's --print mode with context injection
- Automatic history pruning (keeps last 10 exchanges)

### Undo/Restore System
- Click the ↶ button on any message to undo from that point
- Removes the selected message and all subsequent ones
- Restore button appears after undo operations
- Can restore the last undone conversation

### Progress Indicators
- "Claude is thinking..." initial indicator
- Progressive status updates during processing
- Visual pulse animation in the UI

## Configuration

Default port is 8080, customize with:
```bash
./claude-relay -port 3000
```

## WebSocket Protocol

### Client to Server
- Plain text messages sent to Claude
- `/undo-from:N` - Undo from message index N
- `/restore` - Restore last undone conversation

### Server to Client
- Claude responses as plain text
- Progress indicators (with emoji prefixes)
- `UNDO_SUCCESS:N` - Undo confirmation
- `RESTORE_SUCCESS:[...]` - Restore with message array
- Error messages prefixed with ❌

## Troubleshooting

### Authentication Issues
```bash
rm -rf .claude-home
./claude-relay -port 8081
# Complete login when prompted
```

### Port Already in Use
```bash
lsof -ti :8081 | xargs kill -9
```

### Reset Everything
```bash
rm -rf .bun .claude-home
./claude-relay -port 8081
```

### Check Installation
```bash
ls -la .bun/bin/       # Should show bun and claude
ls -la .claude-home/   # Should show .config/claude/
```

## Development

### Building
```bash
go build -o claude-relay .
```

### Testing
```bash
# Run server
./claude-relay -port 8081

# In another terminal
python test_ws.py
```

### Helper Scripts
- `setup.sh` - Interactive setup script
- `test.sh` - Run tests
- `Makefile` - Build and run commands

## Security Notes

- Each session isolated in temporary directory
- Processes killed and cleaned up on disconnect
- Authentication stored in isolated `.claude-home`
- CORS configured to accept any origin (customize as needed)
- Uses `--dangerously-skip-permissions` flag for Claude

## Requirements

- Go 1.19 or higher
- Internet connection for initial setup
- macOS or Linux (Windows not tested)

## License

[Your License Here]