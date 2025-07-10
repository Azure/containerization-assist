# OpenTelemetry Integration Guide

This guide covers how to use Container Kit's OpenTelemetry integration for comprehensive observability.

## Overview

Container Kit includes built-in OpenTelemetry support for:
- **Distributed Tracing**: Track requests across services and components
- **Metrics Collection**: Monitor performance and business metrics
- **Context Propagation**: Maintain trace context across async operations
- **Resource Attribution**: Tag telemetry with deployment information

## Quick Start

### 1. Basic Setup

```go
import "github.com/Azure/container-kit/pkg/mcp/infra/telemetry"

// Initialize telemetry
config := telemetry.DefaultConfig()
tm := telemetry.NewManager(config)

ctx := context.Background()
if err := tm.Initialize(ctx); err != nil {
    log.Fatal("Failed to initialize telemetry:", err)
}
defer tm.Shutdown(ctx)
```

### 2. Environment Configuration

```bash
# Enable telemetry
export CONTAINER_KIT_TRACING_ENABLED=true
export CONTAINER_KIT_METRICS_ENABLED=true

# Configure endpoints
export OTEL_EXPORTER_JAEGER_ENDPOINT=http://jaeger:14268/api/traces
export OTEL_EXPORTER_PROMETHEUS_ENDPOINT=http://prometheus:9090

# Set sampling rate (0.0 to 1.0)
export CONTAINER_KIT_TRACE_SAMPLE_RATE=0.1

# Set service metadata
export CONTAINER_KIT_VERSION=1.0.0
export CONTAINER_KIT_ENV=production
```

### 3. Start Monitoring Stack

```bash
# Set up monitoring infrastructure
./scripts/monitoring/setup_monitoring.sh

# Start monitoring services
./scripts/monitoring/start_monitoring.sh
```

## Instrumentation Patterns

### Tool Execution

```go
func (t *MyTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
    return tm.InstrumentToolExecution(ctx, "my-tool", func(ctx context.Context) error {
        // Add tool-specific attributes
        tm.AddContextualAttributes(ctx,
            attribute.String("tool.version", "1.0.0"),
            attribute.String("input.size", fmt.Sprintf("%d", len(args))),
        )

        // Record significant events
        tm.RecordEvent(ctx, "tool.validation.started")

        // Your tool logic here
        result, err := t.processInput(ctx, args)

        if err != nil {
            tm.RecordEvent(ctx, "tool.validation.failed",
                attribute.String("error.type", "validation"),
            )
            return err
        }

        tm.RecordEvent(ctx, "tool.processing.completed",
            attribute.Int("result.items", len(result)),
        )

        return nil
    })
}
```

### Pipeline Stages

```go
func (p *Pipeline) ExecuteStage(ctx context.Context, stage string) error {
    return tm.InstrumentPipelineStage(ctx, p.Name, stage, func(ctx context.Context) error {
        // Add stage context
        tm.AddContextualAttributes(ctx,
            attribute.String("stage.input", getStageInput()),
            attribute.String("stage.config", getStageConfig()),
        )

        // Execute stage logic
        return p.executeStageLogic(ctx, stage)
    })
}
```

### HTTP Handlers

```go
func (s *Server) handleToolExecution(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    statusCode, err := tm.InstrumentHTTPRequest(ctx, r.Method, r.URL.Path, func(ctx context.Context) (int, error) {
        // Add request context
        tm.AddContextualAttributes(ctx,
            attribute.String("user.id", getUserID(r)),
            attribute.String("client.version", getClientVersion(r)),
        )

        // Process request
        result, err := s.processRequest(ctx, r)
        if err != nil {
            return 500, err
        }

        // Send response
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(result)
        return 200, nil
    })

    if err != nil {
        http.Error(w, err.Error(), statusCode)
    }
}
```

### Session Management

```go
func (sm *SessionManager) CreateSession(ctx context.Context, config SessionConfig) (*Session, error) {
    ctx, span := tm.Tracing().StartSpan(ctx, "session.create")
    defer span.End()

    start := time.Now()

    tm.AddContextualAttributes(ctx,
        attribute.String("session.type", config.Type),
        attribute.String("user.id", config.UserID),
    )

    session, err := sm.createSessionInternal(ctx, config)

    // Record metrics
    if err != nil {
        tm.Metrics().RecordSessionCreation(ctx, "failed")
    } else {
        tm.Metrics().RecordSessionCreation(ctx, config.Type)
        tm.RecordEvent(ctx, "session.created",
            attribute.String("session.id", session.ID),
        )
    }

    return session, err
}
```

