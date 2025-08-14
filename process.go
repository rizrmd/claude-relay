package clauderelay

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type ConversationState struct {
	History   []string
	Timestamp time.Time
}


type ClaudeProcess struct {
	cmd              *exec.Cmd
	pty              *os.File
	tempDir          string
	conversationHistory []string
	conversationStates  []ConversationState
	lastUndoneHistory   []string  // Store the last undone conversation for restore
	setup            *ClaudeSetup
}

func NewClaudeProcess(setup *ClaudeSetup) (*ClaudeProcess, error) {
	// First ensure we have a config file to skip welcome
	configDir := filepath.Join(setup.GetClaudeHome(), ".config", "claude")
	os.MkdirAll(configDir, 0755)
	
	configFile := filepath.Join(configDir, "config.json")
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		// Create config with theme to skip welcome
		config := `{"theme":"dark","outputStyle":"default"}`
		os.WriteFile(configFile, []byte(config), 0644)
	}

	tempDir, err := ioutil.TempDir("", "claude-relay-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	// We don't actually need to start a persistent process
	// since we use --print mode for each message
	// Just return the structure
	return &ClaudeProcess{
		cmd:              nil,
		pty:              nil,
		tempDir:          tempDir,
		conversationHistory: []string{},
		conversationStates:  []ConversationState{},
		lastUndoneHistory:   nil,
		setup:            setup,
	}, nil
}

func (p *ClaudeProcess) Kill() error {
	if p.cmd != nil && p.cmd.Process != nil {
		log.Printf("Killing process PID: %d", p.cmd.Process.Pid)
		if err := p.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %w", err)
		}
		p.cmd.Wait()
	}
	return nil
}

func (p *ClaudeProcess) Cleanup() error {
	if p.pty != nil {
		p.pty.Close()
	}

	if p.tempDir != "" {
		if err := os.RemoveAll(p.tempDir); err != nil {
			return fmt.Errorf("failed to remove temp directory: %w", err)
		}
	}

	return nil
}

func (p *ClaudeProcess) IsRunning() bool {
	if p.cmd == nil || p.cmd.Process == nil {
		return false
	}
	
	process, err := os.FindProcess(p.cmd.Process.Pid)
	if err != nil {
		return false
	}
	
	err = process.Signal(os.Signal(nil))
	return err == nil
}

func (p *ClaudeProcess) GetWorkingDirectory() string {
	return p.tempDir
}

func (p *ClaudeProcess) SaveFile(filename string, content []byte) error {
	if p.tempDir == "" {
		return fmt.Errorf("no temporary directory available")
	}
	
	filePath := filepath.Join(p.tempDir, filename)
	return ioutil.WriteFile(filePath, content, 0644)
}

func (p *ClaudeProcess) ReadFile(filename string) ([]byte, error) {
	if p.tempDir == "" {
		return nil, fmt.Errorf("no temporary directory available")
	}
	
	filePath := filepath.Join(p.tempDir, filename)
	return ioutil.ReadFile(filePath)
}

func (p *ClaudeProcess) SendMessage(message string) (string, error) {
	// Add user message to history
	p.conversationHistory = append(p.conversationHistory, "User: "+message)
	
	// Build context from conversation history
	var contextBuilder strings.Builder
	if len(p.conversationHistory) > 1 {
		contextBuilder.WriteString("Previous conversation:\n")
		for _, msg := range p.conversationHistory[:len(p.conversationHistory)-1] {
			contextBuilder.WriteString(msg + "\n")
		}
		contextBuilder.WriteString("\nLatest message: " + message)
	} else {
		contextBuilder.WriteString(message)
	}
	
	fullPrompt := contextBuilder.String()
	
	// Use claude --print mode for this single request WITHOUT PTY
	// PTY seems to interfere with --print mode's stdin handling
	cmd := exec.Command(p.setup.GetClaudePath(), "--print", "--dangerously-skip-permissions")
	// Run from the base directory for workspace isolation
	cmd.Dir = p.setup.GetBaseDir()
	// Use system environment to access authentication, but add Claude-specific env vars
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, 
		fmt.Sprintf("BUN_INSTALL=%s", filepath.Join(p.setup.GetBaseDir(), ".bun")),
		fmt.Sprintf("PATH=%s:%s", filepath.Join(p.setup.GetBaseDir(), ".bun", "bin"), os.Getenv("PATH")),
	)
	// Add additional environment for relay
	cmd.Env = append(cmd.Env,
		"CLAUDE_RELAY=true",
		"TERM=dumb",
		"NO_COLOR=1",
	)
	
	// Use regular pipes for --print mode
	cmd.Stdin = strings.NewReader(fullPrompt)
	
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	
	// Run the command
	err := cmd.Run()
	if err != nil {
		stderrStr := stderr.String()
		
		if p.setup.IsAuthenticationNeeded(stderrStr) {
			return "", fmt.Errorf("authentication required: please restart the server to login")
		}
		return "", fmt.Errorf("claude command failed: %w, stderr: %s", err, stderrStr)
	}
	
	responseStr := stdout.String()
	
	// Add Claude's response to history
	p.conversationHistory = append(p.conversationHistory, "Claude: "+responseStr)
	
	// Keep history manageable (last 10 exchanges)
	if len(p.conversationHistory) > 20 {
		p.conversationHistory = p.conversationHistory[2:]
	}
	
	return responseStr, nil
}

