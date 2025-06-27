#!/bin/bash
# Ratcheting lint script - only allows issue count to decrease or stay the same

BASELINE_FILE=".lint-baseline"
LINT_ARGS="${@:-./pkg/mcp/...}"

# Get current issue count
CURRENT_COUNT=$(golangci-lint run $LINT_ARGS 2>&1 | grep -E "^[^:]+:[0-9]+:[0-9]+:" | wc -l || echo "999")

# Get baseline count
if [ -f "$BASELINE_FILE" ]; then
    BASELINE_COUNT=$(cat "$BASELINE_FILE")
else
    BASELINE_COUNT=$CURRENT_COUNT
    echo $BASELINE_COUNT > "$BASELINE_FILE"
    echo "Created baseline with $BASELINE_COUNT issues"
fi

echo "Lint Ratchet Check"
echo "=================="
echo "Baseline: $BASELINE_COUNT issues"
echo "Current:  $CURRENT_COUNT issues"
echo ""

if [ $CURRENT_COUNT -gt $BASELINE_COUNT ]; then
    echo "❌ FAILED: Issue count increased! ($CURRENT_COUNT > $BASELINE_COUNT)"
    echo "New issues must be fixed before merging."
    exit 1
elif [ $CURRENT_COUNT -lt $BASELINE_COUNT ]; then
    echo "✅ SUCCESS: Issue count decreased! ($CURRENT_COUNT < $BASELINE_COUNT)"
    echo "Updating baseline to $CURRENT_COUNT"
    echo $CURRENT_COUNT > "$BASELINE_FILE"
    exit 0
else
    echo "✅ SUCCESS: Issue count unchanged ($CURRENT_COUNT = $BASELINE_COUNT)"
    exit 0
fi
