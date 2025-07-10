# Security Best Practices

Container Kit implements comprehensive security measures including session isolation, vulnerability scanning, and path traversal protection. This guide covers security architecture, best practices, and implementation details.

## Security Architecture

### Defense in Depth
Container Kit implements multiple layers of security:

1. **Input Validation**: All inputs validated and sanitized
2. **Session Isolation**: File operations scoped to session workspaces
3. **Path Traversal Protection**: Automatic path validation and sanitization
4. **Vulnerability Scanning**: Comprehensive image and dependency scanning
5. **Access Control**: Service-level authentication and authorization
6. **Audit Logging**: Complete audit trail of all operations

## FileAccessService Security

### Session-Based Workspace Isolation
```go
type FileAccessService interface {
    // All operations are session-scoped
    ReadFile(ctx context.Context, sessionID, relativePath string) (string, error)
    ListDirectory(ctx context.Context, sessionID, relativePath string) ([]FileInfo, error)
    FileExists(ctx context.Context, sessionID, relativePath string) (bool, error)
}

type secureFileAccessService struct {
    workspaceRoot string
    blockedPaths  []string
    maxFileSize   int64
}
```

### Path Traversal Prevention
```go
func (s *secureFileAccessService) validatePath(sessionID, relativePath string) (string, error) {
    // Clean the path to prevent traversal
    cleanPath := filepath.Clean(relativePath)
    
    // Ensure path is relative
    if filepath.IsAbs(cleanPath) {
        return "", ErrAbsolutePathNotAllowed
    }
    
    // Check for path traversal attempts
    if strings.Contains(cleanPath, "..") {
        return "", ErrPathTraversalAttempt
    }
    
    // Build full path within session workspace
    sessionWorkspace := filepath.Join(s.workspaceRoot, sessionID)
    fullPath := filepath.Join(sessionWorkspace, cleanPath)
    
    // Ensure resolved path is within workspace
    if !strings.HasPrefix(fullPath, sessionWorkspace) {
        return "", ErrPathOutsideWorkspace
    }
    
    return fullPath, nil
}
```

### File Type Validation
```go
func (s *secureFileAccessService) validateFileType(path string) error {
    ext := strings.ToLower(filepath.Ext(path))
    
    // Check blocked extensions
    blockedExts := []string{".exe", ".bat", ".sh", ".ps1"}
    for _, blocked := range blockedExts {
        if ext == blocked {
            return ErrBlockedFileType
        }
    }
    
    // Check file size
    info, err := os.Stat(path)
    if err != nil {
        return err
    }
    
    if info.Size() > s.maxFileSize {
        return ErrFileTooLarge
    }
    
    return nil
}
```

## Input Validation and Sanitization

### Validation DSL (ADR-005)
```go
type AnalyzeArgs struct {
    SessionID    string `json:"session_id" validate:"session_id"`
    RepositoryPath string `json:"repository_path" validate:"required,safe_path"`
    Options      string `json:"options" validate:"json_safe"`
}

// Tag-based validation with security rules
type ValidationRule struct {
    Tag      string
    Validate func(value interface{}) error
}

func validateSafePath(value interface{}) error {
    path := value.(string)
    
    // Check for dangerous characters
    dangerousChars := []string{"..", "\\", "$", "`", ";", "|", "&"}
    for _, char := range dangerousChars {
        if strings.Contains(path, char) {
            return ErrUnsafePathCharacter
        }
    }
    
    return nil
}
```

### Request Sanitization
```go
func sanitizeUserInput(input string) string {
    // Remove null bytes
    input = strings.ReplaceAll(input, "\x00", "")
    
    // Remove control characters
    input = regexp.MustCompile(`[\x00-\x1f\x7f]`).ReplaceAllString(input, "")
    
    // Limit length
    if len(input) > 1024 {
        input = input[:1024]
    }
    
    return input
}
```

## Vulnerability Scanning

### Security Scanner Integration
```go
type SecurityScanner interface {
    ScanImage(ctx context.Context, imageRef string) (*ScanResult, error)
    ScanDependencies(ctx context.Context, manifest string) (*ScanResult, error)
    GetPolicies(ctx context.Context) (*SecurityPolicies, error)
}

