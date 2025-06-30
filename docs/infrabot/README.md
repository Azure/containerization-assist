# InfraBot - Core Infrastructure Implementation

## Overview

InfraBot is the foundational workstream of the Container Kit MCP server, responsible for implementing core Docker operations, session tracking infrastructure, and atomic tool frameworks. This documentation provides comprehensive guidance for developers, operators, and other teams integrating with InfraBot.

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Core Components](#core-components)
3. [API Reference](#api-reference)
4. [Integration Guide](#integration-guide)
5. [Performance & Monitoring](#performance--monitoring)
6. [Testing Framework](#testing-framework)
7. [Deployment Guide](#deployment-guide)
8. [Troubleshooting](#troubleshooting)

## Architecture Overview

InfraBot provides the foundational infrastructure for Container Kit's containerization workflows through three primary pillars:

### 1. Docker Operations Framework
- **Pull Operations**: Secure image pulling with authentication and progress tracking
- **Push Operations**: Registry publishing with multi-registry support
- **Tag Operations**: Image tagging with validation and conflict resolution
- **Authentication**: Integrated registry authentication for multiple providers

### 2. Session Tracking Infrastructure
- **Session Management**: Complete lifecycle tracking for all containerization sessions
- **Error Tracking**: Comprehensive error collection and analysis across teams
- **Job Tracking**: Workflow job management with performance metrics
- **Tool Tracking**: Execution history and performance analysis for all tools

### 3. Atomic Tool Framework
- **Base Framework**: Standardized execution patterns for atomic operations
- **Progress Tracking**: Real-time progress reporting and monitoring
- **Resource Management**: Memory, CPU, and I/O resource tracking
- **Error Handling**: Centralized error handling and recovery mechanisms

## Core Components

### Docker Operations (`pkg/mcp/internal/pipeline/operations.go`)

The Docker operations component provides secure, reliable container image management.

#### Key Features
- **Multi-registry Support**: Docker Hub, ECR, GCR, ACR, and private registries
- **Authentication Management**: Secure credential handling with multiple auth methods
- **Progress Tracking**: Real-time operation progress with detailed metrics
- **Error Recovery**: Automatic retry with exponential backoff
- **Resource Monitoring**: Memory and network usage tracking

#### Usage Example
```go
// Initialize operations
ops := pipeline.NewOperations(config, logger)

// Pull an image with authentication
err := ops.PullDockerImage(sessionID, "nginx:latest")
if err != nil {
    log.Error().Err(err).Msg("Image pull failed")
}

// Push to registry
err = ops.PushDockerImage(sessionID, "myregistry.com/app:v1.0")
if err != nil {
    log.Error().Err(err).Msg("Image push failed")
}

// Tag an image
err = ops.TagDockerImage(sessionID, "nginx:latest", "myregistry.com/nginx:prod")
```

### Session Management (`pkg/mcp/internal/session/session_manager.go`)

The session management system provides comprehensive tracking and coordination across all Container Kit operations.

#### Key Features
- **Session Lifecycle**: Complete tracking from creation to completion
- **Multi-team Coordination**: Integration points for BuildSecBot, OrchBot, and AdvancedBot
- **Resource Monitoring**: Memory, CPU, and storage usage per session
- **Error Analytics**: Detailed error classification and trending
- **Performance Metrics**: Operation timing and throughput analysis

#### Session States
- `CREATED`: Session initialized but not started
- `ACTIVE`: Session currently executing operations
- `PAUSED`: Session temporarily suspended
- `COMPLETED`: Session finished successfully
- `FAILED`: Session terminated with errors
- `CLEANUP`: Session resources being cleaned up

#### Usage Example
```go
// Create a new session
session, err := sessionManager.CreateSession("team-alpha", sessionConfig)
if err != nil {
    return fmt.Errorf("session creation failed: %w", err)
}

// Track an operation
err = sessionManager.TrackOperation(session.ID, "docker_pull", metadata)
if err != nil {
    log.Warn().Err(err).Msg("Operation tracking failed")
}

// Handle errors
err = sessionManager.TrackError(session.ID, operationError, context)
if err != nil {
    log.Error().Err(err).Msg("Error tracking failed")
}
```

### Atomic Tool Framework (`pkg/mcp/internal/runtime/atomic_tool_base.go`)

The atomic tool framework provides standardized execution patterns for BuildSecBot and other team integrations.

#### Key Features
- **Standardized Execution**: Consistent patterns for all atomic operations
- **Progress Reporting**: Built-in progress tracking for long-running operations
- **Resource Management**: Automatic resource allocation and cleanup
- **Error Handling**: Centralized error management with retry policies
- **Observability**: Integrated metrics and tracing for all operations

#### Base Interface
```go
type AtomicToolBase interface {
    ExecuteWithoutProgress(ctx context.Context, operation func() error) error
    ExecuteWithProgress(ctx context.Context, operation func(ProgressCallback) error) error
    GetPerformanceMetrics() *PerformanceMetrics
    Cleanup(ctx context.Context) error
}

type ProgressCallback func(progress float64, message string)
```

#### Usage Example
```go
// Create atomic tool
tool := runtime.NewAtomicTool(config, logger)

// Execute without progress tracking
err := tool.ExecuteWithoutProgress(ctx, func() error {
    // Your atomic operation here
    return performOperation()
})

// Execute with progress tracking
err = tool.ExecuteWithProgress(ctx, func(progress ProgressCallback) error {
    progress(0.0, "Starting operation")
    // ... perform work ...
    progress(0.5, "Halfway complete")
    // ... perform more work ...
    progress(1.0, "Operation complete")
    return nil
})
```

## Monitoring & Observability

InfraBot includes comprehensive monitoring and observability capabilities built for production deployments.

### Metrics Collection (`pkg/mcp/internal/monitoring/metrics.go`)

#### Session Metrics
- `infrabot_mcp_sessions_total`: Total number of sessions created
- `infrabot_mcp_sessions_active`: Currently active sessions
- `infrabot_mcp_session_duration_seconds`: Session duration histogram
- `infrabot_mcp_session_errors_total`: Total session errors

#### Docker Metrics
- `infrabot_mcp_docker_operations_total`: Docker operations by type and status
- `infrabot_mcp_docker_operation_duration_seconds`: Operation duration histogram
- `infrabot_mcp_docker_cache_hit_rate`: Docker operation cache hit rate
- `infrabot_mcp_docker_image_size_bytes`: Image size distribution

#### Performance Metrics
- `infrabot_mcp_request_duration_seconds`: Request duration by endpoint
- `infrabot_mcp_throughput_operations_per_second`: Operations throughput
- `infrabot_mcp_latency_percentiles_seconds`: Latency percentiles
- `infrabot_mcp_resource_utilization_percent`: Resource utilization

### Health Monitoring (`pkg/mcp/internal/monitoring/health.go`)

#### Component Health Checks
- **Session Manager**: Validates session creation and tracking
- **Docker Operations**: Checks Docker daemon connectivity
- **Atomic Framework**: Verifies tool execution capabilities
- **Resource Monitor**: Monitors system resource availability

#### Health Status Levels
- `HEALTHY`: All components operating normally
- `DEGRADED`: Some non-critical issues detected
- `UNHEALTHY`: Critical issues affecting functionality
- `UNKNOWN`: Unable to determine component status

### Distributed Tracing (`pkg/mcp/internal/monitoring/observability.go`)

#### Trace Integration
- **OpenTelemetry**: Full OpenTelemetry integration with Jaeger
- **Span Correlation**: Automatic span correlation across team boundaries
- **Context Propagation**: Seamless context passing between operations
- **Performance Analysis**: Detailed performance breakdown by operation

#### Trace Attributes
- `session.id`: Session identifier
- `operation.type`: Type of operation (pull, push, tag)
- `team.name`: Team responsible for the operation
- `resource.usage`: Resource consumption metrics

## Performance Targets

InfraBot maintains strict performance targets to ensure optimal system performance:

### Latency Targets
- **P95 Latency**: < 300μs for all operations
- **P99 Latency**: < 1ms for all operations
- **Mean Response Time**: < 100μs for cached operations

### Throughput Targets
- **Docker Operations**: > 1000 ops/second sustained
- **Session Creation**: > 500 sessions/second
- **Atomic Tool Execution**: > 2000 tool calls/second

### Resource Limits
- **Memory Usage**: < 500MB per 1000 active sessions
- **CPU Usage**: < 10% under normal load
- **Disk I/O**: < 100MB/s sustained throughput

## Integration Guide

### Team Integration Patterns

#### BuildSecBot Integration
```go
// BuildSecBot uses InfraBot's atomic framework
atomicTool := infrabot.NewAtomicTool(config)

// Execute security scan
err := atomicTool.ExecuteWithProgress(ctx, func(progress ProgressCallback) error {
    progress(0.0, "Starting security scan")
    
    // Perform security scanning
    results, err := securityScanner.Scan(image)
    if err != nil {
        return err
    }
    
    progress(1.0, "Security scan complete")
    return nil
})
```

#### OrchBot Integration
```go
// OrchBot coordinates workflows using InfraBot sessions
session, err := infrabot.CreateSession("orchbot", config)
if err != nil {
    return err
}

// Execute workflow steps
for step := range workflowSteps {
    err := infrabot.TrackOperation(session.ID, step.Name, step.Metadata)
    if err != nil {
        log.Warn().Err(err).Msg("Operation tracking failed")
    }
}
```

#### AdvancedBot Integration
```go
// AdvancedBot uses InfraBot for Docker operations in sandboxes
dockerOps := infrabot.GetDockerOperations()

// Pull base image for sandbox
err := dockerOps.PullDockerImage(sandboxSession, baseImage)
if err != nil {
    return fmt.Errorf("sandbox setup failed: %w", err)
}
```

### API Integration

#### RESTful API Endpoints

##### Session Management
```
POST   /api/v1/sessions              # Create new session
GET    /api/v1/sessions/{id}         # Get session details
PUT    /api/v1/sessions/{id}/status  # Update session status
DELETE /api/v1/sessions/{id}         # Delete session
GET    /api/v1/sessions              # List sessions
```

##### Docker Operations
```
POST   /api/v1/docker/pull          # Pull Docker image
POST   /api/v1/docker/push          # Push Docker image
POST   /api/v1/docker/tag           # Tag Docker image
GET    /api/v1/docker/status        # Get operation status
```

##### Health & Metrics
```
GET    /api/v1/health               # System health check
GET    /api/v1/health/components    # Component health details
GET    /api/v1/metrics              # Prometheus metrics
GET    /api/v1/metrics/performance  # Performance metrics
```

## Testing Framework

InfraBot includes a comprehensive integration testing framework for validating cross-team coordination and system reliability.

### Test Categories

#### Cross-Team Integration Tests
- **BuildSecBot Integration**: Validates atomic tool framework usage
- **OrchBot Coordination**: Tests workflow coordination patterns
- **AdvancedBot Sandboxing**: Verifies sandbox environment integration

#### Performance Tests
- **Load Testing**: Validates performance under high load
- **Stress Testing**: Tests system limits and recovery
- **Endurance Testing**: Long-running stability validation

#### Contract Tests
- **API Contracts**: Validates API compatibility between teams
- **Data Contracts**: Tests data format consistency
- **Behavior Contracts**: Validates expected behavior patterns

### Running Tests

```bash
# Run all integration tests
go test -tags integration ./pkg/mcp/internal/testing/...

# Run cross-team tests only
go test -tags crossteam ./pkg/mcp/internal/testing/...

# Run performance tests
go test -tags performance ./pkg/mcp/internal/testing/...

# Run with specific team dependencies
go test -tags buildsecbot ./pkg/mcp/internal/testing/...
```

### Test Configuration

```yaml
# test-config.yaml
test_timeout: 10m
parallel_execution: true
max_retries: 3
environment_setup: true

performance_thresholds:
  latency_p95_max: 300µs
  throughput_min: 1000
  error_rate_max: 0.01

team_endpoints:
  BuildSecBot: "http://localhost:8081"
  OrchBot: "http://localhost:8082"
  AdvancedBot: "http://localhost:8083"
```

## Deployment Guide

### Prerequisites

1. **Go 1.24.1+**: Required for compilation
2. **Docker 20.10+**: Required for container operations
3. **Prometheus**: For metrics collection (optional)
4. **Jaeger**: For distributed tracing (optional)

### Configuration

```yaml
# infrabot-config.yaml
server:
  port: 8080
  timeout: 30s
  max_connections: 1000

docker:
  daemon_host: "unix:///var/run/docker.sock"
  registry_timeout: 300s
  max_concurrent_ops: 10

session:
  max_active_sessions: 1000
  cleanup_interval: 5m
  retention_period: 24h

monitoring:
  enable_metrics: true
  enable_tracing: true
  metrics_port: 9090
  jaeger_endpoint: "http://jaeger:14268/api/traces"

performance:
  target_latency_p95: 300µs
  max_memory_usage: 1GB
  max_cpu_usage: 80%
```

### Docker Deployment

```dockerfile
# Dockerfile
FROM golang:1.24.1-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o infrabot ./cmd/infrabot

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/infrabot .
COPY --from=builder /app/config/infrabot-config.yaml .
CMD ["./infrabot", "--config", "infrabot-config.yaml"]
```

```yaml
# docker-compose.yml
version: '3.8'
services:
  infrabot:
    build: .
    ports:
      - "8080:8080"
      - "9090:9090"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      - INFRABOT_LOG_LEVEL=info
    depends_on:
      - prometheus
      - jaeger

  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9091:9090"
    volumes:
      - ./config/prometheus.yml:/etc/prometheus/prometheus.yml

  jaeger:
    image: jaegertracing/all-in-one:latest
    ports:
      - "16686:16686"
      - "14268:14268"
```

### Kubernetes Deployment

```yaml
# k8s-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: infrabot
  namespace: container-kit
spec:
  replicas: 3
  selector:
    matchLabels:
      app: infrabot
  template:
    metadata:
      labels:
        app: infrabot
    spec:
      containers:
      - name: infrabot
        image: container-kit/infrabot:latest
        ports:
        - containerPort: 8080
        - containerPort: 9090
        env:
        - name: INFRABOT_CONFIG_PATH
          value: "/etc/infrabot/config.yaml"
        volumeMounts:
        - name: config
          mountPath: /etc/infrabot
        - name: docker-sock
          mountPath: /var/run/docker.sock
        resources:
          requests:
            memory: "256Mi"
            cpu: "100m"
          limits:
            memory: "1Gi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /api/v1/health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /api/v1/health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
      volumes:
      - name: config
        configMap:
          name: infrabot-config
      - name: docker-sock
        hostPath:
          path: /var/run/docker.sock
---
apiVersion: v1
kind: Service
metadata:
  name: infrabot-service
  namespace: container-kit
spec:
  selector:
    app: infrabot
  ports:
  - name: http
    port: 8080
    targetPort: 8080
  - name: metrics
    port: 9090
    targetPort: 9090
  type: ClusterIP
```

## Troubleshooting

### Common Issues

#### Docker Operations Failing

**Symptoms**: Docker pull/push operations timeout or fail
**Causes**:
- Docker daemon not accessible
- Registry authentication issues
- Network connectivity problems
- Insufficient disk space

**Solutions**:
1. Verify Docker daemon status: `docker ps`
2. Check registry credentials in configuration
3. Test network connectivity to registry
4. Monitor disk usage and clean up if needed

```bash
# Check Docker daemon
systemctl status docker

# Test registry connectivity
docker pull alpine:latest

# Check disk usage
df -h
```

#### Session Tracking Issues

**Symptoms**: Sessions not being tracked or appearing in wrong state
**Causes**:
- Database connectivity issues
- Session timeout misconfiguration
- Memory pressure causing session cleanup

**Solutions**:
1. Check database connection health
2. Review session timeout configuration
3. Monitor memory usage and adjust limits

```bash
# Check session status
curl http://localhost:8080/api/v1/sessions

# Monitor memory usage
free -h
```

#### Performance Degradation

**Symptoms**: Operations slower than performance targets
**Causes**:
- High system load
- Resource contention
- Network latency
- Insufficient system resources

**Solutions**:
1. Monitor system metrics
2. Scale resources as needed
3. Optimize configuration parameters

```bash
# Check system load
top
htop

# Monitor network latency
ping registry.example.com

# Check InfraBot metrics
curl http://localhost:9090/metrics
```

### Debugging Guide

#### Enable Debug Logging
```yaml
# config.yaml
logging:
  level: debug
  format: json
  enable_source: true
```

#### Collect Diagnostic Information
```bash
# System information
./infrabot --version
docker version
kubectl version

# Configuration
./infrabot config validate
./infrabot config show

# Health status
curl http://localhost:8080/api/v1/health
curl http://localhost:8080/api/v1/health/components

# Metrics
curl http://localhost:9090/metrics | grep infrabot

# Traces (if Jaeger is enabled)
curl http://jaeger:16686/api/traces?service=infrabot
```

#### Performance Analysis
```bash
# CPU profiling
go tool pprof http://localhost:8080/debug/pprof/profile

# Memory profiling
go tool pprof http://localhost:8080/debug/pprof/heap

# Goroutine analysis
go tool pprof http://localhost:8080/debug/pprof/goroutine
```

### Support & Contributing

#### Getting Help
- GitHub Issues: [container-kit/issues](https://github.com/Azure/container-kit/issues)
- Documentation: [container-kit/docs](https://github.com/Azure/container-kit/docs)
- Team Contact: infrabot-team@example.com

#### Contributing
1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass
5. Submit a pull request

#### Development Setup
```bash
# Clone repository
git clone https://github.com/Azure/container-kit.git
cd container-kit

# Install dependencies
go mod download

# Run tests
make test-mcp

# Run integration tests
make test-integration

# Start development server
go run cmd/infrabot/main.go --config config/dev-config.yaml
```

---

This documentation is maintained by the InfraBot team and is updated with each release. For the latest information, please refer to the [Container Kit documentation repository](https://github.com/Azure/container-kit/docs).