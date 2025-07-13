#!/bin/bash

# Quality Metrics Tracking Script
# This script collects and reports various code quality metrics

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo "📊 Container Kit Quality Metrics Report"
echo "======================================"
echo ""

# Function to check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# 1. Lines of Code Metrics
echo "📏 Code Size Metrics:"
echo "--------------------"
TOTAL_GO_LOC=$(find pkg/mcp -name "*.go" -not -path "*/vendor/*" -not -name "*_test.go" | xargs wc -l | tail -1 | awk '{print $1}')
TEST_GO_LOC=$(find pkg/mcp -name "*_test.go" -not -path "*/vendor/*" | xargs wc -l | tail -1 | awk '{print $1}')
NUM_GO_FILES=$(find pkg/mcp -name "*.go" -not -path "*/vendor/*" | wc -l)

echo "  Total Go LOC (pkg/mcp): $TOTAL_GO_LOC"
echo "  Test Go LOC: $TEST_GO_LOC"
echo "  Number of Go files: $NUM_GO_FILES"
if [ "$NUM_GO_FILES" -eq 0 ]; then
    echo "  Average LOC per file: Cannot calculate (no Go files found)"
else
    echo "  Average LOC per file: $((TOTAL_GO_LOC / NUM_GO_FILES))"
fi
echo ""

# 2. Cyclomatic Complexity
echo "🔄 Cyclomatic Complexity:"
echo "------------------------"
if command_exists gocyclo; then
    COMPLEX_OVER_30=$(gocyclo -over 30 pkg/mcp 2>/dev/null | wc -l || echo "0")
    COMPLEX_OVER_20=$(gocyclo -over 20 pkg/mcp 2>/dev/null | wc -l || echo "0")
    COMPLEX_OVER_15=$(gocyclo -over 15 pkg/mcp 2>/dev/null | wc -l || echo "0")
    COMPLEX_OVER_10=$(gocyclo -over 10 pkg/mcp 2>/dev/null | wc -l || echo "0")
    
    echo "  Functions with complexity > 30: $COMPLEX_OVER_30"
    echo "  Functions with complexity > 20: $COMPLEX_OVER_20"
    echo "  Functions with complexity > 15: $COMPLEX_OVER_15"
    echo "  Functions with complexity > 10: $COMPLEX_OVER_10"
else
    echo "  ⚠️  gocyclo not installed"
fi
echo ""

# 3. Interface and Abstraction Metrics
echo "🔌 Interface & Abstraction Metrics:"
echo "-----------------------------------"
NUM_INTERFACES=$(grep -r "type.*interface" pkg/mcp --include="*.go" | wc -l)
NUM_ADAPTERS=$(find pkg/mcp -name "*adapter*.go" | wc -l)
NUM_WRAPPERS=$(find pkg/mcp -name "*wrapper*.go" | grep -v docker_operation | wc -l)
NUM_DOMAIN_INTERFACES=$(grep -r "type.*interface" pkg/mcp/domain --include="*.go" | wc -l || echo "0")
NUM_INFRA_INTERFACES=$(grep -r "type.*interface" pkg/mcp/infrastructure --include="*.go" | wc -l || echo "0")

echo "  Total interfaces: $NUM_INTERFACES"
echo "  Domain interfaces: $NUM_DOMAIN_INTERFACES"
echo "  Infrastructure interfaces: $NUM_INFRA_INTERFACES"
echo "  Adapter files: $NUM_ADAPTERS"
echo "  Wrapper files: $NUM_WRAPPERS"
echo ""

# 4. Package Structure
echo "📦 Package Structure:"
echo "--------------------"
echo "  Current structure (pkg/mcp):"
find pkg/mcp -maxdepth 1 -type d | grep -v "^pkg/mcp$" | sort | sed 's|pkg/mcp/|    - |'
NUM_TOP_DIRS=$(find pkg/mcp -maxdepth 1 -type d | grep -v "^pkg/mcp$" | wc -l)
echo "  Total top-level directories: $NUM_TOP_DIRS"
echo ""

# 5. Test Coverage (if available)
echo "🧪 Test Coverage:"
echo "----------------"
if [ -f coverage.out ]; then
    COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}')
    echo "  Overall coverage: $COVERAGE"
else
    echo "  No coverage data available (run tests first)"
fi
echo ""

# 6. Lint Issues
echo "🔍 Lint Issues:"
echo "---------------"
if command_exists golangci-lint; then
    LINT_OUTPUT=$(golangci-lint run pkg/mcp/... --timeout=5m 2>&1 || true)
    LINT_ISSUES=$(echo "$LINT_OUTPUT" | awk '/issues:/ {match($0, /[0-9]+/); print substr($0, RSTART, RLENGTH)}' | tail -1 || echo "0")
    echo "  Total lint issues: $LINT_ISSUES"
    
    # Count by severity if available
    ERRORS=$(echo "$LINT_OUTPUT" | grep -c "^[^:]*:[0-9]*:[0-9]*: error:" || echo "0")
    WARNINGS=$(echo "$LINT_OUTPUT" | grep -c "^[^:]*:[0-9]*:[0-9]*: warning:" || echo "0")
    echo "  Errors: $ERRORS"
    echo "  Warnings: $WARNINGS"
