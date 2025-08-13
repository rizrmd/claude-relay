#!/usr/bin/env python3

import asyncio
import websockets
import sys

async def test_claude():
    uri = "ws://localhost:8080/ws"
    
    try:
        async with websockets.connect(uri) as websocket:
            print("Connected to Claude relay server")
            
            # Send test message
            test_message = "Hello Claude, what is 2 + 2?"
            print(f"Sending: {test_message}")
            await websocket.send(test_message)
            
            # Read responses for 5 seconds
            try:
                while True:
                    response = await asyncio.wait_for(websocket.recv(), timeout=5.0)
                    print(f"Claude: {response}")
            except asyncio.TimeoutError:
                print("No more responses after 5 seconds")
            
    except Exception as e:
        print(f"Error: {e}")

if __name__ == "__main__":
    asyncio.run(test_claude())