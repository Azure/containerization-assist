#!/bin/bash
echo "=== Adapter Elimination Testing ==="

# Test AI analyzer direct usage
echo "Testing AI analyzer without adapters..."
go test -run TestAIAnalyzer ./pkg/mcp/internal/analyze/

# Test session management without wrapper
echo "Testing session management without wrapper..."
go test -run TestSession ./pkg/mcp/internal/core/

# Test tool registration without adapters
echo "Testing tool registration..."
go test -run TestToolRegistration ./pkg/mcp/internal/core/