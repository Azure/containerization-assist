# Workflow Orchestration System

This package provides a comprehensive workflow orchestration system for the MCP (Model Context Protocol) server, enabling declarative workflow definition and intelligent execution of atomic tools.

## Overview

The orchestration system transforms the existing atomic tools into a powerful, declarative workflow engine that supports:

- **Declarative Workflow Specification**: YAML-based workflow definitions with stages, dependencies, and conditions
- **Intelligent Dependency Resolution**: Automatic topological sorting with parallel execution optimization
- **Advanced Error Handling**: Contextual error routing with automatic recovery strategies
- **Session Management**: Persistent workflow state with checkpoint/restore capabilities
- **Progress Tracking**: Standardized progress reporting across all workflow stages
- **Template Integration**: Seamless integration with the existing template system

## Architecture

### Core Components

1. **WorkflowEngine** (`workflow.go`) - Main orchestration engine
2. **WorkflowSessionManager** (`session_manager.go`) - Persistent session management
3. **DependencyResolver** (`dependency_resolver.go`) - Stage dependency resolution
4. **ErrorRouter** (`error_router.go`) - Intelligent error handling and recovery
5. **CheckpointManager** (`checkpoint_manager.go`) - Workflow checkpoint management
6. **StageExecutor** (`stage_executor.go`) - Individual stage execution

### Workflow Specification Format

```yaml
apiVersion: orchestration/v1
kind: Workflow
metadata:
  name: containerization-pipeline
  description: Complete containerization pipeline
  version: 1.0.0
spec:
  stages:
    - name: analysis
      tools: [analyze_repository_atomic]
      conditions:
        - key: repo_url
          operator: required
    - name: build
      tools: [build_image_atomic]
      depends_on: [analysis]
      retry_policy:
        max_attempts: 3
        backoff_mode: exponential
    - name: security-scan
      tools: [scan_image_security_atomic]
      depends_on: [build]
      parallel: false
  variables:
    registry: myregistry.azurecr.io
    namespace: default
  error_policy:
    mode: fail_fast
    max_failures: 3
```

## Key Features

### 1. Declarative Workflow Definition

- **Stage-based Architecture**: Workflows are composed of stages, each containing one or more tools
- **Dependency Management**: Explicit dependency declaration with automatic resolution
- **Conditional Execution**: Stages can be conditionally executed based on context
- **Variable Substitution**: Support for workflow and stage-level variables

### 2. Intelligent Execution

- **Parallel Execution**: Automatic parallelization of independent stages
- **Resource Optimization**: Smart resource allocation and cleanup
- **Timeout Management**: Configurable timeouts at workflow and stage levels
- **Progress Tracking**: Real-time progress reporting with weight-based calculations

### 3. Advanced Error Handling

- **Error Routing**: Contextual error routing based on error type, stage, and tool
- **Recovery Strategies**: Automatic recovery with configurable retry policies
- **Failure Actions**: Support for retry, redirect, skip, and fail actions
- **Error Analytics**: Comprehensive error tracking and analysis

### 4. Session Management

- **Persistent State**: Workflow state persisted in BoltDB
- **Session Metrics**: Comprehensive metrics and analytics
- **Resource Tracking**: Automatic resource binding and cleanup
- **Context Sharing**: Shared context between stages and tools

### 5. Checkpoint System

- **Automatic Checkpoints**: Configurable checkpoint creation during execution
- **Manual Checkpoints**: Support for manual checkpoint creation
- **Restore Capability**: Resume workflows from any checkpoint
- **Checkpoint Analytics**: Metrics and analysis of checkpoint usage

## Usage Examples

### Basic Workflow Execution

```go
// Create workflow orchestrator
orchestrator := NewWorkflowOrchestrator(db, toolRegistry, toolOrchestrator, logger)

// Execute predefined workflow
result, err := orchestrator.ExecuteWorkflow(
    ctx,
    "containerization-pipeline",
    WithVariables(map[string]string{
        "repo_url": "https://github.com/example/app",
        "registry": "myregistry.azurecr.io",
    }),
    WithCheckpoints(),
    WithMaxConcurrency(3),
)
```

### Custom Workflow Creation

```go
customWorkflow := &WorkflowSpec{
    APIVersion: "orchestration/v1",
    Kind:       "Workflow",
    Metadata: WorkflowMetadata{
        Name:        "custom-security-audit",
        Description: "Custom security audit workflow",
        Version:     "1.0.0",
    },
    Spec: WorkflowDefinition{
        Stages: []WorkflowStage{
            {
                Name:     "security-scan",
                Tools:    []string{"scan_image_security_atomic"},
                Parallel: false,
            },
        },
    },
}

result, err := orchestrator.ExecuteCustomWorkflow(ctx, customWorkflow)
```

