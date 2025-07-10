# Container Kit API Reference

This document provides the complete API reference for Container Kit, sourced from the canonical interface definitions in `pkg/mcp/application/api/interfaces.go`.

## Overview

Container Kit implements a unified interface system that provides a single source of truth for all API definitions. The system is built around core abstractions that enable extensible containerization workflows.

## Core Interfaces

### Tool System

The tool system is the foundation of Container Kit's extensibility, providing a standardized interface for all containerization operations.

#### Tool Interface

```go
type Tool interface {
    // Name returns the unique identifier for this tool
    Name() string

    // Description returns a human-readable description of the tool
    Description() string

    // Execute runs the tool with the given input
    Execute(ctx context.Context, input ToolInput) (ToolOutput, error)

    // Schema returns the JSON schema for the tool's parameters and results
    Schema() ToolSchema
}
```

**Key Features:**
- Standardized execution model with context support
- JSON schema validation for type safety
- Consistent input/output structures
- Error handling with rich context

#### ToolInput Structure

```go
type ToolInput struct {
    // SessionID identifies the session this tool execution belongs to
    SessionID string `json:"session_id"`

    // Data contains the tool-specific input parameters
    Data map[string]interface{} `json:"data"`

    // Context provides additional execution context
    Context map[string]interface{} `json:"context,omitempty"`
}
```

**Methods:**
- `GetSessionID() string` - Returns the session identifier
- `Validate() error` - Validates input structure
- `GetContext() map[string]interface{}` - Returns execution context

#### ToolOutput Structure

```go
type ToolOutput struct {
    // Success indicates if the tool execution was successful
    Success bool `json:"success"`

    // Data contains the tool-specific output
    Data map[string]interface{} `json:"data"`

    // Error contains any error message if Success is false
    Error string `json:"error,omitempty"`

    // Metadata contains additional information about the execution
    Metadata map[string]interface{} `json:"metadata,omitempty"`
}
```

**Methods:**
- `IsSuccess() bool` - Returns success status
- `GetData() interface{}` - Returns output data
- `GetError() string` - Returns error message

### Registry System

The registry provides tool lifecycle management, discovery, and execution orchestration.

#### Registry Interface

```go
type Registry interface {
    // Tool Management
    Register(tool Tool, opts ...RegistryOption) error
    Unregister(name string) error
    Get(name string) (Tool, error)
    
    // Discovery
    List() []string
    ListByCategory(category ToolCategory) []string
    ListByTags(tags ...string) []string
    
    // Execution
    Execute(ctx context.Context, name string, input ToolInput) (ToolOutput, error)
    ExecuteWithRetry(ctx context.Context, name string, input ToolInput, policy RetryPolicy) (ToolOutput, error)
    
    // Metadata and Status
    GetMetadata(name string) (ToolMetadata, error)
    GetStatus(name string) (ToolStatus, error)
    SetStatus(name string, status ToolStatus) error
    
    // Lifecycle
    Close() error
    
    // Monitoring
    GetMetrics() RegistryMetrics
    Subscribe(event RegistryEventType, callback RegistryEventCallback) error
    Unsubscribe(event RegistryEventType, callback RegistryEventCallback) error
}
```

**Key Features:**
- Advanced tool discovery with filtering
- Configurable retry policies
- Built-in metrics collection
- Event-driven monitoring
- Graceful shutdown support

#### Registry Configuration

Tools can be registered with rich configuration options:

```go
type RegistryConfig struct {
    Namespace          string
    Tags              []string
    Priority          int
    Enabled           bool
    Metadata          map[string]interface{}
    Concurrency       int
    Timeout           time.Duration
    RetryPolicy       *RetryPolicy
    CacheEnabled      bool
    CacheDuration     time.Duration
    RateLimitPerMinute int
}
```

