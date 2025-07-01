# Developer Guide: Adding New Tools to the MCP Server

This guide provides a comprehensive walkthrough for developers adding new tools to the Container Kit MCP server. The MCP server uses a zero-configuration auto-registration system that simplifies tool integration.

## Table of Contents
1. [Architecture Overview](#architecture-overview)
2. [Tool Interface Requirements](#tool-interface-requirements)
3. [Step-by-Step Implementation](#step-by-step-implementation)
4. [Testing Your Tool](#testing-your-tool)
5. [Advanced Topics](#advanced-topics)
6. [Troubleshooting](#troubleshooting)

## Architecture Overview

The MCP server uses an auto-registration system that discovers tools at build time and generates registration code. This eliminates manual registration steps and ensures all tools are consistently integrated.

### Key Components

- **Tool Interface** (`pkg/mcp/core/interfaces.go`): Defines the contract all tools must implement
- **Auto-Registration** (`pkg/mcp/internal/runtime/auto_registration.go`): Generated file mapping tool names to factories
- **Tool Registry** (`pkg/mcp/internal/runtime/registry.go`): Type-safe registry using Go generics
- **Orchestrator** (`pkg/mcp/internal/orchestration/`): Manages tool execution and workflows

## Tool Interface Requirements

Every tool must implement the `core.Tool` interface:

```go
type Tool interface {
    Execute(ctx context.Context, args interface{}) (interface{}, error)
    GetMetadata() ToolMetadata
    Validate(ctx context.Context, args interface{}) error
}
```

### Method Descriptions

1. **Execute**: Performs the tool's primary operation
   - Accepts a context and arguments (typically a struct)
   - Returns results and/or error
   - Should handle timeouts via context

2. **GetMetadata**: Returns tool metadata including:
   - Name (unique identifier)
   - Description (user-facing documentation)
   - InputSchema (JSON schema for arguments)
   - OutputSchema (JSON schema for results)

3. **Validate**: Pre-execution validation
   - Validates input arguments
   - Checks prerequisites
   - Returns error if validation fails

## Step-by-Step Implementation

### 1. Define Your Tool Structure

Create a new file in the appropriate domain package under `pkg/mcp/internal/`:

```go
// pkg/mcp/internal/myfeature/my_tool.go
package myfeature

import (
    "context"
    "github.com/rs/zerolog"
    "container-kit/pkg/mcp/core"
)

type MyToolArgs struct {
    // Define your input parameters with json tags
    ProjectPath string `json:"projectPath" jsonschema:"required,description=Path to the project"`
    Options     MyToolOptions `json:"options,omitempty"`
}

type MyToolOptions struct {
    Verbose bool `json:"verbose,omitempty" jsonschema:"description=Enable verbose output"`
}

type MyToolResult struct {
    // Define your output structure
    Status  string   `json:"status"`
    Details []string `json:"details"`
}

type MyTool struct {
    // Add dependencies your tool needs
    logger zerolog.Logger
    // Add other dependencies like sessionManager, pipelineAdapter, etc.
}
```

### 2. Implement Constructor

```go
func NewMyTool(logger zerolog.Logger) *MyTool {
    return &MyTool{
        logger: logger.With().Str("component", "my-tool").Logger(),
    }
}
```

### 3. Implement Tool Interface Methods

```go
func (t *MyTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
    // Type assert the arguments
    typedArgs, ok := args.(*MyToolArgs)
    if !ok {
        return nil, core.NewExecutionError(
            core.ErrCodeInvalidInput,
            "invalid argument type",
            map[string]interface{}{"expected": "*MyToolArgs", "got": fmt.Sprintf("%T", args)},
        )
    }

    t.logger.Info().
        Str("projectPath", typedArgs.ProjectPath).
        Msg("executing my tool")

    // Implement your tool logic here
    result := &MyToolResult{
        Status: "success",
        Details: []string{"Operation completed"},
    }

    return result, nil
}

func (t *MyTool) GetMetadata() core.ToolMetadata {
    return core.ToolMetadata{
        Name:        "my_tool",
        Description: "Brief description of what your tool does",
        InputSchema: core.GenerateSchema(&MyToolArgs{}),
        OutputSchema: core.GenerateSchema(&MyToolResult{}),
        Examples: []core.ToolExample{
            {
                Description: "Basic usage example",
                Input: &MyToolArgs{
                    ProjectPath: "/path/to/project",
                },
                Output: &MyToolResult{
                    Status: "success",
                    Details: []string{"Operation completed"},
                },
            },
        },
    }
}

func (t *MyTool) Validate(ctx context.Context, args interface{}) error {
    typedArgs, ok := args.(*MyToolArgs)
    if !ok {
        return core.NewValidationError("invalid argument type")
    }

    // Validate required fields
    if typedArgs.ProjectPath == "" {
        return core.NewValidationError("projectPath is required")
    }

    // Add additional validation logic
    if !isValidPath(typedArgs.ProjectPath) {
        return core.NewValidationError("invalid project path")
    }

    return nil
}
```

### 4. Register Your Tool

Add your tool to the registration process in `pkg/mcp/internal/core/gomcp_tools.go`:

```go
func registerAtomicToolsWithOrchestrator(
    orchestrator *orchestration.Orchestrator,
    pipelineAdapter core.PipelineOperations,
    sessionManager core.ToolSessionManager,
    logger zerolog.Logger,
) error {
    // ... existing registrations ...

    // Register your new tool
    myTool := myfeature.NewMyTool(logger)
    if err := orchestrator.RegisterTool("my_tool", myTool); err != nil {
        return fmt.Errorf("failed to register my_tool: %w", err)
    }

    return nil
}
```

### 5. Build and Test

```bash
# Build the MCP server with your new tool
make mcp

# Run tests
make test-mcp

# Test your tool interactively
./bin/mcp server
```

## Testing Your Tool

### 1. Unit Tests

Create a test file alongside your tool:

```go
// pkg/mcp/internal/myfeature/my_tool_test.go
package myfeature

import (
    "context"
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/rs/zerolog"
)

func TestMyTool_Execute(t *testing.T) {
    logger := zerolog.New(zerolog.NewTestWriter(t))
    tool := NewMyTool(logger)

    tests := []struct {
        name    string
        args    *MyToolArgs
        wantErr bool
    }{
        {
            name: "successful execution",
            args: &MyToolArgs{
                ProjectPath: "/valid/path",
            },
            wantErr: false,
        },
        {
            name: "invalid path",
            args: &MyToolArgs{
                ProjectPath: "",
            },
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := tool.Execute(context.Background(), tt.args)
            if tt.wantErr {
                assert.Error(t, err)
                return
            }
            require.NoError(t, err)
            assert.NotNil(t, result)
        })
    }
}
```

### 2. Integration Tests

Test your tool with the MCP server:

```go
func TestMyToolIntegration(t *testing.T) {
    // Set up test MCP server
    server := setupTestServer(t)

    // Execute tool through MCP protocol
    result, err := server.CallTool(context.Background(), "my_tool", map[string]interface{}{
        "projectPath": "/test/path",
    })

    require.NoError(t, err)
    assert.Equal(t, "success", result["status"])
}
```

## Advanced Topics

### 1. Session Management

If your tool needs to maintain state across invocations:

```go
type MyTool struct {
    sessionManager core.ToolSessionManager
    // ... other fields
}

func (t *MyTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
    // Get or create session
    session, err := t.sessionManager.GetOrCreateSession(ctx, "my-tool-session")
    if err != nil {
        return nil, err
    }

    // Store state
    session.Set("lastRun", time.Now())

    // Retrieve state
    if lastRun, ok := session.Get("lastRun").(time.Time); ok {
        t.logger.Info().Time("lastRun", lastRun).Msg("previous run detected")
    }
}
```

### 2. Pipeline Integration

For tools that need to interact with the containerization pipeline:

```go
type MyTool struct {
    pipelineAdapter core.PipelineOperations
    // ... other fields
}

func (t *MyTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
    // Use pipeline operations
    config := &pipeline.Config{
        ProjectPath: typedArgs.ProjectPath,
    }

    result, err := t.pipelineAdapter.ExecutePipeline(ctx, config)
    if err != nil {
        return nil, core.NewExecutionError(
            core.ErrCodePipelineFailure,
            "pipeline execution failed",
            map[string]interface{}{"error": err.Error()},
        )
    }

    return result, nil
}
```

### 3. Error Handling

Use structured errors for better debugging:

```go
func (t *MyTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
    if err := someOperation(); err != nil {
        // Wrap errors with context
        return nil, core.NewExecutionError(
            core.ErrCodeOperationFailed,
            "failed to perform operation",
            map[string]interface{}{
                "operation": "someOperation",
                "error": err.Error(),
                "context": typedArgs,
            },
        )
    }
}
```

### 4. Metrics and Monitoring

Add metrics to your tool:

```go
func (t *MyTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
    start := time.Now()
    defer func() {
        duration := time.Since(start)
        t.logger.Info().
            Dur("duration", duration).
            Msg("tool execution completed")

        // Record metrics
        metrics.RecordToolDuration("my_tool", duration)
    }()

    // Tool logic...
}
```

## Troubleshooting

### Common Issues

1. **Tool Not Appearing in MCP Server**
   - Ensure you've registered the tool in `gomcp_tools.go`
   - Rebuild with `make mcp`
   - Check logs for registration errors

2. **Schema Validation Errors**
   - Ensure all struct fields have proper JSON tags
   - Use `jsonschema` tags for validation rules
   - Test schemas with `core.GenerateSchema()`

3. **Type Assertion Failures**
   - Always check type assertions in Execute()
   - Use pointer types for argument structs
   - Return appropriate error messages

4. **Context Cancellation**
   - Always respect context cancellation
   - Use `select` with `ctx.Done()` for long operations
   - Clean up resources on cancellation

### Debugging Tips

1. **Enable Debug Logging**
   ```bash
   export LOG_LEVEL=debug
   ./bin/mcp server
   ```

2. **Test with MCP Client**
   ```bash
   # Use the MCP test client
   ./bin/mcp-test-client call my_tool '{"projectPath": "/test"}'
   ```

3. **Inspect Generated Schemas**
   ```go
   // Add temporary debug code
   schema := core.GenerateSchema(&MyToolArgs{})
   fmt.Printf("Input Schema: %s\n", schema)
   ```

## Best Practices

1. **Keep Tools Focused**: Each tool should do one thing well
2. **Use Structured Logging**: Include relevant context in logs
3. **Handle Errors Gracefully**: Provide meaningful error messages
4. **Document Examples**: Include realistic examples in metadata
5. **Test Edge Cases**: Cover error conditions in tests
6. **Monitor Performance**: Keep execution time under 300μs for simple operations
7. **Use Dependency Injection**: Accept dependencies through constructor
8. **Follow Naming Conventions**: Use snake_case for tool names, CamelCase for types

## Next Steps

- Review existing tools in `pkg/mcp/internal/` for more examples
- Check the orchestration package for workflow integration
- Read the MCP protocol documentation for advanced features
- Join the development discussions for guidance

Remember: The auto-registration system handles most of the complexity. Focus on implementing clean, well-tested tool logic, and the framework will handle the rest.
