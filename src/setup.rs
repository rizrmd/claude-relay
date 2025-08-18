use crate::error::{ClaudeRelayError, Result};
use crate::config::{Config, McpConfig};
use std::env;
use std::fs;
use std::io;
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};
use tracing::{info, warn};
use serde_json::json;

pub struct ClaudeSetup {
    base_dir: PathBuf,
    bun_path: PathBuf,
    claude_path: PathBuf,
    claude_home: PathBuf,
    config: Option<Config>,
}

impl ClaudeSetup {
    pub fn new(base_dir: &str) -> Result<Self> {
        let base_dir = Path::new(base_dir)
            .canonicalize()
            .map_err(|e| ClaudeRelayError::Setup(format!("Failed to get absolute path: {}", e)))?;
        
        // Auto-generate clay.yaml if it doesn't exist
        let yaml_path = base_dir.join("clay.yaml");
        if !yaml_path.exists() {
            let sample_config = Config::generate_sample_yaml();
            fs::write(&yaml_path, sample_config)?;
            info!("Generated clay.yaml configuration file at {:?}", yaml_path);
            println!("📝 Generated clay.yaml configuration file");
            println!("   Edit this file to customize your MCP servers and context settings");
        }
        
        // Load configuration with priority
        let config = Config::load_with_priority(&base_dir).ok();
        
        Ok(ClaudeSetup {
            bun_path: base_dir.join(".bun"),
            claude_path: base_dir.join(".bun").join("bin").join("claude"),
            claude_home: base_dir.join(".claude-home"),
            base_dir,
            config,
        })
    }

    pub fn is_installed(&self) -> bool {
        self.bun_path.exists() && self.claude_path.exists()
    }

    pub fn install_bun(&self) -> Result<()> {
        if self.bun_path.exists() {
            info!("Bun already installed at {:?}", self.bun_path);
            return Ok(());
        }

        info!("Installing portable Bun...");

        let download_url = self.get_bun_download_url()?;
        
        // Download Bun
        let response = reqwest::blocking::get(&download_url)
            .map_err(|e| ClaudeRelayError::Setup(format!("Failed to download Bun: {}", e)))?;
        
        let bytes = response.bytes()
            .map_err(|e| ClaudeRelayError::Setup(format!("Failed to read Bun download: {}", e)))?;

        // Extract the zip
        let reader = std::io::Cursor::new(bytes);
        let mut archive = zip::ZipArchive::new(reader)?;

        // Create .bun/bin directory
        let bun_bin_dir = self.bun_path.join("bin");
        fs::create_dir_all(&bun_bin_dir)?;

        // Extract bun executable
        for i in 0..archive.len() {
            let mut file = archive.by_index(i)?;
            let name = file.name();
            
            if name.ends_with("bun") || name == "bun" {
                let bun_exe_path = bun_bin_dir.join("bun");
                let mut outfile = fs::File::create(&bun_exe_path)?;
                io::copy(&mut file, &mut outfile)?;
                
                // Make executable on Unix
                #[cfg(unix)]
                {
                    use std::os::unix::fs::PermissionsExt;
                    let mut perms = fs::metadata(&bun_exe_path)?.permissions();
                    perms.set_mode(0o755);
                    fs::set_permissions(&bun_exe_path, perms)?;
                }
                
                info!("Bun installed successfully at {:?}", bun_exe_path);
                break;
            }
        }

        Ok(())
    }

    pub fn install_claude(&self) -> Result<()> {
        if self.claude_path.exists() {
            info!("Claude already installed at {:?}", self.claude_path);
            return Ok(());
        }

        info!("Installing Claude Code CLI...");

        let bun_exe = self.bun_path.join("bin").join("bun");
        
        let mut cmd = Command::new(&bun_exe);
        cmd.args(&["install", "-g", "@anthropic-ai/claude-code"])
            .env("BUN_INSTALL", &self.bun_path)
            .env("PATH", format!("{}:{}", 
                self.bun_path.join("bin").display(), 
                env::var("PATH").unwrap_or_default()));

        let output = cmd.output()
            .map_err(|e| ClaudeRelayError::Setup(format!("Failed to install Claude: {}", e)))?;

        if !output.status.success() {
            return Err(ClaudeRelayError::Setup(format!(
                "Failed to install Claude: {}",
                String::from_utf8_lossy(&output.stderr)
            )));
        }

        info!("Claude installed successfully at {:?}", self.claude_path);
        Ok(())
    }

