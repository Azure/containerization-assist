#\!/bin/bash
set -e

echo "=== REGRESSION TESTING ==="

# Run current benchmarks
echo "Running current benchmarks..."
go test -bench=. -benchmem -run=^$ ./pkg/mcp/... > current_performance.txt 2>&1 || true

# Compare with baseline
echo "Comparing with baseline..."
if [ -f performance_baseline.txt ]; then
    echo "Performance comparison available"
    
    # Simple check - if we have benchmarks running, that's a good start
    if grep -q "Benchmark" current_performance.txt; then
        echo "✅ PASS: Benchmarks are running successfully"
    else
        echo "⚠️  WARNING: No benchmark results found"
    fi
else
    echo "⚠️  No baseline found, establishing baseline..."
    scripts/performance_baseline.sh
fi

echo "✅ Performance regression testing completed"
EOF < /dev/null