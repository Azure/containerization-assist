# ADR-009: Messaging and Observability Infrastructure

Date: 2025-07-14
Status: Accepted
Context: Container Kit requires robust messaging for progress tracking and comprehensive observability for monitoring workflow execution across distributed containerization operations
Decision: Implement unified messaging and observability infrastructure with clear separation between progress reporting, event publishing, health monitoring, metrics collection, and distributed tracing
Consequences: Enhanced workflow visibility, improved debugging capabilities, and better operational monitoring at the cost of increased infrastructure complexity

## Context

Container Kit's containerization workflows require comprehensive observability and messaging capabilities:

1. **Progress Tracking**: Real-time progress reporting for long-running containerization workflows (10-step process)
2. **Event Publishing**: Asynchronous event handling for workflow state changes and system events
3. **Health Monitoring**: System health checks and service availability monitoring
4. **Metrics Collection**: Performance metrics and operational statistics
5. **Distributed Tracing**: Request tracing across workflow steps and external service calls

The system needed a unified approach to handle these cross-cutting concerns without tightly coupling them to business logic.

## Decision

We establish unified messaging and observability infrastructure in two main packages:

### 1. Messaging Infrastructure (`pkg/mcp/infrastructure/messaging/`)

#### Progress Reporting (`progress/`)
Handles real-time progress tracking for workflow execution:
- **Base Sink**: `base_sink.go` - Core progress reporting interface
- **CLI Sink**: `cli_sink.go` - Command-line progress output
- **MCP Sink**: `mcp_sink.go` - MCP protocol progress notifications
- **Factory Pattern**: `factory.go` - Sink creation and management
- **Progress Emitter**: `emitter.go` - Event-driven progress broadcasting

#### Event Publishing (`events/`)
Manages asynchronous event handling:
- **Event Publisher**: `publisher.go` - Centralized event publishing
- **Domain Events**: Integration with `pkg/mcp/domain/events/`
- **Async Processing**: Non-blocking event handling for workflow events

#### Unified Providers
```go
var MessagingProviders = wire.NewSet(
    // Progress tracking
    progress.NewSinkFactory,
    
    // Event publishing (when implemented)
    // events.NewPublisher,
)
```

### 2. Observability Infrastructure (`pkg/mcp/infrastructure/observability/`)

#### Health Monitoring (`health/`)
System health checks and service monitoring:
- **Health Monitor**: `monitor.go` - Service health aggregation
- **Component Health**: Individual service health tracking
- **Health Endpoints**: HTTP health check endpoints

#### Metrics Collection (`metrics/`)
Performance and operational metrics:
- **Metrics Collector**: `collector.go` - Metrics aggregation and export
- **Workflow Metrics**: Step execution times, success rates, error counts
- **System Metrics**: Resource usage, performance characteristics

#### Distributed Tracing (`tracing/`)
Request tracing across workflow execution:
- **Tracer Adapter**: `adapter.go` - Tracing implementation abstraction
- **Integration**: `integration.go` - Workflow step tracing
- **Configuration**: `config.go` - Tracing setup and configuration
- **Helpers**: `helpers.go` - Trace context utilities

#### Unified Providers
```go
var ObservabilityProviders = wire.NewSet(
    // Tracing
    tracing.NewTracerAdapter,
    
    // Health monitoring  
    health.NewMonitor,
)
```

## Architecture Principles

### 1. Separation of Concerns
- **Messaging**: Focus on communication and progress reporting
- **Observability**: Focus on monitoring, metrics, and tracing
- **Clean Interfaces**: Clear boundaries between messaging and observability

### 2. Event-Driven Design
- **Progress Events**: Real-time progress updates through event system
- **Workflow Events**: State changes published as domain events
- **Async Processing**: Non-blocking event handling

### 3. Provider Pattern
- **Factory Creation**: Standardized service creation through Wire providers
- **Configuration**: Environment-based configuration for different deployment scenarios
- **Extensibility**: Easy addition of new sinks, collectors, or tracers

### 4. Multi-Modal Output
- **CLI Progress**: Human-readable progress output for command-line usage
- **MCP Progress**: Structured progress for MCP protocol clients
- **Event Streaming**: Real-time event streams for external monitoring

