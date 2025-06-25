package observability

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
)

// ManifestValidator validates Kubernetes manifests against API schemas
type ManifestValidator struct {
	logger    zerolog.Logger
	k8sClient K8sValidationClient
}

// K8sValidationClient interface for Kubernetes validation operations
type K8sValidationClient interface {
	ValidateManifest(ctx context.Context, manifest []byte) (*ValidationResult, error)
	GetSupportedVersions(ctx context.Context) ([]string, error)
	DryRunManifest(ctx context.Context, manifest []byte) (*DryRunResult, error)
}

// ValidationResult represents the result of manifest validation
type ValidationResult struct {
	Valid         bool                `json:"valid"`
	Errors        []ValidationError   `json:"errors,omitempty"`
	Warnings      []ValidationWarning `json:"warnings,omitempty"`
	APIVersion    string              `json:"api_version"`
	Kind          string              `json:"kind"`
	Name          string              `json:"name,omitempty"`
	Namespace     string              `json:"namespace,omitempty"`
	Suggestions   []string            `json:"suggestions,omitempty"`
	SchemaVersion string              `json:"schema_version,omitempty"`
	Timestamp     time.Time           `json:"timestamp"`
	Duration      time.Duration       `json:"duration"`
}

// ValidationError represents a validation error
type ValidationError struct {
	Field    string                 `json:"field"`
	Message  string                 `json:"message"`
	Code     string                 `json:"code,omitempty"`
	Severity ValidationSeverity     `json:"severity"`
	Path     string                 `json:"path,omitempty"`
	Details  map[string]interface{} `json:"details,omitempty"`
}

// ValidationWarning represents a validation warning
type ValidationWarning struct {
	Field      string                 `json:"field"`
	Message    string                 `json:"message"`
	Code       string                 `json:"code,omitempty"`
	Path       string                 `json:"path,omitempty"`
	Suggestion string                 `json:"suggestion,omitempty"`
	Details    map[string]interface{} `json:"details,omitempty"`
}

// ValidationSeverity represents the severity of a validation issue
type ValidationSeverity string

const (
	SeverityCritical ValidationSeverity = "critical"
	SeverityError    ValidationSeverity = "error"
	SeverityWarning  ValidationSeverity = "warning"
	SeverityInfo     ValidationSeverity = "info"
)

// DryRunResult represents the result of a dry-run validation
type DryRunResult struct {
	Accepted  bool                `json:"accepted"`
	Errors    []ValidationError   `json:"errors,omitempty"`
	Warnings  []ValidationWarning `json:"warnings,omitempty"`
	Mutations []string            `json:"mutations,omitempty"`
	Events    []K8sEvent          `json:"events,omitempty"`
	Timestamp time.Time           `json:"timestamp"`
	Duration  time.Duration       `json:"duration"`
}

