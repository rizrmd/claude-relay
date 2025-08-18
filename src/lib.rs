pub mod setup;
pub mod process;
pub mod config;
pub mod auth;
pub mod error;
pub mod server;

pub use setup::ClaudeSetup;
pub use process::{ClaudeProcess, ConversationState};
pub use config::Config;
pub use error::{ClaudeRelayError, Result};
pub use server::start_server;

pub fn new(base_dir: &str) -> Result<ClaudeSetup> {
    ClaudeSetup::new(base_dir)
}

pub fn new_claude_setup_with_base_dir(base_dir: &str) -> Result<ClaudeSetup> {
    ClaudeSetup::new(base_dir)
}