**Configuration Functions:**
- `WithNamespace(namespace string)` - Sets tool namespace
- `WithTags(tags ...string)` - Adds categorization tags
- `WithPriority(priority int)` - Sets execution priority
- `WithConcurrency(maxConcurrency int)` - Limits concurrent executions
- `WithTimeout(timeout time.Duration)` - Sets execution timeout
- `WithRetryPolicy(policy RetryPolicy)` - Configures retry behavior
- `WithCache(duration time.Duration)` - Enables result caching
- `WithRateLimit(perMinute int)` - Sets rate limiting

### Orchestration System

The orchestration system provides higher-level workflow management and tool coordination.

#### Orchestrator Interface

```go
type Orchestrator interface {
    // Tool Management
    RegisterTool(name string, tool Tool) error
    RegisterGenericTool(name string, tool interface{}) error
    GetTool(name string) (Tool, bool)
    ListTools() []string
    
    // Execution
    ExecuteTool(ctx context.Context, toolName string, args interface{}) (interface{}, error)
    ValidateToolArgs(toolName string, args interface{}) error
    
    // Metadata
    GetToolMetadata(toolName string) (*ToolMetadata, error)
    GetTypedToolMetadata(toolName string) (*ToolMetadata, error)
    GetStats() interface{}
}
```

### Workflow System

Workflows enable complex, multi-tool operations with dependency management and error recovery.

#### Workflow Types

```go
type Workflow struct {
    ID          string                 `json:"id"`
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    Steps       []WorkflowStep         `json:"steps"`
    Variables   map[string]interface{} `json:"variables"`
    CreatedAt   time.Time              `json:"created_at"`
    UpdatedAt   time.Time              `json:"updated_at"`
}

type WorkflowStep struct {
    ID         string                 `json:"id"`
    Name       string                 `json:"name"`
    Tool       string                 `json:"tool"`
    Input      map[string]interface{} `json:"input"`
    DependsOn  []string               `json:"depends_on"`
    Condition  string                 `json:"condition"`
    MaxRetries int                    `json:"max_retries"`
    Timeout    time.Duration          `json:"timeout"`
}
```

#### Workflow Results

```go
type WorkflowResult struct {
    WorkflowID   string        `json:"workflow_id"`
    Success      bool          `json:"success"`
    StepResults  []StepResult  `json:"step_results"`
    Error        string        `json:"error,omitempty"`
    StartTime    time.Time     `json:"start_time"`
    EndTime      time.Time     `json:"end_time"`
    Duration     time.Duration `json:"duration"`
    TotalSteps   int           `json:"total_steps"`
    SuccessSteps int           `json:"success_steps"`
    FailedSteps  int           `json:"failed_steps"`
}
```

### Session Management

Sessions provide isolated execution environments with state management and workspace isolation.

#### Session Types

```go
type Session struct {
    ID        string                 `json:"id"`
    CreatedAt time.Time              `json:"created_at"`
    UpdatedAt time.Time              `json:"updated_at"`
    Metadata  map[string]interface{} `json:"metadata"`
    State     map[string]interface{} `json:"state"`
}
```

### MCP Server System

The MCP server provides the main entry point for Model Context Protocol integration.

#### MCPServer Interface

```go
type MCPServer interface {
    // Lifecycle
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    
    // Tool Management
    RegisterTool(tool Tool) error
    GetRegistry() Registry
    GetOrchestrator() Orchestrator
    
    // Session Management
    GetSessionManager() interface{}
}
```

#### GomcpManager Interface

```go
type GomcpManager interface {
    // Lifecycle
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    IsRunning() bool
    
    // Tool Integration
    RegisterTool(name, description string, handler interface{}) error
    GetServer() *server.Server
}
```

### Pipeline System

The pipeline system enables complex, multi-stage workflows with built-in error handling, retries, and monitoring.

#### Pipeline Interface

