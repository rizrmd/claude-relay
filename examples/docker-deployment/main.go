// Example of deploying Claude relay in Docker/production environments.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/rizrmd/claude-relay"
)

func main() {
	// In production/Docker, you typically:
	// 1. Authenticate once on dev machine
	// 2. Copy auth files into Docker image or mount as volume
	// 3. Use PreAuthDirectory to point to those files

	// Check for pre-authenticated config
	authDir := "/auth/claude" // Docker volume mount point
	if envAuthDir := os.Getenv("CLAUDE_AUTH_DIR"); envAuthDir != "" {
		authDir = envAuthDir
	}

	// Create relay
	relay, err := clauderelay.New(clauderelay.Options{
		Port:             "8081",
		BaseDir:          "/app/claude", // Container's working directory
		AutoSetup:        true,
		EnableLogging:    true,
		PreAuthDirectory: authDir,
	})
	if err != nil {
		log.Fatal("Failed to create relay:", err)
	}
	defer relay.Close()

	// Verify authentication
	authenticated, status, _ := relay.GetAuthStatus()
	if !authenticated {
		fmt.Printf("Authentication failed: %s\n", status)
		fmt.Println("\nTo fix this:")
		fmt.Println("1. Run Claude relay on a machine with browser access")
		fmt.Println("2. Complete the /login process")
		fmt.Println("3. Copy .claude-home/.config/claude/ to your Docker image")
		fmt.Println("4. Mount it at /auth/claude or set CLAUDE_AUTH_DIR")
		os.Exit(1)
	}

	// Start the server
	if err := relay.Start(); err != nil {
		log.Fatal("Failed to start relay:", err)
	}

	fmt.Printf("Claude relay running at %s\n", relay.GetWebSocketURL())
	fmt.Println("Authentication loaded from:", authDir)

	// In production, you might want health checks
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if relay.IsRunning() {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "healthy")
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, "unhealthy")
		}
	})

	go http.ListenAndServe(":8080", nil)
	fmt.Println("Health check available at http://localhost:8080/health")

	// Keep running
	select {}
}