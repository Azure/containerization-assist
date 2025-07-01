package deploy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/validation/core"
	"github.com/Azure/container-kit/pkg/mcp/validation/validators"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
)

// UnifiedValidator wraps the new Kubernetes validator with the old interface
type UnifiedValidator struct {
	logger            zerolog.Logger
	k8sValidator      *validators.KubernetesValidator
	yamlValidator     *validators.FormatValidator
	originalValidator *Validator
}

// NewUnifiedValidator creates a new unified manifest validator
func NewUnifiedValidator(logger zerolog.Logger) *Validator {
	// For backward compatibility, return the original Validator type
	// but internally use the unified validation system
	return &Validator{
		logger: logger.With().Str("component", "unified_manifest_validator").Logger(),
	}
}

// ValidateDirectoryUnified validates all manifest files in a directory using unified validation
func (v *Validator) ValidateDirectoryUnified(ctx context.Context, manifestPath string) (*ValidationSummary, *core.ValidationResult, error) {
	// Create unified validators
	k8sValidator := validators.NewKubernetesValidator().WithSecurityValidation(true)
	yamlValidator := validators.NewFormatValidator()

	v.logger.Info().Str("path", manifestPath).Msg("Starting unified manifest validation")

	summary := &ValidationSummary{
		Valid:           true,
		TotalFiles:      0,
		ValidFiles:      0,
		InvalidFiles:    0,
		Results:         make(map[string]FileValidation),
		OverallSeverity: "info",
	}

	// Overall validation result
	overallResult := &core.ValidationResult{
		Valid:    true,
		Errors:   make([]*core.ValidationError, 0),
		Warnings: make([]*core.ValidationWarning, 0),
		Metadata: core.ValidationMetadata{
			ValidatedAt:      time.Now(),
			ValidatorName:    "unified-manifest-validator",
			ValidatorVersion: "1.0.0",
			Context:          make(map[string]interface{}),
		},
		Suggestions: make([]string, 0),
	}

	// Find all YAML files in the directory
	files, err := v.findManifestFiles(manifestPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find manifest files: %w", err)
	}

	summary.TotalFiles = len(files)
	overallResult.Metadata.Context["total_files"] = len(files)

	// Validate each file
	validFiles := 0
	for _, file := range files {
		fileValidation, fileResult, err := v.validateFileUnified(ctx, file, k8sValidator, yamlValidator)
		if err != nil {
			v.logger.Warn().Str("file", file).Err(err).Msg("Failed to validate file")
			fileValidation = &FileValidation{
				Valid: false,
				Errors: []ValidationIssue{{
					Severity: "error",
					Message:  fmt.Sprintf("Failed to validate file: %v", err),
				}},
			}
		}

		fileName := filepath.Base(file)
		summary.Results[fileName] = *fileValidation

		if fileValidation.Valid {
			summary.ValidFiles++
			validFiles++
		} else {
			summary.InvalidFiles++
			summary.Valid = false
		}

		// Merge file validation results into overall result
		if fileResult != nil {
			overallResult.Merge(fileResult)
		}

		// Update overall severity
		if len(fileValidation.Errors) > 0 {
			summary.OverallSeverity = "error"
		} else if len(fileValidation.Warnings) > 0 && summary.OverallSeverity != "error" {
			summary.OverallSeverity = "warning"
		}
	}

	// Add summary metrics to overall result
	overallResult.Metadata.Context["valid_files"] = validFiles
	overallResult.Metadata.Context["invalid_files"] = summary.InvalidFiles
	overallResult.Metadata.Context["validation_summary"] = summary

	// Calculate overall score
	if len(files) > 0 {
		score := float64(validFiles) / float64(len(files)) * 100
		overallResult.Score = score
	}

	v.logger.Info().
		Bool("valid", summary.Valid).
		Int("total_files", summary.TotalFiles).
		Int("valid_files", summary.ValidFiles).
		Int("invalid_files", summary.InvalidFiles).
		Msg("Unified manifest validation completed")

	return summary, overallResult, nil
}

// ValidateFileUnified validates a single manifest file using unified validation
func (v *Validator) ValidateFileUnified(ctx context.Context, filePath string) (*FileValidation, *core.ValidationResult, error) {
	k8sValidator := validators.NewKubernetesValidator().WithSecurityValidation(true)
	yamlValidator := validators.NewFormatValidator()

	return v.validateFileUnified(ctx, filePath, k8sValidator, yamlValidator)
}

