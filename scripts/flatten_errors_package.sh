#!/bin/bash

# Flatten domain/errors package to top-level errors package
# This script moves files and updates all imports

set -e

echo "=== Flattening domain/errors package ==="

# Base directories
OLD_PKG="pkg/mcp/domain/errors"
NEW_PKG="pkg/mcp/errors"
OLD_IMPORT="github.com/Azure/container-kit/pkg/mcp/domain/errors"
NEW_IMPORT="github.com/Azure/container-kit/pkg/mcp/errors"

# Step 1: Copy files to new location
echo "Step 1: Copying files to new location..."
cp -r "$OLD_PKG"/* "$NEW_PKG/"

# Step 2: Update package declarations in the new files
echo "Step 2: Updating package declarations..."
find "$NEW_PKG" -name "*.go" -type f | while read -r file; do
    # Skip test files for now
    if [[ "$file" == *"_test.go" ]]; then
        continue
    fi

    # Update package declaration if it's in a subdirectory
    if [[ "$file" == *"/codes/"* ]]; then
        # Files in codes/ subdirectory should keep their package
        continue
    else
        # Update package declaration from "package errors" to "package errors"
        sed -i 's/^package errors$/package errors/' "$file"
    fi
done

# Step 3: Update imports in the entire codebase
echo "Step 3: Updating imports across codebase..."
echo "Found files to update:"
grep -r "$OLD_IMPORT" --include="*.go" . 2>/dev/null | grep -v "^Binary file" | cut -d: -f1 | sort -u | wc -l

# Update imports
find . -name "*.go" -type f -not -path "./vendor/*" -not -path "./.git/*" | while read -r file; do
    if grep -q "$OLD_IMPORT" "$file" 2>/dev/null; then
        echo "Updating: $file"
        sed -i "s|$OLD_IMPORT|$NEW_IMPORT|g" "$file"
    fi
done

# Step 4: Handle the codes subdirectory specially
echo "Step 4: Handling codes subdirectory..."
OLD_CODES_IMPORT="github.com/Azure/container-kit/pkg/mcp/domain/errors/codes"
NEW_CODES_IMPORT="github.com/Azure/container-kit/pkg/mcp/errors/codes"

find . -name "*.go" -type f -not -path "./vendor/*" -not -path "./.git/*" | while read -r file; do
    if grep -q "$OLD_CODES_IMPORT" "$file" 2>/dev/null; then
        echo "Updating codes import in: $file"
        sed -i "s|$OLD_CODES_IMPORT|$NEW_CODES_IMPORT|g" "$file"
    fi
done

# Step 5: Verify no old imports remain
echo "Step 5: Verifying migration..."
if grep -r "$OLD_IMPORT" --include="*.go" . 2>/dev/null | grep -v "^Binary file" | grep -v "$NEW_PKG"; then
    echo "WARNING: Found remaining old imports!"
    exit 1
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
echo "1. Run tests to ensure nothing broke"
echo "2. Remove old directory: rm -rf $OLD_PKG"
echo "3. Commit the changes"

echo ""
echo "✓ Migration completed successfully!"