type ScanResult struct {
    ImageRef      string                 `json:"image_ref"`
    Vulnerabilities []Vulnerability       `json:"vulnerabilities"`
    Severity      map[string]int          `json:"severity"`
    SBOM          *SBOM                   `json:"sbom,omitempty"`
    Policies      []PolicyViolation       `json:"policy_violations"`
}
```

### Trivy Integration
```go
func (s *TrivyScanner) ScanImage(ctx context.Context, imageRef string) (*ScanResult, error) {
    // Validate image reference
    if err := validateImageRef(imageRef); err != nil {
        return nil, err
    }
    
    // Execute Trivy scan
    cmd := exec.CommandContext(ctx, "trivy", "image", 
        "--format", "json",
        "--severity", "HIGH,CRITICAL",
        imageRef)
    
    output, err := cmd.Output()
    if err != nil {
        return nil, fmt.Errorf("trivy scan failed: %w", err)
    }
    
    // Parse results
    var trivyResult TrivyResult
    if err := json.Unmarshal(output, &trivyResult); err != nil {
        return nil, err
    }
    
    return convertTrivyResult(&trivyResult), nil
}
```

### Policy Engine
```go
type SecurityPolicy struct {
    Name        string         `json:"name"`
    Description string         `json:"description"`
    Rules       []PolicyRule   `json:"rules"`
    Severity    string         `json:"severity"`
}

type PolicyRule struct {
    Type        string      `json:"type"`
    Condition   string      `json:"condition"`
    Value       interface{} `json:"value"`
    Action      string      `json:"action"` // "block", "warn", "log"
}

func (p *PolicyEngine) EvaluateImage(result *ScanResult) []PolicyViolation {
    var violations []PolicyViolation
    
    for _, policy := range p.policies {
        if violation := p.evaluatePolicy(policy, result); violation != nil {
            violations = append(violations, *violation)
        }
    }
    
    return violations
}
```

## Secret Management

### Secret Detection
```go
func detectSecrets(content string) []SecretFinding {
    var findings []SecretFinding
    
    // Common secret patterns
    patterns := map[string]*regexp.Regexp{
        "AWS Access Key": regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
        "GitHub Token":   regexp.MustCompile(`ghp_[a-zA-Z0-9]{36}`),
        "Private Key":    regexp.MustCompile(`-----BEGIN (RSA )?PRIVATE KEY-----`),
        "Password":       regexp.MustCompile(`password\s*[:=]\s*["']?[^\s"']+`),
    }
    
    for secretType, pattern := range patterns {
        matches := pattern.FindAllString(content, -1)
        for _, match := range matches {
            findings = append(findings, SecretFinding{
                Type:    secretType,
                Match:   match,
                Line:    getLineNumber(content, match),
                Context: getContext(content, match),
            })
        }
    }
    
    return findings
}
```

### Secret Sanitization
```go
func sanitizeDockerfile(content string) string {
    // Remove hardcoded secrets
    patterns := []struct {
        pattern     *regexp.Regexp
        replacement string
    }{
        {
            pattern:     regexp.MustCompile(`ENV\s+\w*PASSWORD\s*=\s*\S+`),
            replacement: "ENV PASSWORD=***REDACTED***",
        },
        {
            pattern:     regexp.MustCompile(`ENV\s+\w*TOKEN\s*=\s*\S+`),
            replacement: "ENV TOKEN=***REDACTED***",
        },
    }
    
    for _, p := range patterns {
        content = p.pattern.ReplaceAllString(content, p.replacement)
    }
    
    return content
}
```

## Container Security

### Dockerfile Security Validation
```go
func validateDockerfileSecurity(content string) []SecurityIssue {
    var issues []SecurityIssue
    lines := strings.Split(content, "\n")
    
    for i, line := range lines {
        line = strings.TrimSpace(line)
        
        // Check for root user
        if strings.HasPrefix(line, "USER root") {
            issues = append(issues, SecurityIssue{
                Line:        i + 1,
                Type:        "security",
                Severity:    "high",
                Message:     "Running as root user is not recommended",
                Suggestion:  "Use a non-root user: USER 1000:1000",
            })
        }
        
        // Check for curl/wget with pipes
        if regexp.MustCompile(`(curl|wget).*\|\s*(sh|bash)`).MatchString(line) {
            issues = append(issues, SecurityIssue{
                Line:        i + 1,
                Type:        "security",
                Severity:    "critical",
                Message:     "Piping downloads to shell is dangerous",
                Suggestion:  "Download to file first, then verify and execute",
            })
        }
        
        // Check for latest tag
        if regexp.MustCompile(`FROM\s+\S+:latest`).MatchString(line) {
            issues = append(issues, SecurityIssue{
                Line:        i + 1,
                Type:        "security",
                Severity:    "medium",
                Message:     "Using latest tag reduces reproducibility",
                Suggestion:  "Pin to specific version",
            })
        }
    }
    
    return issues
}
```

### Image Security Best Practices
```go
type SecureImageBuilder struct {
    baseImages map[string]bool // Approved base images
    scanners   []SecurityScanner
}

