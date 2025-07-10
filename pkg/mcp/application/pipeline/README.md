# Unified Pipeline System

The unified pipeline system consolidates all pipeline implementations into a single, coherent interface.

## Pipeline Types

- **AtomicPipeline**: Atomic execution with rollback capability
- **WorkflowPipeline**: Sequential/parallel workflow execution
- **OrchestrationPipeline**: Full orchestration with timeout/retry

## Usage

```go
// Create atomic pipeline
pipeline := pipeline.NewAtomicPipeline(
    stage1,
    stage2,
    stage3,
)

// Execute pipeline
response, err := pipeline.Execute(ctx, request)
```

## Builder Pattern

```go
// Use builder for complex pipelines
pipeline := pipeline.New().
    WithStages(stage1, stage2).
    WithTimeout(30 * time.Second).
    WithRetry(retryPolicy).
    WithMetrics(metrics).
    Build()
```

## Features

- **Unified Interface**: All pipeline types implement the same `api.Pipeline` interface
- **Fluent API**: Builder pattern for easy pipeline construction
- **Thread Safety**: All operations are safe for concurrent use
- **Metrics Support**: Optional metrics collection for performance monitoring
- **Retry Logic**: Configurable retry policies with backoff
- **Stage Registry**: Centralized management of pipeline stages
- **Command Routing**: Map-based routing replacing switch statements
