package scan

import (
	"time"

	coredocker "github.com/Azure/container-kit/pkg/core/docker"
	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
)

// AtomicScanImageSecurityArgs defines arguments for atomic security scanning
type AtomicScanImageSecurityArgs struct {
	types.BaseToolArgs

	// Target image
	ImageName string `json:"image_name" validate:"required,docker_image" description:"Docker image name/tag to scan (e.g., nginx:latest)"`

	// Scanning options
	SeverityThreshold string   `json:"severity_threshold,omitempty" validate:"omitempty,severity" description:"Minimum severity to report (LOW,MEDIUM,HIGH,CRITICAL)"`
	VulnTypes         []string `json:"vuln_types,omitempty" validate:"omitempty,dive,vuln_type" description:"Types of vulnerabilities to scan for (os,library,app)"`
	IncludeFixable    bool     `json:"include_fixable,omitempty" description:"Include only fixable vulnerabilities"`
	MaxResults        int      `json:"max_results,omitempty" validate:"omitempty,min=1,max=10000" description:"Maximum number of vulnerabilities to return"`

	// Output options
	IncludeRemediations bool `json:"include_remediations,omitempty" description:"Include remediation recommendations"`
	GenerateReport      bool `json:"generate_report,omitempty" description:"Generate detailed security report"`
	FailOnCritical      bool `json:"fail_on_critical,omitempty" description:"Fail if critical vulnerabilities found"`
}

// AtomicScanImageSecurityResult represents the result of atomic security scanning
type AtomicScanImageSecurityResult struct {
	types.BaseToolResponse
	core.BaseAIContextResult // Embed AI context methods

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
	Category    string `json:"category"` // image, package, config, deployment
	Priority    string `json:"priority"` // high, medium, low
	Title       string `json:"title"`
	Description string `json:"description"`
	Action      string `json:"action"`
	Impact      string `json:"impact"`
	Effort      string `json:"effort"` // low, medium, high
}

// ComplianceAnalysis tracks security compliance status
type ComplianceAnalysis struct {
	OverallScore float64          `json:"overall_score"` // 0-100
	Framework    string           `json:"framework"`     // CIS, NIST, etc.
	Items        []ComplianceItem `json:"items"`
	Passed       int              `json:"passed"`
	Failed       int              `json:"failed"`
	Skipped      int              `json:"skipped"`
}

// ComplianceItem represents a single compliance check
type ComplianceItem struct {
	CheckID     string `json:"check_id"`
	Title       string `json:"title"`
	Status      string `json:"status"` // pass, fail, skip
	Severity    string `json:"severity"`
	Description string `json:"description"`
	Remediation string `json:"remediation"`
}

// SecurityRemediationPlan provides structured remediation guidance
type SecurityRemediationPlan struct {
	Summary         RemediationSummary       `json:"summary"`
	Priority        string                   `json:"priority"`
	EstimatedTime   string                   `json:"estimated_time"`
	Steps           []RemediationStep        `json:"steps"`
	Actions         []RemediationAction      `json:"actions"`
	BaseImageGuide  *BaseImageGuidance       `json:"base_image_guidance,omitempty"`
	PackageUpdates  map[string]PackageUpdate `json:"package_updates"`
	ConfigFixes     []ConfigFix              `json:"config_fixes"`
	AdditionalNotes string                   `json:"additional_notes"`
}

// RemediationSummary provides high-level remediation metrics
type RemediationSummary struct {
	TotalVulnerabilities   int    `json:"total_vulnerabilities"`
	FixableVulnerabilities int    `json:"fixable_vulnerabilities"`
	CriticalActions        int    `json:"critical_actions"`
	EstimatedEffort        string `json:"estimated_effort"` // low, medium, high
}

// RemediationStep represents a specific step in the remediation process
type RemediationStep struct {
	Priority    string `json:"priority"` // critical, high, medium, low
	Type        string `json:"type"`     // package_upgrade, config_change, etc.
	Description string `json:"description"`
	Command     string `json:"command,omitempty"`
	Impact      string `json:"impact"`
}

// RemediationAction represents a specific remediation step
type RemediationAction struct {
	Step        int    `json:"step"`
	Action      string `json:"action"`
	Description string `json:"description"`
	Command     string `json:"command,omitempty"`
	Risk        string `json:"risk"` // low, medium, high
}

// BaseImageGuidance provides base image recommendations
type BaseImageGuidance struct {
	CurrentImage      string   `json:"current_image"`
	RecommendedImages []string `json:"recommended_images"`
	Rationale         string   `json:"rationale"`
	ImpactAssessment  string   `json:"impact_assessment"`
}

// PackageUpdate represents a package that should be updated
type PackageUpdate struct {
	Package         string `json:"package"`
	CurrentVersion  string `json:"current_version"`
	TargetVersion   string `json:"target_version"`
	VulnCount       int    `json:"vuln_count"`
	SecurityImpact  string `json:"security_impact"`
	BreakingChanges bool   `json:"breaking_changes"`
}

// ConfigFix represents a configuration that should be changed
type ConfigFix struct {
	Area       string `json:"area"` // dockerfile, runtime, deployment
	Issue      string `json:"issue"`
	Fix        string `json:"fix"`
	Impact     string `json:"impact"`
	Complexity string `json:"complexity"` // simple, moderate, complex
}
