package clauderelay

import (
	"fmt"
	"log"
	"os/exec"
	"runtime"
)

// AuthenticateInteractive handles the full authentication flow interactively
// It opens the browser for authentication and returns instructions for getting the token
func (cs *ClaudeSetup) AuthenticateInteractive() (string, error) {
	// Get the authentication URL from Claude CLI
	authURL := "https://claude.ai/auth/login"
	
	// Open browser for authentication
	if err := openBrowser(authURL); err != nil {
		log.Printf("Warning: Could not open browser automatically: %v", err)
		log.Printf("Please open this URL manually: %s", authURL)
	} else {
		log.Printf("Opening browser for authentication: %s", authURL)
	}
	
	// Return instructions for getting the session token
	instructions := fmt.Sprintf(`
Authentication Steps:
1. Complete the login in your browser
2. After successful login, get your session token
3. The token can be found in browser developer tools:
   - Open Developer Tools (F12)
   - Go to Application/Storage → Cookies
   - Find the 'sessionToken' cookie value
   
Alternative: Run this command in another terminal:
%s

Then copy the session token that appears.`, cs.GetSetupTokenInstructions())
	
	return instructions, nil
}

// openBrowser opens the default browser with the given URL
func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "linux":
		cmd = "xdg-open"
		args = []string{url}
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", url}
	default:
		return fmt.Errorf("unsupported platform")
	}

	return exec.Command(cmd, args...).Start()
}

// AuthenticateWithPrompt provides a simple non-interactive authentication flow
// It returns instructions and expects the caller to handle token input
func (cs *ClaudeSetup) AuthenticateWithPrompt() (string, error) {
	// Construct the setup URL that will handle authentication
	setupCommand := fmt.Sprintf("%s setup-token", cs.claudePath)
	
	instructions := fmt.Sprintf(`
To authenticate Claude:

Option 1: Automatic (run in another terminal)
  %s
  
Option 2: Manual browser authentication
  1. Go to: https://claude.ai/auth/login
  2. Complete the login process
  3. Get the session token from browser cookies
  
After getting the token, provide it to complete authentication.`, setupCommand)
	
	return instructions, nil
}