## Advanced Usage

### Custom Metrics

```go
// Create custom histogram
duration, err := tm.Metrics().meter.Float64Histogram(
    "custom_operation_duration",
    metric.WithDescription("Duration of custom operation"),
    metric.WithUnit("s"),
)

// Record measurements
func recordCustomMetric(ctx context.Context, operationType string, dur time.Duration) {
    duration.Record(ctx, dur.Seconds(),
        metric.WithAttributes(attribute.String("operation.type", operationType)),
    )
}
```

### Distributed Tracing

```go
// Propagate trace context across service boundaries
func callExternalService(ctx context.Context, url string) error {
    // Create HTTP client with tracing
    client := &http.Client{}

    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return err
    }

    // OpenTelemetry automatically injects trace headers
    ctx, span := tm.Tracing().StartSpan(ctx, "external.service.call")
    defer span.End()

    tm.AddContextualAttributes(ctx,
        attribute.String("http.url", url),
        attribute.String("service.name", "external-api"),
    )

    resp, err := client.Do(req)
    if err != nil {
        tm.Tracing().RecordError(span, err)
        return err
    }
    defer resp.Body.Close()

    tm.AddContextualAttributes(ctx,
        attribute.Int("http.status_code", resp.StatusCode),
    )

    return nil
}
```

### Error Handling

```go
func handleOperation(ctx context.Context) error {
    ctx, span := tm.Tracing().StartSpan(ctx, "complex.operation")
    defer span.End()

    // Add operation context
    tm.AddContextualAttributes(ctx,
        attribute.String("operation.id", "op-123"),
        attribute.String("operation.type", "batch-processing"),
    )

    for i, item := range items {
        ctx, itemSpan := tm.Tracing().StartSpan(ctx, "process.item")

        tm.AddContextualAttributes(ctx,
            attribute.Int("item.index", i),
            attribute.String("item.id", item.ID),
        )

        if err := processItem(ctx, item); err != nil {
            // Record error with context
            tm.Tracing().RecordError(itemSpan, err)
            tm.RecordEvent(ctx, "item.processing.failed",
                attribute.String("error.type", getErrorType(err)),
                attribute.String("item.id", item.ID),
            )

            itemSpan.End()

            // Decide whether to continue or fail entire operation
            if isCriticalError(err) {
                return fmt.Errorf("critical error processing item %s: %w", item.ID, err)
            }
            continue
        }

        tm.RecordEvent(ctx, "item.processing.completed")
        itemSpan.End()
    }

    return nil
}
```

## Configuration Options

### Development Configuration

```go
config := &telemetry.Config{
    ServiceName:     "container-kit-dev",
    ServiceVersion:  "dev",
    Environment:     "development",
    TracingEnabled:  true,
    TracingEndpoint: "", // Uses stdout exporter
    TraceSampleRate: 1.0, // Sample all traces
    MetricsEnabled:  true,
    MetricsInterval: 5 * time.Second,
}
```

### Production Configuration

```go
config := &telemetry.Config{
    ServiceName:     "container-kit",
    ServiceVersion:  "1.0.0",
    Environment:     "production",
    TracingEnabled:  true,
    TracingEndpoint: "http://jaeger-collector:14268/api/traces",
    TraceSampleRate: 0.1, // Sample 10% of traces
    MetricsEnabled:  true,
    MetricsInterval: 15 * time.Second,
    ResourceAttributes: map[string]string{
        "deployment.region": "us-west-2",
        "deployment.zone":   "us-west-2a",
        "cluster.name":      "prod-cluster",
        "pod.name":          os.Getenv("POD_NAME"),
        "node.name":         os.Getenv("NODE_NAME"),
    },
}
```

### Testing Configuration

```go
config := &telemetry.Config{
    ServiceName:     "container-kit-test",
    ServiceVersion:  "test",
    Environment:     "test",
    TracingEnabled:  false, // Disable for unit tests
    MetricsEnabled:  false, // Disable for unit tests
}
```

## Monitoring and Dashboards

### Grafana Dashboards

Access pre-built dashboards at `http://localhost:3000`:

1. **Performance Overview**: Tool execution latency, error rates
2. **System Metrics**: Memory, CPU, goroutines
3. **HTTP Metrics**: Request latency, status codes
4. **Pipeline Metrics**: Stage execution, success rates
5. **Business Metrics**: Session creation, tool usage

