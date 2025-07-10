package security

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// Service provides a unified interface to security operations
type Service interface {
	// Vulnerability scanning
	ScanImage(ctx context.Context, image string, options ScanOptionsService) (*ScanResult, error)
	ScanDirectory(ctx context.Context, path string, options ScanOptionsService) (*ScanResult, error)
	ScanDockerfile(ctx context.Context, content string, options ScanOptionsService) (*ScanResult, error)

	// Secret detection
	ScanSecrets(ctx context.Context, path string, options SecretScanOptions) (*SecretScanResult, error)
	ValidateSecrets(ctx context.Context, content string, options SecretScanOptions) (*SecretScanResult, error)

	// Policy enforcement
	EvaluatePolicy(ctx context.Context, resource interface{}, policy string) (*PolicyResult, error)
	ValidateCompliance(ctx context.Context, resource interface{}, framework string) (*ComplianceResult, error)

	// Security monitoring
	GetSecurityMetrics(ctx context.Context) (*Metrics, error)
	GetVulnerabilityTrends(ctx context.Context, timeframe string) (*TrendData, error)
	GenerateSecurityReport(ctx context.Context, options ReportOptions) (*Report, error)

	// Configuration
	UpdateScannerConfig(config *ScannerConfig) error
	GetScannerConfig() *ScannerConfig
	GetAvailableScanners() []string
}

// ServiceImpl implements the Security Service interface
type ServiceImpl struct {
	logger       *slog.Logger
	config       *ScannerConfig
	scanHistory  []ScanRecord
	metrics      *Metrics
	policyEngine *PolicyEngine
}

// NewSecurityService creates a new Security service
func NewSecurityService(logger *slog.Logger, config *ScannerConfig) Service {
	if config == nil {
		config = DefaultScannerConfig()
	}

	return &ServiceImpl{
		logger:       logger.With("component", "security_service"),
		config:       config,
		scanHistory:  []ScanRecord{},
		metrics:      &Metrics{},
		policyEngine: &PolicyEngine{},
	}
}

// Supporting types

// ScanOptions contains options for vulnerability scanning
// Note: Uses existing ScanOptions if available, otherwise defines minimal interface
type ScanOptionsService struct {
	Scanners       []string
	Severity       []string
	MaxIssues      int
	Timeout        time.Duration
	SkipUpdate     bool
	SkipFiles      []string
	IncludeSecrets bool
	OutputFormat   string
}

// SecretScanOptions contains options for secret scanning
type SecretScanOptions struct {
	Scanners     []string
	Confidence   float64
	SkipFiles    []string
	MaxSecrets   int
	IncludeTests bool
}

// ReportOptions contains options for security reports
type ReportOptions struct {
	Format         string
	Timeframe      string
	IncludeTrends  bool
	IncludeDetails bool
	Filters        map[string]interface{}
}

// ScanResult contains vulnerability scan results
type ScanResult struct {
	ScanID          string
	Target          string
	ScanType        string
	Scanner         string
	StartTime       time.Time
	EndTime         time.Time
	Duration        time.Duration
	Status          string
	Vulnerabilities []Vulnerability
	Summary         VulnerabilitySummary
	Metadata        map[string]interface{}
}

// SecretScanResult contains secret scan results
type SecretScanResult struct {
	ScanID    string
	Target    string
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
	Status    string
	Secrets   []Secret
	Summary   SecretSummary
	Metadata  map[string]interface{}
}

// PolicyResult contains policy evaluation results
type PolicyResult struct {
	PolicyName  string
	Resource    string
	Passed      bool
	Violations  []PolicyViolation
	Score       int
	Severity    string
	Message     string
	EvaluatedAt time.Time
}

// ComplianceResult contains compliance check results
type ComplianceResult struct {
	Framework   string
	Resource    string
	Passed      bool
	Score       float64
	Controls    []ComplianceControl
	Summary     ComplianceSummary
	EvaluatedAt time.Time
}

