package deploy

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"log/slog"

	"github.com/Azure/container-kit/pkg/common/validation-core/core"
	"github.com/Azure/container-kit/pkg/common/validation-core/validators"
	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/core/tools"
	core "github.com/Azure/container-kit/pkg/mcp/core/tools"
	errors "github.com/Azure/container-kit/pkg/mcp/errors"
	validation "github.com/Azure/container-kit/pkg/mcp/security"
	"gopkg.in/yaml.v3"
)

// ManifestValidator validates Kubernetes manifests using unified validation framework
type ManifestValidator struct {
	logger            *slog.Logger
	manifestValidator core.Validator
}

// UnifiedManifestValidator provides a unified validation interface
type UnifiedManifestValidator struct {
	impl *ManifestValidator
}

// NewManifestValidator creates a new manifest validator with unified validation support
func NewManifestValidator(logger *slog.Logger) *ManifestValidator {
	return &ManifestValidator{
		logger:            logger.With("component", "unified_manifest_validator"),
		manifestValidator: validators.NewKubernetesValidator(),
	}
}

// NewUnifiedManifestValidator creates a new unified manifest validator
func NewUnifiedManifestValidator(logger *slog.Logger) *UnifiedManifestValidator {
	return &UnifiedManifestValidator{
		impl: NewManifestValidator(logger),
	}
}

// ValidateManifestUnified performs manifest validation using unified validation framework
func (v *ManifestValidator) ValidateManifestUnified(ctx context.Context, manifestContent string) (*core.DeployResult, error) {
	v.logger.Info("Starting unified manifest validation")

	// Create manifest validation data
	manifestData := map[string]interface{}{
		"content":   manifestContent,
		"format":    "yaml",
		"type":      "kubernetes_manifest",
		"timestamp": time.Now(),
	}

	// Use unified manifest validator
	options := core.NewValidationOptions().WithStrictMode(true)
	nonGenericResult := v.manifestValidator.Validate(ctx, manifestData, options)

	// Convert to DeployResult
	result := core.NewDeployResult("unified_manifest_validator", "1.0.0")
	result.Valid = nonGenericResult.Valid
	result.Errors = nonGenericResult.Errors
	result.Warnings = nonGenericResult.Warnings
	result.Suggestions = nonGenericResult.Suggestions
	result.Duration = nonGenericResult.Duration
	result.Metadata = nonGenericResult.Metadata

	// Add deployment-specific metadata
	resources := extractKubernetesResources(manifestContent)
	result.Data = core.DeployValidationData{
		Namespace: extractNamespace(manifestContent),
		Resources: resources,
		ClusterInfo: map[string]interface{}{
			"manifest_type":   "kubernetes",
			"validation_type": "manifest",
		},
	}

	v.logger.Info("Unified manifest validation completed",
		"valid", result.Valid,
		"errors", len(result.Errors),
		"warnings", len(result.Warnings))

	return result, nil
}

// ValidateManifest validates a single manifest
func (v *ManifestValidator) ValidateManifest(manifest ManifestFile) error {
	v.logger.Debug("Validating manifest",
		"kind", manifest.Kind,
		"name", manifest.Name)

	// Basic validation
	if manifest.Content == "" {
		return errors.NewError().Messagef("manifest content is empty").WithLocation(
		// Parse YAML to check structure
		).Build()
	}

	var doc map[string]interface{}
	if err := yaml.Unmarshal([]byte(manifest.Content), &doc); err != nil {
		return errors.NewError().Messagef("invalid YAML syntax in manifest: %v", err).WithLocation(
		// Try typed validation first
		).Build()
	}

	if typedDoc, err := v.convertToTypedDocument(doc); err == nil {
		if err := v.validateManifestTyped(typedDoc); err == nil {
			return nil // Successfully validated with typed approach
		}
	}

	// Fall back to legacy interface{} validation
	return v.validateManifestLegacy(manifest, doc)
}

