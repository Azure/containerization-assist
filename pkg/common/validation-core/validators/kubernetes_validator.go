package validators

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/common/validation-core/core"
	errors "github.com/Azure/container-kit/pkg/mcp/errors"
	mcperrors "github.com/Azure/container-kit/pkg/mcp/errors"
)

// This file contains the main KubernetesValidator interface and coordination logic.
// Type definitions are in kubernetes_types.go
// Detailed validation functions are in other kubernetes_*.go files

// NewKubernetesValidator creates a new Kubernetes validator
func NewKubernetesValidator() *KubernetesValidator {
	validator := &KubernetesValidator{
		BaseValidatorImpl:  NewBaseValidator("kubernetes", "1.0.0", []string{"kubernetes", "yaml", "manifest", "k8s"}),
		strictMode:         false,
		securityValidation: true,
		allowedVersions:    getDefaultK8sVersions(),
	}

	return validator
}

// WithStrictMode enables or disables strict validation
func (k *KubernetesValidator) WithStrictMode(enabled bool) *KubernetesValidator {
	k.strictMode = enabled
	return k
}

// WithSecurityValidation enables or disables security validation
func (k *KubernetesValidator) WithSecurityValidation(enabled bool) *KubernetesValidator {
	k.securityValidation = enabled
	return k
}

// Validate validates Kubernetes manifest data (legacy method for backward compatibility)
func (k *KubernetesValidator) Validate(ctx context.Context, data interface{}, options *core.ValidationOptions) *core.NonGenericResult {
	startTime := time.Now()
	result := k.BaseValidatorImpl.Validate(ctx, data, options)

	switch v := data.(type) {
	case string:
		// YAML string content
		k.validateYAMLString(v, result, options)
	case []byte:
		// YAML byte content
		k.validateYAMLString(string(v), result, options)
	case map[string]interface{}:
		// Parsed manifest
		k.validateManifest(v, result, options)
	case []map[string]interface{}:
		// Multiple manifests
		for i, manifest := range v {
			k.validateManifestWithIndex(manifest, i, result, options)
		}
	case []ManifestData:
		// Multiple typed manifests
		for i, manifest := range v {
			k.validateManifestDataWithIndex(manifest, i, result, options)
		}
	case ManifestData:
		// Structured manifest data
		k.validateManifestData(v, result, options)
	default:
		result.AddError(&core.Error{
			Code:     "INVALID_MANIFEST_DATA",
			Message:  fmt.Sprintf("Expected Kubernetes manifest data, got %T", data),
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityHigh,
		})
	}

	result.Duration = time.Since(startTime)
	return result
}

// ValidateTyped validates Kubernetes manifest data with type safety
func (k *KubernetesValidator) ValidateTyped(ctx context.Context, manifests []ManifestData, options *core.ValidationOptions) *core.NonGenericResult {
	startTime := time.Now()
	result := k.BaseValidatorImpl.Validate(ctx, manifests, options)

	for i, manifest := range manifests {
		k.validateManifestDataWithIndex(manifest, i, result, options)
	}

	result.Duration = time.Since(startTime)
	return result
}

// ValidateFullyTyped validates Kubernetes manifests with complete type safety (no interface{} usage)
func (k *KubernetesValidator) ValidateFullyTyped(_ context.Context, manifests []TypedManifestData, options *core.ValidationOptions) *core.Result[[]TypedManifestData] {
	startTime := time.Now()
	result := core.NewGenericResult[[]TypedManifestData](k.Name, k.Version)
	result.Data = manifests

	// Add validation context if available
	if k.ValidationContext != nil {
		result.Metadata.Context["session_id"] = k.ValidationContext.SessionID
		result.Metadata.Context["tool"] = k.ValidationContext.Tool
		result.Metadata.Context["operation"] = k.ValidationContext.Operation
	}

	for i, manifest := range manifests {
		k.validateTypedManifestWithIndex(manifest, i, result, options)
	}

	result.Duration = time.Since(startTime)
	return result
}

// ValidateSingleFullyTyped validates a single typed Kubernetes manifest with complete type safety
func (k *KubernetesValidator) ValidateSingleFullyTyped(_ context.Context, manifest TypedManifestData, options *core.ValidationOptions) *core.Result[TypedManifestData] {
	startTime := time.Now()
	result := core.NewGenericResult[TypedManifestData](k.Name, k.Version)
	result.Data = manifest

	// Add validation context if available
	if k.ValidationContext != nil {
		result.Metadata.Context["session_id"] = k.ValidationContext.SessionID
		result.Metadata.Context["tool"] = k.ValidationContext.Tool
		result.Metadata.Context["operation"] = k.ValidationContext.Operation
	}

	k.validateTypedManifestWithIndex(manifest, 0, result, options)

	result.Duration = time.Since(startTime)
	return result
}

