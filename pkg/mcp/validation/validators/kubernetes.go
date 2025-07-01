package validators

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/validation/core"
	"gopkg.in/yaml.v3"
)

// KubernetesValidator validates Kubernetes manifest files and resources
type KubernetesValidator struct {
	*BaseValidatorImpl
	strictMode         bool
	securityValidation bool
	allowedVersions    map[string][]string // apiVersion -> kind mappings
}

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

// Validate validates Kubernetes manifest data
func (k *KubernetesValidator) Validate(ctx context.Context, data interface{}, options *core.ValidationOptions) *core.ValidationResult {
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
	case ManifestData:
		// Structured manifest data
		k.validateManifestData(v, result, options)
	default:
		result.AddError(&core.ValidationError{
			Code:     "INVALID_MANIFEST_DATA",
			Message:  fmt.Sprintf("Expected Kubernetes manifest data, got %T", data),
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityHigh,
		})
	}

	result.Duration = time.Since(startTime)
	return result
}

// ManifestData represents structured Kubernetes manifest data
type ManifestData struct {
	APIVersion string                 `json:"apiVersion"`
	Kind       string                 `json:"kind"`
	Metadata   map[string]interface{} `json:"metadata"`
	Spec       map[string]interface{} `json:"spec"`
	Data       map[string]interface{} `json:"data,omitempty"`
	BinaryData map[string][]byte      `json:"binaryData,omitempty"`
	Raw        map[string]interface{} `json:"raw,omitempty"` // Full manifest data
}

