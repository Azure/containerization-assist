#!/bin/bash

# Build optimized Containerization Assist MCP binaries for all supported platforms

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$SCRIPT_DIR/../.."
NPM_DIR="$SCRIPT_DIR/.."

echo "Building optimized Containerization Assist MCP binaries..."

# Define platforms to build
declare -a platforms=(
  "darwin-x64:darwin:amd64"
  "darwin-arm64:darwin:arm64"
  "linux-x64:linux:amd64"
  "linux-arm64:linux:arm64"
  "win32-x64:windows:amd64"
  "win32-arm64:windows:arm64"
)

# Create bin directory structure
mkdir -p "$NPM_DIR/bin"

# Check if UPX is available
HAS_UPX=false
if command -v upx &> /dev/null; then
  HAS_UPX=true
  echo "✓ UPX detected, will compress binaries"
else
  echo "⚠ UPX not found, skipping compression (install with: apt-get install upx-ucl)"
fi

# Build for each platform
for platform_spec in "${platforms[@]}"; do
  IFS=':' read -r dir_name goos goarch <<< "$platform_spec"
  
  echo "Building optimized for $dir_name (GOOS=$goos GOARCH=$goarch)..."
  
  # Create platform directory
  mkdir -p "$NPM_DIR/bin/$dir_name"
  
  # Set output binary name
  output_name="containerization-assist-mcp"
  if [ "$goos" = "windows" ]; then
    output_name="${output_name}.exe"
  fi
  
  # Build the binary with maximum optimization
  cd "$PROJECT_ROOT"
  
  # More aggressive build flags for size reduction
  CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch go build \
    -trimpath \
    -ldflags="-s -w -extldflags '-static'" \
    -a \
    -installsuffix cgo \
    -o "$NPM_DIR/bin/$dir_name/$output_name" \
    .
  
  # Get size before compression
  SIZE_BEFORE=$(du -h "$NPM_DIR/bin/$dir_name/$output_name" | cut -f1)
  
  # Compress with UPX if available (skip for macOS due to code signing issues)
  if [ "$HAS_UPX" = true ] && [ "$goos" != "darwin" ]; then
    echo "  Compressing with UPX..."
    upx --best --lzma "$NPM_DIR/bin/$dir_name/$output_name" 2>/dev/null || \
    upx -9 "$NPM_DIR/bin/$dir_name/$output_name" 2>/dev/null || \
    echo "  ⚠ UPX compression failed, keeping uncompressed binary"
  fi
  
  # Get size after
  SIZE_AFTER=$(du -h "$NPM_DIR/bin/$dir_name/$output_name" | cut -f1)
  
  # Make it executable (for Unix platforms)
  if [ "$goos" != "windows" ]; then
    chmod +x "$NPM_DIR/bin/$dir_name/$output_name"
  fi
  
  echo "  ✓ Built $dir_name/$output_name ($SIZE_BEFORE -> $SIZE_AFTER)"
done

echo ""
echo "✅ All binaries built and optimized!"
echo ""
echo "Size summary:"
du -sh "$NPM_DIR/bin/"*/* | sort -rh
echo ""
echo "Total package size:"
du -sh "$NPM_DIR/bin"