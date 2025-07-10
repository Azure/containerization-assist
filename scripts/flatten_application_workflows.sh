#!/bin/bash

# Week 3, Day 1: Flatten application/workflows package
# Move pkg/mcp/application/workflows → pkg/mcp/workflows (depth 4 → 3)

set -e

echo "🔄 Flattening application/workflows package..."
echo "Moving pkg/mcp/application/workflows → pkg/mcp/workflows"

# Create target directory
mkdir -p pkg/mcp/workflows

# Move all files from application/workflows to workflows
if [ -d "pkg/mcp/application/workflows" ]; then
    echo "Moving workflows package files..."
    find pkg/mcp/application/workflows -name "*.go" -exec cp {} pkg/mcp/workflows/ \;

    # Update package declarations in moved files
    find pkg/mcp/workflows -name "*.go" -exec sed -i 's/^package workflows$/package workflows/' {} \;

    echo "Moved $(find pkg/mcp/workflows -name "*.go" | wc -l) Go files"
else
    echo "❌ Source directory pkg/mcp/application/workflows not found"
    exit 1
fi

# Update all import statements across the codebase
echo "Updating import statements..."

# Find all Go files that import the old path
files_to_update=$(grep -r "github.com/Azure/container-kit/pkg/mcp/application/workflows" pkg/ --include="*.go" | cut -d: -f1 | sort | uniq)

if [ -n "$files_to_update" ]; then
    echo "Updating imports in $(echo "$files_to_update" | wc -l) files:"

    for file in $files_to_update; do
        echo "  - $file"
        # Replace the import path
        sed -i 's|github.com/Azure/container-kit/pkg/mcp/application/workflows|github.com/Azure/container-kit/pkg/mcp/workflows|g' "$file"
    done

    echo "✅ Updated all import references"
else
    echo "ℹ️  No files found importing old workflows package"
fi

# Check if we can safely remove the old directory
if [ -d "pkg/mcp/application/workflows" ]; then
    remaining_files=$(find pkg/mcp/application/workflows -name "*.go" | wc -l)
    if [ "$remaining_files" -eq 0 ]; then
        echo "Removing empty source directory..."
        rm -rf pkg/mcp/application/workflows
    else
        echo "⚠️  Source directory still contains $remaining_files Go files - manual review needed"
    fi
fi

echo ""
echo "🔄 Application Workflows Package Flattening Complete!"
echo "   pkg/mcp/application/workflows (depth 4) → pkg/mcp/workflows (depth 3)"
echo ""

# Run a quick verification
echo "🔍 Verification:"
if [ -d "pkg/mcp/workflows" ]; then
    file_count=$(find pkg/mcp/workflows -name "*.go" | wc -l)
    echo "✅ New workflows package exists with $file_count files"
else
    echo "❌ New workflows package not found!"
    exit 1
fi

# Check for any remaining old imports
remaining_old_imports=$(grep -r "github.com/Azure/container-kit/pkg/mcp/application/workflows" pkg/ --include="*.go" | wc -l || echo "0")
if [ "$remaining_old_imports" -eq 0 ]; then
    echo "✅ All old imports updated successfully"
else
    echo "⚠️  Found $remaining_old_imports remaining old import references"
fi

echo ""
echo "📊 Impact: This should reduce import depth violations for workflows package"
