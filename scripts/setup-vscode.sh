#!/bin/bash
# Containerization Assist VS Code Setup Script
# This script installs Containerization Assist and configures it for VS Code with MCP support

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
REPO_OWNER="Azure"
REPO_NAME="containerization-assist"
BINARY_NAME="containerization-assist-mcp"
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
            exit 1
            ;;
    esac

    case $OS in
        linux|darwin)
            ;;
        *)
            print_error "Unsupported operating system: $OS"
            exit 1
            ;;
    esac

    PLATFORM="${OS}_${ARCH}"
    print_info "Detected platform: $PLATFORM"
}

# Check prerequisites
check_prerequisites() {
    print_step "Checking prerequisites..."
    
    local missing_prereqs=false
    
    # Check for VS Code
    if command_exists code; then
        print_success "VS Code CLI found"
    else
        print_warning "VS Code CLI (code) not found in PATH"
        print_info "Please ensure VS Code is installed and the 'code' command is available"
        print_info "You can add it from VS Code: Cmd+Shift+P → 'Shell Command: Install code command in PATH'"
        missing_prereqs=true
    fi
    
    # Check for Docker
    if command_exists docker; then
        print_success "Docker found"
    else
        print_warning "Docker not found"
        print_info "Docker is required for container operations"
        print_info "Install from: https://www.docker.com/products/docker-desktop/"
    fi
    
    # Check for git
    if command_exists git; then
        print_success "Git found"
    else
        print_warning "Git not found"
        print_info "Git is recommended for version control"
    fi
    
    if [ "$missing_prereqs" = true ]; then
        read -p "Continue anyway? (y/N) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            print_info "Installation cancelled"
            exit 0
        fi
    fi
}

# Download and install Containerization Assist
install_containerization_assist() {
    print_step "Installing Containerization Assist..."
    
    # Create temporary directory
    TMP_DIR=$(mktemp -d)
    trap "rm -rf $TMP_DIR" EXIT
    cd "$TMP_DIR"
    
    # Download latest release
    local download_url="https://github.com/$REPO_OWNER/$REPO_NAME/releases/latest/download/${REPO_NAME}_${PLATFORM}.tar.gz"
    local checksum_url="https://github.com/$REPO_OWNER/$REPO_NAME/releases/latest/download/checksums.txt"
    local archive_name="${REPO_NAME}_${PLATFORM}.tar.gz"
    
    print_info "Downloading Containerization Assist..."
    if command_exists curl; then
        curl -sL "$download_url" -o "$archive_name" || {
            print_error "Failed to download Containerization Assist"
            exit 1
        }
        curl -sL "$checksum_url" -o "checksums.txt" 2>/dev/null || true
    else
        wget -q "$download_url" -O "$archive_name" || {
            print_error "Failed to download Containerization Assist"
            exit 1
        }
        wget -q "$checksum_url" -O "checksums.txt" 2>/dev/null || true
    fi
    
    # Verify checksum if available
    if [ -f "checksums.txt" ] && command_exists sha256sum; then
        print_info "Verifying checksum..."
        if grep "$archive_name" checksums.txt | sha256sum -c - >/dev/null 2>&1; then
            print_success "Checksum verified"
        else
            print_error "Checksum verification failed"
            exit 1
        fi
    fi
    
    # Extract archive
    print_info "Extracting binaries..."
    tar xzf "$archive_name" || {
        print_error "Failed to extract archive"
        exit 1
    }
    
    # Install binaries
    local install_path=""
    if [ -w "$INSTALL_DIR" ] || [ -w "$(dirname "$INSTALL_DIR")" ]; then
        install_path="$INSTALL_DIR"
        if [ -f "$BINARY_NAME" ]; then
            mv "$BINARY_NAME" "$install_path/" 2>/dev/null || sudo mv "$BINARY_NAME" "$install_path/"
        fi
        if [ -f "containerization-assist" ]; then
            mv "containerization-assist" "$install_path/" 2>/dev/null || sudo mv "containerization-assist" "$install_path/"
        fi
    else
        # Fallback to user directory
        print_info "Installing to user directory: $FALLBACK_DIR"
        mkdir -p "$FALLBACK_DIR"
        install_path="$FALLBACK_DIR"
        if [ -f "$BINARY_NAME" ]; then
            mv "$BINARY_NAME" "$install_path/"
        fi
        if [ -f "containerization-assist" ]; then
            mv "containerization-assist" "$install_path/"
        fi
        
        # Check if user bin is in PATH
        if ! echo "$PATH" | grep -q "$FALLBACK_DIR"; then
            print_warning "$FALLBACK_DIR is not in your PATH"
            print_info "Add this line to your shell profile (.bashrc, .zshrc, etc.):"
            print_info "  export PATH=\"\$PATH:$FALLBACK_DIR\""
        fi
    fi
    
    print_success "Containerization Assist installed to $install_path"
}

# Find VS Code settings.json location
find_vscode_settings() {
    local settings_file=""
    
    # Common VS Code settings locations
    if [ "$(uname)" = "Darwin" ]; then
        # macOS
        settings_file="$HOME/Library/Application Support/Code/User/settings.json"
    else
        # Linux
        settings_file="$HOME/.config/Code/User/settings.json"
    fi
    
    # Check for VS Code Insiders
    if [ ! -f "$settings_file" ]; then
        if [ "$(uname)" = "Darwin" ]; then
            settings_file="$HOME/Library/Application Support/Code - Insiders/User/settings.json"
        else
            settings_file="$HOME/.config/Code - Insiders/User/settings.json"
        fi
    fi
    
    echo "$settings_file"
}