    pub fn setup_claude_home(&self) -> Result<()> {
        // Create isolated Claude home directory
        fs::create_dir_all(&self.claude_home)?;

        // Create .config/claude directory for Claude's configuration
        let config_dir = self.claude_home.join(".config").join("claude");
        fs::create_dir_all(&config_dir)?;

        // Generate Claude's config.json from clay.yaml settings
        self.generate_claude_config()?;

        info!("Claude home directory set up at {:?}", self.claude_home);
        Ok(())
    }

    /// Generate Claude CLI's config.json file from clay.yaml configuration
    pub fn generate_claude_config(&self) -> Result<()> {
        let config_dir = self.claude_home.join(".config").join("claude");
        fs::create_dir_all(&config_dir)?;
        
        let config_file = config_dir.join("config.json");
        
        // Base Claude configuration
        let mut claude_config = json!({
            "theme": "dark",
            "hasSeenWelcome": true,
            "outputStyle": "default"
        });
        
        // Apply clay.yaml overrides if available
        if let Some(config) = &self.config {
            // If we have server config, we can add Claude-specific settings
            if let Some(server_config) = &config.server {
                // Claude CLI doesn't directly use port config, but we could add other settings
                claude_config["maxProcesses"] = json!(server_config.max_processes);
            }
            
            // Add any other Claude-specific configurations from clay.yaml
            // For now, we keep the basic setup
        }
        
        fs::write(&config_file, serde_json::to_string_pretty(&claude_config)?)?;
        info!("Generated Claude config.json at {:?}", config_file);
        
        Ok(())
    }

    pub fn check_authentication(&self) -> Result<bool> {
        // Check if Claude is authenticated by looking for the .claude.json file
        let claude_config_file = self.claude_home.join(".claude.json");
        
        if claude_config_file.exists() {
            let data = fs::read_to_string(&claude_config_file)?;
            if data.contains("oauthAccount") {
                return Ok(true);
            }
        }

        Ok(false)
    }

    pub fn get_auth_status(&self) -> Result<(bool, String)> {
        let claude_config_file = self.claude_home.join(".claude.json");
        
        if !claude_config_file.exists() {
            return Ok((false, "No authentication file found".to_string()));
        }
        
        let metadata = fs::metadata(&claude_config_file)?;
        if metadata.len() == 0 {
            return Ok((false, "Authentication file is empty".to_string()));
        }
        
        let data = fs::read_to_string(&claude_config_file)?;
        if data.len() < 10 || !data.contains("oauthAccount") {
            return Ok((false, "Authentication file appears invalid".to_string()));
        }
        
        Ok((true, "Authenticated".to_string()))
    }

    pub fn run_claude_login(&self) -> Result<()> {
        info!("Claude needs authentication. Starting login process...");
        println!("Please follow the authentication prompts below:");
        println!("----------------------------------------");
        
        let mut cmd = Command::new(&self.claude_path);
        cmd.env_clear()
            .envs(self.get_claude_env())
            .stdin(Stdio::inherit())
            .stdout(Stdio::inherit())
            .stderr(Stdio::inherit())
            .current_dir(&self.base_dir);

        let mut child = cmd.spawn()
            .map_err(|e| ClaudeRelayError::Authentication(format!("Failed to start Claude for login: {}", e)))?;

        let status = child.wait()
            .map_err(|e| ClaudeRelayError::Authentication(format!("Failed to complete Claude login: {}", e)))?;

        // Check if authentication file was created despite error
        if !status.success() {
            if self.check_authentication()? {
                info!("Claude authentication completed successfully");
                return Ok(());
            }
            return Err(ClaudeRelayError::Authentication("Failed to complete Claude login".into()));
        }

        info!("Claude authentication completed successfully");
        Ok(())
    }

    pub fn get_claude_env(&self) -> Vec<(String, String)> {
        let mut env: Vec<(String, String)> = env::vars()
            .filter(|(k, _)| !k.starts_with("HOME") && !k.starts_with("BUN_INSTALL"))
            .collect();
        
        env.push(("HOME".to_string(), self.claude_home.display().to_string()));
        env.push(("BUN_INSTALL".to_string(), self.bun_path.display().to_string()));
        
        let path = format!("{}:{}", 
            self.bun_path.join("bin").display(),
            env::var("PATH").unwrap_or_default());
        env.push(("PATH".to_string(), path));
        
        env
    }

