package validators

import (
	"fmt"
	"strings"

	"github.com/Azure/container-kit/pkg/common/validation-core/core"
)

// Resource-Specific Validation
// This file contains functions for validating specific Kubernetes resource types

// validateSpec validates spec section based on resource kind
func (k *KubernetesValidator) validateSpec(kind string, spec map[string]interface{}, fieldPrefix string, result *core.NonGenericResult) {
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
func (k *KubernetesValidator) validateDeploymentSpec(spec map[string]interface{}, fieldPrefix string, result *core.NonGenericResult) {
	// Required fields
	requiredFields := []string{"selector", "template"}
	for _, field := range requiredFields {
		if _, exists := spec[field]; !exists {
			result.AddError(&core.Error{
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
func (k *KubernetesValidator) validateServiceSpec(spec map[string]interface{}, fieldPrefix string, result *core.NonGenericResult) {
	// Validate ports
	if ports, exists := spec["ports"]; exists {
		if portsList, ok := ports.([]interface{}); ok {
			if len(portsList) == 0 {
				result.AddWarning(&core.Warning{
					Error: &core.Error{
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
func (k *KubernetesValidator) validateIngressSpec(spec map[string]interface{}, fieldPrefix string, result *core.NonGenericResult) {
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
		result.AddWarning(&core.Warning{
			Error: &core.Error{
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
func (k *KubernetesValidator) validatePodSpec(spec map[string]interface{}, fieldPrefix string, result *core.NonGenericResult) {
	// Validate containers
	if containers, exists := spec["containers"]; exists {
		if containersList, ok := containers.([]interface{}); ok {
			if len(containersList) == 0 {
				result.AddError(&core.Error{
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
		result.AddError(&core.Error{
			Code:     "MISSING_CONTAINERS",
			Message:  "Pod spec missing containers field",
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityCritical,
			Field:    fieldPrefix + ".containers",
		})
	}
}

// validateContainer validates container specification
func (k *KubernetesValidator) validateContainer(container map[string]interface{}, fieldPrefix string, result *core.NonGenericResult) {
	// Required fields
	requiredFields := []string{"name", "image"}
	for _, field := range requiredFields {
		if _, exists := container[field]; !exists {
			result.AddError(&core.Error{
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
func (k *KubernetesValidator) validateContainerImage(image, field string, result *core.NonGenericResult) {
	if image == "" {
		result.AddFieldError(field, "Container image cannot be empty")
		return
	}

	// Check for latest tag warning
	if strings.HasSuffix(image, ":latest") || !strings.Contains(image, ":") {
		result.AddWarning(&core.Warning{
			Error: &core.Error{
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
func (k *KubernetesValidator) validateSelector(selector map[string]interface{}, fieldPrefix string, result *core.NonGenericResult) {
	if matchLabels, exists := selector["matchLabels"]; exists {
		if labelsMap, ok := matchLabels.(map[string]interface{}); ok {
			k.validateLabels(labelsMap, fieldPrefix+".matchLabels", result)
		}
	}
}

// validateServicePort validates service port configuration
func (k *KubernetesValidator) validateServicePort(port map[string]interface{}, fieldPrefix string, result *core.NonGenericResult) {
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

// validateResourceSpecific validates resource-specific data
func (k *KubernetesValidator) validateResourceSpecific(manifest map[string]interface{}, fieldPrefix string, result *core.NonGenericResult) {
	if kind, ok := manifest["kind"].(string); ok {
		switch strings.ToLower(kind) {
		case "configmap":
			k.validateConfigMapData(manifest, fieldPrefix, result)
		case "secret":
			k.validateSecretData(manifest, fieldPrefix, result)
		}
	}
}

// validateConfigMapData validates ConfigMap data section
func (k *KubernetesValidator) validateConfigMapData(manifest map[string]interface{}, fieldPrefix string, result *core.NonGenericResult) {
	hasData := false
	hasBinaryData := false

	if data, exists := manifest["data"]; exists && data != nil {
		hasData = true
	}
	if binaryData, exists := manifest["binaryData"]; exists && binaryData != nil {
		hasBinaryData = true
	}

	if !hasData && !hasBinaryData {
		result.AddWarning(&core.Warning{
			Error: &core.Error{
				Code:     "CONFIGMAP_NO_DATA",
				Message:  "ConfigMap should have either data or binaryData",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityLow,
				Field:    fieldPrefix + ".data",
			},
		})
	}
}

// validateSecretData validates Secret data section
func (k *KubernetesValidator) validateSecretData(manifest map[string]interface{}, fieldPrefix string, result *core.NonGenericResult) {
	hasData := false
	hasStringData := false

	if data, exists := manifest["data"]; exists && data != nil {
		hasData = true
	}
	if stringData, exists := manifest["stringData"]; exists && stringData != nil {
		hasStringData = true
	}

	if !hasData && !hasStringData {
		result.AddWarning(&core.Warning{
			Error: &core.Error{
				Code:     "SECRET_NO_DATA",
				Message:  "Secret should have either data or stringData",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityLow,
				Field:    fieldPrefix + ".data",
			},
		})
	}
}

// validateSpecTyped validates typed spec structure based on resource kind
func (k *KubernetesValidator) validateSpecTyped(kind string, spec interface{}, fieldPrefix string, result *core.NonGenericResult) {
	// For now, fall back to untyped validation until full typed spec implementations are available
	if specMap, ok := spec.(map[string]interface{}); ok {
		k.validateSpec(kind, specMap, fieldPrefix, result)
	}
}
