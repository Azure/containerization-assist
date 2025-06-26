# MCP Observability Guide

Comprehensive guide to the enhanced observability features in MCP, including telemetry, distributed tracing, and advanced dashboards.

## Overview

The MCP observability stack provides:
- **Enhanced Telemetry**: Detailed metrics with SLO tracking
- **Distributed Tracing**: Full request lifecycle visibility
- **Real-time Dashboard**: Interactive metrics visualization
- **Quality Monitoring**: Code quality and error handling metrics

## Architecture

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│  MCP Tools      │────▶│ Telemetry Layer  │────▶│ Exporters       │
└─────────────────┘     └──────────────────┘     └─────────────────┘
                               │                           │
                               ▼                           ▼
                        ┌──────────────────┐     ┌─────────────────┐
                        │ Tracing Manager  │     │ Prometheus      │
                        └──────────────────┘     └─────────────────┘
                               │                           │
                               ▼                           ▼
                        ┌──────────────────┐     ┌─────────────────┐
                        │ Enhanced Dashboard│────▶│ OTLP Collector  │
                        └──────────────────┘     └─────────────────┘
```

## Components

### 1. Enhanced Telemetry (`telemetry_enhancements.go`)

Advanced metrics collection beyond basic Prometheus metrics:

#### Quality Metrics
- Error handling adoption rate
- Test coverage percentage
- Interface compliance score
- Overall code quality score

#### Performance Metrics
- P50, P90, P95, P99 latency percentiles
- Throughput with sliding windows
- Resource utilization (CPU, memory, goroutines)
- File descriptor tracking

#### SLO/SLI Metrics
- Service Level Objective compliance
- Error budget tracking
- Availability rate monitoring
- Burn rate calculations

#### Usage Example
```go
// Initialize enhanced telemetry
enhancedManager, err := observability.NewEnhancedTelemetryManager(baseManager)

// Record tool execution with enhanced metrics
enhancedManager.RecordToolExecution(ctx, "docker_build", duration, success, "BuildError")

// Update code quality metrics
enhancedManager.RecordCodeQualityMetrics(
    errorHandlingRate,
    testCoverage,
    interfaceCompliance,
)

// Track panics
defer func() {
    if r := recover(); r != nil {
        enhancedManager.RecordPanic("main.handler")
    }
}()
```

### 2. Distributed Tracing (`distributed_tracing.go`)

OpenTelemetry-based distributed tracing for request flow visualization:

#### Features
- Automatic span creation and propagation
- Context enrichment with baggage
- Tool-specific tracing helpers
- HTTP/Database/Async operation tracing

#### Configuration
```go
config := &DistributedTracingConfig{
    ServiceName:    "mcp-server",
    ServiceVersion: "1.0.0",
    Environment:    "production",
    OTLPEndpoint:   "localhost:4317",
    SampleRate:     0.1, // 10% sampling
    BatchTimeout:   5 * time.Second,
    ExportTimeout:  10 * time.Second,
}

tracingManager, err := NewDistributedTracingManager(config)
```

#### Usage Patterns
```go
// Tool execution tracing
ctx, span := tracingManager.StartToolSpan(ctx, "terraform", "apply")
defer span.End()

// HTTP operation tracing
ctx, span := tracingManager.StartHTTPSpan(ctx, "POST", "/api/deploy")
defer span.End()

// Database operation tracing
ctx, span := tracingManager.StartDatabaseSpan(ctx, "postgres", "SELECT", "deployments")
defer span.End()

// Error recording
if err != nil {
    tracingManager.RecordError(ctx, err)
}

// Add custom events
tracingManager.AddEvent(ctx, "validation_completed",
    attribute.String("validator", "schema"),
    attribute.Bool("passed", true),
)
```

### 3. Tracing Integration (`tracing_integration.go`)

High-level integration patterns for common scenarios:

#### Workflow Tracing
```go
workflow := []WorkflowStep{
    {
        Name: "validate",
        Type: "validation",
        Handler: validateFunc,
        Timeout: 30 * time.Second,
    },
    {
        Name: "build",
        Type: "build",
        Handler: buildFunc,
        ContinueOnError: false,
    },
}

err := integration.TraceWorkflow(ctx, "deployment_pipeline", workflow)
```

#### Batch Operations
```go
items := []interface{}{item1, item2, item3}
err := integration.TraceBatch(ctx, "bulk_update", items, func(ctx context.Context, item interface{}) error {
    // Process item with automatic tracing
    return processItem(ctx, item)
})
```

#### Cache Operations
```go
result, err := integration.TraceCache(ctx, "GET", "user:123", func(ctx context.Context) (interface{}, error) {
    // Cache miss - fetch from database
    return fetchUser(ctx, "123")
})
```

### 4. Telemetry Exporter (`telemetry_exporter.go`)

Advanced metrics export with dashboard integration:

#### Endpoints
- `/metrics` - Standard Prometheus metrics
- `/metrics/enhanced` - Enhanced metrics with context
- `/dashboard` - Interactive web dashboard
- `/health` - Health check with alert status
- `/alerts` - Active alerts
- `/slo` - SLO compliance status
- `/api/v1/query` - Programmatic metric queries

#### Alert Rules
```go
alertRules := []AlertRule{
    {
        Name:       "High Error Rate",
        Query:      "error_rate",
        Threshold:  5.0,
        Comparator: ">",
        Duration:   5 * time.Minute,
        Severity:   "critical",
    },
}
```

### 5. Enhanced Dashboard (`enhanced_dashboard.go`)

Real-time metrics visualization with WebSocket support:

#### Features
- Real-time metric updates
- Historical data visualization
- Active operation tracking
- Interactive charts
- Dark theme UI
- WebSocket for live updates

#### Dashboard Sections
1. **System Overview**: Key metrics at a glance
2. **Performance**: Throughput, latency, error rates
3. **Error Analysis**: Error categorization and trends
4. **Distributed Traces**: Request flow visualization
5. **Active Operations**: In-progress operation tracking
6. **Code Quality**: Quality metrics and trends
7. **SLO Status**: Service level objective compliance
8. **Alerts**: Active alerts and incidents

## Integration Guide

### 1. Enable Enhanced Telemetry

```go
// In main.go or server initialization
baseManager := telemetry.NewTelemetryManager()
enhancedManager, err := observability.NewEnhancedTelemetryManager(baseManager)
if err != nil {
    log.Fatal(err)
}

