#!/bin/bash

set -euo pipefail

# Configuration  
QUALITY_CONFIG="quality-gates.json"

# Create reports directory if it doesn't exist for lint output
mkdir -p reports

# Check for required tools
if ! command -v jq &> /dev/null; then
    echo "Error: jq is required but not installed."
    exit 1
fi

echo "=== Code Quality Metrics $(date) ==="
echo ""

# ESLint Analysis  
echo "📋 ESLint Analysis"
echo "────────────────────"

# Run lint and capture output in a variable
LINT_OUTPUT=$(npm run lint 2>&1 || true)

# Count warnings and errors more precisely from the summary line
if [ -n "$LINT_OUTPUT" ]; then
    # Parse the final summary line like "✖ 794 problems (1 error, 793 warnings)"
    SUMMARY_LINE=$(echo "$LINT_OUTPUT" | grep -E "problems.*error.*warning" | tail -1 2>/dev/null || echo "")
    if [ -n "$SUMMARY_LINE" ]; then
        TOTAL_ERRORS=$(echo "$SUMMARY_LINE" | sed -n 's/.*(\([0-9]\+\) error.*/\1/p' 2>/dev/null || echo "0")
        TOTAL_WARNINGS=$(echo "$SUMMARY_LINE" | sed -n 's/.*, \([0-9]\+\) warning.*/\1/p' 2>/dev/null || echo "0")
        # Handle case where it says "0 errors"
        if [ -z "$TOTAL_ERRORS" ]; then
            TOTAL_ERRORS=0
        fi
        if [ -z "$TOTAL_WARNINGS" ]; then
            TOTAL_WARNINGS=0
        fi
    else
        # Fallback to counting individual lines
        TOTAL_WARNINGS=$(echo "$LINT_OUTPUT" | grep -c "warning" 2>/dev/null || echo "0")
        TOTAL_ERRORS=$(echo "$LINT_OUTPUT" | grep -c "error" 2>/dev/null || echo "0")
    fi
else
    TOTAL_WARNINGS=0
    TOTAL_ERRORS=0
fi

# Update current metrics in JSON
jq --arg warnings "$TOTAL_WARNINGS" --arg errors "$TOTAL_ERRORS" \
   '.metrics.lint.current = ($warnings | tonumber) | .metrics.lint.warnings = ($warnings | tonumber) | .metrics.lint.errors = ($errors | tonumber)' \
   $QUALITY_CONFIG > ${QUALITY_CONFIG}.tmp && mv ${QUALITY_CONFIG}.tmp $QUALITY_CONFIG

echo "Total warnings: $TOTAL_WARNINGS"
echo "Total errors: $TOTAL_ERRORS"
echo ""

echo "=== Top 10 Warning Types ==="
if [ -n "$LINT_OUTPUT" ]; then
    # Extract and categorize warning types from ESLint rule names
    echo "$LINT_OUTPUT" | grep "warning" 2>/dev/null | \
        grep -o "@typescript-eslint/[a-zA-Z0-9-]*" | \
        sed 's/@typescript-eslint\///' | \
        sort | uniq -c | sort -rn | head -10 | \
        awk '{printf "%5d  %s\n", $1, $2}' || \
    echo "$LINT_OUTPUT" | grep "warning" 2>/dev/null | \
        sed -n 's/.*warning[[:space:]]*\([^[:space:]]*\).*/\1/p' | \
        sort | uniq -c | sort -rn | head -10 | \
        awk '{printf "%5d  %s\n", $1, $2}' || echo "No warnings found in lint output"
else
    echo "No lint output captured"
fi
echo ""

echo "=== Progress Tracking ==="

# Read baseline from JSON
BASELINE=$(jq -r '.metrics.lint.baseline' $QUALITY_CONFIG)

if [ "$BASELINE" != "null" ]; then
    REDUCTION=$((BASELINE - TOTAL_WARNINGS))
    if [ $BASELINE -gt 0 ]; then
        PERCENTAGE=$(echo "scale=1; ($REDUCTION * 100) / $BASELINE" | bc -l 2>/dev/null || echo "N/A")
    else
        PERCENTAGE="N/A"
    fi
    
    echo "Baseline: $BASELINE warnings"
    echo "Current: $TOTAL_WARNINGS warnings"
    
    if [ $REDUCTION -gt 0 ]; then
        echo "✅ Reduced by: $REDUCTION warnings ($PERCENTAGE%)"
    elif [ $REDUCTION -eq 0 ]; then
        echo "✅ Maintaining baseline (no increase)"
    else
        echo "❌ Increased by: $((-REDUCTION)) warnings"
    fi
