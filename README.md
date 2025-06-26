# Container Kit

AI-Powered Application Containerization and Kubernetes Deployment

Container Kit automates the creation of Docker images and Kubernetes manifests using AI-guided workflows. It provides atomic tools for precise control and conversational workflows for guided assistance.

## 🚀 Quick Start

### Prerequisites
- Go 1.21+
- Docker
- kubectl (optional, for Kubernetes features)

### MCP Server Setup
```bash
git clone https://github.com/Azure/container-copilot.git
cd container-copilot

# Build the MCP server
make mcp

# Test the server
./container-kit-mcp --version
```

### Use with Claude Desktop
Add to your Claude Desktop config and ask: *"Help me containerize my application"*

## 📖 Documentation

### For Users
- **[Complete User Guide](MCP_DOCUMENTATION.md)** - Setup, tools, configuration, and troubleshooting
- **[Examples](examples/)** - Working code examples and patterns

### For Developers
- **[Architecture Guide](docs/mcp-architecture.md)** - Technical design and unified interface system
- **[Tool Development Guide](docs/adding-new-tools.md)** - Building new tools and integrations
- **[Interface Patterns](docs/interface-patterns.md)** - Design patterns and best practices
- **[Technical Debt Inventory](docs/TECHNICAL_DEBT_INVENTORY.md)** - Current technical debt and cleanup tasks

### For Contributors
- **[Contributing Guide](CONTRIBUTING.md)** - Development workflow and standards
- **[Development Guidelines](DEVELOPMENT_GUIDELINES.md)** - Coding standards and practices
- **[Migration Guide](docs/migration-guide.md)** - v1 to v2 migration instructions
- **[Breaking Changes](docs/breaking-changes.md)** - Breaking changes in v2.0

## 🏗️ Architecture

Container Kit provides atomic tools and conversational workflows through a unified interface system:

- **Atomic Tools**: Individual containerization operations (analyze, build, deploy, scan)
- **Conversation Mode**: Guided AI workflows for complete containerization
- **Unified Interface**: Consistent tool patterns with auto-registration

> **📖 Technical Details**: See [Architecture Guide](docs/mcp-architecture.md) for complete system design.

## 🛠️ Key Features

- **AI-Guided Workflows**: Interactive containerization assistance
- **Atomic Operations**: Precise control over each step
- **Auto-Registration**: Zero-configuration tool discovery
- **Session Persistence**: Maintain state across operations
- **Multi-Transport**: stdio and HTTP support
- **Kubernetes Integration**: Generate and deploy manifests
- **Security Scanning**: Built-in vulnerability detection

## 🧪 Quick Example

```bash
# Start MCP server
./container-kit-mcp

# Use through Claude Desktop or direct API calls
# Ask: "Analyze my Python Flask app and create a Dockerfile"
```

## 🤝 Contributing

We welcome contributions! See our [Contributing Guide](CONTRIBUTING.md) for:
- Development setup (devcontainer recommended)
- Code standards and testing requirements
- Pull request process

## 📝 License

This project is licensed under the MIT License - see the LICENSE file for details.

## 🔒 Security

See [SECURITY.md](SECURITY.md) for security policy and reporting vulnerabilities.

## 📞 Support

- **Issues**: Use GitHub Issues for bug reports and feature requests
- **Discussions**: Use GitHub Discussions for questions and help
- **Documentation**: Check the [Complete User Guide](MCP_DOCUMENTATION.md)
