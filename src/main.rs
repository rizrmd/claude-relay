use anyhow::Result;
use clap::Parser;
use claude_relay::{ClaudeProcess, ClaudeSetup, start_server};
use std::sync::Arc;
use tracing_subscriber::EnvFilter;

#[derive(Parser, Debug)]
#[command(name = "claude-relay")]
#[command(about = "Claude Relay - OpenAI-compatible API server for Claude CLI", long_about = None)]
struct Args {
    #[arg(short, long, default_value = ".")]
    dir: String,
    
    #[arg(short, long, default_value = "3000", help = "Port to run the server on")]
    port: u16,
    
    #[arg(long, help = "Run setup to install Claude CLI")]
    setup: bool,
    
    #[arg(short, long, help = "Send a message to Claude")]
    message: Option<String>,
    
    #[arg(long, help = "Show status instead of starting server")]
    status: bool,
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
    
    // Create Claude setup
    let claude_setup = Arc::new(ClaudeSetup::new(&args.dir)?);
    
    // Run setup if requested
    if args.setup {
        println!("Setting up Claude CLI...");
        claude_setup.setup()?;
        println!("Claude CLI installed successfully!");
        return Ok(());
    }
    
    // Check if Claude is installed and install automatically if needed
    if !claude_setup.is_installed() {
        println!("Claude CLI is not installed. Installing automatically...");
        claude_setup.setup()?;
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
    start_server(claude_setup, args.port).await?;
    
    Ok(())
}
