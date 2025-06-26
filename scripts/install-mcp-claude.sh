#!/bin/bash
# Install Container Kit MCP Server for Claude Desktop

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Detect OS
OS="$(uname -s)"
case "${OS}" in
    Linux*)     OS_TYPE=Linux;;
    Darwin*)    OS_TYPE=Mac;;
    CYGWIN*|MINGW*|MSYS*) OS_TYPE=Windows;;
    *)          OS_TYPE="UNKNOWN:${OS}"
esac

echo -e "${GREEN}Container Kit MCP Server Installer for Claude Desktop${NC}"
echo "Detected OS: $OS_TYPE"
echo ""

# Check prerequisites
echo "Checking prerequisites..."

if ! command -v go &> /dev/null; then
    echo -e "${RED}Go is not installed. Please install Go 1.21+ first.${NC}"
    exit 1
fi

if ! command -v docker &> /dev/null; then
    echo -e "${YELLOW}Warning: Docker is not installed. Some features will not work.${NC}"
fi

# Get the script directory
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Build the MCP server
echo "Building Container Kit MCP server..."
cd "$PROJECT_ROOT"
go build -tags "mcp" -o container-kit-mcp ./cmd/mcp-server

if [ ! -f "container-kit-mcp" ]; then
    echo -e "${RED}Build failed!${NC}"
    exit 1
fi

# Make it executable
chmod +x container-kit-mcp

echo -e "${GREEN}Build successful!${NC}"

# Determine Claude config location
case "$OS_TYPE" in
    Mac)
        CONFIG_DIR="$HOME/Library/Application Support/Claude"
        ;;
    Linux)
        CONFIG_DIR="$HOME/.config/Claude"
        ;;
    Windows)
        CONFIG_DIR="$APPDATA/Claude"
        ;;
    *)
        echo -e "${RED}Unsupported OS: $OS_TYPE${NC}"
        exit 1
        ;;
esac

CONFIG_FILE="$CONFIG_DIR/claude_desktop_config.json"

# Create config directory if it doesn't exist
mkdir -p "$CONFIG_DIR"

# Check if config file exists
if [ -f "$CONFIG_FILE" ]; then
    echo "Found existing Claude Desktop configuration at: $CONFIG_FILE"
    echo "Creating backup at: $CONFIG_FILE.backup"
    cp "$CONFIG_FILE" "$CONFIG_FILE.backup"
else
    echo "Creating new Claude Desktop configuration..."
    echo '{"mcpServers": {}}' > "$CONFIG_FILE"
fi

# Create the new config entry
MCP_PATH="$PROJECT_ROOT/container-kit-mcp"
echo ""
echo "Adding Container Kit MCP server to Claude Desktop configuration..."

# Use Python to update JSON (more reliable than sed/awk for JSON)
python3 << EOF
import json
import os

config_file = "$CONFIG_FILE"
mcp_path = "$MCP_PATH"

# Read existing config
with open(config_file, 'r') as f:
    config = json.load(f)

# Ensure mcpServers exists
if 'mcpServers' not in config:
    config['mcpServers'] = {}

# Add or update container-kit entry
config['mcpServers']['container-kit'] = {
    "command": mcp_path,
    "args": ["--transport=stdio", "--log-level=info"],
    "env": {
        "HOME": os.environ.get('HOME', '/tmp')
    }
}

# Write updated config
with open(config_file, 'w') as f:
    json.dump(config, f, indent=2)

print("Configuration updated successfully!")
EOF

echo ""
echo -e "${GREEN}Installation complete!${NC}"
echo ""
echo "Next steps:"
echo "1. Restart Claude Desktop"
echo "2. In a new conversation, type: 'Help me containerize my application'"
echo ""
echo "MCP Server location: $MCP_PATH"
echo "Configuration file: $CONFIG_FILE"
echo ""

# Optional: Test the server
read -p "Would you like to test the MCP server? (y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "Testing MCP server (press Ctrl+C to stop)..."
    "$MCP_PATH" --demo=basic
fi
