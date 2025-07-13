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

For detailed usage and troubleshooting, see the examples directory and development guidelines.

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
- **[Examples](examples/)** - Working code examples and patterns

### For Developers
- **[Architectural Decisions](docs/architecture/adr/)** - ADRs documenting key design decisions
- **[Container Kit Design Document](docs/CONTAINER_KIT_DESIGN_DOCUMENT.md)** - Complete system design and architecture
- **[New Developer Guide](docs/NEW_DEVELOPER_GUIDE.md)** - Getting started with development

### For Contributors
- **[Contributing Guide](CONTRIBUTING.md)** - Development workflow and standards
- **[Development Guidelines](DEVELOPMENT_GUIDELINES.md)** - Coding standards and practices

## 🏗️ Architecture

Container Kit uses a **clean 4-layer architecture** with Domain-Driven Design after comprehensive refactoring:

```
pkg/
├── mcp/             # Model Context Protocol server & workflow
│   ├── api/             # Interface definitions and contracts
│   ├── application/     # Application services and orchestration
│   │   ├── commands/    # CQRS command handlers
│   │   ├── queries/     # CQRS query handlers
│   │   ├── config/      # Application configuration
│   │   └── session/     # Session management
│   ├── domain/          # Business logic and workflows
│   │   ├── workflow/    # Core containerization workflow
│   │   ├── events/      # Domain events and handlers
│   │   ├── progress/    # Progress tracking (business concept)
│   │   ├── saga/        # Saga pattern coordination
│   │   └── sampling/    # Domain sampling contracts
│   └── infrastructure/ # Technical implementations
│       ├── steps/       # Workflow step implementations
│       ├── ml/          # Machine learning integrations
│       ├── sampling/    # LLM integration
│       ├── progress/    # Progress tracking implementations
│       ├── prompts/     # MCP prompt management
│       ├── resources/   # MCP resource providers
│       ├── tracing/     # Observability integration
│       ├── utilities/   # Infrastructure utilities
│       └── validation/  # Validation implementations
├── core/            # Core containerization services
│   ├── docker/          # Docker operations
│   ├── kubernetes/      # Kubernetes operations
│   ├── kind/            # Kind cluster management
│   └── security/        # Security scanning
├── common/          # Shared utilities
│   ├── errors/          # Rich error handling
│   ├── filesystem/      # File operations
│   ├── logger/          # Logging utilities
│   └── runner/          # Command execution
├── ai/              # AI integration and analysis
└── pipeline/        # Legacy pipeline stages
```

**Key Improvements:**
- **Clean Architecture**: 4-layer Domain-Driven Design with clear dependencies
- **CQRS Pattern**: Separate command and query handling for better scalability
- **Event-Driven**: Domain events for workflow coordination and observability
- **Saga Orchestration**: Distributed transaction coordination for complex workflows
- **Single Workflow**: One unified tool handles the complete containerization process
- **Progress Tracking**: Structured logging with real-time progress indicators
- **ML Integration**: Machine learning for build optimization and pattern recognition
- **Robust Testing**: Comprehensive test suite with property-based testing
- **Error Recovery**: AI-powered retry logic with actionable error messages

> **📖 Technical Details**: See [Development Guidelines](DEVELOPMENT_GUIDELINES.md) and [Container Kit Design Document](docs/CONTAINER_KIT_DESIGN_DOCUMENT.md).

## 🛠️ Key Features

- **Unified Workflow**: Complete containerization in a single tool (`containerize_and_deploy`)
- **Progress Monitoring**: Structured logging with emoji indicators (🚀🔄✅❌🎉)
- **AI-Guided Process**: Interactive assistance with retry logic throughout workflow
- **Session Persistence**: BoltDB-based state management across operations
- **Event-Driven Architecture**: Domain events for workflow coordination and observability
- **CQRS Implementation**: Separate command and query handling for scalability
- **Saga Orchestration**: Distributed transaction coordination for complex workflows
- **ML-Powered Optimization**: Machine learning for build optimization and pattern recognition
- **Multi-Transport**: stdio and HTTP support with proper error handling
- **Kubernetes Integration**: Generate manifests and deploy with validation retry
- **Security Scanning**: Built-in vulnerability detection with Trivy/Grype
- **Clean Architecture**: 4-layer design with comprehensive test coverage

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
- **Documentation**: Check the [Development Guidelines](DEVELOPMENT_GUIDELINES.md) and [Container Kit Design Document](docs/CONTAINER_KIT_DESIGN_DOCUMENT.md)