// ValidateManifests validates multiple manifests
func (v *ManifestValidator) ValidateManifests(manifests []ManifestFile) []api.ValidationResult {
	v.logger.Info("Validating manifests", "count", len(manifests))

	results := make([]api.ValidationResult, len(manifests))

	for i, manifest := range manifests {
		// Create api.ValidationResult directly
		result := api.ValidationResult{
			Valid:    true,
			Errors:   []api.ValidationError{},
			Warnings: []api.ValidationWarning{},
			Metadata: validation.Metadata{
				ValidatedAt:      time.Now(),
				ValidatorName:    "CoreValidator",
				ValidatorVersion: "1.0.0",
			},
		}
		// Store manifest path in Details instead of Metadata
		if result.Details == nil {
			result.Details = make(map[string]interface{})
		}
		result.Details["manifest_path"] = manifest.FilePath

		if err := v.ValidateManifest(manifest); err != nil {
			result.Valid = false
			validationErr := api.ValidationError{
				Code:    "MANIFEST_VALIDATION_ERROR",
				Message: err.Error(),
				Field:   "manifest",
			}
			result.Errors = append(result.Errors, validationErr)
		}

		// Additional checks that generate warnings
		warningMessages := v.checkBestPractices(manifest)
		for _, msg := range warningMessages {
			warning := api.ValidationWarning{
				Code:    "BEST_PRACTICE_WARNING",
				Message: msg,
			}
			result.Warnings = append(result.Warnings, warning)
		}

		results[i] = result
	}

	// Cross-manifest validation
	v.validateCrossReferences(manifests, results)

	return results
}

// convertToTypedDocument converts map[string]interface{} to TypedValidationDocument
func (v *ManifestValidator) convertToTypedDocument(doc map[string]interface{}) (*tools.TypedValidationDocument, error) {
	typedDoc := &tools.TypedValidationDocument{
		RawFields: make(map[string]json.RawMessage),
	}

	// Extract standard fields
	if apiVersion, ok := doc["apiVersion"].(string); ok {
		typedDoc.APIVersion = apiVersion
	}

	if kind, ok := doc["kind"].(string); ok {
		typedDoc.Kind = kind
	}

	// Convert metadata
	if metadata, ok := doc["metadata"].(map[string]interface{}); ok {
		typedMetadata := &tools.TypedValidationMetadata{}

		if name, ok := metadata["name"].(string); ok {
			typedMetadata.Name = name
		}
		if namespace, ok := metadata["namespace"].(string); ok {
			typedMetadata.Namespace = namespace
		}
		if labels, ok := metadata["labels"].(map[string]string); ok {
			typedMetadata.Labels = labels
		}
		if annotations, ok := metadata["annotations"].(map[string]string); ok {
			typedMetadata.Annotations = annotations
		}

		typedDoc.Metadata = typedMetadata
	}

	// Convert spec to RawMessage for flexible handling
	if spec, ok := doc["spec"]; ok {
		if specBytes, err := json.Marshal(spec); err == nil {
			typedDoc.Spec = specBytes
		}
	}

	// Extract data fields for ConfigMaps and Secrets
	if data, ok := doc["data"].(map[string]string); ok {
		typedDoc.Data = data
	}
	if stringData, ok := doc["stringData"].(map[string]string); ok {
		typedDoc.StringData = stringData
	}
	if binaryData, ok := doc["binaryData"].(map[string][]byte); ok {
		typedDoc.BinaryData = binaryData
	}

	// Store any unknown fields as raw messages
	knownFields := map[string]bool{
		"apiVersion": true, "kind": true, "metadata": true, "spec": true,
		"data": true, "stringData": true, "binaryData": true,
	}

	for k, v := range doc {
		if !knownFields[k] {
			if rawBytes, err := json.Marshal(v); err == nil {
				typedDoc.RawFields[k] = rawBytes
			}
		}
	}

	return typedDoc, nil
}

// validateRequiredFields checks that required fields are present
func (v *ManifestValidator) validateRequiredFields(doc map[string]interface{}, kind string) error {
	// Check API version
	if _, ok := doc["apiVersion"]; !ok {
		return errors.NewError().Messagef("missing required field: apiVersion").WithLocation(

		// Check kind
		).Build()
	}

	if docKind, ok := doc["kind"].(string); !ok || docKind != kind {
		return errors.NewError().Messagef("kind mismatch in manifest: expected '%s' but found '%v'", kind, doc["kind"]).WithLocation(

		// Check metadata
		).Build()
	}

	metadata, ok := doc["metadata"].(map[string]interface{})
	if !ok {
		return errors.NewError().Messagef("missing required field: metadata").WithLocation(

		// Check name
		).Build()
	}

	if _, ok := metadata["name"]; !ok {
		return errors.NewError().Messagef("missing required field: metadata.name").WithLocation(

		// Validate name format
		).Build()
	}

	if name, ok := metadata["name"].(string); ok {
		if err := v.validateKubernetesName(name); err != nil {
			return errors.NewError().Messagef("invalid name: %s", err.Error()).Cause(err).WithLocation().Build()
		}
	}

	return nil
}