// K8sEvent represents a Kubernetes event from dry-run
type K8sEvent struct {
	Type      string    `json:"type"`
	Reason    string    `json:"reason"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// ManifestValidationOptions holds options for manifest validation
type ManifestValidationOptions struct {
	K8sVersion           string   `json:"k8s_version,omitempty"`
	SkipDryRun           bool     `json:"skip_dry_run"`
	SkipSchemaValidation bool     `json:"skip_schema_validation"`
	AllowedKinds         []string `json:"allowed_kinds,omitempty"`
	RequiredLabels       []string `json:"required_labels,omitempty"`
	ForbiddenFields      []string `json:"forbidden_fields,omitempty"`
	StrictValidation     bool     `json:"strict_validation"`
}

// BatchValidationResult represents results for multiple manifests
type BatchValidationResult struct {
	Results        map[string]*ValidationResult `json:"results"`
	OverallValid   bool                         `json:"overall_valid"`
	TotalManifests int                          `json:"total_manifests"`
	ValidManifests int                          `json:"valid_manifests"`
	ErrorCount     int                          `json:"error_count"`
	WarningCount   int                          `json:"warning_count"`
	Duration       time.Duration                `json:"duration"`
	Timestamp      time.Time                    `json:"timestamp"`
}

// NewManifestValidator creates a new manifest validator
func NewManifestValidator(logger zerolog.Logger, k8sClient K8sValidationClient) *ManifestValidator {
	return &ManifestValidator{
		logger:    logger,
		k8sClient: k8sClient,
	}
}

// ValidateManifestFile validates a single manifest file
func (mv *ManifestValidator) ValidateManifestFile(ctx context.Context, filePath string, options ManifestValidationOptions) (*ValidationResult, error) {
	start := time.Now()

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest file %s: %w", filePath, err)
	}

	result, err := mv.ValidateManifestContent(ctx, content, options)
	if err != nil {
		return nil, err
	}

	result.Duration = time.Since(start)

	mv.logger.Debug().
		Str("file_path", filePath).
		Bool("valid", result.Valid).
		Int("error_count", len(result.Errors)).
		Int("warning_count", len(result.Warnings)).
		Dur("duration", result.Duration).
		Msg("Manifest file validation completed")

	return result, nil
}

// ValidateManifestContent validates manifest content directly
func (mv *ManifestValidator) ValidateManifestContent(ctx context.Context, content []byte, options ManifestValidationOptions) (*ValidationResult, error) {
	start := time.Now()

	result := &ValidationResult{
		Valid:     true,
		Errors:    []ValidationError{},
		Warnings:  []ValidationWarning{},
		Timestamp: start,
	}

	// Parse the manifest to extract basic info
	var manifest map[string]interface{}
	if err := yaml.Unmarshal(content, &manifest); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:    "document",
			Message:  fmt.Sprintf("Invalid YAML: %v", err),
			Code:     "INVALID_YAML",
			Severity: SeverityCritical,
		})
		result.Duration = time.Since(start)
		return result, nil
	}

	// Extract basic manifest information
	if apiVersion, ok := manifest["apiVersion"].(string); ok {
		result.APIVersion = apiVersion
	}
	if kind, ok := manifest["kind"].(string); ok {
		result.Kind = kind
	}
	if metadata, ok := manifest["metadata"].(map[string]interface{}); ok {
		if name, ok := metadata["name"].(string); ok {
			result.Name = name
		}
		if namespace, ok := metadata["namespace"].(string); ok {
			result.Namespace = namespace
		}
	}

	// Perform basic structure validation
	mv.validateBasicStructure(manifest, result)

	// Validate required fields
	mv.validateRequiredFields(manifest, result)

	// Validate against allowed kinds
	if len(options.AllowedKinds) > 0 {
		mv.validateAllowedKinds(result.Kind, options.AllowedKinds, result)
	}

	// Validate required labels
	if len(options.RequiredLabels) > 0 {
		mv.validateRequiredLabels(manifest, options.RequiredLabels, result)
	}

	// Validate forbidden fields
	if len(options.ForbiddenFields) > 0 {
		mv.validateForbiddenFields(manifest, options.ForbiddenFields, result)
	}

	// Perform schema validation if not skipped and we have a k8s client
	if !options.SkipSchemaValidation && mv.k8sClient != nil {
		schemaResult, err := mv.k8sClient.ValidateManifest(ctx, content)
		if err != nil {
			mv.logger.Warn().Err(err).Msg("Schema validation failed")
			result.Warnings = append(result.Warnings, ValidationWarning{
				Field:   "schema",
				Message: fmt.Sprintf("Schema validation unavailable: %v", err),
				Code:    "SCHEMA_UNAVAILABLE",
			})
		} else if schemaResult != nil {
			// Merge schema validation results
			result.Errors = append(result.Errors, schemaResult.Errors...)
			result.Warnings = append(result.Warnings, schemaResult.Warnings...)
			result.SchemaVersion = schemaResult.SchemaVersion
			if !schemaResult.Valid {
				result.Valid = false
			}
		}
	}

	// Perform dry-run validation if not skipped
	if !options.SkipDryRun && mv.k8sClient != nil {
		dryRunResult, err := mv.k8sClient.DryRunManifest(ctx, content)
		if err != nil {
			mv.logger.Warn().Err(err).Msg("Dry-run validation failed")
			result.Warnings = append(result.Warnings, ValidationWarning{
				Field:   "dry_run",
				Message: fmt.Sprintf("Dry-run validation unavailable: %v", err),
				Code:    "DRY_RUN_UNAVAILABLE",
			})
		} else if dryRunResult != nil && !dryRunResult.Accepted {
			result.Valid = false
			result.Errors = append(result.Errors, dryRunResult.Errors...)
			result.Warnings = append(result.Warnings, dryRunResult.Warnings...)
		}
	}

	// Generate suggestions for common issues
	mv.generateSuggestions(result)

	// Final validation status
	if len(result.Errors) > 0 {
		for _, err := range result.Errors {
			if err.Severity == SeverityCritical || err.Severity == SeverityError {
				result.Valid = false
				break
			}
		}
	}

	result.Duration = time.Since(start)
	return result, nil
}

// ValidateManifestDirectory validates all manifests in a directory
func (mv *ManifestValidator) ValidateManifestDirectory(ctx context.Context, dirPath string, options ManifestValidationOptions) (*BatchValidationResult, error) {
	start := time.Now()

	result := &BatchValidationResult{
		Results:      make(map[string]*ValidationResult),
		OverallValid: true,
		Timestamp:    start,
	}

	// Find all YAML manifest files
	manifestFiles, err := mv.findManifestFiles(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to find manifest files: %w", err)
	}

	result.TotalManifests = len(manifestFiles)

	// Validate each manifest file
	for _, filePath := range manifestFiles {
		validationResult, err := mv.ValidateManifestFile(ctx, filePath, options)
		if err != nil {
			mv.logger.Error().
				Str("file_path", filePath).
				Err(err).
				Msg("Failed to validate manifest file")

			// Create error result for failed validation
			validationResult = &ValidationResult{
				Valid: false,
				Errors: []ValidationError{
					{
						Field:    "file",
						Message:  fmt.Sprintf("Validation failed: %v", err),
						Code:     "VALIDATION_ERROR",
						Severity: SeverityError,
					},
				},
				Timestamp: time.Now(),
			}
		}

		relPath, _ := filepath.Rel(dirPath, filePath)
		result.Results[relPath] = validationResult

		if validationResult.Valid {
			result.ValidManifests++
		} else {
			result.OverallValid = false
		}

		result.ErrorCount += len(validationResult.Errors)
		result.WarningCount += len(validationResult.Warnings)
	}

	result.Duration = time.Since(start)

	mv.logger.Info().
		Str("directory", dirPath).
		Int("total_manifests", result.TotalManifests).
		Int("valid_manifests", result.ValidManifests).
		Int("error_count", result.ErrorCount).
		Int("warning_count", result.WarningCount).
		Bool("overall_valid", result.OverallValid).
		Dur("duration", result.Duration).
		Msg("Manifest directory validation completed")

	return result, nil
}

// validateBasicStructure validates basic Kubernetes manifest structure
func (mv *ManifestValidator) validateBasicStructure(manifest map[string]interface{}, result *ValidationResult) {
	// Check required top-level fields
	requiredFields := []string{"apiVersion", "kind", "metadata"}
	for _, field := range requiredFields {
		if _, exists := manifest[field]; !exists {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:    field,
				Message:  fmt.Sprintf("Missing required field: %s", field),
				Code:     "MISSING_REQUIRED_FIELD",
				Severity: SeverityError,
				Path:     field,
			})
		}
	}

	// Validate apiVersion format
	if apiVersion, ok := manifest["apiVersion"].(string); ok {
		if !strings.Contains(apiVersion, "/") && !isBuiltinAPIVersion(apiVersion) {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Field:      "apiVersion",
				Message:    fmt.Sprintf("Unusual apiVersion format: %s", apiVersion),
				Code:       "UNUSUAL_API_VERSION",
				Path:       "apiVersion",
				Suggestion: "Ensure this is a valid Kubernetes API version",
			})
		}
	}

	// Validate metadata structure
	if metadata, ok := manifest["metadata"].(map[string]interface{}); ok {
		if _, exists := metadata["name"]; !exists {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:    "metadata.name",
				Message:  "Missing required field: metadata.name",
				Code:     "MISSING_METADATA_NAME",
				Severity: SeverityError,
				Path:     "metadata.name",
			})
		}

		// Validate name format
		if name, ok := metadata["name"].(string); ok {
			if !isValidKubernetesName(name) {
				result.Errors = append(result.Errors, ValidationError{
					Field:    "metadata.name",
					Message:  fmt.Sprintf("Invalid name format: %s", name),
					Code:     "INVALID_NAME_FORMAT",
					Severity: SeverityError,
					Path:     "metadata.name",
					Details: map[string]interface{}{
						"name":         name,
						"requirements": "Name must be lowercase alphanumeric with dashes, max 253 chars",
					},
				})
			}
		}
	}
}

// validateRequiredFields validates manifest-specific required fields
func (mv *ManifestValidator) validateRequiredFields(manifest map[string]interface{}, result *ValidationResult) {
	kind, _ := manifest["kind"].(string)

	switch kind {
	case "Deployment":
		mv.validateDeploymentFields(manifest, result)
	case "Service":
		mv.validateServiceFields(manifest, result)
	case "ConfigMap":
		mv.validateConfigMapFields(manifest, result)
	case "Secret":
		mv.validateSecretFields(manifest, result)
	case "Ingress":
		mv.validateIngressFields(manifest, result)
	}
}

// validateDeploymentFields validates Deployment-specific fields
func (mv *ManifestValidator) validateDeploymentFields(manifest map[string]interface{}, result *ValidationResult) {
	spec, ok := manifest["spec"].(map[string]interface{})
	if !ok {
		result.Errors = append(result.Errors, ValidationError{
			Field:    "spec",
			Message:  "Deployment must have spec field",
			Code:     "MISSING_DEPLOYMENT_SPEC",
			Severity: SeverityError,
			Path:     "spec",
		})
		return
	}

	// Validate template
	template, ok := spec["template"].(map[string]interface{})
	if !ok {
		result.Errors = append(result.Errors, ValidationError{
			Field:    "spec.template",
			Message:  "Deployment spec must have template field",
			Code:     "MISSING_DEPLOYMENT_TEMPLATE",
			Severity: SeverityError,
			Path:     "spec.template",
		})
		return
	}

	// Validate template spec
	templateSpec, ok := template["spec"].(map[string]interface{})
	if !ok {
		result.Errors = append(result.Errors, ValidationError{
			Field:    "spec.template.spec",
			Message:  "Deployment template must have spec field",
			Code:     "MISSING_TEMPLATE_SPEC",
			Severity: SeverityError,
			Path:     "spec.template.spec",
		})
		return
	}

	// Validate containers
	containers, ok := templateSpec["containers"].([]interface{})
	if !ok || len(containers) == 0 {
		result.Errors = append(result.Errors, ValidationError{
			Field:    "spec.template.spec.containers",
			Message:  "Deployment must have at least one container",
			Code:     "MISSING_CONTAINERS",
			Severity: SeverityError,
			Path:     "spec.template.spec.containers",
		})
	}
}

// validateServiceFields validates Service-specific fields
func (mv *ManifestValidator) validateServiceFields(manifest map[string]interface{}, result *ValidationResult) {
	spec, ok := manifest["spec"].(map[string]interface{})
	if !ok {
		result.Errors = append(result.Errors, ValidationError{
			Field:    "spec",
			Message:  "Service must have spec field",
			Code:     "MISSING_SERVICE_SPEC",
			Severity: SeverityError,
			Path:     "spec",
		})
		return
	}

	// Validate ports
	ports, ok := spec["ports"].([]interface{})
	if !ok || len(ports) == 0 {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Field:      "spec.ports",
			Message:    "Service should have at least one port",
			Code:       "MISSING_SERVICE_PORTS",
			Path:       "spec.ports",
			Suggestion: "Add port configuration to make service accessible",
		})
	}
}

// validateConfigMapFields validates ConfigMap-specific fields
func (mv *ManifestValidator) validateConfigMapFields(manifest map[string]interface{}, result *ValidationResult) {
	// Check if ConfigMap has either data or binaryData
	_, hasData := manifest["data"]
	_, hasBinaryData := manifest["binaryData"]

	if !hasData && !hasBinaryData {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Field:      "data",
			Message:    "ConfigMap should have either data or binaryData field",
			Code:       "EMPTY_CONFIGMAP",
			Path:       "data",
			Suggestion: "Add data or binaryData to make ConfigMap useful",
		})
	}

	// Validate data field if present
	if hasData {
		if data, ok := manifest["data"]; ok {
			if dataMap, ok := data.(map[string]interface{}); ok {
				if len(dataMap) == 0 {
					result.Warnings = append(result.Warnings, ValidationWarning{
						Field:      "data",
						Message:    "ConfigMap data field is empty",
						Code:       "EMPTY_CONFIGMAP_DATA",
						Path:       "data",
						Suggestion: "Add key-value pairs to data field",
					})
				}
			}
		}
	}
}

// validateSecretFields validates Secret-specific fields
func (mv *ManifestValidator) validateSecretFields(manifest map[string]interface{}, result *ValidationResult) {
	// Check if Secret has data
	_, hasData := manifest["data"]
	_, hasStringData := manifest["stringData"]

	if !hasData && !hasStringData {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Field:      "data",
			Message:    "Secret should have either data or stringData field",
			Code:       "EMPTY_SECRET",
			Path:       "data",
			Suggestion: "Add data or stringData to make Secret useful",
		})
	}

	// Validate secret type
	if secretType, ok := manifest["type"].(string); ok {
		if !isValidSecretType(secretType) {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Field:      "type",
				Message:    fmt.Sprintf("Unusual secret type: %s", secretType),
				Code:       "UNUSUAL_SECRET_TYPE",
				Path:       "type",
				Suggestion: "Ensure this is a valid Kubernetes secret type",
			})
		}
	}
}

// validateIngressFields validates Ingress-specific fields
func (mv *ManifestValidator) validateIngressFields(manifest map[string]interface{}, result *ValidationResult) {
	spec, ok := manifest["spec"].(map[string]interface{})
	if !ok {
		result.Errors = append(result.Errors, ValidationError{
			Field:    "spec",
			Message:  "Ingress must have spec field",
			Code:     "MISSING_INGRESS_SPEC",
			Severity: SeverityError,
			Path:     "spec",
		})
		return
	}

	// Check for rules or defaultBackend
	rules, hasRules := spec["rules"]
	_, hasDefaultBackend := spec["defaultBackend"]

	if !hasRules && !hasDefaultBackend {
		result.Errors = append(result.Errors, ValidationError{
			Field:    "spec",
			Message:  "Ingress must have either rules or defaultBackend",
			Code:     "MISSING_INGRESS_ROUTING",
			Severity: SeverityError,
			Path:     "spec",
		})
	}

	// Validate rules if present
	if hasRules {
		if rulesList, ok := rules.([]interface{}); ok && len(rulesList) == 0 {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Field:      "spec.rules",
				Message:    "Ingress rules list is empty",
				Code:       "EMPTY_INGRESS_RULES",
				Path:       "spec.rules",
				Suggestion: "Add ingress rules or use defaultBackend",
			})
		}
	}
}

// validateAllowedKinds checks if the manifest kind is in the allowed list
func (mv *ManifestValidator) validateAllowedKinds(kind string, allowedKinds []string, result *ValidationResult) {
	for _, allowedKind := range allowedKinds {
		if kind == allowedKind {
			return
		}
	}

	result.Valid = false
	result.Errors = append(result.Errors, ValidationError{
		Field:    "kind",
		Message:  fmt.Sprintf("Kind %s is not allowed. Allowed kinds: %v", kind, allowedKinds),
		Code:     "FORBIDDEN_KIND",
		Severity: SeverityError,
		Path:     "kind",
		Details: map[string]interface{}{
			"kind":          kind,
			"allowed_kinds": allowedKinds,
		},
	})
}

// validateRequiredLabels checks if required labels are present
func (mv *ManifestValidator) validateRequiredLabels(manifest map[string]interface{}, requiredLabels []string, result *ValidationResult) {
	metadata, ok := manifest["metadata"].(map[string]interface{})
	if !ok {
		return
	}

	labels, ok := metadata["labels"].(map[string]interface{})
	if !ok {
		labels = make(map[string]interface{})
	}

	for _, requiredLabel := range requiredLabels {
		if _, exists := labels[requiredLabel]; !exists {
			result.Errors = append(result.Errors, ValidationError{
				Field:    "metadata.labels",
				Message:  fmt.Sprintf("Missing required label: %s", requiredLabel),
				Code:     "MISSING_REQUIRED_LABEL",
				Severity: SeverityError,
				Path:     fmt.Sprintf("metadata.labels.%s", requiredLabel),
				Details: map[string]interface{}{
					"required_label": requiredLabel,
				},
			})
		}
	}
}

// validateForbiddenFields checks for forbidden fields
func (mv *ManifestValidator) validateForbiddenFields(manifest map[string]interface{}, forbiddenFields []string, result *ValidationResult) {
	for _, forbiddenField := range forbiddenFields {
		if mv.hasField(manifest, forbiddenField) {
			result.Errors = append(result.Errors, ValidationError{
				Field:    forbiddenField,
				Message:  fmt.Sprintf("Forbidden field found: %s", forbiddenField),
				Code:     "FORBIDDEN_FIELD",
				Severity: SeverityError,
				Path:     forbiddenField,
				Details: map[string]interface{}{
					"forbidden_field": forbiddenField,
				},
			})
		}
	}
}

// generateSuggestions generates helpful suggestions for common issues
func (mv *ManifestValidator) generateSuggestions(result *ValidationResult) {
	suggestions := []string{}

	// Suggest adding namespace for namespaced resources
	if result.Namespace == "" && isNamespacedResource(result.Kind) {
		suggestions = append(suggestions, "Consider adding a namespace to the metadata")
	}

	// Suggest adding resource limits for containers
	if result.Kind == "Deployment" && len(result.Errors) == 0 {
		suggestions = append(suggestions, "Consider adding resource limits and requests to containers")
	}

	// Suggest adding health checks
	if result.Kind == "Deployment" {
		suggestions = append(suggestions, "Consider adding readiness and liveness probes")
	}

	// Suggest using labels for better organization
	if len(result.Warnings) > 0 {
		suggestions = append(suggestions, "Add meaningful labels for better resource organization")
	}

	result.Suggestions = suggestions
}

// Helper functions

// findManifestFiles finds all YAML manifest files in a directory
func (mv *ManifestValidator) findManifestFiles(dirPath string) ([]string, error) {
	var manifestFiles []string

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && (strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")) {
			manifestFiles = append(manifestFiles, path)
		}

		return nil
	})

	return manifestFiles, err
}

// hasField checks if a field exists in the manifest (supports nested fields with dot notation)
func (mv *ManifestValidator) hasField(manifest map[string]interface{}, fieldPath string) bool {
	parts := strings.Split(fieldPath, ".")
	current := manifest

	for i, part := range parts {
		if i == len(parts)-1 {
			_, exists := current[part]
			return exists
		}

		next, ok := current[part].(map[string]interface{})
		if !ok {
			return false
		}
		current = next
	}

	return false
}

// isBuiltinAPIVersion checks if an API version is a built-in Kubernetes API version
func isBuiltinAPIVersion(apiVersion string) bool {
	builtinVersions := []string{"v1"}
	for _, version := range builtinVersions {
		if apiVersion == version {
			return true
		}
	}
	return false
}

// isValidKubernetesName validates Kubernetes resource name format
func isValidKubernetesName(name string) bool {
	if len(name) == 0 || len(name) > 253 {
		return false
	}

	// Simple validation - in practice, you'd use regex for full validation
	for _, char := range name {
		if !((char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-' || char == '.') {
			return false
		}
	}

	return true
}

// isValidSecretType checks if a secret type is valid
func isValidSecretType(secretType string) bool {
	validTypes := []string{
		"Opaque",
		"kubernetes.io/service-account-token",
		"kubernetes.io/dockercfg",
		"kubernetes.io/dockerconfigjson",
		"kubernetes.io/basic-auth",
		"kubernetes.io/ssh-auth",
		"kubernetes.io/tls",
		"bootstrap.kubernetes.io/token",
	}

	for _, validType := range validTypes {
		if secretType == validType {
			return true
		}
	}

	return false
}

// isNamespacedResource checks if a resource kind is namespaced
func isNamespacedResource(kind string) bool {
	namespacedResources := []string{
		"Deployment", "Service", "ConfigMap", "Secret", "Ingress",
		"Pod", "ReplicaSet", "StatefulSet", "DaemonSet", "Job", "CronJob",
		"PersistentVolumeClaim", "ServiceAccount", "Role", "RoleBinding",
	}

	for _, resource := range namespacedResources {
		if kind == resource {
			return true
		}
	}

	return false
}
