#!/bin/bash

# Script to validate CI workflow consistency with quality config

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
CONFIG_FILE="$ROOT_DIR/.github/quality-config.json"
WORKFLOWS_DIR="$ROOT_DIR/.github/workflows"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "Validating CI workflow consistency..."
echo "===================================="

# Check if config file exists
if [ ! -f "$CONFIG_FILE" ]; then
    echo -e "${RED}❌ Quality config file not found: $CONFIG_FILE${NC}"
    exit 1
fi

# Function to check for hardcoded values
check_hardcoded_values() {
    local file=$1
    local issues=0

    echo "Checking $file..."

    # Check for hardcoded complexity thresholds
    if grep -q "gocyclo -over [0-9]" "$file" 2>/dev/null; then
        threshold=$(grep -o "gocyclo -over [0-9]*" "$file" | grep -o "[0-9]*" | head -1)
        config_threshold=$(jq -r '.complexity.thresholds.warn' "$CONFIG_FILE")
        if [ "$threshold" != "$config_threshold" ]; then
            echo -e "${YELLOW}  ⚠️  Hardcoded complexity threshold: $threshold (config: $config_threshold)${NC}"
            issues=$((issues + 1))
        fi
    fi

    # Check for hardcoded lint thresholds
    if grep -q "LINT_ERROR_THRESHOLD=[0-9]" "$file" 2>/dev/null; then
        threshold=$(grep -o "LINT_ERROR_THRESHOLD=[0-9]*" "$file" | grep -o "[0-9]*" | head -1)
        echo -e "${YELLOW}  ⚠️  Hardcoded lint threshold: $threshold${NC}"
        issues=$((issues + 1))
    fi

    # Check for TODO limit
    if grep -q "gt [0-9].*TODO" "$file" 2>/dev/null; then
        limit=$(grep -o "gt [0-9]*.*TODO" "$file" | grep -o "[0-9]*" | head -1)
        config_limit=$(jq -r '.technical_debt.todo_limit_per_pr' "$CONFIG_FILE")
        if [ "$limit" != "$config_limit" ]; then
            echo -e "${YELLOW}  ⚠️  Hardcoded TODO limit: $limit (config: $config_limit)${NC}"
            issues=$((issues + 1))
        fi
    fi

    # Check for strict exit 1 without tolerance
    if grep -q "exit 1.*formatting\|exit 1.*TODO" "$file" 2>/dev/null; then
        echo -e "${YELLOW}  ⚠️  Strict failure without tolerance for formatting/TODOs${NC}"
        issues=$((issues + 1))
    fi

    return $issues
}

# Check all workflow files
total_issues=0
for workflow in "$WORKFLOWS_DIR"/*.yml; do
    if check_hardcoded_values "$workflow"; then
        echo -e "${GREEN}  ✅ Consistent with quality config${NC}"
    else
        total_issues=$((total_issues + $?))
    fi
    echo ""
done

# Summary
echo "Summary"
echo "======="
if [ "$total_issues" -eq 0 ]; then
    echo -e "${GREEN}✅ All workflows are consistent with quality config${NC}"
else
    echo -e "${YELLOW}⚠️  Found $total_issues inconsistencies across workflows${NC}"
    echo ""
    echo "Recommendations:"
    echo "1. Update workflows to read thresholds from .github/quality-config.json"
    echo "2. Use consistent ratcheting approach across all checks"
    echo "3. Prefer warnings over failures for non-critical issues"
fi

# Check for workflows not using ratcheting
echo ""
echo "Ratcheting Strategy Compliance"
echo "=============================="
for workflow in "$WORKFLOWS_DIR"/*.yml; do
    name=$(basename "$workflow")

    # Skip certain workflows that don't need ratcheting
    if [[ "$name" == "release.yml" || "$name" == "docker-"* ]]; then
        continue
    fi

    if grep -q "new-from-rev\|check.*only.*changed\|diff.*origin" "$workflow" 2>/dev/null; then
        echo -e "${GREEN}✅ $name - Uses ratcheting/incremental checks${NC}"
    else
        if grep -q "lint\|complexity\|quality" "$workflow" 2>/dev/null; then
            echo -e "${YELLOW}⚠️  $name - May not use ratcheting strategy${NC}"
        fi
    fi
done
