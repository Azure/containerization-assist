package validators

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/common/validation-core/core"
)

// SecurityScanValidator validates security scanning configurations and results
type SecurityScanValidator struct {
	*BaseValidatorImpl
	securityValidator *SecurityValidator
	maxFileSize       int64
	allowedExtensions map[string]bool
	riskThresholds    RiskThresholds
}

// RiskThresholds defines thresholds for security risk assessment
type RiskThresholds struct {
	CriticalVulnLimit int           // Max critical vulnerabilities before marking as critical risk
	HighVulnLimit     int           // Max high vulnerabilities before marking as high risk
	MinSecurityScore  float64       // Minimum security score for acceptable risk
	MaxScanDuration   time.Duration // Maximum acceptable scan duration
}

// NewSecurityScanValidator creates a new security scan validator
func NewSecurityScanValidator() *SecurityScanValidator {
	return &SecurityScanValidator{
		BaseValidatorImpl: NewBaseValidator("security-scan", "1.0.0", []string{"security", "scan", "vulnerability", "secrets"}),
		securityValidator: NewSecurityValidator(),
		maxFileSize:       100 * 1024 * 1024, // 100MB max file size
		allowedExtensions: getDefaultScanExtensions(),
		riskThresholds: RiskThresholds{
			CriticalVulnLimit: 0,    // No critical vulnerabilities allowed
			HighVulnLimit:     5,    // Max 5 high vulnerabilities
			MinSecurityScore:  70.0, // Minimum 70% security score
			MaxScanDuration:   10 * time.Minute,
		},
	}
}

// WithRiskThresholds sets custom risk thresholds
func (s *SecurityScanValidator) WithRiskThresholds(thresholds RiskThresholds) *SecurityScanValidator {
	s.riskThresholds = thresholds
	return s
}

// Validate validates security scan data
func (s *SecurityScanValidator) Validate(ctx context.Context, data interface{}, options *core.ValidationOptions) *core.NonGenericResult {
	startTime := time.Now()
	result := s.BaseValidatorImpl.Validate(ctx, data, options)

	switch v := data.(type) {
	case map[string]interface{}:
		s.validateScanData(v, result, options)
	case SecretScanArgs:
		s.validateSecretScanArgs(v, result, options)
	case SecretScanResult:
		s.validateSecretScanResult(v, result, options)
	case ImageScanArgs:
		s.validateImageScanArgs(v, result, options)
	case ImageScanResult:
		s.validateImageScanResult(v, result, options)
	case VulnerabilityData:
		s.validateVulnerabilityData(v, result, options)
	default:
		result.AddError(&core.Error{
			Code:     "INVALID_SCAN_DATA",
			Message:  fmt.Sprintf("Expected security scan data, got %T", data),
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityHigh,
		})
	}

	result.Duration = time.Since(startTime)
	return result
}

// SecretScanArgs represents secret scanning arguments
type SecretScanArgs struct {
	SessionID          string   `json:"session_id"`
	ScanPath           string   `json:"scan_path"`
	FilePatterns       []string `json:"file_patterns"`
	ExcludePatterns    []string `json:"exclude_patterns"`
	ScanDockerfiles    bool     `json:"scan_dockerfiles"`
	ScanManifests      bool     `json:"scan_manifests"`
	ScanSourceCode     bool     `json:"scan_source_code"`
	ScanEnvFiles       bool     `json:"scan_env_files"`
	SuggestRemediation bool     `json:"suggest_remediation"`
	GenerateSecrets    bool     `json:"generate_secrets"`
}

// SecretScanResult represents secret scanning result
type SecretScanResult struct {
	SessionID         string                 `json:"session_id"`
	ScanPath          string                 `json:"scan_path"`
	FilesScanned      int                    `json:"files_scanned"`
	Duration          time.Duration          `json:"duration"`
	SecretsFound      int                    `json:"secrets_found"`
	DetectedSecrets   []DetectedSecret       `json:"detected_secrets"`
	SeverityBreakdown map[string]int         `json:"severity_breakdown"`
	SecurityScore     int                    `json:"security_score"`
	RiskLevel         string                 `json:"risk_level"`
	Recommendations   []string               `json:"recommendations"`
	ScanContext       map[string]interface{} `json:"scan_context"`
}

// ImageScanArgs represents image security scanning arguments
type ImageScanArgs struct {
	SessionID           string   `json:"session_id"`
	ImageName           string   `json:"image_name"`
	SeverityThreshold   string   `json:"severity_threshold"`
	VulnTypes           []string `json:"vuln_types"`
	IncludeFixable      bool     `json:"include_fixable"`
	MaxResults          int      `json:"max_results"`
	IncludeRemediations bool     `json:"include_remediations"`
	GenerateReport      bool     `json:"generate_report"`
	FailOnCritical      bool     `json:"fail_on_critical"`
}

