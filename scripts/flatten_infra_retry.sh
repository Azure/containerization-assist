#!/bin/bash

# Week 3, Day 1: Flatten infra/retry package
# Move pkg/mcp/infra/retry → pkg/mcp/retry (depth 4 → 3)

set -e

echo "🔁 Flattening infra/retry package..."
echo "Moving pkg/mcp/infra/retry → pkg/mcp/retry"

# Create target directory
mkdir -p pkg/mcp/retry

# Move all files from infra/retry to retry
if [ -d "pkg/mcp/infra/retry" ]; then
    echo "Moving retry package files..."
    find pkg/mcp/infra/retry -name "*.go" -exec cp {} pkg/mcp/retry/ \;

    # Update package declarations in moved files
    find pkg/mcp/retry -name "*.go" -exec sed -i 's/^package retry$/package retry/' {} \;

    echo "Moved $(find pkg/mcp/retry -name "*.go" | wc -l) Go files"
else
    echo "❌ Source directory pkg/mcp/infra/retry not found"
    exit 1
fi

# Update all import statements across the codebase
echo "Updating import statements..."

# Find all Go files that import the old path
files_to_update=$(grep -r "github.com/Azure/container-kit/pkg/mcp/infra/retry" pkg/ --include="*.go" | cut -d: -f1 | sort | uniq)

if [ -n "$files_to_update" ]; then
    echo "Updating imports in $(echo "$files_to_update" | wc -l) files:"

    for file in $files_to_update; do
        echo "  - $file"
        # Replace the import path
        sed -i 's|github.com/Azure/container-kit/pkg/mcp/infra/retry|github.com/Azure/container-kit/pkg/mcp/retry|g' "$file"
    done

    echo "✅ Updated all import references"
else
    echo "ℹ️  No files found importing old retry package"
fi

# Check if we can safely remove the old directory
if [ -d "pkg/mcp/infra/retry" ]; then
    remaining_files=$(find pkg/mcp/infra/retry -name "*.go" | wc -l)
    if [ "$remaining_files" -eq 0 ]; then
        echo "Removing empty source directory..."
        rm -rf pkg/mcp/infra/retry
    else
        echo "⚠️  Source directory still contains $remaining_files Go files - manual review needed"
    fi
fi

echo ""
echo "🔁 Infra Retry Package Flattening Complete!"
echo "   pkg/mcp/infra/retry (depth 4) → pkg/mcp/retry (depth 3)"
echo ""

# Run a quick verification
echo "🔍 Verification:"
if [ -d "pkg/mcp/retry" ]; then
    file_count=$(find pkg/mcp/retry -name "*.go" | wc -l)
    echo "✅ New retry package exists with $file_count files"
else
    echo "❌ New retry package not found!"
    exit 1
fi

# Check for any remaining old imports
remaining_old_imports=$(grep -r "github.com/Azure/container-kit/pkg/mcp/infra/retry" pkg/ --include="*.go" | wc -l || echo "0")
if [ "$remaining_old_imports" -eq 0 ]; then
    echo "✅ All old imports updated successfully"
else
    echo "⚠️  Found $remaining_old_imports remaining old import references"
fi

echo ""
echo "📊 Impact: This should reduce import depth violations for retry package"
