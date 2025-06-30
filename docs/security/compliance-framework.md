# Compliance Framework

## Overview

Container Kit implements comprehensive compliance checking against industry-standard security frameworks and benchmarks. The compliance framework provides automated assessment, continuous monitoring, and reporting capabilities to ensure container operations meet regulatory and security requirements.

## Supported Frameworks

### CIS Docker Benchmark v1.6.0

The Center for Internet Security (CIS) Docker Benchmark provides security configuration guidelines for Docker environments.

#### Implemented Controls

**4.1 - Ensure that a user for the container has been created**
```go
func (cf *ComplianceFramework) CheckCIS_4_1(config ContainerConfig) ComplianceResult {
    // Check for non-root user
    if config.User == "" || config.User == "0" || config.User == "root" {
        return ComplianceResult{
            ControlID:   "CIS-4.1",
            Status:      "FAIL",
            Description: "Container running as root user",
            Remediation: "Set User field to non-root user (e.g., '1000')",
            Severity:    "HIGH",
        }
    }
    
    return ComplianceResult{
        ControlID:   "CIS-4.1",
        Status:      "PASS",
        Description: "Container configured with non-root user",
        Severity:    "INFO",
    }
}
```

**4.5 - Ensure Content trust for Docker is Enabled**
```go
func (cf *ComplianceFramework) CheckCIS_4_5(options AdvancedSandboxOptions) ComplianceResult {
    // Check for trusted registries
    if len(options.SecurityPolicy.TrustedRegistries) == 0 {
        return ComplianceResult{
            ControlID:   "CIS-4.5",
            Status:      "FAIL", 
            Description: "No trusted registries configured",
            Remediation: "Configure trusted container registries",
            Severity:    "MEDIUM",
        }
    }
    
    return ComplianceResult{
        ControlID:   "CIS-4.5",
        Status:      "PASS",
        Description: "Trusted registries configured",
        Severity:    "INFO",
    }
}
```

**5.3 - Ensure that Linux kernel capabilities are restricted within containers**
```go
func (cf *ComplianceFramework) CheckCIS_5_3(config ContainerConfig) ComplianceResult {
    dangerousCapabilities := []string{
        "CAP_SYS_ADMIN", "CAP_NET_ADMIN", "CAP_SYS_PTRACE",
        "CAP_SYS_MODULE", "CAP_DAC_OVERRIDE", "CAP_SETUID",
        "CAP_SETGID", "CAP_NET_RAW",
    }
    
    for _, cap := range config.Capabilities {
        for _, dangerous := range dangerousCapabilities {
            if cap == dangerous {
                return ComplianceResult{
                    ControlID:   "CIS-5.3",
                    Status:      "FAIL",
                    Description: fmt.Sprintf("Dangerous capability detected: %s", cap),
                    Remediation: "Remove unnecessary Linux capabilities",
                    Severity:    "HIGH",
                }
            }
        }
    }
    
    return ComplianceResult{
        ControlID:   "CIS-5.3", 
        Status:      "PASS",
        Description: "No dangerous capabilities detected",
        Severity:    "INFO",
    }
}
```

**5.9 - Ensure that the host's network namespace is not shared**
```go
func (cf *ComplianceFramework) CheckCIS_5_9(config ContainerConfig) ComplianceResult {
    if config.NetworkMode == "host" {
        return ComplianceResult{
            ControlID:   "CIS-5.9",
            Status:      "FAIL",
            Description: "Container sharing host network namespace",
            Remediation: "Avoid using --network=host",
            Severity:    "HIGH",
        }
    }
    
    return ComplianceResult{
        ControlID:   "CIS-5.9",
        Status:      "PASS", 
        Description: "Container network properly isolated",
        Severity:    "INFO",
    }
}
```

