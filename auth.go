package clauderelay

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)


// GetSetupTokenInstructions returns instructions for manual authentication
func (cs *ClaudeSetup) GetSetupTokenInstructions() string {
	return fmt.Sprintf("Run this command to authenticate:\n%s setup-token", cs.claudePath)
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