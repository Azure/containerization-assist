#!/bin/bash

# Week 3, Day 1: Flatten domain/types package
# Move pkg/mcp/domain/types → pkg/mcp/domaintypes (depth 4 → 3)

set -e

echo "📝 Flattening domain/types package..."
echo "Moving pkg/mcp/domain/types → pkg/mcp/domaintypes"

# Create target directory
mkdir -p pkg/mcp/domaintypes

# Move all files from domain/types to domaintypes
if [ -d "pkg/mcp/domain/types" ]; then
    echo "Moving domain types package files..."
    find pkg/mcp/domain/types -name "*.go" -exec cp {} pkg/mcp/domaintypes/ \;

    # Update package declarations in moved files
    find pkg/mcp/domaintypes -name "*.go" -exec sed -i 's/^package types$/package domaintypes/' {} \;

    echo "Moved $(find pkg/mcp/domaintypes -name "*.go" | wc -l) Go files"
else
    echo "❌ Source directory pkg/mcp/domain/types not found"
    exit 1
fi

# Update all import statements across the codebase
echo "Updating import statements..."

# Find all Go files that import the old path
files_to_update=$(grep -r "github.com/Azure/container-kit/pkg/mcp/domain/types" pkg/ --include="*.go" | cut -d: -f1 | sort | uniq)

if [ -n "$files_to_update" ]; then
    echo "Updating imports in $(echo "$files_to_update" | wc -l) files:"

    for file in $files_to_update; do
        echo "  - $file"
        # Replace the import path
        sed -i 's|github.com/Azure/container-kit/pkg/mcp/domain/types|github.com/Azure/container-kit/pkg/mcp/domaintypes|g' "$file"
        # Update any package references in the code
        sed -i 's/types\./domaintypes\./g' "$file"
    done

    echo "✅ Updated all import references"
else
    echo "ℹ️  No files found importing old types package"
fi

# Check if we can safely remove the old directory
if [ -d "pkg/mcp/domain/types" ]; then
    remaining_files=$(find pkg/mcp/domain/types -name "*.go" | wc -l)
    if [ "$remaining_files" -eq 0 ]; then
        echo "Removing empty source directory..."
        rm -rf pkg/mcp/domain/types
    else
        echo "⚠️  Source directory still contains $remaining_files Go files - manual review needed"
    fi
fi

echo ""
echo "📝 Domain Types Package Flattening Complete!"
echo "   pkg/mcp/domain/types (depth 4) → pkg/mcp/domaintypes (depth 3)"
echo ""

# Run a quick verification
echo "🔍 Verification:"
if [ -d "pkg/mcp/domaintypes" ]; then
    file_count=$(find pkg/mcp/domaintypes -name "*.go" | wc -l)
    echo "✅ New domaintypes package exists with $file_count files"
else
    echo "❌ New domaintypes package not found!"
    exit 1
fi

# Check for any remaining old imports
remaining_old_imports=$(grep -r "github.com/Azure/container-kit/pkg/mcp/domain/types" pkg/ --include="*.go" | wc -l || echo "0")
if [ "$remaining_old_imports" -eq 0 ]; then
    echo "✅ All old imports updated successfully"
else
    echo "⚠️  Found $remaining_old_imports remaining old import references"
fi

echo ""
echo "📊 Impact: This should reduce import depth violations for domain types"
