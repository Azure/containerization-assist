#!/bin/bash

# Tool Naming Standardization Script
# This script renames tool files to follow the consistent naming pattern

set -e

echo "=== Tool Naming Standardization ==="
echo ""
echo "This script will rename tool files to follow the pattern:"
echo "  - Atomic tools: tool_name_atomic.go"
echo "  - Non-atomic tools: tool_name.go"
echo ""

# Rename scan tools
echo "Renaming scan tools..."
mv pkg/mcp/internal/scan/scan_image_security.go pkg/mcp/internal/scan/scan_image_security_atomic.go
echo "  ✓ scan_image_security.go -> scan_image_security_atomic.go"

mv pkg/mcp/internal/scan/scan_secrets.go pkg/mcp/internal/scan/scan_secrets_atomic.go
echo "  ✓ scan_secrets.go -> scan_secrets_atomic.go"

# Rename analyze tools
echo ""
echo "Renaming analyze tools..."
mv pkg/mcp/internal/analyze/validate_dockerfile.go pkg/mcp/internal/analyze/validate_dockerfile_atomic.go
echo "  ✓ validate_dockerfile.go -> validate_dockerfile_atomic.go"

# Rename build tools
echo ""
echo "Renaming build tools..."
mv pkg/mcp/internal/build/tag_image.go pkg/mcp/internal/build/tag_image_atomic.go
echo "  ✓ tag_image.go -> tag_image_atomic.go"

mv pkg/mcp/internal/build/pull_image.go pkg/mcp/internal/build/pull_image_atomic.go
echo "  ✓ pull_image.go -> pull_image_atomic.go"

mv pkg/mcp/internal/build/build.go pkg/mcp/internal/build/build_image_atomic.go
echo "  ✓ build.go -> build_image_atomic.go"

# Rename deploy tools
echo ""
echo "Renaming deploy tools..."
mv pkg/mcp/internal/deploy/check_health.go pkg/mcp/internal/deploy/check_health_atomic.go
echo "  ✓ check_health.go -> check_health_atomic.go"

mv pkg/mcp/internal/deploy/deploy_kubernetes.go pkg/mcp/internal/deploy/deploy_kubernetes_atomic.go
echo "  ✓ deploy_kubernetes.go -> deploy_kubernetes_atomic.go"

# Special case: atomic_types.go contains AtomicGenerateManifestsTool
mv pkg/mcp/internal/deploy/atomic_types.go pkg/mcp/internal/deploy/generate_manifests_atomic.go
echo "  ✓ atomic_types.go -> generate_manifests_atomic.go"

echo ""
echo "=== File Renaming Complete ==="
echo ""

# Update imports that might be affected
echo "Updating imports..."

# Update any imports that reference the old file names
find pkg/mcp -name "*.go" -type f -exec grep -l "atomic_types" {} \; | while read file; do
    sed -i 's/atomic_types/generate_manifests_atomic/g' "$file"
    echo "  ✓ Updated imports in $file"
done

echo ""
echo "=== Import Updates Complete ==="
echo ""

# Create a summary of the changes
echo "=== Summary ==="
echo ""
echo "Files renamed: 9"
echo ""
echo "Next steps:"
echo "1. Review the changes with 'git status'"
echo "2. Run tests to ensure nothing is broken"
echo "3. Update any documentation that references these files"
echo "4. Consider consolidating duplicate tool implementations (e.g., BuildImageTool vs AtomicBuildImageTool)"
