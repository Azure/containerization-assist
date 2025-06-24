package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	coredocker "github.com/Azure/container-copilot/pkg/core/docker"
	"github.com/Azure/container-copilot/pkg/mcp/internal/api/contract"
	"github.com/Azure/container-copilot/pkg/mcp/internal/constants"
	"github.com/Azure/container-copilot/pkg/mcp/internal/interfaces"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	sessiontypes "github.com/Azure/container-copilot/pkg/mcp/internal/types/session"
	"github.com/localrivet/gomcp/server"
	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
	"github.com/rs/zerolog"
)

// AtomicScanImageSecurityArgs defines arguments for atomic security scanning
type AtomicScanImageSecurityArgs struct {
	types.BaseToolArgs

	// Target image
	ImageName string `json:"image_name" description:"Docker image name/tag to scan (e.g., nginx:latest)"`

	// Scanning options
	SeverityThreshold string   `json:"severity_threshold,omitempty" description:"Minimum severity to report (LOW,MEDIUM,HIGH,CRITICAL)"`
	VulnTypes         []string `json:"vuln_types,omitempty" description:"Types of vulnerabilities to scan for (os,library,app)"`
	IncludeFixable    bool     `json:"include_fixable,omitempty" description:"Include only fixable vulnerabilities"`
	MaxResults        int      `json:"max_results,omitempty" description:"Maximum number of vulnerabilities to return"`

	// Output options
	IncludeRemediations bool `json:"include_remediations,omitempty" description:"Include remediation recommendations"`
	GenerateReport      bool `json:"generate_report,omitempty" description:"Generate detailed security report"`
	FailOnCritical      bool `json:"fail_on_critical,omitempty" description:"Fail if critical vulnerabilities found"`
}

// AtomicScanImageSecurityResult represents the result of atomic security scanning
type AtomicScanImageSecurityResult struct {
	types.BaseToolResponse
	BaseAIContextResult // Embed AI context methods

	// Scan metadata
	SessionID string        `json:"session_id"`
	ImageName string        `json:"image_name"`
	ScanTime  time.Time     `json:"scan_time"`
	Duration  time.Duration `json:"duration"`
	Scanner   string        `json:"scanner"` // trivy, basic, etc.

	// Scan results
	Success       bool                         `json:"success"`
	SecurityScore int                          `json:"security_score"` // 0-100
	RiskLevel     string                       `json:"risk_level"`     // low, medium, high, critical
	ScanResult    *coredocker.ScanResult       `json:"scan_result"`
	VulnSummary   VulnerabilityAnalysisSummary `json:"vulnerability_summary"`

	// Analysis results
	CriticalFindings []CriticalSecurityFinding `json:"critical_findings"`
	Recommendations  []SecurityRecommendation  `json:"recommendations"`
	ComplianceStatus ComplianceAnalysis        `json:"compliance_status"`

	// Remediation
	RemediationPlan *SecurityRemediationPlan `json:"remediation_plan,omitempty"`
	GeneratedReport string                   `json:"generated_report,omitempty"`

	// Context and debugging
	ScanContext map[string]interface{} `json:"scan_context"`
}

// VulnerabilityAnalysisSummary provides enhanced vulnerability analysis
type VulnerabilityAnalysisSummary struct {
	TotalVulnerabilities   int             `json:"total_vulnerabilities"`
	FixableVulnerabilities int             `json:"fixable_vulnerabilities"`
	SeverityBreakdown      map[string]int  `json:"severity_breakdown"`
	PackageBreakdown       map[string]int  `json:"package_breakdown"`
	LayerBreakdown         map[string]int  `json:"layer_breakdown"`
	AgeAnalysis            VulnAgeAnalysis `json:"age_analysis"`
}

// VulnAgeAnalysis analyzes vulnerability age patterns
type VulnAgeAnalysis struct {
	RecentVulns  int `json:"recent_vulns"`  // < 30 days
	OlderVulns   int `json:"older_vulns"`   // > 30 days
	AncientVulns int `json:"ancient_vulns"` // > 1 year
}

// CriticalSecurityFinding represents a high-priority security issue
type CriticalSecurityFinding struct {
	Type            string   `json:"type"`     // vulnerability, malware, configuration
	Severity        string   `json:"severity"` // critical, high
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	Impact          string   `json:"impact"`
	AffectedPackage string   `json:"affected_package"`
	FixAvailable    bool     `json:"fix_available"`
	CVEReferences   []string `json:"cve_references"`
	Remediation     string   `json:"remediation"`
}

// SecurityRecommendation provides actionable security guidance
type SecurityRecommendation struct {
	Priority    int    `json:"priority"` // 1-5 (1 highest)
	Category    string `json:"category"` // base_image, packages, configuration, best_practices
	Title       string `json:"title"`
	Description string `json:"description"`
	Action      string `json:"action"`
	Impact      string `json:"impact"`
	Effort      string `json:"effort"` // low, medium, high
}

// ComplianceAnalysis assesses security compliance
type ComplianceAnalysis struct {
	OverallScore      int                       `json:"overall_score"`    // 0-100
	ComplianceLevel   string                    `json:"compliance_level"` // excellent, good, fair, poor
	Standards         map[string]ComplianceItem `json:"standards"`
	NonCompliantItems []string                  `json:"non_compliant_items"`
}

// ComplianceItem represents compliance with a security standard
type ComplianceItem struct {
	Standard    string `json:"standard"` // CIS, NIST, etc.
	Score       int    `json:"score"`    // 0-100
	Status      string `json:"status"`   // compliant, non_compliant, warning
	Details     string `json:"details"`
	Remediation string `json:"remediation"`
}

// SecurityRemediationPlan provides comprehensive remediation guidance
type SecurityRemediationPlan struct {
	ImmediateActions   []RemediationAction `json:"immediate_actions"`
	ShortTermActions   []RemediationAction `json:"short_term_actions"`
	LongTermActions    []RemediationAction `json:"long_term_actions"`
	BaseImageUpgrade   *BaseImageGuidance  `json:"base_image_upgrade,omitempty"`
	PackageUpdates     []PackageUpdate     `json:"package_updates"`
	ConfigurationFixes []ConfigFix         `json:"configuration_fixes"`
}

// RemediationAction represents a specific remediation step
type RemediationAction struct {
	Priority    int    `json:"priority"`
	Action      string `json:"action"`
	Description string `json:"description"`
	Command     string `json:"command,omitempty"`
	Expected    string `json:"expected"`
	Validation  string `json:"validation,omitempty"`
}