## Implementation Details

### Progress Sink Architecture
Multiple output formats for workflow progress:
```go
type ProgressSink interface {
    ReportProgress(step int, total int, message string)
    ReportError(err error)
    ReportCompletion()
}
```

### Factory Pattern for Sinks
Environment-aware sink creation:
```go
func NewSinkFactory() *SinkFactory {
    return &SinkFactory{
        sinks: map[string]ProgressSink{
            "cli": NewCLISink(),
            "mcp": NewMCPSink(), 
        },
    }
}
```

### Tracing Integration
Workflow steps are automatically traced:
```go
func (t *TracerAdapter) TraceStep(ctx context.Context, step string) context.Context {
    span := t.tracer.StartSpan(step)
    return context.WithValue(ctx, spanKey, span)
}
```

### Metrics Collection
Performance metrics for workflow monitoring:
```go
type WorkflowMetrics struct {
    StepDuration    time.Duration
    ErrorCount      int
    SuccessRate     float64
    ResourceUsage   ResourceMetrics
}
```

## Consequences

### Positive
- **Enhanced Visibility**: Comprehensive workflow monitoring and progress tracking
- **Better Debugging**: Detailed tracing and metrics for troubleshooting
- **Operational Excellence**: Health monitoring and performance metrics
- **User Experience**: Real-time progress feedback in multiple formats
- **Extensibility**: Easy addition of new monitoring capabilities

### Negative
- **Infrastructure Complexity**: Additional components to deploy and maintain
- **Performance Overhead**: Tracing and metrics collection add computational cost
- **Configuration Complexity**: Multiple observability systems to configure
- **Dependencies**: Additional external dependencies for metrics and tracing

### Performance Characteristics
- **Progress Reporting**: <5ms P95 for progress sink operations
- **Event Publishing**: <10ms P95 for event publishing
- **Tracing Overhead**: <1% performance impact with sampling
- **Metrics Collection**: <2ms P95 for metrics recording

## Integration with Workflow System

### Progress Tracking
Workflow steps automatically report progress:
```go
func (w *WorkflowExecutor) ExecuteStep(ctx context.Context, step WorkflowStep) error {
    w.progress.ReportProgress(step.Number, w.totalSteps, step.Description)
    // ... execute step
    return nil
}
```

### Error Context Integration
Observability integrates with error context system:
```go
func (e *ErrorContext) RecordError(err error) {
    e.errors = append(e.errors, err)
    e.metrics.RecordError(err)
    e.tracer.RecordError(err)
}
```

### Health Check Integration
Health monitoring includes workflow state:
```go
func (h *HealthMonitor) CheckWorkflowHealth() HealthStatus {
    return HealthStatus{
        ActiveWorkflows: h.workflowCount,
        LastSuccess:    h.lastSuccessTime,
        ErrorRate:      h.calculateErrorRate(),
    }
}
```

## Compliance

This infrastructure aligns with Container Kit's four-layer architecture:
- **Infrastructure Layer**: Messaging and observability implementations
- **Domain Layer**: Progress and event interfaces in `pkg/mcp/domain/`
- **Application Layer**: Integration through workflow orchestration
- **API Layer**: Health check and metrics endpoints

## Configuration

### Environment Variables
- `TRACING_ENABLED`: Enable/disable distributed tracing
- `METRICS_ENDPOINT`: Metrics collection endpoint
- `PROGRESS_FORMAT`: Progress output format (cli|mcp|both)
- `HEALTH_CHECK_INTERVAL`: Health check frequency

### Deployment Considerations
- **Development**: CLI progress, minimal tracing
- **Production**: Full observability stack with external metrics/tracing
- **Testing**: Mock implementations for controlled testing

## References
- Progress sink tests: `pkg/mcp/infrastructure/messaging/progress/*_test.go`
- Tracing integration: `pkg/mcp/infrastructure/observability/tracing/integration_test.go`
- Health monitoring: `pkg/mcp/infrastructure/observability/health/monitor.go`
- Performance benchmarks: <300μs P95 maintained with full observability