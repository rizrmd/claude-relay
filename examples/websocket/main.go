// Combined examples for Claude Relay - all use cases in one file
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	clauderelay "github.com/rizrmd/claude-relay"
)

var claudeProcess *clauderelay.ClaudeProcess
var mu sync.RWMutex

func setupClaude() error {
	setup, err := clauderelay.New("./claude-combined")
	if err != nil {
		return err
	}

	if !setup.IsInstalled() {
		if err := setup.Setup(); err != nil {
			return err
		}
	}

	authenticated, _ := setup.CheckAuthentication()
	if !authenticated {
		reader := bufio.NewReader(os.Stdin)
		if err := setup.Authenticate(reader); err != nil {
			return err
		}
	}

	claudeProcess, err = clauderelay.NewClaudeProcess(setup)
	return err
}

// Basic CLI mode
func runBasic() {
	fmt.Println("=== Basic CLI Mode ===")
	
	// Read message from stdin or use default
	var message string
	if stat, _ := os.Stdin.Stat(); (stat.Mode() & os.ModeCharDevice) == 0 {
		input, err := io.ReadAll(os.Stdin)
		if err != nil {
			log.Fatal("Failed to read input:", err)
		}
		message = strings.TrimSpace(string(input))
	}
	
	if message == "" {
		message = "Hello Claude! What's 2+2?"
	}

	response, err := claudeProcess.SendMessage(message)
	if err != nil {
		log.Fatal("Failed to send message:", err)
	}

	fmt.Println("Claude:", response)
}

// Web service handlers
func handleAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Message string `json:"message"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Message == "" {
		http.Error(w, "Message is required", http.StatusBadRequest)
		return
	}

	mu.RLock()
	process := claudeProcess
	mu.RUnlock()

	if process == nil {
		http.Error(w, "Claude not initialized", http.StatusInternalServerError)
		return
	}

	response, err := process.SendMessage(req.Message)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to send message: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": response,
	})
}

func handleTest(w http.ResponseWriter, r *http.Request) {
	message := r.URL.Query().Get("message")
	if message == "" {
		message = "Hello Claude!"
	}
	
	response, err := claudeProcess.SendMessage(message)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "You: %s\nClaude: %s", message, response)
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./index.html")
}

func runWebServer() {
	fmt.Println("=== Web Server Mode ===")
	
	// HTTP routes
	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/api/message", handleAPI)
	http.HandleFunc("/test", handleTest)
	http.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./chat.html")
	})
	
	// Serve static files
	http.HandleFunc("/index.html", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./index.html")
	})

	fmt.Println("Claude Relay Combined Examples")
	fmt.Println("========================================")
	fmt.Println("Web UI: http://localhost:8080")
	fmt.Println("Chat UI: http://localhost:8080/chat")
	fmt.Println("API: POST http://localhost:8080/api/message")
	fmt.Println("Test: http://localhost:8080/test?message=Hello")
	fmt.Println("========================================")
	fmt.Println("Press Ctrl+C to stop...")

	// Start server in background
	go func() {
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()

	// Wait for interrupt
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	fmt.Println("\nShutting down...")
}

func main() {
	// Setup Claude
	if err := setupClaude(); err != nil {
		log.Fatal("Failed to setup Claude:", err)
	}
	defer func() {
		if claudeProcess != nil {
			claudeProcess.Kill()
			claudeProcess.Cleanup()
		}
	}()

	// Check for CLI mode flag
	cliMode := false
	for _, arg := range os.Args[1:] {
		if arg == "--cli" {
			cliMode = true
			break
		}
	}

	if cliMode {
		runBasic()
	} else {
		runWebServer()
	}
}