#!/bin/bash

# Build Container Kit MCP binary for current platform only

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$SCRIPT_DIR/../.."
NPM_DIR="$SCRIPT_DIR/.."

echo "Building Container Kit MCP binary for current platform..."

# Detect current platform
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

# Map to Go's naming
case "$OS" in
  "darwin")
    GOOS="darwin"
    ;;
  "linux")
    GOOS="linux"
    ;;
  "mingw"*|"msys"*|"cygwin"*)
    GOOS="windows"
    ;;
  *)
    echo "Unsupported OS: $OS"
    exit 1
    ;;
esac

case "$ARCH" in
  "x86_64"|"amd64")
    GOARCH="amd64"
    DIR_ARCH="x64"
    ;;
  "arm64"|"aarch64")
    GOARCH="arm64"
    DIR_ARCH="arm64"
    ;;
  *)
    echo "Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

# Determine directory name
if [ "$GOOS" = "windows" ]; then
  DIR_NAME="win32-$DIR_ARCH"
else
  DIR_NAME="$GOOS-$DIR_ARCH"
fi

# Create platform directory
mkdir -p "$NPM_DIR/bin/$DIR_NAME"

# Set output binary name
output_name="container-kit-mcp"
if [ "$GOOS" = "windows" ]; then
  output_name="${output_name}.exe"
fi

echo "Platform: $DIR_NAME (GOOS=$GOOS GOARCH=$GOARCH)"

# Build the binary
cd "$PROJECT_ROOT"
go build \
  -ldflags="-s -w" \
  -o "$NPM_DIR/bin/$DIR_NAME/$output_name" \
  ./cmd/mcp-server

# Make it executable (for Unix platforms)
if [ "$GOOS" != "windows" ]; then
  chmod +x "$NPM_DIR/bin/$DIR_NAME/$output_name"
fi

echo "✓ Built $DIR_NAME/$output_name"
echo ""
echo "Binary location:"
ls -la "$NPM_DIR/bin/$DIR_NAME/$output_name"