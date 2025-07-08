#\!/bin/bash
set -e

echo "=== PERFORMANCE BASELINE ESTABLISHMENT ==="

# Run benchmarks and save baseline
echo "Running performance benchmarks..."
go test -bench=. -benchmem -run=^$ ./pkg/mcp/... > performance_baseline.txt 2>&1 || true

echo "Performance baseline saved to performance_baseline.txt"

# Extract key metrics
echo "=== KEY PERFORMANCE METRICS ==="
if [ -f performance_baseline.txt ]; then
    grep "Benchmark" performance_baseline.txt | while read line; do
        benchmark=$(echo "$line" | awk '{print $1}')
        ops_per_sec=$(echo "$line" | awk '{print $3}')
        ns_per_op=$(echo "$line" | awk '{print $2}')
        
        echo "$benchmark: $ops_per_sec ops/sec ($ns_per_op ns/op)"
    done
else
    echo "No benchmark results found"
fi

# Set performance thresholds (adjust based on baseline)
cat > performance_thresholds.txt << 'THRESHOLDS'
# Performance thresholds (maximum acceptable values)
# Format: benchmark_name:max_ns_per_op
BenchmarkRegistryOperations:1000
BenchmarkSessionOperations:10000
BenchmarkWorkflowExecution:50000
BenchmarkErrorHandling:500
THRESHOLDS

echo "Performance thresholds set in performance_thresholds.txt"
EOF < /dev/null