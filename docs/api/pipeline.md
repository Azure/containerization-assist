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
