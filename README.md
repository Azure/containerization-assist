[![Simple CI](https://github.com/Azure/container-kit/actions/workflows/ci-simple.yml/badge.svg)](https://github.com/Azure/container-kit/actions/workflows/ci-simple.yml)

# Container Kit

AI-Powered Application Containerization and Kubernetes Deployment

Container Kit automates the complete containerization process from repository analysis to Kubernetes deployment using a unified workflow approach. Built on a clean 4-layer architecture with Domain-Driven Design, it provides a single, powerful workflow tool that handles the entire process with AI-powered error recovery and built-in progress tracking.

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

**Key Architecture Features:**
- **Clean 4-Layer Architecture**: API → Domain → Application → Infrastructure with clear dependencies
- **Single Workflow Tool**: `containerize_and_deploy` handles complete 10-step process
- **Event-Driven Design**: Domain events for workflow coordination and observability
- **AI-Enhanced Operations**: Built-in AI error recovery and ML-powered optimization
- **Progress Tracking**: Real-time progress indicators with metadata and visual feedback
- **Rich Error System**: Unified error handling with actionable suggestions
- **Session Management**: BoltDB-based state persistence across operations
- **Dependency Injection**: Wire-based DI with manual configuration for testability
- **Comprehensive Testing**: Integration tests with workflow validation

> **📖 Technical Details**: See [Development Guidelines](DEVELOPMENT_GUIDELINES.md) and [Container Kit Design Document](docs/CONTAINER_KIT_DESIGN_DOCUMENT.md).

## 🛠️ Key Features

- **Single Workflow Tool**: Complete containerization via `containerize_and_deploy` with 10 structured steps
- **AI-Powered Error Recovery**: Intelligent error analysis and automated retry logic with context
- **Real-Time Progress Tracking**: Visual progress indicators with step-by-step feedback
- **Rich Error System**: Structured error handling with actionable suggestions and severity levels
- **ML-Enhanced Optimization**: Machine learning for build optimization and pattern recognition
- **Event-Driven Coordination**: Domain events for workflow orchestration and observability
- **Session Persistence**: BoltDB-based state management with automatic cleanup
- **Clean Architecture**: 4-layer Domain-Driven Design with proper dependency flow
- **Security Integration**: Comprehensive vulnerability scanning with Trivy/Grype
- **Kubernetes Native**: Automated manifest generation and deployment with health checks
- **Multi-Transport Support**: stdio and HTTP transports with graceful shutdown
- **Comprehensive Testing**: Unit and integration tests with workflow validation

## 🧪 Quick Example

```bash
# Start MCP server
./container-kit-mcp

# Container Kit provides a single powerful workflow tool:
# - containerize_and_deploy: Complete containerization workflow
#   ├── 1/10: Analyze repository structure and detect language/framework
#   ├── 2/10: Generate optimized Dockerfile with AI assistance
#   ├── 3/10: Build Docker image with AI-powered error fixing
#   ├── 4/10: Set up local Kubernetes cluster with registry
#   ├── 5/10: Load Docker image into Kubernetes cluster
#   ├── 6/10: Generate Kubernetes deployment manifests
#   ├── 7/10: Deploy application to Kubernetes cluster
#   ├── 8/10: Perform health checks and endpoint discovery
#   ├── 9/10: Run security vulnerability scan (optional)
#   └── 10/10: Finalize workflow results and cleanup

# Use through Claude Desktop or direct MCP protocol
# Example: "Containerize my Node.js app and deploy to Kubernetes"
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
