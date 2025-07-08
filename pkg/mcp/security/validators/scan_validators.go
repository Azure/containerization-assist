package validators

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/errors"
)

// ScanValidator provides validation specific to security scanning operations
type ScanValidator struct {
	unified *UnifiedValidator
}

// NewScanValidator creates a new scan validator
func NewScanValidator() *ScanValidator {
	return &ScanValidator{
		unified: NewUnifiedValidator(),
	}
}

// ValidateSecretScanArgs validates arguments for secret scanning
func (sv *ScanValidator) ValidateSecretScanArgs(ctx context.Context, sessionID, scanPath string, filePatterns []string) error {
	vctx := NewValidateContext(ctx)

	// Validate session ID
	if err := sv.unified.Input.ValidateSessionID(sessionID); err != nil {
		vctx.AddError(err)
	}

	// Validate scan path
	if scanPath == "" {
		vctx.AddError(errors.Validation("scan", "scan_path is required"))
	} else {
		if err := sv.unified.FileSystem.ValidateDirectoryExists(scanPath); err != nil {
			vctx.AddError(err)
		}
	}

	// Validate file patterns if provided
	if len(filePatterns) > 0 {
		for _, pattern := range filePatterns {
			if pattern == "" {
				vctx.AddError(errors.Validation("scan", "file pattern cannot be empty"))
				break
			}
		}
	}

	return vctx.GetFirstError()
}

// ValidateImageScanArgs validates arguments for image security scanning
func (sv *ScanValidator) ValidateImageScanArgs(sessionID, imageName string) error {
	if err := sv.unified.Input.ValidateSessionID(sessionID); err != nil {
		return err
	}

	if err := sv.unified.Input.ValidateImageName(imageName); err != nil {
		return err
	}

	return nil
}

// ValidateScanPatterns validates file patterns for scanning
func (sv *ScanValidator) ValidateScanPatterns(includePatterns, excludePatterns []string) error {
	// Validate include patterns
	for _, pattern := range includePatterns {
		if pattern == "" {
			return errors.Validation("scan", "include pattern cannot be empty")
		}
		if strings.Contains(pattern, "..") {
			return errors.Validationf("scan", "include pattern contains dangerous path traversal: %s", pattern)
		}
	}

	// Validate exclude patterns
	for _, pattern := range excludePatterns {
		if pattern == "" {
			return errors.Validation("scan", "exclude pattern cannot be empty")
		}
		if strings.Contains(pattern, "..") {
			return errors.Validationf("scan", "exclude pattern contains dangerous path traversal: %s", pattern)
		}
	}

	return nil
}

// ValidateTrivyAvailable checks if Trivy scanner is available
func (sv *ScanValidator) ValidateTrivyAvailable() error {
	return sv.unified.System.ValidateCommandAvailable("trivy")
}

// GenerateSecretScanConfig generates validated configuration for secret scanning
func (sv *ScanValidator) GenerateSecretScanConfig(scanPath string, filePatterns, excludePatterns []string) (*SecretScanConfig, error) {
	// Validate inputs
	if err := sv.ValidateScanPatterns(filePatterns, excludePatterns); err != nil {
		return nil, err
	}

	config := &SecretScanConfig{
		ScanPath:        scanPath,
		FilePatterns:    filePatterns,
		ExcludePatterns: excludePatterns,
		ScanTypes:       DefaultScanTypes(),
	}

	// Set default patterns if none provided
	if len(config.FilePatterns) == 0 {
		config.FilePatterns = DefaultFilePatterns()
	}

	return config, nil
}

// SecretScanConfig represents configuration for secret scanning
type SecretScanConfig struct {
	ScanPath        string   `json:"scan_path"`
	FilePatterns    []string `json:"file_patterns"`
	ExcludePatterns []string `json:"exclude_patterns"`
	ScanTypes       []string `json:"scan_types"`
}

// DefaultFilePatterns returns default file patterns for secret scanning
func DefaultFilePatterns() []string {
	return []string{
		"*.env",
		"*.yml",
		"*.yaml",
		"*.json",
		"*.conf",
		"*.config",
		"*.properties",
		"*.ini",
		"*.tf",
		"*.tfvars",
		"Dockerfile*",
		"docker-compose*",
		"*.sh",
		"*.bash",
		"*.ps1",
		"*.py",
		"*.js",
		"*.ts",
		"*.go",
		"*.java",
		"*.cs",
		"*.rb",
		"*.php",
	}
}

