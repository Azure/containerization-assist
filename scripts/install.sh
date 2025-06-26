#!/bin/bash
# Container Kit Installation Script
# This script downloads and installs the latest version of container-kit

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
REPO_OWNER="Azure"
REPO_NAME="container-kit"
BINARY_NAME="container-kit"
INSTALL_DIR="/usr/local/bin"
FALLBACK_DIR="$HOME/bin"

# Print colored messages
print_error() {
    echo -e "${RED}Error: $1${NC}" >&2
}

print_success() {
    echo -e "${GREEN}$1${NC}"
}

print_info() {
    echo -e "${YELLOW}$1${NC}"
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
        mingw*|msys*|cygwin*)
            print_error "Windows detected. Please use the PowerShell installation script instead."
            print_info "Run: Invoke-WebRequest -Uri https://raw.githubusercontent.com/$REPO_OWNER/$REPO_NAME/main/scripts/install.ps1 -OutFile install.ps1; ./install.ps1"
            exit 1
            ;;
        *)
            print_error "Unsupported operating system: $OS"
            exit 1
            ;;
    esac

    PLATFORM="${OS}_${ARCH}"
    print_info "Detected platform: $PLATFORM"
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Get the latest release version from GitHub
get_latest_version() {
    print_info "Fetching latest release information..."

    if command_exists curl; then
        LATEST_VERSION=$(curl -s "https://api.github.com/repos/$REPO_OWNER/$REPO_NAME/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    elif command_exists wget; then
        LATEST_VERSION=$(wget -qO- "https://api.github.com/repos/$REPO_OWNER/$REPO_NAME/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    else
        print_error "Neither curl nor wget found. Please install one of them."
        exit 1
    fi

    if [ -z "$LATEST_VERSION" ]; then
        print_error "Failed to fetch latest release version"
        exit 1
    fi

    print_info "Latest version: $LATEST_VERSION"
}

# Download the binary
download_binary() {
    local version=$1
    local platform=$2

    # Construct download URL
    local archive_name="${BINARY_NAME}_${version#v}_${platform}.tar.gz"
    local download_url="https://github.com/$REPO_OWNER/$REPO_NAME/releases/download/$version/$archive_name"
    local checksum_url="https://github.com/$REPO_OWNER/$REPO_NAME/releases/download/$version/checksums.txt"

    print_info "Downloading $BINARY_NAME $version for $platform..."

    # Create temporary directory
    TMP_DIR=$(mktemp -d)
    trap "rm -rf $TMP_DIR" EXIT

    cd "$TMP_DIR"

    # Download archive
    if command_exists curl; then
        curl -sL "$download_url" -o "$archive_name" || {
            print_error "Failed to download $archive_name"
            exit 1
        }
        curl -sL "$checksum_url" -o "checksums.txt" 2>/dev/null || true
    else
        wget -q "$download_url" -O "$archive_name" || {
            print_error "Failed to download $archive_name"
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
    print_info "Extracting binary..."
    tar xzf "$archive_name" || {
        print_error "Failed to extract archive"
        exit 1
    }

    # Verify binary exists
    if [ ! -f "$BINARY_NAME" ]; then
        print_error "Binary not found in archive"
        exit 1
    fi

    chmod +x "$BINARY_NAME"
}

# Install the binary
install_binary() {
    local install_path=""

    # Try to install to system directory first
    if [ -w "$INSTALL_DIR" ] || [ -w "$(dirname "$INSTALL_DIR")" ]; then
        install_path="$INSTALL_DIR/$BINARY_NAME"
        mv "$BINARY_NAME" "$install_path" 2>/dev/null || {
            # Try with sudo if regular move fails
            print_info "Installing to $INSTALL_DIR (requires sudo)..."
            sudo mv "$BINARY_NAME" "$install_path" || {
                print_error "Failed to install to $INSTALL_DIR"
                return 1
            }
        }
    else
        # Fallback to user directory
        print_info "Cannot write to $INSTALL_DIR, installing to $FALLBACK_DIR instead..."

        # Create user bin directory if it doesn't exist
        mkdir -p "$FALLBACK_DIR"

        install_path="$FALLBACK_DIR/$BINARY_NAME"
        mv "$BINARY_NAME" "$install_path" || {
            print_error "Failed to install to $FALLBACK_DIR"
            return 1
        }

        # Check if user bin is in PATH
        if ! echo "$PATH" | grep -q "$FALLBACK_DIR"; then
            print_info ""
            print_info "Note: $FALLBACK_DIR is not in your PATH."
            print_info "Add the following line to your shell profile (.bashrc, .zshrc, etc.):"
            print_info "  export PATH=\"\$PATH:$FALLBACK_DIR\""
            print_info ""
        fi
    fi

    print_success "Successfully installed $BINARY_NAME to $install_path"
}

# Verify installation
verify_installation() {
    if command_exists "$BINARY_NAME"; then
        local version=$("$BINARY_NAME" --version 2>/dev/null || echo "unknown")
        print_success "✅ $BINARY_NAME is installed and accessible"
        print_info "Version: $version"
        print_info ""
        print_info "To get started, run:"
        print_info "  $BINARY_NAME --help"
    else
        print_error "❌ $BINARY_NAME was installed but is not accessible in PATH"
        print_info "You may need to restart your shell or add the installation directory to PATH"
        exit 1
    fi
}

# Main installation flow
main() {
    print_info "=== Container Kit Installation Script ==="
    print_info ""

    # Check for existing installation
    if command_exists "$BINARY_NAME"; then
        local current_version=$("$BINARY_NAME" --version 2>/dev/null || echo "unknown")
        print_info "Found existing installation: $current_version"
        read -p "Do you want to proceed with reinstallation? (y/N) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            print_info "Installation cancelled"
            exit 0
        fi
    fi

    detect_platform
    get_latest_version
    download_binary "$LATEST_VERSION" "$PLATFORM"
    install_binary
    verify_installation

    print_info ""
    print_success "🎉 Installation complete!"
}

# Run main function
main "$@"
