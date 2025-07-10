#!/bin/bash

# Fix all imports using goimports after ADR-001 migration

set -e

echo "🔧 Fixing all imports using goimports..."
echo ""

# Check if goimports is installed
if ! command -v goimports &> /dev/null; then
    echo "❌ goimports not found. Installing..."
    go install golang.org/x/tools/cmd/goimports@latest
    export PATH=$PATH:$(go env GOPATH)/bin
fi

echo "📦 Running goimports on entire codebase..."
echo "This will:"
echo "  - Add missing imports"
echo "  - Remove unused imports"
echo "  - Format import groups properly"
echo ""

# First, let's update go.mod to ensure all dependencies are known
echo "1️⃣ Updating go.mod..."
go mod tidy || echo "⚠️  Some module issues, continuing..."

echo ""
echo "2️⃣ Running goimports on all packages..."

# Run goimports on each layer separately for better visibility
echo ""
echo "=== DOMAIN LAYER ==="
echo "Processing pkg/mcp/domain/..."
find pkg/mcp/domain -name "*.go" -type f | while read -r file; do
    echo -n "."
    goimports -w "$file"
done
echo " ✓"

echo ""
echo "=== APPLICATION LAYER ==="
echo "Processing pkg/mcp/application/..."
find pkg/mcp/application -name "*.go" -type f | while read -r file; do
    echo -n "."
    goimports -w "$file"
done
echo " ✓"

echo ""
echo "=== INFRASTRUCTURE LAYER ==="
echo "Processing pkg/mcp/infra/..."
find pkg/mcp/infra -name "*.go" -type f | while read -r file; do
    echo -n "."
    goimports -w "$file"
done
echo " ✓"

echo ""
echo "=== OTHER PACKAGES ==="
echo "Processing remaining packages..."
find pkg/ -name "*.go" -type f ! -path "*/vendor/*" ! -path "*/pkg/mcp/*" | while read -r file; do
    echo -n "."
    goimports -w "$file"
done
echo " ✓"

echo ""
echo "3️⃣ Running go mod tidy again..."
go mod tidy

echo ""
echo "4️⃣ Testing compilation..."
echo ""

# Test compilation and capture output
if go build ./pkg/mcp/... 2>&1 | tee /tmp/build_output.txt; then
    echo ""
    echo "✅ Compilation successful!"
else
    echo ""
    echo "⚠️  Some compilation errors remain. Checking for common issues..."
    
    # Check for specific error patterns
    if grep -q "package .* is not in GOROOT" /tmp/build_output.txt; then
        echo ""
        echo "📝 Found package resolution issues. Running go get to fetch dependencies..."
        grep "package .* is not in GOROOT" /tmp/build_output.txt | awk '{print $2}' | sort -u | while read -r pkg; do
            echo "  Fetching: $pkg"
            go get "$pkg" || true
        done
    fi
    
    if grep -q "undefined:" /tmp/build_output.txt; then
        echo ""
        echo "📝 Found undefined symbols. This might indicate:"
        echo "  - Circular dependencies"
        echo "  - Missing type definitions"
        echo "  - Incorrect package names"
    fi
fi

echo ""
echo "5️⃣ Checking for any remaining import issues..."

# Quick check for any obvious import problems
IMPORT_ERRORS=$(go list -e -json ./pkg/mcp/... 2>&1 | grep -E "no Go files|can't load package" | wc -l)
if [ "$IMPORT_ERRORS" -gt 0 ]; then
    echo "⚠️  Found $IMPORT_ERRORS packages with import issues"
    echo "Run 'go list -e ./pkg/mcp/...' for details"
else
    echo "✅ No obvious import issues found"
fi

echo ""
echo "=== GOIMPORTS COMPLETE ==="
echo ""
echo "Summary of actions taken:"
echo "✅ Ran goimports on all Go files"
echo "✅ Fixed import ordering and grouping"
echo "✅ Added missing imports where possible"
echo "✅ Removed unused imports"
echo ""
echo "Next steps:"
echo "1. Review any remaining compilation errors"
echo "2. Check for circular dependencies with: go list -f '{{.ImportPath}} -> {{join .Imports \" \"}}' ./pkg/mcp/... | grep 'pkg/mcp'"
echo "3. Run tests: go test ./pkg/mcp/..."
echo ""

# Clean up
rm -f /tmp/build_output.txt