#!/bin/bash
set -e

echo "=== CONTAINER KIT MCP QUALITY GATES (ENFORCED) ==="

# Track overall status
overall_status=0

# Gate 1: Interface Count
echo "🔍 Checking interface count..."
if scripts/interface-counter pkg/mcp/ >/dev/null 2>&1; then
    echo "✅ PASS: Interface count within limit"
else
    echo "❌ FAIL: Interface count exceeds limit of 50"
    overall_status=1
fi

# Gate 2: File Size
echo "🔍 Checking file sizes..."
if scripts/check_file_size.sh >/dev/null 2>&1; then
    echo "✅ PASS: All files within 800 line limit"
else
    echo "❌ FAIL: Files exceed 800 line limit"
    overall_status=1
fi

# Gate 3: Function Complexity
echo "🔍 Checking function complexity..."
if scripts/complexity-checker pkg/mcp/ >/dev/null 2>&1; then
    echo "✅ PASS: All functions within complexity limit"
else
    echo "❌ FAIL: Functions exceed complexity limit of 20"
    overall_status=1
fi

# Gate 4: Import Depth
echo "🔍 Checking import depth..."
if scripts/check_import_depth.sh >/dev/null 2>&1; then
    echo "✅ PASS: All imports ≤3 levels deep"
else
    echo "❌ FAIL: Deep imports found (>3 levels)"
    overall_status=1
fi

# Gate 5: Test Coverage
echo "🔍 Checking test coverage..."
if scripts/coverage.sh >/dev/null 2>&1; then
    echo "✅ PASS: Test coverage meets minimum requirements"
else
    echo "❌ FAIL: Test coverage below minimum requirements"
    overall_status=1
fi

# Gate 6: Build Success
echo "🔍 Checking build..."
if make build >/dev/null 2>&1; then
    echo "✅ PASS: Build successful"
else
    echo "❌ FAIL: Build failed"
    overall_status=1
fi

# Gate 7: Linting
echo "🔍 Checking linting..."
if make lint >/dev/null 2>&1; then
    echo "✅ PASS: Linting clean"
else
    echo "❌ FAIL: Linting issues found"
    overall_status=1
fi

# Final status
if [ $overall_status -eq 0 ]; then
    echo "🎉 ALL QUALITY GATES PASSED"
else
    echo "❌ QUALITY GATES FAILED"
    exit 1
fi