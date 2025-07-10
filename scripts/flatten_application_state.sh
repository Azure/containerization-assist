#!/bin/bash

# Week 3, Day 1: Flatten application/state package
# Move pkg/mcp/application/state → pkg/mcp/state (depth 4 → 3)

set -e

echo "🗂️  Flattening application/state package..."
echo "Moving pkg/mcp/application/state → pkg/mcp/state"

# Note: There's already a session package, so we need to be careful about naming
# Let's call this appstate to avoid conflicts

echo "Creating appstate package to avoid conflicts with existing session package..."

# Create target directory
mkdir -p pkg/mcp/appstate

# Move all files from application/state to appstate
if [ -d "pkg/mcp/application/state" ]; then
    echo "Moving state package files..."
    find pkg/mcp/application/state -name "*.go" -exec cp {} pkg/mcp/appstate/ \;

    # Update package declarations in moved files
    find pkg/mcp/appstate -name "*.go" -exec sed -i 's/^package state$/package appstate/' {} \;

    echo "Moved $(find pkg/mcp/appstate -name "*.go" | wc -l) Go files"
else
    echo "❌ Source directory pkg/mcp/application/state not found"
    exit 1
fi

# Update all import statements across the codebase
echo "Updating import statements..."

# Find all Go files that import the old path
files_to_update=$(grep -r "github.com/Azure/container-kit/pkg/mcp/application/state" pkg/ --include="*.go" | cut -d: -f1 | sort | uniq)

if [ -n "$files_to_update" ]; then
    echo "Updating imports in $(echo "$files_to_update" | wc -l) files:"

    for file in $files_to_update; do
        echo "  - $file"
        # Replace the import path
        sed -i 's|github.com/Azure/container-kit/pkg/mcp/application/state|github.com/Azure/container-kit/pkg/mcp/appstate|g' "$file"
        # Update any package references in the code
        sed -i 's/state\./appstate\./g' "$file"
    done

    echo "✅ Updated all import references"
else
    echo "ℹ️  No files found importing old state package"
fi

# Check if we can safely remove the old directory
if [ -d "pkg/mcp/application/state" ]; then
    remaining_files=$(find pkg/mcp/application/state -name "*.go" | wc -l)
    if [ "$remaining_files" -eq 0 ]; then
        echo "Removing empty source directory..."
        rm -rf pkg/mcp/application/state
    else
        echo "⚠️  Source directory still contains $remaining_files Go files - manual review needed"
    fi
fi

echo ""
echo "🗂️  Application State Package Flattening Complete!"
echo "   pkg/mcp/application/state (depth 4) → pkg/mcp/appstate (depth 3)"
echo ""

# Run a quick verification
echo "🔍 Verification:"
if [ -d "pkg/mcp/appstate" ]; then
    file_count=$(find pkg/mcp/appstate -name "*.go" | wc -l)
    echo "✅ New appstate package exists with $file_count files"
else
    echo "❌ New appstate package not found!"
    exit 1
fi

# Check for any remaining old imports
remaining_old_imports=$(grep -r "github.com/Azure/container-kit/pkg/mcp/application/state" pkg/ --include="*.go" | wc -l || echo "0")
if [ "$remaining_old_imports" -eq 0 ]; then
    echo "✅ All old imports updated successfully"
else
    echo "⚠️  Found $remaining_old_imports remaining old import references"
fi

echo ""
echo "📊 Impact: This should reduce import depth violations for state package"
echo "ℹ️  Note: Renamed to 'appstate' to avoid conflicts with existing session package"
