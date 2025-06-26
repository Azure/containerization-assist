#!/bin/bash

# MCP Development Environment Setup Script
# Team D: Infrastructure & Quality

set -e

echo "🚀 Setting up MCP development environment..."
echo "=============================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}✅${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠️${NC} $1"
}

print_error() {
    echo -e "${RED}❌${NC} $1"
}

print_info() {
    echo -e "${BLUE}ℹ️${NC} $1"
}

# Check prerequisites
echo
echo "📋 Checking prerequisites..."

# Check Go version
if command -v go &> /dev/null; then
    GO_VERSION=$(go version | cut -d' ' -f3)
    print_status "Go found: $GO_VERSION"
else
    print_error "Go not found. Please install Go 1.21+ from https://golang.org/dl/"
    exit 1
fi

# Check Git
if command -v git &> /dev/null; then
    GIT_VERSION=$(git --version)
    print_status "Git found: $GIT_VERSION"
else
    print_error "Git not found. Please install Git."
    exit 1
fi

# Check Make
if command -v make &> /dev/null; then
    print_status "Make found"
else
    print_error "Make not found. Please install Make."
    exit 1
fi

# Optional tools
echo
echo "🔧 Checking optional development tools..."

# Check golangci-lint
if command -v golangci-lint &> /dev/null; then
    LINT_VERSION=$(golangci-lint --version | head -n1)
    print_status "golangci-lint found: $LINT_VERSION"
else
    print_warning "golangci-lint not found. Installing..."
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.55.2
    if command -v golangci-lint &> /dev/null; then
        print_status "golangci-lint installed successfully"
    else
        print_warning "golangci-lint installation failed. You can install it manually later."
    fi
fi

# Check govulncheck
if command -v govulncheck &> /dev/null; then
    print_status "govulncheck found"
else
    print_warning "govulncheck not found. Installing..."
    go install golang.org/x/vuln/cmd/govulncheck@latest
    if command -v govulncheck &> /dev/null; then
        print_status "govulncheck installed successfully"
    else
        print_warning "govulncheck installation failed. You can install it manually later."
    fi
fi

# Environment setup
echo
echo "⚙️  Setting up environment..."

# Set up Go environment
export GO111MODULE=on
export CGO_ENABLED=0

print_status "Go module mode enabled"
print_status "CGO disabled for consistent builds"

# Download dependencies
echo
echo "📦 Installing dependencies..."
go mod download
go mod verify
print_status "Dependencies downloaded and verified"

# Build tools
echo
echo "🔨 Building development tools..."

TOOLS_DIR="tools"
TOOL_DIRS=$(find $TOOLS_DIR -mindepth 1 -maxdepth 1 -type d)

for tool_dir in $TOOL_DIRS; do
    tool_name=$(basename $tool_dir)
    echo "  Building $tool_name..."

    if go build -o "bin/$tool_name" "./$tool_dir/main.go" 2>/dev/null; then
        print_status "Built $tool_name"
    else
        print_warning "Failed to build $tool_name (may require migration completion)"
    fi
done

# Create bin directory if it doesn't exist
mkdir -p bin

# IDE configuration
echo
echo "🔧 Setting up IDE configurations..."

if [ -d ".vscode" ]; then
    print_status "VS Code configuration already exists"
else
    print_warning "VS Code configuration not found (should be created by Team D)"
fi

if [ -d ".idea" ]; then
    print_status "IntelliJ/GoLand configuration already exists"
else
    print_warning "IntelliJ/GoLand configuration not found (should be created by Team D)"
fi

# Git hooks (optional)
echo
echo "🪝 Setting up Git hooks..."

if [ ! -f ".git/hooks/pre-commit" ]; then
    cat > .git/hooks/pre-commit << 'EOF'
#!/bin/bash
# Pre-commit hook for MCP project

echo "Running pre-commit checks..."

# Run quality enforcement
if command -v make &> /dev/null; then
    make enforce-quality
else
    echo "Make not found, skipping quality checks"
fi
EOF
    chmod +x .git/hooks/pre-commit
    print_status "Pre-commit hook installed"
else
    print_info "Pre-commit hook already exists"
fi

# Performance baseline
echo
echo "📊 Setting up performance baseline..."

if [ ! -f "performance_baseline.json" ]; then
    print_info "Establishing performance baseline..."
    make baseline-performance || print_warning "Failed to establish baseline"
else
    print_status "Performance baseline already exists"
fi

# Run initial validation
echo
echo "🔍 Running initial validation..."

echo "  Checking package structure..."
if make validate-structure >/dev/null 2>&1; then
    print_status "Package structure validation passed"
else
    print_warning "Package structure validation failed (expected during migration)"
fi

echo "  Checking interfaces..."
if make validate-interfaces >/dev/null 2>&1; then
    print_status "Interface validation passed"
else
    print_warning "Interface validation failed (expected during migration)"
fi

echo "  Checking dependency hygiene..."
if make check-hygiene >/dev/null 2>&1; then
    print_status "Dependency hygiene check passed"
else
    print_warning "Dependency hygiene check failed (some issues expected)"
fi

# Build test
echo
echo "🏗️  Testing build..."
if make build >/dev/null 2>&1; then
    print_status "Build successful"
else
    print_error "Build failed"
    echo "Run 'make build' to see detailed error output"
fi

# Summary
echo
echo "📋 Development Environment Summary"
echo "=================================="
print_info "Available make targets:"
echo "  make build           - Build the MCP server"
echo "  make test-all        - Run all tests"
echo "  make lint            - Run linter"
echo "  make validate-structure    - Check package boundaries"
echo "  make validate-interfaces   - Check interface compliance"
echo "  make enforce-quality       - Run all quality checks"
echo "  make migrate-all           - Execute package migration"
echo "  make help                  - Show all available targets"

echo
print_info "IDE configurations:"
echo "  .vscode/             - VS Code settings and tasks"
echo "  .idea/               - IntelliJ/GoLand project files"

echo
print_info "Development tools:"
echo "  tools/               - Migration and validation tools"
echo "  bin/                 - Built development tools"

echo
if [ -f "performance_baseline.json" ]; then
    print_status "Setup complete! Development environment ready."
else
    print_warning "Setup complete with warnings. Some features may not be available until migration is complete."
fi

echo
print_info "Next steps:"
echo "  1. Run 'make help' to see available commands"
echo "  2. Run 'make test-all' to verify everything works"
echo "  3. Start developing with your preferred IDE"
echo
echo "Happy coding! 🎉"
