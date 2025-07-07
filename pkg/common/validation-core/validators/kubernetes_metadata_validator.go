package validators

import (
	"fmt"
	"strings"

	"github.com/Azure/container-kit/pkg/common/validation-core/core"
)

// Metadata and Annotation Validation
// This file contains functions for validating Kubernetes metadata, labels, and annotations

// validateMetadata validates metadata section
func (k *KubernetesValidator) validateMetadata(metadata map[string]interface{}, fieldPrefix string, result *core.NonGenericResult) {
	// Name validation
	if name, exists := metadata["name"]; exists {
		if nameStr, ok := name.(string); ok {
			k.validateResourceName(nameStr, fieldPrefix+".name", result)
		} else {
			result.AddFieldError(fieldPrefix+".name", "Name must be a string")
		}
	} else {
		result.AddError(&core.Error{
			Code:     "MISSING_METADATA_NAME",
			Message:  "metadata.name is required",
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityHigh,
			Field:    fieldPrefix + ".name",
		})
	}

	// Namespace validation
	if namespace, exists := metadata["namespace"]; exists {
		if namespaceStr, ok := namespace.(string); ok {
			k.validateResourceName(namespaceStr, fieldPrefix+".namespace", result)
		}
	}

	// Labels validation
	if labels, exists := metadata["labels"]; exists {
		if labelsMap, ok := labels.(map[string]interface{}); ok {
			k.validateLabels(labelsMap, fieldPrefix+".labels", result)
		}
	}

	// Annotations validation
	if annotations, exists := metadata["annotations"]; exists {
		if annotationsMap, ok := annotations.(map[string]interface{}); ok {
			k.validateAnnotations(annotationsMap, fieldPrefix+".annotations", result)
		}
	}
}

// validateResourceName validates Kubernetes resource names
func (k *KubernetesValidator) validateResourceName(name, field string, result *core.NonGenericResult) {
	if name == "" {
		result.AddFieldError(field, "Resource name cannot be empty")
		return
	}

	// Kubernetes name validation rules
	if len(name) > 253 {
		result.AddFieldError(field, "Resource name cannot exceed 253 characters")
	}

	// Check valid characters (lowercase alphanumeric, hyphens, dots)
	for i, char := range name {
		if !((char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-' || char == '.') {
			nameError := &core.Error{
				Code:     "INVALID_RESOURCE_NAME",
				Message:  fmt.Sprintf("Invalid character '%c' at position %d in resource name", char, i),
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityHigh,
				Field:    field,
			}
			nameError.WithSuggestion("Use only lowercase letters, numbers, hyphens, and dots")
			result.AddError(nameError)
			break
		}
	}

	// Names cannot start or end with hyphens or dots
	if strings.HasPrefix(name, "-") || strings.HasPrefix(name, ".") ||
		strings.HasSuffix(name, "-") || strings.HasSuffix(name, ".") {
		result.AddError(&core.Error{
			Code:     "INVALID_RESOURCE_NAME_FORMAT",
			Message:  "Resource name cannot start or end with hyphen or dot",
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityHigh,
			Field:    field,
		})
	}
}

// validateLabels validates Kubernetes labels
func (k *KubernetesValidator) validateLabels(labels map[string]interface{}, fieldPrefix string, result *core.NonGenericResult) {
	for key, value := range labels {
		// Validate label key
		k.validateLabelKey(key, fieldPrefix+"."+key, result)

		// Validate label value
		if valueStr, ok := value.(string); ok {
			k.validateLabelValue(valueStr, fieldPrefix+"."+key, result)
		} else {
			result.AddFieldError(fieldPrefix+"."+key, "Label value must be a string")
		}
	}
}

// validateLabelKey validates Kubernetes label keys
func (k *KubernetesValidator) validateLabelKey(key, field string, result *core.NonGenericResult) {
	if key == "" {
		result.AddFieldError(field, "Label key cannot be empty")
		return
	}

	// Check length
	if len(key) > 63 {
		result.AddFieldError(field, "Label key cannot exceed 63 characters")
	}

	// Validate format (alphanumeric, hyphens, underscores, dots)
	for _, char := range key {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '-' || char == '_' || char == '.') {
			result.AddFieldError(field, "Label key contains invalid characters")
			break
		}
	}
}

// validateLabelValue validates Kubernetes label values
func (k *KubernetesValidator) validateLabelValue(value, field string, result *core.NonGenericResult) {
	// Check length
	if len(value) > 63 {
		result.AddFieldError(field, "Label value cannot exceed 63 characters")
	}

	// Value can be empty, but if not empty, must follow rules
	if value != "" {
		for _, char := range value {
			if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
				(char >= '0' && char <= '9') || char == '-' || char == '_' || char == '.') {
				result.AddFieldError(field, "Label value contains invalid characters")
				break
			}
		}
	}
}

// validateAnnotations validates Kubernetes annotations
func (k *KubernetesValidator) validateAnnotations(annotations map[string]interface{}, fieldPrefix string, result *core.NonGenericResult) {
	for key, value := range annotations {
		// Validate annotation value is string
		if _, ok := value.(string); !ok {
			result.AddFieldError(fieldPrefix+"."+key, "Annotation value must be a string")
		}

		// Validate annotation key format (more lenient than labels)
		if len(key) > 253 {
			result.AddFieldError(fieldPrefix+"."+key, "Annotation key cannot exceed 253 characters")
		}
	}
}

// validateMetadataTyped validates typed metadata structure
func (k *KubernetesValidator) validateMetadataTyped(metadata *ObjectMetadata, fieldPrefix string, result *core.NonGenericResult) {
	// Name validation
	if metadata.Name == "" {
		result.AddError(&core.Error{
			Code:     "MISSING_METADATA_NAME",
			Message:  fmt.Sprintf("Missing required field: %s.name", fieldPrefix),
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityHigh,
			Field:    fieldPrefix + ".name",
		})
	} else {
		k.validateResourceName(metadata.Name, fieldPrefix+".name", result)
	}

	// Namespace validation
	if metadata.Namespace != "" {
		k.validateResourceName(metadata.Namespace, fieldPrefix+".namespace", result)
	}

	// Labels validation
	if metadata.Labels != nil {
		for key, value := range metadata.Labels {
			k.validateLabelKey(key, fieldPrefix+".labels."+key, result)
			k.validateLabelValue(value, fieldPrefix+".labels."+key, result)
		}
	}

	// Annotations validation
	if metadata.Annotations != nil {
		for key, value := range metadata.Annotations {
			if len(key) > 253 {
				result.AddFieldError(fieldPrefix+".annotations."+key, "Annotation key cannot exceed 253 characters")
			}
			if len(value) > 262144 { // 256KB limit
				result.AddFieldError(fieldPrefix+".annotations."+key, "Annotation value cannot exceed 256KB")
			}
		}
	}
}

// isValidKubernetesName validates if a string is a valid Kubernetes name
func (k *KubernetesValidator) isValidKubernetesName(name string) bool {
	if name == "" || len(name) > 253 {
		return false
	}

	// Check valid characters (lowercase alphanumeric, hyphens, dots)
	for _, char := range name {
		if !((char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-' || char == '.') {
			return false
		}
	}

	// Names cannot start or end with hyphens or dots
	if strings.HasPrefix(name, "-") || strings.HasPrefix(name, ".") ||
		strings.HasSuffix(name, "-") || strings.HasSuffix(name, ".") {
		return false
	}

	return true
}
