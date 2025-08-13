// Example of embedding Claude relay in an existing HTTP server.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/yourusername/claude-relay"
)

type Server struct {
	relay *clauderelay.Relay
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"running":       s.relay.IsRunning(),
		"port":          s.relay.GetPort(),
		"websocket_url": s.relay.GetWebSocketURL(),
		"base_dir":      s.relay.GetBaseDir(),
	}

	authenticated, err := s.relay.IsAuthenticated()
	if err == nil {
		status["authenticated"] = authenticated
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (s *Server) handleStart(w http.ResponseWriter, r *http.Request) {
	if s.relay.IsRunning() {
		http.Error(w, "Relay is already running", http.StatusBadRequest)
		return
	}

	if err := s.relay.Start(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to start relay: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Relay started on %s", s.relay.GetWebSocketURL())
}

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	if !s.relay.IsRunning() {
		http.Error(w, "Relay is not running", http.StatusBadRequest)
		return
	}

	if err := s.relay.Stop(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to stop relay: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Relay stopped")
}

func main() {
	// Create Claude relay instance
	relay, err := clauderelay.New(clauderelay.Options{
		Port:          "8081",
		BaseDir:       "./claude-embedded",
		AutoSetup:     true,
		EnableLogging: true,
	})
	if err != nil {
		log.Fatal("Failed to create relay:", err)
	}
	defer relay.Close()

	// Start the relay
	if err := relay.Start(); err != nil {
		log.Fatal("Failed to start relay:", err)
	}

	server := &Server{relay: relay}

	// Set up HTTP routes
	http.HandleFunc("/api/status", server.handleStatus)
	http.HandleFunc("/api/start", server.handleStart)
	http.HandleFunc("/api/stop", server.handleStop)
	
	// Serve the client HTML
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "client.html")
	})

	// Start HTTP server
	go func() {
		fmt.Println("Control API server running on http://localhost:8080")
		fmt.Printf("Claude WebSocket available at %s\n", relay.GetWebSocketURL())
		fmt.Println("Visit http://localhost:8080 to use the web interface")
		
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatal("HTTP server error:", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down...")
}