// BaseImageGuidance provides base image upgrade recommendations
type BaseImageGuidance struct {
	CurrentImage      string   `json:"current_image"`
	RecommendedImages []string `json:"recommended_images"`
	Rationale         string   `json:"rationale"`
	RiskReduction     string   `json:"risk_reduction"`
}

// PackageUpdate represents a package update recommendation
type PackageUpdate struct {
	PackageName    string `json:"package_name"`
	CurrentVersion string `json:"current_version"`
	FixedVersion   string `json:"fixed_version"`
	VulnsFixed     int    `json:"vulns_fixed"`
	UpdateCommand  string `json:"update_command"`
}

// ConfigFix represents a configuration security fix
type ConfigFix struct {
	Issue   string `json:"issue"`
	Fix     string `json:"fix"`
	Command string `json:"command"`
	Impact  string `json:"impact"`
}

// AtomicScanImageSecurityTool implements atomic security scanning
type AtomicScanImageSecurityTool struct {
	pipelineAdapter PipelineOperations
	sessionManager  ToolSessionManager
	logger          zerolog.Logger
}

// NewAtomicScanImageSecurityTool creates a new atomic security scanning tool
func NewAtomicScanImageSecurityTool(adapter PipelineOperations, sessionManager ToolSessionManager, logger zerolog.Logger) *AtomicScanImageSecurityTool {
	return &AtomicScanImageSecurityTool{
		pipelineAdapter: adapter,
		sessionManager:  sessionManager,
		logger:          logger.With().Str("tool", "atomic_scan_image_security").Logger(),
	}
}


// ExecuteScan runs the atomic security scanning
func (t *AtomicScanImageSecurityTool) ExecuteScan(ctx context.Context, args AtomicScanImageSecurityArgs) (*AtomicScanImageSecurityResult, error) {
	// Direct execution without progress tracker
	return t.executeWithoutProgress(ctx, args)
}

// ExecuteWithContext runs the atomic security scan with GoMCP progress tracking
func (t *AtomicScanImageSecurityTool) ExecuteWithContext(serverCtx *server.Context, args AtomicScanImageSecurityArgs) (*AtomicScanImageSecurityResult, error) {
	// Create progress adapter for GoMCP using standard scan stages
	adapter := NewGoMCPProgressAdapter(serverCtx, interfaces.StandardScanStages())

	// Execute with progress tracking
	ctx := context.Background()
	result, err := t.performSecurityScan(ctx, args, adapter)

	// Complete progress tracking
	if err != nil {
		adapter.Complete("Security scan failed")
		if result != nil {
			result.Success = false
		}
		return result, nil // Return result with error info, not the error itself
	} else {
		adapter.Complete("Security scan completed successfully")
	}

	return result, nil
}

// executeWithoutProgress executes without progress tracking
func (t *AtomicScanImageSecurityTool) executeWithoutProgress(ctx context.Context, args AtomicScanImageSecurityArgs) (*AtomicScanImageSecurityResult, error) {
	return t.performSecurityScan(ctx, args, nil)
}

