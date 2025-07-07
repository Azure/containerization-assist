#!/usr/bin/env bash
set -euo pipefail

# Error Inventory Script for RichError Migration
# Finds all fmt.Errorf/errors.New usage outside of pkg/mcp/errors

echo "🔍 Scanning for fmt.Errorf and errors.New usage..."

# Collect all fmt.Errorf/errors.New that are *not* inside pkg/mcp/errors
grep -R --line-number -E "fmt\.Errorf|errors\.New" \
    --exclude-dir=pkg/mcp/errors \
    --exclude="*.test.go" \
    --exclude-dir=vendor \
    --exclude-dir=.git \
    --include="*.go" \
    . | tee /tmp/error_inventory.txt

echo ""
echo "📊 Summary:"
total_count=$(wc -l < /tmp/error_inventory.txt)
fmt_errorf_count=$(grep -c "fmt\.Errorf" /tmp/error_inventory.txt || echo "0")
errors_new_count=$(grep -c "errors\.New" /tmp/error_inventory.txt || echo "0")

echo "Total error usages found: $total_count"
echo "fmt.Errorf usages: $fmt_errorf_count"
echo "errors.New usages: $errors_new_count"

echo ""
echo "📊 Generating CSV classification..."

# Generate CSV with file:line format for boundary detection
awk -F: '{printf "%s:%s\n",$1,$2}' /tmp/error_inventory.txt > /tmp/error_inventory.csv

echo "✅ Error inventory saved to /tmp/error_inventory.txt"
echo "✅ CSV classification saved to /tmp/error_inventory.csv"
echo "📝 Next step: Run boundary detection tool"
