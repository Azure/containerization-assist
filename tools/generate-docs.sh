#!/bin/bash

# API documentation generator
echo "=== Container Kit Documentation Generator ==="
echo "Generating API documentation..."

# Create main API documentation
cat > docs/api/README.md << 'DOC_EOF'
# Container Kit API Documentation

This directory contains the API documentation for Container Kit.

## Documentation Structure

- `interfaces.md` - Core interface definitions
- `tools.md` - Tool system documentation
- `pipeline.md` - Pipeline system documentation
- `session.md` - Session management documentation
- `errors.md` - Error handling documentation

## Quick Links

- [Architecture Overview](../architecture/README.md)
- [Getting Started Guide](../guides/getting-started.md)
- [Examples](../examples/README.md)

## API Versioning

Container Kit follows semantic versioning. The current API version is v1.0.0.
DOC_EOF

# Generate interface documentation from source
cat > docs/api/interfaces.md << 'DOC_EOF'
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
DOC_EOF

# Generate tool system documentation
cat > docs/api/tools.md << 'DOC_EOF'
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
DOC_EOF

# Generate pipeline documentation
cat > docs/api/pipeline.md << 'DOC_EOF'
# Pipeline System Documentation

## Overview

The Container Kit pipeline system enables complex, multi-stage workflows with built-in error handling, retries, and monitoring.

## Pipeline Architecture

### Core Components

1. **Pipeline Engine**: Orchestrates stage execution
2. **Pipeline Stages**: Individual processing units
3. **Stage Context**: Shared state between stages
4. **Pipeline Metrics**: Performance and error tracking

### Pipeline Types

#### Atomic Pipeline
For simple, single-responsibility operations:

```go
pipeline := NewAtomicPipeline("build-image").
    WithStage(ValidateStage{}).
    WithStage(BuildStage{}).
    WithStage(PushStage{})
```

#### Workflow Pipeline
For complex, multi-tool orchestrations:

```go
pipeline := NewWorkflowPipeline("full-deployment").
    WithStage(AnalyzeStage{}).
    WithStage(BuildStage{}).
    WithStage(ScanStage{}).
    WithStage(DeployStage{}).
    WithRetry(ExponentialBackoff()).
    WithTimeout(5 * time.Minute)
```

## Creating Pipeline Stages

```go
type MyStage struct {
    config StageConfig
}

func (s *MyStage) Name() string {
    return "my-stage"
}

func (s *MyStage) Validate(input StageInput) error {
    if input.Get("required_field") == nil {
        return errors.New("missing required field")
    }
    return nil
}

func (s *MyStage) Execute(ctx context.Context, input StageInput) (StageOutput, error) {
    // Process input
    data := input.Get("data")

    // Perform operations
    result := process(data)

    // Return output
    output := NewStageOutput()
    output.Set("result", result)
    return output, nil
}
```

## Pipeline Execution

```go
// Create pipeline request
request := &PipelineRequest{
    ID:   "build-123",
    Type: "container-build",
    Input: map[string]interface{}{
        "repository": "/path/to/repo",
        "dockerfile": "Dockerfile",
    },
    Options: PipelineOptions{
        Timeout: 30 * time.Second,
        DryRun:  false,
    },
}

// Execute pipeline
response, err := pipeline.Execute(ctx, request)
if err != nil {
    // Handle error with rich context
    log.Error("pipeline failed", "error", err)
    return
}

// Process results
fmt.Printf("Pipeline completed: %s\n", response.Status)
```

## Error Handling and Recovery

### Retry Policies

```go
// Exponential backoff
pipeline.WithRetry(RetryPolicy{
    MaxAttempts: 3,
    InitialDelay: 1 * time.Second,
    MaxDelay: 30 * time.Second,
    Multiplier: 2.0,
})

// Custom retry logic
pipeline.WithRetry(RetryPolicy{
    ShouldRetry: func(err error) bool {
        return IsTransientError(err)
    },
})
```

### Stage Rollback