// validateKubernetesName validates Kubernetes resource names
func (v *ManifestValidator) validateKubernetesName(name string) error {
	if name == "" {
		return errors.NewError().Messagef("resource name cannot be empty").WithLocation().Build()
	}

	if len(name) > 253 {
		return errors.NewError().Messagef("resource name too long: %d characters (max 253)", len(name)).WithLocation(

		// Must consist of lower case alphanumeric characters, '-' or '.'
		).Build()
	}

	validName := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)
	if !validName.MatchString(name) {
		return errors.NewError().Messagef("invalid resource name format: %s", name).WithLocation().Build(

		// checkBestPractices checks for best practice violations
		)
	}

	return nil
}

func (v *ManifestValidator) checkBestPractices(manifest ManifestFile) []string {
	var warnings []string

	// Parse manifest
	var doc map[string]interface{}
	if err := yaml.Unmarshal([]byte(manifest.Content), &doc); err != nil {
		return warnings
	}

	// Check for labels
	if metadata, ok := doc["metadata"].(map[string]interface{}); ok {
		if _, hasLabels := metadata["labels"]; !hasLabels {
			warnings = append(warnings, "Consider adding labels for better resource management")
		}
	}

	// Deployment-specific checks
	if manifest.Kind == "Deployment" {
		warnings = append(warnings, v.checkDeploymentBestPractices(doc)...)
	}

	// Service-specific checks
	if manifest.Kind == "Service" {
		warnings = append(warnings, v.checkServiceBestPractices(doc)...)
	}

	return warnings
}

// checkDeploymentBestPractices checks deployment best practices
func (v *ManifestValidator) checkDeploymentBestPractices(doc map[string]interface{}) []string {
	var warnings []string

	if spec, ok := doc["spec"].(map[string]interface{}); ok {
		// Check replicas
		if replicas, ok := spec["replicas"].(int); ok && replicas == 1 {
			warnings = append(warnings, "Consider using more than 1 replica for high availability")
		}

		// Check pod template
		if template, ok := spec["template"].(map[string]interface{}); ok {
			if templateSpec, ok := template["spec"].(map[string]interface{}); ok {
				// Check containers
				if containers, ok := templateSpec["containers"].([]interface{}); ok {
					for i, container := range containers {
						if cont, ok := container.(map[string]interface{}); ok {
							// Check resource limits
							if _, hasResources := cont["resources"]; !hasResources {
								warnings = append(warnings, fmt.Sprintf("Container %d: Consider setting resource requests and limits", i))
							}

							// Check liveness/readiness probes
							if _, hasLiveness := cont["livenessProbe"]; !hasLiveness {
								warnings = append(warnings, fmt.Sprintf("Container %d: Consider adding a liveness probe", i))
							}
							if _, hasReadiness := cont["readinessProbe"]; !hasReadiness {
								warnings = append(warnings, fmt.Sprintf("Container %d: Consider adding a readiness probe", i))
							}
						}
					}
				}
			}
		}
	}

	return warnings
}

// checkServiceBestPractices checks service best practices
func (v *ManifestValidator) checkServiceBestPractices(doc map[string]interface{}) []string {
	var warnings []string

	if spec, ok := doc["spec"].(map[string]interface{}); ok {
		// Check service type
		if serviceType, ok := spec["type"].(string); ok && serviceType == "LoadBalancer" {
			warnings = append(warnings, "LoadBalancer services can be expensive in cloud environments")
		}
	}

	return warnings
}

// validateCrossReferences validates references between manifests
func (v *ManifestValidator) validateCrossReferences(manifests []ManifestFile, results []api.ValidationResult) {
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
			var doc map[string]interface{}
			if err := yaml.Unmarshal([]byte(manifest.Content), &doc); err != nil {
				continue
			}

			// Check service references in deployment
			if spec, ok := doc["spec"].(map[string]interface{}); ok {
				if template, ok := spec["template"].(map[string]interface{}); ok {
					if templateSpec, ok := template["spec"].(map[string]interface{}); ok {
						// Check environment references
						if containers, ok := templateSpec["containers"].([]interface{}); ok {
							for _, container := range containers {
								if cont, ok := container.(map[string]interface{}); ok {
									v.checkContainerReferences(cont, configMaps, secrets, &results[i])
								}
							}
						}
					}
				}
			}
		}
	}
}

