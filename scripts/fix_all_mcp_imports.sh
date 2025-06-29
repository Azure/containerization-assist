#!/bin/bash

# Script to fix all mcp imports to prevent import cycles

echo "Fixing all mcp imports to use mcptypes..."

# Find all Go files that import mcp directly (excluding test files initially)
files=$(find pkg/mcp/internal -name "*.go" -exec grep -l "\"github.com/Azure/container-kit/pkg/mcp\"" {} \; | sort | uniq)

for file in $files; do
    echo "Processing $file..."

    # Check if the file already imports mcptypes
    if grep -q "mcptypes \"github.com/Azure/container-kit/pkg/mcp/types\"" "$file"; then
        # Already has mcptypes import, just replace mcp usage
        sed -i 's|mcp "github.com/Azure/container-kit/pkg/mcp"|// mcp import removed - using mcptypes|g' "$file"
    else
        # Replace the mcp import with mcptypes
        sed -i 's|mcp "github.com/Azure/container-kit/pkg/mcp"|mcptypes "github.com/Azure/container-kit/pkg/mcp/types"|g' "$file"
    fi

    # Replace mcp.ToolMetadata with mcptypes.ToolMetadata
    sed -i 's/mcp\.ToolMetadata/mcptypes.ToolMetadata/g' "$file"

    # Replace mcp.ToolExample with mcptypes.ToolExample
    sed -i 's/mcp\.ToolExample/mcptypes.ToolExample/g' "$file"

    # Replace mcp.ToolFactory with mcptypes.ToolFactory
    sed -i 's/mcp\.ToolFactory/mcptypes.ToolFactory/g' "$file"

    # Replace mcp.ArgConverter with mcptypes.ArgConverter
    sed -i 's/mcp\.ArgConverter/mcptypes.ArgConverter/g' "$file"

    # Replace mcp.Tool with mcptypes.Tool
    sed -i 's/mcp\.Tool/mcptypes.Tool/g' "$file"

    # Replace mcp.StronglyTypedToolFactory with mcptypes.StronglyTypedToolFactory
    sed -i 's/mcp\.StronglyTypedToolFactory/mcptypes.StronglyTypedToolFactory/g' "$file"
done

echo "Running goimports to clean up..."
goimports -w pkg/mcp/internal/

echo "Done!"
