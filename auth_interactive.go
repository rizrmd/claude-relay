package clauderelay

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// GetAuthenticationURL starts Claude CLI temporarily to extract the authentication URL
func (cs *ClaudeSetup) GetAuthenticationURL() (string, error) {
	log.Println("Getting authentication URL from Claude CLI...")
	
	// Create a command that will output the login URL
	// We send /login command via stdin and capture the output
	cmd := exec.Command(cs.claudePath, "--no-interactive")
	cmd.Env = cs.GetClaudeEnv()
	
	// Prepare input
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	// Start the command
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start Claude CLI: %w", err)
	}
	
	// Send /login command
	go func() {
		defer stdin.Close()
		fmt.Fprintln(stdin, "/login")
		time.Sleep(500 * time.Millisecond)
	}()
	
	// Wait for output with timeout
	done := make(chan error)
	go func() {
		done <- cmd.Wait()
	}()
	
	select {
	case <-done:
		// Command finished
	case <-time.After(2 * time.Second):
		// Kill if taking too long
		cmd.Process.Kill()
	}
	
	// Parse the output for the authentication URL
	output := stdout.String() + stderr.String()
	
	// Look for the authentication URL in the output
	authURL := extractURL(output)
	
	// If we couldn't find the URL in output, try running with --help to find login info
	if authURL == "" {
		// Try another approach - run with print mode
		cmd2 := exec.Command(cs.claudePath, "--print")
		cmd2.Env = cs.GetClaudeEnv()
		
		stdin2, _ := cmd2.StdinPipe()
		var out2 bytes.Buffer
		cmd2.Stdout = &out2
		cmd2.Stderr = &out2
		
		if err := cmd2.Start(); err == nil {
			go func() {
				defer stdin2.Close()
				fmt.Fprintln(stdin2, "/login")
			}()
			
			// Wait briefly
			time.Sleep(1 * time.Second)
			cmd2.Process.Kill()
			
			output2 := out2.String()
			if url := extractURL(output2); url != "" {
				authURL = url
			}
		}
	}
	
	// If still no URL found, use the default
	if authURL == "" {
		log.Println("Could not extract URL from Claude CLI, using default")
		authURL = "https://console.anthropic.com/login"
	}
	
	log.Printf("Authentication URL: %s", authURL)
	return authURL, nil
}

// StartNonInteractiveAuth initiates auth and returns instructions for completing it
func (cs *ClaudeSetup) StartNonInteractiveAuth() (authURL string, sessionID string, err error) {
	log.Println("Starting non-interactive authentication flow...")
	
	// Generate a unique session ID for this auth attempt
	sessionID = fmt.Sprintf("claude-auth-%d", time.Now().Unix())
	
	// First try to get URL by actually running the login command
	authURL = cs.extractLoginURLFromCLI()
	
	// If that didn't work, try the other method
	if authURL == "" {
		authURL, err = cs.GetAuthenticationURL()
		if err != nil {
			// Use default as fallback
			authURL = "https://console.anthropic.com/login"
		}
	}
	
	log.Printf("Authentication URL: %s", authURL)
	log.Printf("Session ID: %s", sessionID)
	
	return authURL, sessionID, nil
}

// extractLoginURLFromCLI runs Claude CLI login command and extracts the URL
func (cs *ClaudeSetup) extractLoginURLFromCLI() string {
	// Run Claude with echo to simulate /login command
	cmd := exec.Command("sh", "-c", fmt.Sprintf("echo '/login' | %s", cs.claudePath))
	cmd.Env = cs.GetClaudeEnv()
	
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	
	// Run with timeout
	done := make(chan error)
	go func() {
		done <- cmd.Run()
	}()
	
	select {
	case <-done:
		// Command finished
	case <-time.After(3 * time.Second):
		// Kill if taking too long
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	}
	
	// Extract URL from output
	return extractURL(output.String())
}

// extractURL extracts a URL from text
func extractURL(text string) string {
	// Pattern to match URLs - more comprehensive
	urlPattern := regexp.MustCompile(`https?://[^\s\]\)\>]+`)
	matches := urlPattern.FindAllString(text, -1)
	
	for _, match := range matches {
		// Clean up the URL
		url := strings.TrimSpace(match)
		// Remove common trailing punctuation
		for _, suffix := range []string{".", ",", ")", "]", "}", ">", "\"", "'"} {
			url = strings.TrimSuffix(url, suffix)
		}
		
		// Prefer URLs with console.anthropic.com or claude.ai
		if strings.Contains(url, "console.anthropic.com") || strings.Contains(url, "claude.ai") {
			return url
		}
	}
	
	// Look for URLs in markdown link format [text](url)
	mdLinkPattern := regexp.MustCompile(`\[.*?\]\((https?://[^\)]+)\)`)
	if mdMatches := mdLinkPattern.FindStringSubmatch(text); len(mdMatches) > 1 {
		return mdMatches[1]
	}
	
	// Return first URL if no Anthropic URL found
	if len(matches) > 0 {
		url := matches[0]
		// Clean up the URL
		for _, suffix := range []string{".", ",", ")", "]", "}", ">", "\"", "'"} {
			url = strings.TrimSuffix(url, suffix)
		}
		return url
	}
	
	return ""
}

