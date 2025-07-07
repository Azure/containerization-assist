#!/bin/bash

# Script to find potentially unused exported functions and types in pkg/mcp

echo "=== Finding Unused Exported Symbols ==="
echo

UNUSED_COUNT=0
UNUSED_LINES=0

# Function to check if an exported symbol is used
check_symbol_usage() {
    local file="$1"
    local symbol="$2"
    local symbol_type="$3"

    # Skip if it's a test file
    if [[ "$file" =~ _test\.go$ ]]; then
        return 1
    fi

    # Get package name from file
    local pkg=$(grep -m1 "^package " "$file" | awk '{print $2}')
    if [[ -z "$pkg" ]]; then
        return 1
    fi

    # Count usage outside the defining file
    local usage_count=$(grep -r "\b$symbol\b" /home/tng/workspace/beta/pkg/mcp --include="*.go" | \
                       grep -v "$(basename $file):" | \
                       grep -v "_test.go:" | \
                       wc -l)

    if [[ $usage_count -eq 0 ]]; then
        return 0  # Unused
    fi
    return 1  # Used
}

# Find unused types
echo "Checking exported types..."
grep -r "^type [A-Z]" /home/tng/workspace/beta/pkg/mcp --include="*.go" | \
    grep -v "_test.go:" | \
    while IFS=: read -r file line; do
        symbol=$(echo "$line" | awk '{print $2}')
        if check_symbol_usage "$file" "$symbol" "type"; then
            echo "Unused type: $file - $symbol"
            ((UNUSED_COUNT++))
        fi
    done

# Find unused functions
echo
echo "Checking exported functions..."
grep -r "^func [A-Z]" /home/tng/workspace/beta/pkg/mcp --include="*.go" | \
    grep -v "_test.go:" | \
    while IFS=: read -r file line; do
        symbol=$(echo "$line" | awk '{print $2}' | sed 's/[(\[].*$//')
        if check_symbol_usage "$file" "$symbol" "func"; then
            echo "Unused function: $file - $symbol"
            ((UNUSED_COUNT++))
        fi
    done

# Find unused constants
echo
echo "Checking exported constants..."
grep -r "^const [A-Z]" /home/tng/workspace/beta/pkg/mcp --include="*.go" | \
    grep -v "_test.go:" | \
    while IFS=: read -r file line; do
        symbol=$(echo "$line" | awk '{print $2}')
        if check_symbol_usage "$file" "$symbol" "const"; then
            echo "Unused const: $file - $symbol"
            ((UNUSED_COUNT++))
        fi
    done

echo
echo "=== Summary ==="
echo "Found $UNUSED_COUNT potentially unused exported symbols"
echo "(Manual review recommended before removal)"
