#!/bin/bash

echo "=== Tool Naming Analysis ==="
echo ""
echo "Finding all tool definitions..."
echo ""

# Find all Go files containing tool structs
echo "Files with Tool structs:"
grep -r "type.*Tool struct" pkg/mcp/internal --include="*.go" | grep -v "_test.go" | while read -r line; do
    file=$(echo "$line" | cut -d: -f1)
    struct=$(echo "$line" | grep -o "type [^ ]* struct" | cut -d' ' -f2)
    basename=$(basename "$file")

    # Check if file name matches expected pattern
    if [[ "$struct" == Atomic* ]]; then
        # Should be *_atomic.go
        if [[ "$basename" == *_atomic.go ]]; then
            echo "✅ $file -> $struct (correct)"
        else
            expected=$(echo "$struct" | sed 's/^Atomic//; s/Tool$//' | sed 's/\([A-Z]\)/_\1/g' | tr '[:upper:]' '[:lower:]' | sed 's/^_//')_atomic.go
            echo "❌ $file -> $struct"
            echo "   Expected filename: $expected"
        fi
    else
        # Non-atomic tools
        if [[ "$basename" == *_atomic.go ]]; then
            echo "❌ $file -> $struct (file has _atomic suffix but struct doesn't have Atomic prefix)"
        else
            echo "✅ $file -> $struct (non-atomic tool)"
        fi
    fi
done

echo ""
echo "=== Summary of Issues ==="
echo ""

# Count tools with naming issues
echo "Atomic tools with incorrect file names:"
grep -r "type Atomic.*Tool struct" pkg/mcp/internal --include="*.go" | grep -v "_test.go" | while read -r line; do
    file=$(echo "$line" | cut -d: -f1)
    basename=$(basename "$file")
    if [[ "$basename" != *_atomic.go ]]; then
        echo "  - $file"
    fi
done

echo ""
echo "Non-atomic tools in atomic files:"
grep -r "type [^A][^t][^o][^m][^i][^c].*Tool struct" pkg/mcp/internal --include="*_atomic.go" | grep -v "_test.go" | while read -r line; do
    file=$(echo "$line" | cut -d: -f1)
    struct=$(echo "$line" | grep -o "type [^ ]* struct" | cut -d' ' -f2)
    echo "  - $file contains $struct"
done

echo ""
echo "=== Recommended Actions ==="
echo ""
echo "1. Rename files to match the pattern:"
echo "   - Atomic tools: tool_name_atomic.go"
echo "   - Non-atomic tools: tool_name.go"
echo ""
echo "2. Ensure struct names match:"
echo "   - Atomic tools: AtomicToolNameTool"
echo "   - Non-atomic tools: ToolNameTool"