**5.12 - Ensure that the container's root filesystem is mounted as read only**
```go
func (cf *ComplianceFramework) CheckCIS_5_12(config ContainerConfig) ComplianceResult {
    if !config.ReadonlyRootfs {
        return ComplianceResult{
            ControlID:   "CIS-5.12",
            Status:      "FAIL",
            Description: "Root filesystem not mounted read-only",
            Remediation: "Use --read-only flag or set ReadonlyRootfs: true",
            Severity:    "MEDIUM",
        }
    }
    
    return ComplianceResult{
        ControlID:   "CIS-5.12",
        Status:      "PASS",
        Description: "Root filesystem mounted read-only",
        Severity:    "INFO",
    }
}
```

### NIST SP 800-190

NIST Special Publication 800-190 provides application container security guidelines.

#### Implemented Controls

**CM-2 - Baseline Configuration**
```go
func (cf *ComplianceFramework) CheckNIST_CM2(config ContainerConfig) ComplianceResult {
    baselineChecks := []func(ContainerConfig) bool{
        cf.hasNonRootUser,
        cf.hasReadOnlyFilesystem, 
        cf.hasResourceLimits,
        cf.hasSecurityOptions,
    }
    
    passed := 0
    for _, check := range baselineChecks {
        if check(config) {
            passed++
        }
    }
    
    percentage := float64(passed) / float64(len(baselineChecks)) * 100
    
    if percentage < 80 {
        return ComplianceResult{
            ControlID:   "NIST-CM-2",
            Status:      "FAIL",
            Description: fmt.Sprintf("Baseline configuration compliance: %.1f%%", percentage),
            Remediation: "Ensure container configuration meets security baseline",
            Severity:    "MEDIUM",
        }
    }
    
    return ComplianceResult{
        ControlID:   "NIST-CM-2",
        Status:      "PASS",
        Description: fmt.Sprintf("Baseline configuration compliance: %.1f%%", percentage),
        Severity:    "INFO",
    }
}
```

**AC-6 - Least Privilege**
```go
func (cf *ComplianceFramework) CheckNIST_AC6(config ContainerConfig) ComplianceResult {
    violations := []string{}
    
    // Check for root user
    if config.User == "" || config.User == "0" || config.User == "root" {
        violations = append(violations, "Running as root user")
    }
    
    // Check for excessive capabilities
    if len(config.Capabilities) > 3 {
        violations = append(violations, "Too many capabilities granted")
    }
    
    // Check for privileged mode
    if config.Privileged {
        violations = append(violations, "Privileged mode enabled")
    }
    
    if len(violations) > 0 {
        return ComplianceResult{
            ControlID:   "NIST-AC-6",
            Status:      "FAIL",
            Description: fmt.Sprintf("Least privilege violations: %s", strings.Join(violations, "; ")),
            Remediation: "Apply principle of least privilege",
            Severity:    "HIGH",
        }
    }
    
    return ComplianceResult{
        ControlID:   "NIST-AC-6",
        Status:      "PASS",
        Description: "Least privilege principle applied",
        Severity:    "INFO",
    }
}
```

**SC-3 - Security Function Isolation**
```go
func (cf *ComplianceFramework) CheckNIST_SC3(config ContainerConfig) ComplianceResult {
    isolationChecks := map[string]bool{
        "Network isolation":     config.NetworkMode != "host",
        "PID namespace":         !config.SharePidNamespace,
        "IPC isolation":         config.IpcMode != "host",
        "User namespace":        config.UsernsMode != "host",
        "UTS isolation":         config.UtsMode != "host",
    }
    
    failures := []string{}
    for check, passed := range isolationChecks {
        if !passed {
            failures = append(failures, check)
        }
    }
    
    if len(failures) > 0 {
        return ComplianceResult{
            ControlID:   "NIST-SC-3",
            Status:      "FAIL",
            Description: fmt.Sprintf("Isolation failures: %s", strings.Join(failures, "; ")),
            Remediation: "Ensure proper namespace isolation",
            Severity:    "HIGH",
        }
    }
    
    return ComplianceResult{
        ControlID:   "NIST-SC-3",
        Status:      "PASS",
        Description: "Security function isolation implemented",
        Severity:    "INFO",
    }
}
```

