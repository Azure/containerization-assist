#!/bin/bash

# Test coverage tracking script
COVERAGE_DIR="test/coverage"
REPORTS_DIR="test/reports"

mkdir -p "$COVERAGE_DIR" "$REPORTS_DIR"

echo "=== Container Kit Test Coverage Analysis ==="
echo "Date: $(date)"
echo ""

# Run comprehensive coverage analysis
echo "Running coverage analysis..."
go test -cover -coverprofile="$COVERAGE_DIR/coverage.out" ./pkg/mcp/... > "$COVERAGE_DIR/coverage_raw.txt" 2>&1

# Generate HTML coverage report
if [ -f "$COVERAGE_DIR/coverage.out" ]; then
    go tool cover -html="$COVERAGE_DIR/coverage.out" -o "$REPORTS_DIR/coverage.html"
    echo "✅ HTML coverage report generated: $REPORTS_DIR/coverage.html"
else
    echo "❌ Coverage profile not generated"
fi

# Generate detailed coverage summary
echo ""
echo "=== COVERAGE SUMMARY ===" > "$REPORTS_DIR/coverage_summary.txt"
echo "Generated: $(date)" >> "$REPORTS_DIR/coverage_summary.txt"
echo "" >> "$REPORTS_DIR/coverage_summary.txt"

# Extract overall coverage
if [ -f "$COVERAGE_DIR/coverage.out" ]; then
    OVERALL_COVERAGE=$(go tool cover -func="$COVERAGE_DIR/coverage.out" | grep total | awk '{print $3}')
    echo "Overall Coverage: $OVERALL_COVERAGE" >> "$REPORTS_DIR/coverage_summary.txt"
    echo "Overall Coverage: $OVERALL_COVERAGE"
else
    OVERALL_COVERAGE="0%"
    echo "Overall Coverage: $OVERALL_COVERAGE (no profile generated)" >> "$REPORTS_DIR/coverage_summary.txt"
    echo "Overall Coverage: $OVERALL_COVERAGE"
fi

# Analyze per-package coverage
echo "" >> "$REPORTS_DIR/coverage_summary.txt"
echo "Per-Package Coverage:" >> "$REPORTS_DIR/coverage_summary.txt"
echo "Package | Coverage | Status" >> "$REPORTS_DIR/coverage_summary.txt"
echo "--------|----------|--------" >> "$REPORTS_DIR/coverage_summary.txt"

echo ""
echo "Per-Package Coverage Analysis:"
echo "Package | Coverage | Status"
echo "--------|----------|--------"

# Parse coverage from test output
while IFS= read -r line; do
    if [[ $line =~ coverage:\ ([0-9.]+)%\ of\ statements ]]; then
        COVERAGE_PERCENT=${BASH_REMATCH[1]}
        PACKAGE=$(echo "$line" | awk '{print $1}')

        # Determine status based on coverage
        if (( $(echo "$COVERAGE_PERCENT >= 80" | bc -l 2>/dev/null || echo "0") )); then
            STATUS="✅ Excellent"
        elif (( $(echo "$COVERAGE_PERCENT >= 55" | bc -l 2>/dev/null || echo "0") )); then
            STATUS="✅ Good"
        elif (( $(echo "$COVERAGE_PERCENT >= 30" | bc -l 2>/dev/null || echo "0") )); then
            STATUS="⚠️  Needs work"
        elif (( $(echo "$COVERAGE_PERCENT > 0" | bc -l 2>/dev/null || echo "0") )); then
            STATUS="❌ Poor"
        else
            STATUS="❌ No coverage"
        fi

        # Short package name for display
        SHORT_PACKAGE=$(echo "$PACKAGE" | sed 's/.*\/pkg\/mcp\///')

        echo "$SHORT_PACKAGE | $COVERAGE_PERCENT% | $STATUS"
        echo "$SHORT_PACKAGE | $COVERAGE_PERCENT% | $STATUS" >> "$REPORTS_DIR/coverage_summary.txt"
    fi
done < "$COVERAGE_DIR/coverage_raw.txt"

# Extract packages with no test files
echo "" >> "$REPORTS_DIR/coverage_summary.txt"
echo "Packages without tests:" >> "$REPORTS_DIR/coverage_summary.txt"

echo ""
echo "Packages without tests:"
grep "no test files" "$COVERAGE_DIR/coverage_raw.txt" | while IFS= read -r line; do
    PACKAGE=$(echo "$line" | awk '{print $2}')
    SHORT_PACKAGE=$(echo "$PACKAGE" | sed 's/.*\/pkg\/mcp\///')
    echo "- $SHORT_PACKAGE"
    echo "- $SHORT_PACKAGE" >> "$REPORTS_DIR/coverage_summary.txt"
done