```go
type Pipeline interface {
    // Execute runs pipeline with context and metrics
    Execute(ctx context.Context, request *PipelineRequest) (*PipelineResponse, error)
    
    // Configuration
    AddStage(stage PipelineStage) Pipeline
    WithTimeout(timeout time.Duration) Pipeline
    WithRetry(policy PipelineRetryPolicy) Pipeline
    WithMetrics(collector MetricsCollector) Pipeline
}
```

#### PipelineStage Interface

```go
type PipelineStage interface {
    Name() string
    Execute(ctx context.Context, input interface{}) (interface{}, error)
    Validate(input interface{}) error
}
```

#### Pipeline Builder

```go
type PipelineBuilder interface {
    New() Pipeline
    FromTemplate(template string) Pipeline
    WithStages(stages ...PipelineStage) PipelineBuilder
    Build() Pipeline
}
```

### Validation System

Container Kit provides a comprehensive validation framework with domain-specific validators.

#### Core Validation

```go
type Validator[T any] interface {
    // Validate validates a value and returns validation result
    Validate(ctx context.Context, value T) ValidationResult
    
    // Name returns the validator name for error reporting
    Name() string
}
```

#### Domain Validation

```go
type DomainValidator[T any] interface {
    Validator[T]
    
    // Domain returns the validation domain (e.g., "kubernetes", "docker", "security")
    Domain() string
    
    // Category returns the validation category (e.g., "manifest", "config", "policy")
    Category() string
    
    // Priority returns validation priority for ordering (higher = earlier)
    Priority() int
    
    // Dependencies returns validator names this depends on
    Dependencies() []string
}
```

#### Validation Results

```go
type ValidationResult struct {
    Valid    bool
    Errors   []error
    Warnings []string
    Context  ValidationContext
}
```

### Transport System

The transport system provides communication mechanisms for MCP protocol integration.

#### Transport Interface

```go
type Transport interface {
    // Lifecycle
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    
    // Communication
    Send(message interface{}) error
    Receive() (interface{}, error)
    IsConnected() bool
}
```

### Factory System

Factory interfaces provide abstraction for creating tools and components without direct dependencies.

#### ToolFactory Interface

```go
type ToolFactory interface {
    // Tool Creation
    CreateTool(category string, name string) (Tool, error)
    RegisterToolCreator(category string, name string, creator ToolCreator)
    
    // Specialized Creation
    CreateAnalyzer(aiAnalyzer interface{}) interface{}
    CreateEnhancedBuildAnalyzer() interface{}
    CreateSessionStateManager(sessionID string) interface{}
}
```

## Build System Types

Container Kit provides comprehensive build system support with multiple strategies and monitoring.

### Build Operations

```go
type BuildArgs struct {
    SessionID  string                 `json:"session_id"`
    Dockerfile string                 `json:"dockerfile"`
    Context    string                 `json:"context"`
    ImageName  string                 `json:"image_name"`
    Tags       []string               `json:"tags"`
    BuildArgs  map[string]string      `json:"build_args"`
    Target     string                 `json:"target,omitempty"`
    Platform   string                 `json:"platform,omitempty"`
    NoCache    bool                   `json:"no_cache,omitempty"`
    PullParent bool                   `json:"pull_parent,omitempty"`
    Labels     map[string]string      `json:"labels,omitempty"`
    Metadata   map[string]interface{} `json:"metadata,omitempty"`
}
```

### Build Results

```go
type BuildResult struct {
    BuildID     string                 `json:"build_id"`
    ImageID     string                 `json:"image_id"`
    ImageName   string                 `json:"image_name"`
    Tags        []string               `json:"tags"`
    Success     bool                   `json:"success"`
    Error       string                 `json:"error,omitempty"`
    Logs        []string               `json:"logs,omitempty"`
    Size        int64                  `json:"size"`
    Duration    time.Duration          `json:"duration"`
    CreatedAt   time.Time              `json:"created_at"`
    CompletedAt *time.Time             `json:"completed_at,omitempty"`
    Metadata    map[string]interface{} `json:"metadata,omitempty"`
}
```

