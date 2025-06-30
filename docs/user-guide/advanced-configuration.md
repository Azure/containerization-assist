# Advanced Configuration Guide

This guide covers advanced configuration options and patterns for Container Kit.

## Advanced Sandbox Options

### Custom Security Policies

```go
// Define custom security policy
policy := utils.SecurityPolicy{
    AllowNetworking:     false,
    AllowFileSystem:     true,
    RequireNonRoot:      true,
    TrustedRegistries:   []string{"docker.io", "gcr.io", "your-registry.com"},
    MaxExecutionTime:    time.Minute * 5,
    AllowedCommands:     []string{"echo", "ls", "cat", "grep"},
    BlockedCommands:     []string{"rm", "sudo", "su", "chmod"},
    EnforceResourceLimits: true,
}

options := utils.SandboxOptions{
    SecurityPolicy: policy,
    // ... other options
}
```

### Production Configuration

```go
// Production-ready configuration
options := utils.SandboxOptions{
    BaseImage:         "alpine:3.18",
    MemoryLimit:       512 * 1024 * 1024, // 512MB
    CPUQuota:          100000,             // 100% of one core
    Timeout:           time.Minute * 10,
    ReadOnly:          true,
    NetworkAccess:     false,
    User:              "1000",
    Group:             "1000",
    Capabilities:      []string{},
    
    // Production features
    EnableMetrics:     true,
    EnableAudit:       true,
    EnableProfiling:   true,
    EnableTracing:     true,
    
    // Security enhancements
    CustomSeccomp:     "/path/to/seccomp.json",
    CustomAppArmor:    "container-default",
    SELinuxLabel:      "system_u:system_r:container_t:s0",
    
    // Monitoring
    MetricsInterval:   time.Second * 30,
    HealthCheckInterval: time.Second * 10,
    
    // Cleanup
    AutoCleanup:       true,
    CleanupTimeout:    time.Minute * 2,
}
```

## Workspace Management

### Multi-tenant Configuration

```go
// Configure for multi-tenant usage
config := utils.WorkspaceConfig{
    BaseDir:           "/var/lib/container-kit/workspaces",
    MaxSizePerSession: 1024 * 1024 * 1024, // 1GB per session
    TotalMaxSize:      10 * 1024 * 1024 * 1024, // 10GB total
    Cleanup:           true,
    SandboxEnabled:    true,
    
    // Multi-tenant settings
    SessionTimeout:    time.Hour * 2,
    MaxConcurrentSessions: 50,
    IsolationLevel:    "strict",
    
    Logger: logger.With().Str("component", "workspace").Logger(),
}
```

### Custom Resource Quotas

```go
// Define per-user quotas
quotas := map[string]utils.ResourceQuota{
    "user1": {
        MaxSessions:       5,
        MaxDiskUsage:      2 * 1024 * 1024 * 1024, // 2GB
        MaxMemoryPerSession: 1024 * 1024 * 1024,   // 1GB
        MaxCPUQuota:       200000,                  // 200% CPU
    },
    "user2": {
        MaxSessions:       10,
        MaxDiskUsage:      5 * 1024 * 1024 * 1024, // 5GB
        MaxMemoryPerSession: 2 * 1024 * 1024 * 1024, // 2GB
        MaxCPUQuota:       400000,                   // 400% CPU
    },
}

workspace.SetUserQuotas(quotas)
```

## Security Validation Configuration

### Custom Threat Model

```go
// Define custom threats
customThreats := map[string]utils.ThreatInfo{
    "T100": {
        ID:          "T100",
        Name:        "Custom Application Threat",
        Description: "Application-specific security concern",
        Impact:      "HIGH",
        Probability: "MEDIUM",
        Category:    "APPLICATION",
        Mitigations: []string{"C100", "C101"},
    },
}

// Define custom controls
customControls := map[string]utils.ControlInfo{
    "C100": {
        ID:            "C100",
        Name:          "Application Input Validation",
        Description:   "Validate all application inputs",
        Type:          "PREVENTIVE",
        Effectiveness: "HIGH",
        Threats:       []string{"T100"},
        Implemented:   true,
    },
}

// Create validator with custom model
validator := utils.NewSecurityValidatorWithCustomModel(logger, customThreats, customControls)
```

### Compliance Configuration

```go
// Configure compliance standards
compliance := utils.ComplianceConfig{
    Standards: []string{"CIS-DOCKER", "NIST-800-190", "PCI-DSS"},
    EnforceCompliance: true,
    AuditLevel: "DETAILED",
    ReportFormat: "JSON",
}

validator.SetComplianceConfig(compliance)
```

## Monitoring and Observability

### Metrics Configuration

```go
// Configure detailed metrics
metricsConfig := utils.MetricsConfig{
    Enabled:           true,
    CollectionInterval: time.Second * 10,
    RetentionPeriod:   time.Hour * 24,
    ExportFormats:     []string{"prometheus", "json"},
    
    // Metrics to collect
    CollectCPU:        true,
    CollectMemory:     true,
    CollectNetwork:    true,
    CollectDisk:       true,
    CollectSecurity:   true,
    
    // Export configuration
    PrometheusEndpoint: "http://prometheus:9090",
    MetricsNamespace:   "container_kit",
}

collector := utils.NewMetricsCollectorWithConfig(metricsConfig)
```

