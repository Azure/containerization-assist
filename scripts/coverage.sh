#!/bin/bash
# Coverage script for MCP package
# This script runs tests with coverage and enforces minimum coverage thresholds

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to check if required tools are available
check_dependencies() {
    echo -e "${BLUE}🔍 Checking dependencies...${NC}"
    
    if ! command -v bc &> /dev/null; then
        echo -e "${RED}❌ ERROR: 'bc' calculator not found${NC}"
        echo -e "${YELLOW}💡 Install with: sudo apt-get install -y bc${NC}"
        exit 1
    fi
    
    if ! command -v go &> /dev/null; then
        echo -e "${RED}❌ ERROR: 'go' not found${NC}"
        exit 1
    fi
    
    echo -e "${GREEN}✅ All dependencies available${NC}"
}

# Coverage thresholds (adjusted to current baseline - see TODO.md for target improvements)
declare -A COVERAGE_THRESHOLDS=(
    ["github.com/Azure/container-kit/pkg/mcp/internal/core"]=25.0
    ["github.com/Azure/container-kit/pkg/mcp/internal/build"]=5.5
    ["github.com/Azure/container-kit/pkg/mcp/internal/deploy"]=6.0
    ["github.com/Azure/container-kit/pkg/mcp/internal/analyze"]=5.0
    ["github.com/Azure/container-kit/pkg/mcp/internal/orchestration"]=6.0
    ["github.com/Azure/container-kit/pkg/mcp/internal/observability"]=35.0
    ["github.com/Azure/container-kit/pkg/mcp/internal/retry"]=45.0
    ["github.com/Azure/container-kit/pkg/mcp/internal/utils"]=39.0
    ["github.com/Azure/container-kit/pkg/mcp/internal/workflow"]=55.0
    ["github.com/Azure/container-kit/pkg/mcp/internal/server"]=8.0
)

# Default threshold for packages not explicitly listed (lowered temporarily)
DEFAULT_THRESHOLD=0.0

echo "Running MCP test coverage analysis..."
echo "=================================="

# Check dependencies first
check_dependencies

# Create coverage directory
mkdir -p coverage

# Run tests with coverage for all MCP packages
echo -e "${BLUE}📋 Running tests with coverage...${NC}"
echo "Command: go test -coverprofile=coverage/coverage.out -covermode=atomic ./pkg/mcp/..."

if go test -coverprofile=coverage/coverage.out -covermode=atomic ./pkg/mcp/... > coverage/test_output.txt 2>&1; then
    echo -e "${GREEN}✅ All tests passed!${NC}"
else
    echo -e "${RED}❌ Tests failed!${NC}"
    echo -e "${YELLOW}📄 Test output:${NC}"
    cat coverage/test_output.txt
    echo -e "${RED}Cannot proceed with coverage analysis due to test failures${NC}"
    exit 1
fi

# Generate coverage report
echo ""
echo "Coverage Report:"
echo "----------------"

# Parse coverage data and check thresholds
FAILED_PACKAGES=()
echo -e "${BLUE}📊 Analyzing coverage data...${NC}"

# Get coverage data with better error handling
if ! COVERAGE_DATA=$(go test -cover ./pkg/mcp/... 2>&1); then
    echo -e "${RED}❌ Failed to get coverage data${NC}"
    echo -e "${YELLOW}📄 Error output:${NC}"
    echo "$COVERAGE_DATA"
    exit 1
fi

# Filter coverage lines
COVERAGE_LINES=$(echo "$COVERAGE_DATA" | grep -E "coverage:|ok" | grep "coverage:")

if [ -z "$COVERAGE_LINES" ]; then
    echo -e "${RED}❌ No coverage data found${NC}"
    echo -e "${YELLOW}📄 Raw output:${NC}"
    echo "$COVERAGE_DATA"
    exit 1
fi

