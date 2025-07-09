#!/bin/bash

# Script to update imports from core to new locations

# Update imports in application files
find pkg/mcp/application -name "*.go" -type f -exec sed -i \
  -e 's|"github.com/Azure/container-kit/pkg/mcp/core/config"|"github.com/Azure/container-kit/pkg/mcp/application/config"|g' \
  -e 's|"github.com/Azure/container-kit/pkg/mcp/core/tools"|"github.com/Azure/container-kit/pkg/mcp/domain/tools"|g' \
  -e 's|"github.com/Azure/container-kit/pkg/mcp/core/types"|"github.com/Azure/container-kit/pkg/mcp/domain/types"|g' \
  {} \;

# Update imports in domain files
find pkg/mcp/domain -name "*.go" -type f -exec sed -i \
  -e 's|"github.com/Azure/container-kit/pkg/mcp/core/config"|"github.com/Azure/container-kit/pkg/mcp/application/config"|g' \
  -e 's|"github.com/Azure/container-kit/pkg/mcp/core/tools"|"github.com/Azure/container-kit/pkg/mcp/domain/tools"|g' \
  -e 's|"github.com/Azure/container-kit/pkg/mcp/core/types"|"github.com/Azure/container-kit/pkg/mcp/domain/types"|g' \
  {} \;

# Update imports throughout the codebase for moved files
find . -path "./vendor" -prune -o -name "*.go" -type f -exec sed -i \
  -e 's|"github.com/Azure/container-kit/pkg/mcp/core/conversation"|"github.com/Azure/container-kit/pkg/mcp/application/conversation"|g' \
  -e 's|"github.com/Azure/container-kit/pkg/mcp/core/state"|"github.com/Azure/container-kit/pkg/mcp/application/state"|g' \
  {} \;

echo "Import updates completed"