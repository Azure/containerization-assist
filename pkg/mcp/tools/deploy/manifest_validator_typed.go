package deploy

import (
	"fmt"
	"strings"
	"time"

	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/api"
	errors "github.com/Azure/container-kit/pkg/mcp/errors"
	validation "github.com/Azure/container-kit/pkg/mcp/security"
	"gopkg.in/yaml.v3"
)

// ============================================================================
// Type-Safe Manifest Validator Enhancement
// ============================================================================

// TypedManifestValidator provides type-safe manifest validation functionality
type TypedManifestValidator struct {
	*ManifestValidator
}

// NewTypedManifestValidator creates a new type-safe manifest validator
func NewTypedManifestValidator(logger *slog.Logger) *TypedManifestValidator {
	return &TypedManifestValidator{
		ManifestValidator: NewManifestValidator(logger),
	}
}

// ValidateTypedManifest validates a manifest using typed structures (primary method)
func (tmv *TypedManifestValidator) ValidateTypedManifest(manifest TypedManifestFile) error {
	tmv.logger.Debug("Validating typed manifest",
		"kind", manifest.Kind,
		"name", manifest.Name)

	// Basic validation
	if manifest.Content == "" {
		return errors.NewError().Messagef("manifest content is empty").WithLocation(

		// Parse YAML to typed structure
		).Build()
	}

	var typedDoc TypedManifest
	if err := yaml.Unmarshal([]byte(manifest.Content), &typedDoc); err != nil {
		return errors.NewError().Messagef("invalid YAML syntax in typed manifest: %v", err).WithLocation(

		// Validate using typed approach
		).Build()
	}

	return tmv.validateTypedManifestStructure(typedDoc)
}

// ValidateTypedManifests validates multiple typed manifests
func (tmv *TypedManifestValidator) ValidateTypedManifests(manifests []TypedManifestFile) []TypedValidationResult {
	tmv.logger.Info("Validating typed manifests", "count", len(manifests))

	results := make([]TypedValidationResult, len(manifests))

	for i, manifest := range manifests {
		result := TypedValidationResult{
			ManifestName: manifest.Name,
			Valid:        true,
			Errors:       []TypedValidationError{},
			Warnings:     []TypedValidationWarning{},
		}

		if err := tmv.ValidateTypedManifest(manifest); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, TypedValidationError{
				Code:    "VALIDATION_ERROR",
				Message: err.Error(),
				Field:   "manifest",
			})
		}

		// Additional typed checks that generate warnings
		warnings := tmv.checkTypedBestPractices(manifest)
		result.Warnings = append(result.Warnings, warnings...)

		results[i] = result
	}

	// Cross-manifest typed validation
	tmv.validateTypedCrossReferences(manifests, results)

	return results
}

// validateTypedManifestStructure validates typed manifest structure
func (tmv *TypedManifestValidator) validateTypedManifestStructure(typedDoc TypedManifest) error {
	// Validate required fields with type safety
	if typedDoc.APIVersion == "" {
		return errors.NewError().Messagef("missing required field: apiVersion").WithLocation().Build()
	}

	if typedDoc.Kind == "" {
		return errors.NewError().Messagef("missing required field: kind").WithLocation().Build()
	}

	if typedDoc.Metadata.Name == "" {
		return errors.NewError().Messagef("missing required field: metadata.name").WithLocation(

		// Validate name format with type safety
		).Build()
	}

	if err := tmv.validateKubernetesName(typedDoc.Metadata.Name); err != nil {
		return errors.NewError().Message("invalid name").Cause(err).WithLocation(

		// Kind-specific typed validation
		).Build()
	}

	switch typedDoc.Kind {
	case "Deployment":
		return tmv.validateTypedDeployment(typedDoc)
	case "Service":
		return tmv.validateTypedService(typedDoc)
	case "ConfigMap":
		return tmv.validateTypedConfigMap(typedDoc)
	case "Secret":
		return tmv.validateTypedSecret(typedDoc)
	default:
		tmv.logger.Debug("No specific typed validation available", "kind", typedDoc.Kind)
		return nil
	}
}