// performSecurityScan performs the actual security scan
func (t *AtomicScanImageSecurityTool) performSecurityScan(ctx context.Context, args AtomicScanImageSecurityArgs, reporter interfaces.ProgressReporter) (*AtomicScanImageSecurityResult, error) {
	startTime := time.Now()

	// Get session
	session, err := t.sessionManager.GetSession(args.SessionID)
	if err != nil {
		result := &AtomicScanImageSecurityResult{
			BaseToolResponse:    types.NewBaseResponse("atomic_scan_image_security", args.SessionID, args.DryRun),
			BaseAIContextResult: NewBaseAIContextResult("scan", false, time.Since(startTime)),
			SessionID:           args.SessionID,
			ImageName:           args.ImageName,
			ScanTime:            startTime,
			Duration:            time.Since(startTime),
			Scanner:             "unavailable",
			RiskLevel:           "unknown",
		}

		t.logger.Error().Err(err).Str("session_id", args.SessionID).Msg("Failed to get session")
		return result, nil
	}

	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("image_name", args.ImageName).
		Str("severity_threshold", args.SeverityThreshold).
		Msg("Starting atomic security scanning")

	// Stage 1: Initialize
	if reporter != nil {
		reporter.ReportStage(0.0, "Initializing security scan")
	}

	// Create base result
	result := &AtomicScanImageSecurityResult{
		BaseToolResponse:    types.NewBaseResponse("atomic_scan_image_security", session.SessionID, args.DryRun),
		BaseAIContextResult: NewBaseAIContextResult("scan", false, 0), // Duration and success will be updated later
		SessionID:           session.SessionID,
		ImageName:           args.ImageName,
		ScanTime:            startTime,
		ScanContext:         make(map[string]interface{}),
	}

	// Default image name from session if not provided
	if args.ImageName == "" {
		if lastBuiltImage, ok := session.Metadata["last_built_image"].(string); ok {
			args.ImageName = lastBuiltImage
			result.ImageName = lastBuiltImage
		} else {
			t.logger.Error().Str("session_id", args.SessionID).Msg("Image name is required and no built image found in session")
			result.Duration = time.Since(startTime)
			return result, nil
		}
	}

	// Handle dry-run
	if args.DryRun {
		result.Scanner = "trivy"
		result.Duration = time.Since(startTime)
		result.ScanContext["dry_run"] = true
		result.ScanContext["would_scan"] = args.ImageName
		result.Recommendations = []SecurityRecommendation{
			{
				Priority:    1,
				Category:    "scanning",
				Title:       "Dry Run - Security Scan",
				Description: fmt.Sprintf("Would scan image %s for security vulnerabilities", args.ImageName),
				Action:      "Run without dry_run flag to perform actual scan",
				Impact:      "Security assessment of container image",
				Effort:      types.SeverityLow,
			},
		}
		return result, nil
	}

	if reporter != nil {
		reporter.ReportStage(0.8, "Scan environment prepared")
	}

	// Stage 2: Pull image if needed
	if reporter != nil {
		reporter.NextStage("Pulling image if needed")
	}

	// Create scanner and validate prerequisites
	scanner := coredocker.NewTrivyScanner(t.logger)
	if !scanner.CheckTrivyInstalled() {
		t.logger.Error().Msg("Trivy scanner not installed")
		result.Duration = time.Since(startTime)
		return result, nil
	}

	result.Scanner = "trivy"

	if reporter != nil {
		reporter.ReportStage(0.5, "Scanner validated")
	}

	// Stage 3: Scan
	if reporter != nil {
		reporter.NextStage("Running security analysis")
		reporter.ReportStage(0.1, "Starting vulnerability scan")
	}

	// Run security scan
	severityThreshold := args.SeverityThreshold
	if severityThreshold == "" {
		severityThreshold = constants.DefaultSeverityThreshold
	}

	// Use VulnTypes if provided, otherwise default to os and library
	vulnTypes := args.VulnTypes
	if len(vulnTypes) == 0 {
		vulnTypes = []string{"os", "library"}
	}

	// Store vulnerability types in scan context for later use
	result.ScanContext["vuln_types"] = vulnTypes

	if reporter != nil {
		reporter.ReportStage(0.3, "Scanning for vulnerabilities")
	}

	scanResult, err := scanner.ScanImage(ctx, args.ImageName, severityThreshold)
	if err != nil {
		t.logger.Error().Err(err).Str("image_name", args.ImageName).Msg("Security scan failed")
		result.Duration = time.Since(startTime)
		return result, nil
	}

	result.ScanResult = scanResult
	result.Duration = time.Since(startTime)

	if reporter != nil {
		reporter.ReportStage(0.9, "Vulnerability scan complete")
	}

	// Stage 4: Analyze
	if reporter != nil {
		reporter.NextStage("Processing scan results")
		reporter.ReportStage(0.1, "Analyzing vulnerabilities")
	}

	// Analyze scan results
	t.analyzeScanResults(result, scanResult)

	if reporter != nil {
		reporter.ReportStage(0.3, "Generating recommendations")
	}

	// Generate security recommendations
	t.generateSecurityRecommendations(result, args)

	if reporter != nil {
		reporter.ReportStage(0.5, "Assessing compliance")
	}

	// Assess compliance
	t.assessCompliance(result)

	if reporter != nil {
		reporter.ReportStage(0.7, "Creating remediation plan")
	}

	// Generate remediation plan if requested
	if args.IncludeRemediations && (result.VulnSummary.TotalVulnerabilities > 0 || len(result.CriticalFindings) > 0) {
		result.RemediationPlan = t.generateRemediationPlan(result)
	}

	if reporter != nil {
		reporter.ReportStage(0.9, "Analysis complete")
	}

	// Stage 5: Report
	if reporter != nil {
		reporter.NextStage("Generating security report")
	}

	// Generate report if requested
	if args.GenerateReport {
		if reporter != nil {
			reporter.ReportStage(0.3, "Creating detailed report")
		}
		result.GeneratedReport = t.generateSecurityReport(result)
	}

	if reporter != nil {
		reporter.ReportStage(0.7, "Finalizing results")
	}

	// Determine overall success
	result.Success = t.determineOverallSuccess(result, args)
	result.BaseAIContextResult.IsSuccessful = result.Success
	result.BaseAIContextResult.Duration = result.Duration
	if result.VulnSummary.TotalVulnerabilities > 0 {
		result.BaseAIContextResult.ErrorCount = result.VulnSummary.SeverityBreakdown["CRITICAL"] + result.VulnSummary.SeverityBreakdown["HIGH"]
		result.BaseAIContextResult.WarningCount = result.VulnSummary.SeverityBreakdown["MEDIUM"] + result.VulnSummary.SeverityBreakdown["LOW"]
	}

	// Handle failure scenarios
	if !result.Success && args.FailOnCritical {
		criticalCount := result.VulnSummary.SeverityBreakdown["CRITICAL"]
		if criticalCount > 0 {
			t.logger.Warn().Int("critical_count", criticalCount).Str("image_name", args.ImageName).Msg("Image has critical vulnerabilities")
		}
	}

	// Update session state
	if err := t.updateSessionState(session, result); err != nil {
		t.logger.Warn().Err(err).Msg("Failed to update session state")
	}

	// Log results
	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("image_name", result.ImageName).
		Bool("success", result.Success).
		Int("total_vulns", result.VulnSummary.TotalVulnerabilities).
		Int("critical_findings", len(result.CriticalFindings)).
		Str("risk_level", result.RiskLevel).
		Int("security_score", result.SecurityScore).
		Dur("duration", result.Duration).
		Msg("Security scan completed")

	if reporter != nil {
		reporter.ReportStage(1.0, "Security scan complete")
	}

	return result, nil
}

// AI Context Interface Implementations

