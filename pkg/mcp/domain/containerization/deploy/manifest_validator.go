package deploy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"log/slog"

	"github.com/Azure/container-kit/pkg/common/validation-core/core"
	"github.com/Azure/container-kit/pkg/common/validation-core/validators"
	"gopkg.in/yaml.v3"
)

// Validator handles manifest validation using unified validation framework
type Validator struct {
	logger          *slog.Logger
	deployValidator core.Validator
	k8sValidator    *validators.KubernetesValidator
}

// NewValidator creates a new manifest validator with unified validation framework
func NewValidator(logger *slog.Logger) *Validator {
	return &Validator{
		logger:          logger.With("component", "unified_manifest_validator"),
		deployValidator: validators.NewDeploymentValidator(),
		k8sValidator:    validators.NewKubernetesValidator(),
	}
}

// UnifiedValidator provides a unified validation interface
type UnifiedValidator struct {
	impl *Validator
}

// NewUnifiedValidator creates a new unified manifest validator
func NewUnifiedValidator(logger *slog.Logger) *UnifiedValidator {
	return &UnifiedValidator{
		impl: NewValidator(logger),
	}
}

// ValidateDirectoryUnified validates all manifest files in a directory using unified validation
func (v *Validator) ValidateDirectoryUnified(ctx context.Context, manifestPath string) (*core.DeployResult, error) {
	v.logger.Info("Starting unified manifest validation", "path", manifestPath)

	// Create deploy validation data
	deployData := map[string]interface{}{
		"manifest_path": manifestPath,
		"cluster_info":  make(map[string]interface{}),
		"resources":     []interface{}{},
		"health_checks": []interface{}{},
	}

	// Find all manifest files
	files, err := v.findManifestFiles(manifestPath)
	if err != nil {
		result := core.NewDeployResult("unified_manifest_validator", "1.0.0")
		result.AddError(core.NewError("MANIFEST_DISCOVERY_ERROR", fmt.Sprintf("Failed to find manifest files: %v", err), core.ErrTypeDeployment, core.SeverityCritical))
		return result, err
	}

	// Add files to cluster info
	clusterInfo := deployData["cluster_info"].(map[string]interface{})
	clusterInfo["manifest_files"] = files
	clusterInfo["total_files"] = len(files)

	// Parse and validate each manifest file
	resources := []interface{}{}
	for _, file := range files {
		resource, err := v.parseManifestFile(file)
		if err != nil {
			v.logger.Warn("Failed to parse manifest file", "file", file, "error", err)
			continue
		}
		if resource != nil {
			resources = append(resources, map[string]interface{}{
				"api_version": resource.APIVersion,
				"kind":        resource.Kind,
				"name":        resource.Name,
				"namespace":   resource.Namespace,
			})
		}
	}
	deployData["resources"] = resources

	// Perform unified validation
	options := core.NewValidationOptions().WithStrictMode(true)
	nonGenericResult := v.deployValidator.Validate(ctx, deployData, options)

	// Convert NonGenericResult to DeployResult
	result := core.NewDeployResult("unified_manifest_validator", "1.0.0")
	result.Valid = nonGenericResult.Valid
	result.Errors = nonGenericResult.Errors
	result.Warnings = nonGenericResult.Warnings
	result.Suggestions = nonGenericResult.Suggestions
	result.Duration = nonGenericResult.Duration
	result.Metadata = nonGenericResult.Metadata

	v.logger.Info("Unified manifest validation completed",
		"valid", result.Valid,
		"errors", len(result.Errors),
		"warnings", len(result.Warnings),
		"files_processed", len(files))

	return result, nil
}

// parseManifestFile parses a manifest file and returns a Kubernetes resource
func (v *Validator) parseManifestFile(filePath string) (*core.KubernetesResource, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	var manifest map[string]interface{}
	if err := yaml.Unmarshal(content, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse YAML in %s: %w", filePath, err)
	}

	// Extract basic resource information
	resource := &core.KubernetesResource{}

	if apiVersion, ok := manifest["apiVersion"].(string); ok {
		resource.APIVersion = apiVersion
	}

	if kind, ok := manifest["kind"].(string); ok {
		resource.Kind = kind
	}

	if metadata, ok := manifest["metadata"].(map[string]interface{}); ok {
		if name, ok := metadata["name"].(string); ok {
			resource.Name = name
		}
		if namespace, ok := metadata["namespace"].(string); ok {
			resource.Namespace = namespace
		}
	}

	return resource, nil
}

