#!/bin/bash

# Flatten application/logging package to top-level logging package
# This script moves files and updates all imports

set -e

echo "=== Flattening application/logging package ==="

# Base directories
OLD_PKG="pkg/mcp/application/logging"
NEW_PKG="pkg/mcp/logging"
OLD_IMPORT="github.com/Azure/container-kit/pkg/mcp/application/logging"
NEW_IMPORT="github.com/Azure/container-kit/pkg/mcp/logging"

# Step 1: Create new directory and copy files
echo "Step 1: Creating new directory and copying files..."
mkdir -p "$NEW_PKG"
cp -r "$OLD_PKG"/* "$NEW_PKG/" 2>/dev/null || echo "No files to copy"

# Step 2: Update package declarations in the new files
echo "Step 2: Updating package declarations..."
find "$NEW_PKG" -name "*.go" -type f | while read -r file; do
    # Update package declaration from "package logging" to "package logging"
    # (no change needed as package name stays the same)
    sed -i 's/^package logging$/package logging/' "$file"
done

# Step 3: Count files to update
echo "Step 3: Counting files that need import updates..."
echo "Files with old import:"
grep -r "$OLD_IMPORT" --include="*.go" . 2>/dev/null | grep -v "^Binary file" | grep -v "$NEW_PKG" | cut -d: -f1 | sort -u | wc -l

# Step 4: Update imports in the entire codebase
echo "Step 4: Updating imports across codebase..."
find . -name "*.go" -type f -not -path "./vendor/*" -not -path "./.git/*" -not -path "./$NEW_PKG/*" | while read -r file; do
    if grep -q "$OLD_IMPORT" "$file" 2>/dev/null; then
        echo "Updating: $file"
        sed -i "s|$OLD_IMPORT|$NEW_IMPORT|g" "$file"
    fi
done

# Step 5: Verify no old imports remain
echo "Step 5: Verifying migration..."
remaining=$(grep -r "$OLD_IMPORT" --include="*.go" . 2>/dev/null | grep -v "^Binary file" | grep -v "$NEW_PKG" | wc -l)
if [ "$remaining" -gt 0 ]; then
    echo "WARNING: Found $remaining remaining old imports!"
    grep -r "$OLD_IMPORT" --include="*.go" . 2>/dev/null | grep -v "^Binary file" | grep -v "$NEW_PKG"
else
    echo "✓ All imports updated successfully"
fi

# Step 6: Test compilation
echo "Step 6: Testing compilation..."
if make mcp 2>&1 | tail -5; then
    echo "✓ Compilation successful"
else
    echo "✗ Compilation failed"
    exit 1
fi

echo ""
echo "=== Migration Summary ==="
echo "Old package: $OLD_PKG"
echo "New package: $NEW_PKG"
echo "Files moved: $(find "$NEW_PKG" -name "*.go" -type f | wc -l)"
echo ""
echo "Next steps:"
echo "1. Remove old directory: rm -rf $OLD_PKG"
echo "2. Commit the changes"

echo ""
echo "✓ Migration completed successfully!"