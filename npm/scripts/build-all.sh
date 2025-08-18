#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Get the directory where this script is located
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
NPM_DIR="$(dirname "$SCRIPT_DIR")"
PROJECT_ROOT="$(dirname "$NPM_DIR")"

# Change to project root
cd "$PROJECT_ROOT"

echo -e "${GREEN}Building Container Kit MCP Server for all platforms...${NC}"

# Get version from git or use default
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

echo -e "${YELLOW}Version: $VERSION${NC}"
echo -e "${YELLOW}Commit: $GIT_COMMIT${NC}"
echo -e "${YELLOW}Build Time: $BUILD_TIME${NC}"

# Build flags for optimization
LDFLAGS="-s -w -X main.Version=$VERSION -X main.GitCommit=$GIT_COMMIT -X main.BuildTime=$BUILD_TIME"

# Ensure bin directory exists
mkdir -p "$NPM_DIR/bin"

# Function to build for a specific platform
build_platform() {
    local GOOS=$1
    local GOARCH=$2
    local OUTPUT=$3
    local DISPLAY_NAME=$4
    
    echo -e "\n${YELLOW}Building for $DISPLAY_NAME...${NC}"
    
    if GOOS=$GOOS GOARCH=$GOARCH go build \
        -ldflags "$LDFLAGS" \
        -o "$NPM_DIR/bin/$OUTPUT" \
        ./cmd/mcp-server; then
        
        # Get file size
        SIZE=$(du -h "$NPM_DIR/bin/$OUTPUT" | cut -f1)
        echo -e "${GREEN}✓ $DISPLAY_NAME built successfully (${SIZE})${NC}"
        
        # Make executable (except Windows)
        if [[ ! "$OUTPUT" == *.exe ]]; then
            chmod +x "$NPM_DIR/bin/$OUTPUT"
        fi
    else
        echo -e "${RED}✗ Failed to build for $DISPLAY_NAME${NC}"
        return 1
    fi
}

# Track build failures
FAILED_BUILDS=()

# Build for all platforms
echo -e "\n${GREEN}Starting platform builds...${NC}"

# macOS ARM64 (Apple Silicon)
if ! build_platform "darwin" "arm64" "mcp-server-darwin-arm64" "macOS ARM64 (Apple Silicon)"; then
    FAILED_BUILDS+=("macOS ARM64")
fi

# macOS x64 (Intel)
if ! build_platform "darwin" "amd64" "mcp-server-darwin-x64" "macOS x64 (Intel)"; then
    FAILED_BUILDS+=("macOS x64")
fi

# Linux x64
if ! build_platform "linux" "amd64" "mcp-server-linux-x64" "Linux x64"; then
    FAILED_BUILDS+=("Linux x64")
fi

# Linux ARM64
if ! build_platform "linux" "arm64" "mcp-server-linux-arm64" "Linux ARM64"; then
    FAILED_BUILDS+=("Linux ARM64")
fi

# Windows x64
if ! build_platform "windows" "amd64" "mcp-server-win-x64.exe" "Windows x64"; then
    FAILED_BUILDS+=("Windows x64")
fi

# Windows ARM64 (optional, lower priority)
if ! build_platform "windows" "arm64" "mcp-server-win-arm64.exe" "Windows ARM64"; then
    echo -e "${YELLOW}Note: Windows ARM64 build failed (optional platform)${NC}"
fi

# Summary
echo -e "\n${GREEN}==================================${NC}"
echo -e "${GREEN}Build Summary${NC}"
echo -e "${GREEN}==================================${NC}"

# List all built binaries
echo -e "\n${YELLOW}Built binaries:${NC}"
ls -lh "$NPM_DIR/bin/" | grep -E "mcp-server-"

# Total size
TOTAL_SIZE=$(du -sh "$NPM_DIR/bin" | cut -f1)
echo -e "\n${YELLOW}Total size: ${TOTAL_SIZE}${NC}"

# Check for failures
if [ ${#FAILED_BUILDS[@]} -eq 0 ]; then
    echo -e "\n${GREEN}✓ All critical platforms built successfully!${NC}"
    exit 0
else
    echo -e "\n${RED}✗ Some builds failed:${NC}"
    for platform in "${FAILED_BUILDS[@]}"; do
        echo -e "${RED}  - $platform${NC}"
    done
    
    # Only fail if critical platforms failed
    if [[ " ${FAILED_BUILDS[@]} " =~ " macOS ARM64 " ]] || \
       [[ " ${FAILED_BUILDS[@]} " =~ " macOS x64 " ]] || \
       [[ " ${FAILED_BUILDS[@]} " =~ " Linux x64 " ]] || \
       [[ " ${FAILED_BUILDS[@]} " =~ " Windows x64 " ]]; then
        echo -e "\n${RED}Critical platform builds failed. Exiting.${NC}"
        exit 1
    else
        echo -e "\n${YELLOW}Only optional platforms failed. Continuing.${NC}"
        exit 0
    fi
fi