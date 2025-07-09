#!/bin/bash

# Script to check if generated schemas are up to date

set -e

echo "🔍 Checking if generated schemas are up to date..."

# Save current state
ORIGINAL_DIR=$(pwd)
cd "$(git rev-parse --show-toplevel)"

# Create temporary directory for comparison
TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

# Find all generated schema files and copy them to temp
echo "📁 Backing up current generated schemas..."
find pkg/mcp/application/internal -name "generated_*.go" -type f | while read -r file; do
    mkdir -p "$TEMP_DIR/$(dirname "$file")"
    cp "$file" "$TEMP_DIR/$file"
done

# Regenerate schemas
echo "🔨 Regenerating schemas..."
make generate > /dev/null 2>&1

# Compare with saved versions
echo "📊 Comparing schemas..."
CHANGES_FOUND=false
find pkg/mcp/application/internal -name "generated_*.go" -type f | while read -r file; do
    if [ -f "$TEMP_DIR/$file" ]; then
        if ! diff -q "$file" "$TEMP_DIR/$file" > /dev/null; then
            echo "❌ Schema out of date: $file"
            CHANGES_FOUND=true
            # Show the diff
            echo "Diff:"
            diff -u "$TEMP_DIR/$file" "$file" || true
            echo ""
        fi
    else
        echo "❌ New schema file generated: $file"
        CHANGES_FOUND=true
    fi
done

# Check for deleted schemas
find "$TEMP_DIR/pkg/mcp/application/internal" -name "generated_*.go" -type f 2>/dev/null | while read -r temp_file; do
    original_file="${temp_file#$TEMP_DIR/}"
    if [ ! -f "$original_file" ]; then
        echo "❌ Schema file deleted: $original_file"
        CHANGES_FOUND=true
    fi
done

# Restore original schemas for now
echo "🔄 Restoring original schemas..."
find pkg/mcp/application/internal -name "generated_*.go" -type f -delete
find "$TEMP_DIR/pkg/mcp/application/internal" -name "generated_*.go" -type f 2>/dev/null | while read -r temp_file; do
    original_file="${temp_file#$TEMP_DIR/}"
    mkdir -p "$(dirname "$original_file")"
    cp "$temp_file" "$original_file"
done

cd "$ORIGINAL_DIR"

if [ "$CHANGES_FOUND" = true ]; then
    echo ""
    echo "⚠️  Generated schemas are out of date!"
    echo ""
    echo "To fix this, run:"
    echo "  make generate"
    echo "  git add pkg/mcp/application/internal/**/generated_*.go"
    echo "  git commit -m 'chore: update generated schemas'"
    exit 1
else
    echo "✅ All generated schemas are up to date!"
    exit 0
fi
