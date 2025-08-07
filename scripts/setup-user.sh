#!/bin/bash
# Container Kit User Setup Script
# This script sets up Container Kit MCP Server for non-technical users

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
REPO_OWNER="Azure"
REPO_NAME="container-kit"
BINARY_NAME="container-kit-mcp"
INSTALL_DIR="/usr/local/bin"
FALLBACK_DIR="$HOME/bin"

# Print colored messages
print_error() {
    echo -e "${RED}❌ Error: $1${NC}" >&2
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

print_step() {
    echo -e "${YELLOW}🔧 $1${NC}"
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Detect OS and Architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case $ARCH in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        *)
            print_error "Unsupported architecture: $ARCH"
            print_info "Supported architectures: amd64, arm64"
            exit 1
            ;;
    esac

    case $OS in
        linux|darwin)
            ;;
        mingw*|msys*|cygwin*)
            print_error "Windows detected. Please use the PowerShell setup script instead."
            print_info "Run this PowerShell command as Administrator:"
            print_info "Invoke-WebRequest -Uri https://raw.githubusercontent.com/$REPO_OWNER/$REPO_NAME/main/scripts/setup-user.ps1 -OutFile setup-user.ps1; ./setup-user.ps1; Remove-Item setup-user.ps1"
            exit 1
            ;;
        *)
            print_error "Unsupported operating system: $OS"
            print_info "Supported systems: Linux, macOS"
            exit 1
            ;;
    esac

    PLATFORM="${OS}_${ARCH}"
    print_info "Detected platform: $PLATFORM"
}

# Check prerequisites
check_prerequisites() {
    print_step "Checking prerequisites..."
    
    local missing_tools=()
    
    # Check for required tools
    if ! command_exists curl && ! command_exists wget; then
        missing_tools+=("curl or wget")
    fi
    
    if ! command_exists tar; then
        missing_tools+=("tar")
    fi
    
    # Check for Docker (warn if missing)
    if ! command_exists docker; then
        print_warning "Docker is not installed. You'll need Docker to use Container Kit's containerization features."
        print_info "Install Docker from: https://www.docker.com/products/docker-desktop/"
    else
        print_success "Docker found"
    fi
    
    # Check for git (warn if missing)
    if ! command_exists git; then
        print_warning "Git is not installed. Some Container Kit features may require Git."
        print_info "Install Git from: https://git-scm.com/downloads"
    else
        print_success "Git found"
    fi
    
    if [ ${#missing_tools[@]} -ne 0 ]; then
        print_error "Missing required tools: ${missing_tools[*]}"
        print_info "Please install the missing tools and run this script again."
        exit 1
    fi
    
    print_success "Prerequisites check passed"
}

# Download and install Container Kit
install_container_kit() {
    print_step "Installing Container Kit..."
    
    # Use the existing install script
    if command_exists curl; then
        curl -sSL https://raw.githubusercontent.com/$REPO_OWNER/$REPO_NAME/main/scripts/install.sh | bash
    else
        wget -qO- https://raw.githubusercontent.com/$REPO_OWNER/$REPO_NAME/main/scripts/install.sh | bash
    fi
    
    # Verify installation
    if command_exists "$BINARY_NAME"; then
        local version=$("$BINARY_NAME" --version 2>/dev/null || echo "unknown")
        print_success "Container Kit installed successfully"
        print_info "Version: $version"
    else
        print_error "Container Kit installation failed"
        print_info "Please check the error messages above and try again"
        exit 1
    fi
}

# Find Claude Desktop config directory
find_claude_config_dir() {
    local config_dir=""
    
    case $OS in
        darwin)
            config_dir="$HOME/Library/Application Support/Claude"
            ;;
        linux)
            config_dir="$HOME/.config/claude"
            ;;
    esac
    
    echo "$config_dir"
}

