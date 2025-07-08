package scan

import (
	"context"
	"log/slog"
	"time"
)

// SecretType represents different types of secrets
type SecretType string

const (
	SecretTypeAPIKey           SecretType = "api_key"
	SecretTypeToken            SecretType = "token"
	SecretTypePassword         SecretType = "password"
	SecretTypeSecret           SecretType = "secret"
	SecretTypeEnvironmentVar   SecretType = "environment_variable"
	SecretTypeCredential       SecretType = "credential"
	SecretTypePrivateKey       SecretType = "private_key"
	SecretTypeCertificate      SecretType = "certificate"
	SecretTypeHighEntropy      SecretType = "high_entropy"
	SecretTypeGeneric          SecretType = "generic"
	SecretTypeConnectionString SecretType = "connection_string"
)

// ContentType represents different types of content to scan
type ContentType string

const (
	ContentTypeSourceCode  ContentType = "source_code"
	ContentTypeConfig      ContentType = "config"
	ContentTypeDockerfile  ContentType = "dockerfile"
	ContentTypeKubernetes  ContentType = "kubernetes"
	ContentTypeCompose     ContentType = "compose"
	ContentTypeDatabase    ContentType = "database"
	ContentTypeEnvironment ContentType = "environment"
	ContentTypeCertificate ContentType = "certificate"
	ContentTypeGeneric     ContentType = "generic"
)

// Severity represents the severity level of a finding
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
	SeverityInfo     Severity = "info"
)

// SensitivityLevel represents the sensitivity level for scanning
type SensitivityLevel string

const (
	SensitivityLow    SensitivityLevel = "low"
	SensitivityMedium SensitivityLevel = "medium"
	SensitivityHigh   SensitivityLevel = "high"
)

// ScanConfig represents configuration for scanning
type ScanConfig struct {
	Content     string       `json:"content"`
	ContentType ContentType  `json:"content_type"`
	FilePath    string       `json:"file_path"`
	Options     ScanOptions  `json:"options"`
	Logger      *slog.Logger `json:"-"`
}

// ScanOptions represents options for scanning
type ScanOptions struct {
	IncludeHighEntropy bool                   `json:"include_high_entropy"`
	IncludeKeywords    bool                   `json:"include_keywords"`
	IncludePatterns    bool                   `json:"include_patterns"`
	IncludeBase64      bool                   `json:"include_base64"`
	MaxFileSize        int64                  `json:"max_file_size"`
	Sensitivity        SensitivityLevel       `json:"sensitivity"`
	SkipBinary         bool                   `json:"skip_binary"`
	SkipArchives       bool                   `json:"skip_archives"`
	SkipSecrets        bool                   `json:"skip_secrets"`
	SkipCompliance     bool                   `json:"skip_compliance"`
	Timeout            time.Duration          `json:"timeout"`
	CustomRules        []string               `json:"custom_rules"`
	Metadata           map[string]interface{} `json:"metadata"`
}

// ScanResult represents the result of a scan with comprehensive fields
type ScanResult struct {
	// Core status fields
	Success bool   `json:"success"`
	Scanner string `json:"scanner,omitempty"`
	Message string `json:"message,omitempty"`

	// Target information
	Target   string `json:"target,omitempty"`
	ScanType string `json:"scan_type,omitempty"`

	// Timing information
	Duration time.Duration `json:"duration"`
	ScanTime time.Time     `json:"scan_time"`

	// Results and findings
	Secrets    []Secret      `json:"secrets,omitempty"`
	Findings   []ScanFinding `json:"findings,omitempty"`
	Confidence float64       `json:"confidence,omitempty"`

	// Status and issues
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`

	// Data and metadata
	Data     map[string]interface{} `json:"data,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ScanFinding represents a security finding
type ScanFinding struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	Title       string `json:"title"`
	Description string `json:"description"`
	File        string `json:"file,omitempty"`
	Line        int    `json:"line,omitempty"`
	Remediation string `json:"remediation,omitempty"`
}

// Vulnerability represents a vulnerability finding
type Vulnerability struct {
	ID          string                 `json:"id"`
	Severity    string                 `json:"severity"`
	Package     string                 `json:"package"`
	Version     string                 `json:"version"`
	FixedIn     string                 `json:"fixed_in"`
	Description string                 `json:"description"`
	CVSS        float64                `json:"cvss"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// SecretScanResult represents secret scan results
type SecretScanResult struct {
	Secrets   []Secret               `json:"secrets"`
	Files     []string               `json:"files"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// ImageScanResult represents image scan results
type ImageScanResult struct {
	ImageRef        string           `json:"image_ref"`
	Vulnerabilities []Vulnerability  `json:"vulnerabilities"`
	Secrets         []SecretFinding  `json:"secrets"`
	Compliance      ComplianceResult `json:"compliance"`
}

// VulnerabilityScanResult represents vulnerability scan results
type VulnerabilityScanResult struct {
	Target          string          `json:"target"`
	Vulnerabilities []Vulnerability `json:"vulnerabilities"`
	Summary         VulnSummary     `json:"summary"`
}

// SecretFinding represents a detected secret
type SecretFinding struct {
	Type        string  `json:"type"`
	File        string  `json:"file"`
	Line        int     `json:"line"`
	Value       string  `json:"value"`
	Confidence  float64 `json:"confidence"`
	Remediation string  `json:"remediation"`
}

// ComplianceResult represents compliance check results
type ComplianceResult struct {
	Framework string            `json:"framework"`
	Passed    bool              `json:"passed"`
	Score     float64           `json:"score"`
	Checks    []ComplianceCheck `json:"checks"`
}

// ComplianceCheck represents a single compliance check
type ComplianceCheck struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Passed      bool   `json:"passed"`
	Description string `json:"description"`
}