else
    echo "  ⚠️  golangci-lint not installed"
fi
echo ""

# 7. Dependency Analysis
echo "📚 Dependency Metrics:"
echo "---------------------"
NUM_IMPORTS=$(grep -r "^import" pkg/mcp --include="*.go" | wc -l)
NUM_EXTERNAL_DEPS=$(go list -m all | grep -v "github.com/Azure/container-kit" | wc -l)

echo "  Total import statements: $NUM_IMPORTS"
echo "  External dependencies: $NUM_EXTERNAL_DEPS"
echo ""

# 8. Architecture Score Summary
echo "🏗️  Architecture Quality Score:"
echo "------------------------------"
SCORE=100
SCORE=$((SCORE - NUM_ADAPTERS * 20))
SCORE=$((SCORE - NUM_WRAPPERS * 15))
LARGE_FILES=$(find pkg/mcp -name "*.go" -not -path "*/test*" -exec wc -l {} \; | awk '$1 > 800 {count++} END {print count+0}')
if [ "$LARGE_FILES" -gt 10 ]; then
    SCORE=$((SCORE - (LARGE_FILES - 10) * 5))
fi
if [ "$COMPLEX_OVER_15" -gt 50 ]; then
    SCORE=$((SCORE - (COMPLEX_OVER_15 - 50) * 2))
fi
if [ $SCORE -lt 0 ]; then SCORE=0; fi

echo "  Current score: $SCORE/100"
echo "  Deductions:"
echo "    - Adapters: -$((NUM_ADAPTERS * 20)) points"
echo "    - Wrappers: -$((NUM_WRAPPERS * 15)) points"
echo "    - Large files: -$(((LARGE_FILES > 10 ? (LARGE_FILES - 10) * 5 : 0))) points"
echo "    - Complex functions: -$(((COMPLEX_OVER_15 > 50 ? (COMPLEX_OVER_15 - 50) * 2 : 0))) points"
echo ""

# 9. Progress Toward Goals
echo "🎯 Progress Toward Architecture Goals:"
echo "-------------------------------------"
GOAL_4_DIRS=4
GOAL_0_ADAPTERS=0
GOAL_0_WRAPPERS=0
GOAL_1_INTERFACE=1

echo -n "  4-folder structure: "
if [ "$NUM_TOP_DIRS" -eq "$GOAL_4_DIRS" ]; then
    echo -e "${GREEN}✅ Achieved${NC}"
else
    echo -e "${RED}❌ Not achieved${NC} (current: $NUM_TOP_DIRS, goal: $GOAL_4_DIRS)"
fi

echo -n "  Zero adapters: "
if [ "$NUM_ADAPTERS" -eq "$GOAL_0_ADAPTERS" ]; then
    echo -e "${GREEN}✅ Achieved${NC}"
else
    echo -e "${RED}❌ Not achieved${NC} (current: $NUM_ADAPTERS, goal: $GOAL_0_ADAPTERS)"
fi

echo -n "  Zero wrappers: "
if [ "$NUM_WRAPPERS" -eq "$GOAL_0_WRAPPERS" ]; then
    echo -e "${GREEN}✅ Achieved${NC}"
else
    echo -e "${RED}❌ Not achieved${NC} (current: $NUM_WRAPPERS, goal: $GOAL_0_WRAPPERS)"
fi

echo -n "  Unified sampler interface: "
if [ "$NUM_DOMAIN_INTERFACES" -le 5 ]; then
    echo -e "${GREEN}✅ Good progress${NC} (domain interfaces: $NUM_DOMAIN_INTERFACES)"
else
    echo -e "${YELLOW}⚠️  In progress${NC} (domain interfaces: $NUM_DOMAIN_INTERFACES)"
fi

echo ""
echo "======================================"

# Output metrics in JSON format for CI integration
if [ "$1" == "--json" ]; then
    cat <<EOF > quality-metrics.json
{
  "timestamp": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "code_size": {
    "total_loc": $TOTAL_GO_LOC,
    "test_loc": $TEST_GO_LOC,
    "num_files": $NUM_GO_FILES,
    "avg_loc_per_file": $([ "$NUM_GO_FILES" -eq 0 ] && echo "null" || echo "$((TOTAL_GO_LOC / NUM_GO_FILES))")
  },
  "complexity": {
    "over_30": ${COMPLEX_OVER_30:-0},
    "over_20": ${COMPLEX_OVER_20:-0},
    "over_15": ${COMPLEX_OVER_15:-0},
    "over_10": ${COMPLEX_OVER_10:-0}
  },
  "abstractions": {
    "total_interfaces": $NUM_INTERFACES,
    "domain_interfaces": $NUM_DOMAIN_INTERFACES,
    "infra_interfaces": $NUM_INFRA_INTERFACES,
    "adapters": $NUM_ADAPTERS,
    "wrappers": $NUM_WRAPPERS
  },
  "structure": {
    "top_level_dirs": $NUM_TOP_DIRS,
    "large_files": $LARGE_FILES
  },
  "quality": {
    "lint_issues": ${LINT_ISSUES:-0},
    "architecture_score": $SCORE
  }
}
EOF
    echo ""
    echo "📄 Metrics saved to quality-metrics.json"
fi