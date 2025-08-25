#!/bin/bash
set -e

# Load Azure keys
source ../azure_keys.sh

# Create directories
mkdir -p /tmp/mcp-workspace
rm -f /tmp/mcp-sessions.db

echo "Starting autonomous containerization workflow..."

# Run mcphost with specific instructions for autonomous operation
exec mcphost --config ./mcp-test-workspace/mcphost.yml --stream=false -p "
Containerize the repository: https://github.com/konveyor-ecosystem/coolstore

INSTRUCTIONS FOR AUTONOMOUS OPERATION:
1. You are running in autonomous mode - NEVER ask for user input
2. If any step fails, automatically troubleshoot and retry
3. For deployment failures with pods in 'ContainerCreating' status:
   - Wait 60 seconds for pods to start
   - Check pod events and logs automatically  
   - Adjust resource limits if needed
   - Retry deployment with fixes
4. If registry push fails, retry with exponential backoff
5. Continue until successful completion or you've exhausted all retry attempts
6. Provide detailed progress updates at each step
7. If you need to make decisions, choose the most reasonable default option

Start the containerization workflow now.
"
