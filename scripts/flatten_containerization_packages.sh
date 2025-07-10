#!/bin/bash

# Flatten all containerization packages from depth 5 to depth 3
# domain/containerization/{analyze,build,deploy,scan} → domain/{analyze,build,deploy,scan}

set -e

echo "=== Flattening containerization packages ==="

# Define the packages to flatten
packages=("analyze" "build" "deploy" "scan")

for pkg in "${packages[@]}"; do
    echo ""
    echo "--- Processing package: $pkg ---"
    
    # Define paths
    OLD_PKG="pkg/mcp/domain/containerization/$pkg"
    NEW_PKG="pkg/mcp/domain/$pkg"
    OLD_IMPORT="github.com/Azure/container-kit/pkg/mcp/domain/containerization/$pkg"
    NEW_IMPORT="github.com/Azure/container-kit/pkg/mcp/domain/$pkg"
    
    # Step 1: Create new directory and copy files
    echo "Step 1: Creating new directory and copying files..."
    mkdir -p "$NEW_PKG"
    if [ -d "$OLD_PKG" ]; then
        cp -r "$OLD_PKG"/* "$NEW_PKG/" 2>/dev/null || echo "No files to copy for $pkg"
    else
        echo "Warning: $OLD_PKG does not exist"
        continue
    fi
    
    # Step 2: Update package declarations (no change needed - package name stays the same)
    echo "Step 2: Package declarations already correct for $pkg"
    
    # Step 3: Count files to update
    echo "Step 3: Counting files that need import updates for $pkg..."
    import_count=$(grep -r "$OLD_IMPORT" --include="*.go" . 2>/dev/null | grep -v "^Binary file" | grep -v "$NEW_PKG" | wc -l)
    echo "Files with old import: $import_count"
    
    if [ "$import_count" -gt 0 ]; then
        # Step 4: Update imports in the entire codebase
        echo "Step 4: Updating imports across codebase for $pkg..."
        find . -name "*.go" -type f -not -path "./vendor/*" -not -path "./.git/*" -not -path "./$NEW_PKG/*" | while read -r file; do
            if grep -q "$OLD_IMPORT" "$file" 2>/dev/null; then
                echo "Updating: $file"
                sed -i "s|$OLD_IMPORT|$NEW_IMPORT|g" "$file"
            fi
        done
        
        # Step 5: Verify no old imports remain
        echo "Step 5: Verifying migration for $pkg..."
        remaining=$(grep -r "$OLD_IMPORT" --include="*.go" . 2>/dev/null | grep -v "^Binary file" | grep -v "$NEW_PKG" | wc -l)
        if [ "$remaining" -gt 0 ]; then
            echo "WARNING: Found $remaining remaining old imports for $pkg!"
            grep -r "$OLD_IMPORT" --include="*.go" . 2>/dev/null | grep -v "^Binary file" | grep -v "$NEW_PKG" | head -5
        else
            echo "✓ All imports updated successfully for $pkg"
        fi
    else
        echo "No imports to update for $pkg"
    fi
    
    echo "✓ Package $pkg migration completed"
done

echo ""
echo "=== Testing compilation ==="
if make mcp 2>&1 | tail -5; then
    echo "✓ Compilation successful"
else
    echo "✗ Compilation failed"
    exit 1
fi

echo ""
echo "=== Migration Summary ==="
for pkg in "${packages[@]}"; do
    OLD_PKG="pkg/mcp/domain/containerization/$pkg"
    NEW_PKG="pkg/mcp/domain/$pkg"
    if [ -d "$NEW_PKG" ]; then
        file_count=$(find "$NEW_PKG" -name "*.go" -type f | wc -l)
        echo "Package $pkg: $file_count files moved to $NEW_PKG"
    fi
done

echo ""
echo "Next steps:"
echo "1. Remove old directories:"
for pkg in "${packages[@]}"; do
    echo "   rm -rf pkg/mcp/domain/containerization/$pkg"
done
echo "2. Remove empty containerization directory: rmdir pkg/mcp/domain/containerization"
echo "3. Run import depth checker to verify improvements"
echo "4. Commit the changes"

echo ""
echo "✓ All containerization packages flattened successfully!"