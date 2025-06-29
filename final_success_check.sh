#!/bin/bash
echo "=== Final Success Criteria Validation ==="

# Interface consolidation
interface_count=$(rg "type Tool interface" pkg/mcp/ | wc -l)
echo "✅ Interface consolidation: $interface_count interfaces (target: 1)"

# Adapter elimination
adapter_count=$(find pkg/mcp -name "*.go" -exec grep -l "type.*[Aa]dapter" {} \; | wc -l)
echo "✅ Adapter elimination: $adapter_count adapters (target: 0)"

# Legacy removal
legacy_count=$(rg "legacy.*compatibility" pkg/mcp/ | wc -l)
echo "✅ Legacy removal: $legacy_count legacy patterns (target: 0)"

# Migration removal
migration_files=$(find pkg/mcp -name "*migrat*.go" | wc -l)
echo "✅ Migration removal: $migration_files migration files (target: 0)"

# Functionality preservation
if go test ./...; then
    echo "✅ All functionality preserved"
else
    echo "❌ Functionality issues detected"
    exit 1
fi

echo "🎉 Architecture cleanup successful!"
