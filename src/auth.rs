use crate::error::{ClaudeRelayError, Result};
use crate::setup::ClaudeSetup;
use std::fs;
use std::time::Duration;
use tracing::info;

impl ClaudeSetup {
    pub fn get_setup_token_instructions(&self) -> String {
        format!("Run this command to authenticate:\n{} setup-token", 
                self.get_claude_path().display())
    }
    
    pub fn complete_auth(&self, session_token: &str) -> Result<()> {
        if session_token.is_empty() {
            return Err(ClaudeRelayError::Authentication("Session token cannot be empty".into()));
        }
        
        // Clean the token - remove everything after # if present
        let clean_token = session_token.split('#').next().unwrap_or(session_token).trim();
        
        if clean_token.is_empty() {
            return Err(ClaudeRelayError::Authentication("Token is empty after cleaning".into()));
        }
        
        // Ensure config directory exists
        let config_dir = self.get_claude_home().join(".config").join("claude");
        fs::create_dir_all(&config_dir)?;
        
        // Try multiple auth formats to match Claude CLI expectations
        let auth_formats = vec![
            // Format 1: Standard session format
            format!(r#"{{"sessionKey":"{}"}}"#, clean_token),
            // Format 2: OAuth account format  
            format!(r#"{{"oauthAccount":{{"sessionKey":"{}"}}}}"#, clean_token),
            // Format 3: Simple key format
            format!(r#"{{"key":"{}"}}"#, clean_token),
            // Format 4: Token format
            format!(r#"{{"token":"{}","type":"session"}}"#, clean_token),
        ];
        
        // Try each format until one works
        for (i, auth_data) in auth_formats.iter().enumerate() {
            // Write to auth.json
            let auth_file = config_dir.join("auth.json");
            fs::write(&auth_file, auth_data)?;
            
            // Also try writing to .claude.json (where check_authentication looks)
            let claude_file = self.get_claude_home().join(".claude.json");
            fs::write(&claude_file, auth_data)?;
            
            // Check if authentication worked
            if self.check_authentication()? {
                info!("Authentication completed successfully with format {}", i + 1);
                return Ok(());
            }
        }
        
        // If none worked, return error with helpful info
        Err(ClaudeRelayError::Authentication(
            format!("Authentication failed with token: {} (tried multiple formats)", clean_token)
        ))
    }
    
    pub fn get_auth_url(&self) -> String {
        // Try to capture the auth URL using PTY
        self.capture_setup_token_output()
            .unwrap_or_else(|| format!("Run: {} setup-token", self.get_claude_path().display()))
    }
    
    pub fn get_auth_instructions(&self) -> String {
        let url = self.get_auth_url();
        if url.starts_with("http") {
            format!("Authentication URL: {}", url)
        } else {
            format!("For authentication URL and instructions, run: {}", url)
        }
    }
    
    pub fn complete_oauth_flow(&self) -> Result<()> {
        println!("Authentication required. Please complete the following steps:");
        
        // Get the auth URL (browser may open automatically)
        let auth_url = self.get_auth_url();
        if auth_url.starts_with("http") {
            println!("1. Visit this URL in your browser: {}", auth_url);
            println!("   (Browser may have opened automatically)");
        } else {
            println!("1. Run: {}", auth_url);
            println!("   Then visit the URL shown");
        }
        
        println!("2. Complete the authentication process");
        println!("3. Copy the authorization code you receive");
        
        let code = prompt_user("\nPaste the authorization code here: ");
        if !code.trim().is_empty() {
            self.complete_auth(code.trim())?;
            println!("✅ Authentication completed successfully!");
        } else {
            return Err(ClaudeRelayError::Authentication("No authentication code provided".into()));
        }
        
        Ok(())
    }
    
    
    fn capture_setup_token_output(&self) -> Option<String> {
        use portable_pty::{native_pty_system, PtySize, CommandBuilder};
        use std::io::Read;
        use std::time::Duration;
        
        let pty_system = native_pty_system();
        let pty_pair = pty_system.openpty(PtySize {
            rows: 24,
            cols: 80,
            pixel_width: 0,
            pixel_height: 0,
        }).ok()?;
        
        let mut cmd = CommandBuilder::new(self.get_claude_path());
        cmd.arg("setup-token");
        
        // Set environment to prevent browser opening
        for (key, value) in self.get_claude_env() {
            cmd.env(key, value);
        }
        cmd.env("NO_BROWSER", "1");
        cmd.env("CLAUDE_NO_BROWSER", "1");
        
        let mut child = pty_pair.slave.spawn_command(cmd).ok()?;
        drop(pty_pair.slave);
        
        // Read all output with timeout using raw read
        let mut reader = pty_pair.master.try_clone_reader().ok()?;
        let mut output = String::new();
        let mut buffer = [0u8; 1024];
        
        // Give it time to output the URL - longer timeout since it needs to open browser
        for _ in 0..100 { // 10 seconds total, 100ms intervals
            std::thread::sleep(Duration::from_millis(100));
            
            match reader.read(&mut buffer) {
                Ok(0) => break, // EOF
                Ok(n) => {
                    let text = String::from_utf8_lossy(&buffer[..n]);
                    output.push_str(&text);
                    
                    // Check if we have a URL now
                    if let Some(url) = self.extract_url_from_text(&output) {
                        let _ = child.kill();
                        return Some(url);
                    }
                }
                Err(_) => {
                    // No data available yet, continue
                    continue;
                }
            }
        }
        
        // Kill the child process
        let _ = child.kill();
        
        // Final attempt to extract URL from all output
        self.extract_url_from_text(&output)
    }
    
    fn extract_url_from_line(&self, line: &str) -> Option<String> {
        // Remove ANSI escape codes first
        let clean_line = Self::strip_ansi_codes(line);
        let clean_line = clean_line.trim();
        
        // Look for URLs in the cleaned line
        if clean_line.starts_with("http") && (clean_line.contains("claude") || clean_line.contains("anthropic")) {
            return Some(clean_line.to_string());
        }
        
        // Look for URLs anywhere in the line
        for word in clean_line.split_whitespace() {
            if word.starts_with("http") && (word.contains("claude") || word.contains("anthropic")) {
                return Some(word.to_string());
            }
        }
        
        None
    }
    
    fn strip_ansi_codes(text: &str) -> String {
        // Simple ANSI escape code removal
        let mut result = String::new();
        let mut chars = text.chars();
        
        while let Some(ch) = chars.next() {
            if ch == '\x1b' {
                // Skip escape sequence
                if chars.next() == Some('[') {
                    // Skip until we find a letter (end of escape sequence)
                    while let Some(esc_ch) = chars.next() {
                        if esc_ch.is_ascii_alphabetic() {
                            break;
                        }
                    }
                }
            } else {
                result.push(ch);
            }
        }
        
        result
    }
    
    fn extract_url_from_text(&self, text: &str) -> Option<String> {
        for line in text.lines() {
            if let Some(url) = self.extract_url_from_line(line) {
                return Some(url);
            }
        }
        None
    }
    
    pub async fn wait_for_auth(&self, timeout: Duration) -> Result<()> {
        let start = std::time::Instant::now();
        let mut interval = tokio::time::interval(Duration::from_millis(500));
        
        loop {
            interval.tick().await;
            
            let (authenticated, _) = self.get_auth_status()?;
            if authenticated {
                return Ok(());
            }
            
            if start.elapsed() > timeout {
                return Err(ClaudeRelayError::Authentication(
                    format!("Authentication timeout after {:?}", timeout)
                ));
            }
        }
    }
    
    pub fn run_interactive_auth(&self) -> Result<()> {
        println!("=== Claude Authentication ===");
        println!("When Claude starts:");
        println!("1. Choose a theme (press 1 for dark or 2 for light)");
        println!("2. Type /login and press Enter");
        println!("3. Follow the browser authentication flow");
        println!("4. Once authenticated, exit Claude (Ctrl+C or type /exit)");
        println!("=============================\n");
        
        self.run_claude_login()
    }
}

pub fn prompt_user(prompt: &str) -> String {
    use std::io::{self, Write};
    
    print!("{}", prompt);
    io::stdout().flush().unwrap();
    
    let mut response = String::new();
    io::stdin().read_line(&mut response).unwrap();
    response.trim().to_string()
}