// Metrics contains security metrics
type Metrics struct {
	TotalScans           int64
	VulnerabilitiesFound int64
	SecretsFound         int64
	PolicyViolations     int64
	ComplianceScore      float64
	HighRiskIssues       int64
	MediumRiskIssues     int64
	LowRiskIssues        int64
	LastScan             time.Time
	ScanFrequency        time.Duration
}

// TrendData contains trend analysis data
type TrendData struct {
	Timeframe   string
	DataPoints  []TrendPoint
	Summary     TrendSummary
	GeneratedAt time.Time
}

// Report contains comprehensive security report
type Report struct {
	ReportID        string
	GeneratedAt     time.Time
	Timeframe       string
	Overview        ReportOverview
	Vulnerabilities VulnerabilityReport
	Secrets         SecretReport
	Compliance      ComplianceReport
	Trends          TrendReport
	Recommendations []Recommendation
}

// Secret represents a detected secret
type Secret struct {
	ID          string
	Type        string
	Description string
	File        string
	Line        int
	Column      int
	Value       string
	Confidence  float64
	Severity    string
	Remediation string
	References  []string
}

// SecretSummary contains secret summary
type SecretSummary struct {
	Total      int
	HighRisk   int
	MediumRisk int
	LowRisk    int
	ByType     map[string]int
	ByFile     map[string]int
}

// ComplianceControl represents a compliance control
type ComplianceControl struct {
	ID          string
	Title       string
	Description string
	Passed      bool
	Score       float64
	Evidence    []string
	Remediation string
}

// ComplianceSummary contains compliance summary
type ComplianceSummary struct {
	TotalControls  int
	PassedControls int
	FailedControls int
	Score          float64
	Grade          string
}

// ScannerConfig contains scanner configuration
type ScannerConfig struct {
	DefaultScanners    []string
	ScannerSettings    map[string]interface{}
	UpdateFrequency    time.Duration
	DefaultSeverity    []string
	MaxConcurrentScans int
	CacheResults       bool
	CacheTTL           time.Duration
}

// DefaultScannerConfig returns default scanner configuration
func DefaultScannerConfig() *ScannerConfig {
	return &ScannerConfig{
		DefaultScanners:    []string{"trivy", "grype"},
		ScannerSettings:    make(map[string]interface{}),
		UpdateFrequency:    24 * time.Hour,
		DefaultSeverity:    []string{"CRITICAL", "HIGH", "MEDIUM"},
		MaxConcurrentScans: 5,
		CacheResults:       true,
		CacheTTL:           4 * time.Hour,
	}
}

// ScanRecord represents a scan record for history
type ScanRecord struct {
	ScanID    string
	Target    string
	ScanType  string
	Scanner   string
	StartTime time.Time
	EndTime   time.Time
	Status    string
	Results   interface{}
}

// Rule represents a policy rule
type Rule struct {
	ID          string
	Description string
	Expression  string
	Severity    string
}

// Additional supporting types for reports
type TrendPoint struct {
	Timestamp time.Time
	Value     float64
	Label     string
}

type TrendSummary struct {
	Direction  string
	ChangeRate float64
	Prediction float64
}

type ReportOverview struct {
	TotalIssues     int
	CriticalIssues  int
	SecurityScore   float64
	ComplianceScore float64
	TrendDirection  string
}

type SecretReport struct {
	Summary     SecretSummary
	RiskAreas   []RiskArea
	CommonTypes []string
	Trends      []TrendPoint
}

type ComplianceReport struct {
	Summary    ComplianceSummary
	Frameworks []FrameworkScore
	Controls   []ComplianceControl
	Gaps       []ComplianceGap
}

type TrendReport struct {
	VulnerabilityTrends []TrendPoint
	SecretTrends        []TrendPoint
	ComplianceTrends    []TrendPoint
	Predictions         []Prediction
}

type Recommendation struct {
	ID          string
	Title       string
	Description string
	Priority    string
	Impact      string
	Effort      string
	Actions     []string
}

type PackageRisk struct {
	Package            string
	VulnerabilityCount int
	HighestSeverity    string
	Score              float64
}

