#!/bin/bash
# Script to install Hadolint for Dockerfile validation
# Hadolint is optional but recommended for comprehensive Dockerfile validation

set -e

HADOLINT_VERSION=${HADOLINT_VERSION:-v2.12.0}
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

# Map architecture names
case $ARCH in
    x86_64)
        ARCH="x86_64"
        ;;
    aarch64|arm64)
        ARCH="arm64"
        ;;
    *)
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

# Determine OS-specific download
case $OS in
    linux)
        HADOLINT_URL="https://github.com/hadolint/hadolint/releases/download/${HADOLINT_VERSION}/hadolint-Linux-${ARCH}"
        ;;
    darwin)
        HADOLINT_URL="https://github.com/hadolint/hadolint/releases/download/${HADOLINT_VERSION}/hadolint-Darwin-${ARCH}"
        ;;
    *)
        echo "Unsupported OS: $OS"
        echo "Please install Hadolint manually from https://github.com/hadolint/hadolint"
        exit 1
        ;;
esac

echo "Installing Hadolint ${HADOLINT_VERSION} for ${OS}-${ARCH}..."

# Download Hadolint
if command -v curl &> /dev/null; then
    curl -L "$HADOLINT_URL" -o /tmp/hadolint
elif command -v wget &> /dev/null; then
    wget "$HADOLINT_URL" -O /tmp/hadolint
else
    echo "Error: Neither curl nor wget is available"
    exit 1
fi

# Make it executable
chmod +x /tmp/hadolint

# Install to /usr/local/bin (may require sudo)
if [ -w /usr/local/bin ]; then
    mv /tmp/hadolint /usr/local/bin/
else
    echo "Installing to /usr/local/bin requires sudo access"
    sudo mv /tmp/hadolint /usr/local/bin/
fi

# Verify installation
if hadolint --version &> /dev/null; then
    echo "✅ Hadolint installed successfully!"
    hadolint --version
else
    echo "❌ Hadolint installation failed"
    exit 1
fi

echo ""
echo "Hadolint is now available for Dockerfile validation in Containerization Assist."
echo "The verify_dockerfile tool will automatically use it when available."
