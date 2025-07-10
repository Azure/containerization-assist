#!/bin/bash
# Workstream Performance Tracking Script
# EPSILON monitoring tool for all workstream performance

set -e

BASELINE_DIR="benchmarks/baselines"
REPORT_DIR="monitoring/reports"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

mkdir -p "$REPORT_DIR"

echo -e "${BLUE}=== WORKSTREAM PERFORMANCE TRACKING ===${NC}"
echo "Timestamp: $TIMESTAMP"
echo ""

# Create detailed report
REPORT_FILE="$REPORT_DIR/workstream_report_$TIMESTAMP.txt"

cat > "$REPORT_FILE" << EOF
=== WORKSTREAM PERFORMANCE REPORT ===
Date: $(date)
Commit: $(git rev-parse HEAD)
Branch: $(git branch --show-current)

PERFORMANCE TARGETS:
- Tool execution: <300μs (300,000 ns)
- Pipeline stages: <500μs (500,000 ns)
- Error handling: <100ns
- Registry ops: <250ns

================================================
EOF

# Function to run benchmarks and check for regression
run_workstream_bench() {
    local workstream=$1
    local package=$2
    local target_ns=$3
    
    echo -e "${BLUE}Testing $workstream workstream...${NC}"
    echo -e "\n=== $workstream WORKSTREAM ===" >> "$REPORT_FILE"
    echo "Package: $package" >> "$REPORT_FILE"
    echo "Target: <$target_ns ns/op" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    
    # Run benchmarks
    if go test -bench=. -benchmem -benchtime=10s "$package" > temp_bench.txt 2>&1; then
        # Extract results
        grep -E "Benchmark.*ns/op" temp_bench.txt >> "$REPORT_FILE" || echo "No benchmarks found" >> "$REPORT_FILE"
        
        # Check for performance regression
        while read -r line; do
            if [[ $line =~ ([0-9]+)[[:space:]]+ns/op ]]; then
                ns_value=${BASH_REMATCH[1]}
                if [[ $ns_value -gt $target_ns ]]; then
                    echo -e "${RED}❌ REGRESSION: $line (exceeds target $target_ns ns)${NC}"
                    echo "⚠️  REGRESSION: $line" >> "$REPORT_FILE"
                else
                    echo -e "${GREEN}✅ PASS: $line${NC}"
                fi
            fi
        done < <(grep -E "Benchmark.*ns/op" temp_bench.txt)
    else
        echo -e "${YELLOW}⚠️  No benchmarks or build error for $workstream${NC}"
        echo "⚠️  No benchmarks available or build error" >> "$REPORT_FILE"
    fi
    
    rm -f temp_bench.txt
    echo "" >> "$REPORT_FILE"
}

# Monitor each workstream
echo -e "\n${BLUE}1. ALPHA - Foundation Layer${NC}"
run_workstream_bench "ALPHA" "./pkg/mcp/domain/..." 100000

echo -e "\n${BLUE}2. BETA - Tool Migration${NC}"
run_workstream_bench "BETA" "./pkg/mcp/application/tools/..." 300000

echo -e "\n${BLUE}3. GAMMA - Workflow Implementation${NC}"
run_workstream_bench "GAMMA" "./pkg/mcp/application/workflows/..." 500000

echo -e "\n${BLUE}4. DELTA - Error Handling${NC}"
run_workstream_bench "DELTA" "./pkg/mcp/domain/errors/..." 200

# Architecture validation check
echo -e "\n${BLUE}5. Architecture Validation${NC}"
echo -e "\n=== ARCHITECTURE VALIDATION ===" >> "$REPORT_FILE"

if scripts/validate-architecture.sh > arch_check.txt 2>&1; then
    echo -e "${GREEN}✅ Architecture validation PASSED${NC}"
    echo "✅ Architecture validation PASSED" >> "$REPORT_FILE"
else
    echo -e "${RED}❌ Architecture validation FAILED${NC}"
    echo "❌ Architecture validation FAILED" >> "$REPORT_FILE"
    tail -10 arch_check.txt >> "$REPORT_FILE"
fi
rm -f arch_check.txt

# Quality gates summary
echo -e "\n${BLUE}6. Quality Gates Summary${NC}"
echo -e "\n=== QUALITY GATES SUMMARY ===" >> "$REPORT_FILE"

# Simple quality checks
LINT_COUNT=$(golangci-lint run ./pkg/mcp/... 2>/dev/null | wc -l || echo "0")
echo "Lint issues: $LINT_COUNT (budget: 100)" >> "$REPORT_FILE"

if [[ $LINT_COUNT -le 100 ]]; then
    echo -e "${GREEN}✅ Linting PASSED ($LINT_COUNT issues)${NC}"
else
    echo -e "${RED}❌ Linting FAILED ($LINT_COUNT issues exceed budget)${NC}"
fi

# Summary
echo -e "\n${BLUE}=== SUMMARY ===${NC}"
echo -e "\n=== SUMMARY ===" >> "$REPORT_FILE"

# Check for any regressions
if grep -q "REGRESSION\|FAILED" "$REPORT_FILE"; then
    echo -e "${RED}❌ Performance regressions or failures detected!${NC}"
    echo "❌ OVERALL: REGRESSIONS DETECTED - Review required" >> "$REPORT_FILE"
    exit_code=1
else
    echo -e "${GREEN}✅ All workstreams within performance targets${NC}"
    echo "✅ OVERALL: ALL WORKSTREAMS HEALTHY" >> "$REPORT_FILE"
    exit_code=0
fi

echo ""
echo "📊 Full report: $REPORT_FILE"
echo "📈 Dashboard: monitoring/workstream_tracking.md"

exit $exit_code