// AI Context methods are now provided by embedded BaseAIContextResult
/*

func (r *AtomicScanImageSecurityResult) calculateConfidenceLevel() int {
	confidence := 80 // Base confidence for security scans

	if r.Success {
		confidence += 15
	} else {
		confidence -= 30
	}

	// Higher confidence with detailed scan results
	if r.ScanResult != nil && len(r.ScanResult.Vulnerabilities) > 0 {
		confidence += 5
	}

	// Ensure bounds
	if confidence > 100 {
		confidence = 100
	}
	if confidence < 0 {
		confidence = 0
	}
	return confidence
}

func (r *AtomicScanImageSecurityResult) determineOverallHealth() string {
	score := r.CalculateScore()
	if score >= 80 {
		return types.SeverityExcellent
	} else if score >= 60 {
		return types.SeverityGood
	} else if score >= 40 {
		return "fair"
	} else {
		return types.SeverityPoor
	}
}

func (r *AtomicScanImageSecurityResult) convertStrengthsToAreas() []ai_context.AssessmentArea {
	areas := make([]ai_context.AssessmentArea, 0)
	strengths := r.GetStrengths()

	for i, strength := range strengths {
		areas = append(areas, ai_context.AssessmentArea{
			Area:        fmt.Sprintf("security_strength_%d", i+1),
			Category:    "security",
			Description: strength,
			Impact:      "high",
			Evidence:    []string{strength},
			Score:       85 + (i * 3), // Progressive scoring
		})
	}

	return areas
}

func (r *AtomicScanImageSecurityResult) convertChallengesToAreas() []ai_context.AssessmentArea {
	areas := make([]ai_context.AssessmentArea, 0)
	challenges := r.GetChallenges()

	for i, challenge := range challenges {
		impact := "medium"
		if strings.Contains(strings.ToLower(challenge), "critical") {
			impact = "critical"
		} else if strings.Contains(strings.ToLower(challenge), "high") {
			impact = "high"
		}

		areas = append(areas, ai_context.AssessmentArea{
			Area:        fmt.Sprintf("security_challenge_%d", i+1),
			Category:    "security",
			Description: challenge,
			Impact:      impact,
			Evidence:    []string{challenge},
			Score:       15 + (i * 5), // Lower scores for challenges
		})
	}

	return areas
}

func (r *AtomicScanImageSecurityResult) extractRiskFactors() []ai_context.RiskFactor {
	risks := make([]ai_context.RiskFactor, 0)

	critical := r.VulnSummary.SeverityBreakdown["CRITICAL"]
	high := r.VulnSummary.SeverityBreakdown["HIGH"]

	if critical > 0 {
		risks = append(risks, ai_context.RiskFactor{
			Risk:           "Critical security vulnerabilities",
			Category:       "security",
			Likelihood:     "high",
			Impact:         "critical",
			CurrentLevel:   types.SeverityCritical,
			Mitigation:     "Immediate patching and update deployment",
			PreventionTips: []string{"Regular security scanning", "Automated patch management", "Vulnerability monitoring"},
		})
	}

	if high > 0 {
		risks = append(risks, ai_context.RiskFactor{
			Risk:           "High-severity security issues",
			Category:       "security",
			Likelihood:     "medium",
			Impact:         "high",
			CurrentLevel:   types.SeverityHigh,
			Mitigation:     "Scheduled remediation within SLA",
			PreventionTips: []string{"Regular base image updates", "Dependency management"},
		})
	}

	unfixable := r.VulnSummary.TotalVulnerabilities - r.VulnSummary.FixableVulnerabilities
	if unfixable > 0 && unfixable > r.VulnSummary.TotalVulnerabilities/2 {
		risks = append(risks, ai_context.RiskFactor{
			Risk:           "Many vulnerabilities lack immediate fixes",
			Category:       "maintenance",
			Likelihood:     "medium",
			Impact:         "medium",
			CurrentLevel:   types.SeverityMedium,
			Mitigation:     "Consider alternative base images or workarounds",
			PreventionTips: []string{"Use minimal base images", "Regular security reviews"},
		})
	}

	return risks
}

func (r *AtomicScanImageSecurityResult) extractDecisionFactors() []ai_context.DecisionFactor {
	factors := make([]ai_context.DecisionFactor, 0)

	factors = append(factors, ai_context.DecisionFactor{
		Factor: "vulnerability_severity",
		Weight: 0.4,
		Value: map[string]int{
			"critical": r.VulnSummary.SeverityBreakdown["CRITICAL"],
			"high":     r.VulnSummary.SeverityBreakdown["HIGH"],
			"medium":   r.VulnSummary.SeverityBreakdown["MEDIUM"],
		},
		Reasoning: "Primary factor determining remediation urgency and strategy",
	})

	factors = append(factors, ai_context.DecisionFactor{
		Factor: "fixable_ratio",
		Weight: 0.3,
		Value: func() float64 {
			if r.VulnSummary.TotalVulnerabilities > 0 {
				return float64(r.VulnSummary.FixableVulnerabilities) / float64(r.VulnSummary.TotalVulnerabilities)
			}
			return 1.0
		}(),
		Reasoning: "Influences remediation feasibility and strategy selection",
	})

	factors = append(factors, ai_context.DecisionFactor{
		Factor:    "security_score",
		Weight:    0.2,
		Value:     r.SecurityScore,
		Reasoning: "Overall security posture indicator",
	})

	factors = append(factors, ai_context.DecisionFactor{
		Factor:    "scan_success",
		Weight:    0.1,
		Value:     r.Success,
		Reasoning: "Confidence in scan results and recommendations",
	})

	return factors
}

func (r *AtomicScanImageSecurityResult) buildAssessmentEvidence() []ai_context.EvidenceItem {
	evidence := make([]ai_context.EvidenceItem, 0)

	evidence = append(evidence, ai_context.EvidenceItem{
		Type:        "security_scan",
		Source:      r.Scanner,
		Description: fmt.Sprintf("Scanned %s with %s", r.ImageName, r.Scanner),
		Weight:      0.9,
		Details: map[string]interface{}{
			"total_vulns": r.VulnSummary.TotalVulnerabilities,
			"scan_time":   r.ScanTime,
			"duration":    r.Duration.String(),
		},
	})

	if len(r.CriticalFindings) > 0 {
		evidence = append(evidence, ai_context.EvidenceItem{
			Type:        "critical_findings",
			Source:      "vulnerability_analysis",
			Description: fmt.Sprintf("%d critical security findings identified", len(r.CriticalFindings)),
			Weight:      1.0,
			Details: map[string]interface{}{
				"findings_count": len(r.CriticalFindings),
			},
		})
	}

	return evidence
}

func (r *AtomicScanImageSecurityResult) buildQualityIndicators() map[string]interface{} {
	indicators := make(map[string]interface{})

	indicators["scan_success"] = map[string]interface{}{
		"value":  r.Success,
		"weight": 1.0,
	}

	indicators["security_coverage"] = map[string]interface{}{
		"value": r.SecurityScore,
		"unit":  "score",
		"max":   100,
	}

	if r.VulnSummary.TotalVulnerabilities > 0 {
		indicators["remediation_feasibility"] = map[string]interface{}{
			"value": float64(r.VulnSummary.FixableVulnerabilities) / float64(r.VulnSummary.TotalVulnerabilities),
			"unit":  "ratio",
			"max":   1.0,
		}
	}

	indicators["risk_distribution"] = map[string]interface{}{
		"critical": r.VulnSummary.SeverityBreakdown["CRITICAL"],
		"high":     r.VulnSummary.SeverityBreakdown["HIGH"],
		"medium":   r.VulnSummary.SeverityBreakdown["MEDIUM"],
		"low":      r.VulnSummary.SeverityBreakdown["LOW"],
	}

	return indicators
}

func (r *AtomicScanImageSecurityResult) getRecommendedApproach() string {
	if !r.Success {
		return "Resolve scan issues and retry security analysis"
	}

	critical := r.VulnSummary.SeverityBreakdown["CRITICAL"]
	high := r.VulnSummary.SeverityBreakdown["HIGH"]

	if critical > 0 {
		return "Immediate remediation required - address critical vulnerabilities before deployment"
	} else if high > 5 {
		return "High-priority remediation - address high-severity issues within SLA"
	} else if high > 0 {
		return "Scheduled remediation - plan fixes for high-severity vulnerabilities"
	} else if r.VulnSummary.TotalVulnerabilities > 20 {
		return "Maintenance window - batch fix medium and low severity issues"
	}

	return "Continue with deployment - security posture acceptable"
}

func (r *AtomicScanImageSecurityResult) getNextSteps() []string {
	steps := make([]string, 0)

	if !r.Success {
		steps = append(steps, "Resolve scan failures and retry security analysis")
		return steps
	}

	critical := r.VulnSummary.SeverityBreakdown["CRITICAL"]
	high := r.VulnSummary.SeverityBreakdown["HIGH"]

	if critical > 0 {
		steps = append(steps, "Address critical vulnerabilities immediately")
		steps = append(steps, "Update vulnerable packages and dependencies")
		steps = append(steps, "Rebuild and re-scan image to verify fixes")
		steps = append(steps, "Deploy only after critical issues resolved")
	} else if high > 0 {
		steps = append(steps, "Schedule remediation for high-severity vulnerabilities")
		steps = append(steps, "Plan dependency updates and testing")
		steps = append(steps, "Consider base image updates")
	} else {
		steps = append(steps, "Proceed with deployment")
		steps = append(steps, "Schedule regular security scanning")
		steps = append(steps, "Monitor for new vulnerabilities")
	}

	return steps
}

func (r *AtomicScanImageSecurityResult) getConsiderationsNote() string {
	considerations := make([]string, 0)

	if !r.Success {
		return "Security scan failed - ensure image is accessible and scanner is properly configured"
	}

	critical := r.VulnSummary.SeverityBreakdown["CRITICAL"]
	high := r.VulnSummary.SeverityBreakdown["HIGH"]
	unfixable := r.VulnSummary.TotalVulnerabilities - r.VulnSummary.FixableVulnerabilities

	if critical > 0 {
		considerations = append(considerations, "critical vulnerabilities present")
	}
	if high > 5 {
		considerations = append(considerations, "many high-severity issues")
	}
	if unfixable > r.VulnSummary.TotalVulnerabilities/2 {
		considerations = append(considerations, "limited fix availability")
	}
	if r.SecurityScore < 50 {
		considerations = append(considerations, "overall security score concerning")
	}

	if len(considerations) > 0 {
		return fmt.Sprintf("Security concerns: %s", strings.Join(considerations, ", "))
	}

	return "Security scan complete - review findings and plan appropriate actions"
}
*/

