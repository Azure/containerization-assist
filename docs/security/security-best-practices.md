# Security Best Practices

## Overview

This document provides comprehensive security best practices for Container Kit operations, covering development, deployment, and operational security guidelines. These practices are designed to maintain the highest security standards while ensuring operational efficiency.

## Container Security Fundamentals

### Principle of Least Privilege

**Always run containers with minimal privileges:**

```go
// ✅ Good: Non-root user with minimal capabilities
options := AdvancedSandboxOptions{
    User:         "1000",
    Group:        "1000",
    Capabilities: []string{}, // No capabilities unless absolutely necessary
    SecurityPolicy: SecurityPolicy{
        RequireNonRoot: true,
    },
}

// ❌ Bad: Root user with excessive privileges
options := AdvancedSandboxOptions{
    User:       "root",     // Avoid root execution
    Privileged: true,       // Never use privileged mode
    Capabilities: []string{"CAP_SYS_ADMIN"}, // Dangerous capability
}
```

**Implementation Guidelines:**
- Use non-root users (UID/GID 1000 or higher)
- Drop all Linux capabilities by default
- Grant only necessary permissions
- Implement time-limited access when possible

### Defense in Depth

**Layer multiple security controls:**

```go
type SecureConfiguration struct {
    // Layer 1: User isolation
    User:  "1000",
    Group: "1000",

    // Layer 2: Filesystem protection
    ReadonlyRootfs: true,

    // Layer 3: Network isolation
    NetworkMode: "none",

    // Layer 4: Resource limits
    MemoryLimit: "512MB",
    CPULimit:    "1.0",

    // Layer 5: Security profiles
    SecurityOpt: []string{
        "no-new-privileges:true",
        "apparmor:container-default",
        "seccomp:container-default.json",
    },
}
```

## Image Security

### Base Image Selection

**Choose minimal, trusted base images:**

```dockerfile
# ✅ Good: Minimal Alpine base
FROM alpine:3.18
RUN adduser -D -s /bin/sh appuser
USER appuser

# ✅ Good: Distroless for static binaries
FROM gcr.io/distroless/static-debian11
COPY --from=builder /app/binary /app
USER 65534:65534

# ❌ Bad: Large, vulnerable base
FROM ubuntu:latest
RUN apt-get update && apt-get install -y curl wget git
# Running as root with unnecessary packages
```

**Best Practices:**
- Use official, regularly updated images
- Prefer Alpine or distroless images
- Avoid images with known vulnerabilities
- Maintain an approved base image registry

### Image Hardening

**Implement multi-stage builds for security:**

```dockerfile
# Build stage
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o app .

# Production stage
FROM alpine:3.18
RUN addgroup -g 1000 appgroup && \
    adduser -D -u 1000 -G appgroup appuser && \
    apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/app .
COPY --chown=appuser:appgroup configs/ ./configs/
USER appuser:appgroup
EXPOSE 8080
CMD ["./app"]
```

### Image Scanning

**Integrate vulnerability scanning into CI/CD:**

```go
func (sv *SecurityValidator) validateImage(ctx context.Context, image string) error {
    // Check image source
    if !sv.isTrustedRegistry(image) {
        return fmt.Errorf("untrusted registry: %s", image)
    }

    // Scan for vulnerabilities
    vulns, err := sv.scanner.ScanImage(ctx, image)
    if err != nil {
        return fmt.Errorf("image scan failed: %w", err)
    }

    // Check vulnerability thresholds
    criticalCount := sv.countVulnerabilities(vulns, "CRITICAL")
    highCount := sv.countVulnerabilities(vulns, "HIGH")

    if criticalCount > 0 {
        return fmt.Errorf("image contains %d critical vulnerabilities", criticalCount)
    }

    if highCount > 5 { // Configurable threshold
        return fmt.Errorf("image contains %d high-severity vulnerabilities", highCount)
    }

    return nil
}
```

## Runtime Security

### Container Configuration

**Secure container runtime configuration:**

