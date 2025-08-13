// Package clauderelay provides a WebSocket relay server for Claude Code CLI
// with isolated, portable installation support.
//
// This package allows you to create multiple isolated Claude instances,
// each with its own configuration and authentication, without requiring
// system-wide installation.
//
// Basic usage:
//
//	import "github.com/yourusername/claude-relay"
//
//	// Create a new relay server
//	relay, err := clauderelay.New(clauderelay.Options{
//		Port:           "8081",
//		BaseDir:        "./claude-instance-1",
//		AutoSetup:      true,
//	})
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer relay.Close()
//
//	// Start the server
//	if err := relay.Start(); err != nil {
//		log.Fatal(err)
//	}
package clauderelay

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"sync"
)

// Options configures the Claude relay server.
type Options struct {
	// Port is the port number for the WebSocket server.
	// Default: "8080"
	Port string

	// BaseDir is the directory where Claude and Bun will be installed.
	// Each instance should have a unique BaseDir for isolation.
	// Default: current directory
	BaseDir string

	// AutoSetup automatically installs Claude and Bun if not present.
	// Default: true
	AutoSetup bool

	// MaxProcesses limits the number of concurrent Claude processes.
	// Default: 100
	MaxProcesses int

	// EnableLogging enables detailed logging output.
	// Default: true
	EnableLogging bool

	// CustomClaudePath allows specifying a custom Claude executable path.
	// If empty, uses the isolated installation.
	CustomClaudePath string

	// APIKey provides the Claude API key for non-interactive authentication.
	// If provided, the library will automatically authenticate without user interaction.
	APIKey string

	// AuthCallback is called when authentication is needed.
	// If provided, this will be used instead of interactive authentication.
	// The callback receives the auth URL and should return the API key.
	AuthCallback func(authURL string) (string, error)
}

// Relay represents a Claude WebSocket relay server instance.
type Relay struct {
	options      Options
	config       *Config
	setup        *ClaudeSetup
	server       *Server
	httpServer   *http.Server
	shutdownChan chan struct{}
	wg           sync.WaitGroup
	mu           sync.RWMutex
	isRunning    bool
}

// New creates a new Claude relay instance with the given options.
//
// Example:
//
//	relay, err := clauderelay.New(clauderelay.Options{
//		Port:    "8081",
//		BaseDir: "./my-claude",
//	})
func New(opts Options) (*Relay, error) {
	// Set defaults
	if opts.Port == "" {
		opts.Port = "8080"
	}
	if opts.BaseDir == "" {
		opts.BaseDir = "."
	}
	if opts.MaxProcesses == 0 {
		opts.MaxProcesses = 100
	}

	// Convert BaseDir to absolute path
	absBaseDir, err := filepath.Abs(opts.BaseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for BaseDir: %w", err)
	}
	opts.BaseDir = absBaseDir

	// Create setup with custom base directory
	setup, err := NewClaudeSetupWithBaseDir(opts.BaseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Claude setup: %w", err)
	}

	// Auto-setup if requested
	if opts.AutoSetup && !setup.IsInstalled() {
		if opts.EnableLogging {
			log.Printf("Claude not installed in %s, starting setup...", opts.BaseDir)
		}
		if err := setup.Setup(); err != nil {
			return nil, fmt.Errorf("failed to setup Claude: %w", err)
		}
	}

	// Handle authentication
	if setup.IsInstalled() {
		authenticated, err := setup.CheckAuthentication()
		if err != nil && opts.EnableLogging {
			log.Printf("Warning: Failed to check authentication: %v", err)
		}
		
		if !authenticated {
			// Try to authenticate using provided options
			if opts.APIKey != "" {
				if opts.EnableLogging {
					log.Printf("Setting up authentication with provided API key")
				}
				if err := setup.SetAuthToken(opts.APIKey); err != nil {
					return nil, fmt.Errorf("failed to set API key: %w", err)
				}
			} else if opts.AuthCallback != nil {
				if opts.EnableLogging {
					log.Printf("Using auth callback for authentication")
				}
				authURL, _ := setup.GetAuthURL()
				apiKey, err := opts.AuthCallback(authURL)
				if err != nil {
					return nil, fmt.Errorf("auth callback failed: %w", err)
				}
				if err := setup.SetAuthToken(apiKey); err != nil {
					return nil, fmt.Errorf("failed to set API key from callback: %w", err)
				}
			} else if opts.EnableLogging {
				log.Printf("Warning: Claude is not authenticated. Use SetAuthToken() or Authenticate() to login")
			}
		}
	}

	// Determine Claude path
	claudePath := opts.CustomClaudePath
	if claudePath == "" {
		claudePath = setup.GetClaudePath()
	}

	config := &Config{
		Port:         opts.Port,
		ClaudePath:   claudePath,
		MaxProcesses: opts.MaxProcesses,
	}

	server := NewServer(config, setup)

	return &Relay{
		options:      opts,
		config:       config,
		setup:        setup,
		server:       server,
		shutdownChan: make(chan struct{}),
	}, nil
}

// Start starts the WebSocket relay server.
// The server runs in the background and returns immediately.
//
// Example:
//
//	if err := relay.Start(); err != nil {
//		log.Fatal(err)
//	}
func (r *Relay) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.isRunning {
		return fmt.Errorf("relay is already running")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", r.server.HandleWebSocket)
	mux.HandleFunc("/health", r.handleHealth)

	r.httpServer = &http.Server{
		Addr:    ":" + r.config.Port,
		Handler: mux,
	}

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		if r.options.EnableLogging {
			log.Printf("Starting Claude relay server on port %s", r.config.Port)
			log.Printf("WebSocket endpoint: ws://localhost:%s/ws", r.config.Port)
			log.Printf("Claude installation: %s", r.setup.GetClaudePath())
			log.Printf("Claude home: %s", r.setup.GetClaudeHome())
		}
		if err := r.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			if r.options.EnableLogging {
				log.Printf("Server error: %v", err)
			}
		}
	}()

	r.isRunning = true
	return nil
}