// validateFileUnified validates a single manifest file with unified validators
func (v *Validator) validateFileUnified(ctx context.Context, filePath string, k8sValidator *validators.KubernetesValidator, yamlValidator *validators.FormatValidator) (*FileValidation, *core.ValidationResult, error) {
	fileValidation := FileValidation{
		Valid:    true,
		Errors:   []ValidationIssue{},
		Warnings: []ValidationIssue{},
		Info:     []ValidationIssue{},
	}

	// Read the file
	content, err := os.ReadFile(filePath)
	if err != nil {
		fileValidation.Valid = false
		fileValidation.Errors = append(fileValidation.Errors, ValidationIssue{
			Severity: "error",
			Message:  fmt.Sprintf("Failed to read file: %v", err),
		})
		return &fileValidation, nil, nil
	}

	// Create validation options
	options := core.NewValidationOptions().WithStrictMode(false)
	options.IncludeWarnings = true

	// First validate YAML format
	yamlResult := yamlValidator.Validate(ctx, string(content), options)
	if !yamlResult.Valid {
		// Convert YAML validation errors
		for _, err := range yamlResult.Errors {
			fileValidation.Errors = append(fileValidation.Errors, ValidationIssue{
				Severity: string(err.Severity),
				Message:  err.Message,
				Field:    err.Field,
			})
			fileValidation.Valid = false
		}
		return &fileValidation, yamlResult, nil
	}

	// Parse YAML for Kubernetes validation
	var manifests []map[string]interface{}

	// Try to parse as multi-document YAML first
	decoder := yaml.NewDecoder(strings.NewReader(string(content)))
	for {
		var manifest map[string]interface{}
		if err := decoder.Decode(&manifest); err != nil {
			if err.Error() == "EOF" {
				break
			}
			// If multi-document parsing fails, try single document
			var singleManifest map[string]interface{}
			if err := yaml.Unmarshal(content, &singleManifest); err != nil {
				fileValidation.Valid = false
				fileValidation.Errors = append(fileValidation.Errors, ValidationIssue{
					Severity: "error",
					Message:  fmt.Sprintf("Invalid YAML: %v", err),
				})
				return &fileValidation, nil, nil
			}
			manifests = append(manifests, singleManifest)
			break
		}
		if manifest != nil {
			manifests = append(manifests, manifest)
		}
	}

	// If no manifests found, try parsing as single document
	if len(manifests) == 0 {
		var manifest map[string]interface{}
		if err := yaml.Unmarshal(content, &manifest); err != nil {
			fileValidation.Valid = false
			fileValidation.Errors = append(fileValidation.Errors, ValidationIssue{
				Severity: "error",
				Message:  fmt.Sprintf("Invalid YAML: %v", err),
			})
			return &fileValidation, nil, nil
		}
		if manifest != nil {
			manifests = append(manifests, manifest)
		}
	}

	// Overall result for this file
	overallResult := &core.ValidationResult{
		Valid:    true,
		Errors:   make([]*core.ValidationError, 0),
		Warnings: make([]*core.ValidationWarning, 0),
		Metadata: core.ValidationMetadata{
			ValidatedAt:      time.Now(),
			ValidatorName:    "file-validator",
			ValidatorVersion: "1.0.0",
			Context:          map[string]interface{}{"file_path": filePath},
		},
	}

	// Validate each manifest using Kubernetes validator
	for i, manifest := range manifests {
		manifestResult := k8sValidator.Validate(ctx, manifest, options)

		// Convert Kubernetes validation results to file validation format
		v.convertValidationResult(manifestResult, &fileValidation, i)

		// Merge into overall file result
		overallResult.Merge(manifestResult)
	}

	// Add file-level information
	overallResult.Metadata.Context["manifest_count"] = len(manifests)
	overallResult.Metadata.Context["file_size"] = len(content)

	// Perform legacy validation for compatibility
	legacyValidation, err := v.validateFile(ctx, filePath)
	if err == nil && legacyValidation != nil {
		// Merge any additional issues from legacy validation
		for _, issue := range legacyValidation.Errors {
			if !v.containsIssue(fileValidation.Errors, issue) {
				fileValidation.Errors = append(fileValidation.Errors, issue)
				fileValidation.Valid = false
			}
		}
		for _, issue := range legacyValidation.Warnings {
			if !v.containsIssue(fileValidation.Warnings, issue) {
				fileValidation.Warnings = append(fileValidation.Warnings, issue)
			}
		}
	}

	return &fileValidation, overallResult, nil
}