```go
func buildSecureContainerConfig() *container.Config {
    return &container.Config{
        User:         "1000:1000",
        WorkingDir:   "/app",
        Env: []string{
            "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
            // No sensitive environment variables
        },
        ExposedPorts: nat.PortSet{
            "8080/tcp": struct{}{}, // Only necessary ports
        },
        Healthcheck: &container.HealthConfig{
            Test:     []string{"CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:8080/health"},
            Interval: 30 * time.Second,
            Timeout:  5 * time.Second,
            Retries:  3,
        },
    }
}

func buildSecureHostConfig() *container.HostConfig {
    return &container.HostConfig{
        // Security restrictions
        ReadonlyRootfs: true,
        SecurityOpt: []string{
            "no-new-privileges:true",
            "apparmor:container-default",
        },

        // Resource limits
        Memory:    512 * 1024 * 1024, // 512MB
        CPUQuota:  100000,            // 1 CPU
        CPUPeriod: 100000,
        PidsLimit: &[]int64{100}[0],  // Process limit

        // Network isolation
        NetworkMode: "bridge", // Use bridge, not host

        // Capability management
        CapDrop: []string{"ALL"},
        CapAdd:  []string{}, // Add only necessary capabilities

        // Filesystem management
        Tmpfs: map[string]string{
            "/tmp":     "rw,noexec,nosuid,size=100m",
            "/var/tmp": "rw,noexec,nosuid,size=100m",
        },

        // Volume restrictions
        Binds: []string{
            // Read-only mounts only
            "/etc/localtime:/etc/localtime:ro",
        },
    }
}
```

### Resource Management

**Implement proper resource limits:**

```go
type ResourcePolicy struct {
    MaxMemory    string        `json:"max_memory"`     // "512MB"
    MaxCPU       string        `json:"max_cpu"`        // "1.0"
    MaxProcesses int           `json:"max_processes"`  // 100
    MaxFileSize  string        `json:"max_file_size"`  // "100MB"
    Timeout      time.Duration `json:"timeout"`        // 5 minutes
}

func (sv *SecurityValidator) enforceResourceLimits(config *container.HostConfig, policy ResourcePolicy) error {
    // Memory limit
    if memoryBytes, err := parseMemory(policy.MaxMemory); err == nil {
        config.Memory = memoryBytes
        config.MemorySwap = memoryBytes // Disable swap
    }

    // CPU limit
    if cpuShares, err := parseCPU(policy.MaxCPU); err == nil {
        config.CPUShares = cpuShares
    }

    // Process limit
    if policy.MaxProcesses > 0 {
        limit := int64(policy.MaxProcesses)
        config.PidsLimit = &limit
    }

    // Execution timeout
    if policy.Timeout > 0 {
        // Implement timeout in execution context
    }

    return nil
}
```

## Network Security

### Network Isolation

**Default to network isolation:**

```go
func (sv *SecurityValidator) configureNetworkSecurity(options AdvancedSandboxOptions) NetworkConfig {
    config := NetworkConfig{
        Mode:     "none", // Default to no network
        Isolated: true,
    }

    // Only enable networking if explicitly required and approved
    if options.SecurityPolicy.AllowNetworking {
        if sv.isNetworkingJustified(options) {
            config.Mode = "bridge"
            config.AllowedPorts = sv.getApprovedPorts(options)
            config.DNSServers = []string{"8.8.8.8", "8.8.4.4"} // Controlled DNS
        }
    }

    return config
}

func (sv *SecurityValidator) isNetworkingJustified(options AdvancedSandboxOptions) bool {
    // Check if networking is required for legitimate functionality
    justifications := []string{
        "API_CLIENT",          // Needs to call external APIs
        "DATABASE_CONNECTION", // Needs database access
        "SERVICE_DISCOVERY",   // Needs service registry access
    }

    return contains(justifications, options.NetworkJustification)
}
```

### Traffic Monitoring

**Monitor and log network activity:**