## Compliance Framework Implementation

### Core Compliance Engine

```go
type ComplianceFramework struct {
    logger    zerolog.Logger
    checkers  map[string]ComplianceChecker
    policies  map[string]CompliancePolicy
    cache     *ComplianceCache
}

type ComplianceChecker interface {
    CheckCompliance(config interface{}) []ComplianceResult
    GetFramework() string
    GetVersion() string
}

type ComplianceResult struct {
    ControlID     string                 `json:"control_id"`
    Framework     string                 `json:"framework"`
    Status        ComplianceStatus       `json:"status"`
    Description   string                 `json:"description"`
    Remediation   string                 `json:"remediation"`
    Severity      string                 `json:"severity"`
    Evidence      map[string]interface{} `json:"evidence,omitempty"`
    Timestamp     time.Time              `json:"timestamp"`
    CheckDuration time.Duration          `json:"check_duration"`
}

type ComplianceStatus string

const (
    StatusPass      ComplianceStatus = "PASS"
    StatusFail      ComplianceStatus = "FAIL"
    StatusWarning   ComplianceStatus = "WARNING"
    StatusNotApplicable ComplianceStatus = "NOT_APPLICABLE"
    StatusError     ComplianceStatus = "ERROR"
)
```

### CIS Docker Benchmark Checker

```go
type CISDockerChecker struct {
    version string
    logger  zerolog.Logger
}

func NewCISDockerChecker(logger zerolog.Logger) *CISDockerChecker {
    return &CISDockerChecker{
        version: "1.6.0",
        logger:  logger,
    }
}

func (cdc *CISDockerChecker) CheckCompliance(config interface{}) []ComplianceResult {
    containerConfig, ok := config.(ContainerConfig)
    if !ok {
        return []ComplianceResult{{
            ControlID:   "CIS-ERROR",
            Framework:   "CIS Docker",
            Status:      StatusError,
            Description: "Invalid configuration type",
            Timestamp:   time.Now(),
        }}
    }
    
    var results []ComplianceResult
    
    // Run all CIS checks
    checks := []func(ContainerConfig) ComplianceResult{
        cdc.checkCIS_4_1,  // Non-root user
        cdc.checkCIS_4_5,  // Content trust
        cdc.checkCIS_5_3,  // Capabilities
        cdc.checkCIS_5_9,  // Network namespace
        cdc.checkCIS_5_12, // Read-only filesystem
        cdc.checkCIS_5_13, // Memory limits
        cdc.checkCIS_5_14, // CPU limits
    }
    
    for _, check := range checks {
        start := time.Now()
        result := check(containerConfig)
        result.Framework = "CIS Docker v" + cdc.version
        result.Timestamp = time.Now()
        result.CheckDuration = time.Since(start)
        results = append(results, result)
    }
    
    return results
}
```

### NIST SP 800-190 Checker

```go
type NISTChecker struct {
    version string
    logger  zerolog.Logger
}

func NewNISTChecker(logger zerolog.Logger) *NISTChecker {
    return &NISTChecker{
        version: "SP 800-190",
        logger:  logger,
    }
}

func (nc *NISTChecker) CheckCompliance(config interface{}) []ComplianceResult {
    containerConfig, ok := config.(ContainerConfig)
    if !ok {
        return []ComplianceResult{{
            ControlID:   "NIST-ERROR",
            Framework:   "NIST",
            Status:      StatusError,
            Description: "Invalid configuration type",
            Timestamp:   time.Now(),
        }}
    }
    
    var results []ComplianceResult
    
    // Run NIST checks
    checks := []func(ContainerConfig) ComplianceResult{
        nc.checkNIST_CM2, // Baseline configuration
        nc.checkNIST_AC6, // Least privilege
        nc.checkNIST_SC3, // Security function isolation
        nc.checkNIST_SI3, // Malicious code protection
    }
    
    for _, check := range checks {
        start := time.Now()
        result := check(containerConfig)
        result.Framework = "NIST " + nc.version
        result.Timestamp = time.Now()
        result.CheckDuration = time.Since(start)
        results = append(results, result)
    }
    
    return results
}
```

