// Command claude-relay starts a WebSocket relay server for Claude Code CLI.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	clauderelay "github.com/rizrmd/claude-relay"
)

var (
	port    = flag.String("port", "8081", "Server port")
	baseDir = flag.String("dir", ".", "Base directory for Claude installation")
	verbose = flag.Bool("verbose", true, "Enable verbose logging")
)

func main() {
	flag.Parse()

	// Create relay instance
	relay, err := clauderelay.New(clauderelay.Options{
		Port:          *port,
		BaseDir:       *baseDir,
		AutoSetup:     true,
		EnableLogging: *verbose,
	})
	if err != nil {
		log.Fatal("Failed to create relay: ", err)
	}
	defer relay.Close()

	// Check authentication
	authenticated, err := relay.IsAuthenticated()
	if err != nil && *verbose {
		log.Printf("Warning: Failed to check authentication: %v", err)
	}

	if !authenticated {
		fmt.Println("========================================")
		fmt.Println("Claude needs authentication")
		fmt.Println("Please complete the login process")
		fmt.Println("When Claude starts, choose a theme (press 1 for dark mode)")
		fmt.Println("Then type /login to authenticate")
		fmt.Println("========================================")
		
		if err := relay.Authenticate(); err != nil {
			log.Fatal("Failed to authenticate: ", err)
		}
	}

	// Start the relay server
	if err := relay.Start(); err != nil {
		log.Fatal("Failed to start relay: ", err)
	}

	fmt.Printf("Claude relay server started on %s\n", relay.GetWebSocketURL())
	fmt.Printf("Open client.html in your browser or connect to %s\n", relay.GetWebSocketURL())

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down...")
}