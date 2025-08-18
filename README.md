# Claude Relay (Rust)

A Rust library for communicating with Claude CLI, providing isolated installations and process management. This is a Rust port of the original [Go implementation](https://github.com/rizrmd/claude-relay).

## Features

- **Isolated Claude CLI Installation**: Manages portable Bun and Claude CLI installations
- **Process Management**: Spawns and controls Claude CLI processes with conversation state
- **Authentication Management**: Handles Claude CLI browser-based authentication
- **Conversation History**: Maintains context across messages with undo/restore functionality
- **Cross-platform**: Supports macOS and Linux (x64 and ARM64)

## Installation

Add to your `Cargo.toml`:

```toml
[dependencies]
claude-relay = "0.1.0"
```

## Library Usage

```rust
use claude_relay::{ClaudeSetup, ClaudeProcess};
use std::sync::Arc;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Initialize Claude setup
    let setup = Arc::new(ClaudeSetup::new(".")?);
    
    // Install Claude CLI if needed
    if !setup.is_installed() {
        setup.setup()?;
    }
    
    // Check and handle authentication
    if !setup.check_authentication()? {
        println!("Please authenticate with Claude:");
        setup.run_claude_login()?;
    }
    
    // Create Claude process
    let mut process = ClaudeProcess::new(setup)?;
    
    // Send message to Claude
    let response = process.send_message("Hello Claude!")?;
    println!("Claude: {}", response);
    
    Ok(())
}
```

## CLI Tool

The crate includes a CLI tool for direct interaction:

```bash
# Install the CLI
cargo install claude-relay

# Send a message (automatically installs Claude CLI and prompts for auth if needed)
claude-relay --message "What is 2+2?"

# Show status (automatically installs Claude CLI if needed)
claude-relay

# Manual setup (optional - done automatically when needed)
claude-relay --setup
```

## Core API

### Setup and Installation

```rust
// Create setup manager
let setup = ClaudeSetup::new(".")?;
let setup = ClaudeSetup::new("/path/to/base/dir")?;

// Install Claude CLI and dependencies
setup.setup()?;

// Check if installed
let installed = setup.is_installed();

// Get installation paths
let claude_path = setup.get_claude_path();
let claude_home = setup.get_claude_home();
```

### Authentication

```rust
// Check authentication status
let authenticated = setup.check_authentication()?;

// Run interactive authentication (requires terminal and browser)
setup.run_claude_login()?;

// Copy authentication from another installation
setup.copy_auth_from(Path::new("/path/to/auth/dir"))?;

// Set authentication token programmatically
setup.set_auth_token("session_token")?;
```

### Process Management

```rust
// Create Claude process
let mut process = ClaudeProcess::new(Arc::new(setup))?;

// Send message and get response
let response = process.send_message("Your message here")?;

// Send message with progress callback (async)
let response = process.send_message_with_progress(
    "Message",
    |progress| println!("{}", progress)
).await?;

// Undo/restore conversation
process.undo_last_exchange()?;
let can_restore = process.can_restore();
if can_restore {
    let restored = process.restore_last_undo()?;
}
```

### Configuration

```rust
use claude_relay::Config;

// Load configuration
let config = Config::load("config.json")?;

// Create default configuration
let config = Config::default();

// Save configuration
config.save("config.json")?;
```

## Architecture Differences from Go Version

- **Memory Safety**: Rust's ownership system ensures automatic cleanup without manual management
- **Error Handling**: Uses `Result<T, E>` types for explicit error handling
- **Async Support**: Built-in async/await support with Tokio runtime
- **Type Safety**: Strong typing prevents runtime errors
- **Dependencies**: Uses Rust ecosystem equivalents (reqwest for HTTP, zip for archives, etc.)

## Requirements

- Rust 1.70 or higher
- Internet connection for initial Claude CLI setup
- macOS or Linux (Windows not tested)
- Browser access for initial authentication

## Development

```bash
# Build the project
cargo build

# Run tests
cargo test

# Run with verbose logging
RUST_LOG=debug cargo run -- --setup
```

## License

MIT License - See LICENSE file for details