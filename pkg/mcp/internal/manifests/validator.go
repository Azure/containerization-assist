package manifests

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Azure/container-copilot/pkg/mcp/internal/observability"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
)

// Validator handles manifest validation
type Validator struct {
	logger       zerolog.Logger
	opsValidator *ops.ManifestValidator
}

// NewValidator creates a new manifest validator
func NewValidator(logger zerolog.Logger) *Validator {
	return &Validator{
		logger: logger.With().Str("component", "manifest_validator").Logger(),
		// Note: We would initialize the ops validator here with appropriate client
		// For now, we'll implement basic validation
	}
}

// ValidateDirectory validates all manifest files in a directory
func (v *Validator) ValidateDirectory(ctx context.Context, manifestPath string) (*ValidationSummary, error) {
	v.logger.Info().Str("path", manifestPath).Msg("Starting manifest validation")

	summary := &ValidationSummary{
		Valid:           true,
		TotalFiles:      0,
		ValidFiles:      0,
		InvalidFiles:    0,
		Results:         make(map[string]FileValidation),
		OverallSeverity: "info",
	}

	// Find all YAML files in the directory
	files, err := v.findManifestFiles(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to find manifest files: %w", err)
	}

	summary.TotalFiles = len(files)

	// Validate each file
	for _, file := range files {
		fileValidation, err := v.validateFile(ctx, file)
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
		} else {
			summary.InvalidFiles++
			summary.Valid = false
		}

		// Update overall severity
		if len(fileValidation.Errors) > 0 {
			summary.OverallSeverity = "error"
		} else if len(fileValidation.Warnings) > 0 && summary.OverallSeverity != "error" {
			summary.OverallSeverity = "warning"
		}
	}

	v.logger.Info().
		Bool("valid", summary.Valid).
		Int("total_files", summary.TotalFiles).
		Int("valid_files", summary.ValidFiles).
		Int("invalid_files", summary.InvalidFiles).
		Msg("Manifest validation completed")

	return summary, nil
}

// ValidateFile validates a single manifest file
func (v *Validator) ValidateFile(ctx context.Context, filePath string) (*FileValidation, error) {
	return v.validateFile(ctx, filePath)
}

