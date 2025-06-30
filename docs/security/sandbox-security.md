# Sandbox Security

## Overview

Container Kit's advanced sandboxing provides secure, isolated execution environments for containerized workloads. The sandboxing implementation leverages Docker's security features combined with additional hardening measures to create defense-in-depth protection.

## Sandboxed Execution

### Core Security Model

```
┌─────────────────────────────────────────────┐
│              Host System                     │
│  ┌───────────────────────────────────────┐  │
│  │           Docker Daemon               │  │
│  │  ┌─────────────────────────────────┐  │  │
│  │  │        Sandbox Container        │  │  │
│  │  │                                 │  │  │
│  │  │  User: 1000 (non-root)         │  │  │
│  │  │  Filesystem: read-only          │  │  │
│  │  │  Network: isolated              │  │  │
│  │  │  Capabilities: minimal          │  │  │
│  │  │  Resources: limited             │  │  │
│  │  │                                 │  │  │
│  │  └─────────────────────────────────┘  │  │
│  └───────────────────────────────────────┘  │
└─────────────────────────────────────────────┘
```

### Security Layers

#### 1. User Namespace Isolation

**Non-root Execution:**
```go
type AdvancedSandboxOptions struct {
    User  string `json:"user"`   // "1000"
    Group string `json:"group"`  // "1000"
}
```

**Security Benefits:**
- Prevents privilege escalation attacks
- Limits access to host resources
- Reduces attack surface for container escape
- Complies with CIS Docker Benchmark 4.1

#### 2. Filesystem Security

**Read-only Root Filesystem:**
```bash
docker run --read-only \
    --tmpfs /tmp:rw,noexec,nosuid,size=100m \
    --tmpfs /var/tmp:rw,noexec,nosuid,size=100m \
    alpine:latest
```

**Security Benefits:**
- Prevents malicious file modifications
- Protects against persistence mechanisms
- Reduces impact of code injection attacks
- Limits data exfiltration capabilities

#### 3. Network Isolation

**Default Network Policy:**
```go
type SecurityPolicy struct {
    AllowNetworking   bool     `json:"allow_networking"`   // false by default
    TrustedRegistries []string `json:"trusted_registries"`
}
```

**Network Security Features:**
- `--network=none` by default
- No external connectivity unless explicitly enabled
- DNS resolution disabled
- Inter-container communication blocked

#### 4. Resource Constraints

**CPU and Memory Limits:**
```bash
docker run \
    --cpus="1.0" \
    --memory="512m" \
    --memory-swap="512m" \
    --pids-limit=100 \
    alpine:latest
```

**Resource Security:**
- Prevents resource exhaustion attacks
- Limits DoS impact on host system
- Enforces fair resource sharing
- Protects against fork bombs

## Advanced Security Controls

### Linux Capabilities

**Capability Dropping:**
```go
// Default: all capabilities dropped
capabilities := []string{} // Empty = no capabilities

// Secure execution with minimal capabilities
if options.RequireNetworking {
    capabilities = []string{"CAP_NET_BIND_SERVICE"}
}
```

**Security Impact:**
- `CAP_SYS_ADMIN` - Prevents mount operations
- `CAP_NET_RAW` - Blocks raw socket access
- `CAP_SYS_PTRACE` - Prevents process debugging
- `CAP_DAC_OVERRIDE` - Blocks file permission bypasses

### Seccomp Profiles

**System Call Filtering:**
```json
{
  "defaultAction": "SCMP_ACT_ERRNO",
  "architectures": ["SCMP_ARCH_X86_64"],
  "syscalls": [
    {
      "names": ["read", "write", "open", "close"],
      "action": "SCMP_ACT_ALLOW"
    }
  ]
}
```

**Blocked System Calls:**
- `mount/umount` - Filesystem manipulation
- `reboot/shutdown` - System control
- `ptrace` - Process debugging
- `kexec_load` - Kernel loading

### AppArmor Integration

**Profile Configuration:**
```
#include <tunables/global>

/usr/bin/sandbox-executor {
  #include <abstractions/base>
  
  capability setuid,
  capability setgid,
  
  /bin/** mr,
  /usr/bin/** mr,
  /lib/** mr,
  /usr/lib/** mr,
  
  deny /proc/sys/** w,
  deny /sys/** w,
  deny @{HOME}/.ssh/** rw,
}
```

## Sandbox Executor Implementation

### Core Execution Flow

