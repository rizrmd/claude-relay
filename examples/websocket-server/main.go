// WebSocket server example for Claude relay.
// This example shows how to build a WebSocket server that provides
// a web interface for interacting with Claude CLI.
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	clauderelay "github.com/rizrmd/claude-relay"
)

// Options configures the WebSocket relay server.
type Options struct {
	Port             string
	BaseDir          string
	AutoSetup        bool
	MaxProcesses     int
	EnableLogging    bool
	CustomClaudePath string
	AuthToken        string
	PreAuthDirectory string
}

// Relay represents a Claude WebSocket relay server instance.
type Relay struct {
	options      Options
	config       *clauderelay.Config
	setup        *clauderelay.ClaudeSetup
	server       *Server
	httpServer   *http.Server
	shutdownChan chan struct{}
	wg           sync.WaitGroup
	mu           sync.RWMutex
	isRunning    bool
}

// New creates a new Claude relay instance with the given options.
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
	setup, err := clauderelay.NewClaudeSetupWithBaseDir(opts.BaseDir)
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

	config := &clauderelay.Config{
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
func (r *Relay) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.isRunning {
		return fmt.Errorf("relay is already running")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", r.server.HandleWebSocket)
	mux.HandleFunc("/health", r.handleHealth)
	mux.HandleFunc("/", r.handleClient)

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
			log.Printf("Web UI: http://localhost:%s", r.config.Port)
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
func (r *Relay) Close() error {
	if err := r.Stop(); err != nil {
		return err
	}
	return nil
}

func (r *Relay) handleHealth(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "OK")
}

func (r *Relay) handleClient(w http.ResponseWriter, req *http.Request) {
	http.ServeFile(w, req, "client.html")
}

// IsAuthenticated checks if Claude is authenticated.
func (r *Relay) IsAuthenticated() (bool, error) {
	return r.setup.CheckAuthentication()
}

// Authenticate runs the Claude authentication process interactively.
func (r *Relay) Authenticate() error {
	return r.setup.RunClaudeLogin()
}

func main() {
	// Parse command line arguments
	port := "8080"
	baseDir := "."
	
	if len(os.Args) > 1 {
		port = os.Args[1]
	}
	if len(os.Args) > 2 {
		baseDir = os.Args[2]
	}

	// Create and start relay
	relay, err := New(Options{
		Port:          port,
		BaseDir:       baseDir,
		AutoSetup:     true,
		EnableLogging: true,
		MaxProcesses:  100,
	})
	if err != nil {
		log.Fatal("Failed to create relay:", err)
	}
	defer relay.Close()

	// Check authentication
	authenticated, err := relay.IsAuthenticated()
	if err != nil {
		log.Printf("Warning: Failed to check authentication: %v", err)
	}

	if !authenticated {
		fmt.Println("========================================")
		fmt.Println("⚠️  Claude is not authenticated!")
		fmt.Println()
		fmt.Println("Please run authentication separately:")
		fmt.Println("  go run ../../cmd/claude-relay -auth")
		fmt.Println()
		fmt.Println("Or use the existing auth from main installation:")
		fmt.Println("  cp -r ../../.claude-home/.config/claude .claude-home/.config/")
		fmt.Println("========================================")
		log.Fatal("Authentication required before starting server")
	}

	// Start server
	if err := relay.Start(); err != nil {
		log.Fatal("Failed to start relay:", err)
	}

	fmt.Printf("\nClaude WebSocket relay server started\n")
	fmt.Printf("Web UI: http://localhost:%s\n", port)
	fmt.Printf("WebSocket endpoint: ws://localhost:%s/ws\n", port)
	fmt.Println("\nPress Ctrl+C to stop...")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down...")
}