```go
type NetworkMonitor struct {
    logger    zerolog.Logger
    whitelist map[string]bool
    blacklist map[string]bool
}

func (nm *NetworkMonitor) MonitorTraffic(ctx context.Context, containerID string) {
    // Monitor outbound connections
    go nm.monitorOutboundConnections(ctx, containerID)

    // Monitor DNS queries
    go nm.monitorDNSQueries(ctx, containerID)

    // Monitor port usage
    go nm.monitorPortUsage(ctx, containerID)
}

func (nm *NetworkMonitor) evaluateConnection(dest string, port int) bool {
    // Check against blacklist
    if nm.blacklist[dest] {
        nm.logger.Warn().Str("destination", dest).Msg("Blocked connection to blacklisted destination")
        return false
    }

    // Check against whitelist (if used)
    if len(nm.whitelist) > 0 && !nm.whitelist[dest] {
        nm.logger.Warn().Str("destination", dest).Msg("Blocked connection to non-whitelisted destination")
        return false
    }

    return true
}
```

## Data Protection

### Secrets Management

**Never embed secrets in containers:**

```go
// ❌ Bad: Hardcoded secrets
const (
    APIKey = "sk-1234567890abcdef" // Never do this
    DBPassword = "mysecretpassword"
)

// ✅ Good: Environment-based secrets (with caution)
func getAPIKey() string {
    key := os.Getenv("API_KEY")
    if key == "" {
        log.Fatal("API_KEY environment variable not set")
    }
    return key
}

// ✅ Better: External secret management
func getSecretFromVault(secretPath string) (string, error) {
    client, err := vault.NewClient(&vault.Config{
        Address: os.Getenv("VAULT_ADDR"),
    })
    if err != nil {
        return "", err
    }

    secret, err := client.Logical().Read(secretPath)
    if err != nil {
        return "", err
    }

    return secret.Data["value"].(string), nil
}
```

### Data Encryption

**Encrypt sensitive data at rest and in transit:**

```go
type DataProtection struct {
    encryptionKey []byte
    cipher       cipher.AEAD
}

func NewDataProtection(key string) (*DataProtection, error) {
    keyBytes, err := base64.StdEncoding.DecodeString(key)
    if err != nil {
        return nil, err
    }

    block, err := aes.NewCipher(keyBytes)
    if err != nil {
        return nil, err
    }

    aead, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }

    return &DataProtection{
        encryptionKey: keyBytes,
        cipher:       aead,
    }, nil
}

func (dp *DataProtection) Encrypt(data []byte) ([]byte, error) {
    nonce := make([]byte, dp.cipher.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return nil, err
    }

    ciphertext := dp.cipher.Seal(nonce, nonce, data, nil)
    return ciphertext, nil
}
```

## Logging and Monitoring

### Security Event Logging

**Implement comprehensive security logging:**

```go
type SecurityLogger struct {
    logger zerolog.Logger
    level  SecurityLogLevel
}

type SecurityEvent struct {
    Timestamp   time.Time                 `json:"timestamp"`
    EventType   string                    `json:"event_type"`
    Severity    string                    `json:"severity"`
    SessionID   string                    `json:"session_id"`
    UserID      string                    `json:"user_id,omitempty"`
    Action      string                    `json:"action"`
    Resource    string                    `json:"resource"`
    Result      string                    `json:"result"`
    Details     map[string]interface{}    `json:"details"`
    RemoteAddr  string                    `json:"remote_addr,omitempty"`
}

func (sl *SecurityLogger) LogSecurityEvent(event SecurityEvent) {
    event.Timestamp = time.Now()

    logEvent := sl.logger.Info()
    if event.Severity == "HIGH" || event.Severity == "CRITICAL" {
        logEvent = sl.logger.Error()
    }

    logEvent.
        Str("event_type", event.EventType).
        Str("severity", event.Severity).
        Str("session_id", event.SessionID).
        Str("action", event.Action).
        Str("resource", event.Resource).
        Str("result", event.Result).
        Interface("details", event.Details).
        Msg("Security event")
}

// Example usage
func (sv *SecurityValidator) logExecutionAttempt(sessionID string, options AdvancedSandboxOptions, result string) {
    sv.securityLogger.LogSecurityEvent(SecurityEvent{
        EventType: "CONTAINER_EXECUTION",
        Severity:  "INFO",
        SessionID: sessionID,
        Action:    "EXECUTE",
        Resource:  "CONTAINER",
        Result:    result,
        Details: map[string]interface{}{
            "user":            options.User,
            "image":           options.Image,
            "capabilities":    options.Capabilities,
            "network_enabled": options.SecurityPolicy.AllowNetworking,
        },
    })
}
```