// validateTypedDeployment validates typed deployment
func (tmv *TypedManifestValidator) validateTypedDeployment(typedDoc TypedManifest) error {
	// Convert spec interface{} to typed deployment spec
	if typedDoc.Spec == nil {
		return errors.NewError().Messagef("missing required field: spec").WithLocation(

		// Marshal and unmarshal to convert interface{} to typed structure
		).Build()
	}

	specBytes, err := yaml.Marshal(typedDoc.Spec)
	if err != nil {
		return errors.NewError().Message("failed to convert spec to YAML").Cause(err).WithLocation().Build()
	}

	var spec TypedDeploymentSpec
	if err := yaml.Unmarshal(specBytes, &spec); err != nil {
		return errors.NewError().Message("invalid deployment spec format").Cause(err).WithLocation(

		// Validate replicas with type safety
		).Build()
	}

	if spec.Replicas < 0 {
		return errors.NewError().Messagef("invalid replicas count: %d", spec.Replicas).WithLocation(

		// Validate containers
		).Build()
	}

	if len(spec.Template.Spec.Containers) == 0 {
		return errors.NewError().Messagef("at least one container is required").WithLocation(

		// Validate each container with type safety
		).Build()
	}

	for i, container := range spec.Template.Spec.Containers {
		if err := tmv.validateTypedContainerSpec(container, i); err != nil {
			return err
		}
	}

	return nil
}

// validateTypedService validates typed service
func (tmv *TypedManifestValidator) validateTypedService(typedDoc TypedManifest) error {
	// Convert spec interface{} to typed service spec
	if typedDoc.Spec == nil {
		return errors.NewError().Messagef("missing required field: spec").WithLocation().Build()
	}

	specBytes, err := yaml.Marshal(typedDoc.Spec)
	if err != nil {
		return errors.NewError().Message("failed to convert spec to YAML").Cause(err).WithLocation().Build()
	}

	var spec TypedServiceSpec
	if err := yaml.Unmarshal(specBytes, &spec); err != nil {
		return errors.NewError().Message("invalid service spec format").Cause(err).WithLocation(

		// Validate ports
		).Build()
	}

	if len(spec.Ports) == 0 {
		return errors.NewError().Messagef("at least one port is required").WithLocation(

		// Validate each port with type safety
		).Build()
	}

	for i, port := range spec.Ports {
		if err := tmv.validateTypedServicePort(port, i); err != nil {
			return err
		}
	}

	return nil
}

// validateTypedConfigMap validates typed ConfigMap
func (tmv *TypedManifestValidator) validateTypedConfigMap(typedDoc TypedManifest) error {
	// ConfigMaps must have either data, stringData, or binaryData
	hasData := len(typedDoc.Data) > 0
	hasStringData := len(typedDoc.StringData) > 0
	hasBinaryData := len(typedDoc.BinaryData) > 0

	if !hasData && !hasStringData && !hasBinaryData {
		return errors.NewError().Messagef("ConfigMap must have either 'data', 'stringData', or 'binaryData'").WithLocation().Build(

		// validateTypedSecret validates typed Secret
		)
	}

	return nil
}

func (tmv *TypedManifestValidator) validateTypedSecret(typedDoc TypedManifest) error {
	// Secrets should have type field in spec
	if typedDoc.Spec != nil {
		specBytes, err := yaml.Marshal(typedDoc.Spec)
		if err == nil {
			var secretSpec struct {
				Type string `yaml:"type"`
			}
			if err := yaml.Unmarshal(specBytes, &secretSpec); err == nil && secretSpec.Type != "" {
				// Validate known secret types
				validTypes := []string{
					"Opaque",
					"kubernetes.io/service-account-token",
					"kubernetes.io/dockercfg",
					"kubernetes.io/dockerconfigjson",
					"kubernetes.io/basic-auth",
					"kubernetes.io/ssh-auth",
					"kubernetes.io/tls",
				}

				isValidType := false
				for _, validType := range validTypes {
					if secretSpec.Type == validType {
						isValidType = true
						break
					}
				}

				if !isValidType {
					tmv.logger.Warn("Unknown secret type", "type", secretSpec.Type)
				}
			}
		}
	}

	return nil
}

// validateTypedContainerSpec validates typed container specification
func (tmv *TypedManifestValidator) validateTypedContainerSpec(container TypedContainer, index int) error {
	// Check name
	if container.Name == "" {
		return errors.NewError().Messagef("container at index %d missing name", index).WithLocation(

		// Check image
		).Build()
	}

	if container.Image == "" {
		return errors.NewError().Messagef("container at index %d missing image", index).WithLocation(

		// Validate image format
		).Build()
	}

	if !tmv.isValidImageReference(container.Image) {
		return errors.NewError().Messagef("container at index %d has invalid image reference: %s", index, container.Image).WithLocation(

		// Validate ports
		).Build()
	}

	for i, port := range container.Ports {
		if err := tmv.validateTypedContainerPort(port, index, i); err != nil {
			return err
		}
	}

	// Validate environment variables
	for i, env := range container.Env {
		if env.Name == "" {
			return errors.NewError().Messagef("container at index %d, env at index %d missing name", index, i).WithLocation(

			// Validate image pull policy
			).Build()
		}
	}

	if container.ImagePullPolicy != "" {
		validPolicies := []string{"Always", "Never", "IfNotPresent"}
		isValid := false
		for _, policy := range validPolicies {
			if container.ImagePullPolicy == policy {
				isValid = true
				break
			}
		}
		if !isValid {
			return errors.NewError().Messagef("invalid imagePullPolicy for container %s: %s", container.Name, container.ImagePullPolicy).WithLocation().Build()
		}
	}

	return nil
}