## Compliance Assessment

### Assessment Engine

```go
func (cf *ComplianceFramework) AssessCompliance(ctx context.Context, 
    sessionID string, config ContainerConfig) (*ComplianceAssessment, error) {
    
    cf.logger.Info().Str("session_id", sessionID).Msg("Starting compliance assessment")
    
    assessment := &ComplianceAssessment{
        SessionID:   sessionID,
        Timestamp:   time.Now(),
        Config:      config,
        Results:     make(map[string][]ComplianceResult),
        Summary:     ComplianceSummary{},
    }
    
    // Run all registered checkers
    for framework, checker := range cf.checkers {
        start := time.Now()
        results := checker.CheckCompliance(config)
        assessment.Results[framework] = results
        
        cf.logger.Debug().
            Str("framework", framework).
            Int("checks", len(results)).
            Dur("duration", time.Since(start)).
            Msg("Framework assessment completed")
    }
    
    // Generate summary
    assessment.Summary = cf.generateSummary(assessment.Results)
    
    // Calculate overall compliance score
    assessment.OverallScore = cf.calculateComplianceScore(assessment.Results)
    assessment.ComplianceLevel = cf.getComplianceLevel(assessment.OverallScore)
    
    return assessment, nil
}

type ComplianceAssessment struct {
    SessionID       string                            `json:"session_id"`
    Timestamp       time.Time                         `json:"timestamp"`
    Config          ContainerConfig                   `json:"config"`
    Results         map[string][]ComplianceResult     `json:"results"`
    Summary         ComplianceSummary                 `json:"summary"`
    OverallScore    float64                           `json:"overall_score"`
    ComplianceLevel string                            `json:"compliance_level"`
    Recommendations []ComplianceRecommendation        `json:"recommendations"`
}

type ComplianceSummary struct {
    TotalChecks    int                          `json:"total_checks"`
    PassedChecks   int                          `json:"passed_checks"`
    FailedChecks   int                          `json:"failed_checks"`
    WarningChecks  int                          `json:"warning_checks"`
    ErrorChecks    int                          `json:"error_checks"`
    ByFramework    map[string]FrameworkSummary  `json:"by_framework"`
    BySeverity     map[string]int               `json:"by_severity"`
}

type FrameworkSummary struct {
    Framework   string  `json:"framework"`
    Total       int     `json:"total"`
    Passed      int     `json:"passed"`
    Failed      int     `json:"failed"`
    Warnings    int     `json:"warnings"`
    Errors      int     `json:"errors"`
    Score       float64 `json:"score"`
}
```

### Compliance Scoring

```go
func (cf *ComplianceFramework) calculateComplianceScore(results map[string][]ComplianceResult) float64 {
    totalWeight := 0.0
    weightedScore := 0.0
    
    weights := map[string]float64{
        "CIS Docker": 0.4,  // 40% weight
        "NIST":       0.3,  // 30% weight
        "Custom":     0.3,  // 30% weight
    }
    
    for framework, frameworkResults := range results {
        weight := weights[framework]
        if weight == 0 {
            weight = 0.1 // Default weight for unknown frameworks
        }
        
        frameworkScore := cf.calculateFrameworkScore(frameworkResults)
        weightedScore += frameworkScore * weight
        totalWeight += weight
    }
    
    if totalWeight == 0 {
        return 0.0
    }
    
    return weightedScore / totalWeight * 100 // Convert to percentage
}

func (cf *ComplianceFramework) calculateFrameworkScore(results []ComplianceResult) float64 {
    if len(results) == 0 {
        return 0.0
    }
    
    totalScore := 0.0
    totalWeight := 0.0
    
    severityWeights := map[string]float64{
        "CRITICAL": 1.0,
        "HIGH":     0.8,
        "MEDIUM":   0.6,
        "LOW":      0.4,
        "INFO":     0.2,
    }
    
    for _, result := range results {
        weight := severityWeights[result.Severity]
        if weight == 0 {
            weight = 0.5 // Default weight
        }
        
        var score float64
        switch result.Status {
        case StatusPass:
            score = 1.0
        case StatusWarning:
            score = 0.5
        case StatusFail, StatusError:
            score = 0.0
        case StatusNotApplicable:
            continue // Skip non-applicable checks
        }
        
        totalScore += score * weight
        totalWeight += weight
    }
    
    if totalWeight == 0 {
        return 0.0
    }
    
    return totalScore / totalWeight
}

func (cf *ComplianceFramework) getComplianceLevel(score float64) string {
    switch {
    case score >= 95:
        return "EXCELLENT"
    case score >= 85:
        return "GOOD"
    case score >= 75:
        return "ACCEPTABLE"
    case score >= 60:
        return "NEEDS_IMPROVEMENT"
    default:
        return "POOR"
    }
}
```

