#!/bin/bash

# Quality Dashboard Runner Script
# This script demonstrates various ways to use the quality dashboard

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "🎯 MCP Quality Dashboard"
echo "========================"
echo ""

# Function to run dashboard with specific options
run_dashboard() {
    local format=$1
    local output=$2
    local extra_args="${3:-}"

    echo "📊 Generating $format report..."
    go run "$SCRIPT_DIR/main.go" \
        -root "$ROOT_DIR" \
        -format "$format" \
        -output "$output" \
        $extra_args
}

# Generate all report formats
run_dashboard "json" "quality-metrics.json"
run_dashboard "html" "quality-dashboard.html"
run_dashboard "text" "-" | tee quality-summary.txt

echo ""
echo "📈 Quality Metrics Summary:"
echo "=========================="

# Extract key metrics from JSON
if command -v jq &> /dev/null; then
    ERROR_RATE=$(jq '.error_handling.adoption_rate' quality-metrics.json)
    COVERAGE=$(jq '.test_coverage.overall_coverage' quality-metrics.json)
    BUILD_TIME=$(jq -r '.build_metrics.build_time' quality-metrics.json)
    TODOS=$(jq '.code_quality.todo_comments' quality-metrics.json)

    echo "• Error Handling Adoption: ${ERROR_RATE}%"
    echo "• Test Coverage: ${COVERAGE}%"
    echo "• Build Time: ${BUILD_TIME}"
    echo "• TODO Comments: ${TODOS}"

    # Check quality gates
    echo ""
    echo "🚦 Quality Gates:"
    echo "================="

    if (( $(echo "$ERROR_RATE >= 80" | bc -l) )); then
        echo "✅ Error handling adoption meets target (≥80%)"
    else
        echo "❌ Error handling adoption below target (<80%)"
    fi

    if (( $(echo "$COVERAGE >= 70" | bc -l) )); then
        echo "✅ Test coverage meets target (≥70%)"
    else
        echo "❌ Test coverage below target (<70%)"
    fi
else
    echo "⚠️  Install jq for detailed metrics summary"
fi

echo ""
echo "📁 Generated Reports:"
echo "===================="
echo "• JSON: quality-metrics.json"
echo "• HTML: quality-dashboard.html"
echo "• Text: quality-summary.txt"

# Optional: Start watch mode
if [ "$1" == "--watch" ]; then
    echo ""
    echo "👀 Starting watch mode..."
    echo "========================"
    go run "$SCRIPT_DIR/main.go" \
        -root "$ROOT_DIR" \
        -watch \
        -interval "${2:-5m}" \
        -format text \
        -output -
fi
