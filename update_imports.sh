#!/bin/bash

# Script to update import paths from old to new three-layer architecture
# This script systematically replaces old import paths with new ones

echo "Starting import path updates for three-layer architecture..."

# Update pkg/mcp/api imports to pkg/mcp/application/api
echo "Updating pkg/mcp/api imports..."
find /home/tng/workspace/container-kit -name "*.go" -type f -exec sed -i 's|"github.com/Azure/container-kit/pkg/mcp/api"|"github.com/Azure/container-kit/pkg/mcp/application/api"|g' {} \;

# Update pkg/mcp/core imports to pkg/mcp/application/core
echo "Updating pkg/mcp/core imports..."
find /home/tng/workspace/container-kit -name "*.go" -type f -exec sed -i 's|"github.com/Azure/container-kit/pkg/mcp/core"|"github.com/Azure/container-kit/pkg/mcp/application/core"|g' {} \;

# Update pkg/mcp/session imports to pkg/mcp/domain/session
echo "Updating pkg/mcp/session imports..."
find /home/tng/workspace/container-kit -name "*.go" -type f -exec sed -i 's|"github.com/Azure/container-kit/pkg/mcp/session"|"github.com/Azure/container-kit/pkg/mcp/domain/session"|g' {} \;

# Update pkg/mcp/tools imports to pkg/mcp/application/commands
echo "Updating pkg/mcp/tools imports..."
find /home/tng/workspace/container-kit -name "*.go" -type f -exec sed -i 's|"github.com/Azure/container-kit/pkg/mcp/tools/[^"]*"|"github.com/Azure/container-kit/pkg/mcp/application/commands"|g' {} \;

# Update pkg/mcp/services imports to pkg/mcp/application/services
echo "Updating pkg/mcp/services imports..."
find /home/tng/workspace/container-kit -name "*.go" -type f -exec sed -i 's|"github.com/Azure/container-kit/pkg/mcp/services"|"github.com/Azure/container-kit/pkg/mcp/application/services"|g' {} \;

# Update pkg/mcp/transport imports to pkg/mcp/infra/transport
echo "Updating pkg/mcp/transport imports..."
find /home/tng/workspace/container-kit -name "*.go" -type f -exec sed -i 's|"github.com/Azure/container-kit/pkg/mcp/transport"|"github.com/Azure/container-kit/pkg/mcp/infra/transport"|g' {} \;

# Update pkg/mcp/workflow imports to pkg/mcp/application/workflows
echo "Updating pkg/mcp/workflow imports..."
find /home/tng/workspace/container-kit -name "*.go" -type f -exec sed -i 's|"github.com/Azure/container-kit/pkg/mcp/workflow"|"github.com/Azure/container-kit/pkg/mcp/application/workflows"|g' {} \;

echo "Import path updates completed!"
echo "You may need to run 'go mod tidy' to update dependencies."