// convertValidationResult converts unified validation result to legacy format
func (v *Validator) convertValidationResult(result *core.ValidationResult, fileValidation *FileValidation, manifestIndex int) {
	prefix := ""
	if manifestIndex >= 0 {
		prefix = fmt.Sprintf("manifest[%d]: ", manifestIndex)
	}

	// Convert errors
	for _, err := range result.Errors {
		issue := ValidationIssue{
			Severity: string(err.Severity),
			Message:  prefix + err.Message,
			Field:    err.Field,
		}
		fileValidation.Errors = append(fileValidation.Errors, issue)
		fileValidation.Valid = false
	}

	// Convert warnings
	for _, warning := range result.Warnings {
		issue := ValidationIssue{
			Severity: string(warning.Severity),
			Message:  prefix + warning.Message,
			Field:    warning.Field,
		}
		fileValidation.Warnings = append(fileValidation.Warnings, issue)
	}

	// Add suggestions as info
	for _, suggestion := range result.Suggestions {
		issue := ValidationIssue{
			Severity: "info",
			Message:  prefix + "Suggestion: " + suggestion,
		}
		fileValidation.Info = append(fileValidation.Info, issue)
	}
}

// containsIssue checks if an issue already exists in the list
func (v *Validator) containsIssue(issues []ValidationIssue, newIssue ValidationIssue) bool {
	for _, issue := range issues {
		if issue.Message == newIssue.Message && issue.Field == newIssue.Field {
			return true
		}
	}
	return false
}

// ValidateManifestContent validates manifest content directly using unified validation
func ValidateManifestContent(content []byte, options *core.ValidationOptions) *core.ValidationResult {
	k8sValidator := validators.NewKubernetesValidator().WithSecurityValidation(true)
	ctx := context.Background()

	if options == nil {
		options = core.NewValidationOptions()
	}

	return k8sValidator.Validate(ctx, content, options)
}

// ValidateManifestString validates manifest YAML string using unified validation
func ValidateManifestString(yamlContent string, options *core.ValidationOptions) *core.ValidationResult {
	return ValidateManifestContent([]byte(yamlContent), options)
}

// ValidateManifestMap validates a parsed manifest map using unified validation
func ValidateManifestMap(manifest map[string]interface{}, options *core.ValidationOptions) *core.ValidationResult {
	k8sValidator := validators.NewKubernetesValidator().WithSecurityValidation(true)
	ctx := context.Background()

	if options == nil {
		options = core.NewValidationOptions()
	}

	return k8sValidator.Validate(ctx, manifest, options)
}

// ValidateKubernetesResource validates a Kubernetes resource with specific validation rules
func ValidateKubernetesResource(resource map[string]interface{}, strictMode bool, securityValidation bool) *core.ValidationResult {
	k8sValidator := validators.NewKubernetesValidator().
		WithStrictMode(strictMode).
		WithSecurityValidation(securityValidation)

	ctx := context.Background()
	options := core.NewValidationOptions().WithStrictMode(strictMode)

	return k8sValidator.Validate(ctx, resource, options)
}

// GetManifestValidationMetrics returns validation metrics for a directory
func (v *Validator) GetManifestValidationMetrics(ctx context.Context, manifestPath string) (map[string]interface{}, error) {
	_, overallResult, err := v.ValidateDirectoryUnified(ctx, manifestPath)
	if err != nil {
		return nil, err
	}

	metrics := make(map[string]interface{})
	if overallResult != nil && overallResult.Metadata.Context != nil {
		metrics = overallResult.Metadata.Context

		// Add computed metrics
		if totalFiles, ok := metrics["total_files"].(int); ok && totalFiles > 0 {
			if validFiles, ok := metrics["valid_files"].(int); ok {
				metrics["success_rate"] = float64(validFiles) / float64(totalFiles)
				metrics["failure_rate"] = float64(totalFiles-validFiles) / float64(totalFiles)
			}
		}

		// Add validation counts
		metrics["error_count"] = len(overallResult.Errors)
		metrics["warning_count"] = len(overallResult.Warnings)
		metrics["suggestion_count"] = len(overallResult.Suggestions)

		// Add risk assessment
		if overallResult.RiskLevel != "" {
			metrics["risk_level"] = overallResult.RiskLevel
		}

		if overallResult.Score > 0 {
			metrics["validation_score"] = overallResult.Score
		}
	}

	return metrics, nil
}
