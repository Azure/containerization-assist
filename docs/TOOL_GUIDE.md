# Complete Tool Guide - Container Kit MCP

This guide provides comprehensive information about Container Kit's MCP tools, including implementation standards, HTTP access patterns, and usage examples.

## Table of Contents
1. [Tool Standards](#tool-standards)
2. [HTTP API Access](#http-api-access)
3. [Available Tools](#available-tools)
4. [Usage Examples](#usage-examples)
5. [Implementation Guide](#implementation-guide)

## Tool Standards

All MCP tools follow canonical patterns defined for consistency and reliability.

### Required Interface

All tools implement the `api.Tool` interface:

```go
type Tool interface {
    Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error)
    Name() string
    Description() string
    Schema() api.ToolSchema
}
```

### Standard Input/Output

Tools use unified input and output structures:

```go
type ToolInput struct {
    SessionID string                 `json:"session_id,omitempty"`
    Data      map[string]interface{} `json:"data"`
}

type ToolOutput struct {
    Success bool                   `json:"success"`
    Data    map[string]interface{} `json:"data,omitempty"`
    Error   string                 `json:"error,omitempty"`
}
```

### Error Handling Standard

Tools MUST return both `ToolOutput` and error consistently:

```go
func (t *Tool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
    // Validation errors
    if err := validateInput(input); err != nil {
        return api.ToolOutput{
            Success: false,
            Error:   err.Error(),
        }, err
    }

    // Success case
    return api.ToolOutput{
        Success: true,
        Data:    map[string]interface{}{"result": result},
    }, nil
}
```

## HTTP API Access

The MCP server provides HTTP endpoints for tool access and schema discovery.

### Available Endpoints

#### 1. List Tools with Schema
```
GET /api/v1/tools?include_schema=true
```

Returns all available tools with optional schema information.

#### 2. Get All Tool Schemas
```
GET /api/v1/tools/schemas
```

Returns comprehensive schema information for all registered tools.

#### 3. Get Specific Tool Schema
```
GET /api/v1/tools/{toolName}/schema
```

Returns detailed schema information for a specific tool.

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

### Analyze Domain

#### `analyze_repository_atomic`
Analyzes repository structure and identifies containerization requirements.

**Parameters:**
- `repo_url` (string, required) - Repository URL to analyze
- `target_arch` (string, optional) - Target architecture (default: amd64)
- `analysis_depth` (string, optional) - Analysis depth (basic/standard/deep)

**Returns:**
- Analysis results (language, framework, dependencies)
- Containerization suggestions and readiness assessment
- Session ID for subsequent operations

#### `generate_dockerfile`
Generates optimized Dockerfile based on repository analysis.

**Parameters:**
- `session_id` (string, required) - Session from analyze_repository
- `language` (string, optional) - Override detected language
- `framework` (string, optional) - Override detected framework

#### `validate_dockerfile_atomic`
Validates Dockerfiles against best practices and security guidelines.

**Parameters:**
- `dockerfile_path` (string, optional) - Path to Dockerfile
- `dockerfile_content` (string, optional) - Dockerfile content to validate

### Deploy Domain

#### `generate_manifests_atomic`
Generates Kubernetes deployment manifests.

**Parameters:**
- `session_id` (string, required) - Session context
- `app_name` (string, required) - Application name
- `image_ref` (string, required) - Docker image reference
- `namespace` (string, optional) - Kubernetes namespace
- `port` (int, optional) - Application port

#### `deploy_kubernetes_atomic`
Deploys applications to Kubernetes with comprehensive fixing capabilities.

**Parameters:**
- `session_id` (string, required) - Session context
- `app_name` (string, required) - Application name
- `image_ref` (string, required) - Docker image reference
- `wait_for_ready` (boolean, optional) - Wait for deployment to be ready

#### `check_health_atomic`
Checks the health status of deployed applications.

**Parameters:**
- `deployment` (string, optional) - Specific deployment name
- `namespace` (string, optional) - Kubernetes namespace

### Scan Domain

#### `scan_image_security_atomic`
Performs comprehensive security scanning of Docker images.

**Parameters:**
- `image_name` (string, required) - Docker image to scan
- `severity_threshold` (string, optional) - Minimum severity to report
- `include_fixable` (boolean, optional) - Include only fixable vulnerabilities

#### `scan_secrets_atomic`
Scans for hardcoded secrets and credentials.

**Parameters:**
- `scan_path` (string, optional) - Path to scan (defaults to session workspace)
- `file_patterns` (array, optional) - File patterns to include
- `suggest_remediation` (boolean, optional) - Provide remediation suggestions

### Session Domain

#### `list_sessions`
Lists all active MCP sessions.

**Returns:**
- Array of session information
- Session states and timestamps
- Associated labels for each session

#### `delete_session`
Deletes a session and its workspace.

**Parameters:**
- `session_id` (string, required) - Session to delete

#### `manage_session_labels`
Manage labels for session organization and filtering.

**Sub-commands:**
- `add_session_label` - Add a label to a session
- `remove_session_label` - Remove a label from a session
- `list_session_labels` - List all labels for a session

## Usage Examples

### Basic Containerization Workflow

```bash
# 1. Analyze repository
curl -X POST "http://localhost:8080/api/v1/tools/analyze_repository_atomic" \
  -H "Content-Type: application/json" \
  -d '{
    "data": {
      "repo_url": "https://github.com/user/my-app"
    }
  }'

# 2. Generate Dockerfile (using session from step 1)
curl -X POST "http://localhost:8080/api/v1/tools/generate_dockerfile" \
  -H "Content-Type: application/json" \
  -d '{
    "session_id": "session-from-step1",
    "data": {
      "language": "node",
      "framework": "express"
    }
  }'

# 3. Generate Kubernetes manifests
curl -X POST "http://localhost:8080/api/v1/tools/generate_manifests_atomic" \
  -H "Content-Type: application/json" \
  -d '{
    "session_id": "session-from-step1",
    "data": {
      "app_name": "my-app",
      "image_ref": "my-app:latest",
      "port": 3000
    }
  }'
```

### Security Scanning

```bash
# Scan image for vulnerabilities
curl -X POST "http://localhost:8080/api/v1/tools/scan_image_security_atomic" \
  -H "Content-Type: application/json" \
  -d '{
    "data": {
      "image_name": "nginx:latest",
      "severity_threshold": "HIGH",
      "include_fixable": true
    }
  }'

# Scan for secrets
curl -X POST "http://localhost:8080/api/v1/tools/scan_secrets_atomic" \
  -H "Content-Type: application/json" \
  -d '{
    "session_id": "my-session",
    "data": {
      "scan_path": "/path/to/code",
      "suggest_remediation": true
    }
  }'
```

### Session Management

```bash
# List all sessions
curl "http://localhost:8080/api/v1/tools/list_sessions"

# Delete a session
curl -X POST "http://localhost:8080/api/v1/tools/delete_session" \
  -H "Content-Type: application/json" \
  -d '{
    "data": {
      "session_id": "session-to-delete"
    }
  }'
```

## Implementation Guide

### Adding New Tools

1. **Choose Domain**: Place in appropriate domain package (`pkg/mcp/domain/`)
2. **Implement Interface**: Follow `api.Tool` interface
3. **Standard Patterns**: Use `ToolInput`/`ToolOutput` structures
4. **Validation**: Add comprehensive input validation
5. **Testing**: Include unit and integration tests

### Example Tool Implementation

```go
package mydomain

type MyTool struct {
    sessionManager session.Manager
    logger         zerolog.Logger
}

func (t *MyTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
    // Extract parameters
    var params MyToolParams
    if err := mapstructure.Decode(input.Data, &params); err != nil {
        return api.ToolOutput{
            Success: false,
            Error:   "Invalid input parameters",
        }, err
    }

    // Perform operation
    result, err := t.performOperation(params)
    if err != nil {
        return api.ToolOutput{
            Success: false,
            Error:   err.Error(),
        }, err
    }

    return api.ToolOutput{
        Success: true,
        Data:    map[string]interface{}{"result": result},
    }, nil
}
```

### Compliance Checklist

- [ ] Tool implements `api.Tool` interface
- [ ] Uses standard `ToolInput`/`ToolOutput` structures
- [ ] Returns both ToolOutput and error consistently
- [ ] Has comprehensive input validation
- [ ] Includes session management integration
- [ ] Has unit and integration tests
- [ ] Documentation includes usage examples

## Testing

Use these commands to test tools:

```bash
# Start MCP server
make mcp
./container-kit-mcp &

# Test HTTP endpoints
curl "http://localhost:8080/api/v1/tools"
curl "http://localhost:8080/api/v1/tools/schemas"
curl "http://localhost:8080/api/v1/tools/analyze_repository_atomic/schema"

# Execute a tool
curl -X POST "http://localhost:8080/api/v1/tools/analyze_repository_atomic" \
  -H "Content-Type: application/json" \
  -d '{"data": {"repo_url": "https://github.com/user/repo"}}'
```

This guide provides the complete reference for working with Container Kit's MCP tools, from understanding the standards to implementing new tools and accessing them via HTTP API.
