#!/bin/bash
set -e

echo "🚀 Setting up Container Kit development environment..."

# Change to workspace directory
WORKSPACE_DIR="${CONTAINER_WORKSPACE_FOLDER:-$(pwd)}"
echo "📁 Working directory: $WORKSPACE_DIR"
cd "$WORKSPACE_DIR"

# Update system packages
echo "📦 Updating system packages..."
sudo apt-get update

# Install additional useful tools
echo "🔧 Installing development tools..."
sudo apt-get install -y \
    curl \
    wget \
    git \
    jq \
    tree \
    htop \
    vim \
    nano \
    unzip \
    ca-certificates \
    gnupg \
    lsb-release

# Install Node.js and npm
echo "📦 Installing Node.js and npm..."
curl -fsSL https://deb.nodesource.com/setup_lts.x | sudo -E bash -
sudo apt-get install -y nodejs
npm install -g npm@latest

# Install golangci-lint
echo "🔍 Installing golangci-lint..."
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.55.2

# Add Go bin to PATH permanently
echo "🔧 Configuring Go environment..."
echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.bashrc
echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.zshrc
export PATH=$PATH:$(go env GOPATH)/bin

# Install kind for local Kubernetes testing
echo "🐋 Installing kind (Kubernetes in Docker)..."
[ $(uname -m) = x86_64 ] && curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64
[ $(uname -m) = aarch64 ] && curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-arm64
chmod +x ./kind
sudo mv ./kind /usr/local/bin/kind

# Install go tools commonly used in development
echo "🛠️  Installing Go development tools..."
go install -a golang.org/x/tools/cmd/goimports@latest
go install -a github.com/go-delve/delve/cmd/dlv@latest
go install -a github.com/fatih/gomodifytags@latest
go install -a github.com/josharian/impl@latest
go install -a github.com/cweill/gotests/gotests@latest

# Verify installations
echo "✅ Verifying installations..."
echo "Go version: $(go version)"
echo "Node.js version: $(node --version)"
echo "npm version: $(npm --version)"
echo "golangci-lint version: $(golangci-lint --version)"
echo "kubectl version: $(kubectl version --client --short 2>/dev/null || echo 'kubectl not available')"
echo "kind version: $(kind version)"
echo "Docker version: $(docker --version)"

# Initialize Go module cache
echo "📚 Warming up Go module cache..."
if [ -f "go.mod" ]; then
    go mod download || echo "⚠️  Failed to download modules, continuing..."
else
    echo "⚠️  No go.mod found, skipping module download"
fi

# Build the project to ensure everything works
echo "🔨 Building project to verify setup..."
if [ -f "main.go" ]; then
    go build -o /tmp/container-kit . || echo "⚠️  Main build failed, continuing..."
fi

if [ -d "cmd/mcp-server" ]; then
    go build -tags mcp -o /tmp/container-kit-mcp ./cmd/mcp-server || echo "⚠️  MCP server build failed, continuing..."
else
    echo "⚠️  cmd/mcp-server not found, skipping MCP build"
fi

# Run tests to ensure everything is working
echo "🧪 Running quick test to verify setup..."
if [ -d "pkg/mcp/tools" ]; then
    go test ./pkg/mcp/tools/ -short || echo "⚠️  Tests failed, continuing..."
else
    echo "⚠️  pkg/mcp/tools not found, skipping tests"
fi

# Create helpful aliases
echo "📝 Setting up helpful aliases..."
cat >> ~/.bashrc << 'EOF'

# Container Kit development aliases
alias build-mcp='go build -tags mcp -o container-kit-mcp ./cmd/mcp-server'
alias test-mcp='go test -tags mcp -race ./pkg/mcp/...'
alias lint-mcp='golangci-lint run ./pkg/mcp/...'
alias run-mcp='./container-kit-mcp'
alias run-mcp-http='./container-kit-mcp --transport=http --port=8080'

# Common git aliases
alias gst='git status'
alias gco='git checkout'
alias gcb='git checkout -b'
alias gp='git push'
alias gl='git pull'
alias gd='git diff'
alias ga='git add'
alias gc='git commit'

# Kubernetes aliases
alias k='kubectl'
alias kgp='kubectl get pods'
alias kgs='kubectl get services'
alias kgd='kubectl get deployments'
alias kdp='kubectl describe pod'
alias kds='kubectl describe service'
alias kdd='kubectl describe deployment'
EOF

# Set up pre-commit hooks directory
echo "🔗 Setting up git hooks..."
if [ -d ".git" ]; then
    mkdir -p .git/hooks
    cat > .git/hooks/pre-commit << 'EOF'
#!/bin/bash
# Run linting and tests before commit
echo "Running pre-commit checks..."

# Run linting
if ! golangci-lint run ./pkg/mcp/...; then
    echo "❌ Linting failed. Please fix the issues before committing."
    exit 1
fi

# Run tests
if ! go test -tags mcp -race ./pkg/mcp/... -short; then
    echo "❌ Tests failed. Please fix the issues before committing."
    exit 1
fi

echo "✅ Pre-commit checks passed!"
EOF
    chmod +x .git/hooks/pre-commit
else
    echo "⚠️  Not in a git repository, skipping git hooks setup"
fi

# Create welcome message
cat > ~/.devcontainer-welcome << 'EOF'
🎉 Welcome to Container Kit Development Environment!

Quick start commands:
  make mcp              # Build MCP server
  make test             # Run all tests
  make lint             # Run linting
  test-mcp              # Run MCP-specific tests
  build-mcp             # Build MCP server binary
  run-mcp-http          # Run MCP server with HTTP transport

Useful aliases have been set up:
  gst, gco, gcb, gp, gl, gd, ga, gc (git shortcuts)
  k, kgp, kgs, kgd (kubectl shortcuts)

The environment includes:
  ✅ Go 1.21+
  ✅ golangci-lint
  ✅ kubectl & kind
  ✅ Docker-in-Docker
  ✅ VS Code extensions
  ✅ Pre-commit hooks

Happy coding! 🚀
EOF

# Show welcome message
echo ""
echo "$(cat ~/.devcontainer-welcome)"
echo ""
echo "🎯 Development environment setup complete!"
echo "💡 Restart your terminal or run 'source ~/.bashrc' to load aliases"
