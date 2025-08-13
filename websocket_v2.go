package clauderelay

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

type ServerV2 struct {
	config       *Config
	connections  map[*ConnectionV2]bool
	connectMutex sync.RWMutex
}

type ConnectionV2 struct {
	ws      *websocket.Conn
	process *ClaudeProcessV2
	server  *ServerV2
	send    chan string
	done    chan bool
}

func NewServerV2(config *Config) *ServerV2 {
	return &ServerV2{
		config:      config,
		connections: make(map[*ConnectionV2]bool),
	}
}

func (s *ServerV2) HandleWebSocketV2(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}

	process, err := NewClaudeProcessV2()
	if err != nil {
		log.Printf("Failed to create Claude process: %v", err)
		ws.Close()
		return
	}

	conn := &ConnectionV2{
		ws:      ws,
		server:  s,
		process: process,
		send:    make(chan string, 256),
		done:    make(chan bool),
	}

	s.connectMutex.Lock()
	s.connections[conn] = true
	s.connectMutex.Unlock()

	go conn.writePump()
	go conn.readPump()

	log.Printf("New WebSocket connection established (V2)")
}

func (conn *ConnectionV2) readPump() {
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
		log.Printf("Received from client: %s", messageStr)

		// Process message with Claude
		go func(msg string) {
			response, err := conn.process.SendMessage(msg)
			if err != nil {
				log.Printf("Error processing message: %v", err)
				errMsg := fmt.Sprintf("Error: %v", err)
				select {
				case conn.send <- errMsg:
				case <-conn.done:
				}
				return
			}

			// Send response back to client
			// Split by lines to send each line separately for better streaming
			lines := strings.Split(response, "\n")
			for _, line := range lines {
				if line != "" {
					select {
					case conn.send <- line:
					case <-conn.done:
						return
					}
				}
			}
		}(messageStr)
	}
}

func (conn *ConnectionV2) writePump() {
	defer func() {
		conn.ws.Close()
	}()

	for {
		select {
		case message, ok := <-conn.send:
			if !ok {
				conn.ws.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := conn.ws.WriteMessage(websocket.TextMessage, []byte(message)); err != nil {
				log.Printf("Error writing to websocket: %v", err)
				return
			}

		case <-conn.done:
			return
		}
	}
}

func (conn *ConnectionV2) cleanup() {
	conn.server.connectMutex.Lock()
	delete(conn.server.connections, conn)
	conn.server.connectMutex.Unlock()

	close(conn.done)
	close(conn.send)

	if conn.process != nil {
		if err := conn.process.Cleanup(); err != nil {
			log.Printf("Failed to cleanup: %v", err)
		}
	}

	conn.ws.Close()
	log.Printf("WebSocket connection closed and cleaned up (V2)")
}