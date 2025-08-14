// Package clauderelay provides isolated Claude Code CLI instances.
package clauderelay

import (
	"archive/zip"
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type ClaudeSetup struct {
	baseDir     string
	bunPath     string
	claudePath  string
	claudeHome  string
}

// New creates a ClaudeSetup with a custom base directory.
// This allows multiple isolated Claude instances.
func New(baseDir string) (*ClaudeSetup, error) {
	absBaseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}
	
	return &ClaudeSetup{
		baseDir:    absBaseDir,
		bunPath:    filepath.Join(absBaseDir, ".bun"),
		claudePath: filepath.Join(absBaseDir, ".bun", "bin", "claude"),
		claudeHome: filepath.Join(absBaseDir, ".claude-home"),
	}, nil
}


func (cs *ClaudeSetup) IsInstalled() bool {
	// Check if both Bun and Claude are installed
	if _, err := os.Stat(cs.bunPath); os.IsNotExist(err) {
		return false
	}
	if _, err := os.Stat(cs.claudePath); os.IsNotExist(err) {
		return false
	}
	return true
}

func (cs *ClaudeSetup) InstallBun() error {
	if _, err := os.Stat(cs.bunPath); err == nil {
		log.Printf("Bun already installed at %s", cs.bunPath)
		return nil
	}

	log.Println("Installing portable Bun...")
	
	// Determine the platform
	var downloadURL string
	switch runtime.GOOS {
	case "darwin":
		if runtime.GOARCH == "arm64" {
			downloadURL = "https://github.com/oven-sh/bun/releases/latest/download/bun-darwin-aarch64.zip"
		} else {
			downloadURL = "https://github.com/oven-sh/bun/releases/latest/download/bun-darwin-x64.zip"
		}
	case "linux":
		if runtime.GOARCH == "arm64" {
			downloadURL = "https://github.com/oven-sh/bun/releases/latest/download/bun-linux-aarch64.zip"
		} else {
			downloadURL = "https://github.com/oven-sh/bun/releases/latest/download/bun-linux-x64.zip"
		}
	default:
		return fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	// Download Bun
	resp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download Bun: %w", err)
	}
	defer resp.Body.Close()

	// Read the zip content
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read Bun download: %w", err)
	}

	// Extract the zip
	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return fmt.Errorf("failed to read zip: %w", err)
	}

	// Create .bun directory
	bunBinDir := filepath.Join(cs.bunPath, "bin")
	if err := os.MkdirAll(bunBinDir, 0755); err != nil {
		return fmt.Errorf("failed to create bun directory: %w", err)
	}

	// Extract bun executable
	for _, file := range reader.File {
		if strings.HasSuffix(file.Name, "bun") || file.Name == "bun" {
			src, err := file.Open()
			if err != nil {
				return fmt.Errorf("failed to open file in zip: %w", err)
			}
			defer src.Close()

			bunExePath := filepath.Join(bunBinDir, "bun")
			dst, err := os.OpenFile(bunExePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return fmt.Errorf("failed to create bun executable: %w", err)
			}
			defer dst.Close()

			if _, err := io.Copy(dst, src); err != nil {
				return fmt.Errorf("failed to extract bun: %w", err)
			}

			log.Printf("Bun installed successfully at %s", bunExePath)
			break
		}
	}

	return nil
}

func (cs *ClaudeSetup) InstallClaude() error {
	if _, err := os.Stat(cs.claudePath); err == nil {
		log.Printf("Claude already installed at %s", cs.claudePath)
		return nil
	}

	log.Println("Installing Claude Code CLI...")

	bunExe := filepath.Join(cs.bunPath, "bin", "bun")
	
	// Set up environment with custom home
	env := os.Environ()
	env = append(env, fmt.Sprintf("BUN_INSTALL=%s", cs.bunPath))
	env = append(env, fmt.Sprintf("PATH=%s:%s", filepath.Join(cs.bunPath, "bin"), os.Getenv("PATH")))

	// Install Claude using bun  
	cmd := exec.Command(bunExe, "install", "-g", "@anthropic-ai/claude-code")
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install Claude: %w", err)
	}

	log.Printf("Claude installed successfully at %s", cs.claudePath)
	return nil
}

