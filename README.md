[![CI Pipeline](https://github.com/Azure/container-kit/actions/workflows/ci-pipeline.yml/badge.svg)](https://github.com/Azure/container-kit/actions/workflows/ci-pipeline.yml)

# Container Kit

AI-Powered Application Containerization and Kubernetes Deployment

Container Kit automates the creation of Docker images and Kubernetes manifests using AI-guided workflows. It provides atomic tools for precise control and conversational workflows for guided assistance.

## 🚀 Quick Install

### One-Line Installation

**Linux/macOS:**
```bash
curl -sSL https://raw.githubusercontent.com/Azure/container-kit/main/scripts/install.sh | bash
```

**Windows (PowerShell as Administrator):**
```powershell
Set-ExecutionPolicy Bypass -Scope Process -Force; Invoke-WebRequest -Uri https://raw.githubusercontent.com/Azure/container-kit/main/scripts/install.ps1 -OutFile install.ps1; ./install.ps1; Remove-Item install.ps1
```

### Verify Installation
```bash
container-kit --version
```

For detailed usage and troubleshooting, see the [Tool Guide](docs/TOOL_GUIDE.md).

## 🏃 Quick Start

### Prerequisites
- Docker
- kubectl (optional, for Kubernetes features)
- Azure OpenAI access (for AI features)

### Basic Usage
```bash
# Containerize your application
container-kit generate /path/to/your/app

# Use the guided setup
container-kit setup --target-repo /path/to/your/app
```

### Building from Source
```bash
git clone https://github.com/Azure/container-kit.git
cd container-kit

# Build the CLI
go build -o container-kit .

# Build the MCP server
make mcp
```

## 📖 Documentation

### For Users
- **[Complete User Guide](MCP_DOCUMENTATION.md)** - Setup, tools, configuration, and troubleshooting
- **[Examples](examples/)** - Working code examples and patterns

### For Developers
- **[Three-Layer Architecture](docs/THREE_LAYER_ARCHITECTURE.md)** - Clean architecture with domain/application/infra layers
- **[Tool Development Guide](docs/ADDING_NEW_TOOLS.md)** - Building new tools and integrations
- **[Architectural Decisions](docs/architecture/adr/)** - ADRs documenting key design decisions
- **[MCP Tool Standards](docs/MCP_TOOL_STANDARDS.md)** - Canonical implementation patterns

### For Contributors
- **[Contributing Guide](CONTRIBUTING.md)** - Development workflow and standards
- **[Development Guidelines](DEVELOPMENT_GUIDELINES.md)** - Coding standards and practices
- **[Quality Standards](docs/QUALITY_STANDARDS.md)** - Code quality and testing requirements

## 🏗️ Architecture

Container Kit provides atomic tools and conversational workflows through a unified interface system:

- **Atomic Tools**: Individual containerization operations (analyze, build, deploy, scan)
- **Conversation Mode**: Guided AI workflows for complete containerization
- **Unified Interface**: Consistent tool patterns with auto-registration

> **📖 Technical Details**: See [Three-Layer Architecture](docs/THREE_LAYER_ARCHITECTURE.md) for complete system design.

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
- **Documentation**: Check the [Tool Guide](docs/TOOL_GUIDE.md) and [Three-Layer Architecture](docs/THREE_LAYER_ARCHITECTURE.md)
