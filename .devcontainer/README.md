# Container Kit Development Container

This development container provides a fully configured environment for Container Kit development with all necessary tools pre-installed.

## 🚀 Quick Start

### Prerequisites
- [Docker](https://docs.docker.com/get-docker/) installed and running
- [VS Code](https://code.visualstudio.com/) with [Dev Containers extension](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers)

### Getting Started

1. **Clone the repository**:
   ```bash
   git clone https://github.com/Azure/container-copilot.git
   cd container-copilot
   ```

2. **Open in VS Code**:
   ```bash
   code .
   ```

3. **Open in Dev Container**:
   - VS Code will detect the `.devcontainer` configuration
   - Click "Reopen in Container" when prompted
   - Or use Command Palette (Ctrl+Shift+P): "Dev Containers: Reopen in Container"

4. **Wait for setup** (first time only):
   - The container will build and run the setup script automatically
   - This takes ~3-5 minutes for initial setup
   - Subsequent starts are much faster

5. **Start developing**:
   ```bash
   # Build the MCP server
   make mcp
   
   # Run tests
   make test
   
   # Run linting
   make lint
   ```

## 🛠️ What's Included

### Core Development Tools
- **Go 1.24+** - Latest Go compiler and tools
- **Node.js LTS** - JavaScript runtime for web tools
- **npm** - Node package manager
- **golangci-lint** - Comprehensive Go linting
- **goimports** - Automatic import formatting
- **delve (dlv)** - Go debugger
- **make** - Build automation

### Container & Kubernetes Tools
- **Docker-in-Docker** - Full Docker support inside container
- **kubectl** - Kubernetes command-line tool
- **kind** - Kubernetes in Docker for local testing
- **Helm** - Kubernetes package manager

### VS Code Extensions
- **Go** - Full Go language support
- **Kubernetes Tools** - K8s manifest editing and cluster management
- **GitHub Copilot** - AI-powered code completion
- **YAML/JSON** - Configuration file editing
- **Makefile Tools** - Makefile syntax support

### Development Conveniences
- **Pre-commit hooks** - Automatic linting and testing
- **Helpful aliases** - Common Git and Kubernetes shortcuts
- **Port forwarding** - MCP server (8080), development (3000), metrics (9090)
- **Persistent workspace** - Your changes are preserved

## 📋 Available Commands

### Build Commands
```bash
make mcp              # Build MCP server
make test             # Run all tests
make lint             # Run linting
build-mcp             # Build MCP server (alias)
```

### Testing Commands
```bash
test-mcp              # Run MCP-specific tests
go test ./pkg/mcp/... # Run MCP package tests
go test -race ./...   # Run all tests with race detection
```

### MCP Server Commands
```bash
run-mcp               # Run MCP server with stdio transport
run-mcp-http          # Run MCP server with HTTP transport on port 8080
./container-kit-mcp --help  # See all available options
```

### Git Shortcuts
```bash
gst                   # git status
gco <branch>          # git checkout
gcb <branch>          # git checkout -b (new branch)
ga <files>            # git add
gc -m "message"       # git commit
gp                    # git push
gl                    # git pull
gd                    # git diff
```

### Kubernetes Shortcuts
```bash
k                     # kubectl
kgp                   # kubectl get pods
kgs                   # kubectl get services
kgd                   # kubectl get deployments
kdp <pod>             # kubectl describe pod
kds <service>         # kubectl describe service
kdd <deployment>      # kubectl describe deployment
```

## 🐛 Troubleshooting

### Container won't start
- Ensure Docker is running
- Try rebuilding: Command Palette → "Dev Containers: Rebuild Container"
- Check Docker has enough resources allocated

### Tests failing
- Run `go mod tidy` to update dependencies
- Ensure you're in the correct directory
- Check if you need to build first: `make mcp`

### golangci-lint issues
- Update to latest version: `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`
- Check `.golangci.yml` configuration in project root

### Port conflicts
- Default ports: 8080 (MCP HTTP), 3000 (dev), 9090 (metrics)
- Change ports in `devcontainer.json` if needed
- Use `run-mcp` (stdio) instead of `run-mcp-http` to avoid port 8080

## 🔧 Customization

### Adding Tools
Edit `.devcontainer/setup.sh` to install additional tools.

### VS Code Settings
Modify `customizations.vscode.settings` in `devcontainer.json`.

### Port Configuration
Update `forwardPorts` and `portsAttributes` in `devcontainer.json`.

## 📚 Learning Resources

- [Container Kit MCP Documentation](../MCP_DOCUMENTATION.md)
- [AI Integration Pattern](../docs/AI_INTEGRATION_PATTERN.md)
- [Contributing Guide](../CONTRIBUTING.md)
- [Dev Containers Documentation](https://containers.dev/)

## 🆘 Getting Help

- **Issues**: [GitHub Issues](https://github.com/Azure/container-copilot/issues)
- **Discussions**: [GitHub Discussions](https://github.com/Azure/container-copilot/discussions)
- **Documentation**: [MCP Documentation](../MCP_DOCUMENTATION.md)

---

**Happy Development!** 🎉