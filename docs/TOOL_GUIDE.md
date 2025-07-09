# Container Kit MCP Tool Guide

This guide provides comprehensive information about Container Kit's MCP tools, HTTP access, and usage examples.

> **📖 For Implementation**: See [MCP_TOOL_STANDARDS.md](./MCP_TOOL_STANDARDS.md) for canonical implementation patterns and [ADDING_NEW_TOOLS.md](./ADDING_NEW_TOOLS.md) for step-by-step development guide.

## Table of Contents
1. [HTTP API Access](#http-api-access)
2. [Available Tools](#available-tools)
3. [Usage Examples](#usage-examples)

## Reference Documentation

- **[MCP_TOOL_STANDARDS.md](./MCP_TOOL_STANDARDS.md)** - Canonical implementation patterns and standards
- **[ADDING_NEW_TOOLS.md](./ADDING_NEW_TOOLS.md)** - Step-by-step tool development guide
- **[TAG_BASED_VALIDATION.md](./TAG_BASED_VALIDATION.md)** - Validation system documentation

## HTTP API Access

The MCP server provides HTTP endpoints for tool access and schema discovery.

### Available Endpoints

⚠️ **Current Status**: HTTP endpoints are planned but may not be fully implemented. Use MCP stdio protocol for reliable access.

#### MCP Protocol Access
The primary interface is through the MCP (Model Context Protocol) using stdio:

```bash
# Start MCP server
./container-kit-mcp

# List available tools
echo '{"method": "tools/list"}' | ./container-kit-mcp

# Get tool schema
echo '{"method": "tools/schema", "params": {"name": "analyze_repository"}}' | ./container-kit-mcp
```

### Example Response

```json
{
  "tools": [
    {
      "name": "analyze_repository_atomic",
      "description": "Analyze repository structure and generate Dockerfile recommendations",
      "endpoint": "/api/v1/tools/analyze_repository_atomic",
      "parameters": {
        "session_id": "Session ID for context management (optional)",
        "repo_url": "Repository URL to analyze (required)",
        "target_arch": "Target architecture (optional, default: amd64)"
      },
      "category": "analyze",
      "version": "1.0.0"
    }
  ]
}
```

## Available Tools

✅ **Current Implementation Status**: Container Kit now has a comprehensive set of containerization tools implemented and ready for use.

### Core Containerization Tools

#### `analyze_repository`
Analyzes repository structure and generates Dockerfile recommendations with real analysis engine.

**Parameters:**
- `repo_url` (string, required) - Repository URL to analyze
- `context` (string, optional) - Analysis context
- `branch` (string, optional) - Git branch to analyze
- `language_hint` (string, optional) - Language hint for analysis
- `shallow` (boolean, optional) - Perform shallow analysis

**Returns:**
- Comprehensive analysis including language, framework, dependencies
- Database detection and configuration analysis
- Entry points, build files, and suggestions
- Generated Dockerfile based on analysis
- Session ID for workflow continuation

#### `generate_dockerfile`
Generates optimized Dockerfile based on templates and analysis.

**Parameters:**
- `base_image` (string, optional) - Base Docker image to use
- `template` (string, optional) - Template type (go, nodejs, python, java, alpine)
- `optimization` (string, optional) - Optimization level
- `include_health_check` (boolean, optional) - Include health check
- `build_args` (map, optional) - Build arguments
- `platform` (string, optional) - Target platform
- `session_id` (string, optional) - Session context
- `dry_run` (boolean, optional) - Dry run mode

**Returns:**
- Generated Dockerfile content
- Dockerfile path information
- Success status and messages

#### `build_image`
Builds Docker images from Dockerfile with comprehensive options.

**Parameters:**
- `image_name` (string, required) - Docker image name
- `image_tag` (string, optional) - Image tag (default: latest)
- `dockerfile_path` (string, optional) - Path to Dockerfile
- `build_context` (string, optional) - Build context path
- `platform` (string, optional) - Target platform
- `no_cache` (boolean, optional) - Disable build cache
- `build_args` (map, optional) - Build arguments
- `session_id` (string, optional) - Session context

**Returns:**
- Built image name, tag, and ID
- Build time and status
- Success confirmation

#### `push_image`
Pushes Docker images to container registries.

**Parameters:**
- `image_name` (string, required) - Docker image name
- `image_tag` (string, optional) - Image tag (default: latest)
- `registry` (string, optional) - Registry URL (default: docker.io)
- `session_id` (string, optional) - Session context

**Returns:**
- Full image reference
- Registry information
- Push time and status

#### `generate_manifests`
Generates Kubernetes manifests for containerized applications.

**Parameters:**
- `session_id` (string, required) - Session context
- `app_name` (string, required) - Application name
- `image_ref` (map, required) - Image reference with registry/repository/tag
- `namespace` (string, optional) - Kubernetes namespace
- `service_type` (string, optional) - Service type (ClusterIP, NodePort, LoadBalancer)
- `replicas` (int, optional) - Number of replicas
- `resources` (map, optional) - Resource requests/limits
- `environment` (map, optional) - Environment variables
- `secrets` (array, optional) - Secret configurations
- `include_ingress` (boolean, optional) - Include Ingress resource
- `helm_template` (boolean, optional) - Generate Helm template
- `configmap_data` (map, optional) - ConfigMap data
- `ingress_hosts` (array, optional) - Ingress host configurations
- `service_ports` (array, optional) - Service port configurations
- `include_network_policy` (boolean, optional) - Include NetworkPolicy

**Returns:**
- Generated Kubernetes manifests (Deployment, Service, etc.)
- Validation status
- Success confirmation

#### `scan_image`
Scans Docker images for security vulnerabilities.

**Parameters:**
- `image_name` (string, required) - Docker image to scan
- `image_tag` (string, optional) - Image tag to scan
- `session_id` (string, optional) - Session context

**Returns:**
- Vulnerability counts by severity (critical, high, medium, low)
- Scan time and image reference
- Total vulnerabilities found

### Session Management Tools

#### `list_sessions`
Lists all active and recent MCP sessions.

**Parameters:**
- `limit` (int, optional) - Maximum number of sessions to return

**Returns:**
- Array of session summaries with metadata
- Total session count
- Session status and timestamps

### Diagnostic Tools

#### `ping`
Diagnostic tool for testing MCP connectivity.

**Parameters:**
- `message` (string, optional) - Custom message to echo

**Returns:**
- Pong response with timestamp
- Echo of custom message if provided

#### `server_status`
Returns comprehensive server status information.

**Parameters:**
- `details` (boolean, optional) - Include detailed information

**Returns:**
- Server status, version, and uptime
- Runtime information

## Usage Examples

### Complete Containerization Workflow

#### 1. Analyze Repository
```bash
# Analyze a repository for containerization
echo '{
  "method": "tools/call",
  "params": {
    "name": "analyze_repository",
    "arguments": {
      "repo_url": "https://github.com/user/my-app",
      "language_hint": "node"
    }
  }
}' | ./container-kit-mcp
```

#### 2. Generate Dockerfile
```bash
# Generate optimized Dockerfile
echo '{
  "method": "tools/call",
  "params": {
    "name": "generate_dockerfile",
    "arguments": {
      "template": "nodejs",
      "include_health_check": true,
      "build_args": {
        "NODE_ENV": "production"
      }
    }
  }
}' | ./container-kit-mcp
```

#### 3. Build Docker Image
```bash
# Build Docker image
echo '{
  "method": "tools/call",
  "params": {
    "name": "build_image",
    "arguments": {
      "image_name": "my-app",
      "image_tag": "v1.0.0",
      "dockerfile_path": "./Dockerfile",
      "build_context": "."
    }
  }
}' | ./container-kit-mcp
```

#### 4. Push to Registry
```bash
# Push image to registry
echo '{
  "method": "tools/call",
  "params": {
    "name": "push_image",
    "arguments": {
      "image_name": "my-app",
      "image_tag": "v1.0.0",
      "registry": "docker.io"
    }
  }
}' | ./container-kit-mcp
```

#### 5. Generate Kubernetes Manifests
```bash
# Generate Kubernetes manifests
echo '{
  "method": "tools/call",
  "params": {
    "name": "generate_manifests",
    "arguments": {
      "session_id": "session_123",
      "app_name": "my-app",
      "image_ref": {
        "registry": "docker.io",
        "repository": "my-app",
        "tag": "v1.0.0"
      },
      "namespace": "production",
      "service_type": "ClusterIP",
      "replicas": 3,
      "include_ingress": true
    }
  }
}' | ./container-kit-mcp
```

#### 6. Security Scan
```bash
# Scan image for vulnerabilities
echo '{
  "method": "tools/call",
  "params": {
    "name": "scan_image",
    "arguments": {
      "image_name": "my-app",
      "image_tag": "v1.0.0"
    }
  }
}' | ./container-kit-mcp
```

### Session Management

#### List Sessions
```bash
# List active sessions
echo '{
  "method": "tools/call",
  "params": {
    "name": "list_sessions",
    "arguments": {
      "limit": 10
    }
  }
}' | ./container-kit-mcp
```

### Diagnostic Commands

#### Test Connectivity
```bash
# Ping the server
echo '{
  "method": "tools/call",
  "params": {
    "name": "ping",
    "arguments": {
      "message": "hello"
    }
  }
}' | ./container-kit-mcp
```

#### Check Server Status
```bash
# Get server status
echo '{
  "method": "tools/call",
  "params": {
    "name": "server_status",
    "arguments": {
      "details": true
    }
  }
}' | ./container-kit-mcp
```

## Implementation Guide

**For detailed implementation guidance, see:**
- **[ADDING_NEW_TOOLS.md](./ADDING_NEW_TOOLS.md)** - Complete step-by-step development guide
- **[MCP_TOOL_STANDARDS.md](./MCP_TOOL_STANDARDS.md)** - Canonical patterns and compliance requirements

## Testing

Use these commands to test tools:

```bash
# Build and start MCP server
make mcp
./container-kit-mcp

# In another terminal, test tools using MCP protocol
echo '{"method": "tools/list"}' | ./container-kit-mcp

# Test analyze repository tool
echo '{
  "method": "tools/call",
  "params": {
    "name": "analyze_repository",
    "arguments": {
      "repo_url": "https://github.com/user/repo"
    }
  }
}' | ./container-kit-mcp
```

This guide provides the complete reference for working with Container Kit's MCP tools, from understanding the standards to implementing new tools and accessing them via HTTP API.
