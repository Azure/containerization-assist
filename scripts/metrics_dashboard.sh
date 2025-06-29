#!/bin/bash
while true; do
    clear
    echo "=== Architecture Cleanup Metrics Dashboard ==="
    echo "$(date)"
    echo ""

    # Interface consolidation
    interfaces=$(rg "type Tool interface" pkg/mcp/ | wc -l)
    echo "🔧 Interfaces: $interfaces (target: 1)"

    # Adapter elimination
    adapters=$(find pkg/mcp -name "*.go" -exec grep -l "type.*[Aa]dapter" {} \; | wc -l)
    echo "🔧 Adapters: $adapters (target: 0)"

    # Legacy removal
    legacy=$(rg "legacy.*compatibility" pkg/mcp/ | wc -l)
    echo "🔧 Legacy patterns: $legacy (target: 0)"

    # Build status
    if go build -tags mcp ./pkg/mcp/... >/dev/null 2>&1; then
        echo "✅ Build: PASSING"
    else
        echo "❌ Build: FAILING"
    fi

    # Test status
    if make test-mcp >/dev/null 2>&1; then
        echo "✅ Tests: PASSING"
    else
        echo "❌ Tests: FAILING"
    fi

    sleep 10
done
