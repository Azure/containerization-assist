# OpenTelemetry Tracing Integration

This package provides OpenTelemetry tracing integration for the Container Kit MCP server, with instrumentation for both sampling and progress components.

## Features

- **Sampling Tracing**: Automatic instrumentation of MCP sampling requests with retry tracking
- **Progress Tracing**: Traces workflow progress updates and step execution
- **Validation Tracing**: Tracks content validation operations with security issue detection
- **Configurable**: Environment variable configuration with sensible defaults
- **Production Ready**: Proper error handling, sampling control, and resource cleanup

## Configuration

### Environment Variables

- `CONTAINER_KIT_OTEL_ENABLED`: Enable/disable tracing (default: false)
- `CONTAINER_KIT_OTEL_ENDPOINT`: OTLP endpoint (default: http://localhost:4318/v1/traces)
- `CONTAINER_KIT_OTEL_HEADERS`: Additional headers as comma-separated key=value pairs
- `CONTAINER_KIT_TRACE_SAMPLE_RATE`: Sampling rate 0.0-1.0 (default: 1.0)

### Server Configuration

```go
// Initialize tracing from server configuration
serverInfo := tracing.ServerInfo{
    ServiceName:     "container-kit-mcp",
    ServiceVersion:  "v1.0.0",
    Environment:     "production",
    TraceSampleRate: 0.1,
}

err := tracing.InitFromServerInfo(ctx, serverInfo, logger)
if err != nil {
    log.Fatal("Failed to initialize tracing:", err)
}

// Cleanup on shutdown
defer tracing.Shutdown(ctx)
```

## Usage Examples

### Manual Span Creation

```go
ctx, span := tracing.StartSpan(ctx, "custom.operation")
defer span.End()

span.SetAttributes(
    attribute.String("operation.type", "analysis"),
    attribute.Int("content.size", len(content)),
)
```

### Sampling Operations

```go
// Automatic tracing for sampling requests
err := tracing.TraceSamplingRequest(ctx, "kubernetes-manifest-fix", func(ctx context.Context) error {
    result, err := client.AnalyzeKubernetesManifest(ctx, manifest, error, dockerfile, analysis)
    return err
})
```

### Progress Operations  

```go
// Automatic tracing for progress updates
err := tracing.TraceProgressUpdate(ctx, workflowID, "build-image", 3, 10, func(ctx context.Context) error {
    // Progress update logic here
    return nil
})
```

### Workflow Steps

```go
// Automatic tracing for workflow steps
err := tracing.TraceWorkflowStep(ctx, workflowID, "deploy", func(ctx context.Context) error {
    // Deployment logic here
    return nil
})
```

## Trace Attributes

### Sampling Attributes

- `sampling.template_id`: Template used for sampling
- `sampling.tokens_used`: Total tokens consumed
- `sampling.content_type`: Type of content being processed
- `sampling.validation_valid`: Whether validation passed
- `sampling.security_issues`: Number of security issues found

### Progress Attributes

- `progress.workflow_id`: Unique workflow identifier
- `progress.step_name`: Name of the current step
- `progress.step_number`: Current step number
- `progress.total_steps`: Total number of steps
- `progress.percentage`: Completion percentage

### Component Attributes

- `component`: Component name (sampling, progress, workflow)
- `operation`: Specific operation being performed
- `duration_ms`: Operation duration in milliseconds

## Integration Points

### Sampling Client

The sampling client automatically creates spans for:
- Individual sampling requests with retry tracking
- Content validation operations
- Security issue detection
- Token usage and performance metrics

### Progress Manager

The progress manager automatically creates spans for:
- Progress updates with step information
- Workflow step execution
- Error handling and recovery

### Workflow Engine

Workflow operations can use the `WorkflowTracer` for:
- End-to-end workflow tracing
- Step-by-step execution tracking
- Error correlation across steps

## Performance Considerations

- **Sampling**: Use appropriate sampling rates for high-throughput scenarios
- **Batching**: OTLP exporter automatically batches spans for efficiency
- **Resource Usage**: Minimal overhead when tracing is disabled
- **Error Handling**: Tracing failures don't affect application functionality

## Observability Platforms

This integration works with any OpenTelemetry-compatible platform:

- **Jaeger**: Distributed tracing and root cause analysis
- **Zipkin**: Request flow visualization
- **Datadog**: APM and correlation with metrics/logs
- **New Relic**: Application performance monitoring
- **AWS X-Ray**: Distributed tracing for AWS environments
- **Google Cloud Trace**: GCP native tracing

## Example Trace Flow

```
containerize_and_deploy [workflow]
├── analyze_repository [step]
├── generate_dockerfile [step]
│   └── sampling.request [sampling]
│       ├── sampling.validation [validation]
│       └── retry.backoff [retry]
├── build_image [step]
└── deploy_kubernetes [step]
    └── sampling.request [sampling]
        └── sampling.validation [validation]
```

Each span includes relevant attributes, events, and error information for comprehensive observability.