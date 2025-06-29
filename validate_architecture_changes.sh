#!/bin/bash
echo "=== Architecture Change Validation ==="

# Interface validation
echo "🔍 Checking interface consolidation..."
interface_count=$(rg "type Tool interface" pkg/mcp/ | wc -l)
echo "Tool interfaces found: $interface_count (target: 1)"

# Adapter validation
echo "🔍 Checking adapter elimination..."
adapter_count=$(find pkg/mcp -name "*.go" -exec grep -l "type.*[Aa]dapter\|type.*[Ww]rapper" {} \; | wc -l)
echo "Adapter files found: $adapter_count (target: 0)"

# Legacy validation
echo "🔍 Checking legacy code removal..."
legacy_count=$(rg "legacy.*compatibility\|migration.*system" pkg/mcp/ | wc -l)
echo "Legacy patterns found: $legacy_count (target: 0)"

# Build validation
echo "🔍 Checking build..."
if go build -tags mcp ./pkg/mcp/...; then
    echo "✅ Build successful"
else
    echo "❌ Build failed"
    exit 1
fi

# Test validation
echo "🔍 Checking tests..."
if go test -short -tags mcp ./pkg/mcp/...; then
    echo "✅ Tests pass"
else
    echo "❌ Tests failing"
    exit 1
fi