// min helper function is defined in pull_image_atomic.go

// analyzeScanResults analyzes the scan results and populates summary data
func (t *AtomicScanImageSecurityTool) analyzeScanResults(result *AtomicScanImageSecurityResult, scanResult *coredocker.ScanResult) {
	// Initialize vulnerability summary
	result.VulnSummary = VulnerabilityAnalysisSummary{
		SeverityBreakdown: make(map[string]int),
		PackageBreakdown:  make(map[string]int),
		LayerBreakdown:    make(map[string]int),
	}

	// Count vulnerabilities by severity
	for _, vuln := range scanResult.Vulnerabilities {
		result.VulnSummary.TotalVulnerabilities++
		result.VulnSummary.SeverityBreakdown[vuln.Severity]++

		if vuln.FixedVersion != "" {
			result.VulnSummary.FixableVulnerabilities++
		}

		// Track package breakdown
		if vuln.PkgName != "" {
			result.VulnSummary.PackageBreakdown[vuln.PkgName]++
		}
	}

	// Calculate security score based on vulnerability count and severity
	result.SecurityScore = t.calculateSecurityScore(result.VulnSummary)

	// Determine risk level
	if result.VulnSummary.SeverityBreakdown["CRITICAL"] > 0 {
		result.RiskLevel = "critical"
	} else if result.VulnSummary.SeverityBreakdown["HIGH"] > 0 {
		result.RiskLevel = "high"
	} else if result.VulnSummary.SeverityBreakdown["MEDIUM"] > 5 {
		result.RiskLevel = "medium"
	} else {
		result.RiskLevel = "low"
	}

	// Extract critical findings
	for _, vuln := range scanResult.Vulnerabilities {
		if vuln.Severity == "CRITICAL" || vuln.Severity == "HIGH" {
			finding := CriticalSecurityFinding{
				Type:            "vulnerability",
				Severity:        vuln.Severity,
				Title:           vuln.VulnerabilityID,
				Description:     vuln.Description,
				Impact:          fmt.Sprintf("Affects %s version %s", vuln.PkgName, vuln.InstalledVersion),
				AffectedPackage: vuln.PkgName,
				FixAvailable:    vuln.FixedVersion != "",
				CVEReferences:   []string{vuln.VulnerabilityID},
			}

			if vuln.FixedVersion != "" {
				finding.Remediation = fmt.Sprintf("Update %s to version %s", vuln.PkgName, vuln.FixedVersion)
			}

			result.CriticalFindings = append(result.CriticalFindings, finding)
		}
	}

	// Analyze vulnerability age
	result.VulnSummary.AgeAnalysis = VulnAgeAnalysis{
		RecentVulns:  0, // Would need published date info from scanner
		OlderVulns:   0,
		AncientVulns: 0,
	}
}

