# Monitoring and Observability Guide

Container Kit provides comprehensive monitoring and observability through Prometheus metrics, OpenTelemetry tracing, and structured logging. This guide covers setup, configuration, and best practices for production monitoring.

## Observability Architecture

### Three Pillars of Observability
1. **Metrics**: Quantitative measurements (Prometheus)
2. **Traces**: Request flow tracking (OpenTelemetry/Jaeger)
3. **Logs**: Event records (Structured logging with slog)

### Components Overview
```
┌─────────────────┐    ┌──────────────┐    ┌─────────────┐
│  Container Kit  │───▶│  Prometheus  │───▶│   Grafana   │
│                 │    │              │    │             │
│                 │    └──────────────┘    └─────────────┘
│                 │    
│                 │    ┌──────────────┐    ┌─────────────┐
│                 │───▶│   Jaeger     │───▶│   UI/Query  │
│                 │    │              │    │             │
│                 │    └──────────────┘    └─────────────┘
│                 │    
│                 │    ┌──────────────┐    ┌─────────────┐
│                 │───▶│   Log Store  │───▶│  Analysis   │
└─────────────────┘    │ (ELK/Loki)   │    │             │
                       └──────────────┘    └─────────────┘
```

## Metrics Collection

### Prometheus Integration
```go
// pkg/mcp/infra/telemetry/metrics.go
type MetricsCollector struct {
    requestDuration *prometheus.HistogramVec
    requestCount    *prometheus.CounterVec
    sessionCount    prometheus.Gauge
    errorCount      *prometheus.CounterVec
    
    // FileAccessService metrics
    fileOperations  *prometheus.CounterVec
    fileSize        *prometheus.HistogramVec
    
    // Performance metrics
    responseTime    *prometheus.HistogramVec
    memoryUsage     prometheus.Gauge
    cpuUsage        prometheus.Gauge
}

func NewMetricsCollector() *MetricsCollector {
    return &MetricsCollector{
        requestDuration: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name:    "container_kit_request_duration_seconds",
                Help:    "Request duration in seconds",
                Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.3, 0.5, 1.0},
            },
            []string{"tool", "session_id", "status"},
        ),
        requestCount: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "container_kit_requests_total",
                Help: "Total number of requests",
            },
            []string{"tool", "status"},
        ),
        sessionCount: prometheus.NewGauge(
            prometheus.GaugeOpts{
                Name: "container_kit_active_sessions",
                Help: "Number of active sessions",
            },
        ),
        fileOperations: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "container_kit_file_operations_total",
                Help: "Total file operations",
            },
            []string{"operation", "session_id", "status"},
        ),
    }
}
```

### Custom Metrics
```go
// Tool-specific metrics
func (m *MetricsCollector) RecordToolExecution(tool string, duration time.Duration, err error) {
    status := "success"
    if err != nil {
        status = "error"
        m.errorCount.WithLabelValues(tool, err.Error()).Inc()
    }
    
    m.requestDuration.WithLabelValues(tool, "", status).Observe(duration.Seconds())
    m.requestCount.WithLabelValues(tool, status).Inc()
}

// FileAccessService metrics
func (m *MetricsCollector) RecordFileOperation(operation, sessionID string, size int64, duration time.Duration, err error) {
    status := "success"
    if err != nil {
        status = "error"
    }
    
    m.fileOperations.WithLabelValues(operation, sessionID, status).Inc()
    m.fileSize.WithLabelValues(operation).Observe(float64(size))
    m.requestDuration.WithLabelValues("file_access", sessionID, status).Observe(duration.Seconds())
}

// Session metrics
func (m *MetricsCollector) UpdateSessionCount(count int) {
    m.sessionCount.Set(float64(count))
}
```

### Metrics Endpoint
```go
func (s *Server) setupMetricsEndpoint() {
    // Register metrics
    prometheus.MustRegister(s.metrics.requestDuration)
    prometheus.MustRegister(s.metrics.requestCount)
    prometheus.MustRegister(s.metrics.sessionCount)
    prometheus.MustRegister(s.metrics.errorCount)
    prometheus.MustRegister(s.metrics.fileOperations)
    
    // Expose metrics endpoint
    http.Handle("/metrics", promhttp.Handler())
    
    // Custom metrics handler with additional info
    http.HandleFunc("/metrics/custom", s.customMetricsHandler)
}

func (s *Server) customMetricsHandler(w http.ResponseWriter, r *http.Request) {
    metrics := map[string]interface{}{
        "uptime":           time.Since(s.startTime).Seconds(),
        "version":          s.version,
        "active_sessions":  s.sessionManager.ActiveCount(),
        "memory_usage":     getMemoryUsage(),
        "cpu_usage":        getCPUUsage(),
        "disk_usage":       getDiskUsage(),
        "goroutines":       runtime.NumGoroutine(),
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(metrics)
}
```