// Stop gracefully stops the relay server.
//
// Example:
//
//	if err := relay.Stop(); err != nil {
//		log.Printf("Error stopping relay: %v", err)
//	}
func (r *Relay) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.isRunning {
		return nil
	}

	if r.options.EnableLogging {
		log.Println("Stopping Claude relay server...")
	}

	if r.httpServer != nil {
		if err := r.httpServer.Close(); err != nil {
			return fmt.Errorf("failed to close HTTP server: %w", err)
		}
	}

	close(r.shutdownChan)
	r.wg.Wait()
	r.isRunning = false

	if r.options.EnableLogging {
		log.Println("Claude relay server stopped")
	}

	return nil
}

// Close stops the server and cleans up resources.
// It's safe to call Close multiple times.
//
// Example:
//
//	defer relay.Close()
func (r *Relay) Close() error {
	if err := r.Stop(); err != nil {
		return err
	}
	// Additional cleanup if needed
	return nil
}

// IsRunning returns true if the relay server is currently running.
func (r *Relay) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.isRunning
}

// GetPort returns the port the server is configured to use.
func (r *Relay) GetPort() string {
	return r.config.Port
}

// GetBaseDir returns the base directory for this Claude instance.
func (r *Relay) GetBaseDir() string {
	return r.options.BaseDir
}

// GetWebSocketURL returns the WebSocket URL for connecting to this relay.
func (r *Relay) GetWebSocketURL() string {
	return fmt.Sprintf("ws://localhost:%s/ws", r.config.Port)
}

// IsAuthenticated checks if Claude is authenticated.
func (r *Relay) IsAuthenticated() (bool, error) {
	return r.setup.CheckAuthentication()
}

// GetAuthStatus returns detailed authentication status.
// Returns: (isAuthenticated, statusMessage, error)
func (r *Relay) GetAuthStatus() (bool, string, error) {
	return r.setup.GetAuthStatus()
}

// Authenticate runs the Claude authentication process interactively.
// This should be called when IsAuthenticated returns false.
//
// Example:
//
//	authenticated, _ := relay.IsAuthenticated()
//	if !authenticated {
//		fmt.Println("Please complete Claude authentication:")
//		if err := relay.Authenticate(); err != nil {
//			log.Fatal(err)
//		}
//	}
func (r *Relay) Authenticate() error {
	return r.setup.RunClaudeLogin()
}

// GetAuthURL returns the URL where users can get their API key.
// Use this for non-interactive authentication flows.
//
// Example:
//
//	url, _ := relay.GetAuthURL()
//	fmt.Printf("Get your API key at: %s\n", url)
//	// ... show URL to user ...
//	relay.SetAuthToken(apiKey)
func (r *Relay) GetAuthURL() (string, error) {
	return r.setup.GetAuthURL()
}

// SetAuthToken sets the API key for non-interactive authentication.
// This method allows programmatic authentication without terminal access.
//
// Example:
//
//	// Get API key from environment, database, or user input
//	apiKey := os.Getenv("CLAUDE_API_KEY")
//	if err := relay.SetAuthToken(apiKey); err != nil {
//		log.Fatal(err)
//	}
func (r *Relay) SetAuthToken(apiKey string) error {
	return r.setup.SetAuthToken(apiKey)
}

// AuthenticateWithCallback performs non-interactive authentication using a callback.
// The callback receives the auth URL and should return the API key.
//
// Example:
//
//	err := relay.AuthenticateWithCallback(func(authURL string) (string, error) {
//		fmt.Printf("Please visit: %s\n", authURL)
//		fmt.Print("Enter your API key: ")
//		var apiKey string
//		fmt.Scanln(&apiKey)
//		return apiKey, nil
//	})
func (r *Relay) AuthenticateWithCallback(callback func(authURL string) (string, error)) error {
	// Get the auth URL
	authURL, err := r.GetAuthURL()
	if err != nil {
		return fmt.Errorf("failed to get auth URL: %w", err)
	}
	
	// Call the callback to get the API key
	apiKey, err := callback(authURL)
	if err != nil {
		return fmt.Errorf("callback failed: %w", err)
	}
	
	// Set the auth token
	if err := r.SetAuthToken(apiKey); err != nil {
		return fmt.Errorf("failed to set auth token: %w", err)
	}
	
	// Verify authentication worked
	authenticated, status, err := r.GetAuthStatus()
	if err != nil {
		return fmt.Errorf("failed to verify authentication: %w", err)
	}
	
	if !authenticated {
		return fmt.Errorf("authentication failed: %s", status)
	}
	
	return nil
}

// Setup installs Claude and Bun if not already installed.
// This is called automatically if AutoSetup is true.
func (r *Relay) Setup() error {
	return r.setup.Setup()
}

// IsInstalled checks if Claude and Bun are installed.
func (r *Relay) IsInstalled() bool {
	return r.setup.IsInstalled()
}

// GetSetup returns the underlying ClaudeSetup instance for advanced usage.
func (r *Relay) GetSetup() *ClaudeSetup {
	return r.setup
}

// GetServer returns the underlying Server instance for advanced usage.
func (r *Relay) GetServer() *Server {
	return r.server
}

func (r *Relay) handleHealth(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "OK")
}

// Wait blocks until the server is stopped.
//
// Example:
//
//	go relay.Start()
//	relay.Wait() // Block until stopped
func (r *Relay) Wait() {
	<-r.shutdownChan
}