# Configure VS Code for MCP
configure_vscode() {
    print_step "Configuring VS Code for Containerization Assist MCP..."
    
    local settings_file=$(find_vscode_settings)
    local settings_dir=$(dirname "$settings_file")
    
    # Create settings directory if it doesn't exist
    mkdir -p "$settings_dir"
    
    # Create temporary file for new settings
    local temp_settings=$(mktemp)
    
    # Check if settings.json exists
    if [ -f "$settings_file" ]; then
        print_info "Backing up existing VS Code settings..."
        cp "$settings_file" "${settings_file}.backup.$(date +%Y%m%d_%H%M%S)"
        
        # Use jq if available, otherwise use a simple approach
        if command_exists jq; then
            # Add MCP configuration using jq
            jq '. + {
                "mcp.servers": {
                    "containerization-assist": {
                        "command": "containerization-assist-mcp",
                        "args": [],
                        "transport": "stdio"
                    }
                },
                "github.copilot.chat.experimental.mcp.enabled": true
            }' "$settings_file" > "$temp_settings"
        else
            # Simple approach: remove last } and append new settings
            if grep -q "mcp.servers" "$settings_file"; then
                print_warning "MCP configuration already exists in settings.json"
                print_info "Please verify the configuration manually"
                rm "$temp_settings"
                return
            fi
            
            # Remove trailing } and whitespace, add comma if needed
            sed '$ s/[[:space:]]*}$//' "$settings_file" > "$temp_settings"
            
            # Add comma if the file doesn't end with one
            if ! tail -c 2 "$temp_settings" | grep -q ','; then
                echo "," >> "$temp_settings"
            fi
            
            # Add MCP configuration
            cat >> "$temp_settings" << 'EOF'
    "mcp.servers": {
        "containerization-assist": {
            "command": "containerization-assist-mcp",
            "args": [],
            "transport": "stdio"
        }
    },
    "github.copilot.chat.experimental.mcp.enabled": true
}
EOF
        fi
    else
        # Create new settings.json
        cat > "$temp_settings" << 'EOF'
{
    "mcp.servers": {
        "containerization-assist": {
            "command": "containerization-assist-mcp",
            "args": [],
            "transport": "stdio"
        }
    },
    "github.copilot.chat.experimental.mcp.enabled": true
}
EOF
    fi
    
    # Move temporary file to settings.json
    mv "$temp_settings" "$settings_file"
    print_success "VS Code configuration updated"
}

# Install VS Code extensions
install_vscode_extensions() {
    print_step "Installing recommended VS Code extensions..."
    
    if ! command_exists code; then
        print_warning "VS Code CLI not found, skipping extension installation"
        print_info "Install these extensions manually:"
        print_info "  - GitHub Copilot"
        print_info "  - GitHub Copilot Chat"
        print_info "  - Docker"
        return
    fi
    
    # List of recommended extensions
    local extensions=(
        "GitHub.copilot"
        "GitHub.copilot-chat"
        "ms-azuretools.vscode-docker"
    )
    
    for ext in "${extensions[@]}"; do
        print_info "Installing $ext..."
        code --install-extension "$ext" --force 2>/dev/null || print_warning "Failed to install $ext"
    done
    
    print_success "VS Code extensions installed"
}

# Verify installation
verify_installation() {
    print_step "Verifying installation..."
    
    # Check Containerization Assist MCP
    if command_exists "$BINARY_NAME"; then
        local version=$("$BINARY_NAME" --version 2>/dev/null || echo "unknown")
        print_success "Containerization Assist MCP is installed"
        print_info "Version: $version"
    else
        print_error "Containerization Assist MCP not found in PATH"
        return 1
    fi
    
    # Check VS Code configuration
    local settings_file=$(find_vscode_settings)
    if [ -f "$settings_file" ] && grep -q "containerization-assist" "$settings_file"; then
        print_success "VS Code MCP configuration found"
    else
        print_warning "VS Code MCP configuration not found"
    fi
    
    return 0
}

# Main installation flow
main() {
    echo
    print_info "=== Containerization Assist VS Code Setup Script ==="
    print_info "This script will install Containerization Assist and configure it for VS Code"
    echo
    
    # Check if already installed
    if command_exists "$BINARY_NAME"; then
        local current_version=$("$BINARY_NAME" --version 2>/dev/null || echo "unknown")
        print_info "Found existing Containerization Assist installation: $current_version"
        read -p "Do you want to reinstall? (y/N) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            print_info "Skipping Containerization Assist installation"
            # Still configure VS Code
            configure_vscode
            install_vscode_extensions
            verify_installation
            print_final_instructions
            exit 0
        fi
    fi
    
    check_prerequisites
    detect_platform
    install_containerization_assist
    configure_vscode
    install_vscode_extensions
    verify_installation
    
    print_final_instructions
}

# Print final instructions
print_final_instructions() {
    echo
    print_success "🎉 Setup complete!"
    echo
    print_info "Next steps:"
    print_info "1. Restart VS Code"
    print_info "2. Open GitHub Copilot Chat (Ctrl+Alt+I or Cmd+Alt+I)"
    print_info "3. Ask: 'What Containerization Assist tools are available?'"
    echo
    print_info "To use Containerization Assist:"
    print_info "• Ask Copilot to analyze your repository"
    print_info "• Request help containerizing your application"
    print_info "• Use specific tools like 'generate_dockerfile' or 'build_image'"
    echo
    
    if ! command_exists docker; then
        print_warning "Remember to install Docker for container operations"
    fi
    
    print_info "For help, visit: https://github.com/Azure/containerization-assist"
}

# Run main function
main "$@"