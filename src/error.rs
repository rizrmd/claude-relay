use thiserror::Error;

#[derive(Error, Debug)]
pub enum ClaudeRelayError {
    #[error("IO error: {0}")]
    Io(#[from] std::io::Error),
    
    #[error("HTTP error: {0}")]
    Http(#[from] reqwest::Error),
    
    #[error("JSON error: {0}")]
    Json(#[from] serde_json::Error),
    
    #[error("ZIP error: {0}")]
    Zip(#[from] zip::result::ZipError),
    
    #[error("Process error: {0}")]
    Process(String),
    
    #[error("Authentication error: {0}")]
    Authentication(String),
    
    #[error("Setup error: {0}")]
    Setup(String),
    
    #[error("Invalid configuration: {0}")]
    Config(String),
    
    #[error("{0}")]
    Other(String),
}

pub type Result<T> = std::result::Result<T, ClaudeRelayError>;

impl From<String> for ClaudeRelayError {
    fn from(s: String) -> Self {
        ClaudeRelayError::Other(s)
    }
}

impl From<&str> for ClaudeRelayError {
    fn from(s: &str) -> Self {
        ClaudeRelayError::Other(s.to_string())
    }
}