### Audit Configuration

```go
// Configure audit logging
auditConfig := utils.AuditConfig{
    Enabled:        true,
    LogLevel:       "INFO",
    LogDestination: "/var/log/container-kit/audit.log",
    RotateSize:     100 * 1024 * 1024, // 100MB
    MaxFiles:       10,
    
    // Events to audit
    AuditExecution:     true,
    AuditSecurity:      true,
    AuditResourceUsage: true,
    AuditAccess:        true,
    
    // Format
    Format: "JSON",
    IncludeStackTrace: false,
}

auditor := utils.NewAuditLoggerWithConfig(auditConfig)
```

## Performance Optimization

### Caching Configuration

```go
// Configure image and layer caching
cacheConfig := utils.CacheConfig{
    Enabled:       true,
    CacheDir:      "/var/cache/container-kit",
    MaxCacheSize:  5 * 1024 * 1024 * 1024, // 5GB
    TTL:           time.Hour * 24,
    
    // Cache strategies
    ImageCaching:  true,
    LayerCaching:  true,
    ResultCaching: true,
    
    // Cleanup
    AutoCleanup:   true,
    CleanupInterval: time.Hour * 6,
}

executor.SetCacheConfig(cacheConfig)
```

### Parallel Execution

```go
// Configure parallel execution limits
parallelConfig := utils.ParallelConfig{
    MaxConcurrentExecutions: 10,
    QueueSize:              100,
    TimeoutStrategy:        "FIFO",
    LoadBalancing:          "ROUND_ROBIN",
}

executor.SetParallelConfig(parallelConfig)
```

## Integration Patterns

### Middleware Integration

```go
// Custom middleware for request processing
type SecurityMiddleware struct {
    validator *utils.SecurityValidator
}

func (m *SecurityMiddleware) Process(ctx context.Context, req *Request) (*Response, error) {
    // Pre-execution validation
    if err := m.validateRequest(req); err != nil {
        return nil, err
    }
    
    // Execute with monitoring
    response, err := m.executeWithMonitoring(ctx, req)
    
    // Post-execution audit
    m.auditExecution(req, response, err)
    
    return response, err
}
```

### Event-driven Architecture

```go
// Event handler for security events
type SecurityEventHandler struct {
    alertManager *AlertManager
}

func (h *SecurityEventHandler) HandleSecurityEvent(event utils.SecurityEvent) {
    switch event.Severity {
    case "CRITICAL":
        h.alertManager.SendImmediateAlert(event)
    case "HIGH":
        h.alertManager.SendAlert(event)
    default:
        h.alertManager.LogEvent(event)
    }
}

// Register event handler
validator.RegisterEventHandler(handler)
```

## Environment-specific Configurations

### Development Environment

```go
// Development configuration - more permissive
devConfig := utils.SandboxOptions{
    BaseImage:         "ubuntu:22.04",
    MemoryLimit:       1024 * 1024 * 1024, // 1GB
    CPUQuota:          200000,              // 200% CPU
    Timeout:           time.Minute * 30,
    ReadOnly:          false,               // Allow writes for development
    NetworkAccess:     true,                // Enable network for package downloads
    EnableMetrics:     true,
    EnableAudit:       false,               // Disable for performance
    LogLevel:          "DEBUG",
}
```

### Production Environment

```go
// Production configuration - security-focused
prodConfig := utils.SandboxOptions{
    BaseImage:         "alpine:3.18",
    MemoryLimit:       256 * 1024 * 1024,  // 256MB
    CPUQuota:          50000,               // 50% CPU
    Timeout:           time.Minute * 5,
    ReadOnly:          true,
    NetworkAccess:     false,
    EnableMetrics:     true,
    EnableAudit:       true,
    LogLevel:          "INFO",
    
    // Production security
    EnforceSeccomp:    true,
    EnforceAppArmor:   true,
    RequireSignedImages: true,
}
```

## Troubleshooting Advanced Issues

### Performance Debugging

```go
// Enable detailed performance profiling
options.EnableProfiling = true
options.ProfileDir = "/tmp/container-kit-profiles"

// Use with pprof
go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()
```

### Security Debugging

```go
// Enable security event tracing
validator.SetLogLevel("TRACE")
validator.EnableSecurityTracing(true)

// Custom security event logger
validator.SetSecurityEventCallback(func(event utils.SecurityEvent) {
    log.Printf("Security event: %+v", event)
})
```

## Best Practices Summary

1. **Use environment-specific configurations**
2. **Enable comprehensive monitoring in production**
3. **Implement proper error handling and recovery**
4. **Use caching for performance optimization**
5. **Configure appropriate resource limits**
6. **Enable audit logging for compliance**
7. **Implement proper cleanup strategies**
8. **Use middleware patterns for cross-cutting concerns**
9. **Monitor and alert on security events**
10. **Regularly review and update security policies**