```go
type RollbackableStage struct {
    BaseStage
}

func (s *RollbackableStage) Rollback(ctx context.Context, input StageInput) error {
    // Cleanup logic
    return cleanup(input)
}
```

## Pipeline Composition

### Sequential Execution
```go
pipeline := NewPipeline().
    AddStage(StageA{}).
    AddStage(StageB{}).
    AddStage(StageC{})
```

### Conditional Execution
```go
pipeline := NewPipeline().
    AddStage(StageA{}).
    AddConditionalStage(StageB{}, func(output StageOutput) bool {
        return output.GetBool("needs_optimization")
    }).
    AddStage(StageC{})
```

### Parallel Execution
```go
pipeline := NewPipeline().
    AddStage(StageA{}).
    AddParallelStages(
        StageB{},
        StageC{},
        StageD{},
    ).
    AddStage(StageE{})
```

## Monitoring and Observability

### Pipeline Metrics

```go
metrics := pipeline.GetMetrics()
fmt.Printf("Total executions: %d\n", metrics.TotalExecutions)
fmt.Printf("Success rate: %.2f%%\n", metrics.SuccessRate)
fmt.Printf("P95 latency: %v\n", metrics.P95Latency)
```

### Distributed Tracing

Pipelines automatically integrate with OpenTelemetry:

```go
// Traces are automatically created for:
// - Pipeline execution
// - Each stage execution
// - Retry attempts
// - Error occurrences
```

## Best Practices

1. **Keep stages focused**: Each stage should have a single responsibility
2. **Use validation**: Always validate inputs before processing
3. **Handle partial failures**: Design for graceful degradation
4. **Monitor performance**: Track stage execution times
5. **Document dependencies**: Clear documentation of stage requirements

## Testing Pipelines

```go
func TestPipeline(t *testing.T) {
    // Create test pipeline
    pipeline := NewTestPipeline().
        WithMockStage("analyze", mockAnalyzeResult).
        WithMockStage("build", mockBuildResult)

    // Execute test
    response, err := pipeline.Execute(ctx, testRequest)

    // Verify results
    assert.NoError(t, err)
    assert.Equal(t, "success", response.Status)
    assert.Contains(t, response.Stages, "analyze")
    assert.Contains(t, response.Stages, "build")
}
```
DOC_EOF

# Generate architecture documentation
cat > docs/architecture/README.md << 'ARCH_EOF'
# Container Kit Architecture

## Overview

Container Kit follows a **three-layer clean architecture** pattern with strict dependency rules to ensure maintainability, testability, and scalability.

## Architecture Principles

1. **Separation of Concerns**: Each layer has distinct responsibilities
2. **Dependency Rule**: Dependencies only point inward
3. **Interface Segregation**: Small, focused interfaces
4. **Dependency Injection**: Manual DI for flexibility
5. **Domain-Driven Design**: Business logic in the domain layer

## Layer Structure

```
┌─────────────────────────────────────────────────┐
│                Infrastructure                    │
│  (External Systems, Transport, Persistence)      │
├─────────────────────────────────────────────────┤
│                 Application                      │
│     (Use Cases, Orchestration, Services)        │
├─────────────────────────────────────────────────┤
│                   Domain                         │
│        (Business Logic, Entities, Rules)         │
└─────────────────────────────────────────────────┘
```

### Domain Layer (`pkg/mcp/domain/`)
**Responsibility**: Core business logic and rules

**Contains**:
- Business entities and value objects
- Domain services and specifications
- Business rules and validation
- Domain events and errors

**Dependencies**: None (pure Go)

**Key Packages**:
- `config/` - Configuration entities
- `containerization/` - Container operations
- `errors/` - Rich error system
- `security/` - Security policies
- `session/` - Session entities
- `types/` - Core domain types

### Application Layer (`pkg/mcp/application/`)
**Responsibility**: Use case orchestration

**Contains**:
- Application services
- Use case implementations
- Command and query handlers
- Application-level validation
- Interface definitions

