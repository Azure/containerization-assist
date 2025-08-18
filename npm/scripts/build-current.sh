#!/bin/bash

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Get the directory where this script is located
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
NPM_DIR="$(dirname "$SCRIPT_DIR")"
PROJECT_ROOT="$(dirname "$NPM_DIR")"

# Change to project root
cd "$PROJECT_ROOT"

echo -e "${GREEN}Building Container Kit MCP Server for current platform...${NC}"

# Detect current platform
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

# Map to Go's naming convention
case "$OS" in
    "darwin") GOOS="darwin" ;;
    "linux") GOOS="linux" ;;
    "mingw"*|"msys"*|"cygwin"*) GOOS="windows" ;;
    *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

case "$ARCH" in
    "x86_64"|"amd64") GOARCH="amd64"; ARCH_NAME="x64" ;;
    "arm64"|"aarch64") GOARCH="arm64"; ARCH_NAME="arm64" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# Determine output binary name
if [ "$GOOS" = "windows" ]; then
    OUTPUT="mcp-server-${GOOS}-${ARCH_NAME}.exe"
else
    OUTPUT="mcp-server-${GOOS}-${ARCH_NAME}"
fi

# Get version info
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

echo -e "${YELLOW}Platform: $GOOS/$GOARCH${NC}"
echo -e "${YELLOW}Output: $OUTPUT${NC}"
echo -e "${YELLOW}Version: $VERSION${NC}"

# Build flags
LDFLAGS="-s -w -X main.Version=$VERSION -X main.GitCommit=$GIT_COMMIT -X main.BuildTime=$BUILD_TIME"

# Ensure bin directory exists
mkdir -p "$NPM_DIR/bin"

# Build
echo -e "\n${YELLOW}Building...${NC}"
go build -ldflags "$LDFLAGS" -o "$NPM_DIR/bin/$OUTPUT" ./cmd/mcp-server

# Make executable (except Windows)
if [[ ! "$OUTPUT" == *.exe ]]; then
    if [ -f "$NPM_DIR/bin/$OUTPUT" ]; then
        chmod +x "$NPM_DIR/bin/$OUTPUT"
    fi
fi

# Create symlink for convenience
ln -sf "$OUTPUT" "$NPM_DIR/bin/mcp-server"

SIZE=$(du -h "$NPM_DIR/bin/$OUTPUT" | cut -f1)
echo -e "${GREEN}✓ Build successful! (${SIZE})${NC}"
echo -e "${GREEN}Binary: $NPM_DIR/bin/$OUTPUT${NC}"