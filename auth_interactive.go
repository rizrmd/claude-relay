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
	
	// Try to run Claude with a special command to get auth info
	cmd := exec.Command(cs.claudePath, "--version")
	cmd.Env = cs.GetClaudeEnv()
	
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	
	// First verify Claude is working
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to run Claude CLI: %w", err)
	}
	
	// For Claude Code CLI, the auth URL is always the Anthropic console
	// In a real implementation, we might extract this from Claude's output
	authURL := "https://console.anthropic.com/login?client=claude-code"
	
	return authURL, nil
}

// StartNonInteractiveAuth initiates auth and returns instructions for completing it
func (cs *ClaudeSetup) StartNonInteractiveAuth() (authURL string, sessionID string, err error) {
	log.Println("Starting non-interactive authentication flow...")
	
	// Generate a unique session ID for this auth attempt
	sessionID = fmt.Sprintf("claude-auth-%d", time.Now().Unix())
	
	// Get the authentication URL
	authURL, err = cs.GetAuthenticationURL()
	if err != nil {
		return "", "", err
	}
	
	log.Printf("Authentication URL: %s", authURL)
	log.Printf("Session ID: %s", sessionID)
	
	return authURL, sessionID, nil
}

// extractURL extracts a URL from text
func extractURL(text string) string {
	// Pattern to match URLs
	urlPattern := regexp.MustCompile(`https://[^\s\]]+`)
	matches := urlPattern.FindAllString(text, -1)
	
	for _, match := range matches {
		// Clean up the URL
		url := strings.TrimSpace(match)
		url = strings.TrimSuffix(url, ".")
		url = strings.TrimSuffix(url, ",")
		url = strings.TrimSuffix(url, ")")
		url = strings.TrimSuffix(url, "]")
		
		// Prefer URLs with console.anthropic.com
		if strings.Contains(url, "console.anthropic.com") {
			return url
		}
	}
	
	// Return first URL if no Anthropic URL found
	if len(matches) > 0 {
		return matches[0]
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