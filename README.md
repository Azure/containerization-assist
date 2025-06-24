# Container Kit

AI-Powered Application Containerization and Kubernetes Deployment

Container Kit automates the creation of Docker images and Kubernetes manifests using AI-guided workflows and atomic operations. It provides two modes of operation optimized for different use cases.

## 🚀 Quick Start

### MCP Server (Recommended)

The MCP server provides both atomic tools and conversational workflows for containerization.

**Prerequisites:**
- Go 1.21+
- Docker
- kubectl (optional, for Kubernetes features)

**Setup:**
```bash
git clone https://github.com/Azure/container-copilot.git
cd container-copilot

# Build the MCP server
make mcp

# Test the server
./container-kit-mcp --version
```

**Use with Claude Desktop:**
Add to your Claude Desktop config and ask Claude: *"Help me containerize my application"*

> **📖 Complete Setup Guide**: See [MCP_DOCUMENTATION.md](MCP_DOCUMENTATION.md) for detailed setup instructions, troubleshooting, and advanced configuration.

### CLI Tool (Legacy)

**Prerequisites:**
- Go 1.21+
- kubectl, Docker, Kind
- Azure OpenAI (for AI features)

**Setup:**
```bash
# Set Azure OpenAI credentials
export AZURE_OPENAI_KEY=xxxxxxx
export AZURE_OPENAI_ENDPOINT=xxxxxx
export AZURE_OPENAI_DEPLOYMENT_ID=container-kit

# Run containerization
go run . generate <path/to/target-repo>
```

## 🛠️ Development Setup

### Option 1: Development Container (Recommended)

Get started in seconds with a fully configured development environment:

**Prerequisites:**
- [Docker](https://docs.docker.com/get-docker/)
- [VS Code](https://code.visualstudio.com/) with [Dev Containers extension](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers)

**Setup:**
1. Clone this repository
2. Open in VS Code: `code .`
3. Click "Reopen in Container" when prompted
4. Wait for automatic setup (3-5 minutes first time)
5. Start coding! All tools are pre-installed and configured.

See [`.devcontainer/README.md`](.devcontainer/README.md) for full details.

### Option 2: Local Development

**Prerequisites:**
- Go 1.21+
- golangci-lint
- Docker, kubectl, kind (for full functionality)

**Setup:**
```bash
# Install golangci-lint
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin

# Build and test
make mcp           # Build MCP server
make test          # Run tests
make lint          # Run linting
make help          # See all available targets
```

## 📖 Documentation

### Core Documentation
- **[MCP Server Documentation](./MCP_DOCUMENTATION.md)** - Complete setup, tools, and usage guide
- **[Architecture Overview](./ARCHITECTURE.md)** - Technical design and system architecture  
- **[AI Integration Pattern](./docs/AI_INTEGRATION_PATTERN.md)** - AI integration guidelines and fixing capabilities

### Development
- **[Contributing Guide](./CONTRIBUTING.md)** - Development workflow and standards
- **[Development Container](./.devcontainer/README.md)** - Instant development setup
- **[Development Guide](./CLAUDE.md)** - Claude Code development guidance
- **[Linting Strategy](./docs/LINTING.md)** - Code quality and error budget approach

### Operations
- **[Security Policy](./SECURITY.md)** - Security guidelines and vulnerability reporting
- **[Support Guide](./SUPPORT.md)** - Getting help and troubleshooting

## 🏗️ Architecture

Container Kit provides two operation modes with different architectural approaches:

- **MCP Server** (Primary): Atomic tools + conversational workflows with session persistence
- **CLI Tool** (Legacy): Pipeline-based iterative refinement with AI integration

> **📖 Complete Architecture Guide**: See [ARCHITECTURE.md](ARCHITECTURE.md) for detailed technical design and system components.

## 🛠️ Available Tools

### MCP Atomic Tools
- `analyze_repository_atomic` - Analyze code for containerization
- `generate_dockerfile` - Generate optimized Dockerfiles
- `build_image_atomic` - Build Docker images with fixing capabilities
- `push_image_atomic` - Push to registries
- `pull_image_atomic` - Pull images from registries
- `tag_image_atomic` - Tag Docker images
- `generate_manifests_atomic` - Create Kubernetes manifests
- `deploy_kubernetes_atomic` - Deploy to Kubernetes with fixing
- `check_health_atomic` - Verify deployment health
- `scan_image_security_atomic` - Security vulnerability scanning
- `scan_secrets_atomic` - Secret detection and remediation
- `validate_dockerfile_atomic` - Dockerfile validation and optimization

### Conversation Mode
- `chat` - Guided conversational workflow through complete containerization process

### Management Tools
- `list_sessions` - List active MCP sessions
- `delete_session` - Clean up sessions and workspaces
- `get_server_health` - Server status and capabilities
- `get_logs` - Export server logs with filtering
- `get_telemetry_metrics` - Export Prometheus metrics

## 🧪 Testing

```bash
# Run automated tests
./test/integration/run_tests.sh

# Manual testing with Claude Desktop
# See test/integration/mcp/claude_desktop_test.md

# Run specific test suites
make test                    # All tests
go test ./pkg/mcp/...       # MCP-specific tests
go test -tags integration   # Integration tests only
```

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details on:

- Development setup (devcontainer recommended)
- Code style and standards
- Testing requirements
- Pull request process

## 📊 Quality & Linting

We use an error budget approach for code quality:

```bash
make lint              # Strict linting (fails on any issue)
make lint-threshold    # Linting with error budget
make lint-report       # Generate detailed reports
```

See [docs/LINTING.md](docs/LINTING.md) for our quality strategy.

## 🚢 Deployment Models

### MCP Server Deployment
- **Development**: Local stdio transport with Claude Desktop
- **Production**: HTTP transport with load balancing
- **Cloud**: Container deployment with persistent volumes
- **Instant Setup**: VS Code devcontainer with all tools pre-configured

### CLI Deployment
- **Local**: Direct execution with local Docker/Kind
- **CI/CD**: Pipeline integration for automated containerization

## 📝 License

This project is licensed under the MIT License - see the LICENSE file for details.

## 🔒 Security

See [SECURITY.md](SECURITY.md) for security policy and reporting vulnerabilities.

## 📞 Support

- **Issues**: Use GitHub Issues for bug reports and feature requests
- **Discussions**: Use GitHub Discussions for questions and help
- **Documentation**: Check the documentation links above

## 🏷️ Version

See [releases](https://github.com/Azure/container-copilot/releases) for version history and changelog.