// checkContainerReferences validates container references to ConfigMaps and Secrets
func (v *ManifestValidator) checkContainerReferences(container map[string]interface{}, configMaps, secrets map[string]bool, result *api.ValidationResult) {
	// Check environment variables
	if env, ok := container["env"].([]interface{}); ok {
		for _, envVar := range env {
			if envVarMap, ok := envVar.(map[string]interface{}); ok {
				// Check valueFrom references
				if valueFrom, ok := envVarMap["valueFrom"].(map[string]interface{}); ok {
					// Check ConfigMap references
					if configMapRef, ok := valueFrom["configMapKeyRef"].(map[string]interface{}); ok {
						if name, ok := configMapRef["name"].(string); ok {
							if !configMaps[name] {
								warning := api.ValidationWarning{
									Code:    "MISSING_CONFIGMAP_REFERENCE",
									Message: fmt.Sprintf("Referenced ConfigMap '%s' not found in manifests", name),
								}
								result.Warnings = append(result.Warnings, warning)
							}
						}
					}

					// Check Secret references
					if secretRef, ok := valueFrom["secretKeyRef"].(map[string]interface{}); ok {
						if name, ok := secretRef["name"].(string); ok {
							if !secrets[name] {
								warning := api.ValidationWarning{
									Code:    "MISSING_SECRET_REFERENCE",
									Message: fmt.Sprintf("Referenced Secret '%s' not found in manifests", name),
								}
								result.Warnings = append(result.Warnings, warning)
							}
						}
					}
				}
			}
		}
	}

	// Check volume mounts
	if volumeMounts, ok := container["volumeMounts"].([]interface{}); ok {
		for _, volumeMount := range volumeMounts {
			if volumeMountMap, ok := volumeMount.(map[string]interface{}); ok {
				if mountName, ok := volumeMountMap["name"].(string); ok {
					v.logger.Debug("Found volume mount reference", "volumeMount", mountName)
					// Note: Volume validation would require checking the pod spec volumes
					// which is more complex and may warrant separate validation
				}
			}
		}
	}
}

// validateManifestTyped validates using typed structures (preferred approach)
func (v *ManifestValidator) validateManifestTyped(typedDoc *tools.TypedValidationDocument) error {
	// Validate required fields with type safety
	if typedDoc.APIVersion == "" {
		return errors.NewError().Messagef("missing required field: apiVersion").WithLocation().Build()
	}

	if typedDoc.Kind == "" {
		return errors.NewError().Messagef("missing required field: kind").WithLocation().Build()
	}

	if typedDoc.Metadata == nil {
		return errors.NewError().Messagef("missing required field: metadata").WithLocation().Build()
	}

	if typedDoc.Metadata.Name == "" {
		return errors.NewError().Messagef("missing required field: metadata.name").WithLocation(

		// Validate name format with type safety
		).Build()
	}

	if err := v.validateKubernetesName(typedDoc.Metadata.Name); err != nil {
		return errors.NewError().Messagef("invalid name: %s", err.Error()).Cause(err).WithLocation().Build()
	}

	switch typedDoc.Kind {
	case "Deployment":
		return v.validateDeploymentTypedSpec(typedDoc)
	case "Service":
		return v.validateServiceTypedSpec(typedDoc)
	case "ConfigMap":
		return v.validateConfigMapTypedSpec(typedDoc)
	case "Secret":
		return v.validateSecretTypedSpec(typedDoc)
	default:
		v.logger.Debug("No specific typed validation available, using basic validation", "kind", typedDoc.Kind)
		return nil
	}
}

// validateManifestLegacy validates using interface{} approach (fallback)
func (v *ManifestValidator) validateManifestLegacy(manifest ManifestFile, doc map[string]interface{}) error {
	// Validate required fields
	if err := v.validateRequiredFields(doc, manifest.Kind); err != nil {
		return err
	}

	// Kind-specific validation
	switch manifest.Kind {
	case "Deployment":
		return v.validateDeployment(doc)
	case "Service":
		return v.validateService(doc)
	case "ConfigMap":
		return v.validateConfigMap(doc)
	case "Secret":
		return v.validateSecret(doc)
	case "Ingress":
		return v.validateIngress(doc)
	case "PersistentVolumeClaim":
		return v.validatePVC(doc)
	default:
		v.logger.Warn("Unknown manifest kind, applying basic validation only", "kind", manifest.Kind)
		return nil
	}
}