type RiskArea struct {
	Area        string
	SecretCount int
	RiskLevel   string
	Files       []string
}

type FrameworkScore struct {
	Framework string
	Score     float64
	Grade     string
	Controls  int
}

type ComplianceGap struct {
	Control     string
	Gap         string
	Impact      string
	Remediation string
}

type Prediction struct {
	Metric     string
	Direction  string
	Confidence float64
	Timeframe  string
}

// ScanImage scans a container image for vulnerabilities
func (s *ServiceImpl) ScanImage(_ context.Context, image string, options ScanOptionsService) (*ScanResult, error) {
	s.logger.Info("Scanning container image", "image", image, "scanners", options.Scanners)

	scanID := fmt.Sprintf("img-scan-%d", time.Now().Unix())
	startTime := time.Now()

	// Simulate vulnerability scanning
	vulnerabilities := s.simulateVulnerabilities(image, "image")

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	summary := s.calculateVulnerabilitySummary(vulnerabilities)

	result := &ScanResult{
		ScanID:          scanID,
		Target:          image,
		ScanType:        "image",
		Scanner:         s.selectScanner(options.Scanners),
		StartTime:       startTime,
		EndTime:         endTime,
		Duration:        duration,
		Status:          "completed",
		Vulnerabilities: vulnerabilities,
		Summary:         summary,
		Metadata: map[string]interface{}{
			"image_size": "150MB",
			"layers":     12,
		},
	}

	s.recordScan(scanID, image, "image", result.Scanner, startTime, endTime, "completed", result)
	s.updateMetrics(result)

	s.logger.Info("Image scan completed", "image", image, "vulnerabilities", len(vulnerabilities), "duration", duration.String())
	return result, nil
}

// ScanDirectory scans a directory for vulnerabilities
func (s *ServiceImpl) ScanDirectory(_ context.Context, path string, options ScanOptionsService) (*ScanResult, error) {
	s.logger.Info("Scanning directory", "path", path, "scanners", options.Scanners)

	scanID := fmt.Sprintf("dir-scan-%d", time.Now().Unix())
	startTime := time.Now()

	// Simulate vulnerability scanning
	vulnerabilities := s.simulateVulnerabilities(path, "directory")

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	summary := s.calculateVulnerabilitySummary(vulnerabilities)

	result := &ScanResult{
		ScanID:          scanID,
		Target:          path,
		ScanType:        "directory",
		Scanner:         s.selectScanner(options.Scanners),
		StartTime:       startTime,
		EndTime:         endTime,
		Duration:        duration,
		Status:          "completed",
		Vulnerabilities: vulnerabilities,
		Summary:         summary,
		Metadata: map[string]interface{}{
			"files_scanned": 42,
			"size":          "25MB",
		},
	}

	s.recordScan(scanID, path, "directory", result.Scanner, startTime, endTime, "completed", result)
	s.updateMetrics(result)

	s.logger.Info("Directory scan completed", "path", path, "vulnerabilities", len(vulnerabilities), "duration", duration.String())
	return result, nil
}

// ScanDockerfile scans a Dockerfile for vulnerabilities
func (s *ServiceImpl) ScanDockerfile(_ context.Context, content string, options ScanOptionsService) (*ScanResult, error) {
	s.logger.Info("Scanning Dockerfile", "length", len(content))

	scanID := fmt.Sprintf("dockerfile-scan-%d", time.Now().Unix())
	startTime := time.Now()

	// Simulate Dockerfile scanning
	vulnerabilities := s.simulateDockerfileVulnerabilities(content)

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	summary := s.calculateVulnerabilitySummary(vulnerabilities)

	result := &ScanResult{
		ScanID:          scanID,
		Target:          "Dockerfile",
		ScanType:        "dockerfile",
		Scanner:         s.selectScanner(options.Scanners),
		StartTime:       startTime,
		EndTime:         endTime,
		Duration:        duration,
		Status:          "completed",
		Vulnerabilities: vulnerabilities,
		Summary:         summary,
		Metadata: map[string]interface{}{
			"lines":        strings.Count(content, "\n") + 1,
			"instructions": 20,
		},
	}

	s.recordScan(scanID, "Dockerfile", "dockerfile", result.Scanner, startTime, endTime, "completed", result)
	s.updateMetrics(result)

	s.logger.Info("Dockerfile scan completed", "vulnerabilities", len(vulnerabilities), "duration", duration.String())
	return result, nil
}