## Distributed Tracing

### OpenTelemetry Setup
```go
// pkg/mcp/infra/telemetry/tracing.go
func InitTracing(serviceName, endpoint string) (*sdktrace.TracerProvider, error) {
    exporter, err := jaeger.New(
        jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(endpoint)),
    )
    if err != nil {
        return nil, err
    }
    
    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exporter),
        sdktrace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceNameKey.String(serviceName),
            semconv.ServiceVersionKey.String(version),
        )),
        sdktrace.WithSampler(sdktrace.AlwaysSample()),
    )
    
    otel.SetTracerProvider(tp)
    otel.SetTextMapPropagator(propagation.TraceContext{})
    
    return tp, nil
}
```

### Request Tracing
```go
func (s *Server) traceMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ctx, span := otel.Tracer("container-kit").Start(r.Context(), r.URL.Path)
        defer span.End()
        
        // Add request attributes
        span.SetAttributes(
            attribute.String("http.method", r.Method),
            attribute.String("http.url", r.URL.String()),
            attribute.String("user_agent", r.UserAgent()),
        )
        
        // Add to context
        r = r.WithContext(ctx)
        
        // Continue with request
        next.ServeHTTP(w, r)
        
        // Add response attributes
        span.SetAttributes(
            attribute.Int("http.status_code", w.Header().Get("Status")),
        )
    })
}
```

### Tool Execution Tracing
```go
func (t *AnalyzeTool) Execute(ctx context.Context, args *AnalyzeArgs) (*AnalyzeResult, error) {
    ctx, span := otel.Tracer("container-kit-tools").Start(ctx, "analyze_repository")
    defer span.End()
    
    // Add tool-specific attributes
    span.SetAttributes(
        attribute.String("tool.name", "analyze_repository"),
        attribute.String("session.id", args.SessionID),
        attribute.String("repository.path", args.RepositoryPath),
    )
    
    // Trace file operations
    ctx, fileSpan := otel.Tracer("container-kit-files").Start(ctx, "file_analysis")
    files, err := t.fileAccess.ListDirectory(ctx, args.SessionID, args.RepositoryPath)
    fileSpan.SetAttributes(
        attribute.Int("files.count", len(files)),
    )
    fileSpan.End()
    
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        return nil, err
    }
    
    // Continue with analysis...
    result, err := t.performAnalysis(ctx, args, files)
    
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
    } else {
        span.SetAttributes(
            attribute.String("analysis.language", result.Language),
            attribute.String("analysis.framework", result.Framework),
        )
        span.SetStatus(codes.Ok, "Analysis completed")
    }
    
    return result, err
}
```

## Structured Logging

### Logger Configuration
```go
// pkg/mcp/infra/telemetry/logging.go
type LogConfig struct {
    Level  string `yaml:"level"`
    Format string `yaml:"format"` // "json" or "text"
    Output string `yaml:"output"` // "stdout", "stderr", or file path
}

func NewLogger(config *LogConfig) (*slog.Logger, error) {
    var level slog.Level
    switch strings.ToLower(config.Level) {
    case "debug":
        level = slog.LevelDebug
    case "info":
        level = slog.LevelInfo
    case "warn":
        level = slog.LevelWarn
    case "error":
        level = slog.LevelError
    default:
        level = slog.LevelInfo
    }
    
    var handler slog.Handler
    
    if config.Format == "json" {
        handler = slog.NewJSONHandler(getOutput(config.Output), &slog.HandlerOptions{
            Level: level,
            AddSource: true,
        })
    } else {
        handler = slog.NewTextHandler(getOutput(config.Output), &slog.HandlerOptions{
            Level: level,
            AddSource: true,
        })
    }
    
    return slog.New(handler), nil
}
```

### Contextual Logging
```go
func (s *Service) processRequest(ctx context.Context, req *Request) {
    logger := s.logger.With(
        "request_id", getRequestID(ctx),
        "session_id", req.SessionID,
        "tool", req.Tool,
        "timestamp", time.Now(),
    )
    
    logger.Info("Processing request",
        "user_id", req.UserID,
        "operation", req.Operation,
    )
    
    start := time.Now()
    defer func() {
        duration := time.Since(start)
        logger.Info("Request completed",
            "duration_ms", duration.Milliseconds(),
            "status", "success",
        )
    }()
    
    // Process request...
}
```

