# Container Kit Documentation

Welcome to the Container Kit documentation. This comprehensive guide covers all aspects of using Container Kit for secure, production-ready containerization.

## Table of Contents

### Getting Started
- [Getting Started Guide](user-guide/getting-started.md) - Quick start and basic usage
- [Installation Guide](user-guide/installation.md) - Detailed installation instructions
- [Basic Examples](user-guide/basic-examples.md) - Simple usage examples

### User Guides
- [Advanced Configuration](user-guide/advanced-configuration.md) - Advanced usage patterns
- [Integration Examples](user-guide/integration-examples.md) - Real-world integration patterns
- [Best Practices](user-guide/best-practices.md) - Recommended practices
- [Troubleshooting](user-guide/troubleshooting.md) - Common issues and solutions

### Security Documentation
- [Security Overview](security/README.md) - Security features overview
- [Security Architecture](security/security-architecture.md) - Detailed security design
- [Security Validation](security/security-validation.md) - Security validation framework
- [Security Policies](security/security-policies.md) - Policy engine documentation
- [Compliance Framework](security/compliance-framework.md) - CIS/NIST compliance
- [Security Best Practices](security/security-best-practices.md) - Security guidelines

### API Reference
- [Workspace API](api/workspace.md) - Workspace management API
- [Sandbox API](api/sandbox.md) - Sandbox execution API
- [Security API](api/security.md) - Security validation API
- [Metrics API](api/metrics.md) - Monitoring and metrics API

### Architecture
- [System Architecture](architecture/overview.md) - High-level system design
- [Component Design](architecture/components.md) - Individual component details
- [Security Design](architecture/security.md) - Security architecture details
- [Performance Design](architecture/performance.md) - Performance considerations

### Development
- [Contributing Guide](development/contributing.md) - How to contribute
- [Development Setup](development/setup.md) - Development environment setup
- [Testing Guide](development/testing.md) - Testing strategies and tools
- [Release Process](development/releases.md) - Release management

## Quick Links

### For New Users
1. [Getting Started](user-guide/getting-started.md) - Start here for basic usage
2. [Security Overview](security/README.md) - Understand security features
3. [Basic Examples](user-guide/basic-examples.md) - See working examples

### For Developers
1. [Advanced Configuration](user-guide/advanced-configuration.md) - Advanced usage patterns
2. [API Reference](api/) - Complete API documentation
3. [Architecture Overview](architecture/overview.md) - System design details

### For Security Engineers
1. [Security Architecture](security/security-architecture.md) - Security design
2. [Compliance Framework](security/compliance-framework.md) - Compliance details
3. [Security Best Practices](security/security-best-practices.md) - Security guidelines

## Key Features

### Production-Ready Sandboxing
- Docker-in-Docker isolation with security controls
- Resource limits and quota management
- Execution timeouts and monitoring
- Comprehensive audit logging

### Security-First Design
- CIS Docker Benchmark compliance
- NIST SP 800-190 container security guidelines
- Built-in threat modeling and risk assessment
- Automated vulnerability scanning

### Comprehensive Monitoring
- Real-time execution metrics
- Resource usage tracking
- Security event monitoring
- Performance profiling

### Enterprise Features
- Multi-tenant workspace management
- Role-based access control
- Compliance reporting
- Integration with existing security tools

## Support and Community

### Getting Help
- Check the [Troubleshooting Guide](user-guide/troubleshooting.md) for common issues
- Search existing [GitHub Issues](https://github.com/Azure/container-kit/issues)
- Create a new issue for bugs or feature requests

### Contributing
- Read the [Contributing Guide](development/contributing.md)
- Follow the [Development Setup](development/setup.md) instructions
- Submit pull requests following our guidelines

### License
Container Kit is licensed under the MIT License. See the [LICENSE](../LICENSE) file for details.

## Version Information

This documentation is for Container Kit v4.0.0 (Sprint 4 - Production Readiness).

### Recent Updates
- Production-ready sandboxing implementation
- Enhanced security validation framework
- Comprehensive test coverage (>90%)
- Complete documentation and user guides
- Quality sign-off for all implementations

### Supported Platforms
- Linux (Ubuntu 20.04+, CentOS 8+, RHEL 8+)
- macOS (10.15+)
- Windows (via WSL2)

### Requirements
- Go 1.24.1+
- Docker 20.10+
- Minimum 4GB RAM
- Minimum 10GB disk space