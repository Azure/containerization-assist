package scan

import (
	"context"
	"fmt"
	"time"

	commonUtils "github.com/Azure/container-kit/pkg/commonutils"
	"github.com/Azure/container-kit/pkg/mcp/validation/core"
	"github.com/Azure/container-kit/pkg/mcp/validation/validators"
)

// ValidateUnified validates scan tool arguments using unified validation
func (t *AtomicScanSecretsTool) ValidateUnified(ctx context.Context, args interface{}) (*core.ValidationResult, error) {
	// Create security scan validator
	securityValidator := validators.NewSecurityScanValidator()

	// Type check arguments
	typedArgs, ok := args.(AtomicScanSecretsArgs)
	if !ok {
		result := &core.ValidationResult{
			Valid: false,
			Errors: []*core.ValidationError{{
				Code:     "INVALID_ARGUMENT_TYPE",
				Message:  "Invalid argument type: expected AtomicScanSecretsArgs",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityCritical,
			}},
		}
		return result, fmt.Errorf("invalid argument type: expected AtomicScanSecretsArgs")
	}

	// Convert to validator format
	scanArgs := validators.SecretScanArgs{
		SessionID:          typedArgs.SessionID,
		ScanPath:           typedArgs.ScanPath,
		FilePatterns:       typedArgs.FilePatterns,
		ExcludePatterns:    typedArgs.ExcludePatterns,
		ScanDockerfiles:    typedArgs.ScanDockerfiles,
		ScanManifests:      typedArgs.ScanManifests,
		ScanSourceCode:     typedArgs.ScanSourceCode,
		ScanEnvFiles:       typedArgs.ScanEnvFiles,
		SuggestRemediation: typedArgs.SuggestRemediation,
		GenerateSecrets:    typedArgs.GenerateSecrets,
	}

	// Perform unified validation
	options := core.NewValidationOptions().WithStrictMode(false)
	validationResult := securityValidator.Validate(ctx, scanArgs, options)

	// Check for critical errors that should prevent execution
	var criticalError error
	for _, err := range validationResult.Errors {
		if err.Severity == core.SeverityCritical {
			criticalError = fmt.Errorf("validation failed: %s", err.Message)
			break
		}
	}

	// Also perform original validation for backward compatibility
	originalErr := t.Validate(ctx, args)
	if originalErr != nil && criticalError == nil {
		criticalError = originalErr
	}

	return validationResult, criticalError
}

// ValidateUnified validates image scan tool arguments using unified validation
func (t *AtomicScanImageSecurityTool) ValidateUnified(ctx context.Context, args interface{}) (*core.ValidationResult, error) {
	// Create security scan validator
	securityValidator := validators.NewSecurityScanValidator()

	// Type check arguments
	scanArgs, ok := args.(AtomicScanImageSecurityArgs)
	if !ok {
		result := &core.ValidationResult{
			Valid: false,
			Errors: []*core.ValidationError{{
				Code:     "INVALID_ARGUMENT_TYPE",
				Message:  fmt.Sprintf("Invalid arguments type: expected AtomicScanImageSecurityArgs, got %T", args),
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityCritical,
			}},
		}
		return result, fmt.Errorf("invalid arguments type: expected AtomicScanImageSecurityArgs, got %T", args)
	}

	// Convert to validator format
	imageArgs := validators.ImageScanArgs{
		SessionID:           scanArgs.SessionID,
		ImageName:           scanArgs.ImageName,
		SeverityThreshold:   scanArgs.SeverityThreshold,
		VulnTypes:           scanArgs.VulnTypes,
		IncludeFixable:      scanArgs.IncludeFixable,
		MaxResults:          scanArgs.MaxResults,
		IncludeRemediations: scanArgs.IncludeRemediations,
		GenerateReport:      scanArgs.GenerateReport,
		FailOnCritical:      scanArgs.FailOnCritical,
	}

	// Perform unified validation
	options := core.NewValidationOptions().WithStrictMode(false)
	validationResult := securityValidator.Validate(ctx, imageArgs, options)

	// Check for critical errors that should prevent execution
	var criticalError error
	for _, err := range validationResult.Errors {
		if err.Severity == core.SeverityCritical {
			criticalError = fmt.Errorf("validation failed: %s", err.Message)
			break
		}
	}

	// Also perform original validation for backward compatibility
	originalErr := t.Validate(ctx, args)
	if originalErr != nil && criticalError == nil {
		criticalError = originalErr
	}

	return validationResult, criticalError
}

// ValidateSecretScanResultUnified validates secret scan results using unified validation
func ValidateSecretScanResultUnified(result AtomicScanSecretsResult) *core.ValidationResult {
	// Convert to validator format
	scanResult := validators.SecretScanResult{
		SessionID:         result.SessionID,
		ScanPath:          result.ScanPath,
		FilesScanned:      result.FilesScanned,
		Duration:          result.Duration,
		SecretsFound:      result.SecretsFound,
		DetectedSecrets:   convertDetectedSecrets(result.DetectedSecrets),
		SeverityBreakdown: result.SeverityBreakdown,
		SecurityScore:     result.SecurityScore,
		RiskLevel:         result.RiskLevel,
		Recommendations:   result.Recommendations,
		ScanContext:       result.ScanContext,
	}

	return validators.ValidateSecurityScanResult(scanResult)
}

