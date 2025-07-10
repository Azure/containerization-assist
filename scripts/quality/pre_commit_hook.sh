#!/bin/bash

# Pre-commit hook for Container Kit quality enforcement
# This script runs essential quality checks before allowing commits

set -e

echo "🔍 Container Kit Pre-Commit Quality Checks"
echo "==========================================="

# Configuration
SKIP_TESTS="${SKIP_TESTS:-false}"
QUICK_MODE="${QUICK_MODE:-false}"
STAGED_FILES=$(git diff --cached --name-only --diff-filter=ACM | grep '\.go$' || true)

if [ -z "$STAGED_FILES" ]; then
    echo "ℹ️  No Go files staged for commit, skipping quality checks"
    exit 0
fi

echo "📝 Checking $(echo "$STAGED_FILES" | wc -l) staged Go files..."

# Pre-commit check results
CHECKS_PASSED=0
CHECKS_FAILED=0

# Helper function to record check results
check_result() {
    local check_name="$1"
    local result="$2"

    if [ "$result" -eq 0 ]; then
        echo "✅ $check_name: PASSED"
        CHECKS_PASSED=$((CHECKS_PASSED + 1))
    else
        echo "❌ $check_name: FAILED"
        CHECKS_FAILED=$((CHECKS_FAILED + 1))
    fi
}

# Check 1: Code Formatting
echo ""
echo "1️⃣ Checking code formatting..."
UNFORMATTED_FILES=""
for file in $STAGED_FILES; do
    if [ -f "$file" ]; then
        if ! gofmt -l "$file" | grep -q .; then
            continue  # File is formatted
        else
            UNFORMATTED_FILES="$UNFORMATTED_FILES $file"
        fi
    fi
done

if [ -z "$UNFORMATTED_FILES" ]; then
    check_result "Code Formatting" 0
else
    echo "   Unformatted files: $UNFORMATTED_FILES"
    echo "   Run: gofmt -w $UNFORMATTED_FILES"
    check_result "Code Formatting" 1
fi

# Check 2: Syntax and Build Verification (for staged files)
echo ""
echo "2️⃣ Checking syntax and build..."
BUILD_ISSUES=0
for file in $STAGED_FILES; do
    if [ -f "$file" ]; then
        # Quick syntax check
        if ! go fmt "$file" >/dev/null 2>&1; then
            echo "   Syntax error in: $file"
            BUILD_ISSUES=$((BUILD_ISSUES + 1))
        fi
    fi
done

# Try to build affected packages
if [ "$BUILD_ISSUES" -eq 0 ]; then
    AFFECTED_PACKAGES=$(for file in $STAGED_FILES; do dirname "$file"; done | sort -u | grep "^pkg/" | head -5)

    for package in $AFFECTED_PACKAGES; do
        if [ -d "$package" ]; then
            if ! go build "$package" >/dev/null 2>&1; then
                echo "   Build error in package: $package"
                BUILD_ISSUES=$((BUILD_ISSUES + 1))
            fi
        fi
    done
fi

check_result "Syntax/Build" $BUILD_ISSUES

# Check 3: Basic Linting
echo ""
echo "3️⃣ Running basic lint checks..."
LINT_ISSUES=0

# Check for common issues in staged files
for file in $STAGED_FILES; do
    if [ -f "$file" ]; then
        # Check for TODO/FIXME without context
        if grep -H "TODO\|FIXME" "$file" | grep -v "TODO:" | grep -v "FIXME:" >/dev/null 2>&1; then
            echo "   TODO/FIXME without context in: $file"
            LINT_ISSUES=$((LINT_ISSUES + 1))
        fi

        # Check for fmt.Print usage (should use structured logging)
        if grep -H "fmt\.Print" "$file" >/dev/null 2>&1; then
            echo "   fmt.Print usage found in: $file (use structured logging)"
            LINT_ISSUES=$((LINT_ISSUES + 1))
        fi

        # Check for panic usage
        if grep -H "panic(" "$file" >/dev/null 2>&1; then
            echo "   panic() usage found in: $file"
            LINT_ISSUES=$((LINT_ISSUES + 1))
        fi
    fi
done

check_result "Basic Linting" $LINT_ISSUES

