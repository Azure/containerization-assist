# Getting Started with Container Kit

Container Kit is an AI-powered containerization platform that provides secure, production-ready sandboxing capabilities with comprehensive monitoring and validation.

## Quick Start

### Prerequisites

- Go 1.24.1 or later
- Docker installed and running
- Basic understanding of containerization concepts

### Installation

1. Clone the repository:
```bash
git clone https://github.com/Azure/container-kit.git
cd container-kit
```

2. Build the project:
```bash
make mcp
```

3. Run tests to verify installation:
```bash
make test
```

### Basic Usage

#### Creating a Workspace

```go
import "github.com/Azure/container-kit/pkg/mcp/internal/utils"

// Configure workspace
config := utils.WorkspaceConfig{
    BaseDir:           "/tmp/workspaces",
    MaxSizePerSession: 512 * 1024 * 1024, // 512MB
    TotalMaxSize:      2 * 1024 * 1024 * 1024, // 2GB
    Cleanup:           true,
    SandboxEnabled:    true,
    Logger:            logger,
}

// Create workspace manager
workspace, err := utils.NewWorkspaceManager(ctx, config)
if err != nil {
    log.Fatal(err)
}

// Initialize session workspace
sessionID := "my-session"
workspaceDir, err := workspace.InitializeWorkspace(ctx, sessionID)
```

#### Executing Commands Securely

```go
// Create sandbox executor
executor := utils.NewSandboxExecutor(workspace, logger)

// Configure security options
options := utils.SandboxOptions{
    BaseImage:     "alpine:3.18",
    MemoryLimit:   256 * 1024 * 1024,
    CPUQuota:      50000,
    Timeout:       30 * time.Second,
    ReadOnly:      true,
    NetworkAccess: false,
    User:          "1000",
    Group:         "1000",
    Capabilities:  []string{}, // No capabilities
    SecurityPolicy: utils.SecurityPolicy{
        AllowNetworking:   false,
        AllowFileSystem:   true,
        RequireNonRoot:    true,
        TrustedRegistries: []string{"docker.io"},
    },
    EnableMetrics: true,
    EnableAudit:   true,
}

// Execute command
cmd := []string{"echo", "Hello from secure sandbox"}
result, err := executor.ExecuteAdvanced(ctx, sessionID, cmd, options)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Exit code: %d\n", result.ExitCode)
fmt.Printf("Output: %s\n", result.Stdout)
```

#### Security Validation

```go
// Create security validator
validator := utils.NewSecurityValidator(logger)

// Validate configuration
report, err := validator.ValidateSecurity(ctx, sessionID, options)
if err != nil {
    log.Fatal(err)
}

if !report.Passed {
    fmt.Printf("Security validation failed: %s\n", report.OverallRisk)
    for _, vuln := range report.Vulnerabilities {
        fmt.Printf("Vulnerability: %s - %s\n", vuln.CVE, vuln.Description)
    }
}
```

## Security Features

### Built-in Security Controls

- **Non-root execution**: All containers run with non-privileged users
- **Read-only filesystems**: Prevents modification of container internals
- **Network isolation**: Containers run without network access by default
- **Resource limits**: CPU and memory constraints prevent resource exhaustion
- **Capability dropping**: Removes dangerous Linux capabilities
- **Execution timeouts**: Prevents long-running processes

### Threat Protection

Container Kit protects against:
- Container escape attacks
- Code injection vulnerabilities  
- Resource exhaustion (DoS)
- Privilege escalation
- Data exfiltration

### Compliance

- CIS Docker Benchmark compliance
- NIST SP 800-190 container security guidelines
- Automated vulnerability scanning
- Audit logging for all operations

## Monitoring and Metrics

### Execution Metrics

```go
// Metrics are automatically collected when enabled
collector := utils.NewSandboxMetricsCollector()

// Access execution history
collector.mutex.RLock()
for _, record := range collector.history {
    fmt.Printf("Execution %s: %v\n", record.ID, record.Duration)
}
collector.mutex.RUnlock()
```

### Audit Logging

All security events are automatically logged:
- Execution attempts
- Security policy violations
- Resource usage
- Access patterns

## Best Practices

### Security

1. **Always use specific image tags** - Avoid `latest` tags
2. **Enable read-only mode** - Set `ReadOnly: true` unless write access is required
3. **Disable network access** - Set `NetworkAccess: false` by default
4. **Use non-root users** - Set appropriate `User` and `Group` values
5. **Drop capabilities** - Keep `Capabilities` array empty
6. **Set resource limits** - Always specify `MemoryLimit` and `CPUQuota`
7. **Use timeouts** - Set reasonable `Timeout` values

### Performance

1. **Reuse workspaces** - Initialize once per session
2. **Monitor resource usage** - Enable metrics collection
3. **Clean up regularly** - Use `Cleanup: true` in workspace config
4. **Use appropriate quotas** - Set realistic disk usage limits

### Troubleshooting

Common issues and solutions:

#### Docker Not Found
```
Error: docker command not found for sandboxing
Solution: Install Docker and ensure it's in PATH
```

#### Permission Denied
```
Error: failed to create workspace directory
Solution: Ensure write permissions to base directory
```

#### Resource Limits
```
Error: container killed due to resource constraints
Solution: Increase MemoryLimit or CPUQuota values
```

## Next Steps

- Read the [Security Architecture Guide](../security/security-architecture.md)
- Review [Security Best Practices](../security/security-best-practices.md)
- Explore [Advanced Configuration](advanced-configuration.md)
- Check out [Integration Examples](integration-examples.md)