// ScanSecrets scans for secrets in a directory
func (s *ServiceImpl) ScanSecrets(_ context.Context, path string, options SecretScanOptions) (*SecretScanResult, error) {
	s.logger.Info("Scanning for secrets", "path", path, "scanners", options.Scanners)

	scanID := fmt.Sprintf("secret-scan-%d", time.Now().Unix())
	startTime := time.Now()

	// Simulate secret scanning
	secrets := s.simulateSecrets(path)

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	summary := s.calculateSecretSummary(secrets)

	result := &SecretScanResult{
		ScanID:    scanID,
		Target:    path,
		StartTime: startTime,
		EndTime:   endTime,
		Duration:  duration,
		Status:    "completed",
		Secrets:   secrets,
		Summary:   summary,
		Metadata: map[string]interface{}{
			"files_scanned": 25,
			"patterns":      150,
		},
	}

	s.logger.Info("Secret scan completed", "path", path, "secrets", len(secrets), "duration", duration.String())
	return result, nil
}

// ValidateSecrets validates content for secrets
func (s *ServiceImpl) ValidateSecrets(_ context.Context, content string, _ SecretScanOptions) (*SecretScanResult, error) {
	s.logger.Info("Validating content for secrets", "length", len(content))

	scanID := fmt.Sprintf("secret-validate-%d", time.Now().Unix())
	startTime := time.Now()

	// Simulate secret validation
	secrets := s.simulateContentSecrets(content)

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	summary := s.calculateSecretSummary(secrets)

	result := &SecretScanResult{
		ScanID:    scanID,
		Target:    "content",
		StartTime: startTime,
		EndTime:   endTime,
		Duration:  duration,
		Status:    "completed",
		Secrets:   secrets,
		Summary:   summary,
		Metadata: map[string]interface{}{
			"content_length": len(content),
			"lines":          strings.Count(content, "\n") + 1,
		},
	}

	s.logger.Info("Secret validation completed", "secrets", len(secrets), "duration", duration.String())
	return result, nil
}

// EvaluatePolicy evaluates a resource against a policy
func (s *ServiceImpl) EvaluatePolicy(_ context.Context, resource interface{}, policy string) (*PolicyResult, error) {
	s.logger.Info("Evaluating policy", "policy", policy)

	// Simulate policy evaluation
	violations := []PolicyViolation{
		{
			RuleID:        "no-root-user",
			Description:   "Container runs as root user",
			Severity:      PolicySeverityHigh,
			Field:         "USER",
			ActualValue:   "root",
			ExpectedValue: "non-root",
			Context: map[string]interface{}{
				"resource":    fmt.Sprintf("%T", resource),
				"location":    "Dockerfile",
				"remediation": "Add USER directive with non-root user",
			},
		},
	}

	result := &PolicyResult{
		PolicyName:  policy,
		Resource:    fmt.Sprintf("%T", resource),
		Passed:      len(violations) == 0,
		Violations:  violations,
		Score:       85,
		Severity:    "HIGH",
		Message:     fmt.Sprintf("Policy evaluation completed with %d violations", len(violations)),
		EvaluatedAt: time.Now(),
	}

	s.logger.Info("Policy evaluation completed", "policy", policy, "passed", result.Passed, "violations", len(violations))
	return result, nil
}

