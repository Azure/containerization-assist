package scan

import (
	"context"
	"log/slog"

	coredocker "github.com/Azure/container-kit/pkg/core/docker"
	coresecurity "github.com/Azure/container-kit/pkg/core/security"
)

// ============================================================================
// Internal Interfaces for Clean Separation of Concerns
// ============================================================================

// ScanEngine defines the interface for security scanning operations.
// This allows for different scanner implementations (Trivy, Grype, etc.)
// while maintaining a consistent interface for the tool.
type ScanEngine interface {
	// PerformImageScan executes a comprehensive security scan using the configured scanner
	PerformImageScan(ctx context.Context, imageName string, args AtomicScanImageSecurityArgs) (*coredocker.ScanResult, error)

	// PerformBasicAssessment provides a fallback assessment when the primary scanner is unavailable
	PerformBasicAssessment(ctx context.Context, imageName string, args AtomicScanImageSecurityArgs) (*coredocker.ScanResult, error)

	// PerformSecurityScan executes the comprehensive security scanning workflow
	PerformSecurityScan(ctx context.Context, args AtomicScanImageSecurityArgs, reporter interface{}) (*AtomicScanImageSecurityResult, error)

	// Analysis methods
	GenerateVulnerabilitySummary(result *coredocker.ScanResult) VulnerabilityAnalysisSummary
	CalculateSecurityScore(summary *VulnerabilityAnalysisSummary) int
	DetermineRiskLevel(score int, summary *VulnerabilityAnalysisSummary) string
	ExtractCriticalFindings(result *coredocker.ScanResult) []CriticalSecurityFinding
	GenerateRecommendations(result *coredocker.ScanResult, summary *VulnerabilityAnalysisSummary) []SecurityRecommendation
	AnalyzeCompliance(result *coredocker.ScanResult) ComplianceAnalysis
	GenerateRemediationPlan(result *coredocker.ScanResult, summary *VulnerabilityAnalysisSummary) *SecurityRemediationPlan
	GenerateSecurityReport(result *AtomicScanImageSecurityResult) string

	// Helper methods
	CalculateFixableVulns(vulns []coresecurity.Vulnerability) int
	IsVulnerabilityFixable(vuln coresecurity.Vulnerability) bool
	ExtractLayerID(vuln coresecurity.Vulnerability) string
	GenerateAgeAnalysis(vulns []coresecurity.Vulnerability) VulnAgeAnalysis
	GroupVulnerabilitiesByPackage(vulns []coresecurity.Vulnerability) map[string][]coresecurity.Vulnerability
	HasFixableVulnerabilities(vulns []coresecurity.Vulnerability) bool
	GetPriorityFromSeverity(vulns []coresecurity.Vulnerability) string
	GenerateUpgradeCommand(pkg string, vulns []coresecurity.Vulnerability) string
	GetCurrentVersion(vulns []coresecurity.Vulnerability) string
	GetTargetVersion(vulns []coresecurity.Vulnerability) string
	CalculateOverallPriority(summary *VulnerabilityAnalysisSummary) string
	EstimateEffort(steps []RemediationStep) string
}

// SecurityAnalyzer defines the interface for vulnerability analysis and scoring.
// This separates the analysis logic from scanning, allowing for different
// scoring algorithms and analysis approaches.
type SecurityAnalyzer interface {
	// GenerateVulnerabilitySummary creates a comprehensive summary of vulnerabilities
	GenerateVulnerabilitySummary(result *coredocker.ScanResult) VulnerabilityAnalysisSummary

	// CalculateSecurityScore computes a risk-based security score (0-100)
	CalculateSecurityScore(summary *VulnerabilityAnalysisSummary) int

	// ExtractCriticalFindings identifies the most critical security issues
	ExtractCriticalFindings(result *coredocker.ScanResult) []CriticalSecurityFinding

	// GenerateRecommendations provides actionable security recommendations
	GenerateRecommendations(result *coredocker.ScanResult, summary *VulnerabilityAnalysisSummary) []SecurityRecommendation
}

// ComplianceReporter defines the interface for compliance checking and reporting.
// This allows for different compliance frameworks and report formats.
type ComplianceReporter interface {
	// AnalyzeCompliance evaluates results against compliance benchmarks
	AnalyzeCompliance(result *coredocker.ScanResult) ComplianceAnalysis

	// GenerateSecurityReport creates a formatted security report
	GenerateSecurityReport(result *AtomicScanImageSecurityResult) string

	// UpdateSessionWithSecurityResults persists scan results to the session
	UpdateSessionWithSecurityResults(sessionID string, result *AtomicScanImageSecurityResult) error
}

// RemediationPlanner defines the interface for remediation strategy generation.
// This separates remediation logic from analysis, enabling different
// remediation approaches and prioritization strategies.
type RemediationPlanner interface {
	// GenerateRemediationPlan creates a comprehensive plan for addressing vulnerabilities
	GenerateRemediationPlan(result *coredocker.ScanResult, summary *VulnerabilityAnalysisSummary) *SecurityRemediationPlan

	// PrioritizeRemediations orders remediation actions by risk and effort
	PrioritizeRemediations(plan *SecurityRemediationPlan) []RemediationStep

	// EstimateRemediationEffort calculates the effort required for remediation
	EstimateRemediationEffort(plan *SecurityRemediationPlan) string
}

// MetricsCollector defines the interface for metrics collection.
// This allows for different metrics backends while maintaining consistent
// instrumentation throughout the tool.
type MetricsCollector interface {
	// RecordScanDuration tracks the time taken for security scans
	RecordScanDuration(duration float64)

	// RecordScanSuccess tracks scan success/failure rates
	RecordScanSuccess(success bool)

	// RecordVulnerabilityCount tracks vulnerabilities by severity
	RecordVulnerabilityCount(severity string, count int)

	// RecordComplianceScore tracks compliance scores
	RecordComplianceScore(score float64)

	// RecordSecurityRiskLevel tracks overall risk levels
	RecordSecurityRiskLevel(level string)
}

// ============================================================================
// Component Factory Interfaces
// ============================================================================

// ScanEngineFactory creates scanner instances with proper configuration
type ScanEngineFactory interface {
	CreateScanEngine(logger *slog.Logger) ScanEngine
}

// AnalyzerFactory creates analyzer instances
type AnalyzerFactory interface {
	CreateAnalyzer(logger *slog.Logger) SecurityAnalyzer
}

// ReporterFactory creates reporter instances
type ReporterFactory interface {
	CreateReporter(logger *slog.Logger) ComplianceReporter
}

// RemediationFactory creates remediation planner instances
type RemediationFactory interface {
	CreatePlanner(logger *slog.Logger) RemediationPlanner
}

// ============================================================================
// Composite Interface for Main Tool
// ============================================================================

// SecurityScanComponents aggregates all components needed by the main tool.
// This provides a clean way to inject all dependencies while maintaining
// separation of concerns.
type SecurityScanComponents struct {
	Engine   ScanEngine
	Analyzer SecurityAnalyzer
	Reporter ComplianceReporter
	Planner  RemediationPlanner
	Metrics  MetricsCollector
}
