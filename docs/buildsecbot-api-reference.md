# BuildSecBot API Reference

## Overview

This document provides a comprehensive API reference for BuildSecBot's atomic tools and supporting components.

## Atomic Tools

### atomic_build_image

Builds Docker images with comprehensive error recovery and progress tracking.

#### Arguments

```go
type AtomicBuildImageArgs struct {
    BaseToolArgs
    SessionID      string            `json:"session_id" jsonschema:"required"`
    DockerfilePath string            `json:"dockerfile_path" jsonschema:"required"`
    ImageName      string            `json:"image_name" jsonschema:"required"`
    WorkDir        string            `json:"work_dir,omitempty"`
    BuildArgs      map[string]string `json:"build_args,omitempty"`
    Platform       string            `json:"platform,omitempty"`
    NoCache        bool              `json:"no_cache,omitempty"`
    Squash         bool              `json:"squash,omitempty"`
    Labels         map[string]string `json:"labels,omitempty"`
    BuildContext   string            `json:"build_context,omitempty"`
    PullAlways     bool              `json:"pull_always,omitempty"`
    Target         string            `json:"target,omitempty"`
    NetworkMode    string            `json:"network_mode,omitempty"`
    ExtraHosts     []string          `json:"extra_hosts,omitempty"`
    SecretMounts   []string          `json:"secret_mounts,omitempty"`
}
```

#### Response

```go
type AtomicBuildImageResult struct {
    BaseToolResponse
    Success         bool              `json:"success"`
    SessionID       string            `json:"session_id"`
    WorkspaceDir    string            `json:"workspace_dir"`
    ImageName       string            `json:"image_name"`
    DockerfilePath  string            `json:"dockerfile_path"`
    ImageID         string            `json:"image_id,omitempty"`
    ImageSize       int64             `json:"image_size,omitempty"`
    BuildLogs       []string          `json:"build_logs,omitempty"`
    BuildDuration   time.Duration     `json:"build_duration"`
    TotalDuration   time.Duration     `json:"total_duration"`
    LayerCount      int               `json:"layer_count,omitempty"`
    BuildContext    *BuildContextInfo `json:"build_context,omitempty"`
}
```

### atomic_push_image

Pushes Docker images to registries with retry logic and progress tracking.

#### Arguments

```go
type AtomicPushImageArgs struct {
    BaseToolArgs
    ImageRef    string `json:"image_ref" jsonschema:"required"`
    RegistryURL string `json:"registry_url,omitempty"`
    Timeout     int    `json:"timeout,omitempty"`
    RetryCount  int    `json:"retry_count,omitempty"`
    Force       bool   `json:"force,omitempty"`
}
```

#### Response

```go
type AtomicPushImageResult struct {
    BaseToolResponse
    Success       bool                    `json:"success"`
    SessionID     string                  `json:"session_id"`
    WorkspaceDir  string                  `json:"workspace_dir"`
    ImageRef      string                  `json:"image_ref"`
    RegistryURL   string                  `json:"registry_url"`
    PushResult    *RegistryPushResult     `json:"push_result"`
    PushDuration  time.Duration           `json:"push_duration"`
    TotalDuration time.Duration           `json:"total_duration"`
    PushContext   *PushContext            `json:"push_context"`
}
```

### atomic_tag_image

Tags Docker images for versioning and organization.

#### Arguments

```go
type AtomicTagImageArgs struct {
    BaseToolArgs
    SourceImage string `json:"source_image" jsonschema:"required"`
    TargetImage string `json:"target_image" jsonschema:"required"`
    Force       bool   `json:"force,omitempty"`
}
```

#### Response

```go
type AtomicTagImageResult struct {
    BaseToolResponse
    Success       bool          `json:"success"`
    SessionID     string        `json:"session_id"`
    WorkspaceDir  string        `json:"workspace_dir"`
    SourceImage   string        `json:"source_image"`
    TargetImage   string        `json:"target_image"`
    TagResult     *TagResult    `json:"tag_result,omitempty"`
    TagDuration   time.Duration `json:"tag_duration"`
    TotalDuration time.Duration `json:"total_duration"`
    TagContext    *TagContext   `json:"tag_context"`
}
```

