#!/bin/bash

# Quality metrics dashboard generator
echo "=== Generating Container Kit Quality Metrics Dashboard ==="

DASHBOARD_DIR="docs/quality"
REPORTS_DIR="test/reports"
METRICS_FILE="$DASHBOARD_DIR/metrics.json"
DASHBOARD_FILE="$DASHBOARD_DIR/dashboard.md"

mkdir -p "$DASHBOARD_DIR" "$REPORTS_DIR"

# Collect current metrics
echo "Collecting quality metrics..."

# Basic code metrics
TOTAL_GO_FILES=$(find pkg -name "*.go" | wc -l)
TOTAL_LINES=$(find pkg -name "*.go" -exec wc -l {} \; | awk '{sum += $1} END {print sum}')
TOTAL_TEST_FILES=$(find pkg -name "*_test.go" | wc -l)

# Package metrics
TOTAL_PACKAGES=$(find pkg -name "*.go" | xargs dirname | sort -u | wc -l)
PACKAGES_WITH_TESTS=$(find pkg -name "*_test.go" | xargs dirname | sort -u | wc -l)

# Function metrics
TOTAL_FUNCTIONS=$(grep -r "^func " pkg --include="*.go" | grep -v "_test.go" | wc -l)
EXPORTED_FUNCTIONS=$(grep -r "^func [A-Z]" pkg --include="*.go" | grep -v "_test.go" | wc -l)

# Interface metrics
TOTAL_INTERFACES=$(grep -r "type.*interface" pkg --include="*.go" | wc -l)

# Test metrics
UNIT_TESTS=$(grep -r "^func Test" pkg --include="*_test.go" | wc -l)
BENCHMARKS=$(grep -r "^func Benchmark" pkg --include="*_test.go" | wc -l)

# Coverage metrics (if available)
COVERAGE="N/A"
if [ -f "$REPORTS_DIR/coverage_summary.txt" ]; then
    COVERAGE=$(grep "Overall Coverage:" "$REPORTS_DIR/coverage_summary.txt" | awk '{print $3}' || echo "N/A")
fi

# Complexity metrics
LARGE_FILES=$(find pkg -name "*.go" -exec wc -l {} \; | awk '$1 > 800 {count++} END {print count+0}')
COMPLEX_FUNCTIONS=0

# Build status
BUILD_STATUS="Unknown"
if go build ./pkg/mcp/... >/dev/null 2>&1; then
    BUILD_STATUS="Passing"
else
    BUILD_STATUS="Failing"
fi

# Performance metrics (if benchmarks exist)
BENCHMARK_COUNT=0
PERFORMANCE_STATUS="N/A"
if [ -f "$REPORTS_DIR/benchmark_gate.txt" ]; then
    BENCHMARK_COUNT=$(grep -c "^Benchmark" "$REPORTS_DIR/benchmark_gate.txt" || echo "0")
    if grep -q "exceeds.*target" "$REPORTS_DIR/benchmark_gate.txt"; then
        PERFORMANCE_STATUS="Some benchmarks slow"
    else
        PERFORMANCE_STATUS="Within targets"
    fi
fi

# Security metrics
SECURITY_ISSUES=0
SECRET_PATTERNS=("password.*=" "token.*=" "key.*=" "secret.*=")
for pattern in "${SECRET_PATTERNS[@]}"; do
    matches=$(find pkg -name "*.go" -exec grep -iH "$pattern" {} \; | grep -v "_test.go" | wc -l)
    SECURITY_ISSUES=$((SECURITY_ISSUES + matches))
done

# Calculate derived metrics
if [ "$TOTAL_PACKAGES" -gt 0 ]; then
    PACKAGE_TEST_COVERAGE=$((PACKAGES_WITH_TESTS * 100 / TOTAL_PACKAGES))
else
    PACKAGE_TEST_COVERAGE=0
fi

if [ "$TOTAL_FUNCTIONS" -gt 0 ]; then
    EXPORTED_FUNCTION_RATIO=$((EXPORTED_FUNCTIONS * 100 / TOTAL_FUNCTIONS))
else
    EXPORTED_FUNCTION_RATIO=0
fi

