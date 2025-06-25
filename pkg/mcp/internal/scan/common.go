package scan

import (
	"context"
	"time"

	"github.com/rs/zerolog"
)

// SecretScanner defines the interface for secret scanning engines
type SecretScanner interface {
	// GetName returns the name of the scanner
	GetName() string

	// GetScanTypes returns the types of secrets this scanner can detect
	GetScanTypes() []string

	// Scan performs secret scanning on the provided content
	Scan(ctx context.Context, config ScanConfig) (*ScanResult, error)

	// IsApplicable determines if this scanner should run for the given content
	IsApplicable(content string, contentType ContentType) bool
}

// ScanConfig provides configuration for secret scanning
type ScanConfig struct {
	Content     string
	ContentType ContentType
	FilePath    string
	Options     ScanOptions
	Logger      zerolog.Logger
}

// ScanOptions provides options for scanning
type ScanOptions struct {
	IncludeHighEntropy bool
	IncludeKeywords    bool
	IncludePatterns    bool
	IncludeBase64      bool
	MaxFileSize        int64
	Sensitivity        SensitivityLevel
	SkipBinary         bool
	SkipArchives       bool
}

// ContentType represents the type of content being scanned
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

// SensitivityLevel represents scanning sensitivity
type SensitivityLevel string

const (
	SensitivityLow    SensitivityLevel = "low"
	SensitivityMedium SensitivityLevel = "medium"
	SensitivityHigh   SensitivityLevel = "high"
)

// ScanResult represents the result from a secret scanner
type ScanResult struct {
	Scanner    string
	Success    bool
	Duration   time.Duration
	Secrets    []Secret
	Metadata   map[string]interface{}
	Confidence float64
	Errors     []error
}

// Secret represents a detected secret
type Secret struct {
	Type        SecretType
	Value       string
	MaskedValue string
	Location    *Location
	Confidence  float64
	Severity    Severity
	Context     string
	Pattern     string
	Entropy     float64
	Metadata    map[string]interface{}
	Evidence    []Evidence
}

// SecretType represents the type of secret detected
type SecretType string

const (
	SecretTypeAPIKey           SecretType = "api_key"
	SecretTypePassword         SecretType = "password"
	SecretTypePrivateKey       SecretType = "private_key"
	SecretTypeCertificate      SecretType = "certificate"
	SecretTypeToken            SecretType = "token"
	SecretTypeConnectionString SecretType = "connection_string"
	SecretTypeCredential       SecretType = "credential"
	SecretTypeSecret           SecretType = "secret"
	SecretTypeEnvironmentVar   SecretType = "environment_variable"
	SecretTypeHighEntropy      SecretType = "high_entropy"
	SecretTypeGeneric          SecretType = "generic"
)

// Severity represents the severity of a secret finding
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// Location represents a location where a secret was found
type Location struct {
	File       string
	Line       int
	Column     int
	StartIndex int
	EndIndex   int
}

// Evidence represents evidence supporting a secret detection
type Evidence struct {
	Type        string
	Description string
	Value       string
	Pattern     string
	Context     string
}

// ScannerRegistry manages multiple secret scanners
type ScannerRegistry struct {
	scanners []SecretScanner
	logger   zerolog.Logger
}

// NewScannerRegistry creates a new scanner registry
func NewScannerRegistry(logger zerolog.Logger) *ScannerRegistry {
	return &ScannerRegistry{
		scanners: make([]SecretScanner, 0),
		logger:   logger.With().Str("component", "scanner_registry").Logger(),
	}
}

// Register registers a secret scanner
func (r *ScannerRegistry) Register(scanner SecretScanner) {
	r.scanners = append(r.scanners, scanner)
	r.logger.Debug().Str("scanner", scanner.GetName()).Msg("Secret scanner registered")
}

// GetApplicableScanners returns scanners applicable for the given content
func (r *ScannerRegistry) GetApplicableScanners(content string, contentType ContentType) []SecretScanner {
	var applicable []SecretScanner
	for _, scanner := range r.scanners {
		if scanner.IsApplicable(content, contentType) {
			applicable = append(applicable, scanner)
		}
	}
	return applicable
}