### Prometheus Queries

Common queries for monitoring:

```promql
# P95 tool execution latency
histogram_quantile(0.95, rate(tool_execution_duration_seconds_bucket[5m]))

# Error rate by tool
rate(tool_errors_total[5m]) / rate(tool_execution_total[5m])

# HTTP request rate
rate(http_requests_total[1m])

# Memory usage
system_memory_usage_bytes / 1024 / 1024 / 1024

# Active sessions
session_total - session_completed_total
```

### Jaeger Tracing

Access traces at `http://localhost:16686`:

1. **Search by service**: Filter traces by Container Kit
2. **Trace timeline**: See request flow across components
3. **Error analysis**: Find failed operations and root causes
4. **Performance analysis**: Identify slow operations

## Alerting

### Critical Alerts

```yaml
# Tool latency above threshold
- alert: HighToolLatency
  expr: histogram_quantile(0.95, rate(tool_execution_duration_seconds_bucket[5m])) > 0.0003
  for: 5m

# High error rate
- alert: HighErrorRate
  expr: rate(tool_errors_total[5m]) > 0.1
  for: 2m

# Service down
- alert: ServiceDown
  expr: up{job="container-kit"} == 0
  for: 1m
```

### Warning Alerts

```yaml
# Memory usage
- alert: HighMemoryUsage
  expr: system_memory_usage_bytes / 1024 / 1024 / 1024 > 2
  for: 5m

# Goroutine leak
- alert: HighGoroutineCount
  expr: system_goroutines_count > 1000
  for: 5m
```

## Best Practices

### 1. Attribute Naming

Use consistent attribute naming:

```go
// Good
attribute.String("tool.name", "analyze")
attribute.String("tool.version", "1.0.0")
attribute.String("user.id", "user123")

// Avoid
attribute.String("toolName", "analyze")
attribute.String("version", "1.0.0")
attribute.String("user", "user123")
```

### 2. Span Naming

Use descriptive, hierarchical span names:

```go
// Good
"tool.analyze"
"pipeline.container-build.analyze"
"http.POST /api/tools/execute"

// Avoid
"analyze"
"build"
"request"
```

### 3. Error Recording

Always record errors with context:

```go
if err := operation(); err != nil {
    tm.Tracing().RecordError(span, err)
    tm.RecordEvent(ctx, "operation.failed",
        attribute.String("error.type", getErrorType(err)),
        attribute.String("error.component", "validator"),
    )
    return err
}
```

### 4. Performance Considerations

- Use appropriate sampling rates for production
- Avoid high-cardinality attributes in metrics
- Batch metric recordings when possible
- Use async exporters for better performance

### 5. Security

- Don't include sensitive data in traces/metrics
- Use attribute filters in production
- Secure monitoring endpoints
- Rotate API keys regularly

## Troubleshooting

### Common Issues

1. **No traces appearing**:
   - Check `CONTAINER_KIT_TRACING_ENABLED=true`
   - Verify Jaeger endpoint configuration
   - Check sampling rate settings

2. **High memory usage**:
   - Reduce trace sampling rate
   - Check for span leaks (unclosed spans)
   - Adjust batch sizes

3. **Missing metrics**:
   - Verify `CONTAINER_KIT_METRICS_ENABLED=true`
   - Check Prometheus scraping configuration
   - Validate metric endpoint `/metrics`

### Debug Mode

Enable debug logging:

```bash
export CONTAINER_KIT_LOG_LEVEL=debug
export OTEL_LOG_LEVEL=debug
```

### Health Checks

```bash
# Check telemetry status
curl http://localhost:8080/health/telemetry

# Check metrics endpoint
curl http://localhost:8080/metrics

# Validate configuration
scripts/monitoring/check_monitoring.sh
```

## Integration Examples

See complete examples in:
- [`pkg/mcp/infra/telemetry/integration_example.go`](../pkg/mcp/infra/telemetry/integration_example.go)
- [`test/integration/telemetry_test.go`](../test/integration/telemetry_test.go)

## Resources

- [OpenTelemetry Go Documentation](https://opentelemetry.io/docs/instrumentation/go/)
- [Prometheus Metrics](https://prometheus.io/docs/concepts/metric_types/)
- [Jaeger Tracing](https://www.jaegertracing.io/docs/)
- [Grafana Dashboards](https://grafana.com/docs/grafana/latest/dashboards/)

---

*For questions or improvements, see the [monitoring setup guide](../scripts/monitoring/README.md)*
