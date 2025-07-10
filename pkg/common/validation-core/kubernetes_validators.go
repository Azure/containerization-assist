package validation

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	errorcodes "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// KubernetesValidators consolidates all Kubernetes/Deployment validation logic
// Replaces: kubernetes_validator.go, kubernetes_metadata_validator.go, kubernetes_resource_validator.go,
//
//	kubernetes_yaml_validator.go, kubernetes_security_validator.go, deploy/core_validator.go,
//	deploy/deploy_kubernetes_validator.go, deploy/manifest_validator*.go, deploy/health_validator.go
type KubernetesValidators struct{}

// NewKubernetesValidators creates a new Kubernetes validator
func NewKubernetesValidators() *KubernetesValidators {
	return &KubernetesValidators{}
}

// ValidateResourceName validates Kubernetes resource names (RFC 1123 compliant)
func (kv *KubernetesValidators) ValidateResourceName(name string) error {
	if name == "" {
		return errors.NewError().
			Code(errorcodes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("Kubernetes resource name cannot be empty").
			Build()
	}

	if len(name) > 253 {
		return errors.NewError().
			Code(errorcodes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("Kubernetes resource name too long (max 253 chars): %s", name).
			Build()
	}

	// RFC 1123 compliant DNS subdomain names
	validName := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)
	if !validName.MatchString(name) {
		return errors.NewError().
			Code(errorcodes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("invalid Kubernetes resource name format: %s", name).
			Build()
	}

	return nil
}

// ValidateNamespace validates Kubernetes namespace names
func (kv *KubernetesValidators) ValidateNamespace(namespace string) error {
	if namespace == "" {
		namespace = "default" // Allow empty namespace (defaults to "default")
		return nil
	}

	if len(namespace) > 63 {
		return errors.NewError().
			Code(errorcodes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("namespace name too long (max 63 chars): %s", namespace).
			Build()
	}

	// Kubernetes namespace validation
	validNamespace := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	if !validNamespace.MatchString(namespace) {
		return errors.NewError().
			Code(errorcodes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("invalid namespace format: %s", namespace).
			Build()
	}

	// Reserved namespaces
	reserved := []string{"kube-system", "kube-public", "kube-node-lease"}
	for _, res := range reserved {
		if namespace == res {
			return errors.NewError().
				Code(errorcodes.VALIDATION_FAILED).
				Type(errors.ErrTypeValidation).
				Messagef("namespace name is reserved: %s", namespace).
				Build()
		}
	}

	return nil
}

// ValidateLabels validates Kubernetes label key-value pairs
func (kv *KubernetesValidators) ValidateLabels(labels map[string]string) error {
	for key, value := range labels {
		if err := kv.ValidateLabelKey(key); err != nil {
			return err
		}
		if err := kv.ValidateLabelValue(value); err != nil {
			return err
		}
	}
	return nil
}

// ValidateLabelKey validates Kubernetes label keys
func (kv *KubernetesValidators) ValidateLabelKey(key string) error {
	if key == "" {
		return errors.NewError().
			Code(errorcodes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("label key cannot be empty").
			Build()
	}

	if len(key) > 253 {
		return errors.NewError().
			Code(errorcodes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("label key too long (max 253 chars): %s", key).
			Build()
	}

	// Label key format validation
	parts := strings.Split(key, "/")
	if len(parts) > 2 {
		return errors.NewError().
			Code(errorcodes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("invalid label key format (too many slashes): %s", key).
			Build()
	}

	return nil
}

// ValidateLabelValue validates Kubernetes label values
func (kv *KubernetesValidators) ValidateLabelValue(value string) error {
	if len(value) > 63 {
		return errors.NewError().
			Code(errorcodes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("label value too long (max 63 chars): %s", value).
			Build()
	}

	if value != "" {
		validValue := regexp.MustCompile(`^[a-zA-Z0-9]([-a-zA-Z0-9_.]*[a-zA-Z0-9])?$`)
		if !validValue.MatchString(value) {
			return errors.NewError().
				Code(errorcodes.VALIDATION_FAILED).
				Type(errors.ErrTypeValidation).
				Messagef("invalid label value format: %s", value).
				Build()
		}
	}

	return nil
}

// ValidateYAMLManifest validates Kubernetes YAML manifests
func (kv *KubernetesValidators) ValidateYAMLManifest(yamlContent string) error {
	if yamlContent == "" {
		return errors.NewError().
			Code(errorcodes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("YAML manifest content cannot be empty").
			Build()
	}

	// Check for required fields
	requiredFields := []string{"apiVersion", "kind", "metadata"}
	for _, field := range requiredFields {
		if !strings.Contains(yamlContent, field+":") {
			return errors.NewError().
				Code(errorcodes.VALIDATION_FAILED).
				Type(errors.ErrTypeValidation).
				Messagef("YAML manifest missing required field: %s", field).
				Build()
		}
	}

	return nil
}

// ValidateDeployment validates Kubernetes Deployment specifications
func (kv *KubernetesValidators) ValidateDeployment(deployment map[string]interface{}) error {
	// Validate deployment-specific requirements
	spec, ok := deployment["spec"].(map[string]interface{})
	if !ok {
		return errors.NewError().
			Code(errorcodes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("deployment spec is required").
			Build()
	}

	// Check replica count
	if replicas, exists := spec["replicas"]; exists {
		if r, ok := replicas.(float64); ok && r < 0 {
			return errors.NewError().
				Code(errorcodes.VALIDATION_FAILED).
				Type(errors.ErrTypeValidation).
				Messagef("deployment replicas cannot be negative: %v", r).
				Build()
		}
	}

	return nil
}

// ValidateService validates Kubernetes Service specifications
func (kv *KubernetesValidators) ValidateService(service map[string]interface{}) error {
	spec, ok := service["spec"].(map[string]interface{})
	if !ok {
		return errors.NewError().
			Code(errorcodes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("service spec is required").
			Build()
	}

	// Validate service type
	if serviceType, exists := spec["type"]; exists {
		validTypes := []string{"ClusterIP", "NodePort", "LoadBalancer", "ExternalName"}
		typeStr := fmt.Sprintf("%v", serviceType)
		valid := false
		for _, validType := range validTypes {
			if typeStr == validType {
				valid = true
				break
			}
		}
		if !valid {
			return errors.NewError().
				Code(errorcodes.VALIDATION_FAILED).
				Type(errors.ErrTypeValidation).
				Messagef("invalid service type: %s", typeStr).
				Build()
		}
	}

	return nil
}

// ValidateHealthCheck validates Kubernetes health check configurations
func (kv *KubernetesValidators) ValidateHealthCheck(healthCheck map[string]interface{}) error {
	// Validate probe configurations
	if httpGet, exists := healthCheck["httpGet"]; exists {
		if httpMap, ok := httpGet.(map[string]interface{}); ok {
			if port, exists := httpMap["port"]; exists {
				if p, ok := port.(float64); ok && (p < 1 || p > 65535) {
					return errors.NewError().
						Code(errorcodes.VALIDATION_FAILED).
						Type(errors.ErrTypeValidation).
						Messagef("invalid health check port: %v", p).
						Build()
				}
			}
		}
	}

	return nil
}
