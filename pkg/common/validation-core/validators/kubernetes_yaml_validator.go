package validators

import (
	"fmt"

	"github.com/Azure/container-kit/pkg/common/validation-core/core"
	"gopkg.in/yaml.v3"
)

// YAML Parsing and Basic Validation
// This file contains functions for parsing YAML content and performing basic structure validation

// validateYAMLString validates YAML string content
func (k *KubernetesValidator) validateYAMLString(yamlContent string, result *core.NonGenericResult, options *core.ValidationOptions) {
	// First validate YAML syntax
	var manifest map[string]interface{}
	if err := yaml.Unmarshal([]byte(yamlContent), &manifest); err != nil {
		yamlError := &core.Error{
			Code:     "INVALID_YAML",
			Message:  fmt.Sprintf("Invalid YAML syntax: %v", err),
			Type:     core.ErrTypeSyntax,
			Severity: core.SeverityHigh,
		}
		yamlError.WithSuggestion("Check YAML indentation and syntax")
		result.AddError(yamlError)
		return
	}

	// Validate as Kubernetes manifest
	k.validateManifest(manifest, result, options)
}

// validateManifest validates a single Kubernetes manifest
func (k *KubernetesValidator) validateManifest(manifest map[string]interface{}, result *core.NonGenericResult, options *core.ValidationOptions) {
	k.validateManifestWithIndex(manifest, -1, result, options)
}

// validateManifestWithIndex validates a manifest with index for multi-document YAML
func (k *KubernetesValidator) validateManifestWithIndex(manifest map[string]interface{}, index int, result *core.NonGenericResult, _ *core.ValidationOptions) {
	fieldPrefix := ""
	if index >= 0 {
		fieldPrefix = fmt.Sprintf("document[%d].", index)
	}

	// Validate required Kubernetes fields
	k.validateRequiredFields(manifest, fieldPrefix, result)

	// Validate API version and kind
	if apiVersion, ok := manifest["apiVersion"].(string); ok {
		if kind, ok := manifest["kind"].(string); ok {
			k.validateAPIVersionAndKind(apiVersion, kind, fieldPrefix, result)
		}
	}

	// Validate metadata
	if metadata, ok := manifest["metadata"].(map[string]interface{}); ok {
		k.validateMetadata(metadata, fieldPrefix+"metadata", result)
	}

	// Validate spec based on resource kind
	if kind, ok := manifest["kind"].(string); ok {
		if spec, ok := manifest["spec"].(map[string]interface{}); ok {
			k.validateSpec(kind, spec, fieldPrefix+"spec", result)
		}
	}

	// Security validation
	if k.securityValidation {
		k.performSecurityValidation(manifest, fieldPrefix, result)
	}

	// Resource-specific validation
	k.validateResourceSpecific(manifest, fieldPrefix, result)
}

// validateManifestData validates structured manifest data
func (k *KubernetesValidator) validateManifestData(data ManifestData, result *core.NonGenericResult, options *core.ValidationOptions) {
	// Validate API version and kind
	k.validateAPIVersionAndKind(data.APIVersion, data.Kind, "", result)

	// Validate metadata
	if data.Metadata != nil {
		k.validateMetadataTyped(data.Metadata, "metadata", result)
	}

	// Validate spec
	if data.Spec != nil {
		k.validateSpecTyped(data.Kind, data.Spec, "spec", result)
	}

	// If raw data is available, validate it as well
	if data.Raw != nil {
		k.validateManifest(data.Raw, result, options)
	}
}

// validateManifestDataWithIndex validates structured manifest data with index for multi-document YAML
func (k *KubernetesValidator) validateManifestDataWithIndex(data ManifestData, index int, result *core.NonGenericResult, options *core.ValidationOptions) {
	fieldPrefix := fmt.Sprintf("document[%d].", index)

	// Validate API version and kind
	k.validateAPIVersionAndKind(data.APIVersion, data.Kind, fieldPrefix, result)

	// Validate metadata
	if data.Metadata != nil {
		k.validateMetadataTyped(data.Metadata, fieldPrefix+"metadata", result)
	}

	// Validate spec
	if data.Spec != nil {
		k.validateSpecTyped(data.Kind, data.Spec, fieldPrefix+"spec", result)
	}

	// If raw data is available, validate it as well
	if data.Raw != nil {
		k.validateManifestWithIndex(data.Raw, index, result, options)
	}
}

