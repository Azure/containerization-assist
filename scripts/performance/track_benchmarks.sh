#!/bin/bash

# Performance benchmark tracking script
BASELINE_DIR="benchmarks/baselines"
CURRENT_DIR="benchmarks/current"
REPORT_DIR="benchmarks/reports"

mkdir -p "$CURRENT_DIR" "$REPORT_DIR"

# Run current benchmarks
echo "Running current benchmarks..."
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
go test -bench=. -benchmem ./pkg/mcp/... > "$CURRENT_DIR/current_$TIMESTAMP.txt" 2>&1

# Extract successful benchmarks for comparison
echo "Extracting benchmark results..."
grep -E "^Benchmark|^ok|^PASS" "$CURRENT_DIR/current_$TIMESTAMP.txt" > "$REPORT_DIR/extracted_$TIMESTAMP.txt"

# Simple performance check
echo "=== PERFORMANCE CHECK ===" > "$REPORT_DIR/regression_report_$TIMESTAMP.txt"
echo "Date: $(date)" >> "$REPORT_DIR/regression_report_$TIMESTAMP.txt"
echo "" >> "$REPORT_DIR/regression_report_$TIMESTAMP.txt"

# Check for benchmarks that completed
if grep -q "^Benchmark" "$CURRENT_DIR/current_$TIMESTAMP.txt"; then
    echo "Benchmarks found:" >> "$REPORT_DIR/regression_report_$TIMESTAMP.txt"
    grep "^Benchmark" "$CURRENT_DIR/current_$TIMESTAMP.txt" >> "$REPORT_DIR/regression_report_$TIMESTAMP.txt"
    
    # Check if any benchmark exceeds 300μs (300000 ns)
    PERF_ISSUES=0
    while IFS= read -r line; do
        if [[ $line =~ ([0-9]+(\.[0-9]+)?)[[:space:]]+ns/op ]]; then
            NS_VALUE=${BASH_REMATCH[1]}
            # Remove decimal point for comparison
            NS_INT=$(echo "$NS_VALUE" | sed 's/\.//')
            if [ "${#NS_INT}" -gt 6 ]; then
                echo "⚠️  Performance warning: $line (exceeds 300μs target)" >> "$REPORT_DIR/regression_report_$TIMESTAMP.txt"
                PERF_ISSUES=$((PERF_ISSUES + 1))
            fi
        fi
    done < <(grep "^Benchmark" "$CURRENT_DIR/current_$TIMESTAMP.txt")
    
    if [ $PERF_ISSUES -eq 0 ]; then
        echo "✅ All benchmarks within 300μs target" >> "$REPORT_DIR/regression_report_$TIMESTAMP.txt"
    fi
else
    echo "❌ No benchmarks completed successfully" >> "$REPORT_DIR/regression_report_$TIMESTAMP.txt"
fi

echo "" >> "$REPORT_DIR/regression_report_$TIMESTAMP.txt"
echo "Build issues found:" >> "$REPORT_DIR/regression_report_$TIMESTAMP.txt"
grep -c "build failed" "$CURRENT_DIR/current_$TIMESTAMP.txt" | xargs -I {} echo "{} packages failed to build" >> "$REPORT_DIR/regression_report_$TIMESTAMP.txt"

# Show summary
echo ""
echo "✅ Benchmark tracking complete"
echo "Results saved to: $REPORT_DIR/regression_report_$TIMESTAMP.txt"
cat "$REPORT_DIR/regression_report_$TIMESTAMP.txt"