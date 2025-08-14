package clauderelay

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/creack/pty"
)

// Authenticate performs the integrated auth flow
// It runs claude setup-token in a PTY and waits for it to complete
func (cs *ClaudeSetup) Authenticate(reader *bufio.Reader) error {
	// Ensure config directory and file exist to skip welcome
	cs.ensureConfig()
	
	fmt.Println("Starting authentication...")
	
	// Run setup-token and let it complete
	if err := cs.runSetupTokenWithReader(reader); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}
	
	// Verify authentication worked
	authenticated, err := cs.CheckAuthentication()
	if err != nil {
		return fmt.Errorf("failed to verify authentication: %w", err)
	}
	
	if !authenticated {
		return fmt.Errorf("authentication was not successful")
	}
	
	fmt.Println("Authentication completed successfully!")
	return nil
}

// runSetupTokenWithReader runs claude setup-token and waits for it to complete
// It handles the case where Claude asks for a code to be pasted
func (cs *ClaudeSetup) runSetupTokenWithReader(reader *bufio.Reader) error {
	// Create the command
	cmd := exec.Command(cs.claudePath, "setup-token")
	
	// Set environment to prevent browser opening
	env := cs.GetClaudeEnv()
	env = append(env, 
		"BROWSER=none",     // Prevent browser from opening
		"DISPLAY=",         // Clear display for X11
		"OPENER=",          // Some tools use OPENER
	)
	cmd.Env = env
	cmd.Dir = cs.baseDir
	
	
	// Start the command with a PTY
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: 24,
		Cols: 80,
		X:    0,
		Y:    0,
	})
	if err != nil {
		return fmt.Errorf("failed to start setup-token: %w", err)
	}
	defer ptmx.Close()
	
	// Start a goroutine to handle output
	urlShown := false
	codeNeeded := make(chan bool, 1)
	authCompleted := make(chan bool, 1)
	outputDone := make(chan bool, 1)
	
	go func() {
		defer close(outputDone)
		buf := make([]byte, 4096)
		var output bytes.Buffer
		codeSubmitted := false
		
		for {
			ptmx.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
			n, err := ptmx.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])
				output.Write(buf[:n])
				
				// No need to stream output in production
				
				// Handle welcome screen if it appears
				if strings.Contains(chunk, "Choose the text style") || 
				   strings.Contains(chunk, "Dark mode") {
					ptmx.Write([]byte("1\n"))
					continue
				}
				
				// Check if it's asking for a code
				if strings.Contains(chunk, "Paste code here") || strings.Contains(output.String(), "Paste code here") {
					if !urlShown {
						// Extract and show URL first
						if url := extractAuthURL(output.String()); url != "" {
							fmt.Println()
							fmt.Println("Please open this URL in your browser to authenticate:")
							fmt.Printf("  %s\n", url)
							fmt.Println()
							fmt.Println("After authenticating, you'll see a code on the success page.")
							urlShown = true
							// Signal that we need a code
							select {
							case codeNeeded <- true:
							default:
							}
						}
					}
					continue
				}
				
				// Track if we submitted a code
				if strings.Contains(chunk, "Verifying") || strings.Contains(chunk, "Processing") {
					codeSubmitted = true
				}
				
				// Check for success/completion messages
				if strings.Contains(chunk, "Successfully authenticated") ||
				   strings.Contains(chunk, "Authentication successful") ||
				   strings.Contains(chunk, "Token saved") ||
				   strings.Contains(chunk, "completed successfully") ||
				   strings.Contains(chunk, "Success") ||
				   strings.Contains(chunk, "Long-lived authentication token created") ||
				   (codeSubmitted && (strings.Contains(chunk, "Welcome") || strings.Contains(chunk, "✓"))) {
					select {
					case authCompleted <- true:
					default:
					}
					return
				}
				
				// Extract and show URL if not yet shown (for other cases)
				if !urlShown && strings.Contains(output.String(), "https://") {
					if url := extractAuthURL(output.String()); url != "" {
						fmt.Println()
						fmt.Println("Please open this URL in your browser to authenticate:")
						fmt.Printf("  %s\n", url)
						fmt.Println()
						fmt.Println("Waiting for authentication to complete...")
						urlShown = true
					}
				}
			}
			
			// Check if we've hit EOF
			if err == io.EOF {
				break
			}
		}
	}()
	
	// Handle code input if needed
	codeEntered := false
	select {
	case <-codeNeeded:
		// Claude is asking for a code
		fmt.Print("Enter the code from the success page: ")
		code, err := reader.ReadString('\n')
		if err == nil {
			code = strings.TrimSpace(code)
			
			// Clean the code of ANSI escape sequences
			ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
			cleanCode := ansiRegex.ReplaceAllString(code, "")
			
			// Also remove other terminal sequences
			cleanCode = regexp.MustCompile(`\x1b\[200~`).ReplaceAllString(cleanCode, "") // bracketed paste start
			cleanCode = regexp.MustCompile(`\x1b\[201~`).ReplaceAllString(cleanCode, "") // bracketed paste end
			cleanCode = strings.TrimSpace(cleanCode)
			
			if cleanCode != "" {
				// Send the code to claude
				fmt.Printf("Submitting authentication code...\n")
				
				// Send just the cleaned code first
				ptmx.Write([]byte(cleanCode))
				
				// Give a moment for the input to be processed
				time.Sleep(200 * time.Millisecond)
				
				// Then send Enter to submit it
				ptmx.Write([]byte("\r"))
				
				codeEntered = true
			}
		}
	case <-time.After(10 * time.Second):
		// No code needed within 10 seconds, continue
	}
	
	// If we entered a code, give Claude time to process and save it
	if codeEntered {
		select {
		case <-authCompleted:
			fmt.Println("Authentication completed successfully!")
		case <-time.After(8 * time.Second):
			// Check if authentication was saved even without explicit success message
			if authenticated, _ := cs.CheckAuthentication(); authenticated {
				fmt.Println("Authentication completed successfully!")
			} else {
				fmt.Println("Authentication may have failed, but continuing...")
			}
		}
		
		// Kill the process 
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		
		// Copy authentication from system home to our isolated home
		systemHome := os.Getenv("HOME")
		systemClaudeFile := filepath.Join(systemHome, ".claude.json")
		isolatedClaudeFile := filepath.Join(cs.claudeHome, ".claude.json")
		
		if _, err := os.Stat(systemClaudeFile); err == nil {
			if data, err := os.ReadFile(systemClaudeFile); err == nil {
				os.WriteFile(isolatedClaudeFile, data, 0644)
				fmt.Println("Authentication copied to isolated environment")
			}
		}
		
		return nil
	} else {
		// If no code was entered, wait normally
		processErr := cmd.Wait()
		
		// Wait for output handler to finish
		<-outputDone
		
		if processErr != nil {
			return fmt.Errorf("setup-token process failed: %w", processErr)
		}
	}
	
	return nil
}

