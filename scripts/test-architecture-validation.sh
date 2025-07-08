#!/bin/bash
# Test script for architecture validation

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "🧪 Testing Architecture Validation Script"
echo "========================================"

# Create temporary test structure
TEST_DIR="/tmp/test-architecture-$$"
mkdir -p "$TEST_DIR/pkg/mcp"

echo "Creating test architecture structure..."

# Create valid three-layer structure
mkdir -p "$TEST_DIR/pkg/mcp/domain/session"
mkdir -p "$TEST_DIR/pkg/mcp/domain/containerization/build"
mkdir -p "$TEST_DIR/pkg/mcp/application/ports"
mkdir -p "$TEST_DIR/pkg/mcp/application/commands"
mkdir -p "$TEST_DIR/pkg/mcp/infra/transport"

# Create minimal Go files
cat > "$TEST_DIR/pkg/mcp/domain/session/types.go" << 'EOF'
package session

type Session struct {
    ID   string
    Name string
}
EOF

cat > "$TEST_DIR/pkg/mcp/application/ports/interfaces.go" << 'EOF'
package ports

type Tool interface {
    Execute() error
}
EOF

cat > "$TEST_DIR/pkg/mcp/application/commands/coordinator.go" << 'EOF'
package commands

import "github.com/Azure/container-kit/pkg/mcp/domain/session"

type Coordinator struct {
    sessions []session.Session
}
EOF

cat > "$TEST_DIR/pkg/mcp/infra/transport/http.go" << 'EOF'
//go:build http

package transport

import "net/http"

type HTTPTransport struct {
    client *http.Client
}
EOF

echo "✅ Test structure created"

# Test the validation script with valid structure
echo ""
echo "Testing with valid architecture..."
if "$SCRIPT_DIR/validate-architecture.sh" "$TEST_DIR"; then
    echo "✅ Valid architecture test PASSED"
else
    echo "❌ Valid architecture test FAILED"
    cd "$REPO_ROOT"
    rm -rf "$TEST_DIR"
    exit 1
fi

# Test with invalid structure (add violation)
echo ""
echo "Testing with architecture violation..."
cat > "$TEST_DIR/pkg/mcp/domain/session/bad.go" << 'EOF'
package session

import "github.com/Azure/container-kit/pkg/mcp/infra/transport"

// This should cause a violation - domain importing infra
type BadSession struct {
    transport transport.HTTPTransport
}
EOF

if "$SCRIPT_DIR/validate-architecture.sh" "$TEST_DIR"; then
    echo "❌ Invalid architecture test FAILED (should have detected violation)"
    cd "$REPO_ROOT"
    rm -rf "$TEST_DIR"
    exit 1
else
    echo "✅ Invalid architecture test PASSED (correctly detected violation)"
fi

# Cleanup
cd "$REPO_ROOT"
rm -rf "$TEST_DIR"

echo ""
echo "🎉 All architecture validation tests PASSED!"
echo "The script correctly detects valid and invalid architectures."