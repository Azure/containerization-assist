#!/bin/bash
# Install Container Kit MCP Server for GitHub Copilot in VS Code

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Detect OS
OS="$(uname -s)"
case "${OS}" in
    Linux*)     OS_TYPE=Linux;;
    Darwin*)    OS_TYPE=Mac;;
    CYGWIN*|MINGW*|MSYS*) OS_TYPE=Windows;;
    *)          OS_TYPE="UNKNOWN:${OS}"
esac

echo -e "${GREEN}Container Kit MCP Server Installer for GitHub Copilot${NC}"
echo "Detected OS: $OS_TYPE"
echo ""

# Check prerequisites
echo "Checking prerequisites..."

if ! command -v go &> /dev/null; then
    echo -e "${RED}Go is not installed. Please install Go 1.21+ first.${NC}"
    exit 1
fi

if ! command -v code &> /dev/null; then
    echo -e "${YELLOW}Warning: VS Code is not found in PATH. Make sure VS Code is installed.${NC}"
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
chmod +x scripts/container-kit-mcp-wrapper.sh

echo -e "${GREEN}Build successful!${NC}"

# Get the absolute paths
MCP_BINARY="$PROJECT_ROOT/container-kit-mcp"
WRAPPER_SCRIPT="$PROJECT_ROOT/scripts/container-kit-mcp-wrapper.sh"

echo ""
echo -e "${BLUE}Setting up GitHub Copilot integration...${NC}"

# Check if we're in a workspace
if [ -d ".vscode" ]; then
    WORKSPACE_DIR="$(pwd)"
    echo "Found VS Code workspace at: $WORKSPACE_DIR"
    
    # Create or update .vscode/mcp.json
    MCP_CONFIG_FILE="$WORKSPACE_DIR/.vscode/mcp.json"
    echo "Creating MCP configuration: $MCP_CONFIG_FILE"
    
    # Create .vscode directory if it doesn't exist
    mkdir -p "$WORKSPACE_DIR/.vscode"
    
    # Create the MCP configuration
    cat > "$MCP_CONFIG_FILE" << EOF
{
  "servers": {
    "containerKit": {
      "type": "stdio",
      "command": "$WRAPPER_SCRIPT",
      "args": [],
      "description": "Container Kit - AI-powered containerization and Kubernetes deployment"
    }
  }
}
EOF
    
    echo -e "${GREEN}MCP configuration created!${NC}"
    echo "Configuration file: $MCP_CONFIG_FILE"
    
    # Add to .gitignore if it exists
    if [ -f ".gitignore" ]; then
        if ! grep -q "\.vscode/mcp\.json" .gitignore; then
            echo ""
            echo "Adding .vscode/mcp.json to .gitignore..."
            echo "# MCP configuration (contains local paths)" >> .gitignore
            echo ".vscode/mcp.json" >> .gitignore
        fi
    fi
    
else
    echo -e "${YELLOW}Not in a VS Code workspace. You'll need to manually create .vscode/mcp.json${NC}"
    echo ""
    echo "To set up in a workspace later, create .vscode/mcp.json with:"
    echo ""
    echo -e "${BLUE}{"
    echo "  \"servers\": {"
    echo "    \"containerKit\": {"
    echo "      \"type\": \"stdio\","
    echo "      \"command\": \"$WRAPPER_SCRIPT\","
    echo "      \"args\": []"
    echo "    }"
    echo "  }"
    echo -e "}${NC}"
fi

echo ""
echo -e "${GREEN}Installation complete!${NC}"
echo ""
echo "Next steps:"
echo "1. Open VS Code (version 1.99 or later)"
echo "2. Enable MCP support: Set 'chat.mcp.enabled' to true in settings"
echo "3. If not in a workspace, create .vscode/mcp.json with the configuration above"
echo "4. Use Command Palette: 'MCP: List Servers' to manage the server"
echo "5. In Copilot Chat, ask: 'Help me containerize my application'"
echo ""
echo "Installed components:"
echo "- MCP Server: $MCP_BINARY"
echo "- Wrapper Script: $WRAPPER_SCRIPT"
if [ -f "$MCP_CONFIG_FILE" ]; then
    echo "- VS Code Config: $MCP_CONFIG_FILE"
fi
echo ""
echo -e "${BLUE}For troubleshooting, check logs at: ~/.container-kit/logs/${NC}"
echo ""
echo -e "${YELLOW}Note: Only one Container Kit MCP server can run at a time.${NC}"
echo "If you get database timeout errors, check for existing processes:"
echo "  ps aux | grep container-kit-mcp"

# Optional: Test the server
echo ""
read -p "Would you like to test the MCP server? (y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "Testing MCP server..."
    
    # Check for existing processes first
    if ps aux | grep -v grep | grep container-kit-mcp >/dev/null; then
        echo -e "${YELLOW}Warning: Found existing Container Kit MCP processes:${NC}"
        ps aux | grep -v grep | grep container-kit-mcp
        echo ""
        echo "You may want to terminate them first before testing."
        echo ""
    fi
    
    # Test the server can start properly by redirecting logs
    echo "Starting server test (logs redirected to /tmp/container-kit-test.log)..."
    if timeout 3s "$WRAPPER_SCRIPT" > /tmp/container-kit-test.log 2>&1; then
        echo -e "${GREEN}✅ Server started successfully${NC}"
    else
        # Check if it timed out (which is expected) or failed
        if [ $? -eq 124 ]; then
            echo -e "${GREEN}✅ Server ran for test duration${NC}"
        else
            echo -e "${RED}❌ Server failed to start${NC}"
            echo "Check logs at: /tmp/container-kit-test.log"
            tail -5 /tmp/container-kit-test.log
        fi
    fi
    
    # Quick check that it registered tools
    if grep -q "Successfully registered all atomic tools" /tmp/container-kit-test.log 2>/dev/null; then
        echo -e "${GREEN}✅ Tools registered successfully${NC}"
    else
        echo -e "${YELLOW}⚠️  Could not verify tool registration${NC}"
    fi
    
    echo ""
    echo -e "${GREEN}Test completed!${NC}"
    echo "Note: The server logs have been saved to /tmp/container-kit-test.log"
fi