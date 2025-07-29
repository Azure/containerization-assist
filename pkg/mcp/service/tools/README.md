# Container Kit Tool Registry

This package contains the consolidated tool registration system for Container Kit's MCP implementation.

## Overview

The tool registry provides a table-driven approach to tool registration that:
- Reduces code duplication by 80%
- Centralizes all tool configurations
- Makes adding new tools simple
- Ensures consistency across all tools

## Architecture

### Core Components

1. **ToolConfig** - Defines the configuration for each tool
2. **ToolDependencies** - Contains all possible dependencies a tool might need
3. **Registration Functions** - Generic handlers for different tool categories
4. **Helper Functions** - Utilities for workflow state management

### Tool Categories

- **Workflow Tools** (10 tools) - Individual steps in the containerization workflow
- **Orchestration Tools** (2 tools) - Workflow management and status
- **Utility Tools** (3 tools) - Helper tools like list_tools, ping, server_status

## Tool Definitions

### Workflow Tools

| Tool | Description | Next Tool |
|------|-------------|-----------|
| `analyze_repository` | Analyze repository to detect language and framework | `generate_dockerfile` |
| `generate_dockerfile` | Generate optimized Dockerfile | `build_image` |
| `build_image` | Build Docker image | `scan_image` |
| `scan_image` | Security vulnerability scan | `tag_image` |
| `tag_image` | Tag Docker image | `push_image` |
| `push_image` | Push to container registry | `generate_k8s_manifests` |
| `generate_k8s_manifests` | Generate Kubernetes manifests | `prepare_cluster` |
| `prepare_cluster` | Prepare Kubernetes cluster | `deploy_application` |
| `deploy_application` | Deploy to Kubernetes | `verify_deployment` |
| `verify_deployment` | Verify deployment health | (end) |

### Orchestration Tools

| Tool | Description |
|------|-------------|
| `start_workflow` | Start complete containerization workflow |
| `workflow_status` | Check workflow progress and status |

### Utility Tools

| Tool | Description |
|------|-------------|
| `list_tools` | List all available tools |
| `ping` | Test MCP connectivity |
| `server_status` | Get server status information |

## Usage

### Registering All Tools

```go
import "github.com/Azure/container-kit/pkg/mcp/service/tools"

// Create dependencies
deps := tools.ToolDependencies{
    StepProvider:    stepProvider,
    ProgressFactory: progressFactory,
    SessionManager:  sessionManager,
    Logger:          logger,
}

// Register all tools
err := tools.RegisterTools(mcpServer, deps)
```

### Adding a New Tool

1. Add the tool configuration to `toolConfigs` in `registry.go`:

```go
{
    Name:                 "my_new_tool",
    Description:          "Description of the tool",
    Category:             CategoryWorkflow,
    RequiredParams:       []string{"session_id"},
    NeedsStepProvider:    true,
    NeedsProgressFactory: true,
    NeedsSessionManager:  true,
    NeedsLogger:          true,
    StepGetterName:       "GetMyNewStep",
    NextTool:             "next_tool_name",
    ChainReason:          "Tool completed successfully",
}
```

2. Add the corresponding step method to your StepProvider:

```go
func (p *StepProviderImpl) GetMyNewStep() domainworkflow.Step {
    return &MyNewStep{
        // implementation
    }
}
```

That's it! No new files or boilerplate needed.

### Custom Tool Handlers

For tools that need special logic beyond the standard patterns:

```go
{
    Name:        "special_tool",
    Description: "Tool with custom behavior",
    Category:    CategoryUtility,
    CustomHandler: func(deps ToolDependencies) mcp.HandlerFunc {
        return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
            // Custom implementation
        }
    },
}
```

## Tool Configuration Structure

```go
type ToolConfig struct {
    // Basic metadata
    Name        string
    Description string
    Category    ToolCategory

    // Input parameters
    RequiredParams []string
    OptionalParams map[string]interface{}

    // Dependencies
    NeedsStepProvider    bool
    NeedsProgressFactory bool
    NeedsSessionManager  bool
    NeedsLogger          bool

    // Workflow configuration
    StepGetterName string  // Method name on StepProvider
    NextTool       string  // Next tool in chain
    ChainReason    string  // Reason for chain

    // Optional custom handler
    CustomHandler func(deps ToolDependencies) mcp.HandlerFunc
}
```

## Helper Functions

The package includes utilities for:
- Session state management (`LoadWorkflowState`, `SaveWorkflowState`)
- Session ID generation (`GenerateSessionID`)
- Parameter extraction (`ExtractStringParam`, `ExtractOptionalStringParam`)
- Progress emitter creation (`CreateProgressEmitter`)

## Testing

See `registry_test.go` and `helpers_test.go` for comprehensive test examples.

## Migration from Old System

If migrating from the old individual tool registration:

1. Remove all imports of old workflow packages
2. Replace individual tool registration calls with `tools.RegisterTools()`
3. Delete old tool registration files
4. Update any custom tool logic to use the `CustomHandler` field

See `MIGRATION.md` for detailed migration instructions.