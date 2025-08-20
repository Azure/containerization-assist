#!/bin/bash

# Build Container Kit MCP binaries for all supported platforms

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$SCRIPT_DIR/../.."
NPM_DIR="$SCRIPT_DIR/.."

echo "Building Container Kit MCP binaries for all platforms..."

# Define platforms to build
declare -a platforms=(
  "darwin-x64:darwin:amd64"
  "darwin-arm64:darwin:arm64"
  "linux-x64:linux:amd64"
  "linux-arm64:linux:arm64"
  "win32-x64:windows:amd64"
)

# Create bin directory structure
mkdir -p "$NPM_DIR/bin"

# Build for each platform
for platform_spec in "${platforms[@]}"; do
  IFS=':' read -r dir_name goos goarch <<< "$platform_spec"
  
  echo "Building for $dir_name (GOOS=$goos GOARCH=$goarch)..."
  
  # Create platform directory
  mkdir -p "$NPM_DIR/bin/$dir_name"
  
  # Set output binary name
  output_name="containerization-assist-mcp"
  if [ "$goos" = "windows" ]; then
    output_name="${output_name}.exe"
  fi
  
  # Build the binary
  cd "$PROJECT_ROOT"
  GOOS=$goos GOARCH=$goarch go build \
    -ldflags="-s -w" \
    -o "$NPM_DIR/bin/$dir_name/$output_name" \
    .
  
  # Make it executable (for Unix platforms)
  if [ "$goos" != "windows" ]; then
    chmod +x "$NPM_DIR/bin/$dir_name/$output_name"
  fi
  
  echo "  ✓ Built $dir_name/$output_name"
done

echo ""
echo "✅ All binaries built successfully!"
echo ""
echo "Binary locations:"
ls -la "$NPM_DIR/bin/"*/containerization-assist-mcp* 2>/dev/null || true