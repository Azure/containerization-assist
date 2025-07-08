#!/bin/bash
set -e

echo "=== FILE SIZE CHECKER ==="

max_lines=800
violations=0

echo "Checking for files exceeding $max_lines lines..."

find pkg/mcp -name "*.go" | while read file; do
    lines=$(wc -l < "$file")
    if [ "$lines" -gt "$max_lines" ]; then
        echo "❌ $file: $lines lines (exceeds $max_lines)"
        violations=$((violations + 1))
    fi
done

# Check if any violations were found
large_files=$(find pkg/mcp -name "*.go" -exec wc -l {} \; | awk '$1 > 800 {print $2 ": " $1 " lines"}')
if [ -n "$large_files" ]; then
    echo "❌ FAIL: Files exceed size limit:"
    echo "$large_files"
    echo "Consider breaking large files into smaller, focused modules"
    exit 1
else
    echo "✅ PASS: All files within $max_lines line limit"
fi
