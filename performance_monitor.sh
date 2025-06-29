#!/bin/bash
echo "=== Performance Monitoring ==="
echo "Running benchmarks..."
go test -bench=. -run='^$' ./pkg/mcp/... 2>/dev/null | grep -E "BenchmarkTool.*ns/op" | while read line; do
    benchmark=$(echo $line | awk '{print $1}')
    timing=$(echo $line | awk '{print $3}')
    echo "📊 $benchmark: $timing"
done
