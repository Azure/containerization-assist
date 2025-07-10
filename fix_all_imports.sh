#!/bin/bash

# Script to fix ALL imports across the entire codebase after three-layer migration

echo "Fixing imports across entire codebase..."

# Function to fix imports in a directory
fix_imports() {
    local dir=$1
    echo "Processing $dir..."
    
    find "$dir" -name "*.go" -type f | while read -r file; do
        # Create a temporary file
        tmp_file="${file}.tmp"
        
        # Apply all import fixes
        sed -E \
            -e 's|"github.com/Azure/container-kit/pkg/mcp/errors"|"github.com/Azure/container-kit/pkg/mcp/domain/errors"|g' \
            -e 's|"github.com/Azure/container-kit/pkg/mcp/errorcodes"|"github.com/Azure/container-kit/pkg/mcp/domain/errors"|g' \
            -e 's|"github.com/Azure/container-kit/pkg/mcp/config"|"github.com/Azure/container-kit/pkg/mcp/domain/config"|g' \
            -e 's|"github.com/Azure/container-kit/pkg/mcp/session"|"github.com/Azure/container-kit/pkg/mcp/domain/session"|g' \
            -e 's|"github.com/Azure/container-kit/pkg/mcp/security"|"github.com/Azure/container-kit/pkg/mcp/domain/security"|g' \
            -e 's|"github.com/Azure/container-kit/pkg/mcp/analyze"|"github.com/Azure/container-kit/pkg/mcp/domain/containerization"|g' \
            -e 's|"github.com/Azure/container-kit/pkg/mcp/build"|"github.com/Azure/container-kit/pkg/mcp/domain/containerization"|g' \
            -e 's|"github.com/Azure/container-kit/pkg/mcp/deploy"|"github.com/Azure/container-kit/pkg/mcp/domain/containerization"|g' \
            -e 's|"github.com/Azure/container-kit/pkg/mcp/scan"|"github.com/Azure/container-kit/pkg/mcp/domain/containerization"|g' \
            -e 's|"github.com/Azure/container-kit/pkg/mcp/domaintypes"|"github.com/Azure/container-kit/pkg/mcp/domain/types"|g' \
            -e 's|"github.com/Azure/container-kit/pkg/mcp/api"|"github.com/Azure/container-kit/pkg/mcp/application/api"|g' \
            -e 's|"github.com/Azure/container-kit/pkg/mcp/commands"|"github.com/Azure/container-kit/pkg/mcp/application/commands"|g' \
            -e 's|"github.com/Azure/container-kit/pkg/mcp/tools"|"github.com/Azure/container-kit/pkg/mcp/application/tools"|g' \
            -e 's|"github.com/Azure/container-kit/pkg/mcp/workflows"|"github.com/Azure/container-kit/pkg/mcp/application/workflows"|g' \
            -e 's|"github.com/Azure/container-kit/pkg/mcp/knowledge"|"github.com/Azure/container-kit/pkg/mcp/application/knowledge"|g' \
            -e 's|"github.com/Azure/container-kit/pkg/mcp/shared"|"github.com/Azure/container-kit/pkg/mcp/application/state"|g' \
            -e 's|"github.com/Azure/container-kit/pkg/mcp/appstate"|"github.com/Azure/container-kit/pkg/mcp/application/state"|g' \
            -e 's|"github.com/Azure/container-kit/pkg/mcp/logging"|"github.com/Azure/container-kit/pkg/mcp/infra/internal/logging"|g' \
            -e 's|"github.com/Azure/container-kit/pkg/mcp/retry"|"github.com/Azure/container-kit/pkg/mcp/infra/retry"|g' \
            "$file" > "$tmp_file"
        
        # Check if sed made any changes
        if ! diff -q "$file" "$tmp_file" >/dev/null 2>&1; then
            mv "$tmp_file" "$file"
            echo "  Fixed: $file"
        else
            rm "$tmp_file"
        fi
    done
}

# Fix imports in ALL directories  
echo "=== Fixing MCP packages ==="
fix_imports "pkg/mcp"

echo ""
echo "=== Fixing other packages ==="
fix_imports "pkg/ai"
fix_imports "pkg/core"
fix_imports "pkg/common"
fix_imports "pkg/integrations"  
fix_imports "pkg/release"
fix_imports "pkg/utils"

echo ""
echo "=== Fixing cmd and test ==="
fix_imports "cmd"
fix_imports "test"

# Run goimports on all Go files
echo ""
echo "Running goimports to clean up..."
find . -name "*.go" -type f -not -path "./vendor/*" -not -path "./.git/*" -exec goimports -w {} \;

echo ""
echo "Import fixing complete!"