package validation

import (
	"context"
	"fmt"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"strings"
)

// KubernetesManifestValidator validates Kubernetes manifests
type KubernetesManifestValidator struct {
	name string
}

func NewKubernetesManifestValidator() *KubernetesManifestValidator {
	return &KubernetesManifestValidator{
		name: "KubernetesManifestValidator",
	}
}

func (v *KubernetesManifestValidator) Validate(ctx context.Context, value interface{}) ValidationResult {
	manifest, ok := value.(map[string]interface{})
	if !ok {
		return ValidationResult{
			Valid:  false,
			Errors: []error{errors.NewValidationFailed("input", "expected map[string]interface{} for Kubernetes manifest")},
		}
	}
	result := ValidationResult{Valid: true, Errors: make([]error, 0)}

	// Check required fields
	if apiVersion, ok := manifest["apiVersion"]; !ok || apiVersion == "" {
		result.Valid = false
		result.Errors = append(result.Errors, errors.NewValidationFailed(
			"apiVersion", "required field missing",
		))
	}

	if kind, ok := manifest["kind"]; !ok || kind == "" {
		result.Valid = false
		result.Errors = append(result.Errors, errors.NewValidationFailed(
			"kind", "required field missing",
		))
	}

	if metadata, ok := manifest["metadata"]; !ok {
		result.Valid = false
		result.Errors = append(result.Errors, errors.NewValidationFailed(
			"metadata", "required field missing",
		))
	} else if metadataMap, ok := metadata.(map[string]interface{}); ok {
		if name, ok := metadataMap["name"]; !ok || name == "" {
			result.Valid = false
			result.Errors = append(result.Errors, errors.NewValidationFailed(
				"metadata.name", "required field missing",
			))
		}
	}

	return result
}

func (v *KubernetesManifestValidator) Name() string {
	return v.name
}

func (v *KubernetesManifestValidator) Domain() string {
	return "kubernetes"
}

func (v *KubernetesManifestValidator) Category() string {
	return "manifest"
}

func (v *KubernetesManifestValidator) Priority() int {
	return 100 // High priority - basic structure validation
}

func (v *KubernetesManifestValidator) Dependencies() []string {
	return []string{} // No dependencies
}

// DockerConfigValidator validates Docker configurations
type DockerConfigValidator struct {
	name string
}

func NewDockerConfigValidator() *DockerConfigValidator {
	return &DockerConfigValidator{
		name: "DockerConfigValidator",
	}
}

func (v *DockerConfigValidator) Validate(ctx context.Context, value interface{}) ValidationResult {
	config, ok := value.(map[string]interface{})
	if !ok {
		return ValidationResult{
			Valid:  false,
			Errors: []error{errors.NewValidationFailed("input", "expected map[string]interface{} for Docker config")},
		}
	}
	result := ValidationResult{Valid: true, Errors: make([]error, 0)}

	// Validate image name if present
	if image, ok := config["image"]; ok {
		if imageStr, ok := image.(string); ok {
			if err := v.validateImageName(imageStr); err != nil {
				result.Valid = false
				result.Errors = append(result.Errors, err)
			}
		}
	}

	// Validate ports if present
	if ports, ok := config["ports"]; ok {
		if portsSlice, ok := ports.([]interface{}); ok {
			for i, port := range portsSlice {
				if portStr, ok := port.(string); ok {
					if err := v.validatePortMapping(portStr, fmt.Sprintf("ports[%d]", i)); err != nil {
						result.Valid = false
						result.Errors = append(result.Errors, err)
					}
				}
			}
		}
	}

	// Validate environment variables if present
	if env, ok := config["environment"]; ok {
		if envMap, ok := env.(map[string]interface{}); ok {
			for key, value := range envMap {
				if err := v.validateEnvVar(key, value); err != nil {
					result.Valid = false
					result.Errors = append(result.Errors, err)
				}
			}
		}
	}

	return result
}