else
    echo "Baseline: Not set (run with --baseline to set)"
    echo "To set baseline: ./scripts/lint-metrics.sh --baseline"
fi

echo ""
echo "🧹 Deadcode Analysis"
echo "────────────────────"

# Run ts-prune to find unused exports
echo "Analyzing unused exports..."
DEADCODE_COUNT=$(npx ts-prune --project tsconfig.json 2>/dev/null | grep -v 'used in module' | wc -l | tr -d ' ' || echo "0")
echo "Total unused exports: $DEADCODE_COUNT"
echo ""

# Update deadcode metrics in JSON
jq --arg count "$DEADCODE_COUNT" \
   '.metrics.deadcode.current = ($count | tonumber)' \
   $QUALITY_CONFIG > ${QUALITY_CONFIG}.tmp && mv ${QUALITY_CONFIG}.tmp $QUALITY_CONFIG

# Read deadcode baseline from JSON
DEADCODE_BASELINE=$(jq -r '.metrics.deadcode.baseline' $QUALITY_CONFIG)

if [ "$DEADCODE_BASELINE" != "null" ]; then
    DEADCODE_REDUCTION=$((DEADCODE_BASELINE - DEADCODE_COUNT))
    if [ $DEADCODE_BASELINE -gt 0 ]; then
        DEADCODE_PERCENTAGE=$(echo "scale=1; ($DEADCODE_REDUCTION * 100) / $DEADCODE_BASELINE" | bc -l 2>/dev/null || echo "N/A")
    else
        DEADCODE_PERCENTAGE="N/A"
    fi
    
    echo "Deadcode Progress:"
    echo "  Baseline: $DEADCODE_BASELINE unused exports"
    echo "  Current: $DEADCODE_COUNT unused exports"
    
    if [ $DEADCODE_REDUCTION -gt 0 ]; then
        echo "  ✅ Reduced by: $DEADCODE_REDUCTION exports ($DEADCODE_PERCENTAGE%)"
    elif [ $DEADCODE_REDUCTION -eq 0 ]; then
        echo "  ✅ Maintaining baseline (no increase)"
    else
        echo "  ❌ Increased by: $((-DEADCODE_REDUCTION)) exports"
    fi
else
    echo "Deadcode baseline: Not set"
    echo "To set baseline: ./scripts/lint-metrics.sh --baseline"
fi

# Save baselines if --baseline flag is provided
if [ "${1:-}" == "--baseline" ]; then
    jq --arg warnings "$TOTAL_WARNINGS" --arg deadcode "$DEADCODE_COUNT" \
       '.metrics.lint.baseline = ($warnings | tonumber) | .metrics.deadcode.baseline = ($deadcode | tonumber)' \
       $QUALITY_CONFIG > ${QUALITY_CONFIG}.tmp && mv ${QUALITY_CONFIG}.tmp $QUALITY_CONFIG
    echo ""
    echo "✅ Baselines updated in $QUALITY_CONFIG:"
    echo "  • Lint baseline: $TOTAL_WARNINGS warnings"
    echo "  • Deadcode baseline: $DEADCODE_COUNT unused exports"
fi

# Show top files with unused exports
echo ""
echo "=== Top 5 Files with Unused Exports ==="
npx ts-prune --project tsconfig.json 2>/dev/null | grep -v 'used in module' | \
    cut -d':' -f1 | sort | uniq -c | sort -rn | head -5 | \
    awk '{printf "%3d unused exports: %s\n", $1, $2}' || echo "No unused exports found"

# Exit with error if there are lint errors (not warnings)
if [ "$TOTAL_ERRORS" -gt 0 ]; then
    echo ""
    echo "❌ Cannot proceed: $TOTAL_ERRORS lint errors must be fixed first"
    exit 1
fi

# Summary
echo ""
echo "📊 Summary"
echo "────────────"
echo "ESLint: $TOTAL_WARNINGS warnings, $TOTAL_ERRORS errors"
echo "Deadcode: $DEADCODE_COUNT unused exports"
echo ""
echo "To update baselines: ./scripts/lint-metrics.sh --baseline"