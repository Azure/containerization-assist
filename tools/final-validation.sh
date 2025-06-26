#!/bin/bash

# Final Validation Script for MCP Reorganization
# Team D: Infrastructure & Quality

set -e

echo "🏁 Running Final Migration Validation"
echo "====================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_status() {
    echo -e "${GREEN}✅${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠️${NC} $1"
}

print_error() {
    echo -e "${RED}❌${NC} $1"
}

print_info() {
    echo -e "${BLUE}ℹ️${NC} $1"
}

# Track validation results
TOTAL_CHECKS=0
PASSED_CHECKS=0
FAILED_CHECKS=0
WARNING_CHECKS=0

run_check() {
    local name="$1"
    local command="$2"
    local critical="$3"  # true/false

    TOTAL_CHECKS=$((TOTAL_CHECKS + 1))
    echo
    echo "🔍 Running: $name"

    if eval "$command" >/dev/null 2>&1; then
        print_status "$name passed"
        PASSED_CHECKS=$((PASSED_CHECKS + 1))
        return 0
    else
        if [ "$critical" = "true" ]; then
            print_error "$name failed (Critical)"
            FAILED_CHECKS=$((FAILED_CHECKS + 1))
            return 1
        else
            print_warning "$name failed (Non-critical)"
            WARNING_CHECKS=$((WARNING_CHECKS + 1))
            return 0
        fi
    fi
}

echo "🔍 Running comprehensive validation checks..."

# 1. Basic Go validation
run_check "Go Build" "go build ./..." true
run_check "Go Vet" "go vet ./..." true
run_check "Go Test" "go test ./..." false
run_check "Module Tidy" "go mod tidy && git diff --exit-code go.mod go.sum" false

# 2. Team D quality checks
if [ -f "tools/validate-interfaces/main.go" ]; then
    run_check "Interface Validation" "go run tools/validate-interfaces/main.go" false
fi

if [ -f "tools/check-boundaries/main.go" ]; then
    run_check "Package Boundaries" "go run tools/check-boundaries/main.go" false
fi

if [ -f "tools/check-hygiene/main.go" ]; then
    run_check "Dependency Hygiene" "go run tools/check-hygiene/main.go" false
fi

# 3. File structure validation
echo
echo "📁 Validating file structure..."

# Check for new architecture compliance
if [ -f "pkg/mcp/interfaces.go" ]; then
    print_status "Unified interfaces file exists"
    PASSED_CHECKS=$((PASSED_CHECKS + 1))
else
    print_warning "Unified interfaces file not found (expected during migration)"
    WARNING_CHECKS=$((WARNING_CHECKS + 1))
fi
TOTAL_CHECKS=$((TOTAL_CHECKS + 1))

# Check for flattened structure
DEEP_NESTING=$(find pkg/mcp -type d -path "*/internal/*/*/*/*/*" 2>/dev/null | wc -l)
if [ "$DEEP_NESTING" -eq 0 ]; then
    print_status "No deep nesting detected (>4 levels)"
    PASSED_CHECKS=$((PASSED_CHECKS + 1))
else
    print_warning "Deep nesting still exists: $DEEP_NESTING locations"
    WARNING_CHECKS=$((WARNING_CHECKS + 1))
fi
TOTAL_CHECKS=$((TOTAL_CHECKS + 1))

# 4. Documentation validation
echo
echo "📚 Validating documentation..."

REQUIRED_DOCS=(
    "ARCHITECTURE_NEW.md"
    "INTERFACES.md"
    "AUTO_REGISTRATION.md"
    "MIGRATION_SUMMARY.md"
    "TEAM_D_PLAN.md"
)

for doc in "${REQUIRED_DOCS[@]}"; do
    if [ -f "$doc" ]; then
        print_status "Documentation exists: $doc"
        PASSED_CHECKS=$((PASSED_CHECKS + 1))
    else
        print_error "Missing documentation: $doc"
        FAILED_CHECKS=$((FAILED_CHECKS + 1))
    fi
    TOTAL_CHECKS=$((TOTAL_CHECKS + 1))
done

# 5. IDE configuration validation
echo
echo "🔧 Validating IDE configurations..."

if [ -d ".vscode" ] && [ -f ".vscode/settings.json" ]; then
    print_status "VS Code configuration present"
    PASSED_CHECKS=$((PASSED_CHECKS + 1))
else
    print_warning "VS Code configuration missing"
    WARNING_CHECKS=$((WARNING_CHECKS + 1))
