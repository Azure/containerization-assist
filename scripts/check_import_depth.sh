#!/bin/bash
set -e

echo "=== IMPORT DEPTH CHECKER ==="

max_depth=3
violations=0

echo "Checking for imports exceeding $max_depth levels..."

# Find all Go files and check their imports
find pkg/mcp -name "*.go" | while read file; do
    # Extract imports from the file
    imports=$(grep -E '^\s*"github\.com/Azure/container-kit/pkg/mcp/' "$file" 2>/dev/null || true)

    if [ -n "$imports" ]; then
        echo "$imports" | while read -r import; do
            # Clean up the import string
            import_path=$(echo "$import" | sed 's/.*"\(.*\)".*/\1/')

            # Count depth after pkg/mcp/
            if [[ "$import_path" =~ pkg/mcp/ ]]; then
                # Extract the part after pkg/mcp/
                after_mcp=$(echo "$import_path" | sed 's|.*pkg/mcp/||')

                # Count slashes (depth)
                depth=$(echo "$after_mcp" | awk -F/ '{print NF}')

                if [ "$depth" -gt "$max_depth" ]; then
                    echo "❌ $file imports $import_path (depth: $depth, exceeds $max_depth)"
                    violations=$((violations + 1))
                fi
            fi
        done
    fi
done

if [ "$violations" -gt 0 ]; then
    echo "❌ FAIL: $violations deep imports found (>$max_depth levels)"
    echo "Consider restructuring packages to reduce import depth"
    exit 1
else
    echo "✅ PASS: All imports within $max_depth levels"
fi
