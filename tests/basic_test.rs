use claude_relay::{ClaudeSetup, Config};

#[test]
fn test_config_default() {
    let config = Config::default();
    assert_eq!(config.port, "8080");
    assert_eq!(config.claude_path, "claude");
    assert_eq!(config.max_processes, 100);
}

#[test]
fn test_setup_creation() {
    let temp_dir = tempfile::tempdir().unwrap();
    let setup = ClaudeSetup::new(temp_dir.path().to_str().unwrap()).unwrap();
    
    // Check that paths are properly constructed
    assert!(!setup.is_installed());
    // Compare canonicalized paths to handle /private/var vs /var on macOS
    assert_eq!(
        setup.get_base_dir().canonicalize().unwrap(),
        temp_dir.path().canonicalize().unwrap()
    );
}

#[test]
fn test_config_serialization() {
    let config = Config::default();
    let json = serde_json::to_string(&config).unwrap();
    let config2: Config = serde_json::from_str(&json).unwrap();
    
    assert_eq!(config.port, config2.port);
    assert_eq!(config.claude_path, config2.claude_path);
}