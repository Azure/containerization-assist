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

### 2. Available Tools

Container Kit provides **12 production-ready tools** through MCP protocol:

#### Containerization Tools (6 tools)
- **`analyze_repository`** - Repository analysis and framework detection
- **`generate_dockerfile`** - AI-powered Dockerfile generation
- **`build_image`** - Docker image building with optimization
- **`push_image`** - Container registry push operations
- **`generate_manifests`** - Kubernetes manifest generation
- **`scan_image`** - Security vulnerability scanning

#### File Access Tools (3 tools)
- **`read_file`** - Secure file reading within session workspace
- **`list_directory`** - Directory listing with path validation
- **`file_exists`** - File existence checking

#### Session & Diagnostic Tools (3 tools)
- **`list_sessions`** - Session management and listing
- **`ping`** - Connectivity testing
- **`server_status`** - Server health and status

### 3. Complete Containerization Workflow

```bash
# Example workflow through MCP client:
# 1. Analyze repository
# 2. Generate Dockerfile
# 3. Build container image
# 4. Scan for vulnerabilities
# 5. Generate Kubernetes manifests
# 6. Push to registry

# All operations use session-based state management
# and support dry-run mode for testing
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

# Storage path for BoltDB session persistence
export CONTAINER_KIT_STORAGE_PATH=~/.container-kit/data

# Session configuration
export CONTAINER_KIT_SESSION_TIMEOUT=30m
export CONTAINER_KIT_MAX_SESSIONS=100
```

### Configuration File

Container Kit uses configuration defined in the domain layer with tag-based validation DSL:

```yaml
# Configuration is handled through domain/config package
# with tag-based validation DSL (ADR-005)

# Session configuration
session:
  workspace_dir: "/tmp/container-kit-sessions"
  cleanup_interval: "1h"
  max_sessions: 100
  timeout: "30m"

# Tool configuration
tools:
  timeout: "30s"
  retry_attempts: 3
  concurrent_limit: 10
  file_access:
    max_file_size: "10MB"
    blocked_paths: [".git", "node_modules", ".env"]

# Storage configuration (BoltDB)
storage:
  type: "boltdb"
  path: "~/.container-kit/data"
  backup_interval: "1h"

# FileAccessService configuration
file_access:
  max_file_size: "10MB"
  blocked_extensions: [".exe", ".bin"]
  security_validation: true

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

# Complete workflow using session continuity:
# 1. analyze_repository - Language detection and framework analysis
# 2. generate_dockerfile - AI-powered Dockerfile generation
# 3. build_image - Docker image building with optimization
# 4. scan_image - Security vulnerability scanning
# 5. generate_manifests - Kubernetes manifest generation
# 6. push_image - Registry push operations

# Session-based state management:
# - Each workflow creates a session with unique workspace
# - FileAccessService provides secure file operations
# - State persisted in BoltDB for recovery
# - Session metadata tracks progress and results

# Example session workflow:
# Session ID: session-abc123
# Workspace: /tmp/container-kit-sessions/session-abc123
# All tools share session state and workspace
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

# Check logs (structured logging with slog)
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