## Compliance Reporting

### Report Generation

```go
type ComplianceReporter struct {
    logger zerolog.Logger
}

func (cr *ComplianceReporter) GenerateReport(assessment *ComplianceAssessment) (*ComplianceReport, error) {
    report := &ComplianceReport{
        Assessment:      assessment,
        GeneratedAt:     time.Now(),
        ExecutiveSummary: cr.generateExecutiveSummary(assessment),
        DetailedFindings: cr.generateDetailedFindings(assessment),
        Recommendations:  cr.generateRecommendations(assessment),
        ActionPlan:       cr.generateActionPlan(assessment),
    }
    
    return report, nil
}

type ComplianceReport struct {
    Assessment       *ComplianceAssessment     `json:"assessment"`
    GeneratedAt      time.Time                 `json:"generated_at"`
    ExecutiveSummary ExecutiveSummary          `json:"executive_summary"`
    DetailedFindings []DetailedFinding         `json:"detailed_findings"`
    Recommendations  []ComplianceRecommendation `json:"recommendations"`
    ActionPlan       ActionPlan                `json:"action_plan"`
}

type ExecutiveSummary struct {
    OverallCompliance string  `json:"overall_compliance"`
    Score             float64 `json:"score"`
    CriticalIssues    int     `json:"critical_issues"`
    HighIssues        int     `json:"high_issues"`
    KeyFindings       []string `json:"key_findings"`
    TopRecommendations []string `json:"top_recommendations"`
}

func (cr *ComplianceReporter) generateExecutiveSummary(assessment *ComplianceAssessment) ExecutiveSummary {
    summary := ExecutiveSummary{
        OverallCompliance: assessment.ComplianceLevel,
        Score:            assessment.OverallScore,
        CriticalIssues:   assessment.Summary.BySeverity["CRITICAL"],
        HighIssues:       assessment.Summary.BySeverity["HIGH"],
    }
    
    // Generate key findings
    summary.KeyFindings = cr.extractKeyFindings(assessment)
    summary.TopRecommendations = cr.extractTopRecommendations(assessment)
    
    return summary
}
```

### Remediation Guidance