// validateRequiredFields validates required Kubernetes fields
func (k *KubernetesValidator) validateRequiredFields(manifest map[string]interface{}, fieldPrefix string, result *core.NonGenericResult) {
	requiredFields := []string{"apiVersion", "kind", "metadata"}

	for _, field := range requiredFields {
		if _, exists := manifest[field]; !exists {
			requiredFieldError := &core.Error{
				Code:     "MISSING_REQUIRED_FIELD",
				Message:  fmt.Sprintf("Missing required field: %s%s", fieldPrefix, field),
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityHigh,
				Field:    fieldPrefix + field,
			}
			requiredFieldError.WithSuggestion(fmt.Sprintf("Add the required '%s' field to the manifest", field))
			result.AddError(requiredFieldError)
		}
	}
}

// validateAPIVersionAndKind validates API version and kind compatibility
func (k *KubernetesValidator) validateAPIVersionAndKind(apiVersion, kind, fieldPrefix string, result *core.NonGenericResult) {
	// Validate API version format
	if apiVersion == "" {
		apiVersionError := &core.Error{
			Code:     "EMPTY_API_VERSION",
			Message:  fmt.Sprintf("API version cannot be empty at %sapiVersion", fieldPrefix),
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityHigh,
			Field:    fieldPrefix + "apiVersion",
		}
		apiVersionError.WithSuggestion("Specify a valid Kubernetes API version (e.g., 'apps/v1', 'v1')")
		result.AddError(apiVersionError)
		return
	}

	// Validate kind
	if kind == "" {
		kindError := &core.Error{
			Code:     "EMPTY_KIND",
			Message:  fmt.Sprintf("Kind cannot be empty at %skind", fieldPrefix),
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityHigh,
			Field:    fieldPrefix + "kind",
		}
		kindError.WithSuggestion("Specify a valid Kubernetes resource kind (e.g., 'Deployment', 'Service')")
		result.AddError(kindError)
		return
	}

	// Check if this combination is allowed (if allowedVersions is configured)
	if k.allowedVersions != nil {
		if allowedKinds, apiVersionExists := k.allowedVersions[apiVersion]; apiVersionExists {
			kindAllowed := false
			for _, allowedKind := range allowedKinds {
				if allowedKind == kind {
					kindAllowed = true
					break
				}
			}
			if !kindAllowed {
				kindNotAllowedError := &core.Error{
					Code:     "KIND_NOT_ALLOWED_FOR_API_VERSION",
					Message:  fmt.Sprintf("Kind '%s' is not allowed for API version '%s' at %s", kind, apiVersion, fieldPrefix),
					Type:     core.ErrTypeValidation,
					Severity: core.SeverityMedium,
					Field:    fieldPrefix + "kind",
				}
				kindNotAllowedError.WithSuggestion(fmt.Sprintf("Use one of the allowed kinds for API version '%s': %v", apiVersion, allowedKinds))
				result.AddError(kindNotAllowedError)
			}
		} else if k.strictMode {
			// In strict mode, unknown API versions are not allowed
			unknownAPIVersionError := &core.Error{
				Code:     "UNKNOWN_API_VERSION",
				Message:  fmt.Sprintf("Unknown API version '%s' at %sapiVersion", apiVersion, fieldPrefix),
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityMedium,
				Field:    fieldPrefix + "apiVersion",
			}
			allowedVersionsList := make([]string, 0, len(k.allowedVersions))
			for version := range k.allowedVersions {
				allowedVersionsList = append(allowedVersionsList, version)
			}
			unknownAPIVersionError.WithSuggestion(fmt.Sprintf("Use one of the allowed API versions: %v", allowedVersionsList))
			result.AddError(unknownAPIVersionError)
		}
	}
}
