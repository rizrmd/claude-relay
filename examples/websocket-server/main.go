// WebSocket server example for Claude relay.
// This example shows how to build a WebSocket server that provides
// a web interface for interacting with Claude CLI.
package main

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
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

	// Check for session token in environment variable (for non-interactive deployments)
	if sessionToken := os.Getenv("CLAUDE_SESSION_TOKEN"); sessionToken != "" {
		log.Println("Found CLAUDE_SESSION_TOKEN in environment, attempting authentication...")
		if err := relay.setup.CompleteNonInteractiveAuth(sessionToken); err != nil {
			log.Printf("Warning: Failed to authenticate with session token: %v", err)
		}
	}
	
	// Check authentication
	authenticated, err := relay.IsAuthenticated()
	if err != nil {
		log.Printf("Warning: Failed to check authentication: %v", err)
	}

	if !authenticated {
		fmt.Println("========================================")
		fmt.Println("⚠️  Claude CLI is not authenticated!")
		fmt.Println()
		fmt.Println("Choose authentication method:")
		fmt.Println()
		fmt.Println("1. Non-Interactive (for servers/automation)")
		fmt.Println("2. Interactive (opens Claude CLI)")
		fmt.Println("3. Use existing auth files")
		fmt.Println()
		fmt.Print("Enter choice (1-3): ")
		
		reader := bufio.NewReader(os.Stdin)
		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)
		
		switch choice {
		case "1":
			// Non-interactive authentication
			fmt.Println("\n=== Non-Interactive Authentication ===")
			
			// Get the authentication URL
			authURL, sessionID, err := relay.setup.StartNonInteractiveAuth()
			if err != nil {
				log.Fatal("Failed to start authentication:", err)
			}
			
			fmt.Println("\n📌 Authentication Instructions:")
			fmt.Printf("1. Visit this URL in your browser:\n   %s\n", authURL)
			fmt.Println("2. Complete the login process")
			fmt.Println("3. After successful login, you'll receive a session token")
			fmt.Printf("4. Session ID: %s\n", sessionID)
			fmt.Println()
			fmt.Println("Enter the session token you received after login:")
			fmt.Print("Token: ")
			
			token, err := reader.ReadString('\n')
			if err != nil {
				log.Fatal("Failed to read token:", err)
			}
			token = strings.TrimSpace(token)
			
			// Complete authentication with the token
			if err := relay.setup.CompleteNonInteractiveAuth(token); err != nil {
				log.Fatal("Failed to complete authentication:", err)
			}
			
		case "2":
			// Interactive authentication
			fmt.Println("\n=== Interactive Authentication ===")
			fmt.Println("Starting Claude CLI for authentication...")
			fmt.Println("When Claude opens, type: /login")
			fmt.Println("Then follow the browser authentication flow.")
			fmt.Println()
			
			if err := relay.setup.RunInteractiveAuth(); err != nil {
				log.Fatal("Authentication failed:", err)
			}
			
		case "3":
			// Use existing auth
			fmt.Println("\n=== Using Existing Authentication ===")
			fmt.Println("Copy auth files from another installation:")
			fmt.Println("cp -r /path/to/.claude-home/.config/claude .claude-home/.config/")
			fmt.Println()
			log.Fatal("Please copy the auth files and restart the server")
			
		default:
			log.Fatal("Invalid choice")
		}
		
		// Verify authentication worked
		authenticated, err = relay.IsAuthenticated()
		if err != nil || !authenticated {
			log.Fatal("Authentication was not completed successfully")
		}
		
		fmt.Println("\n✅ Authentication successful!")
		fmt.Println("========================================")
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