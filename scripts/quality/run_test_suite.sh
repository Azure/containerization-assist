#!/bin/bash

# Comprehensive test runner with reporting
echo "=== Container Kit Test Suite Runner ==="
echo "Date: $(date)"
echo ""

# Configuration
TEST_TIMEOUT="${TEST_TIMEOUT:-10m}"
COVERAGE_THRESHOLD="${COVERAGE_THRESHOLD:-15.0}"
REPORTS_DIR="test/reports"
COVERAGE_DIR="test/coverage"

mkdir -p "$REPORTS_DIR" "$COVERAGE_DIR"

# Parse command line options
RUN_UNIT=true
RUN_INTEGRATION=false
RUN_BENCHMARKS=false
GENERATE_COVERAGE=true
VERBOSE=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --integration)
            RUN_INTEGRATION=true
            shift
            ;;
        --benchmarks)
            RUN_BENCHMARKS=true
            shift
            ;;
        --unit-only)
            RUN_INTEGRATION=false
            RUN_BENCHMARKS=false
            shift
            ;;
        --no-coverage)
            GENERATE_COVERAGE=false
            shift
            ;;
        --verbose)
            VERBOSE=true
            shift
            ;;
        --threshold)
            COVERAGE_THRESHOLD="$2"
            shift 2
            ;;
        --help)
            echo "Usage: $0 [options]"
            echo ""
            echo "Options:"
            echo "  --integration     Run integration tests"
            echo "  --benchmarks      Run benchmark tests"
            echo "  --unit-only       Run only unit tests"
            echo "  --no-coverage     Skip coverage generation"
            echo "  --verbose         Verbose output"
            echo "  --threshold N     Set coverage threshold (default: 15.0)"
            echo "  --help            Show this help"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Test execution function
run_tests() {
    local test_type="$1"
    local test_pattern="$2"
    local output_file="$3"
    
    echo "Running $test_type tests..."
    
    local cmd="go test"
    if [ "$GENERATE_COVERAGE" = true ] && [ "$test_type" = "unit" ]; then
        cmd="$cmd -coverprofile=$COVERAGE_DIR/${test_type}_coverage.out"
    fi
    
    if [ "$VERBOSE" = true ]; then
        cmd="$cmd -v"
    fi
    
    cmd="$cmd -timeout=$TEST_TIMEOUT"
    
    if [ -n "$test_pattern" ]; then
        cmd="$cmd -run=$test_pattern"
    fi
    
    cmd="$cmd ./pkg/mcp/..."
    
    echo "Executing: $cmd"
    if eval "$cmd" > "$output_file" 2>&1; then
        echo "✅ $test_type tests passed"
        return 0
    else
        echo "❌ $test_type tests failed"
        return 1
    fi
}

# Initialize test results
RESULTS=()
OVERALL_SUCCESS=true

# Run unit tests
if [ "$RUN_UNIT" = true ]; then
    echo "=== UNIT TESTS ==="
    if run_tests "unit" "^Test" "$REPORTS_DIR/unit_tests.txt"; then
        RESULTS+=("✅ Unit tests: PASSED")
    else
        RESULTS+=("❌ Unit tests: FAILED")
        OVERALL_SUCCESS=false
    fi
    echo ""
fi

# Run integration tests
if [ "$RUN_INTEGRATION" = true ]; then
    echo "=== INTEGRATION TESTS ==="
    if run_tests "integration" "^TestIntegration" "$REPORTS_DIR/integration_tests.txt"; then
        RESULTS+=("✅ Integration tests: PASSED")
    else
        RESULTS+=("❌ Integration tests: FAILED")
        OVERALL_SUCCESS=false
    fi
    echo ""
fi

# Run benchmark tests
if [ "$RUN_BENCHMARKS" = true ]; then
    echo "=== BENCHMARK TESTS ==="
    echo "Running benchmark tests..."
    
    if go test -bench=. -benchmem ./pkg/mcp/... > "$REPORTS_DIR/benchmark_tests.txt" 2>&1; then
        echo "✅ Benchmark tests completed"
        RESULTS+=("✅ Benchmarks: COMPLETED")
        
        # Extract benchmark results
        echo "Top benchmark results:"
        grep "^Benchmark" "$REPORTS_DIR/benchmark_tests.txt" | head -10
    else
        echo "❌ Benchmark tests failed"
        RESULTS+=("❌ Benchmarks: FAILED")
    fi
    echo ""
fi

# Generate coverage report
if [ "$GENERATE_COVERAGE" = true ] && [ "$RUN_UNIT" = true ]; then
    echo "=== COVERAGE ANALYSIS ==="
    
    if [ -f "$COVERAGE_DIR/unit_coverage.out" ]; then
        # Generate HTML report
        go tool cover -html="$COVERAGE_DIR/unit_coverage.out" -o "$REPORTS_DIR/coverage.html"
        
        # Calculate overall coverage
        COVERAGE_PERCENT=$(go tool cover -func="$COVERAGE_DIR/unit_coverage.out" | grep total | awk '{print $3}' | sed 's/%//')
        
        echo "Overall coverage: ${COVERAGE_PERCENT}%"
        echo "Coverage threshold: ${COVERAGE_THRESHOLD}%"
        
        # Check coverage threshold
        if (( $(echo "$COVERAGE_PERCENT >= $COVERAGE_THRESHOLD" | bc -l 2>/dev/null || echo "0") )); then
            echo "✅ Coverage meets threshold"
            RESULTS+=("✅ Coverage: ${COVERAGE_PERCENT}% (≥ ${COVERAGE_THRESHOLD}%)")
        else
            echo "❌ Coverage below threshold"
            RESULTS+=("❌ Coverage: ${COVERAGE_PERCENT}% (< ${COVERAGE_THRESHOLD}%)")
            OVERALL_SUCCESS=false
        fi
        
        # Generate detailed coverage report
        echo "Generating detailed coverage analysis..."
        scripts/quality/coverage_tracker.sh > "$REPORTS_DIR/detailed_coverage.txt"
        
    else
        echo "❌ No coverage data generated"
        RESULTS+=("❌ Coverage: NO DATA")
        OVERALL_SUCCESS=false
    fi
    echo ""