// validateTypedManifestWithIndex validates a typed manifest with index for multi-document support
func (k *KubernetesValidator) validateTypedManifestWithIndex(manifest TypedManifestData, index int, result interface{}, _ *core.ValidationOptions) {
	fieldPrefix := ""
	if index > 0 {
		fieldPrefix = fmt.Sprintf("document[%d].", index)
	}

	// Validate required fields with type safety
	if manifest.APIVersion == "" {
		k.addTypedError(result, "MISSING_API_VERSION", "apiVersion field is required", fieldPrefix+"apiVersion")
		return
	}

	if manifest.Kind == "" {
		k.addTypedError(result, "MISSING_KIND", "kind field is required", fieldPrefix+"kind")
		return
	}

	if manifest.Metadata == nil {
		k.addTypedError(result, "MISSING_METADATA", "metadata field is required", fieldPrefix+"metadata")
		return
	}

	if manifest.Metadata.Name == "" {
		k.addTypedError(result, "MISSING_NAME", "metadata.name field is required", fieldPrefix+"metadata.name")
		return
	}

	// Validate name format (basic Kubernetes naming rules)
	if err := k.validateKubernetesName(manifest.Metadata.Name); err != nil {
		k.addTypedError(result, "INVALID_NAME", err.Error(), fieldPrefix+"metadata.name")
	}

	// Kind-specific validation can be added here based on requirements
	// For now, we focus on the essential structural validation
}

// addTypedError adds an error to the result, handling different result types
func (k *KubernetesValidator) addTypedError(result interface{}, code, message, field string) {
	validationError := core.NewError(code, message, core.ErrTypeValidation, core.SeverityHigh)
	validationError.WithField(field)

	// Handle different result types using type assertion
	switch r := result.(type) {
	case *core.Result[TypedManifestData]:
		r.Valid = false
		r.Errors = append(r.Errors, validationError)
	case *core.Result[[]TypedManifestData]:
		r.Valid = false
		r.Errors = append(r.Errors, validationError)
	default:
		// If we can't determine the type, we can't add the error
		// This should not happen in normal operation but we handle it gracefully
		_ = code    // Suppress unused variable warning
		_ = message // Suppress unused variable warning
		_ = field   // Suppress unused variable warning
	}
}

// validateKubernetesName validates Kubernetes resource names according to RFC 1123
func (k *KubernetesValidator) validateKubernetesName(name string) error {
	if name == "" {
		return fmt.Errorf("resource name cannot be empty")
	}

	if len(name) > 253 {
		return fmt.Errorf("resource name too long: %d characters (max 253)", len(name))
	}

	// Kubernetes names must consist of lower case alphanumeric characters or '-',
	// and must start and end with an alphanumeric character
	if name[0] == '-' || name[len(name)-1] == '-' {
		return fmt.Errorf("resource name '%s' cannot start or end with '-'", name)
	}

	for i, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
			return fmt.Errorf("resource name '%s' contains invalid character '%c' at position %d", name, r, i)
		}
	}

	return nil
}

// ValidateSingleTyped validates a single typed Kubernetes manifest
func (k *KubernetesValidator) ValidateSingleTyped(ctx context.Context, manifest ManifestData, options *core.ValidationOptions) *core.NonGenericResult {
	startTime := time.Now()
	result := k.BaseValidatorImpl.Validate(ctx, manifest, options)

	k.validateManifestData(manifest, result, options)

	result.Duration = time.Since(startTime)
	return result
}

// ConvertToManifestData converts a raw manifest map to structured ManifestData
func ConvertToManifestData(manifest map[string]interface{}) (ManifestData, error) {
	data := ManifestData{
		Raw: manifest, // Keep raw data for backward compatibility
	}

	// Extract basic fields
	if apiVersion, ok := manifest["apiVersion"].(string); ok {
		data.APIVersion = apiVersion
	}

	if kind, ok := manifest["kind"].(string); ok {
		data.Kind = kind
	}

	// Extract metadata
	if metadataMap, ok := manifest["metadata"].(map[string]interface{}); ok {
		metadata, err := convertToObjectMetadata(metadataMap)
		if err != nil {
			return data, mcperrors.NewError().Messagef("failed to convert metadata: %w", err).WithLocation().Build()
		}
		data.Metadata = &metadata
	}

	// Extract spec
	if specMap, ok := manifest["spec"].(map[string]interface{}); ok {
		spec, err := convertToResourceSpec(specMap)
		if err != nil {
			return data, mcperrors.NewError().Messagef("failed to convert spec: %w", err).WithLocation().Build()
		}
		data.Spec = &spec
	}

	// Extract data (for ConfigMaps)
	if dataMap, ok := manifest["data"].(map[string]interface{}); ok {
		data.Data = make(map[string]string)
		for k, v := range dataMap {
			if str, ok := v.(string); ok {
				data.Data[k] = str
			}
		}
	}

	// Extract stringData (for Secrets)
	if stringDataMap, ok := manifest["stringData"].(map[string]interface{}); ok {
		data.StringData = make(map[string]string)
		for k, v := range stringDataMap {
			if str, ok := v.(string); ok {
				data.StringData[k] = str
			}
		}
	}

	return data, nil
}

