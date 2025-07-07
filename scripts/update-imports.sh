#!/bin/bash

# Update imports across the codebase for moved packages
echo "Updating import paths for moved packages..."

# Update imports for moved packages - only internal packages should import internal/
find ./pkg/mcp -name "*.go" -type f -exec sed -i.bak \
  -e 's|github.com/Azure/container-kit/pkg/mcp/constants|github.com/Azure/container-kit/pkg/mcp/internal/constants|g' \
  -e 's|github.com/Azure/container-kit/pkg/mcp/common|github.com/Azure/container-kit/pkg/mcp/internal/common|g' \
  -e 's|"github.com/Azure/container-kit/pkg/mcp/utils"|"github.com/Azure/container-kit/pkg/mcp/internal/utils"|g' \
  -e 's|"github.com/Azure/container-kit/pkg/mcp/validation"|"github.com/Azure/container-kit/pkg/mcp/internal/validation"|g' \
  {} \;

# Update external packages to use public APIs instead of internal packages
find ./pkg/core ./cmd ./test -name "*.go" -type f -exec sed -i.bak \
  -e 's|github.com/Azure/container-kit/pkg/mcp/internal/utils|github.com/Azure/container-kit/pkg/mcp|g' \
  -e 's|github.com/Azure/container-kit/pkg/mcp/internal/validation/core|github.com/Azure/container-kit/pkg/mcp|g' \
  -e 's|github.com/Azure/container-kit/pkg/mcp/internal/validation/validators|github.com/Azure/container-kit/pkg/mcp|g' \
  {} \;

echo "Running goimports to clean up..."
goimports -w $(find . -name "*.go" -type f)

echo "Removing backup files..."
find . -name "*.go.bak" -delete

echo "Import path updates complete."