fi

# Test result analysis
echo "=== TEST ANALYSIS ==="

# Count packages with tests
PACKAGES_WITH_TESTS=$(find pkg/mcp -name "*_test.go" -exec dirname {} \; | sort -u | wc -l)
TOTAL_PACKAGES=$(find pkg/mcp -name "*.go" -not -name "*_test.go" -exec dirname {} \; | sort -u | wc -l)
TEST_COVERAGE_RATIO=$(( PACKAGES_WITH_TESTS * 100 / TOTAL_PACKAGES ))

echo "Package test coverage: $PACKAGES_WITH_TESTS/$TOTAL_PACKAGES packages (${TEST_COVERAGE_RATIO}%)"

# Count test functions
UNIT_TEST_COUNT=$(find pkg/mcp -name "*_test.go" -exec grep -h "^func Test" {} \; | wc -l)
BENCHMARK_COUNT=$(find pkg/mcp -name "*_test.go" -exec grep -h "^func Benchmark" {} \; | wc -l)

echo "Test function count:"
echo "  - Unit tests: $UNIT_TEST_COUNT"
echo "  - Benchmarks: $BENCHMARK_COUNT"

# Check for test quality indicators
echo ""
echo "Test quality indicators:"

# Check for table-driven tests
TABLE_TESTS=$(find pkg/mcp -name "*_test.go" -exec grep -l "tests := \[\]struct" {} \; | wc -l)
echo "  - Table-driven tests: $TABLE_TESTS files"

# Check for mock usage
MOCK_TESTS=$(find pkg/mcp -name "*_test.go" -exec grep -l "mock\." {} \; | wc -l)
echo "  - Tests with mocks: $MOCK_TESTS files"

# Check for testify usage
TESTIFY_TESTS=$(find pkg/mcp -name "*_test.go" -exec grep -l "github.com/stretchr/testify" {} \; | wc -l)
echo "  - Tests using testify: $TESTIFY_TESTS files"

echo ""

# Performance analysis
if [ "$RUN_BENCHMARKS" = true ] && [ -f "$REPORTS_DIR/benchmark_tests.txt" ]; then
    echo "=== PERFORMANCE ANALYSIS ==="
    
    # Check for slow benchmarks (>300μs target)
    SLOW_BENCHMARKS=$(grep "^Benchmark" "$REPORTS_DIR/benchmark_tests.txt" | awk '{
        if ($3 ~ /ns\/op/) {
            ns = $3
            gsub(/ns\/op/, "", ns)
            if (ns > 300000) print $1 ": " ns "ns/op (>" 300000 "ns target)"
        }
    }')
    
    if [ -n "$SLOW_BENCHMARKS" ]; then
        echo "⚠️  Benchmarks exceeding 300μs target:"
        echo "$SLOW_BENCHMARKS"
    else
        echo "✅ All benchmarks within 300μs target"
    fi
    echo ""
fi

# Generate summary report
echo "=== SUMMARY REPORT ==="

# Create summary file
SUMMARY_FILE="$REPORTS_DIR/test_summary.md"
cat > "$SUMMARY_FILE" << EOF
# Test Suite Summary

**Execution Date**: $(date)
**Overall Status**: $([ "$OVERALL_SUCCESS" = true ] && echo "✅ PASSED" || echo "❌ FAILED")

## Test Results

$(printf '%s\n' "${RESULTS[@]}")

## Package Coverage

- Packages with tests: $PACKAGES_WITH_TESTS/$TOTAL_PACKAGES (${TEST_COVERAGE_RATIO}%)
- Unit test functions: $UNIT_TEST_COUNT
- Benchmark functions: $BENCHMARK_COUNT

## Test Quality

- Table-driven tests: $TABLE_TESTS files
- Tests with mocks: $MOCK_TESTS files  
- Tests using testify: $TESTIFY_TESTS files

## Generated Reports

- [Coverage Report](coverage.html)
- [Unit Test Results](unit_tests.txt)
$([ "$RUN_INTEGRATION" = true ] && echo "- [Integration Test Results](integration_tests.txt)")
$([ "$RUN_BENCHMARKS" = true ] && echo "- [Benchmark Results](benchmark_tests.txt)")
- [Detailed Coverage Analysis](detailed_coverage.txt)

## Next Steps

$(if [ "$OVERALL_SUCCESS" = false ]; then
    echo "1. Review failed test results"
    echo "2. Fix failing tests"
    echo "3. Improve test coverage where needed"
else
    echo "1. Maintain current test quality"
    echo "2. Add tests for new features"
    echo "3. Monitor performance regressions"
fi)
EOF

# Display results
echo ""
echo "Test Results Summary:"
printf '%s\n' "${RESULTS[@]}"
echo ""

if [ "$OVERALL_SUCCESS" = true ]; then
    echo "🎉 All tests passed!"
    exit 0
else
    echo "💥 Some tests failed. Check the reports for details."
    echo ""
    echo "Reports generated:"
    echo "  - Summary: $SUMMARY_FILE"
    echo "  - Reports directory: $REPORTS_DIR/"
    
    if [ "$GENERATE_COVERAGE" = true ]; then
        echo "  - Coverage report: $REPORTS_DIR/coverage.html"
    fi
    
    exit 1
fi