func (b *SecureImageBuilder) BuildSecureImage(dockerfile string) (*BuildResult, error) {
    // Validate Dockerfile security
    issues := validateDockerfileSecurity(dockerfile)
    criticalIssues := filterBySeverity(issues, "critical")
    
    if len(criticalIssues) > 0 {
        return nil, ErrCriticalSecurityIssues
    }
    
    // Build image with security hardening
    buildOptions := &BuildOptions{
        NoCache:       true,
        PullAlways:    true,
        SecurityOpt:   []string{"no-new-privileges"},
        BuildArgs: map[string]string{
            "BUILDKIT_INLINE_CACHE": "1",
        },
    }
    
    result, err := b.buildImage(dockerfile, buildOptions)
    if err != nil {
        return nil, err
    }
    
    // Scan built image
    scanResult, err := b.scanImage(result.ImageID)
    if err != nil {
        return nil, err
    }
    
    // Check security policies
    violations := b.evaluatePolicies(scanResult)
    if b.hasBlockingViolations(violations) {
        return nil, ErrPolicyViolations
    }
    
    return result, nil
}
```

## Access Control and Authentication

### Service-Level Security
```go
type SecurityContext struct {
    UserID      string
    SessionID   string
    Permissions []Permission
    IsAdmin     bool
}

func (s *ServiceContainer) WithSecurity(ctx context.Context) context.Context {
    secCtx := extractSecurityContext(ctx)
    
    // Validate session
    if !s.sessionStore.IsValid(secCtx.SessionID) {
        return contextWithError(ctx, ErrInvalidSession)
    }
    
    // Check permissions
    if !s.hasPermission(secCtx, "file.read") {
        return contextWithError(ctx, ErrInsufficientPermissions)
    }
    
    return ctx
}
```

### Role-Based Access Control (RBAC)
```go
type Role struct {
    Name        string       `json:"name"`
    Permissions []Permission `json:"permissions"`
}

type Permission struct {
    Resource string `json:"resource"` // "file", "docker", "k8s"
    Action   string `json:"action"`   // "read", "write", "execute"
    Scope    string `json:"scope"`    // "session", "global"
}

func (rbac *RBACService) CheckPermission(ctx context.Context, resource, action string) error {
    secCtx := getSecurityContext(ctx)
    
    for _, role := range secCtx.Roles {
        for _, perm := range role.Permissions {
            if perm.Resource == resource && perm.Action == action {
                return nil
            }
        }
    }
    
    return ErrAccessDenied
}
```

## Audit Logging and Monitoring

### Security Event Logging
```go
type SecurityLogger struct {
    logger *slog.Logger
}

func (s *SecurityLogger) LogSecurityEvent(ctx context.Context, event SecurityEvent) {
    secCtx := getSecurityContext(ctx)
    
    s.logger.Info("security_event",
        "event_type", event.Type,
        "user_id", secCtx.UserID,
        "session_id", secCtx.SessionID,
        "resource", event.Resource,
        "action", event.Action,
        "result", event.Result,
        "timestamp", time.Now(),
        "source_ip", getSourceIP(ctx),
    )
}
```

### Intrusion Detection
```go
type IntrusionDetector struct {
    rules []DetectionRule
}

