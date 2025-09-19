#!/bin/bash

set -euo pipefail

echo "🛡️ Running Quality Checks..."

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

print_status() {
    local status=$1
    local message=$2
    case $status in
        "PASS") echo -e "${GREEN}✅ PASS:${NC} $message" ;;
        "FAIL") echo -e "${RED}❌ FAIL:${NC} $message" ;;
    esac
}

# Check 1: ESLint
echo "Checking ESLint..."
if npm run lint > /dev/null 2>&1; then
    print_status "PASS" "No ESLint errors"
else
    print_status "FAIL" "ESLint errors found"
    exit 1
fi

# Check 2: TypeScript
echo "Checking TypeScript..."
if npm run typecheck > /dev/null 2>&1; then
    print_status "PASS" "TypeScript compilation successful"
else
    print_status "FAIL" "TypeScript compilation failed"
    exit 1
fi

# Check 3: Tests
echo "Running tests..."
if npm run test:unit > /dev/null 2>&1; then
    print_status "PASS" "All tests passing"
else
    print_status "FAIL" "Tests failed"
    exit 1
fi

echo ""
echo "🎉 All quality checks passed!"