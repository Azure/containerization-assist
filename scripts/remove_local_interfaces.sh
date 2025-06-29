#!/bin/bash

echo "Phase 3: Removing local interface workarounds..."

# Step 1: Update core package to use main interfaces
echo "Step 3.1: Updating core package..."

# First, let's check what needs to be updated
echo "Files using InternalTransport or InternalRequestHandler:"
grep -l "InternalTransport\|InternalRequestHandler" pkg/mcp/internal/core/*.go

# Update server.go to use main interfaces
echo "Updating server.go..."
sed -i 's/transport InternalTransport/transport mcp.Transport/g' pkg/mcp/internal/core/server.go
sed -i 's/SetTransport(transport InternalTransport)/SetTransport(transport mcp.Transport)/g' pkg/mcp/internal/core/server.go

# Update transport_adapter.go
echo "Updating transport_adapter.go..."
sed -i 's/transport InternalTransport/transport mcp.Transport/g' pkg/mcp/internal/core/transport_adapter.go
sed -i 's/handler InternalRequestHandler/handler mcp.RequestHandler/g' pkg/mcp/internal/core/transport_adapter.go

# Update gomcp_manager.go
echo "Updating gomcp_manager.go..."
sed -i 's/transport InternalTransport/transport mcp.Transport/g' pkg/mcp/internal/core/gomcp_manager.go

# Add mcp import where needed
echo "Adding mcp imports..."
for file in pkg/mcp/internal/core/server.go pkg/mcp/internal/core/transport_adapter.go pkg/mcp/internal/core/gomcp_manager.go; do
    if ! grep -q '"github.com/Azure/container-kit/pkg/mcp"' "$file"; then
        # Add import after package declaration
        sed -i '/^package core/a\\nimport mcp "github.com/Azure/container-kit/pkg/mcp"' "$file"
    fi
done

echo "Running goimports..."
goimports -w pkg/mcp/internal/core/

echo "Done with Step 3.1!"