type DetectionRule struct {
    Name        string
    Pattern     *regexp.Regexp
    Threshold   int
    Window      time.Duration
    Action      string // "log", "block", "alert"
}

func (d *IntrusionDetector) AnalyzeRequest(ctx context.Context, req *Request) error {
    for _, rule := range d.rules {
        if rule.Pattern.MatchString(req.Path) {
            count := d.getRecentCount(req.ClientIP, rule.Window)
            if count > rule.Threshold {
                return d.handleDetection(ctx, rule, req)
            }
        }
    }
    
    return nil
}
```

## Security Configuration

### Security Settings
```yaml
security:
  file_access:
    max_file_size: 10MB
    blocked_extensions: [".exe", ".bat", ".ps1"]
    workspace_isolation: true
    path_validation: strict
  
  scanning:
    enabled: true
    scanners: ["trivy", "grype"]
    fail_on_critical: true
    max_severity: "high"
  
  policies:
    enforce_non_root: true
    require_signed_images: false
    block_latest_tags: true
    scan_dependencies: true
  
  authentication:
    required: true
    session_timeout: 24h
    max_sessions: 1000
```

### Environment Security
```go
func setupSecureEnvironment() {
    // Remove sensitive environment variables
    sensitiveVars := []string{
        "AWS_SECRET_ACCESS_KEY",
        "GITHUB_TOKEN",
        "DOCKER_PASSWORD",
    }
    
    for _, env := range sensitiveVars {
        os.Unsetenv(env)
    }
    
    // Set security defaults
    os.Setenv("DOCKER_CONTENT_TRUST", "1")
    os.Setenv("BUILDKIT_INLINE_CACHE", "1")
}
```

## Security Testing

### Security Test Cases
```go
func TestPathTraversalPrevention(t *testing.T) {
    service := setupFileAccessService(t)
    
    testCases := []struct {
        name    string
        path    string
        expectError bool
    }{
        {"normal path", "file.txt", false},
        {"subdirectory", "dir/file.txt", false},
        {"parent directory", "../file.txt", true},
        {"absolute path", "/etc/passwd", true},
        {"double dot", "dir/../../../etc/passwd", true},
    }
    
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            _, err := service.ReadFile(context.Background(), "session-1", tc.path)
            if tc.expectError {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

### Vulnerability Testing
```go
func TestVulnerabilityScanning(t *testing.T) {
    scanner := setupTrivyScanner(t)
    
    // Test with known vulnerable image
    result, err := scanner.ScanImage(context.Background(), "vulnerable:latest")
    require.NoError(t, err)
    
    // Should detect vulnerabilities
    assert.Greater(t, len(result.Vulnerabilities), 0)
    
    // Should have critical vulnerabilities
    criticalCount := result.Severity["critical"]
    assert.Greater(t, criticalCount, 0)
}
```

## Security Incident Response

### Incident Detection
```go
func (s *SecurityMonitor) DetectAnomalies(ctx context.Context) {
    // Monitor for suspicious patterns
    patterns := []string{
        "multiple failed authentications",
        "path traversal attempts",
        "privilege escalation attempts",
        "unusual file access patterns",
    }
    
    for _, pattern := range patterns {
        if s.detectPattern(pattern) {
            s.triggerAlert(ctx, pattern)
        }
    }
}
```

### Automatic Response
```go
func (s *SecurityService) HandleSecurityIncident(ctx context.Context, incident *SecurityIncident) error {
    switch incident.Severity {
    case "critical":
        // Block session immediately
        return s.blockSession(incident.SessionID)
    case "high":
        // Rate limit and alert
        s.rateLimitSession(incident.SessionID)
        return s.sendAlert(incident)
    case "medium":
        // Log and monitor
        return s.logIncident(incident)
    }
    
    return nil
}
```

## Related Documentation

- [Architecture Overview](../../architecture/overview.md)
- [FileAccessService](../../reference/api/interfaces.md)
- [Error Handling](../developer/error-handling.md)
- [Testing Guide](testing.md)
- [Monitoring Guide](monitoring.md)

Container Kit's security architecture provides comprehensive protection while maintaining usability and performance, ensuring enterprise-grade security for containerization workflows.