func (p *ClaudeProcess) SendMessageWithProgress(message string, progressChan chan<- []byte, doneChan <-chan bool) (string, error) {
	// Save current state before processing (for undo functionality)
	p.SaveState()
	
	// Add user message to history
	p.conversationHistory = append(p.conversationHistory, "User: "+message)
	
	// Build context from conversation history
	var contextBuilder strings.Builder
	if len(p.conversationHistory) > 1 {
		contextBuilder.WriteString("Previous conversation:\n")
		for _, msg := range p.conversationHistory[:len(p.conversationHistory)-1] {
			contextBuilder.WriteString(msg + "\n")
		}
		contextBuilder.WriteString("\nLatest message: " + message)
	} else {
		contextBuilder.WriteString(message)
	}
	
	fullPrompt := contextBuilder.String()
	
	// Use claude --print mode for this single request
	cmd := exec.Command(p.setup.GetClaudePath(), "--print", "--dangerously-skip-permissions")
	// Run from the base directory for workspace isolation
	cmd.Dir = p.setup.GetBaseDir()
	// Use system environment to access authentication, but add Claude-specific env vars
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, 
		fmt.Sprintf("BUN_INSTALL=%s", filepath.Join(p.setup.GetBaseDir(), ".bun")),
		fmt.Sprintf("PATH=%s:%s", filepath.Join(p.setup.GetBaseDir(), ".bun", "bin"), os.Getenv("PATH")),
	)
	// Add additional environment for relay
	cmd.Env = append(cmd.Env,
		"CLAUDE_RELAY=true",
		"TERM=xterm",
		"NO_COLOR=1",
	)
	
	cmd.Stdin = strings.NewReader(fullPrompt)
	
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	// Start progress indicator in background
	progressTicker := make(chan struct{})
	go func() {
		messages := []string{
			"💭 Processing your request...",
			"🔍 Analyzing context...",
			"📖 Gathering information...",
			"🧠 Formulating response...",
		}
		messageIndex := 0
		
		ticker := time.NewTimer(2 * time.Second) // First update after 2 seconds
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				if messageIndex < len(messages) {
					select {
					case progressChan <- []byte(messages[messageIndex]):
					case <-doneChan:
						return
					}
					messageIndex++
					if messageIndex < len(messages) {
						ticker.Reset(3 * time.Second) // Subsequent updates every 3 seconds
					}
				}
			case <-progressTicker:
				return
			case <-doneChan:
				return
			}
		}
	}()
	
	// Run the command
	err := cmd.Run()
	close(progressTicker) // Stop progress updates
	
	if err != nil {
		// Check if authentication is needed
		stderrStr := stderr.String()
		
		if p.setup.IsAuthenticationNeeded(stderrStr) {
			return "", fmt.Errorf("authentication required: please restart the server to login")
		}
		return "", fmt.Errorf("claude command failed: %w, stderr: %s", err, stderrStr)
	}
	
	finalResponse := stdout.String()
	
	// Add Claude's response to history
	p.conversationHistory = append(p.conversationHistory, "Claude: "+finalResponse)
	
	// Keep history manageable (last 10 exchanges)
	if len(p.conversationHistory) > 20 {
		p.conversationHistory = p.conversationHistory[2:]
	}
	
	return finalResponse, nil
}