// ensureConfig ensures Claude config exists to skip welcome screen
func (cs *ClaudeSetup) ensureConfig() {
	configDir := filepath.Join(cs.claudeHome, ".config", "claude")
	os.MkdirAll(configDir, 0755)
	
	configFile := filepath.Join(configDir, "config.json")
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		// Create a minimal config to skip welcome
		config := `{"theme":"dark"}`
		os.WriteFile(configFile, []byte(config), 0644)
	}
}

// extractAuthURL extracts the authentication URL from setup-token output
func extractAuthURL(output string) string {
	// Remove ANSI escape codes
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	cleanOutput := ansiRegex.ReplaceAllString(output, "")
	
	// Look for Claude authentication URLs
	patterns := []string{
		`(https://claude\.ai/login[^\s\]\"]+)`,
		`(https://claude\.ai/auth[^\s\]\"]+)`,
		`(https://claude\.ai/setup[^\s\]\"]+)`,
		`(https://console\.anthropic\.com[^\s\]\"]+)`,
	}
	
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(cleanOutput); len(matches) > 1 {
			url := matches[1]
			// Clean up any trailing characters
			url = strings.TrimRight(url, ".,;:)]}\"'\n\r ")
			return url
		}
	}
	
	// If no specific pattern matched, look for any https URL
	re := regexp.MustCompile(`https://[^\s\]\"]+`)
	if match := re.FindString(cleanOutput); match != "" {
		match = strings.TrimRight(match, ".,;:)]}\"'\n\r ")
		return match
	}
	
	return ""
}