### Real-time Monitoring

**Monitor security metrics and anomalies:**

```go
type SecurityMetrics struct {
    ExecutionAttempts    prometheus.Counter
    SecurityViolations   prometheus.Counter
    HighRiskExecutions   prometheus.Counter
    AverageRiskScore     prometheus.Histogram
    ComplianceScore      prometheus.Gauge
}

func NewSecurityMetrics() *SecurityMetrics {
    return &SecurityMetrics{
        ExecutionAttempts: prometheus.NewCounter(prometheus.CounterOpts{
            Name: "container_execution_attempts_total",
            Help: "Total number of container execution attempts",
        }),
        SecurityViolations: prometheus.NewCounter(prometheus.CounterOpts{
            Name: "security_violations_total",
            Help: "Total number of security violations detected",
        }),
        HighRiskExecutions: prometheus.NewCounter(prometheus.CounterOpts{
            Name: "high_risk_executions_total",
            Help: "Total number of high-risk executions",
        }),
        AverageRiskScore: prometheus.NewHistogram(prometheus.HistogramOpts{
            Name:    "security_risk_score",
            Help:    "Distribution of security risk scores",
            Buckets: []float64{1, 2, 3, 4, 5, 6, 7, 8, 9},
        }),
        ComplianceScore: prometheus.NewGauge(prometheus.GaugeOpts{
            Name: "compliance_score",
            Help: "Current compliance score percentage",
        }),
    }
}
```

## Development Security

### Secure Development Lifecycle

**Integrate security into development workflow:**

```yaml
# .github/workflows/security.yml
name: Security Checks
on: [push, pull_request]

jobs:
  security-scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Run Security Tests
        run: |
          make test-security
          make lint-security

      - name: Vulnerability Scan
        uses: securecodewarrior/github-action-add-sarif@v1
        with:
          sarif-file: security-scan-results.sarif

      - name: Container Security Scan
        run: |
          docker build -t test-image .
          trivy image test-image
```

### Code Security Guidelines

**Follow secure coding practices:**

```go
// ✅ Good: Input validation
func validateInput(input string) error {
    if len(input) > 1000 {
        return fmt.Errorf("input too long")
    }

    // Check for dangerous patterns
    dangerousPatterns := []string{
        `\$\(.*\)`,  // Command substitution
        "`.*`",      // Backtick execution
        "&&", "||",  // Command chaining
        "|", ">", "<", ";", // Shell operators
    }

    for _, pattern := range dangerousPatterns {
        if matched, _ := regexp.MatchString(pattern, input); matched {
            return fmt.Errorf("dangerous pattern detected: %s", pattern)
        }
    }

    return nil
}

// ✅ Good: Parameterized queries (if using database)
func getUserByID(db *sql.DB, userID string) (*User, error) {
    query := "SELECT id, name, email FROM users WHERE id = ?"
    row := db.QueryRow(query, userID)

    var user User
    err := row.Scan(&user.ID, &user.Name, &user.Email)
    if err != nil {
        return nil, err
    }

    return &user, nil
}

// ❌ Bad: String concatenation (SQL injection risk)
func getUserByIDBad(db *sql.DB, userID string) (*User, error) {
    query := "SELECT id, name, email FROM users WHERE id = '" + userID + "'"
    // This is vulnerable to SQL injection
    row := db.QueryRow(query)
    // ... rest of implementation
}
```

## Incident Response

### Security Incident Handling

**Implement automated incident response:**

