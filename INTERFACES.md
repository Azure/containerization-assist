# MCP Unified Interface Documentation

## Overview

This document describes the unified interface system that replaced 11 separate interface files with a single, cohesive design. The unified interfaces provide a consistent contract for all MCP tools and components.

## Core Interfaces

### Tool Interface

The primary interface that all MCP tools implement:

```go
type Tool interface {
    // Execute performs the tool's primary operation
    Execute(ctx context.Context, args interface{}) (interface{}, error)
    
    // GetMetadata returns tool metadata for registration and discovery
    GetMetadata() ToolMetadata
    
    // Validate checks if the provided arguments are valid for this tool
    Validate(ctx context.Context, args interface{}) error
}
```

#### Tool Implementation Pattern

Every tool follows this exact pattern:

```go
type ExampleTool struct {
    // Tool-specific fields
}

func (t *ExampleTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
    // 1. Cast args to expected type
    // 2. Perform validation
    // 3. Execute core logic
    // 4. Return structured result
}

func (t *ExampleTool) GetMetadata() ToolMetadata {
    return ToolMetadata{
        Name:        "example_tool",
        Description: "Tool description",
        Version:     "1.0.0",
        InputSchema: "json-schema-here",
    }
}

func (t *ExampleTool) Validate(ctx context.Context, args interface{}) error {
    // Validate input arguments
    // Return descriptive errors for invalid inputs
}
```

### Session Interface

Manages user sessions and workspace state:

```go
type Session interface {
    // ID returns the unique session identifier
    ID() string
    
    // GetWorkspace returns the filesystem workspace path
    GetWorkspace() string
    
    // UpdateState atomically updates session state
    UpdateState(func(*SessionState))
}
```

#### Session State Management

```go
type SessionState struct {
    ID          string
    CreatedAt   time.Time
    UpdatedAt   time.Time
    TTL         time.Duration
    Workspace   string
    Metadata    map[string]interface{}
    Progress    *ProgressState
}

// Example usage
session.UpdateState(func(state *SessionState) {
    state.Metadata["last_tool"] = "build_image"
    state.UpdatedAt = time.Now()
})
```

### Transport Interface

Handles communication protocols (stdio, HTTP):

```go
type Transport interface {
    // Serve starts the transport server
    Serve(ctx context.Context) error
    
    // Stop gracefully shuts down the transport
    Stop() error
}
```

#### Transport Implementations

- **StdioTransport**: For Claude Desktop and CLI integration
- **HTTPTransport**: For web and API integration

### Orchestrator Interface

Coordinates tool execution and workflow management:

```go
type Orchestrator interface {
    // ExecuteTool runs a specific tool with provided arguments
    ExecuteTool(ctx context.Context, name string, args interface{}) (interface{}, error)
    
    // RegisterTool adds a tool to the registry
    RegisterTool(name string, tool Tool) error
}
```

## Auto-Registration System

### Tool Registration

Tools are automatically registered using build-time code generation:

```go
//go:generate go run tools/register_tools.go

// Generated registration code
func init() {
    RegisterTool("analyze_repository", &AnalyzeRepositoryTool{})
    RegisterTool("build_image", &BuildImageTool{})
    RegisterTool("deploy_kubernetes", &DeployKubernetesTool{})
    // ... all tools registered automatically
}
```

### Registration Helper

```go
func RegisterTool[T Tool](name string, tool T) {
    if err := orchestrator.RegisterTool(name, tool); err != nil {
        log.Fatalf("Failed to register tool %s: %v", name, err)
    }
}
```

## Tool Metadata System

### ToolMetadata Structure

```go
type ToolMetadata struct {
    Name         string            `json:"name"`
    Description  string            `json:"description"`
    Version      string            `json:"version"`
    InputSchema  string            `json:"input_schema"`
    OutputSchema string            `json:"output_schema"`
    Tags         []string          `json:"tags"`
    Category     string            `json:"category"`
    Examples     []ToolExample     `json:"examples"`
    Dependencies []string          `json:"dependencies"`
}

type ToolExample struct {
    Name        string      `json:"name"`
    Description string      `json:"description"`
    Input       interface{} `json:"input"`
    Output      interface{} `json:"output"`
}
```

### Tool Categories

Tools are organized into domain categories:

- **analyze**: Repository and code analysis tools
- **build**: Docker image building and management
- **deploy**: Kubernetes deployment and orchestration  
- **scan**: Security scanning and validation
- **validate**: Input and configuration validation

## Error Handling Patterns

### Standardized Error Types