**Dependencies**: Domain layer only

**Key Packages**:
- `api/` - Canonical interfaces
- `commands/` - Command implementations
- `orchestration/` - Workflow coordination
- `services/` - Service interfaces
- `tools/` - Tool implementations

### Infrastructure Layer (`pkg/mcp/infra/`)
**Responsibility**: External integrations

**Contains**:
- Database implementations
- External service clients
- Transport mechanisms
- Framework integrations
- Infrastructure utilities

**Dependencies**: Domain and Application layers

**Key Packages**:
- `docker/` - Docker integration
- `k8s/` - Kubernetes integration
- `persistence/` - Storage layer
- `telemetry/` - Monitoring
- `transport/` - MCP protocol

## Dependency Flow

```
Infrastructure → Application → Domain
     ↓               ↓           ↓
  External       Use Cases   Business
  Systems      Orchestration   Logic
```

## Interface System

### Single Source of Truth
All public interfaces are defined in:
`pkg/mcp/application/api/interfaces.go`

This file contains:
- Tool interfaces
- Registry interfaces
- Session interfaces
- Workflow interfaces
- Pipeline interfaces

### Service Container Pattern

```go
type ServiceContainer interface {
    // Core Services
    ToolRegistry() ToolRegistry
    SessionManager() SessionManager
    WorkflowEngine() WorkflowEngine

    // Domain Services
    BuildExecutor() BuildExecutor
    Scanner() Scanner
    Deployer() Deployer

    // Infrastructure Services
    Storage() Storage
    DockerClient() DockerClient
    K8sClient() K8sClient
}
```

## Key Architectural Decisions

### ADR-001: Three-Layer Architecture
Simplified from 30+ packages to 3 bounded contexts

### ADR-004: Unified Error System
Single RichError type with context and suggestions

### ADR-006: Manual Dependency Injection
Explicit wiring for clarity and control

## Architecture Diagrams

### Component Overview
```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  MCP Server  │────▶│ Tool Registry │────▶│    Tools     │
└──────────────┘     └──────────────┘     └──────────────┘
        │                    │                     │
        ▼                    ▼                     ▼
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│   Session    │────▶│   Pipeline   │────▶│   Storage    │
│   Manager    │     │    Engine    │     │   (BoltDB)   │
└──────────────┘     └──────────────┘     └──────────────┘
```

### Data Flow
```
Request → Transport → Handler → Service → Domain → Infrastructure
   ↑                                                      ↓
   └──────────────────── Response ←──────────────────────┘
```

## Module Dependencies

```mermaid
graph TD
    A[cmd/mcp-server] --> B[application/core]
    B --> C[application/api]
    B --> D[application/services]
    C --> E[domain]
    D --> E
    B --> F[infra/transport]
    F --> C
    B --> G[infra/persistence]
    G --> E
```

## Performance Architecture

### Design Goals
- <300μs P95 latency for tool operations
- Horizontal scalability
- Efficient resource usage
- Graceful degradation

### Optimization Strategies
1. **Connection pooling** for external services
2. **Caching** at multiple layers
3. **Async processing** for long operations
4. **Batch operations** where possible

## Security Architecture

### Principles
1. **Defense in depth**
2. **Least privilege**
3. **Secure by default**
4. **Audit everything**

### Implementation
- Input validation at boundaries
- Sanitization in domain layer
- Authentication in transport
- Authorization in application
- Encryption at rest and in transit

## Scalability Patterns

### Horizontal Scaling
- Stateless services
- Session affinity via external store
- Load balancing at transport layer

### Vertical Scaling
- Resource limits per operation
- Timeout enforcement
- Circuit breakers for dependencies

## Monitoring and Observability

### Metrics
- Business metrics in domain
- Performance metrics in application
- System metrics in infrastructure

### Tracing
- OpenTelemetry integration
- Distributed trace context
- Automatic span creation

### Logging
- Structured logging (slog)
- Contextual information
- Log aggregation support
ARCH_EOF