# Generate JSON metrics file
cat > "$METRICS_FILE" << EOF
{
  "generated": "$(date -Iseconds)",
  "commit": "$(git rev-parse HEAD 2>/dev/null || echo 'unknown')",
  "branch": "$(git branch --show-current 2>/dev/null || echo 'unknown')",
  "codebase": {
    "total_go_files": $TOTAL_GO_FILES,
    "total_lines": $TOTAL_LINES,
    "total_packages": $TOTAL_PACKAGES,
    "total_functions": $TOTAL_FUNCTIONS,
    "exported_functions": $EXPORTED_FUNCTIONS,
    "exported_function_ratio": $EXPORTED_FUNCTION_RATIO,
    "total_interfaces": $TOTAL_INTERFACES,
    "large_files": $LARGE_FILES,
    "complex_functions": $COMPLEX_FUNCTIONS
  },
  "testing": {
    "test_files": $TOTAL_TEST_FILES,
    "packages_with_tests": $PACKAGES_WITH_TESTS,
    "package_test_coverage": $PACKAGE_TEST_COVERAGE,
    "unit_tests": $UNIT_TESTS,
    "benchmarks": $BENCHMARKS,
    "coverage": "$COVERAGE"
  },
  "quality": {
    "build_status": "$BUILD_STATUS",
    "performance_status": "$PERFORMANCE_STATUS",
    "security_issues": $SECURITY_ISSUES,
    "benchmark_count": $BENCHMARK_COUNT
  }
}
EOF

# Generate Markdown dashboard
cat > "$DASHBOARD_FILE" << EOF
# Container Kit Quality Metrics Dashboard

**Generated**: $(date)  
**Commit**: $(git rev-parse HEAD 2>/dev/null || echo 'unknown')  
**Branch**: $(git branch --show-current 2>/dev/null || echo 'unknown')

## 📊 Current Metrics

### Codebase Overview
| Metric | Value | Target | Status |
|--------|-------|--------|---------|
| Go Files | $TOTAL_GO_FILES | - | ℹ️ |
| Total Lines | $TOTAL_LINES | - | ℹ️ |
| Packages | $TOTAL_PACKAGES | - | ℹ️ |
| Functions | $TOTAL_FUNCTIONS | - | ℹ️ |
| Exported Functions | $EXPORTED_FUNCTIONS (${EXPORTED_FUNCTION_RATIO}%) | - | ℹ️ |
| Interfaces | $TOTAL_INTERFACES | ≤50 | $([ "$TOTAL_INTERFACES" -le 50 ] && echo "✅" || echo "⚠️") |
| Large Files (>800 lines) | $LARGE_FILES | 0 | $([ "$LARGE_FILES" -eq 0 ] && echo "✅" || echo "⚠️") |

### Testing Metrics
| Metric | Value | Target | Status |
|--------|-------|--------|---------|
| Test Files | $TOTAL_TEST_FILES | - | ℹ️ |
| Packages with Tests | $PACKAGES_WITH_TESTS/$TOTAL_PACKAGES (${PACKAGE_TEST_COVERAGE}%) | 80% | $([ "$PACKAGE_TEST_COVERAGE" -ge 80 ] && echo "✅" || echo "⚠️") |
| Unit Tests | $UNIT_TESTS | - | ℹ️ |
| Benchmarks | $BENCHMARKS | - | ℹ️ |
| Code Coverage | $COVERAGE | ≥55% | $(echo "$COVERAGE" | grep -q "N/A" && echo "❓" || echo "⚠️") |

### Quality Gates
| Gate | Status | Details |
|------|--------|---------|
| Build | $([ "$BUILD_STATUS" = "Passing" ] && echo "✅ Passing" || echo "❌ Failing") | $BUILD_STATUS |
| Performance | $([ "$PERFORMANCE_STATUS" = "Within targets" ] && echo "✅ Good" || echo "⚠️ Review") | $PERFORMANCE_STATUS |
| Security | $([ "$SECURITY_ISSUES" -eq 0 ] && echo "✅ Clean" || echo "⚠️ Issues") | $SECURITY_ISSUES potential issues |
| Architecture | ⚠️ Monitoring | Refactoring in progress |

## 📈 Trends

### Quality Improvement Areas
1. **Test Coverage**: Currently $COVERAGE, target 55%
2. **Package Testing**: ${PACKAGE_TEST_COVERAGE}% packages have tests, target 80%
3. **File Size**: $LARGE_FILES large files, target 0
4. **Security**: $SECURITY_ISSUES potential issues, target 0

### Achievements
- ✅ $UNIT_TESTS unit tests implemented
- ✅ $BENCHMARKS performance benchmarks
- ✅ Quality gates infrastructure established
- ✅ Automated testing pipeline