// findManifestFiles finds all YAML manifest files in a directory
func (v *Validator) findManifestFiles(manifestPath string) ([]string, error) {
	var files []string

	err := filepath.Walk(manifestPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && v.isManifestFile(path) {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}

// isManifestFile checks if a file is a Kubernetes manifest file
func (v *Validator) isManifestFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml"
}

// validateFile validates a single manifest file
func (v *Validator) validateFile(ctx context.Context, filePath string) (*FileValidation, error) {
	validation := FileValidation{
		Valid:    true,
		Errors:   []ValidationIssue{},
		Warnings: []ValidationIssue{},
		Info:     []ValidationIssue{},
	}

	// Read the file
	content, err := os.ReadFile(filePath)
	if err != nil {
		validation.Valid = false
		validation.Errors = append(validation.Errors, ValidationIssue{
			Severity: "error",
			Message:  fmt.Sprintf("Failed to read file: %v", err),
		})
		return &validation, nil
	}

	// Basic YAML validation
	var manifest map[string]interface{}
	if err := yaml.Unmarshal(content, &manifest); err != nil {
		validation.Valid = false
		validation.Errors = append(validation.Errors, ValidationIssue{
			Severity: "error",
			Message:  fmt.Sprintf("Invalid YAML: %v", err),
		})
		return &validation, nil
	}

	// Basic Kubernetes manifest structure validation
	if err := v.validateBasicK8sStructure(manifest, &validation); err != nil {
		return &validation, err
	}

	// If we have an ops validator, use it for detailed validation
	if v.opsValidator != nil {
		// This would integrate with the existing validation system
		v.logger.Debug().Str("file", filePath).Msg("Performing detailed validation")
	}

	return &validation, nil
}

// validateBasicK8sStructure performs basic Kubernetes manifest structure validation
func (v *Validator) validateBasicK8sStructure(manifest map[string]interface{}, validation *FileValidation) error {
	// Check for required fields
	requiredFields := []string{"apiVersion", "kind", "metadata"}

	for _, field := range requiredFields {
		if _, exists := manifest[field]; !exists {
			validation.Valid = false
			validation.Errors = append(validation.Errors, ValidationIssue{
				Severity: "error",
				Message:  fmt.Sprintf("Missing required field: %s", field),
				Field:    field,
			})
		}
	}

	// Validate metadata structure if present
	if metadata, exists := manifest["metadata"]; exists {
		if metadataMap, ok := metadata.(map[string]interface{}); ok {
			if _, hasName := metadataMap["name"]; !hasName {
				validation.Warnings = append(validation.Warnings, ValidationIssue{
					Severity: "warning",
					Message:  "metadata.name is recommended",
					Field:    "metadata.name",
				})
			}
		}
	}

	// Validate spec structure for common resources
	if kind, exists := manifest["kind"]; exists {
		if kindStr, ok := kind.(string); ok {
			v.validateResourceSpecificFields(kindStr, manifest, validation)
		}
	}

	return nil
}

// validateResourceSpecificFields validates fields specific to resource types
func (v *Validator) validateResourceSpecificFields(kind string, manifest map[string]interface{}, validation *FileValidation) {
	switch strings.ToLower(kind) {
	case "deployment":
		v.validateDeploymentFields(manifest, validation)
	case "service":
		v.validateServiceFields(manifest, validation)
	case "ingress":
		v.validateIngressFields(manifest, validation)
	case "configmap":
		v.validateConfigMapFields(manifest, validation)
	case "secret":
		v.validateSecretFields(manifest, validation)
	}
}

func (v *Validator) validateDeploymentFields(manifest map[string]interface{}, validation *FileValidation) {
	if spec, exists := manifest["spec"]; exists {
		if specMap, ok := spec.(map[string]interface{}); ok {
			// Check for template
			if _, hasTemplate := specMap["template"]; !hasTemplate {
				validation.Errors = append(validation.Errors, ValidationIssue{
					Severity: "error",
					Message:  "Deployment spec must have template field",
					Field:    "spec.template",
				})
				validation.Valid = false
			}

			// Check for selector
			if _, hasSelector := specMap["selector"]; !hasSelector {
				validation.Errors = append(validation.Errors, ValidationIssue{
					Severity: "error",
					Message:  "Deployment spec must have selector field",
					Field:    "spec.selector",
				})
				validation.Valid = false
			}
		}
	}
}

func (v *Validator) validateServiceFields(manifest map[string]interface{}, validation *FileValidation) {
	if spec, exists := manifest["spec"]; exists {
		if specMap, ok := spec.(map[string]interface{}); ok {
			// Check for ports
			if ports, hasPorts := specMap["ports"]; hasPorts {
				if portsList, ok := ports.([]interface{}); ok && len(portsList) == 0 {
					validation.Warnings = append(validation.Warnings, ValidationIssue{
						Severity: "warning",
						Message:  "Service has empty ports list",
						Field:    "spec.ports",
					})
				}
			} else {
				validation.Warnings = append(validation.Warnings, ValidationIssue{
					Severity: "warning",
					Message:  "Service should define ports",
					Field:    "spec.ports",
				})
			}
		}
	}
}

func (v *Validator) validateIngressFields(manifest map[string]interface{}, validation *FileValidation) {
	if spec, exists := manifest["spec"]; exists {
		if specMap, ok := spec.(map[string]interface{}); ok {
			// Check for rules
			if _, hasRules := specMap["rules"]; !hasRules {
				validation.Warnings = append(validation.Warnings, ValidationIssue{
					Severity: "warning",
					Message:  "Ingress should define rules",
					Field:    "spec.rules",
				})
			}
		}
	}
}

func (v *Validator) validateConfigMapFields(manifest map[string]interface{}, validation *FileValidation) {
	// ConfigMaps should have either data or binaryData
	hasData := false
	if _, exists := manifest["data"]; exists {
		hasData = true
	}
	if _, exists := manifest["binaryData"]; exists {
		hasData = true
	}

	if !hasData {
		validation.Warnings = append(validation.Warnings, ValidationIssue{
			Severity: "warning",
			Message:  "ConfigMap should have data or binaryData",
			Field:    "data",
		})
	}
}

func (v *Validator) validateSecretFields(manifest map[string]interface{}, validation *FileValidation) {
	// Secrets should have data
	if _, hasData := manifest["data"]; !hasData {
		validation.Warnings = append(validation.Warnings, ValidationIssue{
			Severity: "warning",
			Message:  "Secret should have data field",
			Field:    "data",
		})
	}
}