```go
type IncidentResponse struct {
    logger    zerolog.Logger
    alerter   AlertManager
    quarantine QuarantineManager
}

func (ir *IncidentResponse) HandleSecurityIncident(incident SecurityIncident) {
    ir.logger.Error().
        Str("incident_id", incident.ID).
        Str("severity", incident.Severity).
        Msg("Security incident detected")

    switch incident.Severity {
    case "CRITICAL":
        ir.handleCriticalIncident(incident)
    case "HIGH":
        ir.handleHighSeverityIncident(incident)
    default:
        ir.handleStandardIncident(incident)
    }
}

func (ir *IncidentResponse) handleCriticalIncident(incident SecurityIncident) {
    // Immediate containment
    if incident.ContainerID != "" {
        ir.quarantine.QuarantineContainer(incident.ContainerID)
    }

    // Alert security team
    ir.alerter.SendCriticalAlert(incident)

    // Block further executions from affected session
    ir.blockSession(incident.SessionID)

    // Collect forensic data
    go ir.collectForensicData(incident)
}

type SecurityIncident struct {
    ID          string                 `json:"id"`
    Timestamp   time.Time              `json:"timestamp"`
    Severity    string                 `json:"severity"`
    Type        string                 `json:"type"`
    Description string                 `json:"description"`
    SessionID   string                 `json:"session_id"`
    ContainerID string                 `json:"container_id,omitempty"`
    Details     map[string]interface{} `json:"details"`
}
```

### Forensic Data Collection

**Collect evidence for security incidents:**

```go
type ForensicCollector struct {
    logger  zerolog.Logger
    storage ForensicStorage
}

func (fc *ForensicCollector) CollectEvidence(incident SecurityIncident) (*ForensicReport, error) {
    report := &ForensicReport{
        IncidentID: incident.ID,
        Timestamp:  time.Now(),
        Collector:  "container-kit-forensics",
    }

    // Collect container state
    if incident.ContainerID != "" {
        containerState, err := fc.collectContainerState(incident.ContainerID)
        if err != nil {
            fc.logger.Error().Err(err).Msg("Failed to collect container state")
        } else {
            report.ContainerState = containerState
        }
    }

    // Collect system logs
    logs, err := fc.collectSystemLogs(incident.Timestamp.Add(-5*time.Minute), time.Now())
    if err != nil {
        fc.logger.Error().Err(err).Msg("Failed to collect system logs")
    } else {
        report.SystemLogs = logs
    }

    // Collect network data
    networkData, err := fc.collectNetworkData(incident.SessionID)
    if err != nil {
        fc.logger.Error().Err(err).Msg("Failed to collect network data")
    } else {
        report.NetworkData = networkData
    }

    // Store evidence
    if err := fc.storage.StoreEvidence(report); err != nil {
        return nil, fmt.Errorf("failed to store forensic evidence: %w", err)
    }

    return report, nil
}
```

## Compliance and Auditing

### Audit Trail Management

**Maintain comprehensive audit trails:**

```go
type AuditManager struct {
    logger    zerolog.Logger
    storage   AuditStorage
    retention time.Duration
}

func (am *AuditManager) LogAuditEvent(event AuditEvent) error {
    event.Timestamp = time.Now()
    event.ID = generateAuditID()

    // Log to structured logger
    am.logger.Info().
        Str("audit_id", event.ID).
        Str("event_type", event.Type).
        Str("user_id", event.UserID).
        Str("session_id", event.SessionID).
        Str("action", event.Action).
        Str("resource", event.Resource).
        Interface("details", event.Details).
        Msg("Audit event")

    // Store in audit database
    return am.storage.StoreAuditEvent(event)
}

type AuditEvent struct {
    ID          string                 `json:"id"`
    Timestamp   time.Time              `json:"timestamp"`
    Type        string                 `json:"type"`
    UserID      string                 `json:"user_id"`
    SessionID   string                 `json:"session_id"`
    Action      string                 `json:"action"`
    Resource    string                 `json:"resource"`
    Result      string                 `json:"result"`
    IPAddress   string                 `json:"ip_address"`
    UserAgent   string                 `json:"user_agent"`
    Details     map[string]interface{} `json:"details"`
}
```

## Performance and Security Trade-offs

### Optimized Security Checks

**Balance security and performance:**