// ValidateImageScanResultUnified validates image scan results using unified validation
func ValidateImageScanResultUnified(result AtomicScanImageSecurityResult) *core.ValidationResult {
	// Convert to validator format
	imageResult := validators.ImageScanResult{
		SessionID:        result.SessionID,
		ImageName:        result.ImageName,
		ScanTime:         result.ScanTime,
		Duration:         result.Duration,
		Scanner:          result.Scanner,
		Success:          result.Success,
		SecurityScore:    result.SecurityScore,
		RiskLevel:        result.RiskLevel,
		Vulnerabilities:  convertVulnerabilities(result),
		CriticalFindings: convertCriticalFindings(result.CriticalFindings),
		Summary:          convertVulnerabilitySummary(result.VulnSummary),
		Metadata:         make(map[string]interface{}),
	}

	return validators.ValidateSecurityScanResult(imageResult)
}

// Helper conversion functions

func convertDetectedSecrets(secrets []ScannedSecret) []validators.DetectedSecret {
	var converted []validators.DetectedSecret
	for _, secret := range secrets {
		converted = append(converted, validators.DetectedSecret{
			Type:        secret.Type,
			Value:       secret.Value,
			File:        secret.File,
			Line:        secret.Line,
			Column:      0, // Column not available in ScannedSecret
			Severity:    secret.Severity,
			Confidence:  fmt.Sprintf("%d", secret.Confidence), // Convert int to string
			Description: secret.Context,                       // Use Context as Description
		})
	}
	return converted
}

func convertVulnerabilities(result AtomicScanImageSecurityResult) []validators.Vulnerability {
	var vulnerabilities []validators.Vulnerability

	// Convert from scan result if available
	if result.ScanResult != nil && result.ScanResult.Vulnerabilities != nil {
		for _, vuln := range result.ScanResult.Vulnerabilities {
			vulnerability := validators.Vulnerability{
				ID:           vuln.VulnerabilityID,
				Severity:     vuln.Severity,
				Score:        estimateScoreFromSeverity(vuln.Severity),
				Package:      vuln.PkgName,
				Version:      vuln.InstalledVersion,
				FixedVersion: vuln.FixedVersion,
				Title:        vuln.Title,
				Description:  vuln.Description,
				References:   vuln.References,
				Fixable:      vuln.FixedVersion != "",
			}
			vulnerabilities = append(vulnerabilities, vulnerability)
		}
	}

	return vulnerabilities
}

func convertCriticalFindings(findings []CriticalSecurityFinding) []validators.CriticalFinding {
	var converted []validators.CriticalFinding
	for _, finding := range findings {
		converted = append(converted, validators.CriticalFinding{
			Type:        finding.Type,
			Severity:    finding.Severity,
			Description: finding.Description,
			Impact:      finding.Impact,
			Remediation: finding.Remediation,
		})
	}
	return converted
}

func convertVulnerabilitySummary(summary VulnerabilityAnalysisSummary) validators.VulnerabilitySummary {
	return validators.VulnerabilitySummary{
		Total:      summary.TotalVulnerabilities,
		Critical:   summary.SeverityBreakdown["critical"],
		High:       summary.SeverityBreakdown["high"],
		Medium:     summary.SeverityBreakdown["medium"],
		Low:        summary.SeverityBreakdown["low"],
		Fixable:    summary.FixableVulnerabilities,
		BySeverity: summary.SeverityBreakdown,
	}
}

// estimateScoreFromSeverity provides a CVSS-like score estimate based on severity
func estimateScoreFromSeverity(severity string) float64 {
	switch severity {
	case "CRITICAL":
		return 9.5
	case "HIGH":
		return 7.5
	case "MEDIUM":
		return 5.0
	case "LOW":
		return 2.5
	default:
		return 0.0
	}
}

// ValidateSecretScanConfigUnified validates secret scan configuration
func ValidateSecretScanConfigUnified(config map[string]interface{}) *core.ValidationResult {
	return validators.ValidateSecretScanArgs(config)
}

// ValidateImageScanConfigUnified validates image scan configuration
func ValidateImageScanConfigUnified(config map[string]interface{}) *core.ValidationResult {
	return validators.ValidateImageScanArgs(config)
}

