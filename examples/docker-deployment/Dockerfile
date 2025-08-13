# Build stage
FROM golang:1.21-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o claude-relay .

# Runtime stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy the binary
COPY --from=builder /build/claude-relay /app/

# Create directory for Claude installation
RUN mkdir -p /app/claude

# Copy pre-authenticated config (you need to provide this)
# Option 1: Copy from build context
# COPY ./claude-auth /auth/claude

# Option 2: Mount as volume at runtime
# docker run -v /path/to/auth:/auth/claude ...

# Expose ports
EXPOSE 8081 8080

# Set environment
ENV CLAUDE_AUTH_DIR=/auth/claude

# Run the relay
CMD ["/app/claude-relay"]