```go
func (se *SandboxExecutor) ExecuteSecure(ctx context.Context, 
    sessionID string, options AdvancedSandboxOptions) (*ExecutionResult, error) {
    
    // 1. Security validation
    report, err := se.validator.ValidateSecurity(ctx, sessionID, options)
    if err != nil {
        return nil, fmt.Errorf("security validation failed: %w", err)
    }
    
    // 2. Risk assessment
    if report.RiskLevel == "HIGH" || report.RiskLevel == "CRITICAL" {
        return nil, fmt.Errorf("execution blocked: %s risk level", report.RiskLevel)
    }
    
    // 3. Secure container configuration
    config := se.buildSecureConfig(options)
    
    // 4. Execute with monitoring
    return se.executeWithMonitoring(ctx, config)
}
```

### Security Configuration Builder

```go
func (se *SandboxExecutor) buildSecureConfig(options AdvancedSandboxOptions) *container.Config {
    return &container.Config{
        User:         "1000:1000",              // Non-root user
        WorkingDir:   "/workspace",             // Controlled workspace
        Env:          se.sanitizeEnvironment(), // Clean environment
        Cmd:          []string{"/bin/sh"},      // Minimal shell
        AttachStdout: true,
        AttachStderr: true,
        NetworkDisabled: !options.SecurityPolicy.AllowNetworking,
    }
}
```

### Host Configuration Security

```go
func (se *SandboxExecutor) buildHostConfig(options AdvancedSandboxOptions) *container.HostConfig {
    return &container.HostConfig{
        // Resource limits
        Memory:     512 * 1024 * 1024, // 512MB
        CPUQuota:   100000,            // 1 CPU
        CPUPeriod:  100000,
        PidsLimit:  &[]int64{100}[0],  // Process limit
        
        // Security options
        ReadonlyRootfs: true,
        SecurityOpt: []string{
            "no-new-privileges:true",
            "apparmor:sandbox-profile",
        },
        
        // Capability dropping
        CapDrop: []string{"ALL"},
        CapAdd:  options.Capabilities,
        
        // Network isolation
        NetworkMode: container.NetworkMode("none"),
        
        // Filesystem mounts
        Tmpfs: map[string]string{
            "/tmp":     "rw,noexec,nosuid,size=100m",
            "/var/tmp": "rw,noexec,nosuid,size=100m",
        },
    }
}
```

## Security Monitoring

### Real-time Monitoring

**Resource Usage Tracking:**
```go
type ResourceMonitor struct {
    cpuUsage    atomic.Uint64
    memoryUsage atomic.Uint64
    networkIO   atomic.Uint64
    diskIO      atomic.Uint64
}

func (rm *ResourceMonitor) Monitor(ctx context.Context, containerID string) {
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            stats, err := rm.docker.ContainerStats(ctx, containerID, false)
            if err != nil {
                continue
            }
            rm.updateMetrics(stats)
        }
    }
}
```

### Audit Logging

**Security Event Logging:**
```go
type SecurityAuditEvent struct {
    Timestamp time.Time                 `json:"timestamp"`
    SessionID string                    `json:"session_id"`
    EventType string                    `json:"event_type"`
    Severity  string                    `json:"severity"`
    Action    string                    `json:"action"`
    Details   map[string]interface{}    `json:"details"`
}

// Event types
const (
    EventExecutionStarted = "EXECUTION_STARTED"
    EventExecutionBlocked = "EXECUTION_BLOCKED"
    EventResourceLimit    = "RESOURCE_LIMIT_EXCEEDED"
    EventSecurityViolation = "SECURITY_VIOLATION"
)
```

## Container Image Security

### Base Image Hardening

**Minimal Base Images:**
- Alpine Linux (5MB) - minimal attack surface
- Distroless images - no shell, no package manager
- Scratch images - statically linked binaries only

**Image Security Scanning:**
```go
func (se *SandboxExecutor) validateImage(ctx context.Context, image string) error {
    // Check trusted registries
    if !se.isTrustedRegistry(image) {
        return fmt.Errorf("untrusted registry: %s", image)
    }
    
    // Vulnerability scanning
    vulnerabilities, err := se.scanner.ScanImage(ctx, image)
    if err != nil {
        return fmt.Errorf("image scan failed: %w", err)
    }
    
    // Risk assessment
    if se.hasHighRiskVulnerabilities(vulnerabilities) {
        return fmt.Errorf("high-risk vulnerabilities detected")
    }
    
    return nil
}
```

### Runtime Image Protection

**Image Immutability:**
- Read-only root filesystem
- No package installation at runtime
- Signed image verification
- Content trust enforcement

## Escape Prevention

### Container Escape Mitigations

**Kernel Namespace Isolation:**
- PID namespace - process isolation
- NET namespace - network isolation  
- MNT namespace - mount isolation
- UTS namespace - hostname isolation
- IPC namespace - inter-process communication isolation
- USER namespace - user ID isolation