// RunInteractiveAuth runs Claude CLI interactively for authentication
func (cs *ClaudeSetup) RunInteractiveAuth() error {
	log.Println("Starting interactive Claude authentication...")
	log.Println("----------------------------------------")
	log.Println("Claude will open in interactive mode.")
	log.Println("When prompted, type: /login")
	log.Println("Then follow the browser authentication flow.")
	log.Println("----------------------------------------")
	
	// Run Claude interactively
	cmd := exec.Command(cs.claudePath)
	cmd.Env = cs.GetClaudeEnv()
	cmd.Dir = cs.baseDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	// Start and wait for the process
	if err := cmd.Run(); err != nil {
		// Check if authentication succeeded despite error
		if authenticated, _ := cs.CheckAuthentication(); authenticated {
			log.Println("Authentication completed successfully!")
			return nil
		}
		return fmt.Errorf("authentication failed: %w", err)
	}
	
	// Verify authentication
	if authenticated, _ := cs.CheckAuthentication(); !authenticated {
		return fmt.Errorf("authentication not completed")
	}
	
	log.Println("Authentication completed successfully!")
	return nil
}

// AuthenticationHelper provides methods to help with authentication
type AuthenticationHelper struct {
	setup *ClaudeSetup
}

// NewAuthenticationHelper creates a new authentication helper
func NewAuthenticationHelper(setup *ClaudeSetup) *AuthenticationHelper {
	return &AuthenticationHelper{setup: setup}
}

// GetInstructions returns user-friendly authentication instructions
func (ah *AuthenticationHelper) GetInstructions() string {
	return `Claude CLI Authentication Instructions:

1. When Claude starts, you'll see an interactive prompt
2. Type: /login
3. A browser window will open for authentication
4. Sign in with your Anthropic account
5. Complete the authentication flow in the browser
6. Return to the terminal - authentication will be confirmed

Note: This uses Claude Code CLI's browser-based authentication.
No API keys are needed or used.`
}

// WaitForAuthentication waits for the user to complete authentication
func (ah *AuthenticationHelper) WaitForAuthentication(timeout time.Duration) error {
	start := time.Now()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	log.Println("Waiting for authentication to complete...")
	
	for {
		select {
		case <-ticker.C:
			if authenticated, _ := ah.setup.CheckAuthentication(); authenticated {
				log.Println("Authentication detected!")
				return nil
			}
			
			if time.Since(start) > timeout {
				return fmt.Errorf("authentication timeout after %v", timeout)
			}
		}
	}
}

// CompleteNonInteractiveAuth completes authentication using a session token
// The session token should be obtained after user completes browser auth
func (cs *ClaudeSetup) CompleteNonInteractiveAuth(sessionToken string) error {
	if sessionToken == "" {
		return fmt.Errorf("session token cannot be empty")
	}
	
	// The session token format for Claude CLI auth
	// Create the auth.json structure
	authData := fmt.Sprintf(`{"token":"%s","type":"session"}`, sessionToken)
	
	// Ensure config directory exists
	configDir := filepath.Join(cs.claudeHome, ".config", "claude")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	
	// Write auth file
	authFile := filepath.Join(configDir, "auth.json")
	if err := os.WriteFile(authFile, []byte(authData), 0600); err != nil {
		return fmt.Errorf("failed to write auth file: %w", err)
	}
	
	// Verify authentication worked
	authenticated, err := cs.CheckAuthentication()
	if err != nil {
		return fmt.Errorf("failed to verify authentication: %w", err)
	}
	
	if !authenticated {
		// Try alternate format (just the key)
		authData = fmt.Sprintf(`{"key":"%s"}`, sessionToken)
		if err := os.WriteFile(authFile, []byte(authData), 0600); err != nil {
			return fmt.Errorf("failed to write auth file: %w", err)
		}
		
		// Check again
		authenticated, err = cs.CheckAuthentication()
		if err != nil || !authenticated {
			return fmt.Errorf("authentication failed - token may be invalid")
		}
	}
	
	log.Println("Non-interactive authentication completed successfully!")
	return nil
}