### atomic_scan_image_security

Performs comprehensive security scanning of Docker images.

#### Arguments

```go
type AtomicScanImageSecurityArgs struct {
    BaseToolArgs
    ImageName           string   `json:"image_name" jsonschema:"required"`
    SeverityThreshold   string   `json:"severity_threshold,omitempty"`
    VulnTypes           []string `json:"vuln_types,omitempty"`
    IgnoreCVEs          []string `json:"ignore_cves,omitempty"`
    IncludeRemediations bool     `json:"include_remediations,omitempty"`
    GenerateReport      bool     `json:"generate_report,omitempty"`
    ComplianceFramework string   `json:"compliance_framework,omitempty"`
    CustomPolicies      []string `json:"custom_policies,omitempty"`
    MaxResults          int      `json:"max_results,omitempty"`
}
```

#### Response

```go
type AtomicScanImageSecurityResult struct {
    BaseToolResponse
    Success              bool                        `json:"success"`
    SessionID            string                      `json:"session_id"`
    ImageName            string                      `json:"image_name"`
    ScanTime             time.Time                   `json:"scan_time"`
    Scanner              string                      `json:"scanner"`
    ScanResult           *ScanResult                 `json:"scan_result,omitempty"`
    VulnSummary          VulnerabilityAnalysisSummary `json:"vuln_summary"`
    SecurityScore        int                         `json:"security_score"`
    RiskLevel            string                      `json:"risk_level"`
    CriticalFindings     []CriticalSecurityFinding   `json:"critical_findings,omitempty"`
    Recommendations      []SecurityRecommendation    `json:"recommendations,omitempty"`
    ComplianceStatus     ComplianceAnalysis          `json:"compliance_status,omitempty"`
    RemediationPlan      *SecurityRemediationPlan    `json:"remediation_plan,omitempty"`
    GeneratedReport      string                      `json:"generated_report,omitempty"`
    Duration             time.Duration               `json:"duration"`
}
```

## Supporting Components

### BuildValidator

Validates Dockerfiles for syntax, best practices, and security.

```go
type BuildValidator struct {
    logger zerolog.Logger
}

func (v *BuildValidator) Validate(content string, options ValidationOptions) (*ValidationResult, error)
```

### SecurityValidator

Performs security validation and compliance checks.

```go
type SecurityValidator struct {
    logger            zerolog.Logger
    secretPatterns    []*regexp.Regexp
    trustedRegistries []string
    policies          []SecurityPolicy
}

func (v *SecurityValidator) Validate(content string, options ValidationOptions) (*ValidationResult, error)
func (v *SecurityValidator) ValidateCompliance(dockerfile string, framework string) *ComplianceResult
```

### BuildOptimizer

Analyzes and optimizes Docker build configurations.

```go
type BuildOptimizer struct {
    logger zerolog.Logger
}

func (o *BuildOptimizer) AnalyzeLayers(dockerfile string) *LayerAnalysis
func (o *BuildOptimizer) GetOptimizationSuggestions(analysis *LayerAnalysis) *OptimizationSuggestions
func (o *BuildOptimizer) OptimizeDockerfile(dockerfile string) (string, error)
```

### AdvancedBuildFixer

Provides AI-powered build error recovery.

```go
type AdvancedBuildFixer struct {
    analyzer Analyzer
    logger   zerolog.Logger
}

func (f *AdvancedBuildFixer) GetRecoveryStrategy(err *BuildFixerError) RecoveryStrategy
func (f *AdvancedBuildFixer) ApplyRecoveryStrategy(ctx context.Context, strategy RecoveryStrategy) error
```

### PerformanceMonitor

Monitors and analyzes build performance.

```go
type PerformanceMonitor struct {
    logger  zerolog.Logger
    metrics *prometheus.Registry
}

func (m *PerformanceMonitor) AnalyzePerformance(ctx context.Context, imageInfo *BuildImageInfo) *BuildPerformanceAnalysis
func (m *PerformanceMonitor) GetOptimizationRecommendations(analysis *BuildPerformanceAnalysis) []string
```

