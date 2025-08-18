# Clay: OpenAI-Compatible Claude Server

Clay transforms Claude into an OpenAI-compatible API server, allowing you to use Claude with any OpenAI client library while adding powerful project-specific configuration.

## ⚡ Quick Start

### 1. Download Clay

**macOS:**
```bash
curl -L https://github.com/rizrmd/clay/releases/latest/download/clay-macos-x64 -o clay
chmod +x clay
```

**Linux:**
```bash
curl -L https://github.com/rizrmd/clay/releases/latest/download/clay-linux-x64 -o clay
chmod +x clay
```

### 2. First Run

Clay automatically configures everything for you:

```bash
# Start Clay server (zero configuration required)
./clay
```

On first run, Clay will:
- Generate a `clay.yaml` configuration file automatically
- Download and install a portable Claude CLI
- Guide you through Claude authentication 
- Start the OpenAI-compatible server on port 3000

### 3. Use with Any OpenAI Client

Once running, use Clay exactly like OpenAI's API:

```bash
curl -X POST http://localhost:3000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-sonnet",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

## 🎯 Why Clay?

- **Zero Configuration**: Just run `./clay` - no setup files needed
- **OpenAI Compatible**: Drop-in replacement for OpenAI API calls
- **Project Context**: Add custom context that Claude remembers
- **MCP Support**: Connect to Model Context Protocol servers
- **Isolated**: Portable installation, doesn't affect your system
- **Auto-Configuration**: Generates `clay.yaml` automatically and manages Claude CLI config files

## ⚙️ Project Configuration

Clay uses a single `clay.yaml` file to configure both Clay itself and the underlying Claude CLI. When you start Clay, it automatically generates Claude CLI's `config.json` and `mcp.json` files from your `clay.yaml` settings.

Clay automatically creates a `clay.yaml` file when it doesn't exist, so you can run it with zero configuration. However, you'll want to customize it for your project.

### Customizing Configuration

Clay generates a default `clay.yaml` automatically. To customize it for your project, simply edit the file that was created:

```yaml
# Define context that Claude will remember for every conversation
context: |
  You are an expert Python developer working on a data analysis project.
  The codebase uses pandas, numpy, and scikit-learn.
  Always suggest type hints and follow PEP 8 conventions.

# Connect to tools and databases via MCP servers
mcp:
  servers:
    # File system access
    filesystem:
      command: "npx"
      args: ["-y", "@modelcontextprotocol/server-filesystem", "."]
    
    # Connect to your API
    myapi:
      transport: "http"
      url: "http://localhost:8080/mcp"
      headers:
        Authorization: "Bearer ${API_TOKEN}"

# Server settings
server:
  port: 3000
  max_processes: 50
```

### Regenerate or Validate Configuration

Force regenerate the default configuration:

```bash
./clay --init-config
```

Check your configuration is correct:

```bash
./clay --validate-config
```

## 📋 Common Use Cases

### 1. Code Assistant for Your Project

```yaml
context: |
  You are working on a React TypeScript project with Next.js.
  
  Project structure:
  - /pages - Next.js pages
  - /components - Reusable React components  
  - /lib - Utility functions
  - /types - TypeScript type definitions
  
  Code style: Use functional components, prefer const assertions, 
  include proper JSDoc comments.
```

### 2. Database Query Helper

```yaml
context: |
  You are a PostgreSQL expert working with this database schema:
  
  Users table: id, email, created_at, subscription_type
  Posts table: id, user_id, title, content, published_at
  Comments table: id, post_id, user_id, content, created_at
  
  Always write optimized queries and explain the execution plan.

mcp:
  servers:
    database:
      command: "npx"
      args: ["-y", "@modelcontextprotocol/server-postgres"]
      env:
        DATABASE_URL: "${DATABASE_URL}"
```

### 3. Documentation Writer

```yaml
context: |
  You are a technical writer creating API documentation.
  
  Documentation standards:
  - Use OpenAPI 3.0 specification
  - Include request/response examples
  - Add error codes and descriptions
  - Write clear, concise descriptions
  - Include authentication requirements
```

## 🔧 Configuration Options

### Context Configuration

The `context` field lets you define instructions that Claude will remember:

```yaml
context: |
  Your role and expertise here.
  Project-specific information.
  Coding standards and preferences.
  Any other context Claude should know.
```

### MCP Server Types

**Command-based servers** (most common):
```yaml
mcp:
  servers:
    filesystem:
      command: "npx"
      args: ["-y", "@modelcontextprotocol/server-filesystem", "."]
      env:
        NODE_ENV: "production"
```

**HTTP servers** (for remote APIs):
```yaml
mcp:
  servers:
    myapi:
      transport: "http"
      url: "https://api.example.com/mcp"
      headers:
        Authorization: "Bearer ${API_TOKEN}"
      timeout: 30
