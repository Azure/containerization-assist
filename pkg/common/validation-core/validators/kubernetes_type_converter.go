package validators

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/common/validation-core/core"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"gopkg.in/yaml.v3"
)

// Type Conversion Utilities
// This file contains functions for converting between interface{} and typed structures

// ValidateWithAutoConversion validates data by automatically converting to typed structures when possible
func (k *KubernetesValidator) ValidateWithAutoConversion(ctx context.Context, data interface{}, options *core.ValidationOptions) *core.NonGenericResult {
	startTime := time.Now()
	result := k.BaseValidatorImpl.Validate(ctx, data, options)

	switch v := data.(type) {
	case map[string]interface{}:
		// Try to convert to typed structure first
		if manifestData, err := ConvertToManifestData(v); err == nil {
			k.validateManifestData(manifestData, result, options)
		} else {
			// Fall back to untyped validation
			k.validateManifest(v, result, options)
		}
	case []map[string]interface{}:
		// Convert multiple manifests
		for i, manifest := range v {
			if manifestData, err := ConvertToManifestData(manifest); err == nil {
				k.validateManifestDataWithIndex(manifestData, i, result, options)
			} else {
				k.validateManifestWithIndex(manifest, i, result, options)
			}
		}
	default:
		// Use existing validation logic for other types
		return k.Validate(ctx, data, options)
	}

	result.Duration = time.Since(startTime)
	return result
}

// safeStringExtract safely extracts a string value from interface{} with error handling
func (k *KubernetesValidator) safeStringExtract(data interface{}, fieldName string) (string, error) {
	if data == nil {
		return "", errors.NewError().Messagef("field %s is nil", fieldName).WithLocation().Build()
	}

	if strVal, ok := data.(string); ok {
		return strVal, nil
	}

	return "", errors.NewError().Messagef("field %s is not a string, got %T", fieldName, data).WithLocation().Build()
}

// safeIntExtract safely extracts an int value from interface{} with error handling
func (k *KubernetesValidator) safeIntExtract(data interface{}, fieldName string) (int, error) {
	if data == nil {
		return 0, errors.NewError().Messagef("field %s is nil", fieldName).WithLocation().Build()
	}

	switch v := data.(type) {
	case int:
		return v, nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	default:
		return 0, errors.NewError().Messagef("field %s is not a numeric type, got %T", fieldName, data).WithLocation().Build()
	}
}

// convertMapToTypedMetadata safely converts map[string]interface{} to TypedObjectMetadata
func (k *KubernetesValidator) convertMapToTypedMetadata(metadataMap map[string]interface{}) (*TypedObjectMetadata, error) {
	if metadataMap == nil {
		return nil, errors.NewError().Messagef("metadata map is nil").WithLocation().Build()
	}

	metadata := &TypedObjectMetadata{}

	// Extract name (required)
	if nameVal, exists := metadataMap["name"]; exists {
		name, err := k.safeStringExtract(nameVal, "name")
		if err != nil {
			return nil, errors.NewError().Message("invalid metadata.name").Cause(err).WithLocation().Build()
		}
		metadata.Name = name
	}

	// Extract namespace (optional)
	if nsVal, exists := metadataMap["namespace"]; exists {
		namespace, err := k.safeStringExtract(nsVal, "namespace")
		if err != nil {
			return nil, errors.NewError().Message("invalid metadata.namespace").Cause(err).WithLocation().Build()
		}
		metadata.Namespace = namespace
	}

	// Extract labels (optional)
	if labelsVal, exists := metadataMap["labels"]; exists {
		if labelsMap, ok := labelsVal.(map[string]interface{}); ok {
			labels := make(map[string]string)
			for key, value := range labelsMap {
				strVal, err := k.safeStringExtract(value, fmt.Sprintf("labels.%s", key))
				if err != nil {
					return nil, errors.NewError().Message("invalid label value").Cause(err).WithLocation().Build()
				}
				labels[key] = strVal
			}
			metadata.Labels = labels
		}
	}

	// Extract annotations (optional)
	if annotationsVal, exists := metadataMap["annotations"]; exists {
		if annotationsMap, ok := annotationsVal.(map[string]interface{}); ok {
			annotations := make(map[string]string)
			for key, value := range annotationsMap {
				strVal, err := k.safeStringExtract(value, fmt.Sprintf("annotations.%s", key))
				if err != nil {
					return nil, errors.NewError().Message("invalid annotation value").Cause(err).WithLocation().Build()
				}
				annotations[key] = strVal
			}
			metadata.Annotations = annotations
		}
	}

	return metadata, nil
}

