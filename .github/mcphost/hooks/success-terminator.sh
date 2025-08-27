#!/bin/bash

# Monitor log file for completion
while true; do
    if [ -f "mcp.log" ] && grep -q "Enhanced deployment verification completed" mcp.log; then
        echo "SUCCESS: Enhanced deployment verification completed"
        pkill -f mcphost || true
        exit 0
    fi
    sleep 2
done