// validateTypedContainerPort validates typed container port
func (tmv *TypedManifestValidator) validateTypedContainerPort(port TypedContainerPort, containerIndex, portIndex int) error {
	// Validate port number
	if port.ContainerPort <= 0 || port.ContainerPort > 65535 {
		return errors.NewError().Messagef("invalid container port %d for container at index %d, port at index %d", port.ContainerPort, containerIndex, portIndex).WithLocation(

		// Validate protocol
		).Build()
	}

	if port.Protocol != "" && port.Protocol != "TCP" && port.Protocol != "UDP" {
		return errors.NewError().Messagef("invalid protocol %s for container at index %d, port at index %d", port.Protocol, containerIndex, portIndex).WithLocation().Build(

		// validateTypedServicePort validates typed service port
		)
	}

	return nil
}

func (tmv *TypedManifestValidator) validateTypedServicePort(port TypedServicePort, index int) error {
	// Check port number
	if port.Port <= 0 || port.Port > 65535 {
		return errors.NewError().Messagef("port at index %d has invalid port number: %d", index, port.Port).WithLocation(

		// Check target port if specified
		).Build()
	}

	if port.TargetPort != 0 && (port.TargetPort < 1 || port.TargetPort > 65535) {
		return errors.NewError().Messagef("port at index %d has invalid targetPort: %d", index, port.TargetPort).WithLocation(

		// Validate protocol
		).Build()
	}

	if port.Protocol != "" {
		validProtocols := []string{"TCP", "UDP", "SCTP"}
		isValid := false
		for _, protocol := range validProtocols {
			if port.Protocol == protocol {
				isValid = true
				break
			}
		}
		if !isValid {
			return errors.NewError().Messagef("port at index %d has invalid protocol: %s", index, port.Protocol).WithLocation().Build()
		}
	}

	return nil
}

// checkTypedBestPractices checks for best practice violations using typed structures
func (tmv *TypedManifestValidator) checkTypedBestPractices(manifest TypedManifestFile) []TypedValidationWarning {
	var warnings []TypedValidationWarning

	// Parse manifest to typed structure
	var typedDoc TypedManifest
	if err := yaml.Unmarshal([]byte(manifest.Content), &typedDoc); err != nil {
		return warnings
	}

	// Check for labels
	if len(typedDoc.Metadata.Labels) == 0 {
		warnings = append(warnings, TypedValidationWarning{
			Code:    "MISSING_LABELS",
			Message: "Consider adding labels for better resource management",
			Field:   "metadata.labels",
		})
	}

	// Deployment-specific checks
	if manifest.Kind == "Deployment" {
		warnings = append(warnings, tmv.checkTypedDeploymentBestPractices(typedDoc)...)
	}

	// Service-specific checks
	if manifest.Kind == "Service" {
		warnings = append(warnings, tmv.checkTypedServiceBestPractices(typedDoc)...)
	}

	return warnings
}

// checkTypedDeploymentBestPractices checks deployment best practices using typed structures
func (tmv *TypedManifestValidator) checkTypedDeploymentBestPractices(typedDoc TypedManifest) []TypedValidationWarning {
	var warnings []TypedValidationWarning

	if typedDoc.Spec != nil {
		specBytes, err := yaml.Marshal(typedDoc.Spec)
		if err != nil {
			return warnings
		}

		var spec TypedDeploymentSpec
		if err := yaml.Unmarshal(specBytes, &spec); err != nil {
			return warnings
		}

		// Check replicas
		if spec.Replicas == 1 {
			warnings = append(warnings, TypedValidationWarning{
				Code:    "LOW_REPLICA_COUNT",
				Message: "Consider using more than 1 replica for high availability",
				Field:   "spec.replicas",
			})
		}

		// Check containers for best practices
		for i, container := range spec.Template.Spec.Containers {
			// Check resource limits
			if len(container.Resources.Limits) == 0 && len(container.Resources.Requests) == 0 {
				warnings = append(warnings, TypedValidationWarning{
					Code:    "MISSING_RESOURCES",
					Message: fmt.Sprintf("Container %d: Consider setting resource requests and limits", i),
					Field:   fmt.Sprintf("spec.template.spec.containers[%d].resources", i),
				})
			}

			// Check for liveness/readiness probes would require additional fields in TypedContainer
			// This is a simplified check - could be enhanced with probe fields
		}
	}

	return warnings
}

