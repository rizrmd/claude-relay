.PHONY: dev build run clean test install

# Development - run with auto-reload
dev:
	@echo "Starting development server with auto-reload..."
	@which air > /dev/null 2>&1 || (echo "Installing air for hot reload..." && go install github.com/air-verse/air@latest)
	@air

# Build the binary
build:
	@echo "Building claude-relay..."
	@go build -o claude-relay .

# Run the server
run: build
	@echo "Starting claude-relay server..."
	@./claude-relay

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -f claude-relay
	@rm -rf tmp/

# Test the server
test:
	@echo "Running tests..."
	@go test ./...

# Install dependencies
install:
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Run with specific port
run-port: build
	@echo "Starting on port $(PORT)..."
	@./claude-relay -port $(PORT)

# Development without air (simple rebuild and run)
dev-simple:
	@echo "Running in simple dev mode (Ctrl+C to stop, then run 'make dev-simple' again to restart)..."
	@go run . -port 8080

# Help
help:
	@echo "Available commands:"
	@echo "  make dev        - Run with auto-reload (requires air)"
	@echo "  make dev-simple - Run without auto-reload"
	@echo "  make build      - Build the binary"
	@echo "  make run        - Build and run"
	@echo "  make clean      - Clean build artifacts"
	@echo "  make test       - Run tests"
	@echo "  make install    - Install dependencies"
	@echo "  make fmt        - Format code"
	@echo "  make run-port PORT=3000 - Run on specific port"