func (p *ClaudeProcess) SaveState() {
	// Make a copy of current conversation history
	historyCopy := make([]string, len(p.conversationHistory))
	copy(historyCopy, p.conversationHistory)
	
	state := ConversationState{
		History:   historyCopy,
		Timestamp: time.Now(),
	}
	
	p.conversationStates = append(p.conversationStates, state)
	
	// Keep only last 10 states to manage memory
	if len(p.conversationStates) > 10 {
		p.conversationStates = p.conversationStates[1:]
	}
}

func (p *ClaudeProcess) UndoLastExchange() error {
	if len(p.conversationStates) == 0 {
		return fmt.Errorf("no conversation states to undo")
	}
	
	// Get the last saved state
	lastState := p.conversationStates[len(p.conversationStates)-1]
	
	// Restore conversation history to that state
	p.conversationHistory = make([]string, len(lastState.History))
	copy(p.conversationHistory, lastState.History)
	
	// Remove the used state
	p.conversationStates = p.conversationStates[:len(p.conversationStates)-1]
	
	return nil
}

func (p *ClaudeProcess) UndoToIndex(messageIndex int) error {
	// Calculate which conversation history index this corresponds to
	// Each exchange has 2 entries (User: and Claude:)
	historyIndex := messageIndex * 2
	
	if historyIndex < 0 || historyIndex > len(p.conversationHistory) {
		return fmt.Errorf("invalid undo index: %d", messageIndex)
	}
	
	// Save the conversation that will be undone (for restore)
	if historyIndex < len(p.conversationHistory) {
		p.lastUndoneHistory = make([]string, len(p.conversationHistory))
		copy(p.lastUndoneHistory, p.conversationHistory)
	}
	
	// Truncate conversation history to this point
	p.conversationHistory = p.conversationHistory[:historyIndex]
	
	// Also truncate conversation states if needed
	// Find the appropriate state to restore
	for i := len(p.conversationStates) - 1; i >= 0; i-- {
		if len(p.conversationStates[i].History) <= historyIndex {
			p.conversationStates = p.conversationStates[:i+1]
			break
		}
	}
	
	return nil
}

func (p *ClaudeProcess) CanUndo() bool {
	return len(p.conversationStates) > 0
}

func (p *ClaudeProcess) GetLastExchange() (string, string, error) {
	if len(p.conversationHistory) < 2 {
		return "", "", fmt.Errorf("no complete exchange to return")
	}
	
	// Get last user message and Claude response
	userMsg := p.conversationHistory[len(p.conversationHistory)-2]
	claudeMsg := p.conversationHistory[len(p.conversationHistory)-1]
	
	// Strip prefixes
	userMsg = strings.TrimPrefix(userMsg, "User: ")
	claudeMsg = strings.TrimPrefix(claudeMsg, "Claude: ")
	
	return userMsg, claudeMsg, nil
}

func (p *ClaudeProcess) CanRestore() bool {
	return p.lastUndoneHistory != nil && len(p.lastUndoneHistory) > len(p.conversationHistory)
}

func (p *ClaudeProcess) RestoreLastUndo() ([]string, error) {
	if !p.CanRestore() {
		return nil, fmt.Errorf("nothing to restore")
	}
	
	// Get the messages that will be restored (for client display)
	restoredMessages := p.lastUndoneHistory[len(p.conversationHistory):]
	
	// Restore the conversation history
	p.conversationHistory = make([]string, len(p.lastUndoneHistory))
	copy(p.conversationHistory, p.lastUndoneHistory)
	
	// Clear the undo buffer since we've restored it
	p.lastUndoneHistory = nil
	
	// Rebuild conversation states
	p.SaveState()
	
	return restoredMessages, nil
}

func (p *ClaudeProcess) GetRestoredMessagesForClient() []map[string]string {
	if !p.CanRestore() {
		return nil
	}
	
	// Get messages that would be restored
	restoredPart := p.lastUndoneHistory[len(p.conversationHistory):]
	
	var messages []map[string]string
	for i := 0; i < len(restoredPart); i += 2 {
		if i+1 < len(restoredPart) {
			userMsg := strings.TrimPrefix(restoredPart[i], "User: ")
			claudeMsg := strings.TrimPrefix(restoredPart[i+1], "Claude: ")
			messages = append(messages, map[string]string{
				"user": userMsg,
				"claude": claudeMsg,
			})
		}
	}
	
	return messages
}