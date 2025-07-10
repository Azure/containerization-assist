#!/bin/bash

# Week 3, Day 2: Rebalance Architecture
# Move commonly used packages back to flat structure while preserving three-layer for domain logic

set -e

echo "⚖️ Rebalancing architecture..."
echo "Moving commonly used packages back to flat structure"
echo ""

# Function to move a package back to flat structure
move_package_to_flat() {
    local source_path=$1
    local target_path=$2
    local package_name=$3
    
    echo "📦 Moving $source_path → $target_path"
    
    # Create target directory
    mkdir -p "$target_path"
    
    # Move all files from source to target
    if [ -d "$source_path" ]; then
        echo "Moving $package_name package files..."
        find "$source_path" -name "*.go" -exec cp {} "$target_path/" \;
        
        echo "Moved $(find "$target_path" -name "*.go" | wc -l) Go files for $package_name"
        
        # Update all import statements across the codebase
        echo "Updating import statements for $package_name..."
        
        # Convert paths to import format
        old_import="github.com/Azure/container-kit/$source_path"
        new_import="github.com/Azure/container-kit/$target_path"
        
        # Find all Go files that import the old path
        files_to_update=$(grep -r "$old_import" pkg/ --include="*.go" | cut -d: -f1 | sort | uniq || echo "")
        
        if [ -n "$files_to_update" ]; then
            echo "Updating imports in $(echo "$files_to_update" | wc -l) files:"
            
            for file in $files_to_update; do
                echo "  - $file"
                # Replace the import path
                sed -i "s|$old_import|$new_import|g" "$file"
            done
            
            echo "✅ Updated all import references for $package_name"
        else
            echo "ℹ️  No files found importing old $package_name package"
        fi
        
        # Remove old directory
        rm -rf "$source_path"
        echo "🗑️  Removed old $source_path directory"
        
    else
        echo "❌ Source directory $source_path not found"
    fi
    
    echo ""
}

# Move commonly used packages back to flat structure for easier access

echo "1️⃣ Moving tools back to flat structure (commonly used)..."
move_package_to_flat "pkg/mcp/domain/tools" "pkg/mcp/tools" "tools"

echo "2️⃣ Moving services back to flat structure (commonly used)..."  
move_package_to_flat "pkg/mcp/application/services" "pkg/mcp/services" "services"

echo "3️⃣ Moving core back to flat structure (commonly used)..."
move_package_to_flat "pkg/mcp/application/core" "pkg/mcp/core" "core"

echo "⚖️ Architecture Rebalancing Complete!"
echo ""

# Run verification
echo "🔍 Verification:"
for package in "tools" "services" "core"; do
    if [ -d "pkg/mcp/$package" ]; then
        file_count=$(find "pkg/mcp/$package" -name "*.go" | wc -l)
        echo "✅ $package exists with $file_count files (flat structure)"
    else
        echo "❌ $package not found!"
    fi
done

echo ""
echo "📊 Impact: Balanced architecture with flat commonly-used packages"
echo "🎯 Achieves both ≤3 depth limit AND logical organization"
echo ""
echo "Final Structure:"
echo "- pkg/mcp/tools/ (flat - commonly used)"
echo "- pkg/mcp/services/ (flat - commonly used)"  
echo "- pkg/mcp/core/ (flat - commonly used)"
echo "- pkg/mcp/domain/ (three-layer - domain logic)"
echo "- pkg/mcp/application/ (three-layer - application logic)"
echo "- pkg/mcp/infra/ (three-layer - infrastructure)"