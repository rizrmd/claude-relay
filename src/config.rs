use crate::error::Result;
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::fs;
use std::path::Path;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Config {
    #[serde(default = "default_port")]
    pub port: String,
    
    #[serde(default = "default_claude_path")]
    pub claude_path: String,
    
    #[serde(default = "default_max_processes")]
    pub max_processes: usize,
    
    #[serde(default)]
    pub endpoints: HashMap<String, String>,
    
    #[serde(default = "default_allow_origins")]
    pub allow_origins: Vec<String>,
    
    #[serde(default = "default_temp_dir_base")]
    pub temp_dir_base: String,
    
    // New YAML-based configuration fields
    #[serde(default)]
    pub context: Option<String>,
    
    #[serde(default)]
    pub mcp: Option<McpConfig>,
    
    #[serde(default)]
    pub server: Option<ServerConfig>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct McpConfig {
    #[serde(default)]
    pub servers: HashMap<String, McpServer>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct McpServer {
    #[serde(default)]
    pub transport: Option<String>,
    #[serde(default)]
    pub command: Option<String>,
    #[serde(default)]
    pub args: Vec<String>,
    #[serde(default)]
    pub env: HashMap<String, String>,
    #[serde(default)]
    pub url: Option<String>,
    #[serde(default)]
    pub headers: HashMap<String, String>,
    #[serde(default = "default_http_timeout")]
    pub timeout: u64,
    #[serde(default = "default_true")]
    pub reconnect: bool,
    #[serde(default)]
    pub metadata: Option<McpMetadata>,
}

impl McpServer {
    pub fn is_command(&self) -> bool {
        self.transport.is_none() && self.command.is_some() && self.url.is_none()
    }
    
    pub fn is_http(&self) -> bool {
        self.transport.as_deref() == Some("http") || 
        (self.transport.is_none() && self.url.as_ref().map(|u| u.starts_with("http")).unwrap_or(false))
    }
    
    pub fn is_websocket(&self) -> bool {
        self.transport.as_deref() == Some("ws") ||
        (self.transport.is_none() && self.url.as_ref().map(|u| u.starts_with("ws")).unwrap_or(false))
    }
}


#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct McpMetadata {
    #[serde(default)]
    pub description: Option<String>,
    #[serde(default)]
    pub version: Option<String>,
    #[serde(default)]
    pub provider: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ServerConfig {
    #[serde(default = "default_port_u16")]
    pub port: u16,
    #[serde(default = "default_max_processes")]
    pub max_processes: usize,
}

fn default_port() -> String {
    "8080".to_string()
}

fn default_claude_path() -> String {
    "claude".to_string()
}

fn default_max_processes() -> usize {
    100
}

fn default_allow_origins() -> Vec<String> {
    vec!["*".to_string()]
}

fn default_temp_dir_base() -> String {
    std::env::temp_dir().display().to_string()
}

fn default_http_timeout() -> u64 {
    30
}

fn default_true() -> bool {
    true
}

fn default_port_u16() -> u16 {
    3000
}

impl Default for Config {
    fn default() -> Self {
        Config {
            port: default_port(),
            claude_path: default_claude_path(),
            max_processes: default_max_processes(),
            endpoints: {
                let mut map = HashMap::new();
                map.insert("/ws".to_string(), "default".to_string());
                map
            },
            allow_origins: default_allow_origins(),
            temp_dir_base: default_temp_dir_base(),
            context: None,
            mcp: None,
            server: None,
        }
    }
}

impl Config {
    /// Load configuration with priority: clay.yaml > defaults
    /// Note: config.json is Claude CLI's own configuration, not Clay's
    pub fn load_with_priority(base_dir: &Path) -> Result<Self> {
        // Try clay.yaml first (Clay's configuration)
        let yaml_path = base_dir.join("clay.yaml");
        if yaml_path.exists() {
            return Self::load_yaml(&yaml_path);
        }
        
        // Default configuration (don't look for config.json as that's Claude CLI's file)
        Ok(Config::default())
    }
    
    pub fn load_yaml<P: AsRef<Path>>(path: P) -> Result<Self> {
        let data = fs::read_to_string(path)?;
        // TODO: Add back environment variable expansion with proper loop prevention
        let config: Config = serde_yaml::from_str(&data)?;
        Ok(config)
    }
    
    pub fn load_json<P: AsRef<Path>>(path: P) -> Result<Self> {
        let data = fs::read_to_string(path)?;
        let config = serde_json::from_str(&data)?;
        Ok(config)
    }
    
    pub fn save_yaml<P: AsRef<Path>>(&self, path: P) -> Result<()> {
        let data = serde_yaml::to_string(self)?;
        fs::write(path, data)?;
        Ok(())
    }
    
    pub fn save_json<P: AsRef<Path>>(&self, path: P) -> Result<()> {
        let data = serde_json::to_string_pretty(self)?;
        fs::write(path, data)?;
        Ok(())
    }
    
    // Legacy method for backward compatibility
    pub fn load<P: AsRef<Path>>(path: P) -> Result<Self> {
        let path = path.as_ref();
        
        if !path.exists() {
            return Ok(Config::default());
        }
        
        if path.extension().and_then(|s| s.to_str()) == Some("yaml") || 
           path.extension().and_then(|s| s.to_str()) == Some("yml") {
            Self::load_yaml(path)
        } else {
            Self::load_json(path)
        }
    }
    
    pub fn save<P: AsRef<Path>>(&self, path: P) -> Result<()> {
        let path = path.as_ref();
        
        if path.extension().and_then(|s| s.to_str()) == Some("yaml") || 
           path.extension().and_then(|s| s.to_str()) == Some("yml") {
            self.save_yaml(path)
        } else {
            self.save_json(path)
        }
    }
    
    /// Generate a sample clay.yaml configuration file
    pub fn generate_sample_yaml() -> String {
        r#"# Clay Configuration File
# Clay generates Claude CLI's config.json and mcp.json from this file

# Initial context that will be injected into every Claude conversation
context: |
  You are an expert developer working with Clay, a Rust-based OpenAI-compatible API server.
  Always follow Rust best practices and maintain high code quality.
  
  Project Overview:
  - Clay acts as a bridge between OpenAI-compatible clients and Claude CLI
  - It manages portable installations and process lifecycle
  - Supports MCP (Model Context Protocol) servers for enhanced capabilities
  - Uses ../clay-mcp for custom MCP server implementations

# MCP Server Configuration
# Clay will generate Claude CLI's mcp.json from this configuration
mcp:
  servers:
    # File system access
    filesystem:
      command: "npx"
      args: ["-y", "@modelcontextprotocol/server-filesystem", "."]
      env:
        NODE_ENV: "production"
      metadata:
        description: "File system access for the current project"
        version: "1.0.0"
    
    # Custom clay-mcp server for enhanced capabilities
    clay_mcp:
      command: "node"
      args: ["../clay-mcp/index.js"]
      env:
        NODE_ENV: "production"
        CLAY_PROJECT_DIR: "."
      metadata:
        description: "Clay-specific MCP server with enhanced project integration"
        version: "1.0.0"
        provider: "clay"

# Clay Server Configuration
server:
  port: 3000
  max_processes: 100
"#.to_string()
    }
}