**Privileged Operation Prevention:**
```go
// Security checks before execution
func (se *SandboxExecutor) validateExecution(options AdvancedSandboxOptions) error {
    // Check for privileged mode
    if options.Privileged {
        return fmt.Errorf("privileged execution not allowed")
    }
    
    // Check for dangerous mounts
    for _, mount := range options.Mounts {
        if se.isDangerousMount(mount) {
            return fmt.Errorf("dangerous mount detected: %s", mount)
        }
    }
    
    // Check for host network access
    if options.SecurityPolicy.AllowNetworking && !se.isNetworkingAllowed() {
        return fmt.Errorf("network access not permitted")
    }
    
    return nil
}
```

### Container Breakout Detection

**Runtime Monitoring:**
```go
type BreakoutDetector struct {
    processMonitor   *ProcessMonitor
    filesystemWatch  *FilesystemWatcher
    networkMonitor   *NetworkMonitor
}

func (bd *BreakoutDetector) MonitorContainer(ctx context.Context, containerID string) {
    // Monitor for suspicious process activity
    go bd.processMonitor.WatchProcesses(ctx, containerID)
    
    // Monitor filesystem access patterns
    go bd.filesystemWatch.WatchFileAccess(ctx, containerID)
    
    // Monitor network activity
    go bd.networkMonitor.WatchNetworkConnections(ctx, containerID)
}
```

## Performance Considerations

### Security vs Performance Trade-offs

**Optimization Strategies:**
- **Cached Validations**: Reuse security validation results for identical configurations
- **Parallel Security Checks**: Run security validations concurrently
- **Lazy Loading**: Load security profiles on-demand
- **Efficient Monitoring**: Sample-based resource monitoring

**Performance Metrics:**
- Security validation overhead: <10ms
- Container startup with security: <500ms
- Resource monitoring overhead: <1% CPU
- Audit logging latency: <5ms

### Resource Optimization

**Memory Management:**
```go
// Efficient memory allocation for security operations
type SecurityResourcePool struct {
    validatorPool sync.Pool
    reportPool    sync.Pool
    loggerPool    sync.Pool
}

func (srp *SecurityResourcePool) GetValidator() *SecurityValidator {
    if v := srp.validatorPool.Get(); v != nil {
        return v.(*SecurityValidator)
    }
    return NewSecurityValidator()
}
```

## Best Practices

### Development Guidelines

1. **Principle of Least Privilege**
   - Minimal capabilities
   - Non-root execution
   - Limited resource access

2. **Defense in Depth**
   - Multiple security layers
   - Redundant controls
   - Fail-safe defaults

3. **Security by Default**
   - Secure default configurations
   - Opt-in for elevated privileges
   - Explicit security policy requirements

### Deployment Recommendations

1. **Infrastructure Security**
   - Host OS hardening
   - Docker daemon security
   - Network segmentation
   - Log aggregation

2. **Monitoring and Alerting**
   - Real-time security event monitoring
   - Automated incident response
   - Security metrics dashboards
   - Compliance reporting

3. **Regular Security Assessment**
   - Vulnerability scanning
   - Penetration testing
   - Security audit reviews
   - Compliance validation

## Troubleshooting

### Common Security Issues

**Issue: Container execution blocked**
```
Error: execution blocked: HIGH risk level
```
**Solution:** Review security validation report and address identified risks.

**Issue: Resource limit exceeded**
```
Error: container killed: memory limit exceeded
```
**Solution:** Increase memory limits or optimize application memory usage.

**Issue: Network access denied**
```
Error: network access not permitted
```
**Solution:** Enable networking in security policy if required for legitimate use case.

### Security Debugging

**Enable Debug Logging:**
```bash
export CONTAINER_KIT_DEBUG=true
export CONTAINER_KIT_SECURITY_LOG_LEVEL=debug
```

**Security Validation Details:**
```go
// Get detailed security report
report, err := validator.ValidateSecurity(ctx, sessionID, options)
if err != nil {
    log.Error().Err(err).Msg("Security validation failed")
    return
}

// Log security details
log.Debug().
    Str("risk_level", report.RiskLevel).
    Int("threats_detected", len(report.Threats)).
    Int("controls_active", len(report.ActiveControls)).
    Msg("Security validation completed")
```

## References

- [Docker Security Best Practices](https://docs.docker.com/engine/security/)
- [Linux Capabilities](https://man7.org/linux/man-pages/man7/capabilities.7.html)
- [Seccomp Security Profiles](https://docs.docker.com/engine/security/seccomp/)
- [AppArmor Documentation](https://gitlab.com/apparmor/apparmor/-/wikis/Documentation)