func (v *DockerConfigValidator) validateImageName(image string) error {
	if image == "" {
		return errors.NewValidationFailed("image", "cannot be empty")
	}

	// Basic image name validation
	if strings.Contains(image, "..") {
		return errors.NewValidationFailed("image", "cannot contain '..'")
	}

	return nil
}

func (v *DockerConfigValidator) validatePortMapping(port string, field string) error {
	if port == "" {
		return errors.NewValidationFailed(field, "port mapping cannot be empty")
	}

	// Basic port mapping validation (host:container or just container)
	parts := strings.Split(port, ":")
	if len(parts) > 2 {
		return errors.NewValidationFailed(field, "invalid port mapping format")
	}

	return nil
}

func (v *DockerConfigValidator) validateEnvVar(key string, value interface{}) error {
	if key == "" {
		return errors.NewValidationFailed("environment", "environment variable key cannot be empty")
	}

	if strings.Contains(key, "=") {
		return errors.NewValidationFailed("environment", fmt.Sprintf("environment variable key '%s' cannot contain '='", key))
	}

	return nil
}

func (v *DockerConfigValidator) Name() string {
	return v.name
}

func (v *DockerConfigValidator) Domain() string {
	return "docker"
}

func (v *DockerConfigValidator) Category() string {
	return "config"
}

func (v *DockerConfigValidator) Priority() int {
	return 90 // High priority
}

func (v *DockerConfigValidator) Dependencies() []string {
	return []string{} // No dependencies
}

// SecurityPolicyValidator validates security policies
type SecurityPolicyValidator struct {
	name string
}

func NewSecurityPolicyValidator() *SecurityPolicyValidator {
	return &SecurityPolicyValidator{
		name: "SecurityPolicyValidator",
	}
}

func (v *SecurityPolicyValidator) Validate(ctx context.Context, value interface{}) ValidationResult {
	policy, ok := value.(map[string]interface{})
	if !ok {
		return ValidationResult{
			Valid:  false,
			Errors: []error{errors.NewValidationFailed("input", "expected map[string]interface{} for security policy")},
		}
	}
	result := ValidationResult{Valid: true, Errors: make([]error, 0)}

	// Check for security context
	if securityContext, ok := policy["securityContext"]; ok {
		if secMap, ok := securityContext.(map[string]interface{}); ok {
			// Validate runAsNonRoot
			if runAsNonRoot, ok := secMap["runAsNonRoot"]; ok {
				if runAsRoot, ok := runAsNonRoot.(bool); ok && !runAsRoot {
					result.Warnings = append(result.Warnings, "Security warning: container may run as root")
				}
			}

			// Validate readOnlyRootFilesystem
			if readOnly, ok := secMap["readOnlyRootFilesystem"]; ok {
				if isReadOnly, ok := readOnly.(bool); ok && !isReadOnly {
					result.Warnings = append(result.Warnings, "Security warning: root filesystem is writable")
				}
			}
		}
	}

	// Check for privileged containers
	if privileged, ok := policy["privileged"]; ok {
		if isPrivileged, ok := privileged.(bool); ok && isPrivileged {
			result.Valid = false
			result.Errors = append(result.Errors, errors.NewSecurityError(
				"privileged containers not allowed",
				map[string]interface{}{"policy": "privileged_containers"},
			))
		}
	}

	// Check for host network
	if hostNetwork, ok := policy["hostNetwork"]; ok {
		if useHostNetwork, ok := hostNetwork.(bool); ok && useHostNetwork {
			result.Warnings = append(result.Warnings, "Security warning: using host network")
		}
	}

	return result
}

func (v *SecurityPolicyValidator) Name() string {
	return v.name
}

func (v *SecurityPolicyValidator) Domain() string {
	return "security"
}

func (v *SecurityPolicyValidator) Category() string {
	return "policy"
}

func (v *SecurityPolicyValidator) Priority() int {
	return 200 // Highest priority - security is critical
}

func (v *SecurityPolicyValidator) Dependencies() []string {
	return []string{"KubernetesManifestValidator"} // Depends on basic manifest validation
}