// DefaultScanTypes returns default scan types for secret detection
func DefaultScanTypes() []string {
	return []string{
		"api_keys",
		"passwords",
		"tokens",
		"certificates",
		"database_urls",
		"cloud_credentials",
	}
}

// SecurityScanValidator provides validation for comprehensive security scanning
type SecurityScanValidator struct {
	unified *UnifiedValidator
}

// NewSecurityScanValidator creates a new security scan validator
func NewSecurityScanValidator() *SecurityScanValidator {
	return &SecurityScanValidator{
		unified: NewUnifiedValidator(),
	}
}

// ValidateComprehensiveScanArgs validates arguments for comprehensive security scanning
func (ssv *SecurityScanValidator) ValidateComprehensiveScanArgs(sessionID, imageName, scanPath string, scanTypes []string) error {
	if err := ssv.unified.Input.ValidateSessionID(sessionID); err != nil {
		return err
	}

	// At least one target (image or path) must be provided
	if imageName == "" && scanPath == "" {
		return errors.Validation("scan", "either image_name or scan_path must be provided")
	}

	// Validate image name if provided
	if imageName != "" {
		if err := ssv.unified.Input.ValidateImageName(imageName); err != nil {
			return err
		}
	}

	// Validate scan path if provided
	if scanPath != "" {
		if err := ssv.unified.FileSystem.ValidateDirectoryExists(scanPath); err != nil {
			return err
		}
	}

	// Validate scan types if provided
	if len(scanTypes) > 0 {
		validScanTypes := map[string]bool{
			"vulnerabilities": true,
			"secrets":         true,
			"licenses":        true,
			"configuration":   true,
			"malware":         true,
		}

		for _, scanType := range scanTypes {
			if !validScanTypes[scanType] {
				return errors.Validationf("scan", "invalid scan type: %s", scanType)
			}
		}
	}

	return nil
}

// ValidateScanOutputFormat validates the output format for scan results
func (ssv *SecurityScanValidator) ValidateScanOutputFormat(format string) error {
	if format == "" {
		return nil // Default format will be used
	}

	validFormats := map[string]bool{
		"json":      true,
		"table":     true,
		"sarif":     true,
		"cyclonedx": true,
		"spdx":      true,
	}

	if !validFormats[format] {
		return errors.Validationf("scan", "invalid output format: %s", format)
	}

	return nil
}

// AnalyzeValidator provides validation for repository analysis operations
type AnalyzeValidator struct {
	unified *UnifiedValidator
}

// NewAnalyzeValidator creates a new analyze validator
func NewAnalyzeValidator() *AnalyzeValidator {
	return &AnalyzeValidator{
		unified: NewUnifiedValidator(),
	}
}

// ValidateAnalyzeArgs validates arguments for repository analysis
func (av *AnalyzeValidator) ValidateAnalyzeArgs(sessionID, repoURL string) error {
	if err := av.unified.Input.ValidateSessionID(sessionID); err != nil {
		return err
	}

	if err := av.unified.Input.ValidateGitURL(repoURL); err != nil {
		return err
	}

	return nil
}

// ValidateCloneDirectory validates the directory for repository cloning
func (av *AnalyzeValidator) ValidateCloneDirectory(cloneDir string) error {
	if cloneDir == "" {
		return errors.Validation("analyze", "clone directory is required")
	}

	// Check if parent directory exists
	parentDir := filepath.Dir(cloneDir)
	if err := av.unified.FileSystem.ValidateDirectoryExists(parentDir); err != nil {
		return errors.Wrapf(err, "analyze", "parent directory for clone does not exist")
	}

	return nil
}

// ValidateLanguageHint validates the language hint for analysis
func (av *AnalyzeValidator) ValidateLanguageHint(languageHint string) error {
	if languageHint == "" {
		return nil // Language hint is optional
	}

	supportedLanguages := map[string]bool{
		"go":         true,
		"java":       true,
		"javascript": true,
		"typescript": true,
		"python":     true,
		"ruby":       true,
		"php":        true,
		"csharp":     true,
		"cpp":        true,
		"rust":       true,
		"kotlin":     true,
		"swift":      true,
		"scala":      true,
		"dart":       true,
	}

	if !supportedLanguages[strings.ToLower(languageHint)] {
		return errors.Validationf("analyze", "unsupported language hint: %s", languageHint)
	}

	return nil
}