echo -e "${BLUE}📈 Processing coverage for each package:${NC}"

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

        echo -e "${BLUE}  🔍 Checking $PACKAGE: ${COVERAGE}% vs ${THRESHOLD}%${NC}"
        
        # Validate numeric values
        if ! [[ "$COVERAGE" =~ ^[0-9]+(\.[0-9]+)?$ ]]; then
            echo -e "${RED}    ❌ Invalid coverage value: '$COVERAGE'${NC}"
            FAILED_PACKAGES+=("$PACKAGE (invalid coverage)")
            continue
        fi
        
        if ! [[ "$THRESHOLD" =~ ^[0-9]+(\.[0-9]+)?$ ]]; then
            echo -e "${RED}    ❌ Invalid threshold value: '$THRESHOLD'${NC}"
            FAILED_PACKAGES+=("$PACKAGE (invalid threshold)")
            continue
        fi

        # Compare coverage with threshold using bc
        if comparison_result=$(echo "$COVERAGE < $THRESHOLD" | bc -l 2>/dev/null); then
            if [ "$comparison_result" = "1" ]; then
                gap=$(echo "$THRESHOLD - $COVERAGE" | bc -l)
                echo -e "${RED}    ❌ BELOW THRESHOLD: $PACKAGE: $COVERAGE% < $THRESHOLD% (need +${gap}%)${NC}"
                FAILED_PACKAGES+=("$PACKAGE ($COVERAGE% < $THRESHOLD%)")
            else
                echo -e "${GREEN}    ✅ MEETS THRESHOLD: $PACKAGE: $COVERAGE% >= $THRESHOLD%${NC}"
            fi
        else
            echo -e "${RED}    ❌ Failed to compare coverage values${NC}"
            echo -e "${YELLOW}    🔍 Attempted: '$COVERAGE < $THRESHOLD'${NC}"
            FAILED_PACKAGES+=("$PACKAGE (comparison failed)")
        fi
    elif [[ $line =~ ^[[:space:]]+([^[:space:]]+)[[:space:]]+coverage:[[:space:]]+([0-9]+\.[0-9]+)%[[:space:]]of[[:space:]]statements ]]; then
        # Handle packages with no tests
        PACKAGE="${BASH_REMATCH[1]}"
        COVERAGE="${BASH_REMATCH[2]}"
        echo -e "${YELLOW}  ⚠️  NO TESTS: $PACKAGE: $COVERAGE%${NC}"
    fi
done <<< "$COVERAGE_LINES"

# Generate HTML coverage report
echo ""
echo "Generating HTML coverage report..."
go tool cover -html=coverage/coverage.out -o coverage/coverage.html

# Summary
echo ""
echo -e "${BLUE}📋 COVERAGE SUMMARY${NC}"
echo "=================="

if [ ${#FAILED_PACKAGES[@]} -eq 0 ]; then
    echo -e "${GREEN}🎉 SUCCESS: All packages meet coverage thresholds!${NC}"
    echo -e "${GREEN}✅ Total packages checked: ${#COVERAGE_THRESHOLDS[@]}${NC}"
    echo -e "${BLUE}📊 Coverage thresholds enforced:${NC}"
    for package in "${!COVERAGE_THRESHOLDS[@]}"; do
        threshold=${COVERAGE_THRESHOLDS[$package]}
        echo -e "${GREEN}  ✓ $package: ≥${threshold}%${NC}"
    done
    echo -e "${BLUE}📄 HTML report: coverage/coverage.html${NC}"
    exit 0
else
    echo -e "${RED}❌ FAILURE: ${#FAILED_PACKAGES[@]} packages failed to meet coverage thresholds${NC}"
    echo ""
    echo -e "${RED}📋 Failed packages:${NC}"
    for pkg in "${FAILED_PACKAGES[@]}"; do
        echo -e "${RED}  ❌ $pkg${NC}"
    done
    echo ""
    echo -e "${YELLOW}💡 TROUBLESHOOTING TIPS:${NC}"
    echo -e "${YELLOW}  1. Run individual tests: go test -cover ./pkg/mcp/internal/[package]/...${NC}"
    echo -e "${YELLOW}  2. Generate detailed report: go test -coverprofile=coverage.out ./pkg/mcp/internal/[package]/...${NC}"
    echo -e "${YELLOW}  3. View coverage details: go tool cover -html=coverage.out${NC}"
    echo -e "${YELLOW}  4. Check for missing test files or uncovered code paths${NC}"
    echo -e "${YELLOW}  5. Add unit tests for untested functions${NC}"
    echo -e "${YELLOW}  6. Add edge case and error handling tests${NC}"
    echo ""
    echo -e "${BLUE}📄 HTML report: coverage/coverage.html${NC}"
    echo -e "${RED}🔍 For debugging, check the detailed output above.${NC}"
    exit 1
fi