```go
type ComplianceRecommendation struct {
    ID           string    `json:"id"`
    Priority     string    `json:"priority"`     // CRITICAL, HIGH, MEDIUM, LOW
    Category     string    `json:"category"`     // CONFIGURATION, POLICY, PROCESS
    Title        string    `json:"title"`
    Description  string    `json:"description"`
    Remediation  string    `json:"remediation"`
    Impact       string    `json:"impact"`
    Effort       string    `json:"effort"`       // LOW, MEDIUM, HIGH
    ControlIDs   []string  `json:"control_ids"`
    References   []string  `json:"references"`
}

func (cf *ComplianceFramework) generateRecommendations(assessment *ComplianceAssessment) []ComplianceRecommendation {
    var recommendations []ComplianceRecommendation
    
    // Analyze failed checks and generate recommendations
    for framework, results := range assessment.Results {
        for _, result := range results {
            if result.Status == StatusFail {
                rec := cf.createRecommendation(framework, result)
                recommendations = append(recommendations, rec)
            }
        }
    }
    
    // Sort by priority and impact
    sort.Slice(recommendations, func(i, j int) bool {
        return cf.getPriorityWeight(recommendations[i].Priority) > 
               cf.getPriorityWeight(recommendations[j].Priority)
    })
    
    return recommendations
}

func (cf *ComplianceFramework) createRecommendation(framework string, result ComplianceResult) ComplianceRecommendation {
    return ComplianceRecommendation{
        ID:          fmt.Sprintf("REC-%s-%s", framework, result.ControlID),
        Priority:    result.Severity,
        Category:    "CONFIGURATION",
        Title:       fmt.Sprintf("Address %s compliance failure", result.ControlID),
        Description: result.Description,
        Remediation: result.Remediation,
        Impact:      cf.getImpactDescription(result.Severity),
        Effort:      cf.getEffortEstimate(result.ControlID),
        ControlIDs:  []string{result.ControlID},
        References:  cf.getControlReferences(framework, result.ControlID),
    }
}
```

## Continuous Compliance Monitoring

### Monitoring Implementation

```go
type ComplianceMonitor struct {
    framework   *ComplianceFramework
    scheduler   *cron.Cron
    alerts      *AlertManager
    logger      zerolog.Logger
}

func NewComplianceMonitor(framework *ComplianceFramework, logger zerolog.Logger) *ComplianceMonitor {
    return &ComplianceMonitor{
        framework: framework,
        scheduler: cron.New(),
        alerts:    NewAlertManager(),
        logger:    logger,
    }
}

func (cm *ComplianceMonitor) StartMonitoring(ctx context.Context) error {
    // Schedule regular compliance checks
    _, err := cm.scheduler.AddFunc("@hourly", func() {
        cm.runScheduledCompliance(ctx)
    })
    if err != nil {
        return fmt.Errorf("failed to schedule compliance monitoring: %w", err)
    }
    
    cm.scheduler.Start()
    cm.logger.Info().Msg("Compliance monitoring started")
    
    return nil
}

func (cm *ComplianceMonitor) runScheduledCompliance(ctx context.Context) {
    cm.logger.Info().Msg("Running scheduled compliance check")
    
    // Get active sessions
    sessions := cm.getActiveSessions()
    
    for _, session := range sessions {
        assessment, err := cm.framework.AssessCompliance(ctx, session.ID, session.Config)
        if err != nil {
            cm.logger.Error().Err(err).Str("session_id", session.ID).Msg("Compliance assessment failed")
            continue
        }
        
        // Check for compliance violations
        if assessment.OverallScore < 75 { // Threshold for alerting
            cm.alerts.SendComplianceAlert(session.ID, assessment)
        }
        
        // Store assessment results
        cm.storeAssessment(assessment)
    }
}
```

### Compliance Dashboard

```go
type ComplianceDashboard struct {
    assessments []ComplianceAssessment
    metrics     ComplianceMetrics
    trends      ComplianceTrends
}

type ComplianceMetrics struct {
    TotalAssessments    int                    `json:"total_assessments"`
    AverageScore        float64                `json:"average_score"`
    ComplianceTrend     string                 `json:"compliance_trend"` // IMPROVING, STABLE, DECLINING
    FrameworkScores     map[string]float64     `json:"framework_scores"`
    ViolationsByControl map[string]int         `json:"violations_by_control"`
    TopViolations       []ControlViolation     `json:"top_violations"`
}

type ControlViolation struct {
    ControlID   string `json:"control_id"`
    Framework   string `json:"framework"`
    Count       int    `json:"count"`
    Severity    string `json:"severity"`
    Description string `json:"description"`
}
```

## Testing Framework

### Compliance Testing