# Check 4: Go Vet (for affected packages)
echo ""
echo "4️⃣ Running go vet..."
VET_ISSUES=0

if [ "$QUICK_MODE" != "true" ]; then
    AFFECTED_PACKAGES=$(for file in $STAGED_FILES; do dirname "$file"; done | sort -u | grep "^pkg/" | head -3)

    for package in $AFFECTED_PACKAGES; do
        if [ -d "$package" ]; then
            if ! go vet "$package" >/dev/null 2>&1; then
                echo "   go vet issues in: $package"
                VET_ISSUES=$((VET_ISSUES + 1))
            fi
        fi
    done
else
    echo "   Skipped (quick mode)"
fi

check_result "Go Vet" $VET_ISSUES

# Check 5: Test Validation (if tests exist)
echo ""
echo "5️⃣ Checking test validity..."
TEST_ISSUES=0

if [ "$SKIP_TESTS" != "true" ] && [ "$QUICK_MODE" != "true" ]; then
    # Find test files related to staged files
    TEST_FILES=""
    for file in $STAGED_FILES; do
        test_file="${file%%.go}_test.go"
        if [ -f "$test_file" ]; then
            TEST_FILES="$TEST_FILES $test_file"
        fi
    done

    if [ -n "$TEST_FILES" ]; then
        echo "   Found related test files, validating..."
        for test_file in $TEST_FILES; do
            package_dir=$(dirname "$test_file")
            if ! go test -run=^$ "$package_dir" >/dev/null 2>&1; then
                echo "   Test compilation issues in: $test_file"
                TEST_ISSUES=$((TEST_ISSUES + 1))
            fi
        done
    else
        echo "   No related test files found"
    fi
else
    echo "   Skipped (tests disabled or quick mode)"
fi

check_result "Test Validation" $TEST_ISSUES

# Check 6: File Size Check
echo ""
echo "6️⃣ Checking file sizes..."
LARGE_FILES=0

for file in $STAGED_FILES; do
    if [ -f "$file" ]; then
        lines=$(wc -l < "$file")
        if [ "$lines" -gt 800 ]; then
            echo "   Large file (${lines} lines): $file"
            LARGE_FILES=$((LARGE_FILES + 1))
        fi
    fi
done

if [ "$LARGE_FILES" -gt 0 ]; then
    echo "   Consider breaking down large files"
fi

check_result "File Size" $LARGE_FILES

# Check 7: Import Organization
echo ""
echo "7️⃣ Checking import organization..."
IMPORT_ISSUES=0

for file in $STAGED_FILES; do
    if [ -f "$file" ]; then
        # Check if goimports would change the file
        if ! goimports -l "$file" | grep -q .; then
            continue  # File has properly organized imports
        else
            echo "   Import organization needed: $file"
            IMPORT_ISSUES=$((IMPORT_ISSUES + 1))
        fi
    fi
done

if [ "$IMPORT_ISSUES" -gt 0 ]; then
    echo "   Run: goimports -w [files]"
fi

check_result "Import Organization" $IMPORT_ISSUES

# Summary
echo ""
echo "📊 PRE-COMMIT SUMMARY"
echo "===================="
echo "✅ Checks passed: $CHECKS_PASSED"
echo "❌ Checks failed: $CHECKS_FAILED"

# Additional helpful information
if [ "$CHECKS_FAILED" -gt 0 ]; then
    echo ""
    echo "🔧 QUICK FIXES:"
    echo "Format code:     gofmt -w $STAGED_FILES"
    echo "Organize imports: goimports -w $STAGED_FILES"
    echo "Run basic build: go build ./pkg/mcp/..."
    echo ""
    echo "💡 TIPS:"
    echo "- Use SKIP_TESTS=true to skip test validation"
    echo "- Use QUICK_MODE=true for faster checks"
    echo "- Run 'scripts/quality/quality_gates.sh' for full validation"
    echo ""
fi

# Determine exit code
if [ "$CHECKS_FAILED" -eq 0 ]; then
    echo "🎉 All pre-commit checks passed!"
    exit 0
else
    echo "💥 $CHECKS_FAILED checks failed. Please fix before committing."
    echo ""
    echo "To bypass these checks (not recommended):"
    echo "git commit --no-verify"
    exit 1
fi
