# Container Kit Documentation

Welcome to the Container Kit documentation! This guide will help you navigate our comprehensive documentation system.

## Quick Start

**New to Container Kit?** Start here:
- [Getting Started Guide](getting-started/README.md) - Installation and first project
- [Architecture Overview](architecture/README.md) - Understand the system design
- [Tool Guide](reference/tools/usage-guide.md) - Using Container Kit tools

## Documentation Structure

### 🚀 Getting Started
- [Installation & Setup](getting-started/installation.md)
- [First Project Tutorial](getting-started/first-project.md)
- [Local Development](getting-started/local-development.md)

### 🏗️ Architecture
- [System Overview](architecture/overview.md)
- [Three-Layer Architecture](architecture/three-layer-architecture.md)
- [Service Container Pattern](architecture/service-container.md)
- [Architecture Decision Records (ADRs)](architecture/adr/)

### 📚 Developer Guides
- [Adding New Tools](guides/developer/adding-new-tools.md)
- [Error Handling](guides/developer/error-handling.md)
- [Validation System](guides/developer/validation.md)
- [Schema Generation](guides/developer/schema-generation.md)

### 🔧 Operations
- [Testing Guide](guides/operational/testing.md)
- [Performance Optimization](guides/operational/performance.md)
- [Security Best Practices](guides/operational/security.md)
- [Deployment Guide](guides/operational/deployment.md)
- [Monitoring & Observability](guides/operational/monitoring.md)

### 🔗 Integration
- [MCP Protocol Integration](guides/integration/mcp-integration.md)
- [Docker Integration](guides/integration/docker-integration.md)
- [Kubernetes Integration](guides/integration/kubernetes-integration.md)

### 📖 Reference
- [Tool Inventory](reference/tools/inventory.md)
- [Tool Standards](reference/tools/standards.md)
- [API Reference](reference/api/interfaces.md)
- [Configuration Reference](reference/configuration.md)
- [Troubleshooting Guide](reference/troubleshooting.md)

### 💡 Examples
- [Basic Containerization](examples/basic-containerization/)
- [Advanced Workflows](examples/advanced-workflows/)
- [Integration Patterns](examples/integration-patterns/)

### 🤝 Contributing
- [Code Standards](contributing/code-standards.md)
- [Testing Guidelines](contributing/testing-guidelines.md)
- [Documentation Guidelines](contributing/documentation-guidelines.md)

## Container Kit Overview

Container Kit is a production-ready, enterprise-grade AI-powered containerization platform featuring:

- **12 Production Tools**: Complete containerization workflow from analysis to deployment
- **FileAccessService**: Secure file operations with session isolation
- **Three-Layer Architecture**: Clean separation of concerns with strict dependency rules
- **Service Container**: 21 services with manual dependency injection
- **Session Management**: BoltDB-backed state persistence with workspace isolation
- **Security**: Comprehensive vulnerability scanning and path traversal protection

## Key Features

### Core Capabilities
- **AI-Powered Analysis**: Intelligent repository analysis and Dockerfile generation
- **Automated Operations**: Build, scan, deploy with error fixing
- **Multi-Mode Architecture**: Chat, workflow, and dual-mode operations
- **Enterprise Security**: Trivy/Grype integration with comprehensive scanning

### Technology Stack
- **Language**: Go 1.24.1
- **Protocol**: Model Context Protocol (MCP)
- **Storage**: BoltDB for session persistence
- **Containers**: Docker with full lifecycle management
- **Orchestration**: Kubernetes with manifest generation
- **Monitoring**: Prometheus & OpenTelemetry

## Architecture at a Glance

```
Container Kit (606 files, 159k+ lines)
├── Domain Layer (101 files)    # Pure business logic
├── Application Layer (153 files) # Orchestration & coordination
└── Infrastructure Layer (42 files) # External integrations
```

**Service Container**: 21 services including FileAccessService for secure operations
**Tools**: 12 production-ready tools covering the complete containerization workflow
**Quality**: <100 lint issues, <300μs P95 performance targets

## Common Tasks

### For Developers
1. **Adding a New Tool**: See [Adding New Tools](guides/developer/adding-new-tools.md)
2. **Understanding Architecture**: Review [Three-Layer Architecture](architecture/three-layer-architecture.md)
3. **Error Handling**: Use [Error Handling Guide](guides/developer/error-handling.md)

### For Operations
1. **Deployment**: Follow [Deployment Guide](guides/operational/deployment.md)
2. **Monitoring**: Set up [Monitoring & Observability](guides/operational/monitoring.md)
3. **Security**: Implement [Security Best Practices](guides/operational/security.md)

### For Integration
1. **MCP Protocol**: Use [MCP Integration Guide](guides/integration/mcp-integration.md)
2. **Docker**: Follow [Docker Integration](guides/integration/docker-integration.md)
3. **Kubernetes**: Deploy with [Kubernetes Integration](guides/integration/kubernetes-integration.md)

## Need Help?

- **Quick Reference**: Check [Tool Inventory](reference/tools/inventory.md)
- **Troubleshooting**: See [Troubleshooting Guide](reference/troubleshooting.md)
- **API Details**: Review [API Reference](reference/api/interfaces.md)

## Quality Standards

Container Kit maintains high quality standards:
- **Code Quality**: Comprehensive linting and formatting
- **Performance**: Sub-300μs response times
- **Security**: Session isolation and vulnerability scanning
- **Testing**: Unit and integration test coverage
- **Documentation**: Comprehensive guides and examples

---

*This documentation is organized to help you find information quickly. Start with the getting-started section if you're new, or jump to specific guides based on your needs.*