// GetSecretScanValidationMetrics returns validation metrics for secret scan
func GetSecretScanValidationMetrics(result AtomicScanSecretsResult) map[string]interface{} {
	validationResult := ValidateSecretScanResultUnified(result)

	metrics := map[string]interface{}{
		"validation_score":      validationResult.Score,
		"risk_level":            validationResult.RiskLevel,
		"error_count":           len(validationResult.Errors),
		"warning_count":         len(validationResult.Warnings),
		"validation_duration":   validationResult.Duration.String(),
		"scan_success":          result.SecretsFound >= 0, // Scan completed
		"secrets_found":         result.SecretsFound,
		"files_scanned":         result.FilesScanned,
		"security_score":        result.SecurityScore,
		"scan_duration_seconds": result.Duration.Seconds(),
	}

	// Add severity breakdown
	if result.SeverityBreakdown != nil {
		metrics["severity_breakdown"] = result.SeverityBreakdown
	}

	// Add risk assessment
	if result.SecretsFound > 0 {
		metrics["has_secrets"] = true
		metrics["secrets_per_file"] = float64(result.SecretsFound) / float64(commonUtils.Max(result.FilesScanned, 1))
	} else {
		metrics["has_secrets"] = false
	}

	return metrics
}

// GetImageScanValidationMetrics returns validation metrics for image scan
func GetImageScanValidationMetrics(result AtomicScanImageSecurityResult) map[string]interface{} {
	validationResult := ValidateImageScanResultUnified(result)

	metrics := map[string]interface{}{
		"validation_score":      validationResult.Score,
		"risk_level":            validationResult.RiskLevel,
		"error_count":           len(validationResult.Errors),
		"warning_count":         len(validationResult.Warnings),
		"validation_duration":   validationResult.Duration.String(),
		"scan_success":          result.Success,
		"security_score":        result.SecurityScore,
		"scan_duration_seconds": result.Duration.Seconds(),
		"scanner":               result.Scanner,
	}

	// Add vulnerability summary
	if result.VulnSummary.TotalVulnerabilities >= 0 {
		metrics["total_vulnerabilities"] = result.VulnSummary.TotalVulnerabilities
		metrics["critical_vulnerabilities"] = result.VulnSummary.SeverityBreakdown["critical"]
		metrics["high_vulnerabilities"] = result.VulnSummary.SeverityBreakdown["high"]
		metrics["medium_vulnerabilities"] = result.VulnSummary.SeverityBreakdown["medium"]
		metrics["low_vulnerabilities"] = result.VulnSummary.SeverityBreakdown["low"]
		metrics["fixable_vulnerabilities"] = result.VulnSummary.FixableVulnerabilities

		// Calculate risk ratios
		if result.VulnSummary.TotalVulnerabilities > 0 {
			criticalCount := result.VulnSummary.SeverityBreakdown["critical"]
			metrics["critical_ratio"] = float64(criticalCount) / float64(result.VulnSummary.TotalVulnerabilities)
			metrics["fixable_ratio"] = float64(result.VulnSummary.FixableVulnerabilities) / float64(result.VulnSummary.TotalVulnerabilities)
		}
	}

	// Add critical findings count
	metrics["critical_findings_count"] = len(result.CriticalFindings)

	return metrics
}

// ValidateScanPipeline validates an entire security scanning pipeline
func ValidateScanPipeline(config map[string]interface{}) *core.ValidationResult {
	ctx := context.Background()

	// Create security scan validator
	securityValidator := validators.NewSecurityScanValidator()

	// Combined validation result
	overallResult := &core.ValidationResult{
		Valid:    true,
		Errors:   make([]*core.ValidationError, 0),
		Warnings: make([]*core.ValidationWarning, 0),
		Metadata: core.ValidationMetadata{
			ValidatedAt:      time.Now(),
			ValidatorName:    "scan-pipeline-validator",
			ValidatorVersion: "1.0.0",
			Context:          make(map[string]interface{}),
		},
		Suggestions: make([]string, 0),
	}

	options := core.NewValidationOptions()

	// Validate scan configuration
	scanResult := securityValidator.Validate(ctx, config, options)
	overallResult.Merge(scanResult)

	// Pipeline-specific validations
	hasSecretScan := false
	hasImageScan := false

	if _, exists := config["secret_scan"]; exists {
		hasSecretScan = true
	}
	if _, exists := config["image_scan"]; exists {
		hasImageScan = true
	}

	if !hasSecretScan && !hasImageScan {
		overallResult.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "NO_SCAN_TYPES_CONFIGURED",
				Message:  "No scan types configured - pipeline will not perform any scans",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityMedium,
			},
		})
	}

	// Check for required session ID
	if _, hasSessionID := config["session_id"]; !hasSessionID {
		overallResult.AddError(&core.ValidationError{
			Code:     "MISSING_SESSION_ID",
			Message:  "Scan pipeline requires session ID",
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityCritical,
			Field:    "session_id",
		})
	}

	// Set risk level based on configuration
	if hasSecretScan && hasImageScan {
		overallResult.RiskLevel = "low" // Comprehensive scanning
	} else if hasSecretScan || hasImageScan {
		overallResult.RiskLevel = "medium" // Partial scanning
	} else {
		overallResult.RiskLevel = "high" // No scanning
	}

	return overallResult
}