// ValidateCompliance validates compliance against a framework
func (s *ServiceImpl) ValidateCompliance(_ context.Context, resource interface{}, framework string) (*ComplianceResult, error) {
	s.logger.Info("Validating compliance", "framework", framework)

	// Simulate compliance validation
	controls := []ComplianceControl{
		{
			ID:          "CIS-1.1",
			Title:       "Ensure container user is not root",
			Description: "Container should not run as root user",
			Passed:      false,
			Score:       0.0,
			Evidence:    []string{"Dockerfile USER directive missing"},
			Remediation: "Add USER directive with non-root user",
		},
		{
			ID:          "CIS-2.1",
			Title:       "Ensure secrets are not stored in images",
			Description: "No secrets should be embedded in container images",
			Passed:      true,
			Score:       1.0,
			Evidence:    []string{"No secrets detected in image layers"},
			Remediation: "",
		},
	}

	summary := ComplianceSummary{
		TotalControls:  len(controls),
		PassedControls: 1,
		FailedControls: 1,
		Score:          50.0,
		Grade:          "C",
	}

	result := &ComplianceResult{
		Framework:   framework,
		Resource:    fmt.Sprintf("%T", resource),
		Passed:      summary.Score >= 80.0,
		Score:       summary.Score,
		Controls:    controls,
		Summary:     summary,
		EvaluatedAt: time.Now(),
	}

	s.logger.Info("Compliance validation completed", "framework", framework, "score", result.Score, "passed", result.Passed)
	return result, nil
}

// GetSecurityMetrics returns current security metrics
func (s *ServiceImpl) GetSecurityMetrics(_ context.Context) (*Metrics, error) {
	s.logger.Info("Getting security metrics")

	// Update metrics with current data
	metrics := &Metrics{
		TotalScans:           int64(len(s.scanHistory)),
		VulnerabilitiesFound: 150,
		SecretsFound:         5,
		PolicyViolations:     12,
		ComplianceScore:      75.5,
		HighRiskIssues:       8,
		MediumRiskIssues:     25,
		LowRiskIssues:        45,
		LastScan:             time.Now().Add(-2 * time.Hour),
		ScanFrequency:        6 * time.Hour,
	}

	return metrics, nil
}

// GetVulnerabilityTrends returns vulnerability trends
func (s *ServiceImpl) GetVulnerabilityTrends(_ context.Context, timeframe string) (*TrendData, error) {
	s.logger.Info("Getting vulnerability trends", "timeframe", timeframe)

	// Simulate trend data
	dataPoints := []TrendPoint{
		{Timestamp: time.Now().Add(-7 * 24 * time.Hour), Value: 120, Label: "7 days ago"},
		{Timestamp: time.Now().Add(-6 * 24 * time.Hour), Value: 135, Label: "6 days ago"},
		{Timestamp: time.Now().Add(-5 * 24 * time.Hour), Value: 128, Label: "5 days ago"},
		{Timestamp: time.Now().Add(-4 * 24 * time.Hour), Value: 142, Label: "4 days ago"},
		{Timestamp: time.Now().Add(-3 * 24 * time.Hour), Value: 155, Label: "3 days ago"},
		{Timestamp: time.Now().Add(-2 * 24 * time.Hour), Value: 148, Label: "2 days ago"},
		{Timestamp: time.Now().Add(-1 * 24 * time.Hour), Value: 150, Label: "1 day ago"},
	}

	summary := TrendSummary{
		Direction:  "increasing",
		ChangeRate: 25.0,
		Prediction: 165.0,
	}

	trends := &TrendData{
		Timeframe:   timeframe,
		DataPoints:  dataPoints,
		Summary:     summary,
		GeneratedAt: time.Now(),
	}

	return trends, nil
}

