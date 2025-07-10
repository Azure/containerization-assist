#!/bin/bash

# Week 3, Day 1: Flatten application/core package  
# Move pkg/mcp/application/core → pkg/mcp/core (depth 4 → 3)

set -e

echo "🎯 Flattening application/core package..."
echo "Moving pkg/mcp/application/core → pkg/mcp/core"

# Create target directory
mkdir -p pkg/mcp/core

# Move all files from application/core to core
if [ -d "pkg/mcp/application/core" ]; then
    echo "Moving core package files..."
    find pkg/mcp/application/core -name "*.go" -exec cp {} pkg/mcp/core/ \;
    
    # Update package declarations in moved files
    find pkg/mcp/core -name "*.go" -exec sed -i 's/^package core$/package core/' {} \;
    
    echo "Moved $(find pkg/mcp/core -name "*.go" | wc -l) Go files"
else
    echo "❌ Source directory pkg/mcp/application/core not found"
    exit 1
fi

# Update all import statements across the codebase
echo "Updating import statements..."

# Find all Go files that import the old path
files_to_update=$(grep -r "github.com/Azure/container-kit/pkg/mcp/application/core" pkg/ --include="*.go" | cut -d: -f1 | sort | uniq)

if [ -n "$files_to_update" ]; then
    echo "Updating imports in $(echo "$files_to_update" | wc -l) files:"
    
    for file in $files_to_update; do
        echo "  - $file"
        # Replace the import path
        sed -i 's|github.com/Azure/container-kit/pkg/mcp/application/core|github.com/Azure/container-kit/pkg/mcp/core|g' "$file"
    done
    
    echo "✅ Updated all import references"
else
    echo "ℹ️  No files found importing old core package"
fi

# Check if we can safely remove the old directory
if [ -d "pkg/mcp/application/core" ]; then
    remaining_files=$(find pkg/mcp/application/core -name "*.go" | wc -l)
    if [ "$remaining_files" -eq 0 ]; then
        echo "Removing empty source directory..."
        rm -rf pkg/mcp/application/core
    else
        echo "⚠️  Source directory still contains $remaining_files Go files - manual review needed"
    fi
fi

echo ""
echo "🎯 Application Core Package Flattening Complete!"
echo "   pkg/mcp/application/core (depth 4) → pkg/mcp/core (depth 3)"
echo ""

# Run a quick verification
echo "🔍 Verification:"
if [ -d "pkg/mcp/core" ]; then
    file_count=$(find pkg/mcp/core -name "*.go" | wc -l)
    echo "✅ New core package exists with $file_count files"
else
    echo "❌ New core package not found!"
    exit 1
fi

# Check for any remaining old imports
remaining_old_imports=$(grep -r "github.com/Azure/container-kit/pkg/mcp/application/core" pkg/ --include="*.go" | wc -l || echo "0")
if [ "$remaining_old_imports" -eq 0 ]; then
    echo "✅ All old imports updated successfully"
else
    echo "⚠️  Found $remaining_old_imports remaining old import references"
fi

echo ""
echo "📊 Impact: This should reduce import depth violations for core package"