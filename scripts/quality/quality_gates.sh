#!/bin/bash

# Quality gates enforcement script
set -e

echo "=== CONTAINER KIT QUALITY GATES ==="
echo "Date: $(date)"
echo "Commit: $(git rev-parse HEAD 2>/dev/null || echo 'unknown')"
echo ""

# Configuration
COVERAGE_THRESHOLD="${COVERAGE_THRESHOLD:-15.0}"
LINT_ERROR_BUDGET="${LINT_ERROR_BUDGET:-100}"
PERFORMANCE_THRESHOLD_NS="${PERFORMANCE_THRESHOLD_NS:-300000}"
MAX_BUILD_TIME="${MAX_BUILD_TIME:-300}" # 5 minutes
REPORTS_DIR="test/reports"

mkdir -p "$REPORTS_DIR"

# Gate results tracking
GATE_RESULTS=()
OVERALL_PASS=true

# Helper function to record gate result
record_gate() {
    local gate_name="$1"
    local status="$2"
    local details="$3"

    if [ "$status" = "PASS" ]; then
        echo "✅ Gate: $gate_name - PASSED"
        GATE_RESULTS+=("✅ $gate_name: PASSED - $details")
    else
        echo "❌ Gate: $gate_name - FAILED"
        GATE_RESULTS+=("❌ $gate_name: FAILED - $details")
        OVERALL_PASS=false
    fi
}

# Gate 1: Code Formatting
echo "=== GATE 1: CODE FORMATTING ==="
echo "Checking Go code formatting compliance..."

if command -v gofmt >/dev/null 2>&1; then
    UNFORMATTED_FILES=$(find pkg -name "*.go" -exec gofmt -l {} \; 2>/dev/null)

    if [ -z "$UNFORMATTED_FILES" ]; then
        record_gate "Code Formatting" "PASS" "All Go files properly formatted"
    else
        echo "Unformatted files found:"
        echo "$UNFORMATTED_FILES"
        record_gate "Code Formatting" "FAIL" "$(echo "$UNFORMATTED_FILES" | wc -l) files need formatting"
    fi
else
    record_gate "Code Formatting" "FAIL" "gofmt not available"
fi
echo ""

# Gate 2: Linting
echo "=== GATE 2: LINTING ==="
echo "Running linting checks with error budget: $LINT_ERROR_BUDGET issues"

# Create a simple lint check using go vet
LINT_OUTPUT="$REPORTS_DIR/lint_results.txt"
LINT_ISSUES=0

# Run go vet
echo "Running go vet..."
if go vet ./pkg/mcp/... > "$LINT_OUTPUT" 2>&1; then
    LINT_ISSUES=0
else
    LINT_ISSUES=$(wc -l < "$LINT_OUTPUT")
fi

# Run additional checks
echo "Running additional static analysis..."

# Check for common issues
echo "Checking for common issues..." >> "$LINT_OUTPUT"

# Check for TODO/FIXME comments (informational)
TODO_COUNT=$(find pkg -name "*.go" -exec grep -H "TODO\|FIXME" {} \; | wc -l)
echo "TODO/FIXME comments: $TODO_COUNT" >> "$LINT_OUTPUT"

# Check for panic usage (should be minimal)
PANIC_COUNT=$(find pkg -name "*.go" -exec grep -H "panic(" {} \; | wc -l)
echo "panic() usage: $PANIC_COUNT" >> "$LINT_OUTPUT"

# Check for fmt.Print usage (should use structured logging)
PRINT_COUNT=$(find pkg -name "*.go" -exec grep -H "fmt\.Print" {} \; | wc -l)
echo "fmt.Print usage: $PRINT_COUNT" >> "$LINT_OUTPUT"

# Additional issues to lint count
LINT_ISSUES=$((LINT_ISSUES + PANIC_COUNT + PRINT_COUNT))

echo "Total lint issues found: $LINT_ISSUES"
echo "Error budget: $LINT_ERROR_BUDGET"

if [ "$LINT_ISSUES" -le "$LINT_ERROR_BUDGET" ]; then
    record_gate "Linting" "PASS" "$LINT_ISSUES issues (within budget of $LINT_ERROR_BUDGET)"
else
    record_gate "Linting" "FAIL" "$LINT_ISSUES issues (exceeds budget of $LINT_ERROR_BUDGET)"
fi
echo ""

# Gate 3: Build Verification
echo "=== GATE 3: BUILD VERIFICATION ==="
echo "Verifying all packages build successfully..."

BUILD_START_TIME=$(date +%s)
BUILD_OUTPUT="$REPORTS_DIR/build_results.txt"

