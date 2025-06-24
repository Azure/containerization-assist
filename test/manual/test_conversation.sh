#!/bin/bash

# Test the conversation mode by starting the MCP server and sending chat messages

echo "Building MCP server..."
go build -tags mcp -o container-kit-mcp ./cmd/mcp-server || exit 1

echo "Starting MCP server..."
./container-kit-mcp --transport stdio --enable-conversation &
SERVER_PID=$!

# Give it time to start
sleep 2

echo "Testing conversation flow..."

# Test 1: Initial hello
echo '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"chat","arguments":{"message":"Hello, I want to containerize my Go application"}},"id":1}' | ./container-kit-mcp --transport stdio --enable-conversation

echo
echo "Server output should show pre-flight checks running..."
echo

# Clean up
kill $SERVER_PID 2>/dev/null
wait $SERVER_PID 2>/dev/null

echo "Test completed"