// ValidateDirectory validates all manifest files in a directory using unified validation
func (v *Validator) ValidateDirectory(ctx context.Context, manifestPath string) (*core.DeployResult, error) {
	return v.ValidateDirectoryUnified(ctx, manifestPath)
}

// ValidateFile validates a single manifest file using unified validation
func (v *Validator) ValidateFile(ctx context.Context, filePath string) (*core.DeployResult, error) {
	v.logger.Info("Validating single manifest file", "file", filePath)

	// Parse the file to get resource information
	resource, err := v.parseManifestFile(filePath)
	if err != nil {
		result := core.NewDeployResult("unified_manifest_validator", "1.0.0")
		result.AddError(core.NewError("FILE_PARSE_ERROR", fmt.Sprintf("Failed to parse manifest file %s: %v", filePath, err), core.ErrTypeDeployment, core.SeverityCritical))
		return result, err
	}

	// Create deploy validation data for single file
	deployData := map[string]interface{}{
		"manifest_path": filePath,
		"cluster_info":  map[string]interface{}{"single_file": true},
		"resources": []interface{}{
			map[string]interface{}{
				"api_version": resource.APIVersion,
				"kind":        resource.Kind,
				"name":        resource.Name,
				"namespace":   resource.Namespace,
			},
		},
		"health_checks": []interface{}{},
	}

	// Perform unified validation
	options := core.NewValidationOptions().WithStrictMode(true)
	nonGenericResult := v.deployValidator.Validate(ctx, deployData, options)

	// Convert NonGenericResult to DeployResult
	result := core.NewDeployResult("unified_manifest_validator", "1.0.0")
	result.Valid = nonGenericResult.Valid
	result.Errors = nonGenericResult.Errors
	result.Warnings = nonGenericResult.Warnings
	result.Suggestions = nonGenericResult.Suggestions
	result.Duration = nonGenericResult.Duration
	result.Metadata = nonGenericResult.Metadata

	v.logger.Info("Single file validation completed",
		"valid", result.Valid,
		"errors", len(result.Errors),
		"warnings", len(result.Warnings),
		"file", filePath)

	return result, nil
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

// Unified validation interface methods for UnifiedValidator

// Validate implements the GenericValidator interface
func (uv *UnifiedValidator) Validate(ctx context.Context, data core.DeployValidationData, options *core.ValidationOptions) *core.DeployResult {
	result, err := uv.impl.ValidateDirectoryUnified(ctx, data.ManifestPath)
	if err != nil {
		if result == nil {
			result = core.NewDeployResult("unified_deploy_validator", "1.0.0")
		}
		result.AddError(core.NewError("VALIDATION_ERROR", err.Error(), core.ErrTypeDeployment, core.SeverityHigh))
	}
	return result
}

// GetName returns the validator name
func (uv *UnifiedValidator) GetName() string {
	return "unified_deploy_validator"
}

// GetVersion returns the validator version
func (uv *UnifiedValidator) GetVersion() string {
	return "1.0.0"
}

// GetSupportedTypes returns the data types this validator can handle
func (uv *UnifiedValidator) GetSupportedTypes() []string {
	return []string{"DeployValidationData", "KubernetesDeployParams", "map[string]interface{}"}
}

// ValidateWithHealthChecks performs validation and adds health check information
func (uv *UnifiedValidator) ValidateWithHealthChecks(ctx context.Context, manifestPath string, healthChecks []core.HealthCheck) (*core.DeployResult, error) {
	// Use the directory validation method
	result, err := uv.impl.ValidateDirectoryUnified(ctx, manifestPath)
	if err != nil {
		return result, err
	}

	// Add health check validations to suggestions
	if len(healthChecks) > 0 {
		result.AddSuggestion("Health checks configured - ensure they're properly implemented")
	} else {
		result.AddSuggestion("Consider adding health checks for better deployment reliability")
	}

	return result, nil
}

// Migration helpers for backward compatibility

// MigrateManifestValidatorToUnifiedV2 provides a drop-in replacement for legacy ManifestValidator
func MigrateManifestValidatorToUnifiedV2(logger *slog.Logger) *UnifiedValidator {
	return NewUnifiedValidator(logger)
}
