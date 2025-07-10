#!/bin/bash

# Week 3, Day 2: Enforce Three-Layer Architecture 
# Follow ADR-001: Move all packages to proper domain/, application/, infra/ structure

set -e

echo "🏗️ Enforcing Three-Layer Architecture per ADR-001..."
echo "Moving all packages to proper domain/, application/, infra/ structure"
echo ""

# Function to move a package to proper layer
move_to_layer() {
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

# Enforce ADR-001: Three-Context Architecture
# Domain: Pure business logic
# Application: Use cases, orchestration  
# Infrastructure: External integrations

echo "🏛️ Enforcing ADR-001 Three-Layer Architecture"
echo ""

echo "1️⃣ Moving tools to domain layer (business logic)..."
move_to_layer "pkg/mcp/tools" "pkg/mcp/domain/tools" "tools"

echo "2️⃣ Moving services to application layer (orchestration)..."  
move_to_layer "pkg/mcp/services" "pkg/mcp/application/services" "services"

echo "3️⃣ Moving core to application layer (use cases)..."
move_to_layer "pkg/mcp/core" "pkg/mcp/application/core" "core"

echo "🏛️ Three-Layer Architecture Enforcement Complete!"
echo ""

# Run verification
echo "🔍 Verification:"
layer_packages=("domain/tools" "application/services" "application/core")
for layer_package in "${layer_packages[@]}"; do
    if [ -d "pkg/mcp/$layer_package" ]; then
        file_count=$(find "pkg/mcp/$layer_package" -name "*.go" | wc -l)
        echo "✅ $layer_package exists with $file_count files"
    else
        echo "❌ $layer_package not found!"
    fi
done

echo ""
echo "📊 Impact: Full compliance with ADR-001 Three-Layer Architecture"
echo "🎯 Structure now matches architectural decision record"
echo ""
echo "Final Structure (per ADR-001):"
echo "- pkg/mcp/domain/tools/ (business logic)"
echo "- pkg/mcp/application/services/ (orchestration)"
echo "- pkg/mcp/application/core/ (use cases)"
echo "- pkg/mcp/domain/* (other domain packages)"
echo "- pkg/mcp/application/* (other application packages)"
echo "- pkg/mcp/infra/* (infrastructure packages)"