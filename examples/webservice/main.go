// Example of using Claude relay in a web service.
// This demonstrates how to build an HTTP API around the Claude relay for web applications.
// Authentication must be done manually via the included interactive flow.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	clauderelay "github.com/rizrmd/claude-relay"
)

type Service struct {
	setup    *clauderelay.ClaudeSetup
	process  *clauderelay.ClaudeProcess
	mu       sync.RWMutex
}

// Handler to trigger interactive authentication
func (s *Service) handleAuthenticate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if already authenticated
	authenticated, _ := s.setup.CheckAuthentication()
	if authenticated {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"authenticated": true,
			"message":       "Already authenticated",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"authenticated": false,
		"message":       "Authentication required. Please run the server interactively to authenticate.",
		"instructions":  "Stop the server and run it in a terminal where you can complete the authentication flow.",
	})
}

// Handler to get authentication status
func (s *Service) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	authenticated, err := s.setup.CheckAuthentication()
	
	status := map[string]interface{}{
		"authenticated": authenticated,
	}
	
	if !authenticated {
		status["message"] = "Not authenticated. Please restart the server to authenticate."
	} else {
		status["message"] = "Authenticated and ready"
	}
	
	if err != nil {
		status["error"] = err.Error()
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// Handler to send message to Claude
func (s *Service) handleSendMessage(w http.ResponseWriter, r *http.Request) {
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

	s.mu.RLock()
	process := s.process
	s.mu.RUnlock()

	if process == nil {
		http.Error(w, "Not authenticated. Please set token first.", http.StatusUnauthorized)
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

// Handler to get service information
func (s *Service) handleInfo(w http.ResponseWriter, r *http.Request) {
	authenticated, _ := s.setup.CheckAuthentication()
	
	info := map[string]interface{}{
		"claude_installed": s.setup.IsInstalled(),
		"claude_path":      s.setup.GetClaudePath(),
		"claude_home":      s.setup.GetClaudeHome(),
		"authenticated":    authenticated,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

func main() {
	// Create Claude setup manager
	setup, err := clauderelay.New("./claude-webservice")
	if err != nil {
		log.Fatal("Failed to create setup:", err)
	}

	// Install Claude CLI if not already installed
	if !setup.IsInstalled() {
		fmt.Println("Installing Claude CLI...")
		if err := setup.Setup(); err != nil {
			log.Fatal("Failed to install Claude:", err)
		}
		fmt.Println("Claude CLI installed successfully!")
	}

	// Check authentication
	authenticated, _ := setup.CheckAuthentication()
	if !authenticated {
		fmt.Println("Authentication required. Please complete the authentication flow...")
		reader := bufio.NewReader(os.Stdin)
		if err := setup.Authenticate(reader); err != nil {
			log.Fatal("Authentication failed:", err)
		}
		authenticated = true
		fmt.Println("✅ Authentication completed!")
	} else {
		fmt.Println("✅ Already authenticated")
	}

	service := &Service{
		setup: setup,
	}

	// Create initial process
	if authenticated {
		process, err := clauderelay.NewClaudeProcess(setup)
		if err != nil {
			log.Printf("Warning: Failed to create initial process: %v", err)
		} else {
			service.process = process
		}
	}

	// Set up HTTP routes
	http.HandleFunc("/api/auth/authenticate", service.handleAuthenticate)
	http.HandleFunc("/api/auth/status", service.handleAuthStatus)
	http.HandleFunc("/api/message", service.handleSendMessage)
	http.HandleFunc("/api/info", service.handleInfo)
	
	// Serve a simple HTML page for testing
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		html := `
<!DOCTYPE html>
<html>
<head>
    <title>Claude Relay Web Service</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }
        .status { padding: 10px; margin: 10px 0; border-radius: 5px; }
        .authenticated { background: #d4edda; color: #155724; }
        .not-authenticated { background: #f8d7da; color: #721c24; }
        button { padding: 10px 20px; margin: 5px; cursor: pointer; }
        input { padding: 8px; width: 400px; margin: 5px; }
        textarea { width: 100%; height: 100px; padding: 8px; margin: 5px 0; }
        .response { background: #f8f9fa; padding: 10px; border-radius: 5px; margin: 10px 0; }
        pre { white-space: pre-wrap; word-wrap: break-word; }
    </style>
</head>
<body>
    <h1>Claude Relay Web Service</h1>
    <div id="status" class="status">Loading...</div>
    
    <div id="auth-section" style="display:none;">
        <h2>Authentication Required</h2>
        <p id="instructions">Authentication must be completed by restarting the server in interactive mode.</p>
        <p>To authenticate, stop this server and restart it in a terminal where you can complete the authentication flow.</p>
    </div>
    
    <div id="chat-section" style="display:none;">
        <h2>Chat with Claude</h2>
        <textarea id="message" placeholder="Enter your message..."></textarea>
        <button onclick="sendMessage()">Send Message</button>
        <div id="response" class="response" style="display:none;">
            <h3>Claude's Response:</h3>
            <pre id="response-text"></pre>
        </div>
    </div>
    
    <div id="info-section">
        <h2>Service Information</h2>
        <div id="info"></div>
    </div>

    <script>
        async function checkStatus() {
            const res = await fetch('/api/auth/status');
            const data = await res.json();
            
            const statusDiv = document.getElementById('status');
            const authSection = document.getElementById('auth-section');
            const chatSection = document.getElementById('chat-section');
            
            if (data.authenticated) {
                statusDiv.className = 'status authenticated';
                statusDiv.textContent = '✅ Authenticated';
                authSection.style.display = 'none';
                chatSection.style.display = 'block';
            } else {
                statusDiv.className = 'status not-authenticated';
                statusDiv.textContent = '⚠️ Not Authenticated';
                authSection.style.display = 'block';
                chatSection.style.display = 'none';
                
                if (data.instructions) {
                    document.getElementById('instructions').textContent = 
                        'Run this command to get a token: ' + data.instructions;
                }
            }
            
            loadInfo();
        }
        
        // Token setting removed - authentication must be done server-side
        
        async function sendMessage() {
            const message = document.getElementById('message').value;
            if (!message) {
                alert('Please enter a message');
                return;
            }
            
            const responseDiv = document.getElementById('response');
            const responseText = document.getElementById('response-text');
            responseDiv.style.display = 'block';
            responseText.textContent = 'Thinking...';
            
            try {
                const res = await fetch('/api/message', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({message: message})
                });
                
                if (!res.ok) {
                    const text = await res.text();
                    responseText.textContent = 'Error: ' + text;
                    return;
                }
                
                const data = await res.json();
                responseText.textContent = data.response;
            } catch (err) {
                responseText.textContent = 'Error: ' + err.message;
            }
        }
        
        async function loadInfo() {
            const res = await fetch('/api/info');
            const data = await res.json();
            document.getElementById('info').innerHTML = '<pre>' + JSON.stringify(data, null, 2) + '</pre>';
        }
        
        // Check status on load
        checkStatus();
        // Refresh status every 10 seconds
        setInterval(checkStatus, 10000);
    </script>
</body>
</html>
		`
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, html)
	})

	// Clean up on exit
	defer func() {
		if service.process != nil {
			service.process.Kill()
			service.process.Cleanup()
		}
	}()

	fmt.Println("========================================")
	fmt.Println("Claude Relay Web Service")
	fmt.Println("========================================")
	fmt.Printf("HTTP API: http://localhost:8080\n")
	fmt.Println()
	fmt.Println("Endpoints:")
	fmt.Println("  GET  /                       - Web UI")
	fmt.Println("  GET  /api/auth/status        - Check authentication")
	fmt.Println("  POST /api/auth/authenticate  - Trigger authentication (info only)")
	fmt.Println("  POST /api/message            - Send message to Claude")
	fmt.Println("  GET  /api/info               - Get service info")
	fmt.Println("========================================")
	
	if authenticated {
		fmt.Println("✅ Ready to serve requests")
	} else {
		fmt.Println("⚠️  Authentication required during startup")
	}
	
	log.Fatal(http.ListenAndServe(":8080", nil))
}