```go
func TestComplianceFramework(t *testing.T) {
    logger := zerolog.New(os.Stdout)
    framework := NewComplianceFramework(logger)
    
    // Test secure configuration
    secureConfig := ContainerConfig{
        User:           "1000",
        ReadonlyRootfs: true,
        NetworkMode:    "none",
        Capabilities:   []string{},
        SecurityOpt:    []string{"no-new-privileges:true"},
    }
    
    assessment, err := framework.AssessCompliance(context.Background(), "test-session", secureConfig)
    assert.NoError(t, err)
    assert.True(t, assessment.OverallScore > 80)
    assert.Equal(t, "GOOD", assessment.ComplianceLevel)
    
    // Test insecure configuration
    insecureConfig := ContainerConfig{
        User:           "root",
        ReadonlyRootfs: false,
        NetworkMode:    "host",
        Capabilities:   []string{"CAP_SYS_ADMIN"},
        Privileged:     true,
    }
    
    assessment, err = framework.AssessCompliance(context.Background(), "test-session-insecure", insecureConfig)
    assert.NoError(t, err)
    assert.True(t, assessment.OverallScore < 50)
    assert.Contains(t, []string{"POOR", "NEEDS_IMPROVEMENT"}, assessment.ComplianceLevel)
}
```

## Integration with Security Validation

### Unified Compliance Checking

```go
func (sv *SecurityValidator) ValidateSecurityWithCompliance(ctx context.Context,
    sessionID string, options AdvancedSandboxOptions) (*SecurityValidationReport, error) {
    
    // Run standard security validation
    report, err := sv.ValidateSecurity(ctx, sessionID, options)
    if err != nil {
        return nil, err
    }
    
    // Add compliance assessment
    config := sv.buildContainerConfig(options)
    compliance, err := sv.complianceFramework.AssessCompliance(ctx, sessionID, config)
    if err != nil {
        sv.logger.Error().Err(err).Msg("Compliance assessment failed")
    } else {
        report.ComplianceStatus = ComplianceStatus{
            OverallScore:    compliance.OverallScore,
            ComplianceLevel: compliance.ComplianceLevel,
            FrameworkResults: compliance.Summary.ByFramework,
            Violations:      sv.extractViolations(compliance),
        }
    }
    
    return report, nil
}

type ComplianceStatus struct {
    OverallScore     float64                         `json:"overall_score"`
    ComplianceLevel  string                          `json:"compliance_level"`
    FrameworkResults map[string]FrameworkSummary     `json:"framework_results"`
    Violations       []ComplianceViolation           `json:"violations"`
}

type ComplianceViolation struct {
    ControlID   string `json:"control_id"`
    Framework   string `json:"framework"`
    Severity    string `json:"severity"`
    Description string `json:"description"`
    Remediation string `json:"remediation"`
}
```

## Best Practices

### Compliance Management

1. **Regular Assessment**
   - Schedule automated compliance checks
   - Monitor compliance trends over time
   - Set compliance score thresholds

2. **Remediation Prioritization**
   - Address critical violations first
   - Consider effort vs. impact
   - Track remediation progress

3. **Documentation and Audit**
   - Maintain compliance evidence
   - Document exceptions and waivers
   - Prepare for external audits

### Framework Customization

1. **Custom Controls**
   - Implement organization-specific checks
   - Map to internal security policies
   - Maintain custom control catalog

2. **Risk-based Approach**
   - Weight controls by business impact
   - Adjust thresholds based on environment
   - Consider compensating controls

3. **Integration Points**
   - Connect to SIEM systems
   - Integrate with ticketing systems
   - Export to GRC platforms

## References

- [CIS Docker Benchmark v1.6.0](https://www.cisecurity.org/benchmark/docker)
- [NIST SP 800-190](https://csrc.nist.gov/publications/detail/sp/800-190/final)
- [ISO 27001:2013](https://www.iso.org/standard/54534.html)
- [SOC 2 Compliance](https://www.aicpa.org/interestareas/frc/assuranceadvisoryservices/aicpasoc2report.html)
- [PCI DSS Requirements](https://www.pcisecuritystandards.org/)