    pub fn get_claude_path(&self) -> &Path {
        &self.claude_path
    }

    pub fn get_claude_home(&self) -> &Path {
        &self.claude_home
    }

    pub fn get_base_dir(&self) -> &Path {
        &self.base_dir
    }

    pub fn setup(&self) -> Result<()> {
        info!("Setting up isolated Claude environment...");

        self.install_bun()?;
        self.install_claude()?;
        self.setup_claude_home()?;

        info!("Claude setup completed successfully");
        Ok(())
    }

    pub fn is_authentication_needed(&self, output: &str) -> bool {
        output.contains("Invalid API key") 
            || output.contains("not authenticated")
            || output.contains("Please log in")
            || output.contains("claude login")
    }

    pub fn copy_auth_from(&self, source_dir: &Path) -> Result<()> {
        let source_auth_file = source_dir.join("auth.json");
        
        if !source_auth_file.exists() {
            return Err(ClaudeRelayError::Authentication(
                format!("No auth.json found in {:?}", source_dir)
            ));
        }
        
        let auth_data = fs::read(&source_auth_file)?;
        
        let config_dir = self.claude_home.join(".config").join("claude");
        fs::create_dir_all(&config_dir)?;
        
        let dest_auth_file = config_dir.join("auth.json");
        fs::write(&dest_auth_file, auth_data)?;
        
        info!("Authentication copied from {:?}", source_dir);
        Ok(())
    }

