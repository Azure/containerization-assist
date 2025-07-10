package validation

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// KubernetesValidatorOptions represents configuration options for the Kubernetes manifest validator
type KubernetesValidatorOptions struct {
	ValidateSecurity    bool
	ValidateResources   bool
	StrictMode          bool
	AllowedNamespaces   []string
	ForbiddenNamespaces []string
}

// KubernetesManifestValidator validates Kubernetes manifests
type KubernetesManifestValidator struct {
	name                string
	validateSecurity    bool
	validateResources   bool
	strictMode          bool
	allowedNamespaces   []string
	forbiddenNamespaces []string
}

func NewKubernetesManifestValidator() *KubernetesManifestValidator {
	return &KubernetesManifestValidator{
		name:                "KubernetesManifestValidator",
		validateSecurity:    true,
		validateResources:   true,
		strictMode:          false,
		forbiddenNamespaces: []string{"kube-system", "kube-public", "kube-node-lease"},
	}
}

// NewKubernetesManifestValidatorWithOptions creates a new Kubernetes manifest validator with custom options
func NewKubernetesManifestValidatorWithOptions(options KubernetesValidatorOptions) *KubernetesManifestValidator {
	validator := &KubernetesManifestValidator{
		name:                "KubernetesManifestValidator",
		validateSecurity:    options.ValidateSecurity,
		validateResources:   options.ValidateResources,
		strictMode:          options.StrictMode,
		forbiddenNamespaces: options.ForbiddenNamespaces,
		allowedNamespaces:   options.AllowedNamespaces,
	}

	// Set default forbidden namespaces if not provided
	if len(validator.forbiddenNamespaces) == 0 {
		validator.forbiddenNamespaces = []string{"kube-system", "kube-public", "kube-node-lease"}
	}

	return validator
}

