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
)

type ClaudeProcessV2 struct {
	tempDir string
}

func NewClaudeProcessV2() (*ClaudeProcessV2, error) {
	tempDir, err := ioutil.TempDir("", "claude-relay-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	log.Printf("Created temporary directory: %s", tempDir)

	return &ClaudeProcessV2{
		tempDir: tempDir,
	}, nil
}

func (p *ClaudeProcessV2) SendMessage(message string) (string, error) {
	// Create a new claude process for each message
	cmd := exec.Command("/Users/riz/.bun/bin/claude")
	cmd.Dir = p.tempDir
	cmd.Env = append(os.Environ(),
		"HOME="+p.tempDir,
	)

	// Send message as stdin
	cmd.Stdin = strings.NewReader(message)

	// Capture output
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	log.Printf("Running claude with message: %s", message)

	// Run the command
	err := cmd.Run()
	if err != nil {
		log.Printf("Claude process error: %v, stderr: %s", err, stderr.String())
		if stderr.Len() > 0 {
			return "", fmt.Errorf("claude error: %s", stderr.String())
		}
		return "", fmt.Errorf("failed to run claude: %w", err)
	}

	response := stdout.String()
	if response == "" && stderr.Len() > 0 {
		response = fmt.Sprintf("[Error] %s", stderr.String())
	}

	log.Printf("Claude response: %s", response)
	return response, nil
}

func (p *ClaudeProcessV2) Cleanup() error {
	if p.tempDir != "" {
		log.Printf("Removing temporary directory: %s", p.tempDir)
		if err := os.RemoveAll(p.tempDir); err != nil {
			return fmt.Errorf("failed to remove temp directory: %w", err)
		}
	}
	return nil
}

func (p *ClaudeProcessV2) GetWorkingDirectory() string {
	return p.tempDir
}

func (p *ClaudeProcessV2) SaveFile(filename string, content []byte) error {
	if p.tempDir == "" {
		return fmt.Errorf("no temporary directory available")
	}

	filePath := filepath.Join(p.tempDir, filename)
	return ioutil.WriteFile(filePath, content, 0644)
}

func (p *ClaudeProcessV2) ReadFile(filename string) ([]byte, error) {
	if p.tempDir == "" {
		return nil, fmt.Errorf("no temporary directory available")
	}

	filePath := filepath.Join(p.tempDir, filename)
	return ioutil.ReadFile(filePath)
}