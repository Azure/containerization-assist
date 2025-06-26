#!/bin/bash
# Script to install Trivy for container security scanning
# Trivy is optional but recommended for vulnerability scanning

set -e

TRIVY_VERSION=${TRIVY_VERSION:-v0.48.0}
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

# Map architecture names
case $ARCH in
    x86_64)
        ARCH="64bit"
        ;;
    aarch64|arm64)
        ARCH="ARM64"
        ;;
    armv7l)
        ARCH="ARM"
        ;;
    *)
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

# Determine OS-specific download
case $OS in
    linux)
        FILE_NAME="trivy_${TRIVY_VERSION#v}_Linux-${ARCH}.tar.gz"
        ;;
    darwin)
        FILE_NAME="trivy_${TRIVY_VERSION#v}_macOS-${ARCH}.tar.gz"
        ;;
    *)
        echo "Unsupported OS: $OS"
        echo "Please install Trivy manually from https://github.com/aquasecurity/trivy"
        exit 1
        ;;
esac

TRIVY_URL="https://github.com/aquasecurity/trivy/releases/download/${TRIVY_VERSION}/${FILE_NAME}"

echo "Installing Trivy ${TRIVY_VERSION} for ${OS}-${ARCH}..."

# Create temporary directory
TMP_DIR=$(mktemp -d)
trap "rm -rf ${TMP_DIR}" EXIT

# Download Trivy
if command -v curl &> /dev/null; then
    curl -L "$TRIVY_URL" -o "${TMP_DIR}/trivy.tar.gz"
elif command -v wget &> /dev/null; then
    wget "$TRIVY_URL" -O "${TMP_DIR}/trivy.tar.gz"
else
    echo "Error: Neither curl nor wget is available"
    exit 1
fi

# Extract Trivy
tar -xzf "${TMP_DIR}/trivy.tar.gz" -C "${TMP_DIR}"

# Make it executable
chmod +x "${TMP_DIR}/trivy"

# Install to /usr/local/bin (may require sudo)
if [ -w /usr/local/bin ]; then
    mv "${TMP_DIR}/trivy" /usr/local/bin/
else
    echo "Installing to /usr/local/bin requires sudo access"
    sudo mv "${TMP_DIR}/trivy" /usr/local/bin/
fi

# Verify installation
if trivy --version &> /dev/null; then
    echo "✅ Trivy installed successfully!"
    trivy --version
else
    echo "❌ Trivy installation failed"
    exit 1
fi

echo ""
echo "Trivy is now available for container security scanning in Container Kit."
echo "The build_image tool will automatically scan images for vulnerabilities."
echo ""
echo "First run may take longer as Trivy downloads its vulnerability database."