    pub fn set_auth_token(&self, auth_token: &str) -> Result<()> {
        if auth_token.is_empty() {
            return Err(ClaudeRelayError::Authentication("Auth token cannot be empty".into()));
        }
        
        let config_dir = self.claude_home.join(".config").join("claude");
        fs::create_dir_all(&config_dir)?;
        
        let auth_file = config_dir.join("auth.json");
        let auth_data = format!(r#"{{"key":"{}"}}"#, auth_token);
        
        fs::write(&auth_file, auth_data)?;
        
        info!("Authentication token saved successfully");
        Ok(())
    }

    fn get_bun_download_url(&self) -> Result<String> {
        let os = env::consts::OS;
        let arch = env::consts::ARCH;
        
        let url = match (os, arch) {
            ("macos", "aarch64") => "https://github.com/oven-sh/bun/releases/latest/download/bun-darwin-aarch64.zip",
            ("macos", "x86_64") => "https://github.com/oven-sh/bun/releases/latest/download/bun-darwin-x64.zip",
            ("linux", "aarch64") => "https://github.com/oven-sh/bun/releases/latest/download/bun-linux-aarch64.zip",
            ("linux", "x86_64") => "https://github.com/oven-sh/bun/releases/latest/download/bun-linux-x64.zip",
            _ => return Err(ClaudeRelayError::Setup(
                format!("Unsupported platform: {}/{}", os, arch)
            )),
        };
        
        Ok(url.to_string())
    }

    /// Get the configuration loaded from clay.yaml or defaults
    pub fn get_config(&self) -> &Option<Config> {
        &self.config
    }

    /// Get initial context from configuration
    pub fn get_initial_context(&self) -> Option<String> {
        self.config.as_ref().and_then(|c| c.context.clone())
    }

    /// Setup MCP configuration file for Claude CLI and regenerate Claude's config
    pub fn setup_mcp_config(&self) -> Result<()> {
        // Always regenerate Claude's base configuration
        self.generate_claude_config()?;
        
        // Generate MCP configuration if available
        if let Some(config) = &self.config {
            if let Some(mcp_config) = &config.mcp {
                self.write_mcp_config(mcp_config)?;
                info!("MCP configuration written successfully");
            }
        }
        Ok(())
    }

    /// Write MCP configuration to Claude's config directory in the format Claude CLI expects
    fn write_mcp_config(&self, mcp_config: &McpConfig) -> Result<()> {
        let config_dir = self.claude_home.join(".config").join("claude");
        fs::create_dir_all(&config_dir)?;
        
        // Claude CLI expects MCP server configuration in a specific format
        // The file should be named according to Claude CLI's expectations
        let mcp_config_file = config_dir.join("mcp.json");
        let mut claude_mcp_config = json!({
            "mcpServers": {}
        });
        
        for (name, server) in &mcp_config.servers {
            if server.is_command() {
                if let Some(command) = &server.command {
                    claude_mcp_config["mcpServers"][name] = json!({
                        "command": command,
                        "args": server.args,
                        "env": server.env
                    });
                }
            } else if server.is_http() {
                // For HTTP MCP servers, we create a proxy command that Clay can handle
                warn!("HTTP MCP server '{}' will be proxied through Clay", name);
                claude_mcp_config["mcpServers"][name] = json!({
                    "command": "clay-mcp-proxy",
                    "args": ["--type", "http", "--name", name],
                    "env": {}
                });
            } else if server.is_websocket() {
                // For WebSocket MCP servers, we create a proxy command that Clay can handle
                warn!("WebSocket MCP server '{}' will be proxied through Clay", name);
                claude_mcp_config["mcpServers"][name] = json!({
                    "command": "clay-mcp-proxy",
                    "args": ["--type", "ws", "--name", name], 
                    "env": {}
                });
            }
        }
        
        fs::write(&mcp_config_file, serde_json::to_string_pretty(&claude_mcp_config)?)?;
        info!("Claude MCP configuration written to {:?}", mcp_config_file);
        
        // Also write the original clay MCP config for Clay's own use
        let clay_mcp_file = config_dir.join("clay-mcp.json");
        fs::write(&clay_mcp_file, serde_json::to_string_pretty(mcp_config)?)?;
        info!("Clay MCP configuration written to {:?}", clay_mcp_file);
        
        Ok(())
    }

    /// Validate MCP server configurations
    pub fn validate_mcp_servers(&self) -> Result<Vec<String>> {
        let mut issues = Vec::new();
        
        if let Some(config) = &self.config {
            if let Some(mcp_config) = &config.mcp {
                for (name, server) in &mcp_config.servers {
                    if server.is_command() {
                        if let Some(command) = &server.command {
                            if command.is_empty() {
                                issues.push(format!("MCP server '{}': command cannot be empty", name));
                            }
                        } else {
                            issues.push(format!("MCP server '{}': command is required for command transport", name));
                        }
                    } else if server.is_http() {
                        if let Some(url) = &server.url {
                            if !url.starts_with("http://") && !url.starts_with("https://") {
                                issues.push(format!("MCP server '{}': invalid HTTP URL '{}'", name, url));
                            }
                        } else {
                            issues.push(format!("MCP server '{}': url is required for HTTP transport", name));
                        }
                    } else if server.is_websocket() {
                        if let Some(url) = &server.url {
                            if !url.starts_with("ws://") && !url.starts_with("wss://") {
                                issues.push(format!("MCP server '{}': invalid WebSocket URL '{}'", name, url));
                            }
                        } else {
                            issues.push(format!("MCP server '{}': url is required for WebSocket transport", name));
                        }
                    } else {
                        issues.push(format!("MCP server '{}': unable to determine transport type", name));
                    }
                }
            }
        }
        
        Ok(issues)
    }

    /// Force regenerate clay.yaml file in the base directory
    pub fn init_config(&self) -> Result<()> {
        let clay_yaml_path = self.base_dir.join("clay.yaml");
        
        if clay_yaml_path.exists() {
            println!("⚠️  clay.yaml already exists. Overwriting with new template...");
        }
        
        let sample_config = Config::generate_sample_yaml();
        fs::write(&clay_yaml_path, sample_config)?;
        
        info!("Clay.yaml configuration created at {:?}", clay_yaml_path);
        println!("📝 Generated clay.yaml configuration file");
        println!("   Edit this file to customize your MCP servers and context settings");
        
        Ok(())
    }

    /// Update the main setup method to include MCP configuration
    pub fn setup_with_mcp(&self) -> Result<()> {
        info!("Setting up isolated Claude environment with MCP support...");

        self.install_bun()?;
        self.install_claude()?;
        self.setup_claude_home()?;
        self.setup_mcp_config()?;

        // Validate MCP configuration
        let issues = self.validate_mcp_servers()?;
        if !issues.is_empty() {
            warn!("MCP configuration issues found:");
            for issue in &issues {
                warn!("  - {}", issue);
            }
        }

        info!("Claude setup with MCP completed successfully");
        Ok(())
    }
}