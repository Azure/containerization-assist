#!/bin/bash

# Create platform-specific npm packages for containerization-assist-mcp

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
NPM_DIR="$SCRIPT_DIR/.."
PACKAGES_DIR="$NPM_DIR/platform-packages"

# Read version from main package.json
VERSION=$(node -p "require('$NPM_DIR/package.json').version")

echo "Creating platform-specific packages for version $VERSION..."

# Clean up old packages
rm -rf "$PACKAGES_DIR"
mkdir -p "$PACKAGES_DIR"

# Define platforms
declare -a platforms=(
  "darwin-x64:darwin:x64"
  "darwin-arm64:darwin:arm64"
  "linux-x64:linux:x64"
  "linux-arm64:linux:arm64"
  "win32-x64:win32:x64"
  "win32-arm64:win32:arm64"
)

# Create each platform package
for platform_spec in "${platforms[@]}"; do
  IFS=':' read -r dir_name os cpu <<< "$platform_spec"
  
  PACKAGE_NAME="@thgamble/containerization-assist-mcp-$dir_name"
  PACKAGE_DIR="$PACKAGES_DIR/$dir_name"
  
  echo "Creating package: $PACKAGE_NAME"
  
  # Create package directory
  mkdir -p "$PACKAGE_DIR/bin"
  
  # Copy the binary
  if [ -d "$NPM_DIR/bin/$dir_name" ]; then
    cp -r "$NPM_DIR/bin/$dir_name" "$PACKAGE_DIR/bin/"
  else
    echo "  ⚠ Warning: Binary not found for $dir_name"
    continue
  fi
  
  # Create package.json for platform package
  cat > "$PACKAGE_DIR/package.json" << EOF
{
  "name": "$PACKAGE_NAME",
  "version": "$VERSION",
  "description": "Platform-specific binary for Containerization Assist MCP Server ($dir_name)",
  "keywords": ["mcp", "containerization", "internal"],
  "homepage": "https://github.com/Azure/containerization-assist#readme",
  "repository": {
    "type": "git",
    "url": "git+https://github.com/Azure/containerization-assist.git"
  },
  "license": "MIT",
  "author": "Microsoft Azure",
  "files": ["bin/"],
  "os": ["$os"],
  "cpu": ["$cpu"],
  "publishConfig": {
    "access": "public",
    "registry": "https://registry.npmjs.org/"
  }
}
EOF

  # Create README
  cat > "$PACKAGE_DIR/README.md" << EOF
# $PACKAGE_NAME

Platform-specific binary package for Containerization Assist MCP Server.

This package contains the binary for: **$os** on **$cpu**

This is automatically installed as an optional dependency of the main package.

## Main Package

Install the main package instead:
\`\`\`bash
npm install @thgamble/containerization-assist-mcp
\`\`\`
EOF

  echo "  ✓ Created $PACKAGE_NAME"
done

echo ""
echo "✅ Platform packages created successfully!"
echo ""
echo "Package sizes:"
for dir in "$PACKAGES_DIR"/*; do
  if [ -d "$dir" ]; then
    size=$(du -sh "$dir" | cut -f1)
    name=$(basename "$dir")
    echo "  $name: $size"
  fi
done