### Error Logging
```go
func (s *Service) handleError(ctx context.Context, err error, operation string) {
    logger := s.logger.With(
        "error_id", uuid.New().String(),
        "operation", operation,
        "timestamp", time.Now(),
    )
    
    // Log based on error type
    switch e := err.(type) {
    case *ValidationError:
        logger.Warn("Validation error",
            "error", e.Error(),
            "field", e.Field,
            "value", e.Value,
        )
    case *SecurityError:
        logger.Error("Security error",
            "error", e.Error(),
            "threat_type", e.ThreatType,
            "source_ip", getSourceIP(ctx),
        )
    default:
        logger.Error("Unexpected error",
            "error", e.Error(),
            "stack_trace", getStackTrace(e),
        )
    }
}
```

## Dashboard Configuration

### Grafana Dashboard
```json
{
  "dashboard": {
    "title": "Container Kit Monitoring",
    "panels": [
      {
        "title": "Request Rate",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(container_kit_requests_total[5m])",
            "legendFormat": "{{tool}} - {{status}}"
          }
        ]
      },
      {
        "title": "Response Time P95",
        "type": "graph",
        "targets": [
          {
            "expr": "histogram_quantile(0.95, rate(container_kit_request_duration_seconds_bucket[5m]))",
            "legendFormat": "P95 Response Time"
          }
        ]
      },
      {
        "title": "Active Sessions",
        "type": "singlestat",
        "targets": [
          {
            "expr": "container_kit_active_sessions",
            "legendFormat": "Active Sessions"
          }
        ]
      },
      {
        "title": "Error Rate",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(container_kit_requests_total{status=\"error\"}[5m])",
            "legendFormat": "Error Rate"
          }
        ]
      },
      {
        "title": "File Operations",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(container_kit_file_operations_total[5m])",
            "legendFormat": "{{operation}} - {{status}}"
          }
        ]
      }
    ]
  }
}
```

### Prometheus Alerts
```yaml
# alerts.yml
groups:
  - name: container-kit
    rules:
      - alert: HighErrorRate
        expr: rate(container_kit_requests_total{status="error"}[5m]) > 0.1
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "High error rate detected"
          description: "Error rate is {{ $value }} per second"
          
      - alert: HighResponseTime
        expr: histogram_quantile(0.95, rate(container_kit_request_duration_seconds_bucket[5m])) > 0.3
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "High response time detected"
          description: "P95 response time is {{ $value }}s"
          
      - alert: ServiceDown
        expr: up{job="container-kit"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Container Kit service is down"
          description: "Service has been down for more than 1 minute"
          
      - alert: HighMemoryUsage
        expr: container_kit_memory_usage_bytes / 1024 / 1024 > 512
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High memory usage"
          description: "Memory usage is {{ $value }}MB"
```

## Performance Monitoring

### Custom Performance Metrics
```go
type PerformanceMonitor struct {
    metrics *MetricsCollector
    logger  *slog.Logger
}

func (pm *PerformanceMonitor) MonitorToolPerformance(tool string, fn func() error) error {
    start := time.Now()
    
    defer func() {
        duration := time.Since(start)
        pm.metrics.RecordToolExecution(tool, duration, nil)
        
        if duration > 300*time.Millisecond {
            pm.logger.Warn("Slow tool execution",
                "tool", tool,
                "duration_ms", duration.Milliseconds(),
                "threshold_ms", 300,
            )
        }
    }()
    
    return fn()
}

func (pm *PerformanceMonitor) MonitorFileOperation(operation, sessionID string, fn func() (int64, error)) error {
    start := time.Now()
    
    size, err := fn()
    duration := time.Since(start)
    
    pm.metrics.RecordFileOperation(operation, sessionID, size, duration, err)
    
    return err
}
```

### Resource Monitoring
```go
func (pm *PerformanceMonitor) collectSystemMetrics() {
    // Memory usage
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    
    pm.metrics.memoryUsage.Set(float64(m.Alloc))
    
    // CPU usage
    cpuPercent := getCPUUsage()
    pm.metrics.cpuUsage.Set(cpuPercent)
    
    // Goroutine count
    pm.metrics.goroutineCount.Set(float64(runtime.NumGoroutine()))
    
    // Session count
    sessionCount := pm.sessionManager.ActiveCount()
    pm.metrics.sessionCount.Set(float64(sessionCount))
}
```