# Setup Claude Desktop configuration
setup_claude_config() {
    print_step "Setting up Claude Desktop configuration..."
    
    local config_dir
    config_dir=$(find_claude_config_dir)
    local config_file="$config_dir/claude_desktop_config.json"
    
    # Check if Claude Desktop is installed
    local claude_installed=false
    case $OS in
        darwin)
            if [ -d "/Applications/Claude.app" ]; then
                claude_installed=true
            fi
            ;;
        linux)
            if command_exists claude-desktop || [ -f "$HOME/.local/share/applications/claude-desktop.desktop" ]; then
                claude_installed=true
            fi
            ;;
    esac
    
    if [ "$claude_installed" = false ]; then
        print_warning "Claude Desktop not found"
        print_info "Please install Claude Desktop from: https://claude.ai/download"
        print_info "Then run this script again, or manually configure the MCP server"
        return
    fi
    
    # Create config directory if it doesn't exist
    mkdir -p "$config_dir"
    
    # Create or update configuration
    local config_content='{
  "mcpServers": {
    "container-kit": {
      "command": "'$BINARY_NAME'",
      "args": []
    }
  }
}'
    
    if [ -f "$config_file" ]; then
        print_info "Backing up existing Claude Desktop configuration..."
        cp "$config_file" "$config_file.backup.$(date +%Y%m%d-%H%M%S)"
        
        # Try to merge with existing config (basic merge)
        print_warning "Existing configuration found. Please manually merge if needed."
        print_info "Backup saved as: $config_file.backup.*"
    fi
    
    echo "$config_content" > "$config_file"
    print_success "Claude Desktop configuration created"
    print_info "Configuration file: $config_file"
}

# Create desktop shortcut (optional)
create_shortcuts() {
    print_step "Creating shortcuts..."
    
    case $OS in
        linux)
            # Create desktop file for Linux
            local desktop_file="$HOME/.local/share/applications/container-kit.desktop"
            mkdir -p "$(dirname "$desktop_file")"
            
            cat > "$desktop_file" << EOF
[Desktop Entry]
Name=Container Kit MCP Server
Comment=AI-Powered Application Containerization
Exec=$BINARY_NAME
Icon=docker
Terminal=true
Type=Application
Categories=Development;
EOF
            
            print_success "Desktop shortcut created"
            ;;
        darwin)
            print_info "To create a dock shortcut on macOS:"
            print_info "1. Open Terminal"
            print_info "2. Run: $BINARY_NAME"
            print_info "3. Right-click Terminal in dock → Options → Keep in Dock"
            ;;
    esac
}

# Test the installation
test_installation() {
    print_step "Testing installation..."
    
    # Test Container Kit
    if command_exists "$BINARY_NAME"; then
        print_success "Container Kit is accessible from command line"
        
        # Test version command
        local version_output
        if version_output=$("$BINARY_NAME" --version 2>&1); then
            print_success "Version check passed: $version_output"
        else
            print_warning "Version check failed, but binary is accessible"
        fi
    else
        print_error "Container Kit is not accessible from command line"
        print_info "You may need to restart your terminal or add to PATH"
    fi
    
    # Test Docker if available
    if command_exists docker; then
        if docker version >/dev/null 2>&1; then
            print_success "Docker is running"
        else
            print_warning "Docker is installed but not running"
            print_info "Please start Docker Desktop"
        fi
    fi
}

# Show next steps
show_next_steps() {
    echo
    print_success "🎉 Container Kit User Setup Complete!"
    echo
    print_info "Next Steps:"
    echo
    print_info "1. 📱 Open Claude Desktop (restart if it was running)"
    print_info "2. 💬 Start a new conversation"
    print_info "3. 🗣️  Ask: 'What Container Kit tools are available?'"
    print_info "4. 🚀 Try: 'Help me containerize my application at [your-repo-url]'"
    echo
    print_info "📚 Documentation:"
    print_info "   • User Guide: USER_GUIDE.md in the Container Kit repository"
    print_info "   • GitHub: https://github.com/$REPO_OWNER/$REPO_NAME"
    echo
    print_info "🆘 Need Help?"
    print_info "   • Issues: https://github.com/$REPO_OWNER/$REPO_NAME/issues"
    print_info "   • Discussions: https://github.com/$REPO_OWNER/$REPO_NAME/discussions"
    echo
    print_info "🔧 Advanced Configuration:"
    local config_dir
    config_dir=$(find_claude_config_dir)
    print_info "   • Claude Config: $config_dir/claude_desktop_config.json"
    print_info "   • Add debug logging by adding 'env': {'CONTAINER_KIT_LOG_LEVEL': 'debug'}"
    echo
}

# Main installation flow
main() {
    echo
    print_info "=== Container Kit User Setup Script ==="
    print_info "This script will install Container Kit and configure it for Claude Desktop"
    echo
    
    # Check if user wants to continue
    if [ -t 0 ]; then  # Check if running interactively
        read -p "Do you want to continue? (y/N) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            print_info "Setup cancelled"
            exit 0
        fi
    fi
    
    detect_platform
    check_prerequisites
    install_container_kit
    setup_claude_config
    create_shortcuts
    test_installation
    show_next_steps
}

# Handle interruption
trap 'echo; print_warning "Setup interrupted by user"; exit 1' INT

# Run main function
main "$@"