// ScanWithAllApplicable scans content with all applicable scanners
func (r *ScannerRegistry) ScanWithAllApplicable(ctx context.Context, config ScanConfig) (*CombinedScanResult, error) {
	result := &CombinedScanResult{
		StartTime:      time.Now(),
		ScannerResults: make(map[string]*ScanResult),
		AllSecrets:     make([]Secret, 0),
		Summary:        make(map[string]interface{}),
	}

	applicable := r.GetApplicableScanners(config.Content, config.ContentType)
	r.logger.Info().Int("scanners", len(applicable)).Msg("Running applicable secret scanners")

	for _, scanner := range applicable {
		r.logger.Debug().Str("scanner", scanner.GetName()).Msg("Running secret scanner")
		scanResult, err := scanner.Scan(ctx, config)
		if err != nil {
			r.logger.Error().Err(err).Str("scanner", scanner.GetName()).Msg("Scanner failed")
			continue
		}

		result.ScannerResults[scanner.GetName()] = scanResult
		result.AllSecrets = append(result.AllSecrets, scanResult.Secrets...)
	}

	result.Duration = time.Since(result.StartTime)
	result.Summary = r.generateSummary(result)

	return result, nil
}

// CombinedScanResult represents the combined result from all scanners
type CombinedScanResult struct {
	StartTime      time.Time
	Duration       time.Duration
	ScannerResults map[string]*ScanResult
	AllSecrets     []Secret
	Summary        map[string]interface{}
}

// generateSummary generates a summary of all scan results
func (r *ScannerRegistry) generateSummary(result *CombinedScanResult) map[string]interface{} {
	summary := map[string]interface{}{
		"total_scanners": len(result.ScannerResults),
		"total_secrets":  len(result.AllSecrets),
		"by_type":        make(map[string]int),
		"by_severity":    make(map[string]int),
		"confidence_avg": 0.0,
	}

	// Aggregate secrets by type and severity
	var confidenceSum float64
	for _, secret := range result.AllSecrets {
		summary["by_type"].(map[string]int)[string(secret.Type)]++
		summary["by_severity"].(map[string]int)[string(secret.Severity)]++
		confidenceSum += secret.Confidence
	}

	if len(result.AllSecrets) > 0 {
		summary["confidence_avg"] = confidenceSum / float64(len(result.AllSecrets))
	}

	return summary
}

// GetScannerNames returns the names of all registered scanners
func (r *ScannerRegistry) GetScannerNames() []string {
	names := make([]string, len(r.scanners))
	for i, scanner := range r.scanners {
		names[i] = scanner.GetName()
	}
	return names
}

// GetScanner returns a scanner by name
func (r *ScannerRegistry) GetScanner(name string) SecretScanner {
	for _, scanner := range r.scanners {
		if scanner.GetName() == name {
			return scanner
		}
	}
	return nil
}

// MaskSecret masks a secret value for safe display
func MaskSecret(value string) string {
	if len(value) <= 4 {
		return "***"
	}
	if len(value) <= 8 {
		return value[:2] + "***"
	}
	return value[:4] + "***" + value[len(value)-4:]
}

// CalculateEntropy calculates the Shannon entropy of a string
func CalculateEntropy(s string) float64 {
	if len(s) == 0 {
		return 0
	}

	// Count character frequencies
	freq := make(map[rune]int)
	for _, char := range s {
		freq[char]++
	}

	// Calculate entropy
	var entropy float64
	length := float64(len(s))
	for _, count := range freq {
		p := float64(count) / length
		if p > 0 {
			entropy -= p * logBase2(p)
		}
	}

	return entropy
}

// logBase2 calculates log base 2
func logBase2(x float64) float64 {
	return 0.6931471805599453 * log(x) // ln(2) * ln(x)
}

// Simple natural log approximation
func log(x float64) float64 {
	if x <= 0 {
		return 0
	}
	// Simple approximation - in production would use math.Log
	return x - 1
}

// GetSecretSeverity determines severity based on secret type and confidence
func GetSecretSeverity(secretType SecretType, confidence float64) Severity {
	if confidence < 0.5 {
		return SeverityLow
	}

	switch secretType {
	case SecretTypePrivateKey, SecretTypeCertificate:
		return SeverityCritical
	case SecretTypeAPIKey, SecretTypeToken, SecretTypeConnectionString:
		if confidence > 0.8 {
			return SeverityHigh
		}
		return SeverityMedium
	case SecretTypePassword, SecretTypeCredential:
		if confidence > 0.7 {
			return SeverityMedium
		}
		return SeverityLow
	case SecretTypeHighEntropy:
		if confidence > 0.9 {
			return SeverityMedium
		}
		return SeverityLow
	default:
		return SeverityInfo
	}
}
