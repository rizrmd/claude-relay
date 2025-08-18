use claude_relay::ClaudeSetup;
use portable_pty::{native_pty_system, PtySize, CommandBuilder};
use std::io::{BufRead, BufReader};
use std::time::Duration;

#[test]
fn debug_pty_output() {
    // Create a test directory
    let temp_dir = tempfile::tempdir().unwrap();
    let test_dir = temp_dir.path().join("pty_debug");
    std::fs::create_dir_all(&test_dir).unwrap();
    
    // Set up Claude
    let setup = ClaudeSetup::new(test_dir.to_str().unwrap()).unwrap();
    setup.setup().unwrap();
    
    println!("=== Testing PTY capture ===");
    
    let pty_system = native_pty_system();
    let pty_pair = pty_system.openpty(PtySize {
        rows: 24,
        cols: 80,
        pixel_width: 0,
        pixel_height: 0,
    }).expect("Failed to create pty");
    
    let mut cmd = CommandBuilder::new(setup.get_claude_path());
    cmd.arg("setup-token");
    
    // Set environment
    for (key, value) in setup.get_claude_env() {
        cmd.env(key, value);
    }
    
    let mut child = pty_pair.slave.spawn_command(cmd).expect("Failed to spawn command");
    drop(pty_pair.slave);
    
    // Read output with timeout
    let mut reader = BufReader::new(pty_pair.master.try_clone_reader().expect("Failed to clone reader"));
    let mut all_output = String::new();
    let mut line = String::new();
    
    println!("Reading PTY output...");
    
    // Give it time to output
    for i in 0..50 { // 5 seconds total
        std::thread::sleep(Duration::from_millis(100));
        
        match reader.read_line(&mut line) {
            Ok(0) => {
                println!("EOF reached at iteration {}", i);
                break;
            }
            Ok(n) => {
                println!("Read {} bytes: {:?}", n, line.trim());
                all_output.push_str(&line);
                
                // Look for URLs
                if line.contains("http") {
                    println!("Found potential URL in line: {}", line.trim());
                }
                
                line.clear();
            }
            Err(e) => {
                // No data available or error
                if i % 10 == 0 { // Print every second
                    println!("No data at iteration {} ({})", i, e);
                }
                continue;
            }
        }
    }
    
    println!("=== Full output ===");
    println!("{}", all_output);
    println!("=== End output ===");
    
    let _ = child.kill();
    
    println!("✅ PTY debug test completed");
}