# InfraBot API Reference

## Overview

This document provides comprehensive API reference documentation for InfraBot's core interfaces, including Docker operations, session management, atomic tool framework, and monitoring endpoints.

## Table of Contents

1. [REST API Endpoints](#rest-api-endpoints)
2. [Go Package APIs](#go-package-apis)
3. [MCP Protocol Interfaces](#mcp-protocol-interfaces)
4. [Configuration APIs](#configuration-apis)
5. [Monitoring & Metrics APIs](#monitoring--metrics-apis)
6. [Error Codes & Responses](#error-codes--responses)

## REST API Endpoints

### Session Management API

#### Create Session
```http
POST /api/v1/sessions
Content-Type: application/json

{
  "team": "string",
  "description": "string",
  "tags": ["string"],
  "config": {
    "timeout": "duration",
    "max_operations": "integer",
    "resource_limits": {
      "memory": "string",
      "cpu": "string"
    }
  }
}
```

**Response:**
```json
{
  "session_id": "uuid",
  "team": "string",
  "status": "CREATED",
  "created_at": "timestamp",
  "config": {},
  "resource_usage": {
    "memory_used": 0,
    "cpu_used": 0.0
  }
}
```

#### Get Session Details
```http
GET /api/v1/sessions/{session_id}
```

**Response:**
```json
{
  "session_id": "uuid",
  "team": "string",
  "status": "ACTIVE",
  "created_at": "timestamp",
  "updated_at": "timestamp",
  "operations": [
    {
      "operation_id": "uuid",
      "type": "docker_pull",
      "status": "COMPLETED",
      "duration": "duration",
      "metadata": {}
    }
  ],
  "errors": [
    {
      "error_id": "uuid",
      "message": "string",
      "type": "string",
      "timestamp": "timestamp",
      "context": {}
    }
  ],
  "performance": {
    "total_duration": "duration",
    "operation_count": 5,
    "error_rate": 0.02
  },
  "resource_usage": {
    "memory_used": 1048576,
    "cpu_used": 0.15,
    "disk_used": 5242880
  }
}
```

#### Update Session Status
```http
PUT /api/v1/sessions/{session_id}/status
Content-Type: application/json

{
  "status": "PAUSED|ACTIVE|COMPLETED|FAILED",
  "reason": "string"
}
```

#### List Sessions
```http
GET /api/v1/sessions?team={team}&status={status}&limit={limit}&offset={offset}
```

**Response:**
```json
{
  "sessions": [
    {
      "session_id": "uuid",
      "team": "string",
      "status": "string",
      "created_at": "timestamp",
      "operation_count": 10,
      "error_count": 1
    }
  ],
  "total": 100,
  "limit": 20,
  "offset": 0
}
```

#### Delete Session
```http
DELETE /api/v1/sessions/{session_id}
```

### Docker Operations API

#### Pull Docker Image
```http
POST /api/v1/docker/pull
Content-Type: application/json

{
  "session_id": "uuid",
  "image_ref": "nginx:latest",
  "registry_auth": {
    "username": "string",
    "password": "string",
    "registry": "string"
  },
  "pull_policy": "Always|IfNotPresent|Never",
  "platform": "linux/amd64",
  "progress_callback": "webhook_url"
}
```

**Response:**
```json
{
  "operation_id": "uuid",
  "session_id": "uuid",
  "status": "STARTED",
  "image_ref": "nginx:latest",
  "progress": {
    "percentage": 0.0,
    "message": "Starting pull operation",
    "bytes_downloaded": 0,
    "total_bytes": 0
  }
}
```

#### Push Docker Image
```http
POST /api/v1/docker/push
Content-Type: application/json

{
  "session_id": "uuid",
  "image_ref": "myregistry.com/app:v1.0",
  "registry_auth": {
    "username": "string",
    "password": "string",
    "registry": "myregistry.com"
  },
  "push_options": {
    "all_tags": false,
    "compress": true
  },
  "progress_callback": "webhook_url"
}
```

#### Tag Docker Image
```http
POST /api/v1/docker/tag
Content-Type: application/json

{
  "session_id": "uuid",
  "source_ref": "nginx:latest",
  "target_ref": "myregistry.com/nginx:prod",
  "force": false
}
```

#### Get Operation Status
```http
GET /api/v1/docker/operations/{operation_id}
```

**Response:**
```json
{
  "operation_id": "uuid",
  "session_id": "uuid",
  "type": "pull",
  "status": "COMPLETED",
  "image_ref": "nginx:latest",
  "started_at": "timestamp",
  "completed_at": "timestamp",
  "duration": "duration",
  "progress": {
    "percentage": 100.0,
    "message": "Pull completed successfully",
    "bytes_downloaded": 142857216,
    "total_bytes": 142857216
  },
  "result": {
    "image_id": "sha256:abcd1234...",
    "image_size": 142857216,
    "layers": [
      {
        "digest": "sha256:layer1...",
        "size": 71428608
      }
    ]
  }
}
```

### Health & Status API

#### System Health Check
```http
GET /api/v1/health
```

**Response:**
```json
{
  "status": "HEALTHY",
  "timestamp": "timestamp",
  "uptime": "duration",
  "version": "1.0.0",
  "components": {
    "session_manager": "HEALTHY",
    "docker_operations": "HEALTHY",
    "atomic_framework": "HEALTHY",
    "monitoring": "HEALTHY"
  },
  "dependencies": {
    "docker_daemon": "HEALTHY",
    "database": "HEALTHY"
  }
}
```

#### Detailed Component Health
```http
GET /api/v1/health/components
```

**Response:**
```json
{
  "components": {
    "session_manager": {
      "status": "HEALTHY",
      "last_check": "timestamp",
      "details": {
        "active_sessions": 25,
        "total_sessions": 1000,
        "error_rate": 0.01
      }
    },
    "docker_operations": {
      "status": "HEALTHY",
      "last_check": "timestamp",
      "details": {
        "daemon_version": "20.10.24",
        "active_operations": 5,
        "cache_hit_rate": 0.85
      }
    }
  }
}
```

### Metrics API

#### Prometheus Metrics
```http
GET /api/v1/metrics
```

Returns Prometheus-formatted metrics for all InfraBot components.

#### Performance Metrics
```http
GET /api/v1/metrics/performance
```

**Response:**
```json
{
  "latency": {
    "p50": "50µs",
    "p90": "150µs",
    "p95": "250µs",
    "p99": "800µs"
  },
  "throughput": {
    "requests_per_second": 1500.5,
    "operations_per_second": 1200.3
  },
  "resource_usage": {
    "memory_usage_bytes": 536870912,
    "cpu_usage_percent": 12.5,
    "goroutines": 150
  },
  "error_rates": {
    "session_errors": 0.001,
    "docker_errors": 0.005,
    "total_error_rate": 0.003
  }
}
```

## Go Package APIs

### Docker Operations Package

#### Operations Interface
```go
package pipeline

type Operations interface {
    // PullDockerImage pulls a container image from a registry
    PullDockerImage(sessionID, imageRef string) error
    
    // PushDockerImage pushes a container image to a registry
    PushDockerImage(sessionID, imageRef string) error
    
    // TagDockerImage tags a container image with a new reference
    TagDockerImage(sessionID, sourceRef, targetRef string) error
    
    // GetOperationStatus returns the status of an ongoing operation
    GetOperationStatus(operationID string) (*OperationStatus, error)
    
    // CancelOperation cancels an ongoing operation
    CancelOperation(operationID string) error
}

type OperationStatus struct {
    OperationID   string                 `json:"operation_id"`
    SessionID     string                 `json:"session_id"`
    Type          OperationType          `json:"type"`
    Status        OperationState         `json:"status"`
    Progress      *OperationProgress     `json:"progress,omitempty"`
    Result        interface{}            `json:"result,omitempty"`
    Error         error                  `json:"error,omitempty"`
    StartedAt     time.Time              `json:"started_at"`
    CompletedAt   *time.Time             `json:"completed_at,omitempty"`
    Duration      time.Duration          `json:"duration"`
    Metadata      map[string]interface{} `json:"metadata"`
}

type OperationProgress struct {
    Percentage       float64   `json:"percentage"`
    Message          string    `json:"message"`
    BytesDownloaded  int64     `json:"bytes_downloaded"`
    BytesUploaded    int64     `json:"bytes_uploaded"`
    TotalBytes       int64     `json:"total_bytes"`
    CurrentLayer     string    `json:"current_layer,omitempty"`
    CompletedLayers  int       `json:"completed_layers"`
    TotalLayers      int       `json:"total_layers"`
    EstimatedTimeRemaining time.Duration `json:"estimated_time_remaining"`
}
```

#### Usage Example
```go
package main

import (
    "context"
    "fmt"
    "github.com/Azure/container-kit/pkg/mcp/internal/pipeline"
    "github.com/rs/zerolog/log"
)

func main() {
    // Initialize operations
    config := pipeline.OperationsConfig{
        DockerHost:          "unix:///var/run/docker.sock",
        RegistryTimeout:     300 * time.Second,
        MaxConcurrentOps:    10,
        EnableProgressTracker: true,
    }
    
    ops := pipeline.NewOperations(config, log.Logger)
    
    // Pull an image
    sessionID := "session-123"
    imageRef := "nginx:latest"
    
    err := ops.PullDockerImage(sessionID, imageRef)
    if err != nil {
        log.Error().Err(err).Msg("Failed to pull image")
        return
    }
    
    log.Info().Str("image", imageRef).Msg("Image pulled successfully")
}
```

### Session Management Package

#### SessionManager Interface
```go
package session

type SessionManager interface {
    // CreateSession creates a new session
    CreateSession(team string, config SessionConfig) (*Session, error)
    
    // GetSession retrieves session details
    GetSession(sessionID string) (*Session, error)
    
    // UpdateSessionStatus updates session status
    UpdateSessionStatus(sessionID string, status SessionStatus) error
    
    // DeleteSession deletes a session and cleans up resources
    DeleteSession(sessionID string) error
    
    // TrackOperation tracks an operation within a session
    TrackOperation(sessionID string, operation Operation) error
    
    // TrackError tracks an error within a session
    TrackError(sessionID string, err error, context map[string]interface{}) error
    
    // GetSessionMetrics returns performance metrics for a session
    GetSessionMetrics(sessionID string) (*SessionMetrics, error)
}

type Session struct {
    ID              string                 `json:"id"`
    Team            string                 `json:"team"`
    Status          SessionStatus          `json:"status"`
    CreatedAt       time.Time              `json:"created_at"`
    UpdatedAt       time.Time              `json:"updated_at"`
    CompletedAt     *time.Time             `json:"completed_at,omitempty"`
    Config          SessionConfig          `json:"config"`
    Operations      []Operation            `json:"operations"`
    Errors          []SessionError         `json:"errors"`
    ResourceUsage   ResourceUsage          `json:"resource_usage"`
    Metadata        map[string]interface{} `json:"metadata"`
}

type SessionConfig struct {
    Timeout          time.Duration          `json:"timeout"`
    MaxOperations    int                    `json:"max_operations"`
    EnableTracking   bool                   `json:"enable_tracking"`
    ResourceLimits   ResourceLimits         `json:"resource_limits"`
    Tags             []string               `json:"tags"`
    Metadata         map[string]interface{} `json:"metadata"`
}

type Operation struct {
    ID          string                 `json:"id"`
    SessionID   string                 `json:"session_id"`
    Type        string                 `json:"type"`
    Status      OperationStatus        `json:"status"`
    StartedAt   time.Time              `json:"started_at"`
    CompletedAt *time.Time             `json:"completed_at,omitempty"`
    Duration    time.Duration          `json:"duration"`
    Metadata    map[string]interface{} `json:"metadata"`
    Result      interface{}            `json:"result,omitempty"`
    Error       *SessionError          `json:"error,omitempty"`
}
```

#### Usage Example
```go
package main

import (
    "github.com/Azure/container-kit/pkg/mcp/internal/session"
    "time"
)

func main() {
    // Initialize session manager
    config := session.ManagerConfig{
        DatabaseURL:      "bolt://sessions.db",
        CleanupInterval:  5 * time.Minute,
        MaxActiveSessions: 1000,
    }
    
    mgr := session.NewSessionManager(config, log.Logger)
    
    // Create a session
    sessionConfig := session.SessionConfig{
        Timeout:        30 * time.Minute,
        MaxOperations:  100,
        EnableTracking: true,
        Tags:          []string{"integration-test", "docker-ops"},
    }
    
    sess, err := mgr.CreateSession("team-alpha", sessionConfig)
    if err != nil {
        log.Fatal().Err(err).Msg("Failed to create session")
    }
    
    // Track an operation
    operation := session.Operation{
        Type: "docker_pull",
        Metadata: map[string]interface{}{
            "image": "nginx:latest",
            "registry": "docker.io",
        },
    }
    
    err = mgr.TrackOperation(sess.ID, operation)
    if err != nil {
        log.Error().Err(err).Msg("Failed to track operation")
    }
}
```

### Atomic Tool Framework Package

#### AtomicToolBase Interface
```go
package runtime

type AtomicToolBase interface {
    // ExecuteWithoutProgress executes an operation without progress tracking
    ExecuteWithoutProgress(ctx context.Context, operation func() error) error
    
    // ExecuteWithProgress executes an operation with progress tracking
    ExecuteWithProgress(ctx context.Context, operation func(ProgressCallback) error) error
    
    // GetPerformanceMetrics returns performance metrics for the tool
    GetPerformanceMetrics() *PerformanceMetrics
    
    // SetResourceLimits configures resource limits for the tool
    SetResourceLimits(limits ResourceLimits) error
    
    // Cleanup cleans up tool resources
    Cleanup(ctx context.Context) error
}

type ProgressCallback func(progress float64, message string)

type PerformanceMetrics struct {
    ExecutionCount     int64                  `json:"execution_count"`
    TotalDuration      time.Duration          `json:"total_duration"`
    AverageDuration    time.Duration          `json:"average_duration"`
    SuccessRate        float64                `json:"success_rate"`
    ErrorRate          float64                `json:"error_rate"`
    ResourceUsage      ResourceUsageMetrics   `json:"resource_usage"`
    LastExecution      time.Time              `json:"last_execution"`
    Throughput         float64                `json:"throughput"`
}

type ResourceLimits struct {
    MaxMemory    int64         `json:"max_memory"`
    MaxCPU       float64       `json:"max_cpu"`
    MaxDuration  time.Duration `json:"max_duration"`
    MaxGoroutines int          `json:"max_goroutines"`
}
```

#### Usage Example
```go
package main

import (
    "context"
    "fmt"
    "github.com/Azure/container-kit/pkg/mcp/internal/runtime"
    "time"
)

func main() {
    // Initialize atomic tool
    config := runtime.AtomicToolConfig{
        EnableProgressTracking: true,
        EnableResourceMonitoring: true,
        PerformanceTargets: runtime.PerformanceTargets{
            MaxLatency: 300 * time.Microsecond,
            MinThroughput: 1000.0,
        },
    }
    
    tool := runtime.NewAtomicTool(config, log.Logger)
    
    // Set resource limits
    limits := runtime.ResourceLimits{
        MaxMemory:    100 * 1024 * 1024, // 100MB
        MaxCPU:       0.5,               // 50% CPU
        MaxDuration:  5 * time.Minute,   // 5 minutes
    }
    
    err := tool.SetResourceLimits(limits)
    if err != nil {
        log.Fatal().Err(err).Msg("Failed to set resource limits")
    }
    
    // Execute with progress tracking
    ctx := context.Background()
    err = tool.ExecuteWithProgress(ctx, func(progress runtime.ProgressCallback) error {
        progress(0.0, "Starting atomic operation")
        
        // Simulate work
        for i := 0; i <= 100; i += 10 {
            time.Sleep(100 * time.Millisecond)
            progress(float64(i)/100.0, fmt.Sprintf("Processing %d%%", i))
        }
        
        progress(1.0, "Atomic operation completed")
        return nil
    })
    
    if err != nil {
        log.Error().Err(err).Msg("Atomic operation failed")
        return
    }
    
    // Get performance metrics
    metrics := tool.GetPerformanceMetrics()
    log.Info().
        Int64("executions", metrics.ExecutionCount).
        Dur("avg_duration", metrics.AverageDuration).
        Float64("success_rate", metrics.SuccessRate).
        Msg("Atomic tool performance")
}
```

## MCP Protocol Interfaces

### Tool Registration
```go
package mcp

type Tool interface {
    // GetName returns the tool name
    GetName() string
    
    // GetDescription returns the tool description
    GetDescription() string
    
    // GetInputSchema returns the JSON schema for tool inputs
    GetInputSchema() map[string]interface{}
    
    // Execute executes the tool with given inputs
    Execute(ctx context.Context, inputs map[string]interface{}) (*ToolResult, error)
}

type ToolResult struct {
    Success  bool                   `json:"success"`
    Result   interface{}            `json:"result,omitempty"`
    Error    string                 `json:"error,omitempty"`
    Metadata map[string]interface{} `json:"metadata,omitempty"`
}
```

### Docker Tools Registration
```go
// Docker Pull Tool
type DockerPullTool struct {
    operations pipeline.Operations
}

func (t *DockerPullTool) GetName() string {
    return "docker_pull"
}

func (t *DockerPullTool) GetDescription() string {
    return "Pull a Docker image from a registry"
}

func (t *DockerPullTool) GetInputSchema() map[string]interface{} {
    return map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "session_id": map[string]interface{}{
                "type": "string",
                "description": "Session ID for tracking",
            },
            "image_ref": map[string]interface{}{
                "type": "string",
                "description": "Docker image reference (e.g., nginx:latest)",
            },
            "registry_auth": map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "username": map[string]interface{}{"type": "string"},
                    "password": map[string]interface{}{"type": "string"},
                    "registry": map[string]interface{}{"type": "string"},
                },
            },
        },
        "required": []string{"session_id", "image_ref"},
    }
}

func (t *DockerPullTool) Execute(ctx context.Context, inputs map[string]interface{}) (*ToolResult, error) {
    sessionID := inputs["session_id"].(string)
    imageRef := inputs["image_ref"].(string)
    
    err := t.operations.PullDockerImage(sessionID, imageRef)
    if err != nil {
        return &ToolResult{
            Success: false,
            Error:   err.Error(),
        }, nil
    }
    
    return &ToolResult{
        Success: true,
        Result: map[string]interface{}{
            "image_ref": imageRef,
            "status": "pulled",
        },
    }, nil
}
```

## Configuration APIs

### Configuration Structure
```go
type InfraBotConfig struct {
    // Server configuration
    Server ServerConfig `yaml:"server"`
    
    // Docker configuration
    Docker DockerConfig `yaml:"docker"`
    
    // Session management configuration
    Session SessionConfig `yaml:"session"`
    
    // Monitoring configuration
    Monitoring MonitoringConfig `yaml:"monitoring"`
    
    // Performance configuration
    Performance PerformanceConfig `yaml:"performance"`
    
    // Logging configuration
    Logging LoggingConfig `yaml:"logging"`
}

type ServerConfig struct {
    Port           int           `yaml:"port"`
    Host           string        `yaml:"host"`
    Timeout        time.Duration `yaml:"timeout"`
    MaxConnections int           `yaml:"max_connections"`
    TLSConfig      *TLSConfig    `yaml:"tls,omitempty"`
}

type DockerConfig struct {
    DaemonHost         string        `yaml:"daemon_host"`
    RegistryTimeout    time.Duration `yaml:"registry_timeout"`
    MaxConcurrentOps   int           `yaml:"max_concurrent_ops"`
    DefaultRegistry    string        `yaml:"default_registry"`
    RegistryAuth       map[string]RegistryAuth `yaml:"registry_auth"`
    CacheEnabled       bool          `yaml:"cache_enabled"`
    CacheSize          int           `yaml:"cache_size"`
}

type MonitoringConfig struct {
    EnableMetrics    bool          `yaml:"enable_metrics"`
    EnableTracing    bool          `yaml:"enable_tracing"`
    MetricsPort      int           `yaml:"metrics_port"`
    JaegerEndpoint   string        `yaml:"jaeger_endpoint"`
    PrometheusConfig PrometheusConfig `yaml:"prometheus"`
}
```

### Configuration Validation
```go
func (c *InfraBotConfig) Validate() error {
    if c.Server.Port <= 0 || c.Server.Port > 65535 {
        return fmt.Errorf("invalid server port: %d", c.Server.Port)
    }
    
    if c.Docker.DaemonHost == "" {
        return fmt.Errorf("docker daemon host is required")
    }
    
    if c.Session.MaxActiveSessions <= 0 {
        return fmt.Errorf("max active sessions must be positive")
    }
    
    return nil
}

func LoadConfig(path string) (*InfraBotConfig, error) {
    data, err := ioutil.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("failed to read config file: %w", err)
    }
    
    var config InfraBotConfig
    err = yaml.Unmarshal(data, &config)
    if err != nil {
        return nil, fmt.Errorf("failed to parse config: %w", err)
    }
    
    err = config.Validate()
    if err != nil {
        return nil, fmt.Errorf("config validation failed: %w", err)
    }
    
    return &config, nil
}
```

## Error Codes & Responses

### HTTP Status Codes

| Code | Description | Usage |
|------|-------------|-------|
| 200  | OK | Successful operation |
| 201  | Created | Session/resource created |
| 202  | Accepted | Operation started (async) |
| 400  | Bad Request | Invalid input parameters |
| 401  | Unauthorized | Authentication required |
| 403  | Forbidden | Insufficient permissions |
| 404  | Not Found | Resource not found |
| 409  | Conflict | Resource already exists |
| 422  | Unprocessable Entity | Validation failed |
| 429  | Too Many Requests | Rate limit exceeded |
| 500  | Internal Server Error | Server error |
| 502  | Bad Gateway | Upstream service error |
| 503  | Service Unavailable | Service temporarily unavailable |

### Error Response Format
```json
{
  "error": {
    "code": "DOCKER_PULL_FAILED",
    "message": "Failed to pull Docker image",
    "details": {
      "image_ref": "nginx:latest",
      "registry": "docker.io",
      "cause": "authentication failed"
    },
    "timestamp": "2024-01-15T10:30:00Z",
    "request_id": "req-12345",
    "documentation_url": "https://docs.example.com/errors/DOCKER_PULL_FAILED"
  }
}
```

### Error Codes

#### Session Management Errors
- `SESSION_NOT_FOUND`: Session ID does not exist
- `SESSION_CREATION_FAILED`: Failed to create new session
- `SESSION_LIMIT_EXCEEDED`: Maximum active sessions reached
- `SESSION_TIMEOUT`: Session exceeded timeout limit
- `SESSION_INVALID_STATUS`: Invalid status transition

#### Docker Operation Errors
- `DOCKER_DAEMON_UNAVAILABLE`: Docker daemon not accessible
- `DOCKER_PULL_FAILED`: Image pull operation failed
- `DOCKER_PUSH_FAILED`: Image push operation failed
- `DOCKER_TAG_FAILED`: Image tag operation failed
- `DOCKER_AUTH_FAILED`: Registry authentication failed
- `DOCKER_IMAGE_NOT_FOUND`: Specified image not found
- `DOCKER_REGISTRY_UNAVAILABLE`: Registry not accessible

#### Resource Errors
- `RESOURCE_LIMIT_EXCEEDED`: Operation exceeded resource limits
- `MEMORY_LIMIT_EXCEEDED`: Memory usage exceeded limit
- `CPU_LIMIT_EXCEEDED`: CPU usage exceeded limit
- `DISK_SPACE_INSUFFICIENT`: Insufficient disk space

#### Configuration Errors
- `CONFIG_INVALID`: Configuration validation failed
- `CONFIG_MISSING`: Required configuration missing
- `CONFIG_PARSE_ERROR`: Configuration file parse error

### Error Handling Best Practices

#### Client Error Handling
```go
type APIError struct {
    Code           string                 `json:"code"`
    Message        string                 `json:"message"`
    Details        map[string]interface{} `json:"details"`
    Timestamp      time.Time              `json:"timestamp"`
    RequestID      string                 `json:"request_id"`
    DocumentationURL string               `json:"documentation_url"`
}

func handleAPIError(resp *http.Response) error {
    if resp.StatusCode >= 200 && resp.StatusCode < 300 {
        return nil
    }
    
    var apiError APIError
    err := json.NewDecoder(resp.Body).Decode(&apiError)
    if err != nil {
        return fmt.Errorf("API error (status %d): %s", resp.StatusCode, resp.Status)
    }
    
    return fmt.Errorf("API error %s: %s", apiError.Code, apiError.Message)
}
```

#### Retry Logic
```go
func withRetry(operation func() error, maxRetries int) error {
    var lastErr error
    
    for attempt := 0; attempt <= maxRetries; attempt++ {
        err := operation()
        if err == nil {
            return nil
        }
        
        lastErr = err
        
        // Check if error is retryable
        if !isRetryableError(err) {
            return err
        }
        
        // Exponential backoff
        if attempt < maxRetries {
            delay := time.Duration(1<<uint(attempt)) * time.Second
            time.Sleep(delay)
        }
    }
    
    return lastErr
}

func isRetryableError(err error) bool {
    // Check for retryable error conditions
    switch {
    case strings.Contains(err.Error(), "DOCKER_DAEMON_UNAVAILABLE"):
        return true
    case strings.Contains(err.Error(), "DOCKER_REGISTRY_UNAVAILABLE"):
        return true
    case strings.Contains(err.Error(), "RESOURCE_TEMPORARILY_UNAVAILABLE"):
        return true
    default:
        return false
    }
}
```

---

This API reference provides comprehensive documentation for all InfraBot interfaces. For implementation examples and detailed guides, refer to the main [InfraBot documentation](README.md).