### Build States

```go
type BuildState string

const (
    BuildStateQueued    BuildState = "queued"
    BuildStateRunning   BuildState = "running"
    BuildStateCompleted BuildState = "completed"
    BuildStateFailed    BuildState = "failed"
    BuildStateCancelled BuildState = "cancelled"
)
```

### Build Strategies

```go
type BuildStrategy string

const (
    BuildStrategyDocker   BuildStrategy = "docker"
    BuildStrategyBuildkit BuildStrategy = "buildkit"
    BuildStrategyPodman   BuildStrategy = "podman"
    BuildStrategyKaniko   BuildStrategy = "kaniko"
)
```

## Error Handling

Container Kit uses the unified RichError system for comprehensive error reporting. All interfaces return structured errors with context, suggestions, and recovery information.

### Error Types

- **ValidationError**: Input validation failures
- **ExecutionError**: Tool execution failures
- **NetworkError**: Communication failures
- **ConfigurationError**: Configuration issues
- **AuthenticationError**: Security failures

### Best Practices

1. **Context Usage**: Always pass `context.Context` as the first parameter
2. **Error Handling**: Return structured errors with context
3. **Timeouts**: Set appropriate timeouts for all operations
4. **Validation**: Validate inputs before processing
5. **Metrics**: Use built-in metrics collection for monitoring
6. **Cleanup**: Implement proper resource cleanup
7. **Retries**: Configure retry policies for transient failures

## Monitoring and Observability

### Metrics Collection

All interfaces provide built-in metrics collection:

```go
type RegistryMetrics struct {
    TotalTools           int           `json:"total_tools"`
    ActiveTools          int           `json:"active_tools"`
    TotalExecutions      int64         `json:"total_executions"`
    FailedExecutions     int64         `json:"failed_executions"`
    AverageExecutionTime time.Duration `json:"average_execution_time"`
    UpTime               time.Duration `json:"up_time"`
    LastExecution        *time.Time    `json:"last_execution,omitempty"`
}
```

### Event System

Registry events provide real-time monitoring:

```go
const (
    EventToolRegistered    RegistryEventType = "tool_registered"
    EventToolUnregistered  RegistryEventType = "tool_unregistered"
    EventToolExecuted      RegistryEventType = "tool_executed"
    EventToolFailed        RegistryEventType = "tool_failed"
    EventToolStatusChanged RegistryEventType = "tool_status_changed"
)
```

## Built-in Tools

Container Kit provides several built-in tools for common containerization workflows:

### Containerization Tools

- **analyze**: Repository analysis and Dockerfile generation
- **build**: Docker image building with AI-powered fixes
- **deploy**: Kubernetes manifest generation and deployment
- **scan**: Security vulnerability scanning with Trivy/Grype

### Utility Tools

- **validate**: Configuration and Dockerfile validation
- **optimize**: Dockerfile optimization suggestions
- **migrate**: Legacy application containerization

## Version History

- **v1.0.0**: Unified interface system with single source of truth
- **v0.9.0**: Legacy multi-manager system (deprecated)

## Related Documentation

- [Architecture Overview](../../architecture/three-layer-architecture.md)
- [Adding New Tools](../../guides/developer/adding-new-tools.md)
- [Error Handling Guide](../../guides/developer/error-handling.md)
- [Service Container](../../architecture/service-container.md)

## Source Code Reference

All interfaces are defined in:
- **Primary**: `/pkg/mcp/application/api/interfaces.go` (Single source of truth)
- **Services**: `/pkg/mcp/application/services/interfaces.go` (Service layer)
- **Compatibility**: `/pkg/mcp/application/interfaces/interfaces.go` (Type aliases)

---

*This document reflects the current state of the Container Kit API as of the unified interface system implementation. All interface definitions are sourced from the canonical implementations in the codebase.*