## Error Types

### BuildFixerError

Represents build-related errors with context for recovery.

```go
type BuildFixerError struct {
    Type    string                 `json:"type"`
    Message string                 `json:"message"`
    Stage   string                 `json:"stage,omitempty"`
    Context map[string]interface{} `json:"context,omitempty"`
}
```

### ValidationError

Represents validation errors in Dockerfiles.

```go
type ValidationError struct {
    Type        string `json:"type"`
    Message     string `json:"message"`
    Line        int    `json:"line,omitempty"`
    Column      int    `json:"column,omitempty"`
    Severity    string `json:"severity"`
    Rule        string `json:"rule,omitempty"`
    Suggestion  string `json:"suggestion,omitempty"`
}
```

## Context Types

### BuildContextInfo

Provides rich context about build operations.

```go
type BuildContextInfo struct {
    BuildStatus      string   `json:"build_status"`
    StagesCompleted  int      `json:"stages_completed"`
    TotalStages      int      `json:"total_stages"`
    CurrentStage     string   `json:"current_stage,omitempty"`
    LayersCreated    int      `json:"layers_created"`
    CacheUtilization float64  `json:"cache_utilization"`
    ErrorType        string   `json:"error_type,omitempty"`
    ErrorCategory    string   `json:"error_category,omitempty"`
    IsRetryable      bool     `json:"is_retryable"`
    NextStepSuggestions   []string `json:"next_step_suggestions"`
    TroubleshootingTips   []string `json:"troubleshooting_tips,omitempty"`
    OptimizationHints     []string `json:"optimization_hints,omitempty"`
}
```

### PushContext

Provides context about push operations.

```go
type PushContext struct {
    PushStatus       string   `json:"push_status"`
    LayersPushed     int      `json:"layers_pushed"`
    LayersCached     int      `json:"layers_cached"`
    PushSizeMB       float64  `json:"push_size_mb"`
    CacheHitRatio    float64  `json:"cache_hit_ratio"`
    RegistryType     string   `json:"registry_type"`
    RegistryEndpoint string   `json:"registry_endpoint"`
    AuthMethod       string   `json:"auth_method,omitempty"`
    ErrorType        string   `json:"error_type,omitempty"`
    ErrorCategory    string   `json:"error_category,omitempty"`
    IsRetryable      bool     `json:"is_retryable"`
    NextStepSuggestions   []string `json:"next_step_suggestions"`
    TroubleshootingTips   []string `json:"troubleshooting_tips,omitempty"`
    AuthenticationGuide   []string `json:"authentication_guide,omitempty"`
}
```

## Security Types

### VulnerabilityAnalysisSummary

Summary of vulnerability analysis results.

```go
type VulnerabilityAnalysisSummary struct {
    TotalVulnerabilities   int                `json:"total_vulnerabilities"`
    FixableVulnerabilities int                `json:"fixable_vulnerabilities"`
    SeverityBreakdown      map[string]int     `json:"severity_breakdown"`
    PackageBreakdown       map[string]int     `json:"package_breakdown"`
    LayerBreakdown         map[string]int     `json:"layer_breakdown"`
    AgeAnalysis            VulnAgeAnalysis    `json:"age_analysis"`
}
```

### SecurityRemediationPlan

Comprehensive plan for addressing security issues.

```go
type SecurityRemediationPlan struct {
    Summary        RemediationSummary        `json:"summary"`
    Steps          []RemediationStep         `json:"steps"`
    PackageUpdates map[string]PackageUpdate  `json:"package_updates"`
    Priority       string                    `json:"priority"`
}
```

### ComplianceResult

Results of compliance validation.

```go
type ComplianceResult struct {
    Framework  string                         `json:"framework"`
    Compliant  bool                           `json:"compliant"`
    Score      float64                        `json:"score"`
    Violations []SecurityComplianceViolation  `json:"violations"`
}
```

## Configuration

### ValidationOptions

Options for validation operations.

