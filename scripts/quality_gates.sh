#!/bin/bash
set -e

echo "=== CONTAINER KIT MCP QUALITY GATES ==="

# Gate 1: Interface Count
echo "🔍 Checking interface count..."
interface_count=$(find pkg/mcp -name "*.go" -exec grep -H "type.*interface" {} \; | wc -l)
if [ "$interface_count" -gt 50 ]; then
    echo "❌ FAIL: Interface count $interface_count exceeds limit of 50"
    exit 1
fi
echo "✅ PASS: Interface count $interface_count within limit"

# Gate 2: File Size
echo "🔍 Checking file sizes..."
violations=0
find pkg/mcp -name "*.go" | while read file; do
    lines=$(wc -l < "$file")
    if [ "$lines" -gt 800 ]; then
        echo "❌ $file: $lines lines (exceeds 800)"
        violations=$((violations + 1))
    fi
done

if [ "$violations" -gt 0 ]; then
    echo "❌ FAIL: Files exceed 800 line limit"
    exit 1
fi
echo "✅ PASS: All files within 800 line limit"

# Gate 3: Import Depth
echo "🔍 Checking import depth..."
deep_imports=$(grep -r "pkg/mcp" pkg/mcp/ | grep -E "pkg/mcp/[^/]+/[^/]+/[^/]+/" | wc -l)
if [ "$deep_imports" -gt 0 ]; then
    echo "❌ FAIL: $deep_imports deep imports found (>3 levels)"
    exit 1
fi
echo "✅ PASS: All imports ≤3 levels deep"

# Gate 4: Build Success
echo "🔍 Checking build..."
if ! make build >/dev/null 2>&1; then
    echo "❌ FAIL: Build failed"
    exit 1
fi
echo "✅ PASS: Build successful"

# Gate 5: Test Coverage
echo "🔍 Checking test coverage..."
if ! go test -coverprofile=coverage.out ./pkg/mcp/... >/dev/null 2>&1; then
    echo "❌ FAIL: Tests failed"
    exit 1
fi

# Extract overall coverage
overall_coverage=$(go tool cover -func=coverage.out | grep "total:" | awk '{print $3}' | tr -d '%')
min_coverage=30  # Conservative baseline

if [ -n "$overall_coverage" ] && (( $(echo "$overall_coverage < $min_coverage" | bc -l) )); then
    echo "❌ FAIL: Overall coverage $overall_coverage% below minimum $min_coverage%"
    exit 1
fi
echo "✅ PASS: Test coverage $overall_coverage% meets minimum requirements"

# Gate 6: Linting
echo "🔍 Checking linting..."
if ! make lint >/dev/null 2>&1; then
    echo "❌ FAIL: Linting issues found"
    exit 1
fi
echo "✅ PASS: Linting clean"

echo "🎉 ALL QUALITY GATES PASSED"