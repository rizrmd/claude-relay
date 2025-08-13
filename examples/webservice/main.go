// Example of using Claude relay in a web service with non-interactive authentication.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"claude-relay"
)

type Service struct {
	relay    *clauderelay.Relay
	mu       sync.RWMutex
	apiKeyChan chan string
}

// Handler for setting API key via HTTP
func (s *Service) handleSetAPIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		APIKey string `json:"api_key"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.APIKey == "" {
		http.Error(w, "API key is required", http.StatusBadRequest)
		return
	}

	// Set the API key
	if err := s.relay.SetAuthToken(req.APIKey); err != nil {
		http.Error(w, fmt.Sprintf("Failed to set API key: %v", err), http.StatusInternalServerError)
		return
	}

	// Verify authentication
	authenticated, message, _ := s.relay.GetAuthStatus()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"authenticated": authenticated,
		"message":       message,
	})
}

// Handler to get authentication status
func (s *Service) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	authenticated, message, err := s.relay.GetAuthStatus()
	
	status := map[string]interface{}{
		"authenticated": authenticated,
		"message":       message,
		"auth_url":      "",
	}
	
	if !authenticated {
		url, _ := s.relay.GetAuthURL()
		status["auth_url"] = url
	}
	
	if err != nil {
		status["error"] = err.Error()
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// Handler to get relay information
func (s *Service) handleInfo(w http.ResponseWriter, r *http.Request) {
	info := map[string]interface{}{
		"websocket_url": s.relay.GetWebSocketURL(),
		"port":          s.relay.GetPort(),
		"base_dir":      s.relay.GetBaseDir(),
		"is_running":    s.relay.IsRunning(),
	}
	
	authenticated, _, _ := s.relay.GetAuthStatus()
	info["authenticated"] = authenticated
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

func main() {
	// Try to get API key from environment
	apiKey := os.Getenv("CLAUDE_API_KEY")
	
	// Create relay with optional API key
	// If no API key is provided, it can be set later via the HTTP API
	relay, err := clauderelay.New(clauderelay.Options{
		Port:          "8081",
		BaseDir:       "./claude-webservice",
		AutoSetup:     true,
		EnableLogging: true,
		APIKey:        apiKey, // May be empty, can be set later
	})
	if err != nil {
		log.Fatal("Failed to create relay:", err)
	}
	defer relay.Close()

	// Start the relay
	if err := relay.Start(); err != nil {
		log.Fatal("Failed to start relay:", err)
	}

	service := &Service{
		relay: relay,
	}

	// Set up HTTP routes
	http.HandleFunc("/api/auth/set-key", service.handleSetAPIKey)
	http.HandleFunc("/api/auth/status", service.handleAuthStatus)
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
        input { padding: 8px; width: 300px; margin: 5px; }
    </style>
</head>
<body>
    <h1>Claude Relay Web Service</h1>
    <div id="status" class="status">Loading...</div>
    
    <div id="auth-section" style="display:none;">
        <h2>Authentication Required</h2>
        <p>Get your API key from: <a id="auth-url" href="#" target="_blank"></a></p>
        <input type="password" id="api-key" placeholder="Enter your API key">
        <button onclick="setAPIKey()">Set API Key</button>
    </div>
    
    <div id="info-section" style="display:none;">
        <h2>Relay Information</h2>
        <div id="info"></div>
    </div>

    <script>
        async function checkStatus() {
            const res = await fetch('/api/auth/status');
            const data = await res.json();
            
            const statusDiv = document.getElementById('status');
            const authSection = document.getElementById('auth-section');
            const infoSection = document.getElementById('info-section');
            
            if (data.authenticated) {
                statusDiv.className = 'status authenticated';
                statusDiv.textContent = 'Authenticated: ' + data.message;
                authSection.style.display = 'none';
                infoSection.style.display = 'block';
                loadInfo();
            } else {
                statusDiv.className = 'status not-authenticated';
                statusDiv.textContent = 'Not Authenticated: ' + data.message;
                authSection.style.display = 'block';
                infoSection.style.display = 'none';
                
                if (data.auth_url) {
                    const authLink = document.getElementById('auth-url');
                    authLink.href = data.auth_url;
                    authLink.textContent = data.auth_url;
                }
            }
        }
        
        async function setAPIKey() {
            const apiKey = document.getElementById('api-key').value;
            if (!apiKey) {
                alert('Please enter an API key');
                return;
            }
            
            const res = await fetch('/api/auth/set-key', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({api_key: apiKey})
            });
            
            const data = await res.json();
            if (data.authenticated) {
                document.getElementById('api-key').value = '';
                checkStatus();
            } else {
                alert('Authentication failed: ' + data.message);
            }
        }
        
        async function loadInfo() {
            const res = await fetch('/api/info');
            const data = await res.json();
            document.getElementById('info').innerHTML = '<pre>' + JSON.stringify(data, null, 2) + '</pre>';
        }
        
        // Check status on load
        checkStatus();
        // Refresh every 5 seconds
        setInterval(checkStatus, 5000);
    </script>
</body>
</html>
		`
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, html)
	})

	fmt.Println("========================================")
	fmt.Println("Claude Relay Web Service")
	fmt.Println("========================================")
	fmt.Printf("HTTP API: http://localhost:8080\n")
	fmt.Printf("Claude WebSocket: %s\n", relay.GetWebSocketURL())
	fmt.Println()
	fmt.Println("Endpoints:")
	fmt.Println("  GET  /                   - Web UI")
	fmt.Println("  GET  /api/auth/status    - Check authentication")
	fmt.Println("  POST /api/auth/set-key   - Set API key")
	fmt.Println("  GET  /api/info           - Get relay info")
	fmt.Println("========================================")
	
	if apiKey != "" {
		fmt.Println("✓ Using API key from CLAUDE_API_KEY environment variable")
	} else {
		fmt.Println("⚠ No API key provided. Set via web UI or POST to /api/auth/set-key")
	}
	
	log.Fatal(http.ListenAndServe(":8080", nil))
}