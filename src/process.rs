use crate::error::{ClaudeRelayError, Result};
use crate::setup::ClaudeSetup;
use chrono::{DateTime, Utc};
use std::fs;
use std::io::Write;
use std::path::Path;
use std::process::{Command, Stdio};
use std::sync::Arc;
use tempfile::TempDir;

#[derive(Clone, Debug)]
pub struct ConversationState {
    pub history: Vec<String>,
    pub timestamp: DateTime<Utc>,
}

pub struct ClaudeProcess {
    temp_dir: TempDir,
    conversation_history: Vec<String>,
    conversation_states: Vec<ConversationState>,
    last_undone_history: Option<Vec<String>>,
    setup: Arc<ClaudeSetup>,
}

impl ClaudeProcess {
    pub fn new(setup: Arc<ClaudeSetup>) -> Result<Self> {
        // Ensure config file exists to skip welcome
        let config_dir = setup.get_claude_home().join(".config").join("claude");
        fs::create_dir_all(&config_dir)?;
        
        let config_file = config_dir.join("config.json");
        if !config_file.exists() {
            let config = r#"{"theme":"dark","outputStyle":"default"}"#;
            fs::write(&config_file, config)?;
        }

        let temp_dir = TempDir::new()
            .map_err(|e| ClaudeRelayError::Process(format!("Failed to create temp directory: {}", e)))?;

        Ok(ClaudeProcess {
            temp_dir,
            conversation_history: Vec::new(),
            conversation_states: Vec::new(),
            last_undone_history: None,
            setup,
        })
    }

    pub fn get_working_directory(&self) -> &Path {
        self.temp_dir.path()
    }

    pub fn save_file(&self, filename: &str, content: &[u8]) -> Result<()> {
        let file_path = self.temp_dir.path().join(filename);
        fs::write(&file_path, content)?;
        Ok(())
    }

    pub fn read_file(&self, filename: &str) -> Result<Vec<u8>> {
        let file_path = self.temp_dir.path().join(filename);
        let content = fs::read(&file_path)?;
        Ok(content)
    }

    pub fn send_message(&mut self, message: &str) -> Result<String> {
        // Add user message to history
        self.conversation_history.push(format!("User: {}", message));
        
        // Build context from conversation history
        let full_prompt = if self.conversation_history.len() > 1 {
            let mut context = String::from("Previous conversation:\n");
            for msg in &self.conversation_history[..self.conversation_history.len() - 1] {
                context.push_str(msg);
                context.push('\n');
            }
            context.push_str("\nLatest message: ");
            context.push_str(message);
            context
        } else {
            message.to_string()
        };
        
        // Use claude --print mode for this single request
        let mut cmd = Command::new(self.setup.get_claude_path());
        cmd.args(&["--print", "--dangerously-skip-permissions"])
            .current_dir(self.setup.get_base_dir())
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::piped());
        
        // Set environment
        for (key, value) in self.setup.get_claude_env() {
            cmd.env(key, value);
        }
        
        // Add additional environment for relay
        cmd.env("CLAUDE_RELAY", "true")
            .env("TERM", "dumb")
            .env("NO_COLOR", "1");
        
        let mut child = cmd.spawn()
            .map_err(|e| ClaudeRelayError::Process(format!("Failed to spawn Claude: {}", e)))?;
        
        // Write prompt to stdin
        if let Some(mut stdin) = child.stdin.take() {
            stdin.write_all(full_prompt.as_bytes())
                .map_err(|e| ClaudeRelayError::Process(format!("Failed to write to stdin: {}", e)))?;
        }
        
        let output = child.wait_with_output()
            .map_err(|e| ClaudeRelayError::Process(format!("Claude command failed: {}", e)))?;
        
        if !output.status.success() {
            let stderr = String::from_utf8_lossy(&output.stderr);
            
            if self.setup.is_authentication_needed(&stderr) {
                return Err(ClaudeRelayError::Authentication(
                    "Authentication required: please restart the server to login".into()
                ));
            }
            return Err(ClaudeRelayError::Process(
                format!("Claude command failed: {}", stderr)
            ));
        }
        
        let response = String::from_utf8_lossy(&output.stdout).to_string();
        
        // Add Claude's response to history
        self.conversation_history.push(format!("Claude: {}", response));
        
        // Keep history manageable (last 10 exchanges)
        if self.conversation_history.len() > 20 {
            self.conversation_history.drain(0..2);
        }
        
