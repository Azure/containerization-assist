#!/bin/bash
echo "=== Integration Testing ==="

# Test tool execution with unified interfaces, no adapters, no legacy
echo "Testing complete tool execution pipeline..."
go test -run TestToolExecution -v ./pkg/mcp/internal/core/

# Test orchestration with all changes
echo "Testing orchestration integration..."
go test -run TestOrchestration -v ./pkg/mcp/internal/orchestration/

# Test build tools with all architecture changes
echo "Testing build tool integration..."
go test -run TestBuildTools -v ./pkg/mcp/internal/build/

# Performance validation
echo "Performance validation..."
go test -bench=. -run='^$' ./pkg/mcp/...