fi
TOTAL_CHECKS=$((TOTAL_CHECKS + 1))

if [ -d ".idea" ] && [ -f ".idea/workspace.xml" ]; then
    print_status "IntelliJ/GoLand configuration present"
    PASSED_CHECKS=$((PASSED_CHECKS + 1))
else
    print_warning "IntelliJ/GoLand configuration missing"
    WARNING_CHECKS=$((WARNING_CHECKS + 1))
fi
TOTAL_CHECKS=$((TOTAL_CHECKS + 1))

# 6. Performance validation
echo
echo "📊 Validating performance..."

if [ -f "performance_baseline.json" ]; then
    print_status "Performance baseline exists"
    PASSED_CHECKS=$((PASSED_CHECKS + 1))

    # Check if we can measure current performance
    if [ -f "tools/measure-performance/main.go" ]; then
        print_info "Performance measurement tools available"
    fi
else
    print_warning "Performance baseline missing"
    WARNING_CHECKS=$((WARNING_CHECKS + 1))
fi
TOTAL_CHECKS=$((TOTAL_CHECKS + 1))

# 7. Tool validation
echo
echo "🛠️  Validating Team D tools..."

TEAM_D_TOOLS=(
    "tools/migrate/main.go"
    "tools/update-imports/main.go"
    "tools/validate-interfaces/main.go"
    "tools/check-boundaries/main.go"
    "tools/check-hygiene/main.go"
    "tools/measure-performance/main.go"
    "tools/build-enforcement/main.go"
    "tools/test-migration/main.go"
)

for tool in "${TEAM_D_TOOLS[@]}"; do
    if [ -f "$tool" ]; then
        print_status "Tool exists: $(basename $(dirname $tool))"
        PASSED_CHECKS=$((PASSED_CHECKS + 1))
    else
        print_error "Missing tool: $tool"
        FAILED_CHECKS=$((FAILED_CHECKS + 1))
    fi
    TOTAL_CHECKS=$((TOTAL_CHECKS + 1))
done

# 8. Makefile validation
echo
echo "🏗️  Validating Makefile targets..."

REQUIRED_TARGETS=(
    "validate-structure"
    "validate-interfaces"
    "check-hygiene"
    "enforce-quality"
    "migrate-all"
    "update-imports"
    "bench-performance"
)

for target in "${REQUIRED_TARGETS[@]}"; do
    if grep -q "^${target}:" Makefile 2>/dev/null; then
        print_status "Makefile target exists: $target"
        PASSED_CHECKS=$((PASSED_CHECKS + 1))
    else
        print_error "Missing Makefile target: $target"
        FAILED_CHECKS=$((FAILED_CHECKS + 1))
    fi
    TOTAL_CHECKS=$((TOTAL_CHECKS + 1))
done

# Final report
echo
echo "📊 Final Validation Report"
echo "=========================="
echo "Total Checks: $TOTAL_CHECKS"
echo "✅ Passed: $PASSED_CHECKS"
echo "❌ Failed: $FAILED_CHECKS"
echo "⚠️  Warnings: $WARNING_CHECKS"

# Calculate percentages
PASS_PERCENTAGE=$((PASSED_CHECKS * 100 / TOTAL_CHECKS))
echo "📈 Pass Rate: $PASS_PERCENTAGE%"

echo
if [ $FAILED_CHECKS -eq 0 ]; then
    if [ $WARNING_CHECKS -eq 0 ]; then
        print_status "🎉 All validations passed! Migration is complete and successful."
        echo
        print_info "Migration Summary:"
        echo "  • Team D infrastructure complete"
        echo "  • Quality gates operational"
        echo "  • Documentation comprehensive"
        echo "  • Development tooling ready"
        echo "  • Architecture validated"
        echo
        print_info "Next steps:"
        echo "  1. Teams A, B, C can use the infrastructure for their work"
        echo "  2. Run 'make help' to see available commands"
        echo "  3. Use 'tools/setup-dev-environment.sh' for new developers"
        echo "  4. Monitor performance with 'make bench-performance'"
        EXIT_CODE=0
    else
        print_warning "🎯 Validation completed with warnings ($WARNING_CHECKS warnings)"
        echo "   Most warnings are expected during migration - Teams A, B, C work in progress"
        EXIT_CODE=0
    fi
else
    print_error "💥 Validation failed with $FAILED_CHECKS critical errors"
    echo "   Fix the critical errors above before proceeding"
    EXIT_CODE=1
fi

echo
echo "🏁 Final validation complete"
exit $EXIT_CODE
