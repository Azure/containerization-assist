#!/bin/bash
echo "=== $(date) - Test Monitoring ==="
echo "Running MCP tests..."
go test -short -tags mcp ./pkg/mcp/...
echo "Running performance tests..."  
go test -bench=. -run='^$' ./pkg/mcp/... | grep -E "(BenchmarkTool|ns/op)"
echo "Checking for race conditions..."
go test -race -short ./pkg/mcp/...