```go
type ValidationOptions struct {
    CheckSyntax        bool `json:"check_syntax"`
    CheckBestPractices bool `json:"check_best_practices"`
    CheckSecurity      bool `json:"check_security"`
    StrictMode         bool `json:"strict_mode"`
}
```

### BuildFixOptions

Options for build recovery operations.

```go
type BuildFixOptions struct {
    MaxRetries       int           `json:"max_retries"`
    RetryDelay       time.Duration `json:"retry_delay"`
    AutoFix          bool          `json:"auto_fix"`
    PreserveBehavior bool          `json:"preserve_behavior"`
}
```

## Metrics

### Prometheus Metrics

BuildSecBot exposes the following Prometheus metrics:

```
# Build metrics
container_kit_build_duration_seconds{tool,status}
container_kit_build_errors_total{tool,error_type}
container_kit_build_cache_hit_ratio

# Security metrics
container_kit_security_scan_duration_seconds{scanner,status}
container_kit_vulnerabilities_total{image,severity}
container_kit_compliance_score{image,framework}
container_kit_risk_score{image}

# Performance metrics
container_kit_image_size_bytes{image}
container_kit_layer_count{image}
container_kit_build_stage_duration_seconds{stage}
```

## Usage Examples

### Basic Build with Security Scan

```go
// Build image
buildArgs := AtomicBuildImageArgs{
    SessionID:      "session-123",
    DockerfilePath: "./Dockerfile",
    ImageName:      "myapp:latest",
}
buildResult, err := buildTool.ExecuteBuild(ctx, buildArgs)

// Scan for vulnerabilities
scanArgs := AtomicScanImageSecurityArgs{
    SessionID:           "session-123",
    ImageName:           buildResult.ImageName,
    SeverityThreshold:   "HIGH",
    IncludeRemediations: true,
}
scanResult, err := scanTool.ExecuteScan(ctx, scanArgs)
```

### Build with Optimization

```go
// Analyze Dockerfile
optimizer := NewBuildOptimizer(logger)
analysis := optimizer.AnalyzeLayers(dockerfileContent)

// Get optimization suggestions
suggestions := optimizer.GetOptimizationSuggestions(analysis)

// Apply optimizations
optimizedDockerfile, err := optimizer.OptimizeDockerfile(dockerfileContent)

// Build with optimized Dockerfile
buildArgs := AtomicBuildImageArgs{
    SessionID:      "session-123",
    DockerfilePath: "./Dockerfile.optimized",
    ImageName:      "myapp:optimized",
}
```

### Error Recovery

```go
// Build with automatic error recovery
fixer := NewAdvancedBuildFixer(analyzer, logger)

buildErr := &BuildFixerError{
    Type:    "dependency_error",
    Message: "Package installation failed",
}

// Get recovery strategy
strategy := fixer.GetRecoveryStrategy(buildErr)

// Apply recovery
err := fixer.ApplyRecoveryStrategy(ctx, strategy)
```

## Error Handling

All atomic tools follow consistent error handling patterns:

1. Validation errors return immediately with descriptive messages
2. Transient errors (network, timeouts) trigger retry logic
3. Build errors include recovery strategies
4. All errors include context for troubleshooting

Example error handling:

```go
result, err := tool.ExecuteBuild(ctx, args)
if err != nil {
    // Check if error is retryable
    if buildErr, ok := err.(*BuildFixerError); ok {
        if buildErr.IsRetryable() {
            // Retry logic
        }
    }
    // Handle non-retryable error
    return err
}

if !result.Success {
    // Check build context for issues
    if result.BuildContext.ErrorType == "dockerfile_syntax" {
        // Handle syntax error
    }
}
```

## Best Practices

1. **Always validate before building**: Use BuildValidator to catch issues early
2. **Scan all images**: Use atomic_scan_image_security before deployment
3. **Monitor metrics**: Track build performance and security metrics
4. **Handle errors gracefully**: Use recovery strategies for common failures
5. **Optimize builds**: Use BuildOptimizer to improve performance
6. **Follow security guidelines**: Implement security best practices in Dockerfiles

For more detailed examples and use cases, see the [Best Practices Guide](./buildsecbot-best-practices.md).