// validateYAMLString validates YAML string content
func (k *KubernetesValidator) validateYAMLString(yamlContent string, result *core.ValidationResult, options *core.ValidationOptions) {
	// First validate YAML syntax
	var manifest map[string]interface{}
	if err := yaml.Unmarshal([]byte(yamlContent), &manifest); err != nil {
		yamlError := &core.ValidationError{
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
func (k *KubernetesValidator) validateManifest(manifest map[string]interface{}, result *core.ValidationResult, options *core.ValidationOptions) {
	k.validateManifestWithIndex(manifest, -1, result, options)
}

// validateManifestWithIndex validates a manifest with index for multi-document YAML
func (k *KubernetesValidator) validateManifestWithIndex(manifest map[string]interface{}, index int, result *core.ValidationResult, options *core.ValidationOptions) {
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
func (k *KubernetesValidator) validateManifestData(data ManifestData, result *core.ValidationResult, options *core.ValidationOptions) {
	// Validate API version and kind
	k.validateAPIVersionAndKind(data.APIVersion, data.Kind, "", result)

	// Validate metadata
	if data.Metadata != nil {
		k.validateMetadata(data.Metadata, "metadata", result)
	}

	// Validate spec
	if data.Spec != nil {
		k.validateSpec(data.Kind, data.Spec, "spec", result)
	}

	// If raw data is available, validate it as well
	if data.Raw != nil {
		k.validateManifest(data.Raw, result, options)
	}
}

// validateRequiredFields validates required Kubernetes fields
func (k *KubernetesValidator) validateRequiredFields(manifest map[string]interface{}, fieldPrefix string, result *core.ValidationResult) {
	requiredFields := []string{"apiVersion", "kind", "metadata"}

	for _, field := range requiredFields {
		if _, exists := manifest[field]; !exists {
			fieldError := &core.ValidationError{
				Code:     "MISSING_REQUIRED_FIELD",
				Message:  fmt.Sprintf("Missing required field: %s", field),
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityCritical,
				Field:    fieldPrefix + field,
			}
			fieldError.WithSuggestion(fmt.Sprintf("Add the required %s field", field))
			result.AddError(fieldError)
		}
	}
}

// validateAPIVersionAndKind validates API version and kind combination
func (k *KubernetesValidator) validateAPIVersionAndKind(apiVersion, kind, fieldPrefix string, result *core.ValidationResult) {
	// Validate API version format
	if apiVersion == "" {
		result.AddFieldError(fieldPrefix+"apiVersion", "API version cannot be empty")
		return
	}

	// Validate kind
	if kind == "" {
		result.AddFieldError(fieldPrefix+"kind", "Kind cannot be empty")
		return
	}

	// Check if the API version and kind combination is valid
	if allowedKinds, exists := k.allowedVersions[apiVersion]; exists {
		kindFound := false
		for _, allowedKind := range allowedKinds {
			if strings.EqualFold(kind, allowedKind) {
				kindFound = true
				break
			}
		}

		if !kindFound {
			if k.strictMode {
				result.AddError(&core.ValidationError{
					Code:     "INVALID_KIND_FOR_API_VERSION",
					Message:  fmt.Sprintf("Kind '%s' is not valid for API version '%s'", kind, apiVersion),
					Type:     core.ErrTypeValidation,
					Severity: core.SeverityHigh,
					Field:    fieldPrefix + "kind",
				})
			} else {
				result.AddWarning(&core.ValidationWarning{
					ValidationError: &core.ValidationError{
						Code:     "UNKNOWN_KIND_FOR_API_VERSION",
						Message:  fmt.Sprintf("Unknown kind '%s' for API version '%s'", kind, apiVersion),
						Type:     core.ErrTypeValidation,
						Severity: core.SeverityMedium,
						Field:    fieldPrefix + "kind",
					},
				})
			}
		}
	} else if k.strictMode {
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "UNKNOWN_API_VERSION",
				Message:  fmt.Sprintf("Unknown API version: %s", apiVersion),
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityLow,
				Field:    fieldPrefix + "apiVersion",
			},
		})
	}
}

// validateMetadata validates metadata section
func (k *KubernetesValidator) validateMetadata(metadata map[string]interface{}, fieldPrefix string, result *core.ValidationResult) {
	// Name validation
	if name, exists := metadata["name"]; exists {
		if nameStr, ok := name.(string); ok {
			k.validateResourceName(nameStr, fieldPrefix+".name", result)
		} else {
			result.AddFieldError(fieldPrefix+".name", "Name must be a string")
		}
	} else {
		result.AddError(&core.ValidationError{
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
func (k *KubernetesValidator) validateResourceName(name, field string, result *core.ValidationResult) {
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
			nameError := &core.ValidationError{
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
		result.AddError(&core.ValidationError{
			Code:     "INVALID_RESOURCE_NAME_FORMAT",
			Message:  "Resource name cannot start or end with hyphen or dot",
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityHigh,
			Field:    field,
		})
	}
}

// validateLabels validates Kubernetes labels
func (k *KubernetesValidator) validateLabels(labels map[string]interface{}, fieldPrefix string, result *core.ValidationResult) {
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
func (k *KubernetesValidator) validateLabelKey(key, field string, result *core.ValidationResult) {
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
func (k *KubernetesValidator) validateLabelValue(value, field string, result *core.ValidationResult) {
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
func (k *KubernetesValidator) validateAnnotations(annotations map[string]interface{}, fieldPrefix string, result *core.ValidationResult) {
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

// validateSpec validates spec section based on resource kind
func (k *KubernetesValidator) validateSpec(kind string, spec map[string]interface{}, fieldPrefix string, result *core.ValidationResult) {
	switch strings.ToLower(kind) {
	case "deployment":
		k.validateDeploymentSpec(spec, fieldPrefix, result)
	case "service":
		k.validateServiceSpec(spec, fieldPrefix, result)
	case "ingress":
		k.validateIngressSpec(spec, fieldPrefix, result)
	case "configmap":
		// ConfigMaps don't typically have spec, data is at root level
		return
	case "secret":
		// Secrets don't typically have spec, data is at root level
		return
	case "pod":
		k.validatePodSpec(spec, fieldPrefix, result)
	}
}

// validateDeploymentSpec validates Deployment spec
func (k *KubernetesValidator) validateDeploymentSpec(spec map[string]interface{}, fieldPrefix string, result *core.ValidationResult) {
	// Required fields
	requiredFields := []string{"selector", "template"}
	for _, field := range requiredFields {
		if _, exists := spec[field]; !exists {
			result.AddError(&core.ValidationError{
				Code:     "MISSING_DEPLOYMENT_FIELD",
				Message:  fmt.Sprintf("Deployment spec missing required field: %s", field),
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityHigh,
				Field:    fieldPrefix + "." + field,
			})
		}
	}

	// Validate replicas
	if replicas, exists := spec["replicas"]; exists {
		if replicasNum, ok := replicas.(int); ok {
			if replicasNum < 0 {
				result.AddFieldError(fieldPrefix+".replicas", "Replicas cannot be negative")
			}
		}
	}

	// Validate selector
	if selector, exists := spec["selector"]; exists {
		if selectorMap, ok := selector.(map[string]interface{}); ok {
			k.validateSelector(selectorMap, fieldPrefix+".selector", result)
		}
	}
}

// validateServiceSpec validates Service spec
func (k *KubernetesValidator) validateServiceSpec(spec map[string]interface{}, fieldPrefix string, result *core.ValidationResult) {
	// Validate ports
	if ports, exists := spec["ports"]; exists {
		if portsList, ok := ports.([]interface{}); ok {
			if len(portsList) == 0 {
				result.AddWarning(&core.ValidationWarning{
					ValidationError: &core.ValidationError{
						Code:     "EMPTY_SERVICE_PORTS",
						Message:  "Service has empty ports list",
						Type:     core.ErrTypeValidation,
						Severity: core.SeverityMedium,
						Field:    fieldPrefix + ".ports",
					},
				})
			} else {
				for i, port := range portsList {
					if portMap, ok := port.(map[string]interface{}); ok {
						k.validateServicePort(portMap, fmt.Sprintf("%s.ports[%d]", fieldPrefix, i), result)
					}
				}
			}
		}
	}

	// Validate service type
	if serviceType, exists := spec["type"]; exists {
		if typeStr, ok := serviceType.(string); ok {
			validTypes := []string{"ClusterIP", "NodePort", "LoadBalancer", "ExternalName"}
			found := false
			for _, validType := range validTypes {
				if typeStr == validType {
					found = true
					break
				}
			}
			if !found {
				result.AddFieldError(fieldPrefix+".type", fmt.Sprintf("Invalid service type: %s", typeStr))
			}
		}
	}
}

// validateIngressSpec validates Ingress spec
func (k *KubernetesValidator) validateIngressSpec(spec map[string]interface{}, fieldPrefix string, result *core.ValidationResult) {
	// Check for rules or defaultBackend
	hasRules := false
	hasDefaultBackend := false

	if _, exists := spec["rules"]; exists {
		hasRules = true
	}
	if _, exists := spec["defaultBackend"]; exists {
		hasDefaultBackend = true
	}

	if !hasRules && !hasDefaultBackend {
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "INGRESS_NO_ROUTES",
				Message:  "Ingress should define either rules or defaultBackend",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityMedium,
				Field:    fieldPrefix + ".rules",
			},
		})
	}
}

// validatePodSpec validates Pod spec
func (k *KubernetesValidator) validatePodSpec(spec map[string]interface{}, fieldPrefix string, result *core.ValidationResult) {
	// Validate containers
	if containers, exists := spec["containers"]; exists {
		if containersList, ok := containers.([]interface{}); ok {
			if len(containersList) == 0 {
				result.AddError(&core.ValidationError{
					Code:     "NO_CONTAINERS",
					Message:  "Pod spec must have at least one container",
					Type:     core.ErrTypeValidation,
					Severity: core.SeverityCritical,
					Field:    fieldPrefix + ".containers",
				})
			} else {
				for i, container := range containersList {
					if containerMap, ok := container.(map[string]interface{}); ok {
						k.validateContainer(containerMap, fmt.Sprintf("%s.containers[%d]", fieldPrefix, i), result)
					}
				}
			}
		}
	} else {
		result.AddError(&core.ValidationError{
			Code:     "MISSING_CONTAINERS",
			Message:  "Pod spec missing containers field",
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityCritical,
			Field:    fieldPrefix + ".containers",
		})
	}
}

// validateContainer validates container specification
func (k *KubernetesValidator) validateContainer(container map[string]interface{}, fieldPrefix string, result *core.ValidationResult) {
	// Required fields
	requiredFields := []string{"name", "image"}
	for _, field := range requiredFields {
		if _, exists := container[field]; !exists {
			result.AddError(&core.ValidationError{
				Code:     "MISSING_CONTAINER_FIELD",
				Message:  fmt.Sprintf("Container missing required field: %s", field),
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityHigh,
				Field:    fieldPrefix + "." + field,
			})
		}
	}

	// Validate image
	if image, exists := container["image"]; exists {
		if imageStr, ok := image.(string); ok {
			k.validateContainerImage(imageStr, fieldPrefix+".image", result)
		}
	}
}

// validateContainerImage validates container image reference
func (k *KubernetesValidator) validateContainerImage(image, field string, result *core.ValidationResult) {
	if image == "" {
		result.AddFieldError(field, "Container image cannot be empty")
		return
	}

	// Check for latest tag warning
	if strings.HasSuffix(image, ":latest") || !strings.Contains(image, ":") {
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "LATEST_IMAGE_TAG",
				Message:  "Using 'latest' tag or no tag is not recommended for production",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityLow,
				Field:    field,
			},
		})
	}
}

// validateSelector validates label selector
func (k *KubernetesValidator) validateSelector(selector map[string]interface{}, fieldPrefix string, result *core.ValidationResult) {
	if matchLabels, exists := selector["matchLabels"]; exists {
		if labelsMap, ok := matchLabels.(map[string]interface{}); ok {
			k.validateLabels(labelsMap, fieldPrefix+".matchLabels", result)
		}
	}
}

// validateServicePort validates service port configuration
func (k *KubernetesValidator) validateServicePort(port map[string]interface{}, fieldPrefix string, result *core.ValidationResult) {
	// Validate port number
	if portNum, exists := port["port"]; exists {
		if portInt, ok := portNum.(int); ok {
			if portInt < 1 || portInt > 65535 {
				result.AddFieldError(fieldPrefix+".port", "Port must be between 1 and 65535")
			}
		}
	} else {
		result.AddFieldError(fieldPrefix+".port", "Service port must specify port number")
	}

	// Validate protocol
	if protocol, exists := port["protocol"]; exists {
		if protocolStr, ok := protocol.(string); ok {
			validProtocols := []string{"TCP", "UDP", "SCTP"}
			found := false
			for _, validProtocol := range validProtocols {
				if protocolStr == validProtocol {
					found = true
					break
				}
			}
			if !found {
				result.AddFieldError(fieldPrefix+".protocol", fmt.Sprintf("Invalid protocol: %s", protocolStr))
			}
		}
	}
}

// performSecurityValidation performs security validation
func (k *KubernetesValidator) performSecurityValidation(manifest map[string]interface{}, fieldPrefix string, result *core.ValidationResult) {
	// Check for privileged containers
	if kind, ok := manifest["kind"].(string); ok && strings.ToLower(kind) == "pod" {
		if spec, ok := manifest["spec"].(map[string]interface{}); ok {
			if containers, ok := spec["containers"].([]interface{}); ok {
				for i, container := range containers {
					if containerMap, ok := container.(map[string]interface{}); ok {
						k.validateContainerSecurity(containerMap, fmt.Sprintf("%s.containers[%d]", fieldPrefix, i), result)
					}
				}
			}
		}
	}
}

// validateContainerSecurity validates container security context
func (k *KubernetesValidator) validateContainerSecurity(container map[string]interface{}, fieldPrefix string, result *core.ValidationResult) {
	if securityContext, exists := container["securityContext"]; exists {
		if secCtx, ok := securityContext.(map[string]interface{}); ok {
			// Check for privileged
			if privileged, exists := secCtx["privileged"]; exists {
				if privBool, ok := privileged.(bool); ok && privBool {
					result.AddWarning(&core.ValidationWarning{
						ValidationError: &core.ValidationError{
							Code:     "PRIVILEGED_CONTAINER",
							Message:  "Container is configured to run in privileged mode",
							Type:     core.ErrTypeSecurity,
							Severity: core.SeverityHigh,
							Field:    fieldPrefix + ".securityContext.privileged",
						},
					})
				}
			}

			// Check for runAsRoot
			if runAsUser, exists := secCtx["runAsUser"]; exists {
				if userID, ok := runAsUser.(int); ok && userID == 0 {
					result.AddWarning(&core.ValidationWarning{
						ValidationError: &core.ValidationError{
							Code:     "RUN_AS_ROOT",
							Message:  "Container is configured to run as root user",
							Type:     core.ErrTypeSecurity,
							Severity: core.SeverityMedium,
							Field:    fieldPrefix + ".securityContext.runAsUser",
						},
					})
				}
			}
		}
	}
}

// validateResourceSpecific performs resource-specific validation
func (k *KubernetesValidator) validateResourceSpecific(manifest map[string]interface{}, fieldPrefix string, result *core.ValidationResult) {
	if kind, ok := manifest["kind"].(string); ok {
		switch strings.ToLower(kind) {
		case "configmap":
			k.validateConfigMapData(manifest, fieldPrefix, result)
		case "secret":
			k.validateSecretData(manifest, fieldPrefix, result)
		}
	}
}

// validateConfigMapData validates ConfigMap data
func (k *KubernetesValidator) validateConfigMapData(manifest map[string]interface{}, fieldPrefix string, result *core.ValidationResult) {
	hasData := false
	if _, exists := manifest["data"]; exists {
		hasData = true
	}
	if _, exists := manifest["binaryData"]; exists {
		hasData = true
	}

	if !hasData {
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "CONFIGMAP_NO_DATA",
				Message:  "ConfigMap should have data or binaryData",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityMedium,
				Field:    fieldPrefix + ".data",
			},
		})
	}
}

