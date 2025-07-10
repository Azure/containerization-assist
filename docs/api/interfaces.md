# Container Kit API Interfaces

## Overview
This document describes the public API interfaces for Container Kit, as defined in `pkg/mcp/application/api/interfaces.go`.

## Core Interfaces

### Tool System

#### Tool Interface
```go
type Tool interface {
    Name() string
    Description() string
    Version() string
    Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error)
    GetSchema() (*ToolSchema, error)
}
```

The Tool interface is the foundation of Container Kit's extensibility. All tools must implement this interface.

#### ToolRegistry Interface
```go
type ToolRegistry interface {
    Register(tool Tool) error
    RegisterWithMetadata(tool Tool, metadata ToolMetadata) error
    Get(name string) (Tool, error)
    List() []ToolInfo
    Execute(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error)
    GetMetrics() map[string]ToolMetrics
    Shutdown(ctx context.Context) error
}
```

The ToolRegistry manages tool lifecycle and execution.

### Pipeline System

#### Pipeline Interface
```go
type Pipeline interface {
    Execute(ctx context.Context, request *PipelineRequest) (*PipelineResponse, error)
    AddStage(stage PipelineStage) Pipeline
    WithTimeout(timeout time.Duration) Pipeline
    WithRetry(policy RetryPolicy) Pipeline
    GetMetrics() PipelineMetrics
}
```

Pipelines orchestrate multi-stage workflows with built-in retry and timeout support.

#### PipelineStage Interface
```go
type PipelineStage interface {
    Name() string
    Execute(ctx context.Context, input StageInput) (StageOutput, error)
    Validate(input StageInput) error
}
```

### Session Management

#### SessionManager Interface
```go
type SessionManager interface {
    Create(ctx context.Context, config SessionConfig) (*Session, error)
    Get(ctx context.Context, id string) (*Session, error)
    Update(ctx context.Context, id string, updates SessionUpdates) error
    Delete(ctx context.Context, id string) error
    List(ctx context.Context, filter SessionFilter) ([]*Session, error)
    Checkpoint(ctx context.Context, id string) error
    Restore(ctx context.Context, id string, checkpointID string) error
}
```

Sessions provide isolated execution environments with state management.

### Workflow System

#### WorkflowEngine Interface
```go
type WorkflowEngine interface {
    Execute(ctx context.Context, workflow *Workflow) (*WorkflowResult, error)
    Validate(workflow *Workflow) error
    GetStatus(ctx context.Context, workflowID string) (*WorkflowStatus, error)
    Cancel(ctx context.Context, workflowID string) error
    List(ctx context.Context, filter WorkflowFilter) ([]*WorkflowInfo, error)
}
```

Workflows define complex, multi-tool operations with dependency management.

## Data Types

### ToolSchema
```go
type ToolSchema struct {
    Type        string                 `json:"type"`
    Properties  map[string]interface{} `json:"properties"`
    Required    []string              `json:"required,omitempty"`
    Description string                `json:"description,omitempty"`
}
```

### PipelineRequest
```go
type PipelineRequest struct {
    ID          string                 `json:"id"`
    Type        string                 `json:"type"`
    Input       map[string]interface{} `json:"input"`
    Context     PipelineContext        `json:"context"`
    Options     PipelineOptions        `json:"options"`
}
```

### Session
```go
type Session struct {
    ID          string            `json:"id"`
    Created     time.Time         `json:"created"`
    Updated     time.Time         `json:"updated"`
    State       SessionState      `json:"state"`
    Metadata    map[string]string `json:"metadata"`
    Workspace   string            `json:"workspace"`
    Checkpoints []Checkpoint      `json:"checkpoints"`
}
```

## Error Handling

All interfaces use the unified RichError system for comprehensive error reporting:

```go
type RichError interface {
    Error() string
    Code() ErrorCode
    Type() ErrorType
    Severity() ErrorSeverity
    Context() map[string]interface{}
    Suggestion() string
    Unwrap() error
}
```

## Best Practices

1. **Context Usage**: Always pass context.Context as the first parameter
2. **Error Handling**: Return RichError for detailed error information
3. **Metrics**: Use built-in metrics collection for monitoring
4. **Validation**: Validate inputs before processing
5. **Timeouts**: Set appropriate timeouts for all operations

## Version History

- v1.0.0 - Initial unified interface system
- v0.9.0 - Legacy multi-manager system (deprecated)