// checkTypedServiceBestPractices checks service best practices using typed structures
func (tmv *TypedManifestValidator) checkTypedServiceBestPractices(typedDoc TypedManifest) []TypedValidationWarning {
	var warnings []TypedValidationWarning

	if typedDoc.Spec != nil {
		specBytes, err := yaml.Marshal(typedDoc.Spec)
		if err != nil {
			return warnings
		}

		var spec TypedServiceSpec
		if err := yaml.Unmarshal(specBytes, &spec); err != nil {
			return warnings
		}

		// Check service type
		if spec.Type == "LoadBalancer" {
			warnings = append(warnings, TypedValidationWarning{
				Code:    "EXPENSIVE_SERVICE_TYPE",
				Message: "LoadBalancer services can be expensive in cloud environments",
				Field:   "spec.type",
			})
		}
	}

	return warnings
}

// validateTypedCrossReferences validates references between typed manifests
func (tmv *TypedManifestValidator) validateTypedCrossReferences(manifests []TypedManifestFile, results []TypedValidationResult) {
	// Build maps of available resources
	services := make(map[string]bool)
	configMaps := make(map[string]bool)
	secrets := make(map[string]bool)

	for _, manifest := range manifests {
		switch manifest.Kind {
		case "Service":
			services[manifest.Name] = true
		case "ConfigMap":
			configMaps[manifest.Name] = true
		case "Secret":
			secrets[manifest.Name] = true
		}
	}

	// Check references in deployments
	for i, manifest := range manifests {
		if manifest.Kind == "Deployment" {
			var typedDoc TypedManifest
			if err := yaml.Unmarshal([]byte(manifest.Content), &typedDoc); err != nil {
				continue
			}

			tmv.checkTypedContainerReferences(typedDoc, configMaps, secrets, &results[i])
		}
	}
}

// checkTypedContainerReferences checks container references using typed structures
func (tmv *TypedManifestValidator) checkTypedContainerReferences(typedDoc TypedManifest, configMaps, secrets map[string]bool, result *TypedValidationResult) {
	if typedDoc.Spec == nil {
		return
	}

	specBytes, err := yaml.Marshal(typedDoc.Spec)
	if err != nil {
		return
	}

	var spec TypedDeploymentSpec
	if err := yaml.Unmarshal(specBytes, &spec); err != nil {
		return
	}

	// Check environment variable references in containers
	for containerIndex, container := range spec.Template.Spec.Containers {
		for envIndex, env := range container.Env {
			// This is a simplified check - in practice, you'd check for configMapRef, secretRef, etc.
			// For now, just check if the value looks like a reference pattern
			if strings.Contains(env.Value, "$(") || strings.Contains(env.Value, "${") {
				result.Warnings = append(result.Warnings, TypedValidationWarning{
					Code:    "UNRESOLVED_REFERENCE",
					Message: fmt.Sprintf("Container %d, env %d may contain unresolved reference: %s", containerIndex, envIndex, env.Name),
					Field:   fmt.Sprintf("spec.template.spec.containers[%d].env[%d]", containerIndex, envIndex),
				})
			}
		}
	}
}

// ============================================================================
// Supporting Types for Typed Validation
// ============================================================================

// TypedManifestFile represents a typed Kubernetes manifest file
type TypedManifestFile struct {
	Kind       string            `json:"kind"`
	Name       string            `json:"name"`
	Content    string            `json:"content"`
	FilePath   string            `json:"filePath"`
	IsSecret   bool              `json:"isSecret"`
	SecretInfo string            `json:"secretInfo,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"` // Type-safe metadata
}

// TypedValidationResult represents the result of typed manifest validation
type TypedValidationResult struct {
	ManifestName string                   `json:"manifestName"`
	Valid        bool                     `json:"valid"`
	Errors       []TypedValidationError   `json:"github.com/Azure/container-kit/pkg/mcp/errors"`
	Warnings     []TypedValidationWarning `json:"warnings"`
	Score        float64                  `json:"score,omitempty"`
	Metadata     map[string]string        `json:"metadata,omitempty"` // Type-safe metadata
}

