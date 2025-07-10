#!/bin/bash

# Week 3, Day 1: Flatten errors/codes package
# Move pkg/mcp/errors/codes → pkg/mcp/errorcodes (depth 4 → 3)

set -e

echo "🚨 Flattening errors/codes package..."
echo "Moving pkg/mcp/errors/codes → pkg/mcp/errorcodes"

# Create target directory
mkdir -p pkg/mcp/errorcodes

# Move all files from errors/codes to errorcodes
if [ -d "pkg/mcp/errors/codes" ]; then
    echo "Moving error codes package files..."
    find pkg/mcp/errors/codes -name "*.go" -exec cp {} pkg/mcp/errorcodes/ \;

    # Update package declarations in moved files
    find pkg/mcp/errorcodes -name "*.go" -exec sed -i 's/^package codes$/package errorcodes/' {} \;

    echo "Moved $(find pkg/mcp/errorcodes -name "*.go" | wc -l) Go files"
else
    echo "❌ Source directory pkg/mcp/errors/codes not found"
    exit 1
fi

# Update all import statements across the codebase
echo "Updating import statements..."

# Find all Go files that import the old path
files_to_update=$(grep -r "github.com/Azure/container-kit/pkg/mcp/errors/codes" pkg/ --include="*.go" | cut -d: -f1 | sort | uniq)

if [ -n "$files_to_update" ]; then
    echo "Updating imports in $(echo "$files_to_update" | wc -l) files:"

    for file in $files_to_update; do
        echo "  - $file"
        # Replace the import path
        sed -i 's|github.com/Azure/container-kit/pkg/mcp/errors/codes|github.com/Azure/container-kit/pkg/mcp/errorcodes|g' "$file"
        # Update any package references in the code
        sed -i 's/codes\./errorcodes\./g' "$file"
    done

    echo "✅ Updated all import references"
else
    echo "ℹ️  No files found importing old codes package"
fi

# Check if we can safely remove the old directory
if [ -d "pkg/mcp/errors/codes" ]; then
    remaining_files=$(find pkg/mcp/errors/codes -name "*.go" | wc -l)
    if [ "$remaining_files" -eq 0 ]; then
        echo "Removing empty source directory..."
        rm -rf pkg/mcp/errors/codes
    else
        echo "⚠️  Source directory still contains $remaining_files Go files - manual review needed"
    fi
fi

echo ""
echo "🚨 Error Codes Package Flattening Complete!"
echo "   pkg/mcp/errors/codes (depth 4) → pkg/mcp/errorcodes (depth 3)"
echo ""

# Run a quick verification
echo "🔍 Verification:"
if [ -d "pkg/mcp/errorcodes" ]; then
    file_count=$(find pkg/mcp/errorcodes -name "*.go" | wc -l)
    echo "✅ New errorcodes package exists with $file_count files"
else
    echo "❌ New errorcodes package not found!"
    exit 1
fi

# Check for any remaining old imports
remaining_old_imports=$(grep -r "github.com/Azure/container-kit/pkg/mcp/errors/codes" pkg/ --include="*.go" | wc -l || echo "0")
if [ "$remaining_old_imports" -eq 0 ]; then
    echo "✅ All old imports updated successfully"
else
    echo "⚠️  Found $remaining_old_imports remaining old import references"
fi

echo ""
echo "📊 Impact: This should reduce import depth violations for error codes"
