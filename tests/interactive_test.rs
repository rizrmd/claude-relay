use std::process::{Command, Stdio};
use std::io::{Write, BufRead, BufReader};
use std::thread;
use std::time::Duration;

#[test]
fn test_interactive_auth_simulation() {
    // Create a test directory
    let temp_dir = tempfile::tempdir().unwrap();
    let test_dir = temp_dir.path().join("interactive_test");
    std::fs::create_dir_all(&test_dir).unwrap();
    
    // Set up Claude first
    let setup_output = Command::new("cargo")
        .args(&["run", "--", "--setup", "--dir", test_dir.to_str().unwrap()])
        .output()
        .expect("Failed to setup Claude");
    
    assert!(setup_output.status.success(), "Setup should succeed");
    println!("✅ Claude setup completed");
    
    // Start the auth process
    let mut child = Command::new("cargo")
        .args(&["run", "--", "--dir", test_dir.to_str().unwrap(), "--message", "Test message"])
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .expect("Failed to start claude-relay");
    
    let stdout = child.stdout.take().expect("Failed to get stdout");
    let mut reader = BufReader::new(stdout);
    
    // Read the authentication prompt
    let mut line = String::new();
    let mut found_auth_prompt = false;
    
    // Give it time to start and read initial output
    for _ in 0..10 {
        thread::sleep(Duration::from_millis(200));
        
        if reader.read_line(&mut line).is_ok() && !line.trim().is_empty() {
            println!("Output: {}", line.trim());
            
            if line.contains("not authenticated") || 
               line.contains("Starting authentication") ||
               line.contains("choose a theme") {
                found_auth_prompt = true;
                break;
            }
            line.clear();
        }
    }
    
    // Try to simulate sending "1" for theme selection
    if let Some(stdin) = child.stdin.as_mut() {
        let _ = stdin.write_all(b"1\n");
        let _ = stdin.flush();
    }
    
    // Give it a moment then terminate
    thread::sleep(Duration::from_millis(1000));
    let _ = child.kill();
    let output = child.wait_with_output().expect("Failed to get final output");
    
    let combined_output = format!(
        "{}\n{}", 
        String::from_utf8_lossy(&output.stdout),
        String::from_utf8_lossy(&output.stderr)
    );
    
    println!("Final output:\n{}", combined_output);
    
    // Verify we got the authentication prompt
    assert!(
        found_auth_prompt || combined_output.contains("not authenticated") ||
        combined_output.contains("Starting authentication") ||
        combined_output.contains("choose a theme"),
        "Should show authentication prompt. Got: {}", combined_output
    );
    
    println!("✅ Authentication prompt detected successfully");
}

#[test]
fn test_claude_cli_direct_execution() {
    // Test that we can execute the Claude CLI directly
    let temp_dir = tempfile::tempdir().unwrap();
    let test_dir = temp_dir.path().join("direct_test");
    std::fs::create_dir_all(&test_dir).unwrap();
    
    // Set up Claude
    let setup_output = Command::new("cargo")
        .args(&["run", "--", "--setup", "--dir", test_dir.to_str().unwrap()])
        .output()
        .expect("Failed to setup Claude");
    
    assert!(setup_output.status.success());
    
    // Try to run Claude CLI directly to see its output
    let claude_bin = test_dir.join(".bun/bin/claude");
    assert!(claude_bin.exists(), "Claude binary should exist");
    
    // Test with --version flag
    let version_output = Command::new(&claude_bin)
        .arg("--version")
        .output()
        .expect("Failed to run Claude CLI");
    
    let version_str = String::from_utf8_lossy(&version_output.stdout);
    println!("Claude CLI version: {}", version_str.trim());
    assert!(version_str.contains("Claude Code"), "Should be Claude Code CLI");
    
    println!("✅ Claude CLI direct execution works");
}

#[test]  
fn test_auth_flow_components() {
    // Test individual components of the auth flow
    let temp_dir = tempfile::tempdir().unwrap();
    let test_dir = temp_dir.path().join("auth_components");
    std::fs::create_dir_all(&test_dir).unwrap();
    
    // Setup
    let setup = claude_relay::ClaudeSetup::new(test_dir.to_str().unwrap()).unwrap();
    setup.setup().unwrap();
    
    // Test authentication check
    assert!(!setup.check_authentication().unwrap(), "Should not be authenticated initially");
    
    // Test setting fake auth token
    setup.set_auth_token("fake_session_token").unwrap();
    
    // Create fake .claude.json for testing
    let claude_config = test_dir.join(".claude-home/.claude.json");
    std::fs::write(&claude_config, r#"{"oauthAccount": {"id": "test_user"}}"#).unwrap();
    
    // Now should be considered authenticated
    assert!(setup.check_authentication().unwrap(), "Should be authenticated after setup");
    
    // Test process creation
    let process_result = claude_relay::ClaudeProcess::new(std::sync::Arc::new(setup));
    assert!(process_result.is_ok(), "Should be able to create process");
    
    println!("✅ All auth flow components work correctly");
}