// GenerateSecurityReport generates a comprehensive security report
func (s *ServiceImpl) GenerateSecurityReport(_ context.Context, options ReportOptions) (*Report, error) {
	s.logger.Info("Generating security report", "format", options.Format, "timeframe", options.Timeframe)

	reportID := fmt.Sprintf("report-%d", time.Now().Unix())

	// Simulate report generation
	report := &Report{
		ReportID:    reportID,
		GeneratedAt: time.Now(),
		Timeframe:   options.Timeframe,
		Overview: ReportOverview{
			TotalIssues:     150,
			CriticalIssues:  8,
			SecurityScore:   78.5,
			ComplianceScore: 75.0,
			TrendDirection:  "stable",
		},
		Vulnerabilities: VulnerabilityReport{
			Summary: VulnerabilitySummary{
				Total:    150,
				Critical: 8,
				High:     25,
				Medium:   67,
				Low:      45,
				Unknown:  5,
			},
		},
		Secrets: SecretReport{
			Summary: SecretSummary{
				Total:      5,
				HighRisk:   1,
				MediumRisk: 2,
				LowRisk:    2,
			},
		},
		Recommendations: []Recommendation{
			{
				ID:          "REC-001",
				Title:       "Update vulnerable packages",
				Description: "Several packages have known vulnerabilities with available fixes",
				Priority:    "HIGH",
				Impact:      "Reduces attack surface",
				Effort:      "LOW",
				Actions:     []string{"Update package versions", "Test compatibility"},
			},
		},
	}

	s.logger.Info("Security report generated", "reportID", reportID, "totalIssues", report.Overview.TotalIssues)
	return report, nil
}

// UpdateScannerConfig updates scanner configuration
func (s *ServiceImpl) UpdateScannerConfig(config *ScannerConfig) error {
	s.logger.Info("Updating scanner configuration")

	s.config = config

	s.logger.Info("Scanner configuration updated successfully")
	return nil
}

// GetScannerConfig returns current scanner configuration
func (s *ServiceImpl) GetScannerConfig() *ScannerConfig {
	return s.config
}

// GetAvailableScanners returns list of available scanners
func (s *ServiceImpl) GetAvailableScanners() []string {
	return []string{"trivy", "grype", "snyk", "clair", "anchore"}
}

// Helper methods

func (s *ServiceImpl) selectScanner(scanners []string) string {
	if len(scanners) > 0 {
		return scanners[0]
	}
	if len(s.config.DefaultScanners) > 0 {
		return s.config.DefaultScanners[0]
	}
	return "trivy"
}

func (s *ServiceImpl) simulateVulnerabilities(_, _ string) []Vulnerability {
	// Simulate vulnerability findings
	return []Vulnerability{
		{
			VulnerabilityID:  "VULN-001",
			PkgName:          "example-lib",
			InstalledVersion: "1.0.0",
			FixedVersion:     "1.0.1",
			Title:            "Buffer overflow in library",
			Description:      "A buffer overflow vulnerability exists in the library",
			Severity:         "HIGH",
			PublishedDate:    time.Now().Add(-30 * 24 * time.Hour).Format(time.RFC3339),
			LastModifiedDate: time.Now().Add(-1 * 24 * time.Hour).Format(time.RFC3339),
		},
		{
			VulnerabilityID:  "VULN-002",
			PkgName:          "web-framework",
			InstalledVersion: "2.1.0",
			FixedVersion:     "2.1.3",
			Title:            "SQL injection vulnerability",
			Description:      "SQL injection vulnerability in web framework",
			Severity:         "CRITICAL",
			PublishedDate:    time.Now().Add(-15 * 24 * time.Hour).Format(time.RFC3339),
			LastModifiedDate: time.Now().Add(-2 * 24 * time.Hour).Format(time.RFC3339),
		},
	}
}

func (s *ServiceImpl) simulateDockerfileVulnerabilities(content string) []Vulnerability {
	// Simulate Dockerfile-specific vulnerabilities
	vulns := []Vulnerability{}

	if strings.Contains(content, "FROM ubuntu:18.04") {
		vulns = append(vulns, Vulnerability{
			VulnerabilityID:  "DOCKER-001",
			Title:            "Outdated base image",
			Description:      "Base image ubuntu:18.04 has known vulnerabilities",
			Severity:         "MEDIUM",
			PkgName:          "ubuntu",
			InstalledVersion: "18.04",
		})
	}

	if strings.Contains(content, "USER root") || !strings.Contains(content, "USER ") {
		vulns = append(vulns, Vulnerability{
			VulnerabilityID: "DOCKER-002",
			Title:           "Running as root user",
			Description:     "Container runs as root user, increasing security risk",
			Severity:        "HIGH",
		})
	}

	return vulns
}