// convertToObjectMetadata converts a raw metadata map to structured ObjectMetadata
func convertToObjectMetadata(metadataMap map[string]interface{}) (ObjectMetadata, error) {
	metadata := ObjectMetadata{}

	if name, ok := metadataMap["name"].(string); ok {
		metadata.Name = name
	}

	if namespace, ok := metadataMap["namespace"].(string); ok {
		metadata.Namespace = namespace
	}

	if labelsMap, ok := metadataMap["labels"].(map[string]string); ok {
		metadata.Labels = labelsMap
	} else if labelsInterface, ok := metadataMap["labels"].(map[string]interface{}); ok {
		// Convert interface{} values to strings
		metadata.Labels = make(map[string]string)
		for k, v := range labelsInterface {
			if strVal, ok := v.(string); ok {
				metadata.Labels[k] = strVal
			}
		}
	}

	if annotationsMap, ok := metadataMap["annotations"].(map[string]string); ok {
		metadata.Annotations = annotationsMap
	} else if annotationsInterface, ok := metadataMap["annotations"].(map[string]interface{}); ok {
		// Convert interface{} values to strings
		metadata.Annotations = make(map[string]string)
		for k, v := range annotationsInterface {
			if strVal, ok := v.(string); ok {
				metadata.Annotations[k] = strVal
			}
		}
	}

	return metadata, nil
}

// convertToResourceSpec converts a raw spec map to structured ResourceSpec
func convertToResourceSpec(specMap map[string]interface{}) (ResourceSpec, error) {
	spec := ResourceSpec{
		Raw: specMap, // Keep raw data for backward compatibility
	}

	if replicas, ok := specMap["replicas"].(int32); ok {
		spec.Replicas = &replicas
	} else if replicasFloat, ok := specMap["replicas"].(float64); ok {
		replicasInt := int32(replicasFloat)
		spec.Replicas = &replicasInt
	}

	if selectorMap, ok := specMap["selector"].(map[string]interface{}); ok {
		selector, err := convertToLabelSelector(selectorMap)
		if err != nil {
			return spec, errors.NewError().Message("failed to convert selector").Cause(err).WithLocation().Build()
		}
		spec.Selector = &selector
	}

	if typeStr, ok := specMap["type"].(string); ok {
		spec.Type = typeStr
	}

	// Convert ports
	if portsInterface, ok := specMap["ports"].([]interface{}); ok {
		for _, portInterface := range portsInterface {
			if portMap, ok := portInterface.(map[string]interface{}); ok {
				port, err := convertToServicePort(portMap)
				if err == nil {
					spec.Ports = append(spec.Ports, port)
				}
			}
		}
	}

	return spec, nil
}

// convertToLabelSelector converts map[string]interface{} to LabelSelector
func convertToLabelSelector(selectorMap map[string]interface{}) (LabelSelector, error) {
	selector := LabelSelector{}

	if matchLabelsMap, ok := selectorMap["matchLabels"].(map[string]string); ok {
		selector.MatchLabels = matchLabelsMap
	} else if matchLabelsInterface, ok := selectorMap["matchLabels"].(map[string]interface{}); ok {
		selector.MatchLabels = make(map[string]string)
		for k, v := range matchLabelsInterface {
			if strVal, ok := v.(string); ok {
				selector.MatchLabels[k] = strVal
			}
		}
	}

	return selector, nil
}

// convertToServicePort converts a raw port map to structured ServicePort
func convertToServicePort(portMap map[string]interface{}) (ServicePort, error) {
	port := ServicePort{}

	if name, ok := portMap["name"].(string); ok {
		port.Name = name
	}

	if portNum, ok := portMap["port"].(int32); ok {
		port.Port = portNum
	} else if portFloat, ok := portMap["port"].(float64); ok {
		port.Port = int32(portFloat)
	}

	if targetPort, ok := portMap["targetPort"].(int32); ok {
		port.TargetPort = targetPort
	} else if targetPortFloat, ok := portMap["targetPort"].(float64); ok {
		port.TargetPort = int32(targetPortFloat)
	}

	if protocol, ok := portMap["protocol"].(string); ok {
		port.Protocol = protocol
	}

	return port, nil
}
