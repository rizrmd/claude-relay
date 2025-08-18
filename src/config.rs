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
        }
    }
}

impl Config {
    pub fn load<P: AsRef<Path>>(path: P) -> Result<Self> {
        let path = path.as_ref();
        
        if !path.exists() {
            return Ok(Config::default());
        }
        
        let data = fs::read_to_string(path)?;
        let config = serde_json::from_str(&data)?;
        Ok(config)
    }
    
    pub fn save<P: AsRef<Path>>(&self, path: P) -> Result<()> {
        let data = serde_json::to_string_pretty(self)?;
        fs::write(path, data)?;
        Ok(())
    }
}