// generateSecurityRecommendations generates security recommendations based on scan results
func (t *AtomicScanImageSecurityTool) generateSecurityRecommendations(result *AtomicScanImageSecurityResult, args AtomicScanImageSecurityArgs) {
	recommendations := make([]SecurityRecommendation, 0)

	// Critical vulnerability recommendations
	if result.VulnSummary.SeverityBreakdown["CRITICAL"] > 0 {
		recommendations = append(recommendations, SecurityRecommendation{
			Priority:    1,
			Category:    "vulnerability",
			Title:       "Fix Critical Security Vulnerabilities",
			Description: fmt.Sprintf("Image contains %d critical vulnerabilities that require immediate attention", result.VulnSummary.SeverityBreakdown["CRITICAL"]),
			Action:      "Update affected packages to patched versions",
			Impact:      "Eliminates critical security risks",
			Effort:      "high",
		})
	}

	// High severity recommendations
	if result.VulnSummary.SeverityBreakdown["HIGH"] > 0 {
		recommendations = append(recommendations, SecurityRecommendation{
			Priority:    2,
			Category:    "vulnerability",
			Title:       "Address High-Severity Vulnerabilities",
			Description: fmt.Sprintf("Image contains %d high-severity vulnerabilities", result.VulnSummary.SeverityBreakdown["HIGH"]),
			Action:      "Plan remediation for high-severity issues",
			Impact:      "Significantly reduces attack surface",
			Effort:      "medium",
		})
	}

	// Base image recommendations
	if result.VulnSummary.TotalVulnerabilities > 20 {
		recommendations = append(recommendations, SecurityRecommendation{
			Priority:    3,
			Category:    "base_image",
			Title:       "Consider Alternative Base Image",
			Description: "High vulnerability count suggests outdated base image",
			Action:      "Update to latest base image or consider minimal alternatives",
			Impact:      "Reduces overall vulnerability count",
			Effort:      "medium",
		})
	}

	// Package update recommendations
	if result.VulnSummary.FixableVulnerabilities > 0 {
		fixRatio := float64(result.VulnSummary.FixableVulnerabilities) / float64(result.VulnSummary.TotalVulnerabilities)
		if fixRatio > 0.5 {
			recommendations = append(recommendations, SecurityRecommendation{
				Priority:    4,
				Category:    "packages",
				Title:       "Update Vulnerable Packages",
				Description: fmt.Sprintf("%d vulnerabilities have fixes available", result.VulnSummary.FixableVulnerabilities),
				Action:      "Run package updates to apply available security patches",
				Impact:      "Reduces vulnerability count by fixing known issues",
				Effort:      "low",
			})
		}
	}

	// Best practices recommendations
	recommendations = append(recommendations, SecurityRecommendation{
		Priority:    5,
		Category:    "best_practices",
		Title:       "Implement Security Scanning in CI/CD",
		Description: "Automate security scanning to catch vulnerabilities early",
		Action:      "Add security scanning to build pipeline",
		Impact:      "Prevents vulnerable images from reaching production",
		Effort:      "medium",
	})

	result.Recommendations = recommendations
}

// assessCompliance assesses security compliance
func (t *AtomicScanImageSecurityTool) assessCompliance(result *AtomicScanImageSecurityResult) {
	compliance := ComplianceAnalysis{
		Standards:         make(map[string]ComplianceItem),
		NonCompliantItems: make([]string, 0),
	}

	// Calculate overall compliance score
	baseScore := 100
	criticalCount := result.VulnSummary.SeverityBreakdown["CRITICAL"]
	highCount := result.VulnSummary.SeverityBreakdown["HIGH"]

	// Deduct points for vulnerabilities
	baseScore -= criticalCount * 20
	baseScore -= highCount * 10
	baseScore -= result.VulnSummary.SeverityBreakdown["MEDIUM"] * 2

	if baseScore < 0 {
		baseScore = 0
	}

	compliance.OverallScore = baseScore

	// Determine compliance level
	if baseScore >= 90 {
		compliance.ComplianceLevel = "excellent"
	} else if baseScore >= 70 {
		compliance.ComplianceLevel = "good"
	} else if baseScore >= 50 {
		compliance.ComplianceLevel = "fair"
	} else {
		compliance.ComplianceLevel = "poor"
	}

	// Check against common standards
	// CIS Benchmark compliance
	cisScore := 100
	if criticalCount > 0 {
		cisScore = 20
		compliance.NonCompliantItems = append(compliance.NonCompliantItems, "Critical vulnerabilities violate CIS security benchmarks")
	} else if highCount > 5 {
		cisScore = 60
		compliance.NonCompliantItems = append(compliance.NonCompliantItems, "High vulnerability count exceeds CIS recommended thresholds")
	}

	compliance.Standards["CIS"] = ComplianceItem{
		Standard: "CIS Docker Benchmark",
		Score:    cisScore,
		Status: func() string {
			if cisScore >= 70 {
				return "compliant"
			} else {
				return "non_compliant"
			}
		}(),
		Details:     fmt.Sprintf("Security vulnerability assessment score: %d/100", cisScore),
		Remediation: "Address critical and high-severity vulnerabilities to meet CIS standards",
	}

	// NIST compliance
	nistScore := baseScore
	compliance.Standards["NIST"] = ComplianceItem{
		Standard: "NIST Cybersecurity Framework",
		Score:    nistScore,
		Status: func() string {
			if nistScore >= 70 {
				return "compliant"
			} else if nistScore >= 50 {
				return "warning"
			} else {
				return "non_compliant"
			}
		}(),
		Details:     "Vulnerability management and risk assessment",
		Remediation: "Implement vulnerability remediation plan to align with NIST guidelines",
	}

	result.ComplianceStatus = compliance
}

// generateRemediationPlan generates a comprehensive remediation plan
func (t *AtomicScanImageSecurityTool) generateRemediationPlan(result *AtomicScanImageSecurityResult) *SecurityRemediationPlan {
	plan := &SecurityRemediationPlan{
		ImmediateActions:   make([]RemediationAction, 0),
		ShortTermActions:   make([]RemediationAction, 0),
		LongTermActions:    make([]RemediationAction, 0),
		PackageUpdates:     make([]PackageUpdate, 0),
		ConfigurationFixes: make([]ConfigFix, 0),
	}

	// Immediate actions for critical vulnerabilities
	if result.VulnSummary.SeverityBreakdown["CRITICAL"] > 0 {
		plan.ImmediateActions = append(plan.ImmediateActions, RemediationAction{
			Priority:    1,
			Action:      "Fix critical vulnerabilities",
			Description: "Update packages with critical security vulnerabilities",
			Command:     "apt-get update && apt-get upgrade -y",
			Expected:    "Critical vulnerabilities patched",
			Validation:  "Re-run security scan to verify fixes",
		})
	}

	// Short-term actions for high vulnerabilities
	if result.VulnSummary.SeverityBreakdown["HIGH"] > 0 {
		plan.ShortTermActions = append(plan.ShortTermActions, RemediationAction{
			Priority:    2,
			Action:      "Address high-severity issues",
			Description: "Plan and execute high-severity vulnerability remediation",
			Command:     "Review and update vulnerable packages",
			Expected:    "High-severity vulnerabilities reduced",
			Validation:  "Security scan shows reduced high-severity count",
		})
	}

	// Long-term actions
	plan.LongTermActions = append(plan.LongTermActions, RemediationAction{
		Priority:    3,
		Action:      "Implement automated security scanning",
		Description: "Add security scanning to CI/CD pipeline",
		Expected:    "Automated vulnerability detection in place",
		Validation:  "Security scans run on every build",
	})

	// Extract package updates from scan results
	if result.ScanResult != nil {
		packageMap := make(map[string]*PackageUpdate)

		for _, vuln := range result.ScanResult.Vulnerabilities {
			if vuln.FixedVersion != "" {
				key := vuln.PkgName
				if update, exists := packageMap[key]; exists {
					update.VulnsFixed++
				} else {
					packageMap[key] = &PackageUpdate{
						PackageName:    vuln.PkgName,
						CurrentVersion: vuln.InstalledVersion,
						FixedVersion:   vuln.FixedVersion,
						VulnsFixed:     1,
						UpdateCommand:  fmt.Sprintf("Update %s to %s", vuln.PkgName, vuln.FixedVersion),
					}
				}
			}
		}

		for _, update := range packageMap {
			plan.PackageUpdates = append(plan.PackageUpdates, *update)
		}
	}

	// Base image guidance if needed
	if result.VulnSummary.TotalVulnerabilities > 20 {
		plan.BaseImageUpgrade = &BaseImageGuidance{
			CurrentImage:      result.ImageName,
			RecommendedImages: []string{"alpine:latest", "distroless", "ubuntu:22.04"},
			Rationale:         "Current base image contains numerous vulnerabilities",
			RiskReduction:     "Could reduce vulnerability count by 50% or more",
		}
	}

	return plan
}