func (cs *ClaudeSetup) SetupClaudeHome() error {
	// Create isolated Claude home directory
	if err := os.MkdirAll(cs.claudeHome, 0755); err != nil {
		return fmt.Errorf("failed to create Claude home directory: %w", err)
	}

	// Create .config/claude directory for Claude's configuration
	configDir := filepath.Join(cs.claudeHome, ".config", "claude")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create Claude config directory: %w", err)
	}

	// Pre-configure Claude to skip the welcome screen
	// Set a default theme to avoid the initial setup prompt
	configFile := filepath.Join(configDir, "config.json")
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		// Create a minimal config to skip welcome
		config := `{"theme":"dark","hasSeenWelcome":true}`
		if err := os.WriteFile(configFile, []byte(config), 0644); err != nil {
			log.Printf("Warning: Failed to create config file: %v", err)
			// Not fatal, continue anyway
		}
	}

	log.Printf("Claude home directory set up at %s", cs.claudeHome)
	return nil
}

func (cs *ClaudeSetup) CheckAuthentication() (bool, error) {
	// Check if Claude is authenticated by looking for the .claude.json file with oauthAccount
	claudeConfigFile := filepath.Join(cs.claudeHome, ".claude.json")
	if _, err := os.Stat(claudeConfigFile); err == nil {
		// Verify the config file contains oauth account data
		data, err := os.ReadFile(claudeConfigFile)
		if err == nil && strings.Contains(string(data), "oauthAccount") {
			return true, nil
		}
	}

	return false, nil
}

// GetAuthStatus returns detailed authentication status
func (cs *ClaudeSetup) GetAuthStatus() (bool, string, error) {
	claudeConfigFile := filepath.Join(cs.claudeHome, ".claude.json")
	
	// Check if file exists
	info, err := os.Stat(claudeConfigFile)
	if os.IsNotExist(err) {
		return false, "No authentication file found", nil
	}
	if err != nil {
		return false, "", fmt.Errorf("failed to check auth file: %w", err)
	}
	
	// Check if file has content
	if info.Size() == 0 {
		return false, "Authentication file is empty", nil
	}
	
	// Read and validate
	data, err := os.ReadFile(claudeConfigFile)
	if err != nil {
		return false, "", fmt.Errorf("failed to read auth file: %w", err)
	}
	
	// Basic validation - check if it contains oauthAccount
	if len(data) < 10 || !strings.Contains(string(data), "oauthAccount") {
		return false, "Authentication file appears invalid", nil
	}
	
	return true, "Authenticated", nil
}

func (cs *ClaudeSetup) RunClaudeLogin() error {
	log.Println("Claude needs authentication. Starting login process...")
	log.Println("Please follow the authentication prompts below:")
	log.Println("----------------------------------------")
	
	// Run claude login command interactively
	// We need to run this in a way that allows user interaction
	cmd := exec.Command(cs.claudePath)
	cmd.Env = cs.GetClaudeEnv()
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = cs.baseDir

	// Start the process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start Claude for login: %w", err)
	}

	// Wait for the process to complete
	if err := cmd.Wait(); err != nil {
		// Check if authentication file was created despite error
		if authenticated, _ := cs.CheckAuthentication(); authenticated {
			log.Println("Claude authentication completed successfully")
			return nil
		}
		return fmt.Errorf("failed to complete Claude login: %w", err)
	}

	log.Println("Claude authentication completed successfully")
	return nil
}

func (cs *ClaudeSetup) GetClaudeEnv() []string {
	env := os.Environ()
	
	// Filter out existing HOME variable
	var filteredEnv []string
	for _, e := range env {
		if !strings.HasPrefix(e, "HOME=") && !strings.HasPrefix(e, "BUN_INSTALL=") {
			filteredEnv = append(filteredEnv, e)
		}
	}
	
	// Add our custom environment
	filteredEnv = append(filteredEnv,
		fmt.Sprintf("HOME=%s", cs.claudeHome),
		fmt.Sprintf("BUN_INSTALL=%s", cs.bunPath),
		fmt.Sprintf("PATH=%s:%s", filepath.Join(cs.bunPath, "bin"), os.Getenv("PATH")),
	)
	
	return filteredEnv
}

func (cs *ClaudeSetup) GetClaudePath() string {
	return cs.claudePath
}

func (cs *ClaudeSetup) GetClaudeHome() string {
	return cs.claudeHome
}