# Generate current state documentation
cat > docs/architecture/current_state.md << 'STATE_EOF'
# Container Kit Architecture - Current State

As of: January 2025

## Overview

Container Kit is undergoing a major architectural refactoring to improve maintainability, performance, and extensibility. This document captures the current state during the transition.

## Refactoring Status

### Completed
- ✅ Three-layer architecture established
- ✅ Unified interface system (api/interfaces.go)
- ✅ Rich error system implementation
- ✅ Performance baseline established (<300μs target)
- ✅ Basic documentation infrastructure

### In Progress
- 🔄 Tool registry consolidation (BETA workstream)
- 🔄 Pipeline unification (DELTA workstream)
- 🔄 Error system migration (GAMMA workstream)
- 🔄 Foundation cleanup (ALPHA workstream)

### Pending
- ⏳ OpenTelemetry integration
- ⏳ Complete test coverage (target: 55%)
- ⏳ Production deployment guide

## Current Architecture

### Package Structure
```
pkg/mcp/
├── domain/              # ✅ Clean, no circular deps
│   ├── config/         # ✅ Validation DSL implemented
│   ├── containerization/ # ✅ Core operations defined
│   ├── errors/         # ✅ RichError system ready
│   ├── security/       # ✅ Policies defined
│   ├── session/        # ✅ Entity definitions
│   └── types/          # ✅ Core types
├── application/         # 🔄 Consolidation in progress
│   ├── api/            # ✅ Single source of truth
│   ├── commands/       # 🔄 Being consolidated
│   ├── core/           # 🔄 Registry work
│   ├── orchestration/  # 🔄 Pipeline unification
│   ├── services/       # ✅ Interface definitions
│   └── tools/          # 🔄 Migration ongoing
└── infra/              # ⚠️ Some build issues
    ├── docker/         # ✅ Functional
    ├── persistence/    # ✅ BoltDB working
    ├── telemetry/      # ⏳ Not yet implemented
    └── transport/      # ⚠️ Build errors
```

### Build Status

#### Working Packages
- `pkg/mcp/domain/*` - All domain packages
- `pkg/mcp/application/internal/*` - Internal utilities
- `pkg/mcp/application/workflows` - Workflow management
- `pkg/mcp/infra/retry` - Retry mechanisms

#### Build Issues (4 packages)
1. **pkg/mcp/application** - Context parameter mismatches
2. **pkg/mcp/application/core** - Interface implementation issues
3. **pkg/mcp/application/orchestration/pipeline** - Method signature updates needed
4. **pkg/mcp/infra/transport** - Depends on application/core

## Performance Status

### Current Benchmarks
| Benchmark | Performance | Status |
|-----------|-------------|---------|
| HandleConversation | 914.2 ns/op | ✅ Excellent |
| StructValidation | 8,700 ns/op | ✅ Good |

Target: <300μs (300,000 ns) P95 - Currently meeting targets

### Monitoring Infrastructure
- ✅ Benchmark tracking: `scripts/performance/track_benchmarks.sh`
- ✅ Regression detection: `scripts/performance/compare_benchmarks.py`
- ✅ Baseline established: `benchmarks/baselines/initial_baseline.txt`

## Interface Evolution

### Before (Multiple Managers)
```go
// 4 large interfaces with 65+ methods
type ToolManager interface { /* 20+ methods */ }
type SessionManager interface { /* 15+ methods */ }
type WorkflowManager interface { /* 15+ methods */ }
type ConfigManager interface { /* 15+ methods */ }
```

### After (Focused Services)
```go
// 8 focused services with ~32 methods total
type ServiceContainer interface {
    ToolRegistry() ToolRegistry      // 7 methods
    SessionManager() SessionManager   // 7 methods
    WorkflowEngine() WorkflowEngine   // 5 methods
    BuildExecutor() BuildExecutor     // 3 methods
    Scanner() Scanner                 // 3 methods
    ConfigValidator() ConfigValidator // 3 methods
    ErrorReporter() ErrorReporter     // 2 methods
    Storage() Storage                 // 4 methods
}
```