// ImageScanResult represents image security scanning result
type ImageScanResult struct {
	SessionID        string                 `json:"session_id"`
	ImageName        string                 `json:"image_name"`
	ScanTime         time.Time              `json:"scan_time"`
	Duration         time.Duration          `json:"duration"`
	Scanner          string                 `json:"scanner"`
	Success          bool                   `json:"success"`
	SecurityScore    int                    `json:"security_score"`
	RiskLevel        string                 `json:"risk_level"`
	Vulnerabilities  []Vulnerability        `json:"vulnerabilities"`
	CriticalFindings []CriticalFinding      `json:"critical_findings"`
	Summary          VulnerabilitySummary   `json:"summary"`
	Metadata         map[string]interface{} `json:"metadata"`
}

// VulnerabilityData represents vulnerability information
type VulnerabilityData struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Severity    string    `json:"severity"`
	Score       float64   `json:"score"`
	Package     string    `json:"package"`
	Version     string    `json:"version"`
	FixedIn     string    `json:"fixed_in"`
	PublishedAt time.Time `json:"published_at"`
	References  []string  `json:"references"`
}

// DetectedSecret represents a detected secret
type DetectedSecret struct {
	Type        string `json:"type"`
	Value       string `json:"value"`
	File        string `json:"file"`
	Line        int    `json:"line"`
	Column      int    `json:"column"`
	Severity    string `json:"severity"`
	Confidence  string `json:"confidence"`
	Description string `json:"description"`
}

