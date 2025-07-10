# Tool System Documentation

## Overview

The Container Kit tool system provides a flexible, extensible framework for implementing containerization operations.

## Core Concepts

### Tool Registration

Tools are automatically registered at startup using the registration helper:

```go
import "github.com/Azure/container-kit/pkg/mcp/application/internal/runtime"

func init() {
    runtime.MustRegisterTool(&MyTool{})
}
```

### Tool Implementation

Implement the Tool interface:

```go
type MyTool struct {
    config ToolConfig
}

func (t *MyTool) Name() string {
    return "my-tool"
}

func (t *MyTool) Description() string {
    return "Description of what my tool does"
}

func (t *MyTool) Version() string {
    return "1.0.0"
}

func (t *MyTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
    // Parse arguments
    var input MyToolInput
    if err := json.Unmarshal(args, &input); err != nil {
        return nil, errors.NewError().
            Code(errors.CodeValidationFailed).
            Message("invalid input").
            Wrap(err).
            Build()
    }
    
    // Execute tool logic
    result := processInput(input)
    
    // Return result
    return json.Marshal(result)
}

func (t *MyTool) GetSchema() (*ToolSchema, error) {
    return GenerateSchema(MyToolInput{})
}
```

### Tool Metadata

Enhance tools with metadata for better discovery and monitoring:

```go
metadata := ToolMetadata{
    Category:    "build",
    Tags:        []string{"docker", "container"},
    Timeout:     30 * time.Second,
    RetryPolicy: DefaultRetryPolicy(),
}

runtime.MustRegisterToolWithMetadata(&MyTool{}, metadata)
```

## Built-in Tools

### Containerization Tools

- **analyze** - Repository analysis and Dockerfile generation
- **build** - Docker image building with AI-powered fixes
- **deploy** - Kubernetes manifest generation and deployment
- **scan** - Security vulnerability scanning

### Utility Tools

- **validate** - Configuration and Dockerfile validation
- **optimize** - Dockerfile optimization suggestions
- **migrate** - Legacy application containerization

## Tool Execution Flow

1. **Request Reception**: Tool receives execution request with arguments
2. **Validation**: Arguments are validated against schema
3. **Execution**: Tool logic is executed with timeout enforcement
4. **Error Handling**: Errors are wrapped with context
5. **Response**: Results are returned as JSON

## Performance Considerations

- Tools should complete execution within 300μs P95
- Use context for cancellation support
- Implement proper cleanup in defer blocks
- Cache expensive operations when possible

## Testing Tools

```go
func TestMyTool(t *testing.T) {
    tool := &MyTool{}
    
    input := MyToolInput{
        Field: "value",
    }
    
    args, _ := json.Marshal(input)
    result, err := tool.Execute(context.Background(), args)
    
    assert.NoError(t, err)
    assert.NotNil(t, result)
}
```

## Monitoring and Metrics

Tools automatically collect metrics:

- Execution count
- Success/failure rates
- Execution duration
- Error types

Access metrics via:

```go
registry := GetToolRegistry()
metrics := registry.GetMetrics()
```
