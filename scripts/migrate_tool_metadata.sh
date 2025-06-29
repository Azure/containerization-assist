#!/bin/bash

# Script to migrate tool metadata types from mcptypes to mcp

echo "Starting migration of tool metadata types..."

# Step 1: Replace mcptypes.ToolMetadata with mcp.ToolMetadata
echo "Migrating ToolMetadata..."
find pkg/mcp/internal -name "*.go" -exec sed -i 's/mcptypes\.ToolMetadata/mcp.ToolMetadata/g' {} \;

# Step 2: Replace mcptypes.ToolExample with mcp.ToolExample
echo "Migrating ToolExample..."
find pkg/mcp/internal -name "*.go" -exec sed -i 's/mcptypes\.ToolExample/mcp.ToolExample/g' {} \;

# Step 3: Replace mcptypes.ToolFactory with mcp.ToolFactory
echo "Migrating ToolFactory..."
find pkg/mcp/internal -name "*.go" -exec sed -i 's/mcptypes\.ToolFactory/mcp.ToolFactory/g' {} \;

# Step 4: Replace mcptypes.ArgConverter with mcp.ArgConverter
echo "Migrating ArgConverter..."
find pkg/mcp/internal -name "*.go" -exec sed -i 's/mcptypes\.ArgConverter/mcp.ArgConverter/g' {} \;

# Step 5: Replace mcptypes.ParameterDefinition with mcp.ParameterDefinition
echo "Migrating ParameterDefinition..."
find pkg/mcp/internal -name "*.go" -exec sed -i 's/mcptypes\.ParameterDefinition/mcp.ParameterDefinition/g' {} \;

# Step 6: Add mcp import where needed (will be cleaned up by goimports)
echo "Adding mcp imports..."
find pkg/mcp/internal -name "*.go" -exec grep -l "mcp\.Tool" {} \; | while read file; do
    if ! grep -q '"github.com/Azure/container-kit/pkg/mcp"' "$file"; then
        # Add import after package declaration
        sed -i '/^package /a\\nimport mcp "github.com/Azure/container-kit/pkg/mcp"' "$file"
    fi
done

echo "Migration complete. Running goimports to clean up..."
goimports -w pkg/mcp/internal/

echo "Done!"