// parseYAMLToTypedManifest safely parses YAML string to TypedManifestData with error handling
func (k *KubernetesValidator) parseYAMLToTypedManifest(yamlContent string) (*TypedManifestData, error) {
	if strings.TrimSpace(yamlContent) == "" {
		return nil, errors.NewError().Messagef("YAML content is empty").WithLocation().Build()
	}

	// First parse to generic map to validate structure
	var rawManifest map[string]interface{}
	if err := yaml.Unmarshal([]byte(yamlContent), &rawManifest); err != nil {
		return nil, errors.NewError().Message("invalid YAML syntax").Cause(err).WithLocation().Build()
	}

	// Create typed manifest
	manifest := &TypedManifestData{}

	// Extract API version (required)
	if apiVersionVal, exists := rawManifest["apiVersion"]; exists {
		apiVersion, err := k.safeStringExtract(apiVersionVal, "apiVersion")
		if err != nil {
			return nil, errors.NewError().Message("invalid apiVersion").Cause(err).WithLocation().Build()
		}
		manifest.APIVersion = apiVersion
	} else {
		return nil, errors.NewError().Messagef("missing required field: apiVersion").WithLocation().Build()
	}

	// Extract kind (required)
	if kindVal, exists := rawManifest["kind"]; exists {
		kind, err := k.safeStringExtract(kindVal, "kind")
		if err != nil {
			return nil, errors.NewError().Message("invalid kind").Cause(err).WithLocation().Build()
		}
		manifest.Kind = kind
	} else {
		return nil, errors.NewError().Messagef("missing required field: kind").WithLocation().Build()
	}

	// Extract metadata (required)
	if metadataVal, exists := rawManifest["metadata"]; exists {
		if metadataMap, ok := metadataVal.(map[string]interface{}); ok {
			metadata, err := k.convertMapToTypedMetadata(metadataMap)
			if err != nil {
				return nil, errors.NewError().Message("invalid metadata").Cause(err).WithLocation().Build()
			}
			manifest.Metadata = metadata
		} else {
			return nil, errors.NewError().Messagef("metadata must be an object").WithLocation().Build()
		}
	} else {
		return nil, errors.NewError().Messagef("missing required field: metadata").WithLocation().Build()
	}

	// Extract optional fields
	if dataVal, exists := rawManifest["data"]; exists {
		if dataMap, ok := dataVal.(map[string]interface{}); ok {
			data := make(map[string]string)
			for key, value := range dataMap {
				strVal, err := k.safeStringExtract(value, fmt.Sprintf("data.%s", key))
				if err != nil {
					return nil, errors.NewError().Message("invalid data field").Cause(err).WithLocation().Build()
				}
				data[key] = strVal
			}
			manifest.Data = data
		}
	}

	return manifest, nil
}

// safeBoolExtract safely extracts a bool value from interface{} with error handling
func (k *KubernetesValidator) safeBoolExtract(data interface{}, fieldName string) (bool, error) {
	if data == nil {
		return false, errors.NewError().Messagef("field %s is nil", fieldName).WithLocation().Build()
	}

	if boolVal, ok := data.(bool); ok {
		return boolVal, nil
	}

	return false, errors.NewError().Messagef("field %s is not a boolean, got %T", fieldName, data).WithLocation().Build()
}

// safeMapExtract safely extracts a map from interface{} with error handling
func (k *KubernetesValidator) safeMapExtract(data interface{}, fieldName string) (map[string]interface{}, error) {
	if data == nil {
		return nil, errors.NewError().Messagef("field %s is nil", fieldName).WithLocation().Build()
	}

	if mapVal, ok := data.(map[string]interface{}); ok {
		return mapVal, nil
	}

	return nil, errors.NewError().Messagef("field %s is not a map, got %T", fieldName, data).WithLocation().Build()
}

// safeSliceExtract safely extracts a slice from interface{} with error handling
func (k *KubernetesValidator) safeSliceExtract(data interface{}, fieldName string) ([]interface{}, error) {
	if data == nil {
		return nil, errors.NewError().Messagef("field %s is nil", fieldName).WithLocation().Build()
	}

	if sliceVal, ok := data.([]interface{}); ok {
		return sliceVal, nil
	}

	return nil, errors.NewError().Messagef("field %s is not a slice, got %T", fieldName, data).WithLocation().Build()
}