// Vulnerability represents a security vulnerability
type Vulnerability struct {
	ID           string   `json:"id"`
	Severity     string   `json:"severity"`
	Score        float64  `json:"score"`
	Package      string   `json:"package"`
	Version      string   `json:"version"`
	FixedVersion string   `json:"fixed_version"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	References   []string `json:"references"`
	Fixable      bool     `json:"fixable"`
}

// CriticalFinding represents a critical security finding
type CriticalFinding struct {
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
	Impact      string `json:"impact"`
	Remediation string `json:"remediation"`
}

// VulnerabilitySummary represents vulnerability summary
type VulnerabilitySummary struct {
	Total      int            `json:"total"`
	Critical   int            `json:"critical"`
	High       int            `json:"high"`
	Medium     int            `json:"medium"`
	Low        int            `json:"low"`
	Fixable    int            `json:"fixable"`
	BySeverity map[string]int `json:"by_severity"`
}

// validateScanData validates general scan data
func (s *SecurityScanValidator) validateScanData(data map[string]interface{}, result *core.NonGenericResult, options *core.ValidationOptions) {
	// Validate session ID
	if sessionID, exists := data["session_id"]; exists {
		if sessionStr, ok := sessionID.(string); ok {
			if sessionStr == "" {
				result.AddFieldError("session_id", "Session ID cannot be empty")
			}
		} else {
			result.AddFieldError("session_id", "Session ID must be a string")
		}
	} else {
		result.AddFieldError("session_id", "Session ID is required")
	}

	// Validate scan path or image name
	if scanPath, exists := data["scan_path"]; exists {
		if pathStr, ok := scanPath.(string); ok {
			s.validateScanPath(pathStr, "scan_path", result)
		}
	} else if imageName, exists := data["image_name"]; exists {
		if imageStr, ok := imageName.(string); ok {
			s.validateImageName(imageStr, "image_name", result)
		}
	} else {
		result.AddError(&core.Error{
			Code:     "MISSING_SCAN_TARGET",
			Message:  "Either scan_path or image_name must be specified",
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityHigh,
		})
	}
}

// validateSecretScanArgs validates secret scanning arguments
func (s *SecurityScanValidator) validateSecretScanArgs(args SecretScanArgs, result *core.NonGenericResult, options *core.ValidationOptions) {
	// Validate session ID
	if args.SessionID == "" {
		result.AddFieldError("session_id", "Session ID is required")
	}

	// Validate scan path
	s.validateScanPath(args.ScanPath, "scan_path", result)

	// Validate file patterns
	for i, pattern := range args.FilePatterns {
		if pattern == "" {
			result.AddFieldError(fmt.Sprintf("file_patterns[%d]", i), "File pattern cannot be empty")
		}
	}

	// Validate exclude patterns
	for i, pattern := range args.ExcludePatterns {
		if pattern == "" {
			result.AddFieldError(fmt.Sprintf("exclude_patterns[%d]", i), "Exclude pattern cannot be empty")
		}
	}

	// Logical validations
	if !args.ScanDockerfiles && !args.ScanManifests && !args.ScanSourceCode && !args.ScanEnvFiles {
		result.AddWarning(&core.Warning{
			Error: &core.Error{
				Code:     "NO_SCAN_TYPES_ENABLED",
				Message:  "No scan types enabled - scan may not find any secrets",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityMedium,
			},
		})
	}
}

// validateSecretScanResult validates secret scanning result
func (s *SecurityScanValidator) validateSecretScanResult(result SecretScanResult, validationResult *core.NonGenericResult, options *core.ValidationOptions) {
	// Validate consistency
	if result.SecretsFound != len(result.DetectedSecrets) {
		validationResult.AddError(&core.Error{
			Code:     "INCONSISTENT_SECRET_COUNT",
			Message:  fmt.Sprintf("SecretsFound (%d) doesn't match DetectedSecrets length (%d)", result.SecretsFound, len(result.DetectedSecrets)),
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityHigh,
		})
	}

	// Validate security score
	if result.SecurityScore < 0 || result.SecurityScore > 100 {
		validationResult.AddFieldError("security_score", "Security score must be between 0 and 100")
	}

	// Validate risk level
	validRiskLevels := []string{"low", "medium", "high", "critical"}
	if !s.contains(validRiskLevels, strings.ToLower(result.RiskLevel)) {
		validationResult.AddFieldError("risk_level", fmt.Sprintf("Invalid risk level: %s", result.RiskLevel))
	}

	// Validate detected secrets
	for i, secret := range result.DetectedSecrets {
		s.validateDetectedSecret(secret, fmt.Sprintf("detected_secrets[%d]", i), validationResult)
	}

	// Risk assessment
	if result.SecretsFound > 0 && float64(result.SecurityScore) > s.riskThresholds.MinSecurityScore {
		validationResult.AddWarning(&core.Warning{
			Error: &core.Error{
				Code:     "SECRETS_FOUND_HIGH_SCORE",
				Message:  "Secrets found but security score is still high - review secret types",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityMedium,
			},
		})
	}

	// Performance check
	if result.Duration > s.riskThresholds.MaxScanDuration {
		validationResult.AddWarning(&core.Warning{
			Error: &core.Error{
				Code:     "LONG_SCAN_DURATION",
				Message:  fmt.Sprintf("Scan took %v, which exceeds recommended maximum %v", result.Duration, s.riskThresholds.MaxScanDuration),
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityMedium,
			},
		})
	}
}

// validateImageScanArgs validates image scanning arguments
func (s *SecurityScanValidator) validateImageScanArgs(args ImageScanArgs, result *core.NonGenericResult, options *core.ValidationOptions) {
	// Validate session ID
	if args.SessionID == "" {
		result.AddFieldError("session_id", "Session ID is required")
	}

	// Validate image name
	s.validateImageName(args.ImageName, "image_name", result)

	// Validate severity threshold
	if args.SeverityThreshold != "" {
		validSeverities := []string{"LOW", "MEDIUM", "HIGH", "CRITICAL"}
		if !s.contains(validSeverities, strings.ToUpper(args.SeverityThreshold)) {
			result.AddFieldError("severity_threshold", fmt.Sprintf("Invalid severity threshold: %s", args.SeverityThreshold))
		}
	}

	// Validate vulnerability types
	validVulnTypes := []string{"os", "library", "app"}
	for i, vulnType := range args.VulnTypes {
		if !s.contains(validVulnTypes, strings.ToLower(vulnType)) {
			result.AddFieldError(fmt.Sprintf("vuln_types[%d]", i), fmt.Sprintf("Invalid vulnerability type: %s", vulnType))
		}
	}

	// Validate max results
	if args.MaxResults < 0 {
		result.AddFieldError("max_results", "Max results cannot be negative")
	} else if args.MaxResults > 10000 {
		result.AddWarning(&core.Warning{
			Error: &core.Error{
				Code:     "HIGH_MAX_RESULTS",
				Message:  "Max results is very high - this may impact performance",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityLow,
				Field:    "max_results",
			},
		})
	}
}

// validateImageScanResult validates image scanning result
func (s *SecurityScanValidator) validateImageScanResult(result ImageScanResult, validationResult *core.NonGenericResult, options *core.ValidationOptions) {
	// Validate security score
	if result.SecurityScore < 0 || result.SecurityScore > 100 {
		validationResult.AddFieldError("security_score", "Security score must be between 0 and 100")
	}

	// Validate risk level
	validRiskLevels := []string{"low", "medium", "high", "critical"}
	if !s.contains(validRiskLevels, strings.ToLower(result.RiskLevel)) {
		validationResult.AddFieldError("risk_level", fmt.Sprintf("Invalid risk level: %s", result.RiskLevel))
	}

	// Validate vulnerabilities
	for i, vuln := range result.Vulnerabilities {
		s.validateVulnerability(vuln, fmt.Sprintf("vulnerabilities[%d]", i), validationResult)
	}

	// Validate critical findings
	for i, finding := range result.CriticalFindings {
		s.validateCriticalFinding(finding, fmt.Sprintf("critical_findings[%d]", i), validationResult)
	}

	// Risk assessment based on thresholds
	if result.Summary.Critical > s.riskThresholds.CriticalVulnLimit {
		validationResult.AddError(&core.Error{
			Code:     "CRITICAL_VULNERABILITIES_EXCEEDED",
			Message:  fmt.Sprintf("Found %d critical vulnerabilities (limit: %d)", result.Summary.Critical, s.riskThresholds.CriticalVulnLimit),
			Type:     core.ErrTypeSecurity,
			Severity: core.SeverityCritical,
		})
	}

	if result.Summary.High > s.riskThresholds.HighVulnLimit {
		validationResult.AddWarning(&core.Warning{
			Error: &core.Error{
				Code:     "HIGH_VULNERABILITIES_EXCEEDED",
				Message:  fmt.Sprintf("Found %d high vulnerabilities (limit: %d)", result.Summary.High, s.riskThresholds.HighVulnLimit),
				Type:     core.ErrTypeSecurity,
				Severity: core.SeverityHigh,
			},
		})
	}

	if float64(result.SecurityScore) < s.riskThresholds.MinSecurityScore {
		validationResult.AddError(&core.Error{
			Code:     "LOW_SECURITY_SCORE",
			Message:  fmt.Sprintf("Security score %d is below minimum threshold %.1f", result.SecurityScore, s.riskThresholds.MinSecurityScore),
			Type:     core.ErrTypeSecurity,
			Severity: core.SeverityHigh,
		})
	}

	// Consistency checks
	calculatedTotal := result.Summary.Critical + result.Summary.High + result.Summary.Medium + result.Summary.Low
	if calculatedTotal != result.Summary.Total {
		validationResult.AddError(&core.Error{
			Code:     "INCONSISTENT_VULNERABILITY_TOTALS",
			Message:  fmt.Sprintf("Vulnerability count mismatch: total=%d, calculated=%d", result.Summary.Total, calculatedTotal),
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityMedium,
		})
	}
}

// validateVulnerabilityData validates vulnerability data
func (s *SecurityScanValidator) validateVulnerabilityData(data VulnerabilityData, result *core.NonGenericResult, options *core.ValidationOptions) {
	// Validate required fields
	if data.ID == "" {
		result.AddFieldError("id", "Vulnerability ID is required")
	}

	if data.Severity == "" {
		result.AddFieldError("severity", "Severity is required")
	} else {
		validSeverities := []string{"LOW", "MEDIUM", "HIGH", "CRITICAL"}
		if !s.contains(validSeverities, strings.ToUpper(data.Severity)) {
			result.AddFieldError("severity", fmt.Sprintf("Invalid severity: %s", data.Severity))
		}
	}

	// Validate score
	if data.Score < 0 || data.Score > 10 {
		result.AddFieldError("score", "Vulnerability score must be between 0 and 10")
	}

	// Validate package information
	if data.Package == "" {
		result.AddWarning(&core.Warning{
			Error: &core.Error{
				Code:     "MISSING_PACKAGE_INFO",
				Message:  "Package information is missing for vulnerability",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityLow,
				Field:    "package",
			},
		})
	}
}

// Helper validation methods

func (s *SecurityScanValidator) validateScanPath(path, field string, result *core.NonGenericResult) {
	if path == "" {
		return // Empty path might use default
	}

	// Check for dangerous paths
	dangerousPaths := []string{"/..", "/etc", "/root", "/home"}
	for _, dangerous := range dangerousPaths {
		if strings.HasPrefix(path, dangerous) {
			result.AddWarning(&core.Warning{
				Error: &core.Error{
					Code:     "POTENTIALLY_DANGEROUS_PATH",
					Message:  fmt.Sprintf("Scan path starts with potentially dangerous prefix: %s", dangerous),
					Type:     core.ErrTypeSecurity,
					Severity: core.SeverityMedium,
					Field:    field,
				},
			})
		}
	}

	// Check for directory traversal patterns
	if strings.Contains(path, "..") {
		result.AddWarning(&core.Warning{
			Error: &core.Error{
				Code:     "DIRECTORY_TRAVERSAL_PATTERN",
				Message:  "Scan path contains directory traversal pattern",
				Type:     core.ErrTypeSecurity,
				Severity: core.SeverityMedium,
				Field:    field,
			},
		})
	}
}

func (s *SecurityScanValidator) validateImageName(imageName, field string, result *core.NonGenericResult) {
	if imageName == "" {
		result.AddFieldError(field, "Image name is required")
		return
	}

	// Basic image name validation
	if !strings.Contains(imageName, ":") {
		result.AddWarning(&core.Warning{
			Error: &core.Error{
				Code:     "MISSING_IMAGE_TAG",
				Message:  "Image name should include a tag",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityLow,
				Field:    field,
			},
		})
	}

	// Check for latest tag
	if strings.HasSuffix(imageName, ":latest") {
		result.AddWarning(&core.Warning{
			Error: &core.Error{
				Code:     "LATEST_TAG_WARNING",
				Message:  "Using 'latest' tag may lead to inconsistent scan results",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityLow,
				Field:    field,
			},
		})
	}
}

func (s *SecurityScanValidator) validateDetectedSecret(secret DetectedSecret, field string, result *core.NonGenericResult) {
	if secret.Type == "" {
		result.AddFieldError(field+".type", "Secret type is required")
	}

	if secret.File == "" {
		result.AddFieldError(field+".file", "File path is required")
	}

	if secret.Line <= 0 {
		result.AddFieldError(field+".line", "Line number must be positive")
	}

	// Validate severity
	validSeverities := []string{"low", "medium", "high", "critical"}
	if !s.contains(validSeverities, strings.ToLower(secret.Severity)) {
		result.AddFieldError(field+".severity", fmt.Sprintf("Invalid severity: %s", secret.Severity))
	}

	// Validate confidence
	validConfidences := []string{"low", "medium", "high"}
	if secret.Confidence != "" && !s.contains(validConfidences, strings.ToLower(secret.Confidence)) {
		result.AddFieldError(field+".confidence", fmt.Sprintf("Invalid confidence: %s", secret.Confidence))
	}
}

func (s *SecurityScanValidator) validateVulnerability(vuln Vulnerability, field string, result *core.NonGenericResult) {
	if vuln.ID == "" {
		result.AddFieldError(field+".id", "Vulnerability ID is required")
	}

	if vuln.Severity == "" {
		result.AddFieldError(field+".severity", "Severity is required")
	}

	if vuln.Score < 0 || vuln.Score > 10 {
		result.AddFieldError(field+".score", "Score must be between 0 and 10")
	}
}

func (s *SecurityScanValidator) validateCriticalFinding(finding CriticalFinding, field string, result *core.NonGenericResult) {
	if finding.Type == "" {
		result.AddFieldError(field+".type", "Finding type is required")
	}

	if finding.Severity == "" {
		result.AddFieldError(field+".severity", "Severity is required")
	}

	if finding.Description == "" {
		result.AddFieldError(field+".description", "Description is required")
	}
}

func (s *SecurityScanValidator) contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// getDefaultScanExtensions returns default file extensions for scanning
func getDefaultScanExtensions() map[string]bool {
	return map[string]bool{
		".py":         true,
		".js":         true,
		".ts":         true,
		".java":       true,
		".go":         true,
		".php":        true,
		".rb":         true,
		".sh":         true,
		".bash":       true,
		".zsh":        true,
		".env":        true,
		".yml":        true,
		".yaml":       true,
		".json":       true,
		".xml":        true,
		".properties": true,
		".conf":       true,
		".config":     true,
		".dockerfile": true,
	}
}

// Public validation functions

// ValidateSecretScanArgs validates secret scanning arguments
func ValidateSecretScanArgs(args interface{}) *core.NonGenericResult {
	validator := NewSecurityScanValidator()
	ctx := context.Background()
	options := core.NewValidationOptions()

	return validator.Validate(ctx, args, options)
}

// ValidateImageScanArgs validates image scanning arguments
func ValidateImageScanArgs(args interface{}) *core.NonGenericResult {
	validator := NewSecurityScanValidator()
	ctx := context.Background()
	options := core.NewValidationOptions()

	return validator.Validate(ctx, args, options)
}

// ValidateSecurityScanResult validates security scan results
func ValidateSecurityScanResult(result interface{}) *core.NonGenericResult {
	validator := NewSecurityScanValidator()
	ctx := context.Background()
	options := core.NewValidationOptions()

	return validator.Validate(ctx, result, options)
}
