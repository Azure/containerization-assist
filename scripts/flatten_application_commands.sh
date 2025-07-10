#!/bin/bash

# Week 3, Day 1: Flatten application/commands package
# Move pkg/mcp/application/commands → pkg/mcp/commands (depth 4 → 3)

set -e

echo "⚡ Flattening application/commands package..."
echo "Moving pkg/mcp/application/commands → pkg/mcp/commands"

# Create target directory
mkdir -p pkg/mcp/commands

# Move all files from application/commands to commands
if [ -d "pkg/mcp/application/commands" ]; then
    echo "Moving commands package files..."
    find pkg/mcp/application/commands -name "*.go" -exec cp {} pkg/mcp/commands/ \;

    # Update package declarations in moved files
    find pkg/mcp/commands -name "*.go" -exec sed -i 's/^package commands$/package commands/' {} \;

    echo "Moved $(find pkg/mcp/commands -name "*.go" | wc -l) Go files"
else
    echo "❌ Source directory pkg/mcp/application/commands not found"
    exit 1
fi

# Update all import statements across the codebase
echo "Updating import statements..."

# Find all Go files that import the old path
files_to_update=$(grep -r "github.com/Azure/container-kit/pkg/mcp/application/commands" pkg/ --include="*.go" | cut -d: -f1 | sort | uniq)

if [ -n "$files_to_update" ]; then
    echo "Updating imports in $(echo "$files_to_update" | wc -l) files:"

    for file in $files_to_update; do
        echo "  - $file"
        # Replace the import path
        sed -i 's|github.com/Azure/container-kit/pkg/mcp/application/commands|github.com/Azure/container-kit/pkg/mcp/commands|g' "$file"
    done

    echo "✅ Updated all import references"
else
    echo "ℹ️  No files found importing old commands package"
fi

# Check if we can safely remove the old directory
if [ -d "pkg/mcp/application/commands" ]; then
    remaining_files=$(find pkg/mcp/application/commands -name "*.go" | wc -l)
    if [ "$remaining_files" -eq 0 ]; then
        echo "Removing empty source directory..."
        rm -rf pkg/mcp/application/commands
    else
        echo "⚠️  Source directory still contains $remaining_files Go files - manual review needed"
    fi
fi

echo ""
echo "⚡ Application Commands Package Flattening Complete!"
echo "   pkg/mcp/application/commands (depth 4) → pkg/mcp/commands (depth 3)"
echo ""

# Run a quick verification
echo "🔍 Verification:"
if [ -d "pkg/mcp/commands" ]; then
    file_count=$(find pkg/mcp/commands -name "*.go" | wc -l)
    echo "✅ New commands package exists with $file_count files"
else
    echo "❌ New commands package not found!"
    exit 1
fi

# Check for any remaining old imports
remaining_old_imports=$(grep -r "github.com/Azure/container-kit/pkg/mcp/application/commands" pkg/ --include="*.go" | wc -l || echo "0")
if [ "$remaining_old_imports" -eq 0 ]; then
    echo "✅ All old imports updated successfully"
else
    echo "⚠️  Found $remaining_old_imports remaining old import references"
fi

echo ""
echo "📊 Impact: This should reduce import depth violations for commands package"