        Ok(response)
    }

    pub async fn send_message_with_progress<F>(
        &mut self,
        message: &str,
        _progress_callback: F,
    ) -> Result<String> 
    where
        F: FnMut(&str),
    {
        // Save current state before processing (for undo functionality)
        self.save_state();
        
        // Send progress updates
        let messages = [
            "💭 Processing your request...",
            "🔍 Analyzing context...",
            "📖 Gathering information...",
            "🧠 Formulating response...",
        ];
        
        // Start a task to send progress updates
        let (tx, mut rx) = tokio::sync::mpsc::channel(1);
        let progress_task = tokio::spawn(async move {
            let mut index = 0;
            let mut interval = tokio::time::interval(tokio::time::Duration::from_secs(2));
            
            loop {
                tokio::select! {
                    _ = interval.tick() => {
                        if index < messages.len() {
                            if tx.send(messages[index]).await.is_err() {
                                break;
                            }
                            index += 1;
                        }
                    }
                    _ = rx.recv() => {
                        break;
                    }
                }
            }
        });
        
        // Send the actual message
        let result = self.send_message(message);
        
        // Stop progress updates
        drop(progress_task);
        
        result
    }

    pub fn save_state(&mut self) {
        let state = ConversationState {
            history: self.conversation_history.clone(),
            timestamp: Utc::now(),
        };
        
        self.conversation_states.push(state);
        
        // Keep only last 10 states to manage memory
        if self.conversation_states.len() > 10 {
            self.conversation_states.remove(0);
        }
    }

    pub fn undo_last_exchange(&mut self) -> Result<()> {
        if self.conversation_states.is_empty() {
            return Err(ClaudeRelayError::Process("No conversation states to undo".into()));
        }
        
        // Get the last saved state
        let last_state = self.conversation_states.pop().unwrap();
        
        // Restore conversation history to that state
        self.conversation_history = last_state.history;
        
        Ok(())
    }

    pub fn undo_to_index(&mut self, message_index: usize) -> Result<()> {
        // Calculate which conversation history index this corresponds to
        // Each exchange has 2 entries (User: and Claude:)
        let history_index = message_index * 2;
        
        if history_index > self.conversation_history.len() {
            return Err(ClaudeRelayError::Process(
                format!("Invalid undo index: {}", message_index)
            ));
        }
        
        // Save the conversation that will be undone (for restore)
        if history_index < self.conversation_history.len() {
            self.last_undone_history = Some(self.conversation_history.clone());
        }
        
        // Truncate conversation history to this point
        self.conversation_history.truncate(history_index);
        
        // Also truncate conversation states if needed
        self.conversation_states.retain(|state| state.history.len() <= history_index);
        
        Ok(())
    }

    pub fn can_undo(&self) -> bool {
        !self.conversation_states.is_empty()
    }

    pub fn get_last_exchange(&self) -> Result<(String, String)> {
        if self.conversation_history.len() < 2 {
            return Err(ClaudeRelayError::Process("No complete exchange to return".into()));
        }
        
        let user_msg = &self.conversation_history[self.conversation_history.len() - 2];
        let claude_msg = &self.conversation_history[self.conversation_history.len() - 1];
        
        let user_msg = user_msg.strip_prefix("User: ").unwrap_or(user_msg);
        let claude_msg = claude_msg.strip_prefix("Claude: ").unwrap_or(claude_msg);
        
        Ok((user_msg.to_string(), claude_msg.to_string()))
    }

    pub fn can_restore(&self) -> bool {
        self.last_undone_history.as_ref()
            .map(|history| history.len() > self.conversation_history.len())
            .unwrap_or(false)
    }

    pub fn restore_last_undo(&mut self) -> Result<Vec<String>> {
        if !self.can_restore() {
            return Err(ClaudeRelayError::Process("Nothing to restore".into()));
        }
        
        let last_undone = self.last_undone_history.as_ref().unwrap();
        
        // Get the messages that will be restored (for client display)
        let restored_messages = last_undone[self.conversation_history.len()..].to_vec();
        
        // Restore the conversation history
        self.conversation_history = last_undone.clone();
        
        // Clear the undo buffer since we've restored it
        self.last_undone_history = None;
        
        // Rebuild conversation states
        self.save_state();
        
        Ok(restored_messages)
    }

    pub fn get_restored_messages_for_client(&self) -> Vec<(String, String)> {
        if !self.can_restore() {
            return Vec::new();
        }
        
        let last_undone = self.last_undone_history.as_ref().unwrap();
        let restored_part = &last_undone[self.conversation_history.len()..];
        
        let mut messages = Vec::new();
        for chunk in restored_part.chunks(2) {
            if chunk.len() == 2 {
                let user_msg = chunk[0].strip_prefix("User: ").unwrap_or(&chunk[0]);
                let claude_msg = chunk[1].strip_prefix("Claude: ").unwrap_or(&chunk[1]);
                messages.push((user_msg.to_string(), claude_msg.to_string()));
            }
        }
        
        messages
    }
}