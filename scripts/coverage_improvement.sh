#!/bin/bash
set -e

echo "=== COVERAGE IMPROVEMENT TRACKER ==="

# File to store baseline coverage
baseline_file="coverage_baseline.json"
improvement_target=5 # Target 5% improvement per package

# Generate current coverage
echo "Generating current coverage report..."
go test -coverprofile=coverage.out -covermode=atomic ./pkg/mcp/... 2>/dev/null || true

# Extract package-level coverage
echo "{" > current_coverage.json
first=true

go list ./pkg/mcp/... | while read package; do
    # Get coverage for this package
    coverage=$(go test -cover "$package" 2>/dev/null | grep -oE '[0-9]+\.[0-9]+%' | tr -d '%' || echo "0")

    if [ "$first" = true ]; then
        first=false
    else
        echo "," >> current_coverage.json
    fi

    echo "  \"$package\": $coverage" >> current_coverage.json
done

echo "}" >> current_coverage.json

# If baseline exists, compare
if [ -f "$baseline_file" ]; then
    echo "Comparing with baseline..."

    # Read baseline and current coverage
    packages_improved=0
    packages_total=0

    go list ./pkg/mcp/... | while read package; do
        packages_total=$((packages_total + 1))

        # Get baseline coverage
        baseline=$(grep "\"$package\":" "$baseline_file" 2>/dev/null | sed 's/.*: \([0-9.]*\).*/\1/' || echo "0")

        # Get current coverage
        current=$(grep "\"$package\":" current_coverage.json | sed 's/.*: \([0-9.]*\).*/\1/' || echo "0")

        # Calculate improvement
        improvement=$(echo "$current - $baseline" | bc -l)

        if (( $(echo "$improvement >= $improvement_target" | bc -l) )); then
            echo "✅ $package: +${improvement}% improvement (baseline: ${baseline}%, current: ${current}%)"
            packages_improved=$((packages_improved + 1))
        elif (( $(echo "$improvement > 0" | bc -l) )); then
            echo "🔄 $package: +${improvement}% improvement (target: +${improvement_target}%)"
        else
            echo "❌ $package: ${improvement}% change (needs +${improvement_target}% improvement)"
        fi
    done

    echo ""
    echo "Summary: $packages_improved/$packages_total packages achieved +${improvement_target}% improvement"

    # Update baseline if requested
    if [ "$1" = "--update-baseline" ]; then
        echo "Updating baseline..."
        cp current_coverage.json "$baseline_file"
        echo "✅ Baseline updated"
    fi
else
    echo "No baseline found. Creating initial baseline..."
    cp current_coverage.json "$baseline_file"
    echo "✅ Initial baseline created"
    echo "Run this script again to track improvements"
fi

# Cleanup
rm -f current_coverage.json coverage.out
