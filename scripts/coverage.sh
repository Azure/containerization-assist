#!/bin/bash
# Coverage script for MCP package
# This script runs tests with coverage and enforces minimum coverage thresholds

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Coverage thresholds (based on current baseline + improvement targets)
declare -A COVERAGE_THRESHOLDS=(
    ["github.com/Azure/container-kit/pkg/mcp/internal/core"]=25.0
    ["github.com/Azure/container-kit/pkg/mcp/internal/build"]=10.0
    ["github.com/Azure/container-kit/pkg/mcp/internal/deploy"]=10.0
    ["github.com/Azure/container-kit/pkg/mcp/internal/analyze"]=5.0
    ["github.com/Azure/container-kit/pkg/mcp/internal/orchestration"]=10.0
    ["github.com/Azure/container-kit/pkg/mcp/internal/observability"]=35.0
    ["github.com/Azure/container-kit/pkg/mcp/internal/retry"]=45.0
    ["github.com/Azure/container-kit/pkg/mcp/internal/utils"]=45.0
    ["github.com/Azure/container-kit/pkg/mcp/internal/workflow"]=60.0
)

# Default threshold for packages not explicitly listed
DEFAULT_THRESHOLD=15.0

echo "Running MCP test coverage analysis..."
echo "=================================="

# Create coverage directory
mkdir -p coverage

# Run tests with coverage for all MCP packages
echo "Running tests with coverage..."
go test -coverprofile=coverage/coverage.out -covermode=atomic ./pkg/mcp/... > coverage/test_output.txt 2>&1

# Check if tests passed
if [ $? -ne 0 ]; then
    echo -e "${RED}Tests failed! See coverage/test_output.txt for details${NC}"
    cat coverage/test_output.txt
    exit 1
fi

echo -e "${GREEN}All tests passed!${NC}"

# Generate coverage report
echo ""
echo "Coverage Report:"
echo "----------------"

# Parse coverage data and check thresholds
FAILED_PACKAGES=()
COVERAGE_DATA=$(go test -cover ./pkg/mcp/... 2>&1 | grep -E "coverage:|ok" | grep "coverage:")

while IFS= read -r line; do
    if [[ $line =~ ^ok[[:space:]]+([^[:space:]]+)[[:space:]]+.*coverage:[[:space:]]+([0-9]+\.[0-9]+)%[[:space:]]of[[:space:]]statements ]]; then
        PACKAGE="${BASH_REMATCH[1]}"
        COVERAGE="${BASH_REMATCH[2]}"

        # Get threshold for this package
        if [[ -v COVERAGE_THRESHOLDS[$PACKAGE] ]]; then
            THRESHOLD=${COVERAGE_THRESHOLDS[$PACKAGE]}
        else
            THRESHOLD=$DEFAULT_THRESHOLD
        fi

        # Compare coverage with threshold
        if (( $(echo "$COVERAGE < $THRESHOLD" | bc -l) )); then
            echo -e "${RED}✗ $PACKAGE: $COVERAGE% (threshold: $THRESHOLD%)${NC}"
            FAILED_PACKAGES+=("$PACKAGE")
        else
            echo -e "${GREEN}✓ $PACKAGE: $COVERAGE% (threshold: $THRESHOLD%)${NC}"
        fi
    elif [[ $line =~ ^[[:space:]]+([^[:space:]]+)[[:space:]]+coverage:[[:space:]]+([0-9]+\.[0-9]+)%[[:space:]]of[[:space:]]statements ]]; then
        # Handle packages with no tests
        PACKAGE="${BASH_REMATCH[1]}"
        COVERAGE="${BASH_REMATCH[2]}"
        echo -e "${YELLOW}⚠ $PACKAGE: $COVERAGE% (no tests)${NC}"
    fi
done <<< "$COVERAGE_DATA"

# Generate HTML coverage report
echo ""
echo "Generating HTML coverage report..."
go tool cover -html=coverage/coverage.out -o coverage/coverage.html

# Summary
echo ""
echo "Coverage Summary:"
echo "-----------------"
if [ ${#FAILED_PACKAGES[@]} -eq 0 ]; then
    echo -e "${GREEN}All packages meet coverage thresholds!${NC}"
    echo "HTML report: coverage/coverage.html"
    exit 0
else
    echo -e "${RED}${#FAILED_PACKAGES[@]} packages failed to meet coverage thresholds:${NC}"
    for pkg in "${FAILED_PACKAGES[@]}"; do
        echo -e "${RED}  - $pkg${NC}"
    done
    echo ""
    echo "To improve coverage:"
    echo "1. Add unit tests for untested functions"
    echo "2. Add edge case tests"
    echo "3. Add integration tests where appropriate"
    echo ""
    echo "HTML report: coverage/coverage.html"
    exit 1
fi