// TypedValidationError represents a typed validation error
type TypedValidationError struct {
	Code     string            `json:"code"`
	Message  string            `json:"message"`
	Field    string            `json:"field"`
	Severity string            `json:"severity,omitempty"`
	Context  map[string]string `json:"context,omitempty"` // Type-safe context
}

// TypedValidationWarning represents a typed validation warning
type TypedValidationWarning struct {
	Code       string            `json:"code"`
	Message    string            `json:"message"`
	Field      string            `json:"field"`
	Suggestion string            `json:"suggestion,omitempty"`
	Context    map[string]string `json:"context,omitempty"` // Type-safe context
}

// TypedManifestValidationConfig represents typed validation configuration
type TypedManifestValidationConfig struct {
	StrictMode          bool              `json:"strictMode"`
	EnableBestPractices bool              `json:"enableBestPractices"`
	AllowedKinds        []string          `json:"allowedKinds,omitempty"`
	RequiredLabels      []string          `json:"requiredLabels,omitempty"`
	ForbiddenFields     []string          `json:"forbiddenFields,omitempty"`
	CustomRules         map[string]string `json:"customRules,omitempty"` // Type-safe custom rules
}

// ============================================================================
// Conversion Utilities for Legacy Compatibility
// ============================================================================

// ConvertToTypedManifestFile converts legacy ManifestFile to TypedManifestFile
func ConvertToTypedManifestFile(legacy ManifestFile) TypedManifestFile {
	return TypedManifestFile{
		Kind:       legacy.Kind,
		Name:       legacy.Name,
		Content:    legacy.Content,
		FilePath:   legacy.FilePath,
		IsSecret:   legacy.IsSecret,
		SecretInfo: legacy.SecretInfo,
		Metadata:   make(map[string]string), // Initialize typed metadata
	}
}

// ConvertToLegacyManifestFile converts TypedManifestFile to legacy ManifestFile
func ConvertToLegacyManifestFile(typed TypedManifestFile) ManifestFile {
	return ManifestFile{
		Kind:       typed.Kind,
		Name:       typed.Name,
		Content:    typed.Content,
		FilePath:   typed.FilePath,
		IsSecret:   typed.IsSecret,
		SecretInfo: typed.SecretInfo,
	}
}

// ConvertToTypedValidationResult converts legacy ValidationResult to TypedValidationResult
func ConvertToTypedValidationResult(legacy api.ValidationResult) TypedValidationResult {
	// Extract manifest path from Details instead of Metadata
	manifestPath := ""
	if legacy.Details != nil {
		if path, ok := legacy.Details["manifest_path"].(string); ok {
			manifestPath = path
		}
	}

	result := TypedValidationResult{
		ManifestName: manifestPath,
		Valid:        legacy.Valid,
		Errors:       make([]TypedValidationError, len(legacy.Errors)),
		Warnings:     make([]TypedValidationWarning, len(legacy.Warnings)),
		Metadata:     make(map[string]string),
	}

	// Convert ValidationError to typed errors
	for i, err := range legacy.Errors {
		result.Errors[i] = TypedValidationError{
			Code:    err.Code,
			Message: err.Message,
			Field:   err.Field,
			Context: make(map[string]string),
		}
	}

	// Convert ValidationWarning to typed warnings
	for i, warn := range legacy.Warnings {
		result.Warnings[i] = TypedValidationWarning{
			Code:    warn.Code,
			Message: warn.Message,
			Field:   warn.Field,
			Context: make(map[string]string),
		}
	}

	return result
}

// ConvertToLegacyValidationResult converts TypedValidationResult to legacy ValidationResult
func ConvertToLegacyValidationResult(typed TypedValidationResult) api.ValidationResult {
	result := api.ValidationResult{
		Valid:    typed.Valid,
		Errors:   make([]api.ValidationError, len(typed.Errors)),
		Warnings: make([]api.ValidationWarning, len(typed.Warnings)),
		Metadata: validation.Metadata{
			ValidatedAt:      time.Now(),
			ValidatorName:    "TypedManifestValidator",
			ValidatorVersion: "1.0.0",
		},
		Details: map[string]interface{}{
			"manifest_path": typed.ManifestName,
		},
	}

	// Convert typed errors to ValidationError
	for i, err := range typed.Errors {
		result.Errors[i] = api.ValidationError{
			Code:    err.Code,
			Message: err.Message,
			Field:   err.Field,
		}
	}

	// Convert typed warnings to ValidationWarning
	for i, warning := range typed.Warnings {
		result.Warnings[i] = api.ValidationWarning{
			Code:    warning.Code,
			Message: warning.Message,
			Field:   warning.Field,
		}
	}

	return result
}
