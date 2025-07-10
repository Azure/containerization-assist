#!/bin/bash

# Week 3, Day 1: Flatten application/knowledge package
# Move pkg/mcp/application/knowledge → pkg/mcp/knowledge (depth 4 → 3)

set -e

echo "🧠 Flattening application/knowledge package..."
echo "Moving pkg/mcp/application/knowledge → pkg/mcp/knowledge"

# Create target directory
mkdir -p pkg/mcp/knowledge

# Move all files from application/knowledge to knowledge
if [ -d "pkg/mcp/application/knowledge" ]; then
    echo "Moving knowledge package files..."
    find pkg/mcp/application/knowledge -name "*.go" -exec cp {} pkg/mcp/knowledge/ \;
    
    # Update package declarations in moved files
    find pkg/mcp/knowledge -name "*.go" -exec sed -i 's/^package knowledge$/package knowledge/' {} \;
    
    echo "Moved $(find pkg/mcp/knowledge -name "*.go" | wc -l) Go files"
else
    echo "❌ Source directory pkg/mcp/application/knowledge not found"
    exit 1
fi

# Update all import statements across the codebase
echo "Updating import statements..."

# Find all Go files that import the old path
files_to_update=$(grep -r "github.com/Azure/container-kit/pkg/mcp/application/knowledge" pkg/ --include="*.go" | cut -d: -f1 | sort | uniq)

if [ -n "$files_to_update" ]; then
    echo "Updating imports in $(echo "$files_to_update" | wc -l) files:"
    
    for file in $files_to_update; do
        echo "  - $file"
        # Replace the import path
        sed -i 's|github.com/Azure/container-kit/pkg/mcp/application/knowledge|github.com/Azure/container-kit/pkg/mcp/knowledge|g' "$file"
    done
    
    echo "✅ Updated all import references"
else
    echo "ℹ️  No files found importing old knowledge package"
fi

# Check if we can safely remove the old directory
if [ -d "pkg/mcp/application/knowledge" ]; then
    remaining_files=$(find pkg/mcp/application/knowledge -name "*.go" | wc -l)
    if [ "$remaining_files" -eq 0 ]; then
        echo "Removing empty source directory..."
        rm -rf pkg/mcp/application/knowledge
    else
        echo "⚠️  Source directory still contains $remaining_files Go files - manual review needed"
    fi
fi

echo ""
echo "🧠 Application Knowledge Package Flattening Complete!"
echo "   pkg/mcp/application/knowledge (depth 4) → pkg/mcp/knowledge (depth 3)"
echo ""

# Run a quick verification
echo "🔍 Verification:"
if [ -d "pkg/mcp/knowledge" ]; then
    file_count=$(find pkg/mcp/knowledge -name "*.go" | wc -l)
    echo "✅ New knowledge package exists with $file_count files"
else
    echo "❌ New knowledge package not found!"
    exit 1
fi

# Check for any remaining old imports
remaining_old_imports=$(grep -r "github.com/Azure/container-kit/pkg/mcp/application/knowledge" pkg/ --include="*.go" | wc -l || echo "0")
if [ "$remaining_old_imports" -eq 0 ]; then
    echo "✅ All old imports updated successfully"
else
    echo "⚠️  Found $remaining_old_imports remaining old import references"
fi

echo ""
echo "📊 Impact: This should reduce import depth violations for knowledge package"