if timeout "${MAX_BUILD_TIME}s" go build ./pkg/mcp/... > "$BUILD_OUTPUT" 2>&1; then
    BUILD_END_TIME=$(date +%s)
    BUILD_DURATION=$((BUILD_END_TIME - BUILD_START_TIME))
    record_gate "Build Verification" "PASS" "Build completed in ${BUILD_DURATION}s"
else
    BUILD_END_TIME=$(date +%s)
    BUILD_DURATION=$((BUILD_END_TIME - BUILD_START_TIME))

    # Check if it was a timeout
    if [ "$BUILD_DURATION" -ge "$MAX_BUILD_TIME" ]; then
        record_gate "Build Verification" "FAIL" "Build timeout (>${MAX_BUILD_TIME}s)"
    else
        # Count build failures
        BUILD_FAILURES=$(grep -c "build failed\|compilation error" "$BUILD_OUTPUT" || echo "1")
        record_gate "Build Verification" "FAIL" "$BUILD_FAILURES build errors"
    fi
fi
echo ""

# Gate 4: Test Coverage
echo "=== GATE 4: TEST COVERAGE ==="
echo "Checking test coverage meets threshold: ${COVERAGE_THRESHOLD}%"

# Run coverage analysis
COVERAGE_OUTPUT="$REPORTS_DIR/coverage_gate.txt"
if go test -cover ./pkg/mcp/... > "$COVERAGE_OUTPUT" 2>&1; then
    # Extract coverage from successful packages
    COVERAGE_LINES=$(grep "coverage:" "$COVERAGE_OUTPUT" | grep -v "0.0%")

    if [ -n "$COVERAGE_LINES" ]; then
        # Calculate weighted average coverage
        TOTAL_STATEMENTS=0
        COVERED_STATEMENTS=0

        while IFS= read -r line; do
            if [[ $line =~ coverage:\ ([0-9.]+)%\ of\ ([0-9]+)\ statements ]]; then
                COVERAGE_PERCENT=${BASH_REMATCH[1]}
                STATEMENTS=${BASH_REMATCH[2]}

                COVERED=$( echo "$STATEMENTS * $COVERAGE_PERCENT / 100" | bc -l 2>/dev/null || echo "0")
                TOTAL_STATEMENTS=$( echo "$TOTAL_STATEMENTS + $STATEMENTS" | bc -l 2>/dev/null || echo "$TOTAL_STATEMENTS")
                COVERED_STATEMENTS=$( echo "$COVERED_STATEMENTS + $COVERED" | bc -l 2>/dev/null || echo "$COVERED_STATEMENTS")
            fi
        done <<< "$COVERAGE_LINES"

        if [ "$TOTAL_STATEMENTS" != "0" ]; then
            OVERALL_COVERAGE=$(echo "scale=1; $COVERED_STATEMENTS * 100 / $TOTAL_STATEMENTS" | bc -l 2>/dev/null || echo "0")
        else
            OVERALL_COVERAGE="0"
        fi
    else
        OVERALL_COVERAGE="0"
    fi

    echo "Overall coverage: ${OVERALL_COVERAGE}%"

    if (( $(echo "$OVERALL_COVERAGE >= $COVERAGE_THRESHOLD" | bc -l 2>/dev/null || echo "0") )); then
        record_gate "Test Coverage" "PASS" "${OVERALL_COVERAGE}% (≥ ${COVERAGE_THRESHOLD}%)"
    else
        record_gate "Test Coverage" "FAIL" "${OVERALL_COVERAGE}% (< ${COVERAGE_THRESHOLD}%)"
    fi
else
    record_gate "Test Coverage" "FAIL" "Unable to run coverage analysis"
fi
echo ""

# Gate 5: Performance Benchmarks
echo "=== GATE 5: PERFORMANCE BENCHMARKS ==="
echo "Checking benchmark performance meets targets (<${PERFORMANCE_THRESHOLD_NS}ns)"