func (cs *ClaudeSetup) GetBaseDir() string {
	return cs.baseDir
}

func (cs *ClaudeSetup) Setup() error {
	log.Println("Setting up isolated Claude environment...")

	// Install Bun
	if err := cs.InstallBun(); err != nil {
		return fmt.Errorf("failed to install Bun: %w", err)
	}

	// Install Claude
	if err := cs.InstallClaude(); err != nil {
		return fmt.Errorf("failed to install Claude: %w", err)
	}

	// Setup Claude home directory
	if err := cs.SetupClaudeHome(); err != nil {
		return fmt.Errorf("failed to setup Claude home: %w", err)
	}

	// Don't try to authenticate during setup
	// Let the user handle authentication separately

	log.Println("Claude setup completed successfully")
	return nil
}

func (cs *ClaudeSetup) IsAuthenticationNeeded(output string) bool {
	// Check if the output indicates authentication is needed
	return strings.Contains(output, "Invalid API key") || 
	       strings.Contains(output, "not authenticated") ||
	       strings.Contains(output, "Please log in") ||
	       strings.Contains(output, "claude login")
}

func (cs *ClaudeSetup) HandleAuthenticationPrompt(process *ClaudeProcess) error {
	log.Println("Authentication required. Please complete the login process...")
	
	// Kill the current process if it's running
	if process != nil {
		process.Kill()
	}
	
	// Run the login command
	if err := cs.RunClaudeLogin(); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}
	
	return nil
}

// CopyAuthFrom copies authentication config from another directory
func (cs *ClaudeSetup) CopyAuthFrom(sourceDir string) error {
	sourceAuthFile := filepath.Join(sourceDir, "auth.json")
	
	// Check if source auth file exists
	if _, err := os.Stat(sourceAuthFile); os.IsNotExist(err) {
		return fmt.Errorf("no auth.json found in %s", sourceDir)
	}
	
	// Read the source auth file
	authData, err := os.ReadFile(sourceAuthFile)
	if err != nil {
		return fmt.Errorf("failed to read auth file: %w", err)
	}
	
	// Ensure our config directory exists
	configDir := filepath.Join(cs.claudeHome, ".config", "claude")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	
	// Write to our auth file
	destAuthFile := filepath.Join(configDir, "auth.json")
	if err := os.WriteFile(destAuthFile, authData, 0600); err != nil {
		return fmt.Errorf("failed to write auth file: %w", err)
	}
	
	log.Printf("Authentication copied from %s", sourceDir)
	return nil
}

// GetAuthURL returns the URL for Claude CLI authentication
func (cs *ClaudeSetup) GetAuthURL() string {
	// Claude CLI uses browser-based authentication, not API keys
	return "https://console.anthropic.com/login"
}

// SetAuthToken sets the Claude Code CLI authentication token programmatically
// Note: This accepts the session token from a completed Claude CLI login, NOT an API key
func (cs *ClaudeSetup) SetAuthToken(authToken string) error {
	if authToken == "" {
		return fmt.Errorf("auth token cannot be empty")
	}
	
	// Ensure the config directory exists
	configDir := filepath.Join(cs.claudeHome, ".config", "claude")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	
	// Create the auth.json file with the auth token
	// This mimics what Claude Code CLI saves after login
	authFile := filepath.Join(configDir, "auth.json")
	authData := fmt.Sprintf(`{"key":"%s"}`, authToken)
	
	if err := os.WriteFile(authFile, []byte(authData), 0600); err != nil {
		return fmt.Errorf("failed to write auth file: %w", err)
	}
	
	log.Printf("Authentication token saved successfully")
	return nil
}


// WaitForAuth waits for authentication to be completed
// This can be used after SetAuthToken to verify authentication worked
func (cs *ClaudeSetup) WaitForAuth(timeout time.Duration) error {
	startTime := time.Now()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			authenticated, _, err := cs.GetAuthStatus()
			if err != nil {
				return fmt.Errorf("error checking auth status: %w", err)
			}
			if authenticated {
				return nil
			}
			
			if time.Since(startTime) > timeout {
				return fmt.Errorf("authentication timeout after %v", timeout)
			}
		}
	}
}

func PromptUser(prompt string) string {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	return strings.TrimSpace(response)
}