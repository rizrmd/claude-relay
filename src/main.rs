use anyhow::Result;
use clap::Parser;
use clay::{ClaudeProcess, ClaudeSetup, start_server};
use std::sync::Arc;
use tracing_subscriber::EnvFilter;

#[derive(Parser, Debug)]
#[command(name = "claude-relay")]
#[command(about = "Claude Relay - OpenAI-compatible API server for Claude CLI", long_about = None)]
struct Args {
    #[arg(short, long, default_value = ".")]
    dir: String,
    
    #[arg(short, long, help = "Port to run the server on")]
    port: Option<u16>,
    
    #[arg(long, help = "Run setup to install Claude CLI")]
    setup: bool,
    
    #[arg(short, long, help = "Send a message to Claude")]
    message: Option<String>,
    
    #[arg(long, help = "Show status instead of starting server")]
    status: bool,
    
    #[arg(long, help = "Force regenerate clay.yaml configuration file")]
    init_config: bool,
    
    #[arg(long, help = "Path to clay.yaml configuration file")]
    config: Option<String>,
    
    #[arg(long, help = "Validate clay.yaml configuration")]
    validate_config: bool,
}

#[tokio::main]
async fn main() -> Result<()> {
    // Initialize tracing
    tracing_subscriber::fmt()
        .with_env_filter(
            EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| EnvFilter::new("info"))
        )
        .init();
    
    let args = Args::parse();
    
    // Handle init-config command (force regenerate clay.yaml)
    if args.init_config {
        let claude_setup = Arc::new(ClaudeSetup::new(&args.dir)?);
        claude_setup.init_config()?;
        return Ok(());
    }
    
    // Create Claude setup (this will load clay.yaml if present)
    let claude_setup = Arc::new(ClaudeSetup::new(&args.dir)?);
    
    // Handle config validation
    if args.validate_config {
        println!("Validating clay.yaml configuration...");
        let issues = claude_setup.validate_mcp_servers()?;
        if issues.is_empty() {
            println!("✅ Configuration is valid!");
            if let Some(config) = claude_setup.get_config() {
                if let Some(context) = &config.context {
                    println!("📝 Initial context configured ({} characters)", context.len());
                }
                if let Some(mcp) = &config.mcp {
                    println!("🔗 {} MCP server(s) configured", mcp.servers.len());
                    for (name, server) in &mcp.servers {
                        if server.is_command() {
                            if let Some(command) = &server.command {
                                println!("  - {} (command: {})", name, command);
                            }
                        } else if server.is_http() {
                            if let Some(url) = &server.url {
                                println!("  - {} (HTTP: {})", name, url);
                            }
                        } else if server.is_websocket() {
                            if let Some(url) = &server.url {
                                println!("  - {} (WebSocket: {})", name, url);
                            }
                        }
                    }
                }
            }
        } else {
            println!("❌ Configuration issues found:");
            for issue in &issues {
                println!("  - {}", issue);
            }
        }
        return Ok(());
    }
    
    // Run setup if requested (use enhanced MCP setup)
    if args.setup {
        println!("Setting up Claude CLI with MCP support...");
        claude_setup.setup_with_mcp().await?;
        println!("Claude CLI installed successfully!");
        return Ok(());
    }
    
    // Check if Claude is installed and install automatically if needed
    if !claude_setup.is_installed() {
        println!("Claude CLI is not installed. Installing automatically...");
        claude_setup.setup_with_mcp().await?;
        println!("Claude CLI installed successfully!");
    }
    
    // Send message if provided
    if let Some(message) = args.message {
        // Check authentication and prompt if needed
        if !claude_setup.check_authentication()? {
            claude_setup.complete_oauth_flow()?;
        }
        
        let mut process = ClaudeProcess::new(claude_setup.clone())?;
        let response = process.send_message(&message)?;
        println!("{}", response);
        return Ok(());
    }
    
    // Status mode
    if args.status {
        let authenticated = claude_setup.check_authentication()?;
        
        println!("Claude Relay Status:");
        println!("  Installation directory: {}", args.dir);
        println!("  Claude installed: {}", claude_setup.is_installed());
        println!("  Authenticated: {}", authenticated);
        
        if !authenticated {
            println!();
            match claude_setup.complete_oauth_flow() {
                Ok(()) => {
                    println!("Authentication complete! You can now start the server.");
                }
                Err(e) => {
                    eprintln!("Authentication failed: {}", e);
                    eprintln!("You can try again by running the command again.");
                    std::process::exit(1);
                }
            }
        } else {
            println!();
            println!("Ready to start server!");
        }
        
        return Ok(());
    }
    
    // Default behavior: Start the OpenAI-compatible server
    // Check authentication first
    if !claude_setup.check_authentication()? {
        println!("Authentication required before starting server.");
        claude_setup.complete_oauth_flow()?;
        println!("Authentication complete!");
    }
    
    println!("Starting Claude Relay OpenAI-compatible API server...");
    
    // Determine port from clay.yaml config or CLI argument
    let port = if let Some(cli_port) = args.port {
        cli_port
    } else if let Some(config) = claude_setup.get_config() {
        if let Some(server_config) = &config.server {
            server_config.port
        } else {
            3000 // Default port
        }
    } else {
        3000 // Default port
    };
    
    start_server(claude_setup, port).await?;
    
    Ok(())
}
