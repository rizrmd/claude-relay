package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	clauderelay "github.com/rizrmd/claude-relay"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Server struct {
	config       *clauderelay.Config
	connections  map[*Connection]bool
	connectMutex sync.RWMutex
	setup        *clauderelay.ClaudeSetup
}

type Connection struct {
	ws      *websocket.Conn
	process *clauderelay.ClaudeProcess
	server  *Server
	send    chan []byte
	done    chan bool
}

func NewServer(config *clauderelay.Config, setup *clauderelay.ClaudeSetup) *Server {
	return &Server{
		config:      config,
		connections: make(map[*Connection]bool),
		setup:       setup,
	}
}

func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}

	conn := &Connection{
		ws:     ws,
		server: s,
		send:   make(chan []byte, 256),
		done:   make(chan bool),
	}

	s.connectMutex.Lock()
	s.connections[conn] = true
	s.connectMutex.Unlock()

	process, err := clauderelay.NewClaudeProcess(s.setup)
	if err != nil {
		log.Printf("Failed to create Claude process: %v", err)
		ws.Close()
		return
	}
	conn.process = process

	go conn.writePump()
	go conn.readPump()

	log.Printf("New WebSocket connection established")
}

func (conn *Connection) readPump() {
	defer func() {
		conn.cleanup()
	}()

	conn.ws.SetReadLimit(512 * 1024)

	for {
		_, message, err := conn.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		messageStr := string(message)
		
		// Handle special commands
		if strings.HasPrefix(messageStr, "/") {
			conn.handleCommand(messageStr)
			continue
		}
		
		// Regular message processing
		if conn.process != nil {
			// Send initial thinking indicator
			select {
			case conn.send <- []byte("🤔 Claude is thinking..."):
			case <-conn.done:
				return
			}
			
			// Process message with Claude
			response, err := conn.process.SendMessage(messageStr)
			if err != nil {
				// Check if it's an auth error
				if strings.Contains(err.Error(), "authentication") || strings.Contains(err.Error(), "Invalid API key") {
					conn.send <- []byte("AUTH_REQUIRED")
					continue
				}
				
				errMsg := fmt.Sprintf("❌ Error: %v", err)
				select {
				case conn.send <- []byte(errMsg):
				case <-conn.done:
					return
				}
				continue
			}
			
			// Send response
			select {
			case conn.send <- []byte(response):
			case <-conn.done:
				return
			}
		} else {
			// No process available, likely need auth
			conn.send <- []byte("AUTH_REQUIRED")
		}
	}
}

func (conn *Connection) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		conn.ws.Close()
	}()

	for {
		select {
		case message, ok := <-conn.send:
			conn.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				conn.ws.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := conn.ws.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			conn.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-conn.done:
			return
		}
	}
}

func (conn *Connection) handleCommand(command string) {
	parts := strings.SplitN(command, ":", 2)
	cmd := strings.TrimSpace(parts[0])
	
	switch cmd {
	case "/auth-status":
		// Check authentication status
		authenticated, err := conn.server.setup.CheckAuthentication()
		if err != nil {
			conn.send <- []byte(fmt.Sprintf("AUTH_STATUS:error:%v", err))
			return
		}
		
		if authenticated {
			conn.send <- []byte("AUTH_STATUS:authenticated")
		} else {
			// Get auth URL
			authURL, sessionID, err := conn.server.setup.StartAuth()
			if err != nil {
				conn.send <- []byte(fmt.Sprintf("AUTH_STATUS:error:%v", err))
				return
			}
			conn.send <- []byte(fmt.Sprintf("AUTH_STATUS:not_authenticated:%s:%s", authURL, sessionID))
		}
		
	case "/auth-token":
		// Set authentication token
		if len(parts) < 2 {
			conn.send <- []byte("AUTH_ERROR:Token required")
			return
		}
		
		token := strings.TrimSpace(parts[1])
		if err := conn.server.setup.CompleteAuth(token); err != nil {
			conn.send <- []byte(fmt.Sprintf("AUTH_ERROR:%v", err))
			return
		}
		
		// Verify authentication worked
		authenticated, _ := conn.server.setup.CheckAuthentication()
		if authenticated {
			// Create new process for this connection
			process, err := clauderelay.NewClaudeProcess(conn.server.setup)
			if err != nil {
				conn.send <- []byte(fmt.Sprintf("AUTH_ERROR:Failed to create process: %v", err))
				return
			}
			conn.process = process
			conn.send <- []byte("AUTH_SUCCESS")
		} else {
			conn.send <- []byte("AUTH_ERROR:Authentication failed")
		}
		
	case "/ping":
		conn.send <- []byte("PONG")
		
	default:
		conn.send <- []byte(fmt.Sprintf("UNKNOWN_COMMAND:%s", cmd))
	}
}

func (conn *Connection) cleanup() {
	conn.server.connectMutex.Lock()
	delete(conn.server.connections, conn)
	conn.server.connectMutex.Unlock()

	close(conn.done)
	close(conn.send)

	if conn.process != nil {
		if err := conn.process.Kill(); err != nil {
			log.Printf("Failed to kill process: %v", err)
		}
		if err := conn.process.Cleanup(); err != nil {
			log.Printf("Failed to cleanup process: %v", err)
		}
	}

	conn.ws.Close()
	log.Printf("WebSocket connection closed")
}