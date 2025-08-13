#!/bin/bash

# Simple test script using websocat or wscat
echo "Testing WebSocket connection to claude-relay..."

# Send a test message using curl
(echo "Hello Claude"; sleep 2) | websocat ws://localhost:8080/ws 2>/dev/null || {
    echo "websocat not found, trying with curl..."
    
    # Alternative: use Node.js if available
    node -e "
    const WebSocket = require('ws');
    const ws = new WebSocket('ws://localhost:8080/ws');
    
    ws.on('open', function open() {
        console.log('Connected to server');
        ws.send('Hello Claude, this is a test message');
        
        setTimeout(() => {
            ws.close();
            process.exit(0);
        }, 5000);
    });
    
    ws.on('message', function incoming(data) {
        console.log('Received:', data.toString());
    });
    
    ws.on('error', function error(err) {
        console.error('Error:', err);
    });
    " 2>/dev/null || echo "Please install websocat or ensure Node.js is available"
}