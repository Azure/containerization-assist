#!/bin/bash

# Script to fix mcp imports in analyze package to prevent import cycles

echo "Fixing imports in analyze package..."

# List of files that need fixing
files=(
    "pkg/mcp/internal/analyze/analyze_repository.go"
    "pkg/mcp/internal/analyze/validate_dockerfile_atomic.go"
    "pkg/mcp/internal/analyze/analyze_simple.go"
    "pkg/mcp/internal/analyze/generate_dockerfile.go"
    "pkg/mcp/internal/analyze/analyze_repository_atomic.go"
    "pkg/mcp/internal/analyze/generate_dockerfile_enhanced.go"
)

for file in "${files[@]}"; do
    echo "Processing $file..."

    # Replace the mcp import with mcptypes
    sed -i 's|mcp "github.com/Azure/container-kit/pkg/mcp"|mcptypes "github.com/Azure/container-kit/pkg/mcp/types"|g' "$file"

    # Replace mcp.ToolMetadata with mcptypes.ToolMetadata
    sed -i 's/mcp\.ToolMetadata/mcptypes.ToolMetadata/g' "$file"

    # Replace mcp.ToolExample with mcptypes.ToolExample
    sed -i 's/mcp\.ToolExample/mcptypes.ToolExample/g' "$file"
done

echo "Running goimports to clean up..."
goimports -w pkg/mcp/internal/analyze/

echo "Done!"
