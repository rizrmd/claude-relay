package clauderelay

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

// StartAuth initiates authentication and returns the OAuth URL and session ID
func (cs *ClaudeSetup) StartAuth() (authURL string, sessionID string, err error) {
	log.Println("Starting authentication flow...")
	
	// Generate a unique session ID for this auth attempt
	sessionID = fmt.Sprintf("claude-auth-%d", time.Now().Unix())
	
	// Get the correct OAuth URL for Claude CLI
	authURL = cs.buildClaudeOAuthURL()
	
	log.Printf("Authentication URL: %s", authURL)
	log.Printf("Session ID: %s", sessionID)
	
	return authURL, sessionID, nil
}

// buildClaudeOAuthURL constructs the correct OAuth URL for Claude CLI
func (cs *ClaudeSetup) buildClaudeOAuthURL() string {
	// These are the exact OAuth configuration values used by Claude CLI
	// Extracted from the Claude CLI source code
	baseURL := "https://console.anthropic.com/oauth/authorize"
	clientID := "9d1c250a-e61b-44d9-88ed-5944d1962f5e" // Official Claude CLI client ID
	redirectURI := "http://localhost:54545/callback"   // Default redirect port for Claude CLI
	scopes := "org:create_api_key user:profile user:inference" // Required scopes
	
	// URL encode the parameters properly
	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", clientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("scope", scopes)
	
	return fmt.Sprintf("%s?%s", baseURL, params.Encode())
}

// CompleteAuth completes authentication using a session token
// The session token should be obtained after user completes browser auth
func (cs *ClaudeSetup) CompleteAuth(sessionToken string) error {
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
	
	log.Println("Authentication completed successfully!")
	return nil
}