# Getting Started with Container Kit

## Installation

### Prerequisites
- Go 1.24.1 or later
- Docker Engine
- Git

### Building from Source

```bash
# Clone the repository
git clone https://github.com/Azure/container-kit.git
cd container-kit

# Set up make alias (required for WSL/Linux)
alias make='/usr/bin/make'

# Build the MCP server
make mcp

# Run tests
make test              # MCP package tests only
make test-mcp          # MCP tests with build tags
make test-all          # All packages
```

## Basic Usage

### 1. Start the MCP Server

```bash
# Run the MCP server (main executable)
./container-kit-mcp

# Or run specific command tools
./cmd/mcp-server/mcp-server
./cmd/mcp-richify/mcp-richify
./cmd/mcp-schema-gen/mcp-schema-gen
```

### 2. Analyze a Repository

```bash
# Container Kit operates via MCP protocol
# Connect via MCP client and use analyze tool
# Available tools: analyze, build, deploy, scan

# Tools are accessed through MCP protocol, not direct CLI
# See MCP client documentation for integration
```

### 3. Build a Container

```bash
# Build operations are available through MCP tools
# Use build tool via MCP client with parameters:
# - repository: path to source code
# - dockerfile: path to Dockerfile (optional)
# - tag: container image tag
# - platforms: target platforms (linux/amd64, linux/arm64)
```

### 4. Security Scanning

```bash
# Security scanning via MCP scan tool
# Supports Trivy and Grype scanners
# Parameters: image, severity_filter, output_format
```

### 5. Deploy to Kubernetes

```bash
# Deploy via MCP deploy tool
# Generates Kubernetes manifests and applies them
# Parameters: image, kubeconfig, namespace, replicas
```

## Configuration

### Environment Variables

```bash
# Set log level
export CONTAINER_KIT_LOG_LEVEL=debug

# Enable tracing
export CONTAINER_KIT_TRACING_ENABLED=true

# Set timeout
export CONTAINER_KIT_TIMEOUT=5m

# Storage path for BoltDB
export CONTAINER_KIT_STORAGE_PATH=~/.container-kit/data
```

### Configuration File

Container Kit uses configuration defined in the domain layer:

```yaml
# Configuration is handled through domain/config package
# with tag-based validation DSL

# Example session configuration
session:
  workspace_dir: "/tmp/container-kit-sessions"
  cleanup_interval: "1h"
  max_sessions: 100

# Tool configuration
tools:
  timeout: "30s"
  retry_attempts: 3
  concurrent_limit: 10

# Storage configuration
storage:
  type: "boltdb"
  path: "~/.container-kit/data"

# Monitoring
monitoring:
  metrics_enabled: true
  tracing_enabled: true
  prometheus_port: 9090
```

## Common Workflows

### Containerize a Node.js Application

```bash
# Container Kit operates through MCP protocol
# Connect your MCP client and use the following workflow:

# 1. Use analyze tool to examine repository
# 2. Use build tool to create container image
# 3. Use scan tool to check for vulnerabilities
# 4. Use deploy tool to generate Kubernetes manifests

# All operations are performed through MCP tool calls
# See MCP client documentation for specific implementation
```

### Multi-Stage Python Build

```bash
# Multi-stage workflows are supported through the workflow engine
# Located in pkg/mcp/application/workflows/

# Workflow execution involves:
# 1. Session management with BoltDB persistence
# 2. Tool orchestration through the registry
# 3. State management and checkpointing
# 4. Error handling and recovery

# Example workflow structure:
# - Stage 1: analyze tool (repository analysis)
# - Stage 2: build tool (container creation)
# - Stage 3: scan tool (security validation)
# - Stage 4: deploy tool (Kubernetes deployment)

# Workflows are defined through the MCP protocol
# and executed by the workflow engine
```

## Troubleshooting

### Common Issues

1. **Build failures**: Check Docker daemon is running
2. **Permission denied**: Ensure proper file permissions
3. **Timeout errors**: Increase timeout values
4. **Memory issues**: Adjust resource limits

### Debug Mode

```bash
# Enable debug logging
export CONTAINER_KIT_LOG_LEVEL=debug

# Run MCP server with debug output
./container-kit-mcp --log-level debug

# Check logs (structured logging with zerolog)
tail -f ~/.container-kit/logs/mcp-server.log

# Performance monitoring
make bench              # Run benchmarks
make coverage-html      # Generate coverage report
```

## Next Steps

- [API Documentation](../api/README.md)
- [Examples](../examples/README.md)
- [Architecture Guide](../architecture/README.md)
- [Adding New Tools](../ADDING_NEW_TOOLS.md)
- [Three-Layer Architecture](../THREE_LAYER_ARCHITECTURE.md)
- [Tool Development Guide](../TOOL_GUIDE.md)