// VulnSummary represents vulnerability summary
type VulnSummary struct {
	Total    int `json:"total"`
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
}

// Secret represents a detected secret with location and context
type Secret struct {
	Type        SecretType             `json:"type"`
	Value       string                 `json:"value"`
	MaskedValue string                 `json:"masked_value"`
	Location    *Location              `json:"location,omitempty"`
	Confidence  float64                `json:"confidence"`
	Severity    Severity               `json:"severity"`
	Context     string                 `json:"context"`
	Pattern     string                 `json:"pattern,omitempty"`
	Entropy     float64                `json:"entropy,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Evidence    []Evidence             `json:"evidence,omitempty"`
}

// SecretLocation represents the location of a secret
type SecretLocation struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

// Location represents a location in a file with detailed position info
type Location struct {
	File       string `json:"file"`
	Line       int    `json:"line"`
	Column     int    `json:"column"`
	StartIndex int    `json:"start_index,omitempty"`
	EndIndex   int    `json:"end_index,omitempty"`
}

// Evidence represents evidence supporting a finding
type Evidence struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Value       string `json:"value"`
	Pattern     string `json:"pattern,omitempty"`
	Context     string `json:"context,omitempty"`
}

// CombinedScanResult represents results from multiple scanners
type CombinedScanResult struct {
	ScannerResults map[string]*ScanResult `json:"scanner_results"`
	AllSecrets     []Secret               `json:"all_secrets"`
	Summary        map[string]interface{} `json:"summary"`
	Duration       time.Duration          `json:"duration"`
}

// Scanner interface for different types of scanners
type Scanner interface {
	GetName() string
	GetScanTypes() []string
	IsApplicable(content string, contentType ContentType) bool
	Scan(ctx context.Context, config ScanConfig) (*ScanResult, error)
}

// ScannerRegistry manages multiple scanners
type ScannerRegistry struct {
	scanners map[string]Scanner
	logger   *slog.Logger
}

// NewScannerRegistry creates a new scanner registry
func NewScannerRegistry(logger *slog.Logger) *ScannerRegistry {
	return &ScannerRegistry{
		scanners: make(map[string]Scanner),
		logger:   logger,
	}
}

// Register registers a new scanner
func (r *ScannerRegistry) Register(scanner Scanner) {
	r.scanners[scanner.GetName()] = scanner
}

// RegisterScanner registers a new scanner (deprecated, use Register)
func (r *ScannerRegistry) RegisterScanner(scanner Scanner) {
	r.Register(scanner)
}

// GetScanner returns a scanner by name
func (r *ScannerRegistry) GetScanner(name string) Scanner {
	return r.scanners[name]
}

// GetScannerNames returns all scanner names
func (r *ScannerRegistry) GetScannerNames() []string {
	names := make([]string, 0, len(r.scanners))
	for name := range r.scanners {
		names = append(names, name)
	}
	return names
}

// GetApplicableScanners returns scanners applicable to the given content
func (r *ScannerRegistry) GetApplicableScanners(content string, contentType ContentType) []Scanner {
	var applicable []Scanner
	for _, scanner := range r.scanners {
		if scanner.IsApplicable(content, contentType) {
			applicable = append(applicable, scanner)
		}
	}
	return applicable
}

// GetAllScanners returns all registered scanners
func (r *ScannerRegistry) GetAllScanners() []Scanner {
	scanners := make([]Scanner, 0, len(r.scanners))
	for _, scanner := range r.scanners {
		scanners = append(scanners, scanner)
	}
	return scanners
}

// ScanWithAllApplicable runs all applicable scanners on the content
func (r *ScannerRegistry) ScanWithAllApplicable(ctx context.Context, config ScanConfig) (*CombinedScanResult, error) {
	startTime := time.Now()
	result := &CombinedScanResult{
		ScannerResults: make(map[string]*ScanResult),
		AllSecrets:     []Secret{},
		Summary:        make(map[string]interface{}),
	}

	// Get applicable scanners
	applicable := r.GetApplicableScanners(config.Content, config.ContentType)
	result.Summary["total_scanners"] = len(applicable)

	// Run each scanner
	for _, scanner := range applicable {
		scanResult, err := scanner.Scan(ctx, config)
		if err != nil {
			r.logger.Error("Scanner failed", "scanner", scanner.GetName(), "error", err)
			continue
		}
		result.ScannerResults[scanner.GetName()] = scanResult
		if scanResult.Success && scanResult.Secrets != nil {
			result.AllSecrets = append(result.AllSecrets, scanResult.Secrets...)
		}
	}

	// Update summary
	result.Summary["total_secrets"] = len(result.AllSecrets)
	result.Summary["by_type"] = r.groupSecretsByType(result.AllSecrets)
	result.Summary["by_severity"] = r.groupSecretsBySeverity(result.AllSecrets)
	result.Summary["confidence_avg"] = r.calculateAverageConfidence(result.AllSecrets)
	result.Duration = time.Since(startTime)

	return result, nil
}

// groupSecretsByType groups secrets by their type
func (r *ScannerRegistry) groupSecretsByType(secrets []Secret) map[string]int {
	counts := make(map[string]int)
	for _, secret := range secrets {
		counts[string(secret.Type)]++
	}
	return counts
}

// groupSecretsBySeverity groups secrets by their severity
func (r *ScannerRegistry) groupSecretsBySeverity(secrets []Secret) map[string]int {
	counts := make(map[string]int)
	for _, secret := range secrets {
		counts[string(secret.Severity)]++
	}
	return counts
}

// calculateAverageConfidence calculates the average confidence of all secrets
func (r *ScannerRegistry) calculateAverageConfidence(secrets []Secret) float64 {
	if len(secrets) == 0 {
		return 0
	}
	total := 0.0
	for _, secret := range secrets {
		total += secret.Confidence
	}
	return total / float64(len(secrets))
}

// MaskSecret masks a secret value for safe display
func MaskSecret(value string) string {
	if len(value) <= 4 {
		return "***"
	}
	if len(value) <= 8 {
		return value[:2] + "***"
	}
	return value[:4] + "***" + value[len(value)-2:]
}

// CalculateEntropy calculates the entropy of a string
func CalculateEntropy(s string) float64 {
	if len(s) == 0 {
		return 0.0
	}

	// Count frequency of each character
	freq := make(map[rune]int)
	for _, char := range s {
		freq[char]++
	}

	// Calculate entropy
	length := float64(len(s))
	entropy := 0.0
	for _, count := range freq {
		p := float64(count) / length
		if p > 0 {
			entropy -= p * (log2(p))
		}
	}

	return entropy
}

// log2 calculates log base 2
func log2(x float64) float64 {
	return logN(x) / logN(2)
}

// logN calculates natural logarithm
func logN(x float64) float64 {
	// Simple implementation for small values
	if x <= 0 {
		return 0
	}
	// Using approximation: ln(x) ≈ (x-1) - (x-1)²/2 + (x-1)³/3 - ...
	// For simplicity, using a basic approximation
	return 2.0 * (x - 1.0) / (x + 1.0)
}

// GetSecretSeverity determines the severity based on secret type and confidence
func GetSecretSeverity(secretType SecretType, confidence float64) Severity {
	switch secretType {
	case SecretTypePrivateKey, SecretTypeCertificate:
		if confidence >= 0.8 {
			return SeverityCritical
		}
		return SeverityHigh
	case SecretTypeAPIKey:
		if confidence >= 0.9 {
			return SeverityHigh
		}
		if confidence >= 0.6 {
			return SeverityMedium
		}
		return SeverityLow
	case SecretTypePassword:
		if confidence >= 0.8 {
			return SeverityMedium
		}
		return SeverityLow
	case SecretTypeHighEntropy:
		if confidence >= 0.95 {
			return SeverityMedium
		}
		return SeverityLow
	case SecretTypeGeneric:
		if confidence >= 0.9 {
			return SeverityInfo
		}
		return SeverityLow
	default:
		return SeverityLow
	}
}

// ScanResult methods for compatibility with typesafe_scan_tool_simple.go

// AddError adds an error to the result
func (r *ScanResult) AddError(err string) {
	if r.Errors == nil {
		r.Errors = []string{}
	}
	r.Errors = append(r.Errors, err)
}

// AddWarning adds a warning to the result
func (r *ScanResult) AddWarning(warning string) {
	if r.Warnings == nil {
		r.Warnings = []string{}
	}
	r.Warnings = append(r.Warnings, warning)
}

// AddFinding adds a finding to the result
func (r *ScanResult) AddFinding(finding ScanFinding) {
	if r.Findings == nil {
		r.Findings = []ScanFinding{}
	}
	r.Findings = append(r.Findings, finding)
}

// SetData sets data in the result
func (r *ScanResult) SetData(key string, value interface{}) {
	if r.Data == nil {
		r.Data = make(map[string]interface{})
	}
	r.Data[key] = value
}
