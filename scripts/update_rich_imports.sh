#!/bin/bash

# Script to update imports from errors/rich to errors package

echo "=== Updating Rich Error Imports ==="

# Files that need import updates
FILES=$(grep -r "github.com/Azure/container-kit/pkg/mcp/errors/rich" /home/tng/workspace/beta/pkg/mcp --include="*.go" | grep -v "_test.go" | cut -d: -f1 | sort -u)

for file in $FILES; do
    echo "Updating $file..."

    # Replace the import
    sed -i 's|"github.com/Azure/container-kit/pkg/mcp/errors/rich"|"github.com/Azure/container-kit/pkg/mcp/errors"|g' "$file"

    # Replace rich.RichError with errors.RichError
    sed -i 's/\brich\.RichError\b/errors.RichError/g' "$file"
    sed -i 's/\brich\.NewError\b/errors.NewError/g' "$file"
    sed -i 's/\brich\.ErrorBuilder\b/errors.ErrorBuilder/g' "$file"
    sed -i 's/\brich\.ErrorCode\b/errors.ErrorCode/g' "$file"
    sed -i 's/\brich\.ErrorType\b/errors.ErrorType/g' "$file"
    sed -i 's/\brich\.ErrorSeverity\b/errors.ErrorSeverity/g' "$file"
    sed -i 's/\brich\.ErrorContext\b/errors.ErrorContext/g' "$file"

    # Replace constants
    sed -i 's/\brich\.Code/errors.Code/g' "$file"
    sed -i 's/\brich\.ErrType/errors.ErrType/g' "$file"
    sed -i 's/\brich\.Severity/errors.Severity/g' "$file"
done

echo "Done updating imports!"