### Workflow Management

```go
// Pause active workflow
err := orchestrator.PauseWorkflow(sessionID)

// Resume paused workflow
result, err := orchestrator.ResumeWorkflow(ctx, sessionID)

// Cancel workflow
err := orchestrator.CancelWorkflow(sessionID)

// Get workflow status
session, err := orchestrator.GetWorkflowStatus(sessionID)
```

## Predefined Workflows

The system includes several predefined workflows:

1. **containerization-pipeline**: Complete containerization from source to deployment
2. **security-focused-pipeline**: Enhanced security scanning and validation
3. **development-workflow**: Fast development workflow with minimal checks
4. **production-deployment**: Production-ready deployment with comprehensive validation
5. **ci-cd-pipeline**: Complete CI/CD pipeline with staging and production promotion

## Integration with Existing MCP System

The orchestration system is designed to seamlessly integrate with the existing MCP atomic tools:

### Tool Registry Integration

```go
type ToolRegistry interface {
    RegisterTool(name string, tool interface{}) error
    GetTool(name string) (interface{}, error)
    ListTools() []string
    ValidateTool(name string) error
}
```

### Tool Orchestrator Integration

```go
type ToolOrchestrator interface {
    ExecuteTool(ctx context.Context, toolName string, args interface{}, session interface{}) (interface{}, error)
    ValidateToolArgs(toolName string, args interface{}) error
    GetToolMetadata(toolName string) (*ToolMetadata, error)
}
```

## Advanced Features

### 1. Dependency Analysis

```go
// Analyze workflow complexity
analysis, err := orchestrator.AnalyzeWorkflowComplexity(workflowSpec)

// Get optimization suggestions
suggestions, err := orchestrator.GetOptimizationSuggestions(workflowSpec)

// Get dependency graph
graph, err := orchestrator.GetDependencyGraph(workflowSpec)
```

### 2. Error Routing Configuration

```go
// Add custom error routing
orchestrator.AddCustomErrorRoute("build_stage", ErrorRoutingRule{
    ID:          "build_error_redirect",
    Name:        "Build Error Redirect",
    Conditions:  []RoutingCondition{{Field: "error_type", Operator: "contains", Value: "build_error"}},
    Action:      "redirect",
    RedirectTo:  "validate_dockerfile",
    Priority:    100,
})
```

### 3. Checkpoint Management

```go
// Create manual checkpoint
checkpoint, err := orchestrator.CreateCheckpoint(sessionID, "critical-stage", "Pre-production checkpoint")

// List checkpoints
checkpoints, err := orchestrator.ListCheckpoints(sessionID)

// Restore from checkpoint
session, err := orchestrator.RestoreFromCheckpoint(sessionID, checkpointID)
```

### 4. Metrics and Analytics

```go
// Get comprehensive metrics
metrics, err := orchestrator.GetMetrics()

// Cleanup old resources
cleanup, err := orchestrator.CleanupResources(24 * time.Hour)
```

## Benefits

### For Users
- **Simplified Workflows**: Declarative configuration eliminates complex scripting
- **Reliability**: Automatic error handling and recovery improve success rates
- **Visibility**: Rich progress tracking and metrics provide clear insights
- **Flexibility**: Support for both predefined and custom workflows

### For Developers
- **Maintainability**: Clear separation of concerns and modular design
- **Extensibility**: Easy to add new tools and workflow capabilities
- **Testability**: Each component can be tested independently
- **Monitoring**: Comprehensive logging and metrics for debugging

### For Operations
- **Scalability**: Parallel execution and resource optimization
- **Reliability**: Checkpoint system enables recovery from failures
- **Observability**: Detailed metrics and error tracking
- **Automation**: Reduced manual intervention requirements

## Future Enhancements

1. **Advanced Scheduling**: Support for scheduled and recurring workflows
2. **Workflow Templates**: Template-based workflow generation
3. **Performance Optimization**: Machine learning-based execution optimization
4. **Integration APIs**: REST/GraphQL APIs for external integration
5. **Workflow Visualization**: Interactive workflow visualization and monitoring
6. **Multi-tenant Support**: Support for multiple isolated workflow environments

## File Organization

```
pkg/mcp/internal/orchestration/
├── workflow.go              # Core workflow engine and types
├── session_manager.go       # Workflow session management
├── dependency_resolver.go   # Stage dependency resolution
├── error_router.go         # Error handling and routing
├── checkpoint_manager.go   # Checkpoint management
├── stage_executor.go       # Stage execution logic
├── examples.go             # Predefined workflow examples
├── integration_example.go  # Integration examples
└── README.md              # This documentation
```

This orchestration system represents a significant enhancement to the MCP server, providing enterprise-grade workflow capabilities while maintaining the simplicity and power of the existing atomic tools.