```

**WebSocket servers** (for real-time data):
```yaml
mcp:
  servers:
    realtime:
      transport: "ws"  
      url: "ws://localhost:9000/mcp"
      reconnect: true
```

### Environment Variables

Reference environment variables in your configuration:

```yaml
mcp:
  servers:
    database:
      command: "npx"
      args: ["-y", "@modelcontextprotocol/server-postgres"]
      env:
        DATABASE_URL: "${DATABASE_URL}"  # Uses $DATABASE_URL from environment
```

## 🚀 Advanced Usage

### Custom Port

```bash
# Run on different port
./clay --port 8080

# Or set in clay.yaml
server:
  port: 8080
```

### Multiple Projects

Each project can have its own `clay.yaml`:

```bash
cd /path/to/project1
./clay --port 3000 &

cd /path/to/project2  
./clay --port 3001 &
```

### Status and Debugging

```bash
# Check installation and auth status
./clay --status

# Validate configuration without starting server
./clay --validate-config

# Send test message
./clay --message "Hello Claude!"
```

## 🔌 Integration Examples

### Python with OpenAI Library

```python
import openai

client = openai.OpenAI(
    base_url="http://localhost:3000/v1",
    api_key="not-required"  # Clay doesn't require API keys
)

response = client.chat.completions.create(
    model="claude-3-sonnet",
    messages=[{"role": "user", "content": "Help me debug this code"}]
)

print(response.choices[0].message.content)
```

### Node.js

```javascript
import OpenAI from 'openai';

const openai = new OpenAI({
  baseURL: 'http://localhost:3000/v1',
  apiKey: 'not-required'
});

const completion = await openai.chat.completions.create({
  model: 'claude-3-sonnet',
  messages: [{ role: 'user', content: 'Explain this algorithm' }],
});

console.log(completion.choices[0].message.content);
```

### Curl

```bash
curl -X POST http://localhost:3000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-sonnet",
    "messages": [
      {"role": "user", "content": "Write a Python function to calculate fibonacci"}
    ]
  }'
```

## 🛠️ How It Works

Clay creates a bridge between OpenAI-compatible clients and Claude CLI:

1. **Isolated Installation**: Downloads portable Claude CLI and Bun runtime
2. **Configuration Generation**: Reads `clay.yaml` and generates Claude CLI's `config.json` and `mcp.json` files
3. **Context Injection**: Adds your custom context to every conversation
4. **MCP Integration**: Connects Claude to external tools and data sources (proxies HTTP/WebSocket servers)
5. **API Translation**: Converts OpenAI API calls to Claude CLI commands
6. **Response Formatting**: Returns Claude responses in OpenAI format

Clay acts as a configuration layer on top of Claude CLI, automatically managing all the Claude-specific config files from your single `clay.yaml` file.

### Files Clay Manages

**Your files:**
- `clay.yaml` - Your project configuration (you edit this)

**Generated automatically by Clay:**
- `.claude-home/.config/claude/config.json` - Claude CLI's main configuration
- `.claude-home/.config/claude/mcp.json` - Claude CLI's MCP server configuration  
- `.claude-home/.config/claude/clay-mcp.json` - Clay's internal MCP configuration backup

You only need to edit `clay.yaml` - Clay handles the rest.

## 🆘 Troubleshooting

**Clay won't start:**
```bash
# Check status and authentication
./clay --status

# Reinstall Claude CLI
./clay --setup
```

**Configuration errors:**
```bash
# Validate your clay.yaml
./clay --validate-config

# Regenerate sample config
./clay --init-config
```

**Port conflicts:**
```bash
# Use different port
./clay --port 8080
```

## 📚 MCP Resources

- [MCP Server Directory](https://github.com/modelcontextprotocol/servers) - Official MCP servers
- [MCP Documentation](https://spec.modelcontextprotocol.io/) - Protocol specification
- [Claude MCP Guide](https://docs.anthropic.com/en/docs/build-with-claude/mcp) - Getting started with MCP

---

## For Rust Developers

If you're building Rust applications, you can use Clay as a library:

```toml
[dependencies]
clay = "0.1.0"
```

```rust
use clay::{ClaudeSetup, ClaudeProcess};
use std::sync::Arc;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let setup = Arc::new(ClaudeSetup::new(".")?);
    
    if !setup.is_installed() {
        setup.setup_with_mcp()?;
    }
    
    if !setup.check_authentication()? {
        setup.run_claude_login()?;
    }
    
    let mut process = ClaudeProcess::new(setup)?;
    let response = process.send_message("Hello Claude!")?;
    println!("Claude: {}", response);
    
    Ok(())
}
```