# Extract packages with build failures
BUILD_FAILURES=$(grep -c "build failed" "$COVERAGE_DIR/coverage_raw.txt")
echo "" >> "$REPORTS_DIR/coverage_summary.txt"
echo "Build failures: $BUILD_FAILURES packages" >> "$REPORTS_DIR/coverage_summary.txt"

echo ""
echo "Build failures: $BUILD_FAILURES packages"

# Generate targets and recommendations
echo "" >> "$REPORTS_DIR/coverage_summary.txt"
echo "=== TARGETS AND RECOMMENDATIONS ===" >> "$REPORTS_DIR/coverage_summary.txt"

# Check against baseline (current ~15%)
BASELINE_COVERAGE="15.0"
CURRENT_COVERAGE_NUM=$(echo "$OVERALL_COVERAGE" | sed 's/%//')

echo "Current vs Baseline:" >> "$REPORTS_DIR/coverage_summary.txt"
echo "- Current: $OVERALL_COVERAGE" >> "$REPORTS_DIR/coverage_summary.txt"
echo "- Baseline: ${BASELINE_COVERAGE}%" >> "$REPORTS_DIR/coverage_summary.txt"
echo "- Target: 55%" >> "$REPORTS_DIR/coverage_summary.txt"

echo ""
echo "Targets:"
echo "- Current: $OVERALL_COVERAGE"
echo "- Baseline: ${BASELINE_COVERAGE}%"
echo "- Target: 55%"

# Recommendations
echo "" >> "$REPORTS_DIR/coverage_summary.txt"
echo "Recommendations:" >> "$REPORTS_DIR/coverage_summary.txt"

if (( $(echo "$CURRENT_COVERAGE_NUM < 55" | bc -l 2>/dev/null || echo "1") )); then
    echo "1. Add unit tests for packages with 0% coverage" >> "$REPORTS_DIR/coverage_summary.txt"
    echo "2. Focus on domain layer tests (business logic)" >> "$REPORTS_DIR/coverage_summary.txt"
    echo "3. Add integration tests for application layer" >> "$REPORTS_DIR/coverage_summary.txt"
    echo "4. Fix build failures to enable testing" >> "$REPORTS_DIR/coverage_summary.txt"
fi

# Create coverage badge
COVERAGE_COLOR="red"
if (( $(echo "$CURRENT_COVERAGE_NUM >= 80" | bc -l 2>/dev/null || echo "0") )); then
    COVERAGE_COLOR="brightgreen"
elif (( $(echo "$CURRENT_COVERAGE_NUM >= 55" | bc -l 2>/dev/null || echo "0") )); then
    COVERAGE_COLOR="green"
elif (( $(echo "$CURRENT_COVERAGE_NUM >= 30" | bc -l 2>/dev/null || echo "0") )); then
    COVERAGE_COLOR="yellow"
elif (( $(echo "$CURRENT_COVERAGE_NUM >= 15" | bc -l 2>/dev/null || echo "0") )); then
    COVERAGE_COLOR="orange"
fi

echo "![Coverage](https://img.shields.io/badge/coverage-$OVERALL_COVERAGE-$COVERAGE_COLOR)" > "$REPORTS_DIR/coverage_badge.md"

# Generate test recommendations by package
echo "" >> "$REPORTS_DIR/coverage_summary.txt"
echo "=== PRIORITY TESTING RECOMMENDATIONS ===" >> "$REPORTS_DIR/coverage_summary.txt"

echo "High Priority (Core Business Logic):" >> "$REPORTS_DIR/coverage_summary.txt"
echo "- domain/containerization/* - Add comprehensive unit tests" >> "$REPORTS_DIR/coverage_summary.txt"
echo "- domain/errors - Test error handling scenarios" >> "$REPORTS_DIR/coverage_summary.txt"
echo "- domain/security - Test validation and policies" >> "$REPORTS_DIR/coverage_summary.txt"

echo "Medium Priority (Application Logic):" >> "$REPORTS_DIR/coverage_summary.txt"
echo "- application/core - Test server lifecycle" >> "$REPORTS_DIR/coverage_summary.txt"
echo "- application/commands - Test command implementations" >> "$REPORTS_DIR/coverage_summary.txt"
echo "- application/orchestration - Test pipeline logic" >> "$REPORTS_DIR/coverage_summary.txt"

echo "Lower Priority (Infrastructure):" >> "$REPORTS_DIR/coverage_summary.txt"
echo "- infra/persistence - Test storage operations" >> "$REPORTS_DIR/coverage_summary.txt"
echo "- infra/transport - Test protocol handling" >> "$REPORTS_DIR/coverage_summary.txt"

echo ""
echo "✅ Coverage analysis complete"
echo "📁 Reports saved to: $REPORTS_DIR/"
echo "📊 View HTML report: open $REPORTS_DIR/coverage.html"
echo "📋 Summary: $REPORTS_DIR/coverage_summary.txt"
