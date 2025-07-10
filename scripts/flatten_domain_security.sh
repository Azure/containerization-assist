#!/bin/bash

# Week 3, Day 1: Flatten domain/security package
# Move pkg/mcp/domain/security → pkg/mcp/security (depth 4 → 3)

set -e

echo "🔒 Flattening domain/security package..."
echo "Moving pkg/mcp/domain/security → pkg/mcp/security"

# Create target directory
mkdir -p pkg/mcp/security

# Move all files from domain/security to security
if [ -d "pkg/mcp/domain/security" ]; then
    echo "Moving security package files..."
    find pkg/mcp/domain/security -name "*.go" -exec cp {} pkg/mcp/security/ \;
    
    # Update package declarations in moved files
    find pkg/mcp/security -name "*.go" -exec sed -i 's/^package security$/package security/' {} \;
    
    echo "Moved $(find pkg/mcp/security -name "*.go" | wc -l) Go files"
else
    echo "❌ Source directory pkg/mcp/domain/security not found"
    exit 1
fi

# Update all import statements across the codebase
echo "Updating import statements..."

# Find all Go files that import the old path
files_to_update=$(grep -r "github.com/Azure/container-kit/pkg/mcp/domain/security" pkg/ --include="*.go" | cut -d: -f1 | sort | uniq)

if [ -n "$files_to_update" ]; then
    echo "Updating imports in $(echo "$files_to_update" | wc -l) files:"
    
    for file in $files_to_update; do
        echo "  - $file"
        # Replace the import path
        sed -i 's|github.com/Azure/container-kit/pkg/mcp/domain/security|github.com/Azure/container-kit/pkg/mcp/security|g' "$file"
    done
    
    echo "✅ Updated all import references"
else
    echo "ℹ️  No files found importing old security package"
fi

# Check if we can safely remove the old directory
if [ -d "pkg/mcp/domain/security" ]; then
    remaining_files=$(find pkg/mcp/domain/security -name "*.go" | wc -l)
    if [ "$remaining_files" -eq 0 ]; then
        echo "Removing empty source directory..."
        rm -rf pkg/mcp/domain/security
    else
        echo "⚠️  Source directory still contains $remaining_files Go files - manual review needed"
    fi
fi

echo ""
echo "🔒 Domain Security Package Flattening Complete!"
echo "   pkg/mcp/domain/security (depth 4) → pkg/mcp/security (depth 3)"
echo ""

# Run a quick verification
echo "🔍 Verification:"
if [ -d "pkg/mcp/security" ]; then
    file_count=$(find pkg/mcp/security -name "*.go" | wc -l)
    echo "✅ New security package exists with $file_count files"
else
    echo "❌ New security package not found!"
    exit 1
fi

# Check for any remaining old imports
remaining_old_imports=$(grep -r "github.com/Azure/container-kit/pkg/mcp/domain/security" pkg/ --include="*.go" | wc -l || echo "0")
if [ "$remaining_old_imports" -eq 0 ]; then
    echo "✅ All old imports updated successfully"
else
    echo "⚠️  Found $remaining_old_imports remaining old import references"
fi

echo ""
echo "📊 Impact: This should reduce import depth violations for security package"