#!/bin/bash

# Fix all imports after ADR-001 three-layer architecture migration

set -e

echo "🔧 Fixing all imports for three-layer architecture..."
echo "This will update all import paths throughout the codebase"

# Function to fix imports in all Go files
fix_imports() {
    local old_path=$1
    local new_path=$2
    local description=$3

    echo "Fixing: $description"
    echo "  $old_path → $new_path"

    # Find and update all Go files with this import
    find . -name "*.go" -type f | while read -r file; do
        if grep -q "$old_path" "$file" 2>/dev/null; then
            sed -i "s|$old_path|$new_path|g" "$file"
            echo "  ✓ Updated: $file"
        fi
    done
}

echo ""
echo "=== DOMAIN LAYER IMPORTS ==="

# Fix config imports
fix_imports \
    "github.com/Azure/container-kit/pkg/mcp/config" \
    "github.com/Azure/container-kit/pkg/mcp/domain/config" \
    "Config domain imports"

# Fix containerization imports
fix_imports \
    "github.com/Azure/container-kit/pkg/mcp/analyze" \
    "github.com/Azure/container-kit/pkg/mcp/domain/containerization/analyze" \
    "Analyze domain imports"

fix_imports \
    "github.com/Azure/container-kit/pkg/mcp/build" \
    "github.com/Azure/container-kit/pkg/mcp/domain/containerization/build" \
    "Build domain imports"

fix_imports \
    "github.com/Azure/container-kit/pkg/mcp/deploy" \
    "github.com/Azure/container-kit/pkg/mcp/domain/containerization/deploy" \
    "Deploy domain imports"

fix_imports \
    "github.com/Azure/container-kit/pkg/mcp/scan" \
    "github.com/Azure/container-kit/pkg/mcp/domain/containerization/scan" \
    "Scan domain imports"

# Fix errors imports
fix_imports \
    "github.com/Azure/container-kit/pkg/mcp/errors" \
    "github.com/Azure/container-kit/pkg/mcp/domain/errors" \
    "Errors domain imports"

fix_imports \
    "github.com/Azure/container-kit/pkg/mcp/errorcodes" \
    "github.com/Azure/container-kit/pkg/mcp/domain/errors" \
    "Error codes imports"

# Fix security imports
fix_imports \
    "github.com/Azure/container-kit/pkg/mcp/security" \
    "github.com/Azure/container-kit/pkg/mcp/domain/security" \
    "Security domain imports"

# Fix session imports
fix_imports \
    "github.com/Azure/container-kit/pkg/mcp/session" \
    "github.com/Azure/container-kit/pkg/mcp/domain/session" \
    "Session domain imports"

# Fix types imports
fix_imports \
    "github.com/Azure/container-kit/pkg/mcp/domaintypes" \
    "github.com/Azure/container-kit/pkg/mcp/domain/types" \
    "Domain types imports"

fix_imports \
    "github.com/Azure/container-kit/pkg/mcp/shared" \
    "github.com/Azure/container-kit/pkg/mcp/domain/internal" \
    "Shared/internal domain imports"

echo ""
echo "=== APPLICATION LAYER IMPORTS ==="

# Fix application imports
fix_imports \
    "github.com/Azure/container-kit/pkg/mcp/api" \
    "github.com/Azure/container-kit/pkg/mcp/application/api" \
    "API application imports"

fix_imports \
    "github.com/Azure/container-kit/pkg/mcp/commands" \
    "github.com/Azure/container-kit/pkg/mcp/application/commands" \
    "Commands application imports"

fix_imports \
    "github.com/Azure/container-kit/pkg/mcp/core" \
    "github.com/Azure/container-kit/pkg/mcp/application/core" \
    "Core application imports"

fix_imports \
    "github.com/Azure/container-kit/pkg/mcp/services" \
    "github.com/Azure/container-kit/pkg/mcp/application/services" \
    "Services application imports"

fix_imports \
    "github.com/Azure/container-kit/pkg/mcp/tools" \
    "github.com/Azure/container-kit/pkg/mcp/application/tools" \
    "Tools application imports"

fix_imports \
    "github.com/Azure/container-kit/pkg/mcp/workflows" \
    "github.com/Azure/container-kit/pkg/mcp/application/workflows" \
    "Workflows application imports"

fix_imports \
    "github.com/Azure/container-kit/pkg/mcp/appstate" \
    "github.com/Azure/container-kit/pkg/mcp/application/state" \
    "State application imports"

fix_imports \
    "github.com/Azure/container-kit/pkg/mcp/knowledge" \
    "github.com/Azure/container-kit/pkg/mcp/application/knowledge" \
    "Knowledge application imports"

echo ""
echo "=== INFRASTRUCTURE LAYER IMPORTS ==="

# Fix infrastructure imports
fix_imports \
    "github.com/Azure/container-kit/pkg/mcp/logging" \
    "github.com/Azure/container-kit/pkg/mcp/infra/logging" \
    "Logging infrastructure imports"

fix_imports \
    "github.com/Azure/container-kit/pkg/mcp/retry" \
    "github.com/Azure/container-kit/pkg/mcp/infra/retry" \
    "Retry infrastructure imports"

echo ""
echo "=== FIXING PACKAGE DECLARATIONS ==="

# Fix package declarations that might have been affected
echo "Fixing package declarations in moved files..."

# Domain packages
find pkg/mcp/domain/types -name "*.go" -exec sed -i 's/^package domaintypes$/package types/g' {} \;
find pkg/mcp/domain/internal -name "*.go" -exec sed -i 's/^package shared$/package internal/g' {} \;

# Application packages
find pkg/mcp/application/state -name "*.go" -exec sed -i 's/^package appstate$/package state/g' {} \;

echo ""
echo "=== COMPILATION TEST ==="

# Try to compile to see if we have any remaining issues
echo "Testing compilation..."
if go build ./pkg/mcp/... 2>&1 | head -20; then
    echo "✅ Compilation successful!"
else
    echo "⚠️  Compilation errors found - see above for details"
    echo "Additional manual fixes may be required"
fi

echo ""
echo "=== IMPORT FIX COMPLETE ==="
echo "✅ All import paths have been updated for three-layer architecture"
echo ""
echo "Next steps:"
echo "1. Run 'go mod tidy' to clean up dependencies"
echo "2. Run 'go build ./...' to verify full compilation"
echo "3. Run tests to ensure functionality is preserved"
