use claude_relay::ClaudeSetup;
use std::process::{Command, Stdio};
use std::io::{Write, BufRead, BufReader};
use std::thread;
use std::time::Duration;
use portable_pty::{native_pty_system, PtySize, CommandBuilder};

#[test]
fn test_auth_simulation() {
    // Skip this test if we can't create a pty
    let pty_system = native_pty_system();
    let pty_pair = pty_system
        .openpty(PtySize {
            rows: 24,
            cols: 80,
            pixel_width: 0,
            pixel_height: 0,
        })
        .expect("Failed to create pty");

    // Create a test directory
    let temp_dir = tempfile::tempdir().unwrap();
    let test_dir = temp_dir.path().join("auth_test");
    std::fs::create_dir_all(&test_dir).unwrap();
    
    // First, set up Claude
    let setup = ClaudeSetup::new(test_dir.to_str().unwrap()).unwrap();
    setup.setup().unwrap();
    
    // Verify Claude is installed but not authenticated
    assert!(setup.is_installed());
    assert!(!setup.check_authentication().unwrap());
    
    println!("✅ Claude installed successfully");
    println!("✅ Authentication check works correctly");
    
    // Test that the CLI correctly detects missing authentication
    let output = Command::new("cargo")
        .args(&["run", "--", "--dir", test_dir.to_str().unwrap()])
        .output()
        .expect("Failed to run claude-relay");
    
    let stdout = String::from_utf8_lossy(&output.stdout);
    assert!(stdout.contains("Claude installed: true"));
    assert!(stdout.contains("Authenticated: false"));
    
    println!("✅ Status command shows correct authentication state");
}

#[test]
fn test_message_auth_prompt() {
    // Create a test directory
    let temp_dir = tempfile::tempdir().unwrap();
    let test_dir = temp_dir.path().join("message_test");
    std::fs::create_dir_all(&test_dir).unwrap();
    
    // First, set up Claude
    let setup = ClaudeSetup::new(test_dir.to_str().unwrap()).unwrap();
    setup.setup().unwrap();
    
    // Test that sending a message without auth triggers auth prompt
    let mut child = Command::new("cargo")
        .args(&["run", "--", "--dir", test_dir.to_str().unwrap(), "--message", "Hello"])
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .expect("Failed to start claude-relay");
    
    // Give it a moment to start
    thread::sleep(Duration::from_millis(500));
    
    // Kill the process since we can't complete auth in test
    child.kill().expect("Failed to kill process");
    let output = child.wait_with_output().expect("Failed to get output");
    
    let stdout = String::from_utf8_lossy(&output.stdout);
    let stderr = String::from_utf8_lossy(&output.stderr);
    
    // Should show authentication prompt
    let combined = format!("{}{}", stdout, stderr);
    assert!(
        combined.contains("not authenticated") || 
        combined.contains("Starting authentication") ||
        combined.contains("choose a theme"),
        "Expected authentication prompt, got: {}", combined
    );
    
    println!("✅ Message command correctly prompts for authentication");
}

#[test] 
fn test_auth_file_creation() {
    let temp_dir = tempfile::tempdir().unwrap();
    let test_dir = temp_dir.path().join("auth_file_test");
    std::fs::create_dir_all(&test_dir).unwrap();
    
    let setup = ClaudeSetup::new(test_dir.to_str().unwrap()).unwrap();
    setup.setup().unwrap();
    
    // Test setting auth token programmatically
    setup.set_auth_token("test_token_123").unwrap();
    
    // Verify auth file was created
    let auth_file = test_dir.join(".claude-home/.config/claude/auth.json");
    assert!(auth_file.exists(), "Auth file should be created");
    
    let auth_content = std::fs::read_to_string(&auth_file).unwrap();
    assert!(auth_content.contains("test_token_123"));
    
    println!("✅ Auth token setting works correctly");
    
    // Test that it's now considered authenticated (though token may be invalid)
    // Note: We can't easily test with a real token in a unit test
    let config_file = test_dir.join(".claude-home/.claude.json");
    std::fs::write(&config_file, r#"{"oauthAccount": {"id": "test"}}"#).unwrap();
    
    assert!(setup.check_authentication().unwrap());
    println!("✅ Authentication detection works correctly");
}