```go
type ToolError struct {
    Code    string      `json:"code"`
    Message string      `json:"message"`
    Details interface{} `json:"details,omitempty"`
    Cause   error       `json:"-"`
}

func (e *ToolError) Error() string {
    return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Common error codes
const (
    ErrCodeValidation   = "VALIDATION_ERROR"
    ErrCodeExecution    = "EXECUTION_ERROR"
    ErrCodeDependency   = "DEPENDENCY_ERROR"
    ErrCodeTimeout      = "TIMEOUT_ERROR"
    ErrCodePermission   = "PERMISSION_ERROR"
)
```

### Error Creation Helpers

```go
func NewValidationError(message string, details interface{}) *ToolError {
    return &ToolError{
        Code:    ErrCodeValidation,
        Message: message,
        Details: details,
    }
}

func NewExecutionError(message string, cause error) *ToolError {
    return &ToolError{
        Code:    ErrCodeExecution,
        Message: message,
        Cause:   cause,
    }
}
```

## Context and Cancellation

### Context Usage

All interface methods accept context for cancellation and timeout handling:

```go
func (t *BuildImageTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
    // Check for cancellation
    if err := ctx.Err(); err != nil {
        return nil, err
    }
    
    // Use context for timeouts
    buildCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
    defer cancel()
    
    // Pass context to child operations
    return t.dockerClient.Build(buildCtx, buildArgs)
}
```

### Timeout Patterns

```go
// Tool-specific timeouts
var defaultTimeouts = map[string]time.Duration{
    "analyze_repository": 30 * time.Second,
    "build_image":       5 * time.Minute,
    "deploy_kubernetes": 10 * time.Minute,
    "scan_image":        2 * time.Minute,
}
```

## Testing Patterns

### Interface Testing

```go
func TestToolInterface(t *testing.T) {
    tool := &ExampleTool{}
    
    // Test metadata
    metadata := tool.GetMetadata()
    assert.NotEmpty(t, metadata.Name)
    assert.NotEmpty(t, metadata.Description)
    
    // Test validation
    err := tool.Validate(context.Background(), validArgs)
    assert.NoError(t, err)
    
    err = tool.Validate(context.Background(), invalidArgs)
    assert.Error(t, err)
    
    // Test execution
    result, err := tool.Execute(context.Background(), validArgs)
    assert.NoError(t, err)
    assert.NotNil(t, result)
}
```

### Mock Implementations

```go
type MockTool struct {
    ExecuteFunc    func(context.Context, interface{}) (interface{}, error)
    GetMetadataFunc func() ToolMetadata
    ValidateFunc   func(context.Context, interface{}) error
}

func (m *MockTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
    if m.ExecuteFunc != nil {
        return m.ExecuteFunc(ctx, args)
    }
    return nil, nil
}

// ... implement other interface methods
```

## Migration from Legacy Interfaces

### Before (Multiple Interfaces)

```go
// OLD: Multiple interface files
// pkg/mcp/internal/interfaces/tool.go
// pkg/mcp/internal/tools/interfaces.go  
// pkg/mcp/internal/adapter/interfaces.go
// ... 8 more interface files

type OldToolInterface interface {
    Run(map[string]interface{}) (map[string]interface{}, error)
}

type OldSessionInterface interface {
    GetID() string
    Save() error
}
```

### After (Unified Interface)

```go
// NEW: Single interface file
// pkg/mcp/interfaces.go

type Tool interface {
    Execute(ctx context.Context, args interface{}) (interface{}, error)
    GetMetadata() ToolMetadata
    Validate(ctx context.Context, args interface{}) error
}

type Session interface {
    ID() string
    GetWorkspace() string
    UpdateState(func(*SessionState))
}
```

### Migration Checklist

- [ ] All tools implement the unified Tool interface
- [ ] Legacy interface files removed
- [ ] Import statements updated to use `pkg/mcp` 
- [ ] Tool registration uses auto-registration system
- [ ] Error handling uses standardized ToolError types
- [ ] Tests updated for new interface signatures

## Best Practices

### Tool Implementation

1. **Stateless Design**: Tools should not maintain state between calls
2. **Context Awareness**: Always check context cancellation
3. **Input Validation**: Comprehensive validation with helpful error messages
4. **Structured Output**: Return consistent, well-typed results
5. **Error Handling**: Use standardized error types and codes

### Performance Considerations

1. **Metadata Caching**: Cache tool metadata for repeated calls
2. **Resource Cleanup**: Ensure proper cleanup in all code paths
3. **Timeout Handling**: Set appropriate timeouts for long-running operations
4. **Memory Management**: Avoid memory leaks in long-running tools

### Security Guidelines

1. **Input Sanitization**: Validate and sanitize all inputs
2. **Permission Checks**: Verify permissions before operations
3. **Secret Handling**: Never log or expose sensitive data
4. **Resource Limits**: Implement resource usage limits

## Example Implementation

See the [build tools](pkg/mcp/internal/build/) for complete examples of unified interface implementation.