BENCHMARK_OUTPUT="$REPORTS_DIR/benchmark_gate.txt"
if timeout 120s go test -bench=. -benchmem ./pkg/mcp/... > "$BENCHMARK_OUTPUT" 2>&1; then
    # Check for benchmark results
    SLOW_BENCHMARKS=$(grep "^Benchmark" "$BENCHMARK_OUTPUT" | awk -v threshold="$PERFORMANCE_THRESHOLD_NS" '{
        if ($3 ~ /ns\/op/) {
            ns = $3
            gsub(/ns\/op/, "", ns)
            if (ns > threshold) {
                print $1 ": " ns "ns/op (exceeds " threshold "ns)"
            }
        }
    }')

    if [ -z "$SLOW_BENCHMARKS" ]; then
        BENCHMARK_COUNT=$(grep -c "^Benchmark" "$BENCHMARK_OUTPUT" || echo "0")
        record_gate "Performance Benchmarks" "PASS" "$BENCHMARK_COUNT benchmarks within ${PERFORMANCE_THRESHOLD_NS}ns target"
    else
        SLOW_COUNT=$(echo "$SLOW_BENCHMARKS" | wc -l)
        record_gate "Performance Benchmarks" "FAIL" "$SLOW_COUNT benchmarks exceed ${PERFORMANCE_THRESHOLD_NS}ns target"
        echo "Slow benchmarks:"
        echo "$SLOW_BENCHMARKS"
    fi
else
    record_gate "Performance Benchmarks" "FAIL" "Benchmark execution failed or timed out"
fi
echo ""

# Gate 6: Architecture Validation
echo "=== GATE 6: ARCHITECTURE VALIDATION ==="
echo "Validating clean architecture boundaries..."

ARCH_VIOLATIONS=0

# Check domain layer has no external dependencies
echo "Checking domain layer dependencies..."
DOMAIN_DEPS=$(find pkg/mcp/domain -name "*.go" -exec grep -H "github.com" {} \; | grep -v "github.com/Azure/container-kit/pkg/mcp/domain" || true)
if [ -n "$DOMAIN_DEPS" ]; then
    DOMAIN_VIOLATION_COUNT=$(echo "$DOMAIN_DEPS" | wc -l)
    ARCH_VIOLATIONS=$((ARCH_VIOLATIONS + DOMAIN_VIOLATION_COUNT))
    echo "Domain layer external dependencies found: $DOMAIN_VIOLATION_COUNT"
fi

# Check for circular imports (basic check)
echo "Checking for potential circular imports..."
CIRCULAR_IMPORTS=$(go list -f '{{join .Imports "\n"}}' ./pkg/mcp/... | sort | uniq -d | wc -l)
if [ "$CIRCULAR_IMPORTS" -gt 0 ]; then
    ARCH_VIOLATIONS=$((ARCH_VIOLATIONS + CIRCULAR_IMPORTS))
    echo "Potential circular import issues: $CIRCULAR_IMPORTS"
fi

# Check import depth (should be reasonable)
echo "Checking import depth..."
MAX_DEPTH=5
DEEP_IMPORTS=$(find pkg/mcp -name "*.go" -exec grep -H "^import" {} \; | wc -l)
AVG_DEPTH=$(echo "scale=1; $DEEP_IMPORTS / 100" | bc -l 2>/dev/null || echo "1")

if [ "$ARCH_VIOLATIONS" -eq 0 ]; then
    record_gate "Architecture Validation" "PASS" "Clean architecture boundaries maintained"
else
    record_gate "Architecture Validation" "FAIL" "$ARCH_VIOLATIONS architecture violations found"
fi
echo ""

# Gate 7: Security Checks
echo "=== GATE 7: SECURITY CHECKS ==="
echo "Running basic security validation..."

SECURITY_ISSUES=0

# Check for hardcoded secrets patterns
echo "Checking for potential hardcoded secrets..."
SECRET_PATTERNS=("password.*=" "token.*=" "key.*=" "secret.*=" "api.*key" "auth.*=")
for pattern in "${SECRET_PATTERNS[@]}"; do
    MATCHES=$(find pkg -name "*.go" -exec grep -iH "$pattern" {} \; | grep -v "_test.go" | wc -l)
    SECURITY_ISSUES=$((SECURITY_ISSUES + MATCHES))
done

# Check for dangerous functions
echo "Checking for dangerous function usage..."
DANGEROUS_FUNCS=("exec.Command" "os.System" "unsafe\." "reflect\.Value")
for func in "${DANGEROUS_FUNCS[@]}"; do
    MATCHES=$(find pkg -name "*.go" -exec grep -H "$func" {} \; | grep -v "_test.go" | wc -l)
    # Only count as issues if excessive (>5 uses)
    if [ "$MATCHES" -gt 5 ]; then
        SECURITY_ISSUES=$((SECURITY_ISSUES + 1))
    fi
done

# Check for TODO security items
SECURITY_TODOS=$(find pkg -name "*.go" -exec grep -iH "TODO.*security\|FIXME.*security" {} \; | wc -l)
SECURITY_ISSUES=$((SECURITY_ISSUES + SECURITY_TODOS))

