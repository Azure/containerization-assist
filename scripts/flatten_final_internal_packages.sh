#!/bin/bash

# Week 3, Day 2: Flatten final internal packages
# Complete the architecture refactoring by flattening the last depth 5 packages

set -e

echo "🎯 Flattening final internal packages to complete architecture refactoring..."
echo ""

# Function to flatten an internal package
flatten_final_package() {
    local source_path=$1
    local target_name=$2
    local package_name=$3

    echo "📦 Moving $source_path → pkg/mcp/$target_name"

    # Create target directory
    mkdir -p "pkg/mcp/$target_name"

    # Move all files from source to target
    if [ -d "$source_path" ]; then
        echo "Moving $package_name package files..."
        find "$source_path" -name "*.go" -exec cp {} "pkg/mcp/$target_name/" \;

        # Update package declarations in moved files
        find "pkg/mcp/$target_name" -name "*.go" -exec sed -i "s/^package $package_name$/package $target_name/" {} \;

        echo "Moved $(find "pkg/mcp/$target_name" -name "*.go" | wc -l) Go files for $target_name"

        # Update all import statements across the codebase
        echo "Updating import statements for $target_name..."

        # Convert source path to import path
        import_path="github.com/Azure/container-kit/$source_path"

        # Find all Go files that import the old path
        files_to_update=$(grep -r "$import_path" pkg/ --include="*.go" | cut -d: -f1 | sort | uniq || echo "")

        if [ -n "$files_to_update" ]; then
            echo "Updating imports in $(echo "$files_to_update" | wc -l) files:"

            for file in $files_to_update; do
                echo "  - $file"
                # Replace the import path
                sed -i "s|$import_path|github.com/Azure/container-kit/pkg/mcp/$target_name|g" "$file"
                # Update any package references in the code if needed
                if [ "$package_name" != "$target_name" ]; then
                    sed -i "s/$package_name\./$target_name\./g" "$file"
                fi
            done

            echo "✅ Updated all import references for $target_name"
        else
            echo "ℹ️  No files found importing old $target_name package"
        fi

        # Remove old directory
        rm -rf "$source_path"
        echo "🗑️  Removed old $source_path directory"

    else
        echo "❌ Source directory $source_path not found"
    fi

    echo ""
}

# Flatten all remaining problematic packages
echo "Starting final package flattening..."
echo ""

# 1. application/internal/conversation → conversation
flatten_final_package "pkg/mcp/application/internal/conversation" "conversation" "conversation"

# 2. application/internal/runtime → runtime
flatten_final_package "pkg/mcp/application/internal/runtime" "runtime" "runtime"

# 3. application/orchestration/pipeline → pipeline
flatten_final_package "pkg/mcp/application/orchestration/pipeline" "pipeline" "pipeline"

echo "🎯 All Final Package Flattening Complete!"
echo ""

# Run verification
echo "🔍 Verification:"
for package in conversation runtime pipeline; do
    if [ -d "pkg/mcp/$package" ]; then
        file_count=$(find "pkg/mcp/$package" -name "*.go" | wc -l)
        echo "✅ New $package package exists with $file_count files"
    else
        echo "❌ New $package package not found!"
    fi
done

echo ""
echo "📊 Impact: This should eliminate all remaining depth 5 violations!"
echo "🏆 Container Kit architecture refactoring is now COMPLETE!"