func (v *KubernetesManifestValidator) Validate(_ context.Context, value interface{}) ValidationResult {
	manifest, ok := value.(map[string]interface{})
	if !ok {
		return ValidationResult{
			Valid:  false,
			Errors: []error{errors.NewValidationFailed("input", "expected map[string]interface{} for Kubernetes manifest")},
		}
	}

	var allErrors []error
	var allWarnings []string

	// Basic structure validation
	if errs, warnings := v.validateBasicStructure(manifest); len(errs) > 0 {
		allErrors = append(allErrors, errs...)
	} else {
		allWarnings = append(allWarnings, warnings...)
	}

	// Metadata validation
	if errs, warnings := v.validateMetadata(manifest); len(errs) > 0 {
		allErrors = append(allErrors, errs...)
	} else {
		allWarnings = append(allWarnings, warnings...)
	}

	// API version and kind validation
	if errs, warnings := v.validateAPIVersionAndKind(manifest); len(errs) > 0 {
		allErrors = append(allErrors, errs...)
	} else {
		allWarnings = append(allWarnings, warnings...)
	}

	// Resource-specific validation (only if basic validation passed)
	if v.validateResources && len(allErrors) == 0 {
		if errs, warnings := v.validateResourceSpecific(manifest); len(errs) > 0 {
			allErrors = append(allErrors, errs...)
		} else {
			allWarnings = append(allWarnings, warnings...)
		}
	}

	// Security validation (only if basic validation passed)
	if v.validateSecurity && len(allErrors) == 0 {
		if errs, warnings := v.validateSecurityContextInternal(manifest); len(errs) > 0 {
			allErrors = append(allErrors, errs...)
		} else {
			allWarnings = append(allWarnings, warnings...)
		}
	}

	return ValidationResult{
		Valid:    len(allErrors) == 0,
		Errors:   allErrors,
		Warnings: allWarnings,
	}
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

func (v *DockerConfigValidator) Validate(_ context.Context, value interface{}) ValidationResult {
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

func (v *SecurityPolicyValidator) Validate(_ context.Context, value interface{}) ValidationResult {
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

// Add validation helper methods for KubernetesManifestValidator
func (v *KubernetesManifestValidator) validateBasicStructure(manifest map[string]interface{}) ([]error, []string) {
	var errs []error
	var warnings []string

	// Check required fields
	if _, ok := manifest["apiVersion"]; !ok {
		errs = append(errs, errors.NewValidationFailed("apiVersion", "apiVersion is required"))
	}

	if _, ok := manifest["kind"]; !ok {
		errs = append(errs, errors.NewValidationFailed("kind", "kind is required"))
	}

	if _, ok := manifest["metadata"]; !ok {
		errs = append(errs, errors.NewValidationFailed("metadata", "metadata is required"))
	}

	return errs, warnings
}

func (v *KubernetesManifestValidator) validateMetadata(manifest map[string]interface{}) ([]error, []string) {
	var errs []error
	var warnings []string

	metadata, ok := manifest["metadata"].(map[string]interface{})
	if !ok {
		errs = append(errs, errors.NewValidationFailed("metadata", "metadata must be an object"))
		return errs, warnings
	}

	// Validate name
	if name, ok := metadata["name"].(string); ok {
		if name == "" {
			errs = append(errs, errors.NewValidationFailed("metadata.name", "name cannot be empty"))
		} else if err := v.validateResourceName(name); err != nil {
			errs = append(errs, errors.NewValidationFailed("metadata.name", err.Error()))
		}
	} else {
		errs = append(errs, errors.NewValidationFailed("metadata.name", "name is required"))
	}

	// Validate namespace
	if namespace, ok := metadata["namespace"].(string); ok {
		if err := v.validateNamespace(namespace); err != nil {
			errs = append(errs, errors.NewValidationFailed("metadata.namespace", err.Error()))
		}
	}

	// Validate labels
	if labels, ok := metadata["labels"].(map[string]interface{}); ok {
		for key, value := range labels {
			if err := v.validateLabelKey(key); err != nil {
				errs = append(errs, errors.NewValidationFailed(fmt.Sprintf("metadata.labels[%s]", key), err.Error()))
			}
			if valueStr, ok := value.(string); ok {
				if err := v.validateLabelValue(valueStr); err != nil {
					errs = append(errs, errors.NewValidationFailed(fmt.Sprintf("metadata.labels[%s]", key), err.Error()))
				}
			}
		}
	}

	return errs, warnings
}

func (v *KubernetesManifestValidator) validateAPIVersionAndKind(manifest map[string]interface{}) ([]error, []string) {
	var errs []error
	var warnings []string

	apiVersion, _ := manifest["apiVersion"].(string)
	kind, _ := manifest["kind"].(string)

	if apiVersion == "" {
		errs = append(errs, errors.NewValidationFailed("apiVersion", "apiVersion cannot be empty"))
	}

	if kind == "" {
		errs = append(errs, errors.NewValidationFailed("kind", "kind cannot be empty"))
	}

	// Validate API version/kind combinations
	// Check for known invalid combinations even in non-strict mode
	if apiVersion != "" && kind != "" {
		if v.isKnownInvalidCombination(apiVersion, kind) {
			errs = append(errs, errors.NewValidationFailed("apiVersion/kind", fmt.Sprintf("invalid combination: %s/%s", apiVersion, kind)))
		} else if v.strictMode && !v.isValidAPIVersionKindCombination(apiVersion, kind) {
			// In strict mode, only allow known valid combinations
			errs = append(errs, errors.NewValidationFailed("apiVersion/kind", fmt.Sprintf("unknown combination: %s/%s", apiVersion, kind)))
		}
	}

	return errs, warnings
}

func (v *KubernetesManifestValidator) validateResourceSpecific(manifest map[string]interface{}) ([]error, []string) {
	var errs []error
	var warnings []string

	kind, _ := manifest["kind"].(string)

	switch kind {
	case "Pod":
		if podErrs, podWarnings := v.validatePodSpec(manifest); len(podErrs) > 0 {
			errs = append(errs, podErrs...)
		} else {
			warnings = append(warnings, podWarnings...)
		}
	case "Deployment":
		if deployErrs, deployWarnings := v.validateDeploymentSpec(manifest); len(deployErrs) > 0 {
			errs = append(errs, deployErrs...)
		} else {
			warnings = append(warnings, deployWarnings...)
		}
	case "Service":
		if serviceErrs, serviceWarnings := v.validateServiceSpec(manifest); len(serviceErrs) > 0 {
			errs = append(errs, serviceErrs...)
		} else {
			warnings = append(warnings, serviceWarnings...)
		}
	}

	return errs, warnings
}

func (v *KubernetesManifestValidator) validateSecurityContextInternal(manifest map[string]interface{}) ([]error, []string) {
	var errs []error
	var warnings []string

	kind, _ := manifest["kind"].(string)

	// Security validation is mainly applicable to Pod, Deployment, DaemonSet, StatefulSet
	if !v.hasSecurityContext(kind) {
		return errs, warnings
	}

	spec, hasSpec := manifest["spec"]
	if !hasSpec {
		return errs, warnings
	}

	specMap, ok := spec.(map[string]interface{})
	if !ok {
		return errs, warnings
	}

	// For Deployment, DaemonSet, StatefulSet, check template.spec
	if kind == "Deployment" || kind == "DaemonSet" || kind == "StatefulSet" {
		if template, ok := specMap["template"].(map[string]interface{}); ok {
			if templateSpec, ok := template["spec"].(map[string]interface{}); ok {
				if secErrs, secWarnings := v.validatePodSecurityContext(templateSpec); len(secErrs) > 0 {
					errs = append(errs, secErrs...)
				} else {
					warnings = append(warnings, secWarnings...)
				}
			}
		}
	} else if kind == "Pod" {
		// Direct Pod security validation
		if secErrs, secWarnings := v.validatePodSecurityContext(specMap); len(secErrs) > 0 {
			errs = append(errs, secErrs...)
		} else {
			warnings = append(warnings, secWarnings...)
		}
	}

	return errs, warnings
}

func (v *KubernetesManifestValidator) validateResourceName(name string) error {
	// Kubernetes resource names must:
	// - be less than 253 characters
	// - contain only lowercase alphanumeric characters, '-' or '.'
	// - start with an alphanumeric character
	// - end with an alphanumeric character

	if len(name) > 253 {
		return fmt.Errorf("resource name is too long (max 253 characters)")
	}

	if name == "" {
		return fmt.Errorf("resource name cannot be empty")
	}

	// Check if it starts with a number
	if name[0] >= '0' && name[0] <= '9' {
		return fmt.Errorf("resource name cannot start with a number")
	}

	// Check for uppercase letters
	for _, ch := range name {
		if ch >= 'A' && ch <= 'Z' {
			return fmt.Errorf("resource name must be lowercase")
		}
		// Check for valid characters
		if !((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '.') {
			return fmt.Errorf("resource name contains invalid characters")
		}
	}

	// Check if it ends with alphanumeric
	lastChar := name[len(name)-1]
	if !((lastChar >= 'a' && lastChar <= 'z') || (lastChar >= '0' && lastChar <= '9')) {
		return fmt.Errorf("resource name must end with an alphanumeric character")
	}

	return nil
}

func (v *KubernetesManifestValidator) validateLabelKey(key string) error {
	// Label keys have two segments: an optional prefix and name, separated by a slash (/)
	// The name segment is required and must be 63 characters or less
	// The prefix is optional. If specified, must be a DNS subdomain

	parts := strings.Split(key, "/")
	var name string

	if len(parts) == 1 {
		name = parts[0]
	} else if len(parts) == 2 {
		// Has prefix
		prefix := parts[0]
		name = parts[1]

		// Validate prefix (DNS subdomain)
		if len(prefix) > 253 {
			return fmt.Errorf("label key prefix is too long (max 253 characters)")
		}
	} else {
		return fmt.Errorf("label key can have at most one '/'")
	}

	// Validate name segment
	if len(name) > 63 {
		return fmt.Errorf("label key name is too long (max 63 characters)")
	}

	if name == "" {
		return fmt.Errorf("label key name cannot be empty")
	}

	// Must start and end with alphanumeric
	if !((name[0] >= 'a' && name[0] <= 'z') || (name[0] >= 'A' && name[0] <= 'Z') || (name[0] >= '0' && name[0] <= '9')) {
		return fmt.Errorf("label key name must start with alphanumeric character")
	}

	lastChar := name[len(name)-1]
	if !((lastChar >= 'a' && lastChar <= 'z') || (lastChar >= 'A' && lastChar <= 'Z') || (lastChar >= '0' && lastChar <= '9')) {
		return fmt.Errorf("label key name must end with alphanumeric character")
	}

	// Check valid characters
	for _, ch := range name {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_' || ch == '.') {
			return fmt.Errorf("label key name contains invalid characters")
		}
	}

	return nil
}

func (v *KubernetesManifestValidator) validateLabelValue(value string) error {
	// Label values must be 63 characters or less
	// Can be empty
	// Must begin and end with alphanumeric if not empty
	// Can contain dashes, underscores, dots, and alphanumerics

	if len(value) > 63 {
		return fmt.Errorf("label value is too long (max 63 characters)")
	}

	if value == "" {
		return nil // Empty values are allowed
	}

	// Must start and end with alphanumeric
	if !((value[0] >= 'a' && value[0] <= 'z') || (value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= '0' && value[0] <= '9')) {
		return fmt.Errorf("label value must start with alphanumeric character")
	}

	lastChar := value[len(value)-1]
	if !((lastChar >= 'a' && lastChar <= 'z') || (lastChar >= 'A' && lastChar <= 'Z') || (lastChar >= '0' && lastChar <= '9')) {
		return fmt.Errorf("label value must end with alphanumeric character")
	}

	// Check valid characters
	for _, ch := range value {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_' || ch == '.') {
			return fmt.Errorf("label value contains invalid characters")
		}
	}

	return nil
}

func (v *KubernetesManifestValidator) validateNamespace(namespace string) error {
	// Check forbidden namespaces
	for _, forbidden := range v.forbiddenNamespaces {
		if namespace == forbidden {
			return fmt.Errorf("namespace '%s' is reserved and cannot be used", namespace)
		}
	}

	// Check allowed namespaces if specified
	if len(v.allowedNamespaces) > 0 {
		found := false
		for _, allowed := range v.allowedNamespaces {
			if namespace == allowed {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("namespace '%s' is not in the allowed list", namespace)
		}
	}

	return nil
}

func (v *KubernetesManifestValidator) isKnownInvalidCombination(apiVersion, kind string) bool {
	// Check for resources that are explicitly in the wrong API version
	invalidCombinations := map[string][]string{
		"v1": {"Deployment", "DaemonSet", "StatefulSet", "ReplicaSet", "Ingress", "NetworkPolicy"},
		// Add more known invalid combinations as needed
	}

	if kinds, ok := invalidCombinations[apiVersion]; ok {
		for _, invalidKind := range kinds {
			if kind == invalidKind {
				return true
			}
		}
	}

	return false
}

func (v *KubernetesManifestValidator) isValidAPIVersionKindCombination(apiVersion, kind string) bool {
	validCombinations := map[string][]string{
		"v1":                           {"Pod", "Service", "ConfigMap", "Secret", "PersistentVolume", "PersistentVolumeClaim", "Namespace", "ServiceAccount"},
		"apps/v1":                      {"Deployment", "DaemonSet", "StatefulSet", "ReplicaSet"},
		"extensions/v1beta1":           {"Deployment", "DaemonSet", "ReplicaSet", "Ingress"},
		"networking.k8s.io/v1":         {"NetworkPolicy", "Ingress"},
		"rbac.authorization.k8s.io/v1": {"Role", "RoleBinding", "ClusterRole", "ClusterRoleBinding"},
	}

	if kinds, ok := validCombinations[apiVersion]; ok {
		for _, validKind := range kinds {
			if kind == validKind {
				return true
			}
		}
	}

	return false
}

func (v *KubernetesManifestValidator) hasSecurityContext(kind string) bool {
	securityContextKinds := map[string]bool{
		"Pod":         true,
		"Deployment":  true,
		"DaemonSet":   true,
		"StatefulSet": true,
		"Job":         true,
		"CronJob":     true,
	}
	return securityContextKinds[kind]
}

func (v *KubernetesManifestValidator) validatePodSecurityContext(podSpec map[string]interface{}) ([]error, []string) {
	var errs []error
	var warnings []string

	// Validate container security contexts
	if containers, ok := podSpec["containers"].([]interface{}); ok {
		for i, container := range containers {
			if containerMap, ok := container.(map[string]interface{}); ok {
				if securityContext, ok := containerMap["securityContext"].(map[string]interface{}); ok {
					// Check for privileged containers
					if privileged, ok := securityContext["privileged"].(bool); ok && privileged {
						errs = append(errs, errors.NewSecurityError(
							"privileged containers are not allowed",
							map[string]interface{}{
								"container": i,
								"policy":    "no_privileged_containers",
							},
						))
					}
				}
			}
		}
	}

	return errs, warnings
}

// validatePodSpec validates Pod specifications
func (v *KubernetesManifestValidator) validatePodSpec(manifest map[string]interface{}) ([]error, []string) {
	var errs []error
	var warnings []string

	spec, ok := manifest["spec"].(map[string]interface{})
	if !ok {
		errs = append(errs, errors.NewValidationFailed("spec", "spec is required for Pod"))
		return errs, warnings
	}

	// Validate containers
	if containers, ok := spec["containers"].([]interface{}); ok {
		if len(containers) == 0 {
			errs = append(errs, errors.NewValidationFailed("spec.containers", "Pod must have at least one container"))
		} else {
			for i, container := range containers {
				if containerErrs, containerWarnings := v.validateContainer(container, fmt.Sprintf("spec.containers[%d]", i)); len(containerErrs) > 0 {
					errs = append(errs, containerErrs...)
				} else {
					warnings = append(warnings, containerWarnings...)
				}
			}
		}
	} else {
		errs = append(errs, errors.NewValidationFailed("spec.containers", "containers field is required"))
	}

	// Validate restart policy
	if restartPolicy, ok := spec["restartPolicy"].(string); ok {
		if !v.isValidRestartPolicy(restartPolicy) {
			errs = append(errs, errors.NewValidationFailed("spec.restartPolicy", fmt.Sprintf("invalid restart policy: %s", restartPolicy)))
		}
	}

	return errs, warnings
}

func (v *KubernetesManifestValidator) validateDeploymentSpec(manifest map[string]interface{}) ([]error, []string) {
	var errs []error
	var warnings []string

	spec, ok := manifest["spec"].(map[string]interface{})
	if !ok {
		errs = append(errs, errors.NewValidationFailed("spec", "spec is required for Deployment"))
		return errs, warnings
	}

	// Validate replicas
	if replicas, ok := spec["replicas"]; ok {
		var replicaCount int
		switch r := replicas.(type) {
		case int:
			replicaCount = r
		case float64:
			replicaCount = int(r)
		default:
			errs = append(errs, errors.NewValidationFailed("spec.replicas", "replicas must be a number"))
		}

		if replicaCount < 0 {
			errs = append(errs, errors.NewValidationFailed("spec.replicas", "replicas cannot be negative"))
		}
	}

	// Validate selector
	if selector, ok := spec["selector"].(map[string]interface{}); ok {
		if matchLabels, ok := selector["matchLabels"].(map[string]interface{}); ok {
			if len(matchLabels) == 0 {
				errs = append(errs, errors.NewValidationFailed("spec.selector.matchLabels", "matchLabels cannot be empty"))
			}
		} else {
			errs = append(errs, errors.NewValidationFailed("spec.selector", "selector must have matchLabels"))
		}
	} else {
		errs = append(errs, errors.NewValidationFailed("spec.selector", "selector is required for Deployment"))
	}

	// Validate template
	if template, ok := spec["template"].(map[string]interface{}); ok {
		if templateSpec, ok := template["spec"].(map[string]interface{}); ok {
			// Validate template containers
			if containers, ok := templateSpec["containers"].([]interface{}); ok {
				if len(containers) == 0 {
					errs = append(errs, errors.NewValidationFailed("spec.template.spec.containers", "template must have at least one container"))
				} else {
					for i, container := range containers {
						if containerErrs, containerWarnings := v.validateContainer(container, fmt.Sprintf("spec.template.spec.containers[%d]", i)); len(containerErrs) > 0 {
							errs = append(errs, containerErrs...)
						} else {
							warnings = append(warnings, containerWarnings...)
						}
					}
				}
			} else {
				errs = append(errs, errors.NewValidationFailed("spec.template.spec.containers", "template containers are required"))
			}
		} else {
			errs = append(errs, errors.NewValidationFailed("spec.template.spec", "template spec is required"))
		}
	} else {
		errs = append(errs, errors.NewValidationFailed("spec.template", "template is required for Deployment"))
	}

	return errs, warnings
}

func (v *KubernetesManifestValidator) validateServiceSpec(manifest map[string]interface{}) ([]error, []string) {
	var errs []error
	var warnings []string

	spec, ok := manifest["spec"].(map[string]interface{})
	if !ok {
		errs = append(errs, errors.NewValidationFailed("spec", "spec is required for Service"))
		return errs, warnings
	}

	// Validate service type
	if serviceType, ok := spec["type"].(string); ok {
		if !v.isValidServiceType(serviceType) {
			errs = append(errs, errors.NewValidationFailed("spec.type", fmt.Sprintf("invalid service type: %s", serviceType)))
		}
	}

	// Validate ports
	if ports, ok := spec["ports"].([]interface{}); ok {
		if len(ports) == 0 {
			errs = append(errs, errors.NewValidationFailed("spec.ports", "Service must have at least one port"))
		} else {
			for i, port := range ports {
				if portErrs, portWarnings := v.validateServicePort(port, fmt.Sprintf("spec.ports[%d]", i)); len(portErrs) > 0 {
					errs = append(errs, portErrs...)
				} else {
					warnings = append(warnings, portWarnings...)
				}
			}
		}
	} else {
		errs = append(errs, errors.NewValidationFailed("spec.ports", "ports field is required for Service"))
	}

	// Validate selector
	if selector, ok := spec["selector"].(map[string]interface{}); ok {
		if len(selector) == 0 {
			warnings = append(warnings, "Service selector is empty - service will not select any pods")
		}
	}

	return errs, warnings
}

// validateContainer validates container specifications
func (v *KubernetesManifestValidator) validateContainer(container interface{}, path string) ([]error, []string) {
	var errs []error
	var warnings []string

	containerMap, ok := container.(map[string]interface{})
	if !ok {
		errs = append(errs, errors.NewValidationFailed(path, "container must be an object"))
		return errs, warnings
	}

	// Validate name
	if name, ok := containerMap["name"].(string); ok {
		if name == "" {
			errs = append(errs, errors.NewValidationFailed(path+".name", "container name cannot be empty"))
		}
	} else {
		errs = append(errs, errors.NewValidationFailed(path+".name", "container name is required"))
	}

	// Validate image
	if image, ok := containerMap["image"].(string); ok {
		if image == "" {
			errs = append(errs, errors.NewValidationFailed(path+".image", "container image cannot be empty"))
		}
	} else {
		errs = append(errs, errors.NewValidationFailed(path+".image", "container image is required"))
	}

	// Validate ports
	if ports, ok := containerMap["ports"].([]interface{}); ok {
		for i, port := range ports {
			if portErrs, portWarnings := v.validateContainerPort(port, fmt.Sprintf("%s.ports[%d]", path, i)); len(portErrs) > 0 {
				errs = append(errs, portErrs...)
			} else {
				warnings = append(warnings, portWarnings...)
			}
		}
	}

	return errs, warnings
}

// validateContainerPort validates container port specifications
func (v *KubernetesManifestValidator) validateContainerPort(port interface{}, path string) ([]error, []string) {
	var errs []error
	var warnings []string

	portMap, ok := port.(map[string]interface{})
	if !ok {
		errs = append(errs, errors.NewValidationFailed(path, "port must be an object"))
		return errs, warnings
	}

	// Validate containerPort
	if containerPort, ok := portMap["containerPort"]; ok {
		var portNum int
		switch p := containerPort.(type) {
		case int:
			portNum = p
		case float64:
			portNum = int(p)
		default:
			errs = append(errs, errors.NewValidationFailed(path+".containerPort", "containerPort must be a number"))
		}

		if portNum < 1 || portNum > 65535 {
			errs = append(errs, errors.NewValidationFailed(path+".containerPort", "containerPort must be between 1 and 65535"))
		}
	} else {
		errs = append(errs, errors.NewValidationFailed(path+".containerPort", "containerPort is required"))
	}

	// Validate protocol if present
	if protocol, ok := portMap["protocol"].(string); ok {
		if !v.isValidProtocol(protocol) {
			errs = append(errs, errors.NewValidationFailed(path+".protocol", fmt.Sprintf("invalid protocol: %s", protocol)))
		}
	}

	return errs, warnings
}

// validateServicePort validates service port specifications
func (v *KubernetesManifestValidator) validateServicePort(port interface{}, path string) ([]error, []string) {
	var errs []error
	var warnings []string

	portMap, ok := port.(map[string]interface{})
	if !ok {
		errs = append(errs, errors.NewValidationFailed(path, "port must be an object"))
		return errs, warnings
	}

	// Validate port
	if portNum, ok := portMap["port"]; ok {
		var port int
		switch p := portNum.(type) {
		case int:
			port = p
		case float64:
			port = int(p)
		default:
			errs = append(errs, errors.NewValidationFailed(path+".port", "port must be a number"))
		}

		if port < 1 || port > 65535 {
			errs = append(errs, errors.NewValidationFailed(path+".port", "port must be between 1 and 65535"))
		}
	} else {
		errs = append(errs, errors.NewValidationFailed(path+".port", "port is required"))
	}

	// Validate targetPort if present
	if targetPort, ok := portMap["targetPort"]; ok {
		switch tp := targetPort.(type) {
		case int:
			if tp < 1 || tp > 65535 {
				errs = append(errs, errors.NewValidationFailed(path+".targetPort", "targetPort must be between 1 and 65535"))
			}
		case float64:
			if int(tp) < 1 || int(tp) > 65535 {
				errs = append(errs, errors.NewValidationFailed(path+".targetPort", "targetPort must be between 1 and 65535"))
			}
		case string:
			// Named ports are allowed
		default:
			errs = append(errs, errors.NewValidationFailed(path+".targetPort", "targetPort must be a number or string"))
		}
	}

	// Validate protocol if present
	if protocol, ok := portMap["protocol"].(string); ok {
		if !v.isValidProtocol(protocol) {
			errs = append(errs, errors.NewValidationFailed(path+".protocol", fmt.Sprintf("invalid protocol: %s", protocol)))
		}
	}

	return errs, warnings
}

// Helper validation functions
func (v *KubernetesManifestValidator) isValidRestartPolicy(policy string) bool {
	validPolicies := []string{"Always", "OnFailure", "Never"}
	for _, valid := range validPolicies {
		if policy == valid {
			return true
		}
	}
	return false
}

func (v *KubernetesManifestValidator) isValidServiceType(serviceType string) bool {
	validTypes := []string{"ClusterIP", "NodePort", "LoadBalancer", "ExternalName"}
	for _, valid := range validTypes {
		if serviceType == valid {
			return true
		}
	}
	return false
}

func (v *KubernetesManifestValidator) isValidProtocol(protocol string) bool {
	validProtocols := []string{"TCP", "UDP", "SCTP"}
	for _, valid := range validProtocols {
		if protocol == valid {
			return true
		}
	}
	return false
}