## Migration Path

### Phase 1: Foundation (Week 1-2) - ALPHA
- Clean up package structure
- Fix circular dependencies
- Establish boundaries

### Phase 2: Unification (Week 3-4) - BETA/GAMMA
- Consolidate registries
- Unify error handling
- Standardize interfaces

### Phase 3: Integration (Week 5-6) - DELTA
- Pipeline consolidation
- Workflow improvements
- Performance optimization

### Phase 4: Polish (Week 7-9) - EPSILON
- Documentation completion
- Test coverage improvement
- Production readiness

## Known Issues

1. **Context Parameters**: Ongoing addition of context.Context to methods
2. **Interface Mismatches**: Some implementations need updating
3. **Import Cycles**: Being resolved by ALPHA workstream
4. **Test Coverage**: Currently ~15%, target 55%

## Next Steps

1. Fix compilation errors in 4 packages
2. Complete interface migrations
3. Implement OpenTelemetry
4. Increase test coverage
5. Create production deployment guide
STATE_EOF

# Create examples directory structure
cat > docs/examples/README.md << 'EXAMPLES_EOF'
# Container Kit Examples

This directory contains examples of using Container Kit for various containerization tasks.

## Quick Start Examples

- [Basic Containerization](basic-containerization.md)
- [Multi-Stage Builds](multi-stage-builds.md)
- [Security Scanning](security-scanning.md)
- [Kubernetes Deployment](kubernetes-deployment.md)

## Advanced Examples

- [Custom Tool Implementation](custom-tool.md)
- [Pipeline Composition](pipeline-composition.md)
- [Workflow Automation](workflow-automation.md)
- [Error Handling Patterns](error-handling.md)

## Code Examples

### Basic Tool Usage

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"

    "github.com/Azure/container-kit/pkg/mcp/application/core"
)

func main() {
    // Initialize server
    server, err := core.NewServer(core.ServerConfig{
        Mode: core.ModeDual,
    })
    if err != nil {
        log.Fatal(err)
    }

    // Get tool registry
    registry := server.ToolRegistry()

    // Execute analyze tool
    args := map[string]interface{}{
        "repository": "/path/to/repo",
        "framework": "auto-detect",
    }

    argsJSON, _ := json.Marshal(args)
    result, err := registry.Execute(context.Background(), "analyze", argsJSON)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Analysis result: %s\n", result)
}
```

### Pipeline Example

```go
// Create a build pipeline
pipeline := NewPipeline("container-build").
    AddStage(&AnalyzeStage{}).
    AddStage(&BuildStage{
        EnableCache: true,
        Platforms: []string{"linux/amd64", "linux/arm64"},
    }).
    AddStage(&ScanStage{
        Severity: "CRITICAL,HIGH",
    }).
    AddStage(&PushStage{
        Registry: "myregistry.azurecr.io",
    })

// Execute pipeline
request := &PipelineRequest{
    ID: "build-123",
    Input: map[string]interface{}{
        "repository": "/workspace/myapp",
        "tag": "v1.0.0",
    },
}

response, err := pipeline.Execute(ctx, request)
```

### Session Management

```go
// Create a session
session, err := sessionManager.Create(ctx, SessionConfig{
    Name: "containerization-session",
    Metadata: map[string]string{
        "project": "myapp",
        "environment": "production",
    },
})

// Use session workspace
workspace := session.Workspace
fmt.Printf("Working in: %s\n", workspace)

// Create checkpoint
err = sessionManager.Checkpoint(ctx, session.ID)

// ... do work ...