## Log Aggregation

### ELK Stack Integration
```yaml
# docker-compose-elk.yml
version: '3.8'
services:
  elasticsearch:
    image: docker.elastic.co/elasticsearch/elasticsearch:8.0.0
    environment:
      - discovery.type=single-node
      - "ES_JAVA_OPTS=-Xms512m -Xmx512m"
    ports:
      - "9200:9200"
      
  logstash:
    image: docker.elastic.co/logstash/logstash:8.0.0
    volumes:
      - ./logstash.conf:/usr/share/logstash/pipeline/logstash.conf
    ports:
      - "5044:5044"
    depends_on:
      - elasticsearch
      
  kibana:
    image: docker.elastic.co/kibana/kibana:8.0.0
    ports:
      - "5601:5601"
    depends_on:
      - elasticsearch
      
  filebeat:
    image: docker.elastic.co/beats/filebeat:8.0.0
    volumes:
      - ./filebeat.yml:/usr/share/filebeat/filebeat.yml
      - /var/log/container-kit:/var/log/container-kit:ro
    depends_on:
      - logstash
```

### Loki Integration
```yaml
# promtail-config.yml
server:
  http_listen_port: 9080
  grpc_listen_port: 0

positions:
  filename: /tmp/positions.yaml

clients:
  - url: http://loki:3100/loki/api/v1/push

scrape_configs:
  - job_name: container-kit
    static_configs:
      - targets:
          - localhost
        labels:
          job: container-kit
          __path__: /var/log/container-kit/*.log
    pipeline_stages:
      - json:
          expressions:
            level: level
            timestamp: timestamp
            message: message
            session_id: session_id
            tool: tool
      - timestamp:
          source: timestamp
          format: RFC3339
      - labels:
          level:
          session_id:
          tool:
```

## Health Monitoring

### Health Check Endpoints
```go
func (s *Server) setupHealthEndpoints() {
    http.HandleFunc("/health", s.healthHandler)
    http.HandleFunc("/health/live", s.livenessHandler)
    http.HandleFunc("/health/ready", s.readinessHandler)
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
    health := &HealthStatus{
        Status:    "healthy",
        Timestamp: time.Now(),
        Version:   s.version,
        Uptime:    time.Since(s.startTime),
        Checks:    make(map[string]interface{}),
    }
    
    // Check database
    if err := s.db.Ping(); err != nil {
        health.Status = "unhealthy"
        health.Checks["database"] = map[string]interface{}{
            "status": "failed",
            "error":  err.Error(),
        }
    } else {
        health.Checks["database"] = map[string]interface{}{
            "status": "healthy",
        }
    }
    
    // Check file system
    if err := checkFileSystem(); err != nil {
        health.Status = "unhealthy"
        health.Checks["filesystem"] = map[string]interface{}{
            "status": "failed",
            "error":  err.Error(),
        }
    } else {
        health.Checks["filesystem"] = map[string]interface{}{
            "status": "healthy",
        }
    }
    
    if health.Status != "healthy" {
        w.WriteHeader(http.StatusServiceUnavailable)
    }
    
    json.NewEncoder(w).Encode(health)
}
```

## Alerting

### AlertManager Configuration
```yaml
# alertmanager.yml
global:
  smtp_smarthost: 'localhost:587'
  smtp_from: 'alerts@container-kit.example.com'

route:
  group_by: ['alertname']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 1h
  receiver: 'web.hook'

receivers:
  - name: 'web.hook'
    email_configs:
      - to: 'admin@container-kit.example.com'
        subject: 'Container Kit Alert: {{ .GroupLabels.alertname }}'
        body: |
          {{ range .Alerts }}
          Alert: {{ .Annotations.summary }}
          Description: {{ .Annotations.description }}
          {{ end }}
    slack_configs:
      - api_url: 'YOUR_SLACK_WEBHOOK_URL'
        channel: '#alerts'
        title: 'Container Kit Alert'
        text: '{{ range .Alerts }}{{ .Annotations.description }}{{ end }}'
```

## Related Documentation

- [Performance Guide](performance.md)
- [Security Guide](security.md)
- [Deployment Guide](deployment.md)
- [Architecture Overview](../../architecture/overview.md)

Container Kit's comprehensive monitoring and observability stack provides deep insights into system performance, user behavior, and potential issues, enabling proactive maintenance and optimization.