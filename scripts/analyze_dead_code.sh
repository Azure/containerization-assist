#!/bin/bash

# Dead code analysis script for pkg/mcp

echo "=== Dead Code Analysis for pkg/mcp ==="
echo ""

# 1. Find exported functions/types that are never used
echo "### 1. Checking for unused exported symbols..."

# Get all exported symbols
find pkg/mcp -name "*.go" -not -path "*/test/*" -not -name "*_test.go" | xargs grep -h "^func [A-Z]" | sed 's/func \([A-Za-z0-9_]*\).*/\1/' | sort -u > /tmp/exported_funcs.txt
find pkg/mcp -name "*.go" -not -path "*/test/*" -not -name "*_test.go" | xargs grep -h "^type [A-Z]" | sed 's/type \([A-Za-z0-9_]*\).*/\1/' | sort -u > /tmp/exported_types.txt

# Check usage of each exported function
echo "Potentially unused exported functions:"
while IFS= read -r func; do
    usage_count=$(grep -r "\b$func\b" pkg/mcp --include="*.go" | grep -v "^[^:]*:func $func" | grep -v "^[^:]*:type $func" | wc -l)
    if [ "$usage_count" -eq 0 ]; then
        echo "  - $func"
    fi
done < /tmp/exported_funcs.txt

echo ""
echo "Potentially unused exported types:"
while IFS= read -r type; do
    usage_count=$(grep -r "\b$type\b" pkg/mcp --include="*.go" | grep -v "^[^:]*:type $type" | wc -l)
    if [ "$usage_count" -eq 0 ]; then
        echo "  - $type"
    fi
done < /tmp/exported_types.txt

# 2. Find test files without corresponding source files
echo ""
echo "### 2. Orphaned test files..."
find pkg/mcp -name "*_test.go" | while read test_file; do
    base_file="${test_file%_test.go}.go"
    if [ ! -f "$base_file" ]; then
        echo "  - $test_file (no corresponding source file)"
    fi
done

# 3. Find mock files that might be unused
echo ""
echo "### 3. Mock files analysis..."
find pkg/mcp -name "mock_*.go" -o -name "*_mock.go" | while read mock_file; do
    mock_name=$(basename "$mock_file" .go)
    usage_count=$(grep -r "$mock_name" pkg/mcp --include="*.go" | grep -v "^$mock_file:" | wc -l)
    if [ "$usage_count" -eq 0 ]; then
        echo "  - $mock_file (unused mock)"
    fi
done

# 4. Find deprecated/legacy patterns
echo ""
echo "### 4. Deprecated/legacy code patterns..."
grep -r "deprecated\|legacy\|TODO.*remove\|FIXME.*remove" pkg/mcp --include="*.go" | grep -i "deprecated\|legacy" | head -20

# 5. Find unused constants
echo ""
echo "### 5. Potentially unused constants..."
grep -r "^const [A-Z]" pkg/mcp --include="*.go" | while IFS=: read -r file line; do
    const_name=$(echo "$line" | sed 's/const \([A-Za-z0-9_]*\).*/\1/')
    usage_count=$(grep -r "\b$const_name\b" pkg/mcp --include="*.go" | grep -v "^$file:" | wc -l)
    if [ "$usage_count" -eq 0 ]; then
        echo "  - $file: $const_name"
    fi
done | head -20

# 6. Find unused build tags
echo ""
echo "### 6. Build tags analysis..."
grep -r "// +build" pkg/mcp --include="*.go" | sort -u

# 7. Find unused interfaces
echo ""
echo "### 7. Potentially unused interfaces..."
grep -r "^type.*interface {" pkg/mcp --include="*.go" | while IFS=: read -r file line; do
    interface_name=$(echo "$line" | sed 's/type \([A-Za-z0-9_]*\) interface.*/\1/')
    usage_count=$(grep -r "\b$interface_name\b" pkg/mcp --include="*.go" | grep -v "^$file:" | wc -l)
    if [ "$usage_count" -eq 0 ]; then
        echo "  - $file: $interface_name"
    fi
done | head -20

echo ""
echo "=== Analysis Complete ==="