// Cleanup
err = sessionManager.Delete(ctx, session.ID)
```

## Best Practices

1. **Always use contexts** for cancellation and timeout support
2. **Handle errors properly** using the RichError system
3. **Set appropriate timeouts** for long-running operations
4. **Use sessions** for isolated workspaces
5. **Monitor performance** using built-in metrics

## Getting Help

- [API Documentation](../api/README.md)
- [Architecture Guide](../architecture/README.md)
- [Troubleshooting Guide](../guides/troubleshooting.md)
EXAMPLES_EOF

# Create getting started guide
cat > docs/guides/getting-started.md << 'GUIDE_EOF'
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

# Build the MCP server
make mcp

# Run tests
make test
```

## Basic Usage

### 1. Start the MCP Server

```bash
# Run in chat mode (interactive)
./bin/mcp-server --mode chat

# Run in workflow mode (automation)
./bin/mcp-server --mode workflow

# Run in dual mode (both)
./bin/mcp-server --mode dual
```

### 2. Analyze a Repository

```bash
# Using the CLI
mcp-cli analyze --repo /path/to/your/app

# Using the API
curl -X POST http://localhost:8080/tools/analyze \
  -H "Content-Type: application/json" \
  -d '{"repository": "/path/to/your/app"}'
```

### 3. Build a Container

```bash
# Build with automatic Dockerfile generation
mcp-cli build --repo /path/to/your/app --tag myapp:latest

# Build with existing Dockerfile
mcp-cli build --dockerfile /path/to/Dockerfile --tag myapp:latest
```

### 4. Security Scanning

```bash
# Scan for vulnerabilities
mcp-cli scan --image myapp:latest --severity HIGH,CRITICAL
```

### 5. Deploy to Kubernetes

```bash
# Generate manifests
mcp-cli deploy --image myapp:latest --type kubernetes

# Deploy directly
mcp-cli deploy --image myapp:latest --kubeconfig ~/.kube/config
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
```

### Configuration File

Create `~/.container-kit/config.yaml`:

```yaml
server:
  mode: dual
  port: 8080

tools:
  timeout: 30s
  retry:
    max_attempts: 3
    backoff: exponential

storage:
  type: boltdb
  path: ~/.container-kit/data

monitoring:
  metrics: true
  tracing: true
  prometheus_port: 9090
```

## Common Workflows

### Containerize a Node.js Application

```bash
# Analyze and generate Dockerfile
mcp-cli analyze --repo ./my-node-app --framework node

# Review generated Dockerfile
cat ./my-node-app/Dockerfile

# Build and scan
mcp-cli build --repo ./my-node-app --tag my-node-app:latest
mcp-cli scan --image my-node-app:latest

# Deploy
mcp-cli deploy --image my-node-app:latest --type kubernetes > k8s-manifests.yaml
kubectl apply -f k8s-manifests.yaml
```

### Multi-Stage Python Build

```bash
# Create a workflow file
cat > containerize.yaml << EOF
name: containerize-python
stages:
  - name: analyze
    tool: analyze
    args:
      repository: ./my-python-app
      framework: python

  - name: optimize
    tool: optimize
    args:
      dockerfile: ./my-python-app/Dockerfile
      target_size: minimal

  - name: build
    tool: build
    args:
      dockerfile: ./my-python-app/Dockerfile
      tag: my-python-app:latest
      platforms:
        - linux/amd64
        - linux/arm64

  - name: scan
    tool: scan
    args:
      image: my-python-app:latest
      fail_on: CRITICAL
EOF

# Execute workflow
mcp-cli workflow run containerize.yaml
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

# Run with verbose output
mcp-cli --verbose analyze --repo ./myapp

# Check logs
tail -f ~/.container-kit/logs/mcp-server.log
```

## Next Steps

- [API Documentation](../api/README.md)
- [Examples](../examples/README.md)
- [Advanced Configuration](./advanced-config.md)
- [Custom Tool Development](./custom-tools.md)
GUIDE_EOF

echo "✅ Documentation infrastructure setup complete"
echo ""
echo "Generated documentation:"
echo "- API documentation: docs/api/"
echo "- Architecture docs: docs/architecture/"
echo "- Examples: docs/examples/"
echo "- Guides: docs/guides/"