func (s *ServiceImpl) simulateSecrets(path string) []Secret {
	// Simulate secret detection
	return []Secret{
		{
			ID:          "SECRET-001",
			Type:        "AWS Access Key",
			Description: "Potential AWS access key found",
			File:        path + "/config.env",
			Line:        15,
			Column:      12,
			Confidence:  0.95,
			Severity:    "HIGH",
			Remediation: "Remove hardcoded credentials and use environment variables",
		},
		{
			ID:          "SECRET-002",
			Type:        "Database Password",
			Description: "Database password in configuration file",
			File:        path + "/database.conf",
			Line:        8,
			Column:      20,
			Confidence:  0.85,
			Severity:    "MEDIUM",
			Remediation: "Use secure secret management system",
		},
	}
}

func (s *ServiceImpl) simulateContentSecrets(content string) []Secret {
	secrets := []Secret{}

	if strings.Contains(content, "AKIA") {
		secrets = append(secrets, Secret{
			ID:          "SECRET-003",
			Type:        "AWS Access Key",
			Description: "AWS access key pattern detected",
			Line:        strings.Count(content[:strings.Index(content, "AKIA")], "\n") + 1,
			Confidence:  0.90,
			Severity:    "HIGH",
		})
	}

	if strings.Contains(content, "password=") {
		secrets = append(secrets, Secret{
			ID:          "SECRET-004",
			Type:        "Password",
			Description: "Hardcoded password detected",
			Line:        strings.Count(content[:strings.Index(content, "password=")], "\n") + 1,
			Confidence:  0.75,
			Severity:    "MEDIUM",
		})
	}

	return secrets
}

func (s *ServiceImpl) calculateVulnerabilitySummary(vulns []Vulnerability) VulnerabilitySummary {
	summary := VulnerabilitySummary{
		Total: len(vulns),
	}

	for _, vuln := range vulns {
		switch vuln.Severity {
		case "CRITICAL":
			summary.Critical++
		case "HIGH":
			summary.High++
		case "MEDIUM":
			summary.Medium++
		case "LOW":
			summary.Low++
		default:
			summary.Unknown++
		}

		if vuln.FixedVersion != "" {
			summary.Fixable++
		}
	}

	return summary
}

func (s *ServiceImpl) calculateSecretSummary(secrets []Secret) SecretSummary {
	summary := SecretSummary{
		Total:  len(secrets),
		ByType: make(map[string]int),
		ByFile: make(map[string]int),
	}

	for _, secret := range secrets {
		switch secret.Severity {
		case "HIGH":
			summary.HighRisk++
		case "MEDIUM":
			summary.MediumRisk++
		case "LOW":
			summary.LowRisk++
		}

		summary.ByType[secret.Type]++
		summary.ByFile[secret.File]++
	}

	return summary
}

func (s *ServiceImpl) recordScan(scanID, target, scanType, scanner string, startTime, endTime time.Time, status string, results interface{}) {
	record := ScanRecord{
		ScanID:    scanID,
		Target:    target,
		ScanType:  scanType,
		Scanner:   scanner,
		StartTime: startTime,
		EndTime:   endTime,
		Status:    status,
		Results:   results,
	}

	s.scanHistory = append(s.scanHistory, record)

	// Keep only last 100 scans
	if len(s.scanHistory) > 100 {
		s.scanHistory = s.scanHistory[len(s.scanHistory)-100:]
	}
}

func (s *ServiceImpl) updateMetrics(result *ScanResult) {
	s.metrics.TotalScans++
	s.metrics.VulnerabilitiesFound += int64(len(result.Vulnerabilities))
	s.metrics.LastScan = result.EndTime

	for _, vuln := range result.Vulnerabilities {
		switch vuln.Severity {
		case "CRITICAL", "HIGH":
			s.metrics.HighRiskIssues++
		case "MEDIUM":
			s.metrics.MediumRiskIssues++
		case "LOW", "INFO":
			s.metrics.LowRiskIssues++
		}
	}
}
