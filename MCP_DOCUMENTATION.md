# Container Kit MCP Server - Complete Documentation

This document provides comprehensive documentation for the Container Kit MCP (Model Context Protocol) server, which enables AI assistants to containerize applications and generate Kubernetes manifests.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Installation & Setup](#installation--setup)
- [Available Tools](#available-tools)
- [Usage Examples](#usage-examples)
- [Configuration](#configuration)
- [Troubleshooting](#troubleshooting)
- [Security Considerations](#security-considerations)
- [API Reference](#api-reference)
- [Development](#development)

## Overview

Container Kit's MCP server provides two primary operation modes:

1. **Atomic Tools** - Deterministic, composable operations for specific containerization tasks
2. **Conversation Mode** - Guided workflow through the `chat` tool with stage-based progression

The MCP server eliminates the need for external API keys by leveraging the calling AI assistant's language model capabilities.

## Architecture

### Core Components

- **MCP Server** (`pkg/mcp/internal/core/server.go`) - Main server handling MCP protocol
- **Tool Registry** (`pkg/mcp/internal/runtime/`) - Automatic tool registration and discovery
- **Conversation Engine** (`pkg/mcp/internal/conversation/`) - Guided workflow management
- **Session Management** (`pkg/mcp/internal/session/`) - Persistent session state with label support
- **Transport Layer** (`pkg/mcp/internal/transport/`) - Communication protocols (stdio, HTTP)
- **Workflow Engine** (`pkg/mcp/internal/workflow/`) - Multi-tool workflow orchestration
- **Observability** (`pkg/mcp/internal/observability/`) - Metrics and distributed tracing

### Operation Modes

#### Atomic Tools Mode
- Each tool performs a single, well-defined task
- Composable and can be used independently
- Session state managed persistently
- No AI reasoning within tools themselves

#### Conversation Mode
- Guided workflow through `chat` tool
- Stateless prompt manager with stage-based routing
- User preferences and session persistence
- Built-in telemetry and error recovery

## Installation & Setup

### Prerequisites

- Go 1.21+
- Docker
- kubectl (optional, for Kubernetes features)
- kind (optional, for local Kubernetes testing)

### Building from Source

```bash
git clone https://github.com/Azure/container-kit.git
cd container-kit

# Build the MCP server (recommended)
make mcp

# Alternative: direct go build
go build -tags mcp -o container-kit-mcp ./cmd/mcp-server

# Verify installation
./container-kit-mcp --version
```

### Development Setup

For contributors, use the development container for instant setup:

```bash
# Prerequisites: VS Code + Dev Containers extension
git clone https://github.com/Azure/container-kit.git
cd container-kit
code .  # Open in VS Code, click "Reopen in Container"
```

See [Development Container Guide](.devcontainer/README.md) for details.

### Configuration with AI Assistants

#### Claude Desktop

Add to your Claude Desktop configuration file:

**macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
**Linux**: `~/.config/Claude/claude_desktop_config.json`
**Windows**: `%APPDATA%\Claude\claude_desktop_config.json`

```json
{
  "mcpServers": {
    "container-kit": {
      "command": "/path/to/container-kit/container-kit-mcp",
      "args": ["--transport=stdio"]
    }
  }
}
```

#### Other MCP Clients

For HTTP transport:
```bash
./container-kit-mcp --transport=http --port=8080
```

Then configure your MCP client to connect to `http://localhost:8080`.

## Available Tools

### Core Containerization Tools

#### `analyze_repository_atomic`
Analyzes repository structure and identifies containerization requirements.

**Parameters:**
- `repo_url` (string) - Local path or Git URL to analyze
- `branch` (string, optional) - Git branch to analyze
- `dry_run` (boolean, optional) - Validate without cloning

**Returns:**
- Session ID for subsequent operations
- Analysis results (language, framework, dependencies)
- Containerization suggestions and readiness assessment

#### `generate_dockerfile`
Generates optimized Dockerfile based on repository analysis.

**Parameters:**
- `session_id` (string) - Session from analyze_repository
- `language` (string, optional) - Override detected language
- `framework` (string, optional) - Override detected framework
- `custom_instructions` (string, optional) - Additional requirements

**Returns:**
- Generated Dockerfile content
- Validation results and optimization suggestions

#### `build_image_atomic`
Builds Docker image from generated Dockerfile with AI-driven fixing capabilities.

**Parameters:**
- `session_id` (string) - Session with generated Dockerfile
- `image_name` (string, optional) - Custom image name
- `image_tag` (string, optional) - Custom image tag
- `platform` (string, optional) - Target platform (e.g., linux/amd64)
- `push_after_build` (boolean, optional) - Auto-push after successful build
- `dry_run` (boolean, optional) - Validate without building

**Returns:**
- Build status and logs
- Image reference for subsequent operations
- Build failure analysis and remediation steps

#### `push_image_atomic`
Pushes built image to container registry.

**Parameters:**
- `image_ref` (string) - Full image reference (registry/name:tag)
- `registry_url` (string, optional) - Override registry URL
- `timeout` (int, optional) - Push timeout in seconds
- `retry_count` (int, optional) - Number of retry attempts
- `force` (boolean, optional) - Force push even if image exists

**Returns:**
- Push status and registry information
- Authentication guidance if needed

#### `pull_image_atomic`
Pulls Docker images from registries.

**Parameters:**
- `image_ref` (string) - Full image reference to pull
- `registry_url` (string, optional) - Override registry URL
- `timeout` (int, optional) - Pull timeout in seconds
- `retry_count` (int, optional) - Number of retry attempts

**Returns:**
- Pull status and image information
- Layer cache efficiency analysis

#### `tag_image_atomic`
Tags Docker images with new references.

**Parameters:**
- `source_image` (string) - Source image reference
- `target_image` (string) - Target image reference
- `force` (boolean, optional) - Force tag even if target exists

**Returns:**
- Tag operation status
- Image reference information

#### `generate_manifests_atomic`
Generates Kubernetes deployment manifests with AI-driven fixing.

**Parameters:**
- `session_id` (string) - Session context
- `app_name` (string) - Application name
- `image_ref` (string) - Docker image reference
- `namespace` (string, optional) - Kubernetes namespace
- `port` (int, optional) - Application port
- `replicas` (int, optional) - Number of replicas
- `resources` (object, optional) - Resource limits and requests

**Returns:**
- Generated manifest files
- Deployment configuration and security recommendations

#### `deploy_kubernetes_atomic`
Deploys applications to Kubernetes with comprehensive fixing capabilities.

**Parameters:**
- `session_id` (string) - Session context
- `app_name` (string) - Application name
- `image_ref` (string) - Docker image reference
- `namespace` (string, optional) - Kubernetes namespace
- `wait_for_ready` (boolean, optional) - Wait for deployment to be ready
- `timeout` (int, optional) - Deployment timeout in seconds

**Returns:**
- Deployment status and health information
- Detailed failure analysis and remediation guidance

#### `check_health_atomic`
Checks the health status of deployed applications.

**Parameters:**
- `deployment` (string, optional) - Specific deployment name
- `namespace` (string, optional) - Kubernetes namespace
- `wait_time` (int, optional) - Wait time for health check

**Returns:**
- Health status of deployments
- Pod status, readiness, and diagnostic information

### Security and Validation Tools

#### `scan_image_security_atomic`
Performs comprehensive security scanning of Docker images.

**Parameters:**
- `image_name` (string) - Docker image to scan
- `severity_threshold` (string, optional) - Minimum severity to report
- `vuln_types` (array, optional) - Types of vulnerabilities to scan for
- `include_fixable` (boolean, optional) - Include only fixable vulnerabilities
- `fail_on_critical` (boolean, optional) - Fail if critical vulnerabilities found

**Returns:**
- Vulnerability scan results and security score
- Critical security findings and remediation recommendations

#### `scan_secrets_atomic`
Scans for hardcoded secrets and credentials in code and manifests.

**Parameters:**
- `scan_path` (string, optional) - Path to scan (defaults to session workspace)
- `file_patterns` (array, optional) - File patterns to include
- `scan_dockerfiles` (boolean, optional) - Include Dockerfiles in scan
- `scan_manifests` (boolean, optional) - Include Kubernetes manifests
- `suggest_remediation` (boolean, optional) - Provide remediation suggestions

**Returns:**
- Detected secrets and security score
- Kubernetes Secret manifest generation for found secrets

#### `validate_dockerfile_atomic`
Validates Dockerfiles against best practices and security guidelines.

**Parameters:**
- `dockerfile_path` (string, optional) - Path to Dockerfile
- `dockerfile_content` (string, optional) - Dockerfile content to validate
- `use_hadolint` (boolean, optional) - Use Hadolint for advanced validation
- `check_security` (boolean, optional) - Perform security-focused checks
- `generate_fixes` (boolean, optional) - Generate corrected Dockerfile

**Returns:**
- Validation results with errors and warnings
- Security analysis and optimization recommendations

### Conversation Mode

#### `chat`
Conversational workflow management for guided containerization.

**Parameters:**
- `message` (string) - User message or command
- `session_id` (string, optional) - Continue existing conversation

**Returns:**
- AI assistant response
- Updated session state
- Next step recommendations

### Session Management Tools

#### `list_sessions`
Lists all active MCP sessions.

**Returns:**
- Array of session information
- Session states and timestamps
- Associated labels for each session

#### `delete_session`
Deletes a session and its workspace.

**Parameters:**
- `session_id` (string) - Session to delete

**Returns:**
- Deletion confirmation

#### `manage_session_labels`
Manage labels for session organization and filtering.

**Sub-commands:**
- `add_session_label`: Add a label to a session
  - `session_id` (string) - Session ID
  - `key` (string) - Label key
  - `value` (string) - Label value
- `remove_session_label`: Remove a label from a session
  - `session_id` (string) - Session ID
  - `key` (string) - Label key to remove
- `list_session_labels`: List all labels for a session
  - `session_id` (string) - Session ID

**Returns:**
- Updated label information
- Operation confirmation

### Monitoring and Observability Tools

#### `get_server_health`
Retrieves server health and status information.

**Returns:**
- Server uptime and status
- Available tools and capabilities
- System resource usage

#### `get_logs`
Exports server logs with powerful filtering capabilities.

**Parameters:**
- `level` (string, optional) - Minimum log level (trace, debug, info, warn, error)
- `time_range` (string, optional) - Time range filter (e.g., "5m", "1h", "24h")
- `pattern` (string, optional) - Pattern to search for in log messages
- `limit` (int, optional) - Maximum number of log entries to return
- `format` (string, optional) - Output format ("json" or "text")
- `include_callers` (boolean, optional) - Include source code location

**Returns:**
- Filtered log entries
- Log statistics and metadata

#### `get_telemetry_metrics`
Exports Prometheus metrics from the MCP server.

**Parameters:**
- `format` (string, optional) - Output format ("prometheus" or "json")
- `metric_names` (array, optional) - Filter by specific metric names
- `include_help` (boolean, optional) - Include metric HELP text
- `include_empty` (boolean, optional) - Include metrics with zero values

**Returns:**
- Prometheus-formatted metrics
- Metric counts and server statistics

## Usage Examples

### Basic Containerization Workflow

```
User: "Help me containerize my Python Flask application at /home/user/my-app"

Assistant uses:
1. analyze_repository_atomic(repo_url="/home/user/my-app")
2. generate_dockerfile(session_id="...", language="python", framework="flask")
3. build_image_atomic(session_id="...")
4. generate_manifests_atomic(session_id="...", app_name="my-app")
```

### Guided Conversation Mode

```
User: "I want to containerize and deploy my application"

Assistant uses:
chat(message="I'll help you containerize and deploy your application...")
- Guides through repository analysis
- Assists with Dockerfile creation
- Helps with image building and deployment
- Provides troubleshooting and optimization advice
```

## Testing

### Automated Testing
```bash
# Run all automated tests
./test/integration/run_tests.sh

# Run specific test suites
make test                    # All tests
go test ./pkg/mcp/...       # MCP-specific tests
go test -tags integration   # Integration tests only
```

### Manual Testing with Claude Desktop
For manual testing procedures, see [test/integration/mcp/claude_desktop_test.md](test/integration/mcp/claude_desktop_test.md).

### Quality Assurance
```bash
make lint              # Strict linting (fails on any issue)
make lint-threshold    # Linting with error budget
make lint-report       # Generate detailed reports
```

See [docs/LINTING.md](docs/LINTING.md) for our quality strategy.

## Deployment Models

### MCP Server Deployment Options
- **Development**: Local stdio transport with Claude Desktop
- **Production**: HTTP transport with load balancing
- **Cloud**: Container deployment with persistent volumes
- **Instant Setup**: VS Code devcontainer with all tools pre-configured

### CLI Tool Deployment (Legacy)
- **Local**: Direct execution with local Docker/Kind
- **CI/CD**: Pipeline integration for automated containerization

### Performance Considerations
- Session persistence uses BoltDB for lightweight storage
- Tool registration is automatic via build-time code generation
- HTTP transport supports concurrent connections
- Memory usage scales with active session count
- Circuit breaker patterns prevent cascading failures
- AI context caching reduces redundant operations
- Distributed tracing enables performance monitoring

## Advanced Configuration

### Environment Variables
- `CONTAINER_KIT_LOG_LEVEL`: Set logging level (debug, info, warn, error)
- `CONTAINER_KIT_SESSION_DIR`: Custom session storage directory
- `CONTAINER_KIT_METRICS_PORT`: Metrics server port (default: 8080)
- `CONTAINER_KIT_TRACE_ENABLED`: Enable OpenTelemetry tracing
- `CONTAINER_KIT_TRACE_ENDPOINT`: OpenTelemetry collector endpoint
- `CONTAINER_KIT_REGISTRY_PROVIDER`: Default registry provider (aws-ecr, azure, generic)

### Transport Configuration
#### stdio Transport (Default)
Used with Claude Desktop and terminal applications.

#### HTTP Transport
```bash
./container-kit-mcp --transport=http --port=8080
```

### Session Management
Sessions are automatically created and persisted. You can manage them using:
- `list_sessions` - View all active sessions
- `delete_session` - Clean up specific sessions
- Session data is stored in BoltDB for reliability

## Development and Extension

For detailed development information, see:
- [Architecture Guide](docs/mcp-architecture.md) - Technical system design
- [Tool Development Guide](docs/adding-new-tools.md) - Creating new tools with auto-registration
- [Contributing Guide](CONTRIBUTING.md) - Development workflow
- [Development Guidelines](DEVELOPMENT_GUIDELINES.md) - Coding standards
- [Design Document](DESIGN.md) - Comprehensive architectural design

### Key Development Features
- **Auto-Registration**: Tools automatically register via naming convention
- **Unified Server**: Single server supporting multiple operation modes
- **Built-in Observability**: Prometheus metrics and OpenTelemetry tracing
- **AI Context Management**: Intelligent context caching and aggregation
- **Registry Providers**: Extensible support for container registries