## 🎯 Quality Targets

### Short Term (1-2 weeks)
- [ ] Increase test coverage to 25%
- [ ] Fix build issues in all packages
- [ ] Reduce large files to <5
- [ ] Add 50 more unit tests

### Medium Term (1 month)
- [ ] Achieve 55% code coverage
- [ ] 80% of packages have tests
- [ ] Zero large files (>800 lines)
- [ ] Comprehensive integration tests

### Long Term (3 months)
- [ ] 80% code coverage for new code
- [ ] Performance benchmarks for all critical paths
- [ ] Complete security audit
- [ ] Full CI/CD automation

## 🔧 Tools and Infrastructure

### Quality Gates
- ✅ Automated quality gates in CI/CD
- ✅ Pre-commit hooks for local validation
- ✅ Performance regression detection
- ✅ Coverage tracking and reporting

### Scripts and Tools
- \`scripts/quality/quality_gates.sh\` - Comprehensive quality validation
- \`scripts/quality/coverage_tracker.sh\` - Coverage analysis
- \`scripts/quality/pre_commit_hook.sh\` - Local pre-commit validation
- \`scripts/quality/run_test_suite.sh\` - Test execution and reporting

### Reports
- [Coverage Report](../test/reports/coverage.html)
- [Quality Dashboard](dashboard.md)
- [Test Summary](../test/reports/test_summary.md)

## 📋 Recent Changes

$(if [ -f "$REPORTS_DIR/quality_dashboard.md" ]; then
    echo "Last quality gate run: $(stat -f %Sm "$REPORTS_DIR/quality_dashboard.md" 2>/dev/null || stat -c %y "$REPORTS_DIR/quality_dashboard.md" 2>/dev/null | cut -d' ' -f1-2)"
else
    echo "No recent quality gate runs found"
fi)

## 🚀 Getting Started

### Running Quality Checks Locally
\`\`\`bash
# Full quality gate validation
scripts/quality/quality_gates.sh

# Quick pre-commit check
scripts/quality/pre_commit_hook.sh

# Coverage analysis
scripts/quality/coverage_tracker.sh

# Test suite with coverage
scripts/quality/run_test_suite.sh
\`\`\`

### Adding Tests
\`\`\`bash
# Generate tests for a package
scripts/quality/generate_tests.sh pkg/mcp/domain/errors unit

# Run tests for specific package
go test ./pkg/mcp/domain/errors -v
\`\`\`

### Monitoring Performance
\`\`\`bash
# Run benchmarks
go test -bench=. ./pkg/mcp/...

# Track performance
scripts/performance/track_benchmarks.sh
\`\`\`

---

*Dashboard generated automatically by Container Kit Quality Infrastructure*  
*For questions or improvements, see [Quality Standards](../QUALITY_STANDARDS.md)*
EOF

# Generate trend data (if historical data exists)
TREND_FILE="$DASHBOARD_DIR/trends.json"
if [ -f "$TREND_FILE" ]; then
    # Append to existing trends
    CURRENT_DATE=$(date -Iseconds)
    TEMP_FILE=$(mktemp)
    
    # Read existing trends and add current data
    jq --arg date "$CURRENT_DATE" \
       --arg coverage "$COVERAGE" \
       --argjson tests "$UNIT_TESTS" \
       --argjson packages "$TOTAL_PACKAGES" \
       '. + [{"date": $date, "coverage": $coverage, "tests": $tests, "packages": $packages}]' \
       "$TREND_FILE" > "$TEMP_FILE" 2>/dev/null || echo "[]" > "$TEMP_FILE"
    
    mv "$TEMP_FILE" "$TREND_FILE"
else
    # Create initial trend data
    cat > "$TREND_FILE" << EOF
[
  {
    "date": "$(date -Iseconds)",
    "coverage": "$COVERAGE",
    "tests": $UNIT_TESTS,
    "packages": $TOTAL_PACKAGES,
    "interfaces": $TOTAL_INTERFACES
  }
]
EOF
fi

echo "✅ Quality metrics dashboard generated:"
echo "   📊 Dashboard: $DASHBOARD_FILE"
echo "   📈 Metrics: $METRICS_FILE"
echo "   📋 Trends: $TREND_FILE"
echo ""
echo "View dashboard: open $DASHBOARD_FILE"