#!/bin/bash

echo "=========================================="
echo "Claude Relay Setup Script"
echo "=========================================="
echo ""
echo "This script will:"
echo "1. Install portable Bun in the current directory"
echo "2. Install Claude Code CLI locally"
echo "3. Guide you through Claude authentication"
echo ""
echo "When Claude starts, type '/login' to authenticate"
echo "=========================================="
echo ""
echo "Starting setup..."

# Run the relay which will trigger setup
./claude-relay -port 8081