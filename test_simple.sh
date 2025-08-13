#!/bin/bash

# Simple test using curl to test WebSocket
echo "Testing Claude Relay WebSocket Server..."

# Use Node.js to test since it's available
node -e "
const WebSocket = require('ws');
const ws = new WebSocket('ws://localhost:8080/ws');

ws.on('open', function open() {
    console.log('✅ Connected to Claude relay server');
    
    // Send test message
    const testMessage = 'What is 2 + 2?';
    console.log('📤 Sending:', testMessage);
    ws.send(testMessage);
    
    // Set timeout to close after receiving response
    setTimeout(() => {
        console.log('⏰ Closing connection after 10 seconds');
        ws.close();
        process.exit(0);
    }, 10000);
});

ws.on('message', function incoming(data) {
    console.log('📥 Claude says:', data.toString());
});

ws.on('error', function error(err) {
    console.error('❌ Error:', err.message);
    process.exit(1);
});

ws.on('close', function close() {
    console.log('👋 Connection closed');
});
"