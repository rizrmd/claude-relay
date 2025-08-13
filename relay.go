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

	// AuthToken provides a pre-existing Claude Code CLI auth token for reuse.
	// This is obtained from a previous Claude Code CLI login session.
	// Note: This is NOT a Claude API key.
	AuthToken string

	// PreAuthDirectory specifies a directory containing pre-authenticated Claude config.
	// Copy from another machine's .claude-home/.config/claude/ directory.
	PreAuthDirectory string
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
			if opts.AuthToken != "" {
				if opts.EnableLogging {
					log.Printf("Setting up authentication with provided auth token")
				}
				if err := setup.SetAuthToken(opts.AuthToken); err != nil {
					return nil, fmt.Errorf("failed to set auth token: %w", err)
				}
			} else if opts.PreAuthDirectory != "" {
				if opts.EnableLogging {
					log.Printf("Copying pre-authenticated config from %s", opts.PreAuthDirectory)
				}
				if err := setup.CopyAuthFrom(opts.PreAuthDirectory); err != nil {
					return nil, fmt.Errorf("failed to copy auth config: %w", err)
				}
			} else if opts.EnableLogging {
				log.Printf("Warning: Claude Code CLI is not authenticated. Use Authenticate() to login interactively")
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

// GetAuthConfigPath returns the path where Claude Code CLI stores authentication.
// This can be used to backup or transfer authentication between machines.
//
// Example:
//
//	path := relay.GetAuthConfigPath()
//	// Returns: "/path/to/.claude-home/.config/claude"
func (r *Relay) GetAuthConfigPath() string {
	return filepath.Join(r.setup.GetClaudeHome(), ".config", "claude")
}

// CopyAuthFrom copies authentication from another Claude installation.
// Use this to reuse authentication from another machine or instance.
//
// Example:
//
//	// Copy auth from another instance
//	err := relay.CopyAuthFrom("/backup/claude-auth/")
func (r *Relay) CopyAuthFrom(sourceDir string) error {
	return r.setup.CopyAuthFrom(sourceDir)
}

// SetAuthToken sets a pre-existing Claude Code CLI auth token.
// Note: This is NOT a Claude API key, but the token from Claude Code CLI login.
// This method is mainly for advanced use cases where you have the raw token.
//
// Example:
//
//	// If you somehow have the auth token from Claude Code CLI
//	token := getClaudeAuthToken() // from previous session
//	if err := relay.SetAuthToken(token); err != nil {
//		log.Fatal(err)
//	}
func (r *Relay) SetAuthToken(authToken string) error {
	return r.setup.SetAuthToken(authToken)
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