```go
type OptimizedSecurityValidator struct {
    cache       *SecurityCache
    riskProfile RiskProfile
    fastMode    bool
}

func (osv *OptimizedSecurityValidator) ValidateWithProfile(ctx context.Context,
    sessionID string, options AdvancedSandboxOptions) (*SecurityValidationReport, error) {

    // Check cache for recent validation
    if cached := osv.cache.Get(sessionID, options.Hash()); cached != nil {
        if time.Since(cached.Timestamp) < 5*time.Minute {
            return cached.Report, nil
        }
    }

    // Determine validation depth based on risk profile
    depth := osv.getValidationDepth(options)

    var report *SecurityValidationReport
    var err error

    switch depth {
    case ValidationDepthFast:
        report, err = osv.performFastValidation(ctx, sessionID, options)
    case ValidationDepthStandard:
        report, err = osv.performStandardValidation(ctx, sessionID, options)
    case ValidationDepthDeep:
        report, err = osv.performDeepValidation(ctx, sessionID, options)
    }

    if err != nil {
        return nil, err
    }

    // Cache result
    osv.cache.Set(sessionID, options.Hash(), report)

    return report, nil
}

func (osv *OptimizedSecurityValidator) getValidationDepth(options AdvancedSandboxOptions) ValidationDepth {
    // Fast validation for low-risk, known-good configurations
    if osv.isKnownSafeConfiguration(options) {
        return ValidationDepthFast
    }

    // Deep validation for high-risk configurations
    if osv.isHighRiskConfiguration(options) {
        return ValidationDepthDeep
    }

    // Standard validation for everything else
    return ValidationDepthStandard
}
```

## Training and Awareness

### Security Training Guidelines

**Educate team members on security practices:**

1. **Regular Security Training**
   - Container security fundamentals
   - Threat modeling workshops
   - Incident response drills
   - Compliance requirements

2. **Security Champions Program**
   - Designate security champions in each team
   - Provide advanced security training
   - Regular security review meetings
   - Security knowledge sharing sessions

3. **Documentation and Resources**
   - Maintain up-to-date security documentation
   - Create security checklists and playbooks
   - Provide quick reference guides
   - Share security best practices

### Security Culture

**Foster a security-first mindset:**

```go
// Example: Security code review checklist
type SecurityReviewChecklist struct {
    InputValidation    bool `json:"input_validation"`     // All inputs validated?
    OutputSanitization bool `json:"output_sanitization"`  // Outputs properly sanitized?
    AuthenticationAuth bool `json:"authentication"`       // Proper authentication?
    AuthorizationCheck bool `json:"authorization"`        // Authorization implemented?
    ErrorHandling      bool `json:"error_handling"`       // Secure error handling?
    LoggingAuditing   bool `json:"logging_auditing"`     // Comprehensive logging?
    CryptographyUse   bool `json:"cryptography"`         // Proper crypto usage?
    DependencyCheck   bool `json:"dependency_check"`     // Dependencies scanned?
}

func (src *SecurityReviewChecklist) IsComplete() bool {
    return src.InputValidation &&
           src.OutputSanitization &&
           src.AuthenticationAuth &&
           src.AuthorizationCheck &&
           src.ErrorHandling &&
           src.LoggingAuditing &&
           src.CryptographyUse &&
           src.DependencyCheck
}
```

## Summary

Container Kit's security best practices provide a comprehensive framework for maintaining strong security posture while enabling efficient container operations. Key principles include:

1. **Defense in Depth**: Multiple layered security controls
2. **Least Privilege**: Minimal necessary permissions
3. **Continuous Monitoring**: Real-time security awareness
4. **Incident Response**: Rapid containment and recovery
5. **Compliance**: Adherence to industry standards
6. **Security Culture**: Organization-wide security awareness

Regular review and updates of these practices ensure continued effectiveness against evolving threats.

## References

- [OWASP Container Security](https://owasp.org/www-project-container-security/)
- [NIST Container Security Guide](https://csrc.nist.gov/publications/detail/sp/800-190/final)
- [CIS Docker Benchmark](https://www.cisecurity.org/benchmark/docker)
- [Docker Security Best Practices](https://docs.docker.com/engine/security/)
- [Kubernetes Security Best Practices](https://kubernetes.io/docs/concepts/security/)
