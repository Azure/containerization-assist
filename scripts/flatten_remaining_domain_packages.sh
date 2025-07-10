#!/bin/bash

# Week 3, Day 1: Flatten remaining domain packages
# Move domain/analyze, domain/build, domain/deploy, domain/scan from depth 4 to 3

set -e

echo "🔧 Flattening remaining domain packages..."

# Function to flatten a domain package
flatten_domain_package() {
    local package_name=$1
    echo "Moving pkg/mcp/domain/$package_name → pkg/mcp/$package_name"
    
    # Create target directory
    mkdir -p "pkg/mcp/$package_name"
    
    # Move all files from domain/package to package
    if [ -d "pkg/mcp/domain/$package_name" ]; then
        echo "Moving $package_name package files..."
        find "pkg/mcp/domain/$package_name" -name "*.go" -exec cp {} "pkg/mcp/$package_name/" \;
        
        # Update package declarations in moved files
        find "pkg/mcp/$package_name" -name "*.go" -exec sed -i "s/^package $package_name$/package $package_name/" {} \;
        
        echo "Moved $(find "pkg/mcp/$package_name" -name "*.go" | wc -l) Go files for $package_name"
        
        # Update all import statements across the codebase
        echo "Updating import statements for $package_name..."
        
        # Find all Go files that import the old path
        files_to_update=$(grep -r "github.com/Azure/container-kit/pkg/mcp/domain/$package_name" pkg/ --include="*.go" | cut -d: -f1 | sort | uniq || echo "")
        
        if [ -n "$files_to_update" ]; then
            echo "Updating imports in $(echo "$files_to_update" | wc -l) files:"
            
            for file in $files_to_update; do
                echo "  - $file"
                # Replace the import path
                sed -i "s|github.com/Azure/container-kit/pkg/mcp/domain/$package_name|github.com/Azure/container-kit/pkg/mcp/$package_name|g" "$file"
            done
            
            echo "✅ Updated all import references for $package_name"
        else
            echo "ℹ️  No files found importing old $package_name package"
        fi
        
        # Remove old directory
        rm -rf "pkg/mcp/domain/$package_name"
        echo "🗑️  Removed old domain/$package_name directory"
        
    else
        echo "❌ Source directory pkg/mcp/domain/$package_name not found"
    fi
    
    echo ""
}

# Flatten all remaining domain packages
echo "Starting domain package flattening..."
echo ""

flatten_domain_package "analyze"
flatten_domain_package "build" 
flatten_domain_package "deploy"
flatten_domain_package "scan"

echo "🔧 All Domain Package Flattening Complete!"
echo ""

# Run verification
echo "🔍 Verification:"
for package in analyze build deploy scan; do
    if [ -d "pkg/mcp/$package" ]; then
        file_count=$(find "pkg/mcp/$package" -name "*.go" | wc -l)
        echo "✅ New $package package exists with $file_count files"
    else
        echo "❌ New $package package not found!"
    fi
done

echo ""
echo "📊 Impact: This should significantly reduce depth 4 violations"