// generateSecurityReport generates a detailed security report
func (t *AtomicScanImageSecurityTool) generateSecurityReport(result *AtomicScanImageSecurityResult) string {
	var report strings.Builder

	report.WriteString(fmt.Sprintf("# Security Scan Report\n\n"))
	report.WriteString(fmt.Sprintf("**Image:** %s\n", result.ImageName))
	report.WriteString(fmt.Sprintf("**Scan Time:** %s\n", result.ScanTime.Format(time.RFC3339)))
	report.WriteString(fmt.Sprintf("**Scanner:** %s\n", result.Scanner))
	report.WriteString(fmt.Sprintf("**Duration:** %s\n\n", result.Duration))

	report.WriteString("## Executive Summary\n\n")
	report.WriteString(fmt.Sprintf("- **Security Score:** %d/100\n", result.SecurityScore))
	report.WriteString(fmt.Sprintf("- **Risk Level:** %s\n", result.RiskLevel))
	report.WriteString(fmt.Sprintf("- **Total Vulnerabilities:** %d\n", result.VulnSummary.TotalVulnerabilities))
	report.WriteString(fmt.Sprintf("- **Fixable Vulnerabilities:** %d\n\n", result.VulnSummary.FixableVulnerabilities))

	report.WriteString("## Vulnerability Breakdown\n\n")
	report.WriteString("| Severity | Count |\n")
	report.WriteString("|----------|-------|\n")
	for _, severity := range []string{"CRITICAL", "HIGH", "MEDIUM", "LOW"} {
		if count, exists := result.VulnSummary.SeverityBreakdown[severity]; exists {
			report.WriteString(fmt.Sprintf("| %s | %d |\n", severity, count))
		}
	}
	report.WriteString("\n")

	if len(result.CriticalFindings) > 0 {
		report.WriteString("## Critical Findings\n\n")
		for i, finding := range result.CriticalFindings {
			report.WriteString(fmt.Sprintf("%d. **%s** (%s)\n", i+1, finding.Title, finding.Severity))
			report.WriteString(fmt.Sprintf("   - Package: %s\n", finding.AffectedPackage))
			report.WriteString(fmt.Sprintf("   - Description: %s\n", finding.Description))
			if finding.FixAvailable {
				report.WriteString(fmt.Sprintf("   - Fix: %s\n", finding.Remediation))
			}
			report.WriteString("\n")
		}
	}

	if len(result.Recommendations) > 0 {
		report.WriteString("## Recommendations\n\n")
		for _, rec := range result.Recommendations {
			report.WriteString(fmt.Sprintf("### %d. %s\n", rec.Priority, rec.Title))
			report.WriteString(fmt.Sprintf("- **Category:** %s\n", rec.Category))
			report.WriteString(fmt.Sprintf("- **Description:** %s\n", rec.Description))
			report.WriteString(fmt.Sprintf("- **Action:** %s\n", rec.Action))
			report.WriteString(fmt.Sprintf("- **Impact:** %s\n", rec.Impact))
			report.WriteString(fmt.Sprintf("- **Effort:** %s\n\n", rec.Effort))
		}
	}

	if result.ComplianceStatus.OverallScore > 0 {
		report.WriteString("## Compliance Status\n\n")
		report.WriteString(fmt.Sprintf("- **Overall Score:** %d/100\n", result.ComplianceStatus.OverallScore))
		report.WriteString(fmt.Sprintf("- **Compliance Level:** %s\n\n", result.ComplianceStatus.ComplianceLevel))

		if len(result.ComplianceStatus.Standards) > 0 {
			report.WriteString("### Standards Compliance\n\n")
			for name, item := range result.ComplianceStatus.Standards {
				report.WriteString(fmt.Sprintf("- **%s:** %s (Score: %d/100)\n", name, item.Status, item.Score))
			}
			report.WriteString("\n")
		}
	}

	return report.String()
}

// determineOverallSuccess determines if the scan was successful based on criteria
func (t *AtomicScanImageSecurityTool) determineOverallSuccess(result *AtomicScanImageSecurityResult, args AtomicScanImageSecurityArgs) bool {
	// Scan itself must have succeeded
	if result.ScanResult == nil {
		return false
	}

	// If fail_on_critical is set, check for critical vulnerabilities
	if args.FailOnCritical && result.VulnSummary.SeverityBreakdown["CRITICAL"] > 0 {
		return false
	}

	// Otherwise, consider it successful if we got results
	return true
}