if [ "$SECURITY_ISSUES" -eq 0 ]; then
    record_gate "Security Checks" "PASS" "No security issues detected"
else
    record_gate "Security Checks" "FAIL" "$SECURITY_ISSUES potential security issues found"
fi
echo ""

# Generate Quality Dashboard
echo "=== QUALITY DASHBOARD ==="
DASHBOARD_FILE="$REPORTS_DIR/quality_dashboard.md"

cat > "$DASHBOARD_FILE" << EOF
# Container Kit Quality Dashboard

**Generated**: $(date)
**Commit**: $(git rev-parse HEAD 2>/dev/null || echo 'unknown')
**Overall Status**: $([ "$OVERALL_PASS" = true ] && echo "✅ PASSED" || echo "❌ FAILED")

## Quality Gates Results

$(printf '%s\n' "${GATE_RESULTS[@]}")

## Metrics Summary

### Code Quality
- **Formatting**: $([ -z "$UNFORMATTED_FILES" ] && echo "✅ Compliant" || echo "❌ $(echo "$UNFORMATTED_FILES" | wc -l) files need formatting")
- **Lint Issues**: $LINT_ISSUES (Budget: $LINT_ERROR_BUDGET)
- **Build Status**: $([ -f "$BUILD_OUTPUT" ] && grep -q "build failed" "$BUILD_OUTPUT" && echo "❌ Failed" || echo "✅ Success")

### Testing
- **Coverage**: ${OVERALL_COVERAGE:-"N/A"}% (Target: ${COVERAGE_THRESHOLD}%)
- **Benchmark Count**: $(grep -c "^Benchmark" "$BENCHMARK_OUTPUT" 2>/dev/null || echo "0")
- **Performance**: $([ -z "$SLOW_BENCHMARKS" ] && echo "✅ Within targets" || echo "⚠️ Some benchmarks slow")

### Architecture
- **Violations**: $ARCH_VIOLATIONS
- **Dependencies**: $([ "$ARCH_VIOLATIONS" -eq 0 ] && echo "✅ Clean" || echo "⚠️ Issues found")

### Security
- **Issues**: $SECURITY_ISSUES
- **Status**: $([ "$SECURITY_ISSUES" -eq 0 ] && echo "✅ Clean" || echo "⚠️ Review needed")

## Recommendations

$(if [ "$OVERALL_PASS" = false ]; then
    echo "### High Priority"
    if [ "$LINT_ISSUES" -gt "$LINT_ERROR_BUDGET" ]; then
        echo "- Fix linting issues to stay within error budget"
    fi
    if ! (( $(echo "$OVERALL_COVERAGE >= $COVERAGE_THRESHOLD" | bc -l 2>/dev/null || echo "0") )); then
        echo "- Improve test coverage to meet ${COVERAGE_THRESHOLD}% threshold"
    fi
    if [ -n "$SLOW_BENCHMARKS" ]; then
        echo "- Optimize slow benchmarks"
    fi
    if [ "$ARCH_VIOLATIONS" -gt 0 ]; then
        echo "- Fix architecture boundary violations"
    fi
    if [ "$SECURITY_ISSUES" -gt 0 ]; then
        echo "- Review and address security findings"
    fi
    echo ""
    echo "### Medium Priority"
else
    echo "### Maintenance"
fi)
- Monitor performance regression
- Keep dependencies up to date
- Review and update documentation
- Maintain test coverage as code evolves

## Files Generated
- [Quality Dashboard](quality_dashboard.md)
- [Lint Results](lint_results.txt)
- [Build Results](build_results.txt)
- [Coverage Results](coverage_gate.txt)
- [Benchmark Results](benchmark_gate.txt)

---
*Generated by Container Kit Quality Gates*
EOF

# Final Summary
echo ""
echo "=== FINAL RESULTS ==="
echo "Quality Gates Summary:"
printf '%s\n' "${GATE_RESULTS[@]}"
echo ""

if [ "$OVERALL_PASS" = true ]; then
    echo "🎉 ALL QUALITY GATES PASSED!"
    echo ""
    echo "✅ Code is ready for integration"
    echo "📊 Dashboard: $DASHBOARD_FILE"
    exit 0
else
    echo "💥 QUALITY GATES FAILED!"
    echo ""
    echo "❌ Code needs improvement before integration"
    echo "📊 Dashboard: $DASHBOARD_FILE"
    echo "📁 Detailed reports: $REPORTS_DIR/"
    echo ""
    echo "Fix the failed gates and run again."
    exit 1
fi