// Register enhanced metrics endpoints
exporter := observability.NewTelemetryExporter(enhancedManager)
http.Handle("/metrics", exporter)
http.Handle("/dashboard", exporter)
```

### 2. Configure Distributed Tracing

```go
// Set up tracing
tracingConfig := &observability.DistributedTracingConfig{
    ServiceName:  "mcp-server",
    OTLPEndpoint: os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
    SampleRate:   0.1,
}

tracingManager, err := observability.NewDistributedTracingManager(tracingConfig)
if err != nil {
    log.Fatal(err)
}
defer tracingManager.Shutdown(context.Background())

// Add tracing middleware
handler = observability.TracingMiddleware(tracingManager)(handler)
```

### 3. Instrument Tools

```go
// In tool execution
func (t *MyTool) Execute(ctx context.Context) error {
    // Start tool span
    ctx, span := tracingManager.StartToolSpan(ctx, "my_tool", "execute")
    defer span.End()

    // Record start
    start := time.Now()

    // Execute tool logic
    err := t.doWork(ctx)

    // Record metrics
    enhancedManager.RecordToolExecution(ctx, "my_tool", time.Since(start), err == nil, categorizeError(err))

    return err
}
```

### 4. Track Code Quality

```go
// Run periodically or in CI/CD
metrics := collectQualityMetrics()
enhancedManager.RecordCodeQualityMetrics(
    metrics.ErrorHandlingRate,
    metrics.TestCoverage,
    metrics.InterfaceCompliance,
)
```

## Configuration

### Environment Variables

```bash
# Telemetry
TELEMETRY_ENABLED=true
TELEMETRY_PORT=9090
TELEMETRY_EXPORT_INTERVAL=30s

# Tracing
OTEL_ENABLED=true
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317
OTEL_TRACES_SAMPLER_RATIO=0.1
OTEL_SERVICE_NAME=mcp-server
OTEL_SERVICE_VERSION=1.0.0

# Dashboard
DASHBOARD_ENABLED=true
DASHBOARD_PORT=8080
DASHBOARD_THEME=dark
DASHBOARD_REFRESH_INTERVAL=5s
```

### Prometheus Configuration

```yaml
# prometheus.yml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'mcp-server'
    static_configs:
      - targets: ['localhost:9090']
    metric_relabel_configs:
      - source_labels: [__name__]
        regex: 'mcp_.*'
        action: keep
```

### OTLP Collector Configuration

```yaml
# otel-collector.yml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

processors:
  batch:
    timeout: 5s
    send_batch_size: 1024

exporters:
  prometheus:
    endpoint: "0.0.0.0:8889"
  jaeger:
    endpoint: jaeger:14250
    tls:
      insecure: true

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [jaeger]
    metrics:
      receivers: [otlp]
      processors: [batch]
      exporters: [prometheus]
```

## Best Practices

### 1. Metric Naming
- Use consistent prefixes: `mcp_`
- Include units in names: `_seconds`, `_bytes`
- Use standard labels: `tool`, `operation`, `status`

### 2. Trace Sampling
- Start with 1-10% sampling in production
- Increase for debugging specific issues
- Always sample errors and slow requests

### 3. Dashboard Usage
- Set up alerts for critical metrics
- Review SLO compliance weekly
- Use historical data for capacity planning

### 4. Performance Impact
- Metrics collection: <1% overhead
- Tracing (10% sampling): ~2-3% overhead
- Dashboard: Minimal (separate process)

## Monitoring Workflows

### 1. Incident Response
1. Check dashboard for anomalies
2. Review active alerts
3. Examine distributed traces
4. Analyze error patterns
5. Check SLO burn rate

### 2. Performance Optimization
1. Identify slow operations in traces
2. Review P95/P99 latency trends
3. Analyze resource utilization
4. Check for error rate spikes
5. Optimize based on data

### 3. Quality Improvement
1. Monitor error handling adoption
2. Track test coverage trends
3. Review interface compliance
4. Identify technical debt areas
5. Plan improvements based on metrics

## Troubleshooting

### High Memory Usage
- Check goroutine count
- Review trace sampling rate
- Verify metric cardinality
- Check for memory leaks in dashboard

### Missing Traces
- Verify OTLP endpoint connectivity
- Check sampling configuration
- Ensure context propagation
- Review trace export errors

### Dashboard Not Loading
- Check WebSocket connection
- Verify metric endpoints
- Review browser console
- Check CORS settings

### Metrics Not Updating
- Verify scrape configuration
- Check metric registration
- Review export intervals
- Ensure no name conflicts

## Future Enhancements

1. **Machine Learning Integration**
   - Anomaly detection
   - Predictive alerting
   - Capacity forecasting

2. **Advanced Visualizations**
   - Service dependency maps
   - Error flow diagrams
   - Performance flame graphs

3. **Integration Expansions**
   - Slack/PagerDuty alerts
   - Grafana dashboards
   - DataDog/NewRelic export

4. **Security Monitoring**
   - Authentication metrics
   - API usage tracking
   - Security event correlation
