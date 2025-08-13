package main

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"

	clauderelay "github.com/rizrmd/claude-relay"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// ANSI escape code regex to strip formatting
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[mK]|\x1b\[2J|\x1b\[[0-9]+;[0-9]+[Hf]|\x1b\[[0-9]*[ABCDEFGH]|\x1b\[[?][0-9;]*[hlH]|\x1b\[[0-9]*[JKABCDEFG]`)

// Patterns to filter out UI elements and non-content
var uiFilterRegexes = []*regexp.Regexp{
	regexp.MustCompile(`^╭─+╮$`),                                    // Top border
	regexp.MustCompile(`^╰─+╯$`),                                    // Bottom border  
	regexp.MustCompile(`^│.*│$`),                                    // Side borders (input box)
	regexp.MustCompile(`^\s*>\s*.*$`),                              // Input prompt lines
	regexp.MustCompile(`^\s*bypass permissions on.*$`),             // Footer messages
	regexp.MustCompile(`^\s*Claude.*limit reached.*$`),             // Rate limit messages
	regexp.MustCompile(`^\s*Auto-updating.*$`),                     // Update messages
	regexp.MustCompile(`^\s*/help for help.*$`),                    // Help text
	regexp.MustCompile(`^\s*cwd:.*$`),                              // Working directory
	regexp.MustCompile(`^\s*Tips for getting started:.*$`),         // Tips header
	regexp.MustCompile(`^\s*[0-9]+\.\s*.*$`),                       // Numbered tips
	regexp.MustCompile(`^\s*✻ Welcome to Claude Code!.*$`),         // Welcome message
	regexp.MustCompile(`^\s*※.*$`),                                  // Tip messages
	regexp.MustCompile(`^\s*◯.*$`),                                  // Circle indicators
	regexp.MustCompile(`^\s*✔.*$`),                                  // Checkmarks
	regexp.MustCompile(`^\s*\(shift\+tab to cycle\).*$`),          // UI hints
}

func stripANSI(text string) string {
	return ansiRegex.ReplaceAllString(text, "")
}

func isContentLine(text string) bool {
	// Strip ANSI codes first
	clean := stripANSI(text)
	
	// Skip empty lines
	if strings.TrimSpace(clean) == "" {
		return false
	}
	
	// Check against UI filter patterns
	for _, regex := range uiFilterRegexes {
		if regex.MatchString(clean) {
			return false
		}
	}
	
	// If it passed all filters, it's likely content
	return true
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

		if conn.process != nil {
			messageStr := string(message)
			
			// Check for undo-from command (undo from a specific message)
			if strings.HasPrefix(messageStr, "/undo-from:") {
				parts := strings.Split(messageStr, ":")
				if len(parts) == 2 {
					messageIndex, err := strconv.Atoi(parts[1])
					if err != nil {
						select {
						case conn.send <- []byte("❌ Invalid undo index"):
						case <-conn.done:
							return
						}
						continue
					}
					
					log.Printf("Processing undo-from command: index %d", messageIndex)
					
					// Perform the undo to that specific point
					err = conn.process.UndoToIndex(messageIndex)
					if err != nil {
						select {
						case conn.send <- []byte("❌ Undo failed: " + err.Error()):
						case <-conn.done:
							return
						}
						continue
					}
					
					// Send success confirmation with the index
					select {
					case conn.send <- []byte(fmt.Sprintf("UNDO_SUCCESS:%d", messageIndex)):
					case <-conn.done:
						return
					}
					continue
				}
			}
			
			// Check for restore command
			if messageStr == "/restore" {
				log.Printf("Processing restore command")
				
				if !conn.process.CanRestore() {
					select {
					case conn.send <- []byte("❌ Nothing to restore"):
					case <-conn.done:
						return
					}
					continue
				}
				
				// Get the messages that will be restored for client display
				restoredMessages := conn.process.GetRestoredMessagesForClient()
				
				// Perform the restore
				_, err := conn.process.RestoreLastUndo()
				if err != nil {
					select {
					case conn.send <- []byte("❌ Restore failed: " + err.Error()):
					case <-conn.done:
						return
					}
					continue
				}
				
				// Send restore result to client with the restored messages
				// Format: RESTORE_SUCCESS:{json array of messages}
				var messagesJson strings.Builder
				messagesJson.WriteString("RESTORE_SUCCESS:[")
				for i, msg := range restoredMessages {
					if i > 0 {
						messagesJson.WriteString(",")
					}
					messagesJson.WriteString("{")
					messagesJson.WriteString("\"user\":\"" + strings.ReplaceAll(msg["user"], "\"", "\\\"") + "\",")
					messagesJson.WriteString("\"claude\":\"" + strings.ReplaceAll(msg["claude"], "\"", "\\\"") + "\"")
					messagesJson.WriteString("}")
				}
				messagesJson.WriteString("]")
				
				select {
				case conn.send <- []byte(messagesJson.String()):
				case <-conn.done:
					return
				}
				continue
			}
			
			// Check for simple undo command (legacy support)
			if messageStr == "/undo" || messageStr == "undo" {
				log.Printf("Processing undo command")
				
				if !conn.process.CanUndo() {
					select {
					case conn.send <- []byte("❌ Nothing to undo"):
					case <-conn.done:
						return
					}
					continue
				}
				
				// Perform the undo
				err := conn.process.UndoLastExchange()
				if err != nil {
					select {
					case conn.send <- []byte("❌ Undo failed: " + err.Error()):
					case <-conn.done:
						return
					}
					continue
				}
				
				// Send undo result to client
				select {
				case conn.send <- []byte("UNDO_SUCCESS:last"):
				case <-conn.done:
					return
				}
				continue
			}
			
			log.Printf("Sending to Claude: %s", messageStr)
			
			// Send thinking indicator to client
			select {
			case conn.send <- []byte("🤔 Claude is thinking..."):
			case <-conn.done:
				return
			}
			
			// Use the new SendMessage method that handles context and provides progress updates
			go func() {
				response, err := conn.process.SendMessageWithProgress(messageStr, conn.send, conn.done)
				if err != nil {
					log.Printf("Failed to get response from Claude: %v", err)
					// Send error message to client
					select {
					case conn.send <- []byte("❌ Error: " + err.Error()):
					case <-conn.done:
						return
					}
				} else {
					// Send response directly to client
					select {
					case conn.send <- []byte(response):
					case <-conn.done:
						return
					}
				}
			}()
		}
	}
}

func (conn *Connection) writePump() {
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

			conn.ws.WriteMessage(websocket.TextMessage, message)

		case <-conn.done:
			return
		}
	}
}

func (conn *Connection) processOutputReader() {
	if conn.process == nil || conn.process.pty == nil {
		return
	}

	// Read from PTY
	go func() {
		scanner := bufio.NewScanner(conn.process.pty)
		// Increase buffer size for longer lines
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024)
		
		for scanner.Scan() {
			// Make a copy of the bytes since scanner reuses the buffer
			text := scanner.Text()
			
			// Only process lines that contain actual content (not UI elements)
			if isContentLine(text) {
				cleanText := stripANSI(text)
				log.Printf("Claude content: %s", cleanText)
				select {
				case conn.send <- []byte(cleanText):
				case <-conn.done:
					return
				}
			}
		}
		if err := scanner.Err(); err != nil {
			log.Printf("Error reading PTY: %v", err)
		}
	}()
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
			log.Printf("Failed to cleanup: %v", err)
		}
	}

	conn.ws.Close()
	log.Printf("WebSocket connection closed and cleaned up")
}