// validateSecretData validates Secret data
func (k *KubernetesValidator) validateSecretData(manifest map[string]interface{}, fieldPrefix string, result *core.ValidationResult) {
	if _, hasData := manifest["data"]; !hasData {
		if _, hasStringData := manifest["stringData"]; !hasStringData {
			result.AddWarning(&core.ValidationWarning{
				ValidationError: &core.ValidationError{
					Code:     "SECRET_NO_DATA",
					Message:  "Secret should have data or stringData",
					Type:     core.ErrTypeValidation,
					Severity: core.SeverityMedium,
					Field:    fieldPrefix + ".data",
				},
			})
		}
	}
}

// getDefaultK8sVersions returns default Kubernetes API versions and their supported kinds
func getDefaultK8sVersions() map[string][]string {
	return map[string][]string{
		"v1": {
			"Pod", "Service", "ConfigMap", "Secret", "PersistentVolume",
			"PersistentVolumeClaim", "Namespace", "ServiceAccount", "Endpoints",
		},
		"apps/v1": {
			"Deployment", "StatefulSet", "DaemonSet", "ReplicaSet",
		},
		"networking.k8s.io/v1": {
			"Ingress", "NetworkPolicy",
		},
		"rbac.authorization.k8s.io/v1": {
			"Role", "RoleBinding", "ClusterRole", "ClusterRoleBinding",
		},
		"batch/v1": {
			"Job",
		},
		"batch/v1beta1": {
			"CronJob",
		},
		"autoscaling/v1": {
			"HorizontalPodAutoscaler",
		},
		"autoscaling/v2": {
			"HorizontalPodAutoscaler",
		},
		"policy/v1": {
			"PodDisruptionBudget",
		},
	}
}
