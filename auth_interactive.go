package clauderelay

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// StartInteractiveAuth starts Claude CLI and captures the authentication URL
func (cs *ClaudeSetup) StartInteractiveAuth() (authURL string, err error) {
	log.Println("Starting Claude CLI for authentication...")
	
	// Create a pseudo-terminal to interact with Claude
	cmd := exec.Command(cs.claudePath)
	cmd.Env = cs.GetClaudeEnv()
	cmd.Dir = cs.baseDir
	
	// Create pipes for stdin and stdout
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	
	// Start the process
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start Claude: %w", err)
	}
	
	// Channel to collect output
	outputChan := make(chan string, 100)
	done := make(chan bool)
	
	// Read stdout
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			outputChan <- line
		}
		done <- true
	}()
	
	// Read stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			outputChan <- line
		}
	}()
	
	// Wait a bit for Claude to start
	time.Sleep(2 * time.Second)
	
	// Send /login command
	log.Println("Sending /login command to Claude...")
	fmt.Fprintln(stdin, "/login")
	
	// Collect output for a few seconds to capture the URL
	var output bytes.Buffer
	timeout := time.After(5 * time.Second)
	
	collectLoop:
	for {
		select {
		case line := <-outputChan:
			output.WriteString(line + "\n")
			// Look for URL pattern in the output
			if strings.Contains(line, "https://") || strings.Contains(line, "console.anthropic.com") {
				authURL = extractURL(line)
				if authURL != "" {
					break collectLoop
				}
			}
		case <-timeout:
			break collectLoop
		case <-done:
			break collectLoop
		}
	}
	
	// Kill the process - we just needed the URL
	cmd.Process.Kill()
	
	// Try to extract URL from all collected output if not found yet
	if authURL == "" {
		authURL = extractURL(output.String())
	}
	
	// If still no URL found, provide the standard login URL
	if authURL == "" {
		authURL = "https://console.anthropic.com/login"
		log.Println("Could not extract authentication URL from Claude output, using default URL")
	}
	
	return authURL, nil
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