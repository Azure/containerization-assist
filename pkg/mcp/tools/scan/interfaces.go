package scan

import (
	"context"
	"time"
)

// ScanService consolidates all scanning operations
// Replaces: ScanEngine, SecurityAnalyzer, SecretScanner, ComplianceReporter
// Replaces: RemediationPlanner, MetricsCollector, ScanEngineExtended
type ScanService interface {
	// Core scanning (was ScanEngine + SecurityAnalyzer)
	ScanImage(ctx context.Context, imageRef string) (*ScanResult, error)
	ScanSecrets(ctx context.Context, path string) (*SecretScanResult, error)
	AnalyzeSecurity(ctx context.Context, imageRef string) (*SecurityAnalysis, error)

	// Analysis and reporting (was ComplianceReporter + RemediationPlanner)
	GenerateComplianceReport(result *ScanResult) (*ComplianceReport, error)
	GetRemediationPlan(vulnerabilities []Vulnerability) (*RemediationPlan, error)

	// Metrics and monitoring (was MetricsCollector)
	GetScanMetrics() (*ScanMetrics, error)

	// Extended functionality (was ScanEngineExtended)
	ScanWithOptions(ctx context.Context, imageRef string, options ScanOptions) (*ScanResult, error)
	ValidateScanResult(result *ScanResult) error
}

// ContainerService handles container operations
// Replaces: DockerClient, SecurityClient
type ContainerService interface {
	PullImage(ctx context.Context, imageRef string) error
	InspectImage(ctx context.Context, imageRef string) (*ImageInfo, error)
	CleanupImage(ctx context.Context, imageRef string) error
	GetSecurityContext(ctx context.Context, imageRef string) (*SecurityContext, error)
}

// Supporting types for the unified interfaces
// ScanResult is defined in types.go

// SecretScanResult is defined in types.go

type SecurityAnalysis struct {
	RiskScore       int
	Vulnerabilities []Vulnerability
	Recommendations []string
	Compliance      ComplianceStatus
	Metadata        map[string]interface{}
}

type ComplianceReport struct {
	Status          ComplianceStatus
	Violations      []ComplianceViolation
	Recommendations []string
	Score           int
	Timestamp       time.Time
}

type RemediationPlan struct {
	Actions     []RemediationAction
	Priority    string
	Effort      string
	Description string
	Metadata    map[string]interface{}
}

type ScanMetrics struct {
	TotalScans      int64
	SuccessfulScans int64
	FailedScans     int64
	AverageDuration time.Duration
	LastScanTime    time.Time
	Metadata        map[string]interface{}
}

// ScanOptions is defined in types.go

type ImageInfo struct {
	ID           string
	Tags         []string
	Size         int64
	Created      time.Time
	Architecture string
	OS           string
	Metadata     map[string]interface{}
}

type SecurityContext struct {
	User         string
	Privileges   []string
	Capabilities []string
	Metadata     map[string]interface{}
}

// Vulnerability is defined in types.go

// Secret is defined in types.go

type ComplianceStatus struct {
	Compliant  bool
	Score      int
	Violations []ComplianceViolation
	Metadata   map[string]interface{}
}

type ComplianceViolation struct {
	Rule        string
	Severity    string
	Description string
	Remediation string
	Metadata    map[string]interface{}
}

// RemediationAction is defined in scan_security_types.go

// Factory functions to replace the removed factory interfaces
func NewScanService() ScanService {
	// Implementation would be provided by concrete types
	return nil
}

func NewContainerService(dockerEndpoint string) ContainerService {
	// Implementation would be provided by concrete types
	return nil
}

// ScanConfig is defined in types.go
