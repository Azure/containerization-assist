[![Simple CI](https://github.com/Azure/container-kit/actions/workflows/ci-simple.yml/badge.svg)](https://github.com/Azure/container-kit/actions/workflows/ci-simple.yml)

# Container Kit

AI-Powered Application Containerization and Kubernetes Deployment

Container Kit automates the complete containerization process from repository analysis to Kubernetes deployment using a unified workflow approach. After aggressive simplification, it now provides a single, powerful workflow tool that handles the entire process with built-in progress tracking.

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
# Check executable
./container-kit-mcp --version

# Verify build
make build
```

For detailed usage and troubleshooting, see the [Tool Guide](docs/TOOL_GUIDE.md).

## 🏃 Quick Start

### Prerequisites
- Docker
- kubectl (optional, for Kubernetes features)
- Azure OpenAI access (for AI features)

### Basic Usage
```bash
# Run MCP server (main executable)
./container-kit-mcp

# Container Kit operates via MCP protocol
# Connect with MCP client for guided containerization
```

### Building from Source
```bash
git clone https://github.com/Azure/container-kit.git
cd container-kit

# Set up make alias (required for WSL/Linux)
alias make='/usr/bin/make'

# Build the MCP server
make build

# Run tests
make test              # Unit tests
make test-integration  # Integration tests

# Code quality
make fmt               # Format code
make lint              # Run linter
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

Container Kit uses a simplified workflow-focused architecture after aggressive cleanup:

- **Single Workflow**: One unified tool handles the complete containerization process
- **Progress Tracking**: Built-in progress indicators for each of the 10 workflow steps
- **Simplified Codebase**: Reduced from 294 files to 25 core files (82% reduction)
- **Error Recovery**: Centralized error handling with actionable messages

> **📖 Technical Details**: See [Development Guidelines](DEVELOPMENT_GUIDELINES.md) for the simplified architecture.

## 🛠️ Key Features

- **Unified Workflow**: Complete containerization in a single tool
- **Progress Tracking**: Visual progress indicators for all 10 steps
- **AI-Guided Process**: Interactive assistance throughout the workflow
- **Session Persistence**: Maintain state across operations
- **Multi-Transport**: stdio and HTTP support
- **Kubernetes Integration**: Generate and deploy manifests
- **Security Scanning**: Built-in vulnerability detection
- **Simplified Architecture**: Dramatically reduced complexity

## 🧪 Quick Example

```bash
# Start MCP server
./container-kit-mcp

# Container Kit provides a single powerful workflow tool:
# - containerize_and_deploy: Complete containerization workflow
#   ├── 1/10: Repository analysis
#   ├── 2/10: Dockerfile generation
#   ├── 3/10: Container building
#   ├── 4/10: Security scanning
#   ├── 5/10: Image tagging
#   ├── 6/10: Registry push
#   ├── 7/10: Kubernetes manifest generation
#   ├── 8/10: Cluster setup
#   ├── 9/10: Deployment
#   └── 10/10: Health verification

# Use through Claude Desktop or direct API calls
# Ask: "Containerize my Python Flask app and deploy to Kubernetes"
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
- **Documentation**: Check the [Development Guidelines](DEVELOPMENT_GUIDELINES.md) and [MCP Documentation](MCP_DOCUMENTATION.md)