// updateSessionState updates the session state with scan results
func (t *AtomicScanImageSecurityTool) updateSessionState(session *sessiontypes.SessionState, result *AtomicScanImageSecurityResult) error {
	// Update session metadata
	if session.Metadata == nil {
		session.Metadata = make(map[string]interface{})
	}

	session.Metadata["last_security_scan"] = map[string]interface{}{
		"image_name":     result.ImageName,
		"scan_time":      result.ScanTime,
		"security_score": result.SecurityScore,
		"risk_level":     result.RiskLevel,
		"total_vulns":    result.VulnSummary.TotalVulnerabilities,
		"critical_vulns": result.VulnSummary.SeverityBreakdown["CRITICAL"],
		"high_vulns":     result.VulnSummary.SeverityBreakdown["HIGH"],
	}

	// Update session state
	session.UpdateLastAccessed()

	// Save session
	return t.sessionManager.UpdateSession(session.SessionID, func(s *sessiontypes.SessionState) {
		*s = *session
	})
}

// calculateSecurityScore calculates a security score based on vulnerabilities
func (t *AtomicScanImageSecurityTool) calculateSecurityScore(summary VulnerabilityAnalysisSummary) int {
	score := 100

	// Deduct points based on severity
	score -= summary.SeverityBreakdown["CRITICAL"] * 20
	score -= summary.SeverityBreakdown["HIGH"] * 10
	score -= summary.SeverityBreakdown["MEDIUM"] * 5
	score -= summary.SeverityBreakdown["LOW"] * 1

	// Bonus for fixable vulnerabilities
	if summary.TotalVulnerabilities > 0 && summary.FixableVulnerabilities > 0 {
		fixRatio := float64(summary.FixableVulnerabilities) / float64(summary.TotalVulnerabilities)
		if fixRatio > 0.8 {
			score += 5
		}
	}

	// Ensure score bounds
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return score
}

// Tool interface implementation (unified interface)

// GetMetadata returns comprehensive tool metadata
func (t *AtomicScanImageSecurityTool) GetMetadata() mcptypes.ToolMetadata {
	return mcptypes.ToolMetadata{
		Name:        "atomic_scan_image_security",
		Description: "Performs comprehensive security vulnerability scanning on Docker images using industry-standard scanners",
		Version:     constants.AtomicToolVersion,
		Category:    "security",
		Dependencies: []string{"docker", "security_scanner"},
		Capabilities: []string{
			"supports_streaming",
			"vulnerability_scanning",
		},
		Requirements: []string{"docker_daemon", "image_available"},
		Parameters: map[string]string{
			"image_name":           "required - Docker image name/tag to scan",
			"severity_threshold":   "optional - Minimum severity to report",
			"vuln_types":           "optional - Types of vulnerabilities to scan",
			"include_fixable":      "optional - Include only fixable vulnerabilities",
			"max_results":          "optional - Maximum number of results",
			"include_remediations": "optional - Include remediation recommendations",
			"generate_report":      "optional - Generate detailed security report",
			"fail_on_critical":     "optional - Fail if critical vulnerabilities found",
		},
		Examples: []mcptypes.ToolExample{
			{
				Name:        "basic_scan",
				Description: "Scan a Docker image for security vulnerabilities",
				Input: map[string]interface{}{
					"session_id":         "session-123",
					"image_name":         "nginx:latest",
					"severity_threshold": "HIGH",
				},
				Output: map[string]interface{}{
					"success":             true,
					"total_vulnerabilities": 5,
					"critical_count":       0,
					"high_count":           2,
				},
			},
		},
	}
}

// Validate validates the tool arguments (unified interface)
func (t *AtomicScanImageSecurityTool) Validate(ctx context.Context, args interface{}) error {
	scanArgs, ok := args.(AtomicScanImageSecurityArgs)
	if !ok {
		return types.NewValidationErrorBuilder("Invalid argument type for atomic_scan_image_security", "args", args).
			WithField("expected", "AtomicScanImageSecurityArgs").
			WithField("received", fmt.Sprintf("%T", args)).
			Build()
	}

	if scanArgs.ImageName == "" {
		return types.NewValidationErrorBuilder("ImageName is required", "image_name", scanArgs.ImageName).
			WithField("field", "image_name").
			Build()
	}

	if scanArgs.SessionID == "" {
		return types.NewValidationErrorBuilder("SessionID is required", "session_id", scanArgs.SessionID).
			WithField("field", "session_id").
			Build()
	}

	// Validate severity threshold if provided
	if scanArgs.SeverityThreshold != "" {
		validSeverities := map[string]bool{
			"LOW": true, "MEDIUM": true, "HIGH": true, "CRITICAL": true,
		}
		if !validSeverities[strings.ToUpper(scanArgs.SeverityThreshold)] {
			return types.NewValidationErrorBuilder("Invalid severity threshold", "severity_threshold", scanArgs.SeverityThreshold).
				WithField("valid_values", "LOW, MEDIUM, HIGH, CRITICAL").
				Build()
		}
	}

	return nil
}

// Execute implements unified Tool interface
func (t *AtomicScanImageSecurityTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	scanArgs, ok := args.(AtomicScanImageSecurityArgs)
	if !ok {
		return nil, types.NewValidationErrorBuilder("Invalid argument type for atomic_scan_image_security", "args", args).
			WithField("expected", "AtomicScanImageSecurityArgs").
			WithField("received", fmt.Sprintf("%T", args)).
			Build()
	}

	// Call the typed Execute method
	return t.ExecuteTyped(ctx, scanArgs)
}

// Legacy interface methods for backward compatibility

// GetName returns the tool name (legacy SimpleTool compatibility)
func (t *AtomicScanImageSecurityTool) GetName() string {
	return t.GetMetadata().Name
}

// GetDescription returns the tool description (legacy SimpleTool compatibility)
func (t *AtomicScanImageSecurityTool) GetDescription() string {
	return t.GetMetadata().Description
}

// GetVersion returns the tool version (legacy SimpleTool compatibility)
func (t *AtomicScanImageSecurityTool) GetVersion() string {
	return t.GetMetadata().Version
}

// GetCapabilities returns the tool capabilities (legacy SimpleTool compatibility)
func (t *AtomicScanImageSecurityTool) GetCapabilities() contract.ToolCapabilities {
	return contract.ToolCapabilities{
		SupportsDryRun:    true,
		SupportsStreaming: true,
		IsLongRunning:     true,
		RequiresAuth:      false,
	}
}

// ExecuteTyped provides the original typed execute method
func (t *AtomicScanImageSecurityTool) ExecuteTyped(ctx context.Context, args AtomicScanImageSecurityArgs) (*AtomicScanImageSecurityResult, error) {
	return t.ExecuteScan(ctx, args)
}