// ============================================================================
// UNIFIED VALIDATION INTERFACE METHODS
// ============================================================================

// Validate implements the GenericValidator interface
func (umv *UnifiedManifestValidator) Validate(ctx context.Context, data core.DeployValidationData, options *core.ValidationOptions) *core.DeployResult {
	// Convert DeployValidationData to the format expected by ValidateManifestUnified
	manifestContent := ""
	if data.Resources != nil && len(data.Resources) > 0 {
		// Create a basic YAML manifest from the first resource
		resource := data.Resources[0]
		manifestContent = fmt.Sprintf(`apiVersion: %s
kind: %s
metadata:
  name: %s
  namespace: %s
`, resource.APIVersion, resource.Kind, resource.Name, resource.Namespace)
	}

	result, err := umv.impl.ValidateManifestUnified(ctx, manifestContent)
	if err != nil {
		if result == nil {
			result = core.NewDeployResult("unified_manifest_validator", "1.0.0")
		}
		result.AddError(core.NewDeployError("VALIDATION_ERROR", err.Error(), "validation"))
	}
	return result
}

// GetName returns the validator name
func (umv *UnifiedManifestValidator) GetName() string {
	return "unified_manifest_validator"
}

// GetVersion returns the validator version
func (umv *UnifiedManifestValidator) GetVersion() string {
	return "1.0.0"
}

// GetSupportedTypes returns the data types this validator can handle
func (umv *UnifiedManifestValidator) GetSupportedTypes() []string {
	return []string{"DeployValidationData", "map[string]interface{}", "string"}
}

// ValidateManifestWithMetadata performs validation with additional metadata
func (umv *UnifiedManifestValidator) ValidateManifestWithMetadata(ctx context.Context, content string, metadata map[string]interface{}) (*core.DeployResult, error) {
	result, err := umv.impl.ValidateManifestUnified(ctx, content)
	if err != nil && result == nil {
		result = core.NewDeployResult("unified_manifest_validator", "1.0.0")
		result.AddError(core.NewDeployError("VALIDATION_ERROR", err.Error(), "validation"))
	}

	// Add metadata to result
	if result.Metadata.Context == nil {
		result.Metadata.Context = make(map[string]interface{})
	}
	for k, v := range metadata {
		result.Metadata.Context[k] = v
	}

	return result, nil
}

// Helper functions for unified validation

func extractNamespace(manifestContent string) string {
	// Simple extraction - in production this would be more sophisticated
	var doc map[string]interface{}
	if err := yaml.Unmarshal([]byte(manifestContent), &doc); err != nil {
		return "default"
	}

	if metadata, ok := doc["metadata"].(map[string]interface{}); ok {
		if namespace, ok := metadata["namespace"].(string); ok {
			return namespace
		}
	}
	return "default"
}

func extractResources(manifestContent string) []map[string]interface{} {
	resources := make([]map[string]interface{}, 0)

	var doc map[string]interface{}
	if err := yaml.Unmarshal([]byte(manifestContent), &doc); err != nil {
		return resources
	}

	// Add the parsed document as a resource
	resources = append(resources, doc)
	return resources
}

// extractKubernetesResources extracts resources as KubernetesResource structs
func extractKubernetesResources(manifestContent string) []core.KubernetesResource {
	resources := make([]core.KubernetesResource, 0)

	var doc map[string]interface{}
	if err := yaml.Unmarshal([]byte(manifestContent), &doc); err != nil {
		return resources
	}

	// Convert to KubernetesResource struct
	resource := core.KubernetesResource{
		APIVersion: getStringFromMap(doc, "apiVersion"),
		Kind:       getStringFromMap(doc, "kind"),
	}

	if metadata, ok := doc["metadata"].(map[string]interface{}); ok {
		resource.Name = getStringFromMap(metadata, "name")
		resource.Namespace = getStringFromMap(metadata, "namespace")
	}

	if resource.APIVersion != "" || resource.Kind != "" {
		resources = append(resources, resource)
	}

	return resources
}

// Helper function to safely get string from map
func getStringFromMap(m map[string]interface{}, key string) string {
	if value, ok := m[key].(string); ok {
		return value
	}
	return ""
}

// Migration helpers for backward compatibility

// MigrateManifestValidatorToUnified provides a drop-in replacement for legacy ManifestValidator
func MigrateManifestValidatorToUnified(logger *slog.Logger) *UnifiedManifestValidator {
	return NewUnifiedManifestValidator(logger)
}
