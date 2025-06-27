#!/bin/bash
# Lint with error budget/threshold support

# Configuration
ERROR_THRESHOLD=${LINT_ERROR_THRESHOLD:-50}
WARN_THRESHOLD=${LINT_WARN_THRESHOLD:-30}
LINT_ARGS="${@:-./pkg/mcp/...}"

echo "Running linter with error budget..."
echo "Error threshold: $ERROR_THRESHOLD"
echo "Warning threshold: $WARN_THRESHOLD"
echo ""

# Run golangci-lint and capture output
LINT_OUTPUT=$(golangci-lint run $LINT_ARGS 2>&1)
LINT_EXIT_CODE=$?

# Count issues
if [ $LINT_EXIT_CODE -eq 0 ]; then
    ISSUE_COUNT=0
    echo "✅ No linting issues found!"
else
    # Count issues from text output
    ISSUE_COUNT=$(echo "$LINT_OUTPUT" | grep -E "^[^:]+:[0-9]+:[0-9]+:" | wc -l)

    echo "Found $ISSUE_COUNT linting issues"

    # Show summary by linter
    echo ""
    echo "Issues by linter:"
    # Since we can't use JSON output with v2.1.0, parse text output
        golangci-lint run $LINT_ARGS 2>&1 | grep -oE '\([a-z]+\)$' | sort | uniq -c
fi

echo ""

# Determine exit status based on threshold
if [ $ISSUE_COUNT -gt $ERROR_THRESHOLD ]; then
    echo "❌ FAILED: Issue count ($ISSUE_COUNT) exceeds error threshold ($ERROR_THRESHOLD)"
    echo ""
    echo "To see detailed issues, run:"
    echo "  golangci-lint run $LINT_ARGS"
    exit 1
elif [ $ISSUE_COUNT -gt $WARN_THRESHOLD ]; then
    echo "⚠️  WARNING: Issue count ($ISSUE_COUNT) exceeds warning threshold ($WARN_THRESHOLD)"
    echo "Consider reducing technical debt before it reaches the error threshold."
    exit 0
else
    echo "✅ PASSED: Issue count ($ISSUE_COUNT) is within acceptable limits"
    exit 0
fi
