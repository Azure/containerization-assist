package utils

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

const (
	MaxNameLength       = 63
	MaxLabelLength      = 63
	MaxAnnotationLength = 256 * 1024 // 256KB
	MaxNamespaceLength  = 63
)

const (
	LabelApp       = "app"
	LabelVersion   = "version"
	LabelComponent = "app.kubernetes.io/component"
	LabelInstance  = "app.kubernetes.io/instance"
	LabelName      = "app.kubernetes.io/name"
	LabelPartOf    = "app.kubernetes.io/part-of"
	LabelManagedBy = "app.kubernetes.io/managed-by"
	LabelVersion2  = "app.kubernetes.io/version"
)

const (
	AnnotationDescription = "description"
	AnnotationLastApplied = "kubectl.kubernetes.io/last-applied-configuration"
	AnnotationChangeSet   = "container-kit.azure.com/changeset"
	AnnotationCreatedBy   = "container-kit.azure.com/created-by"
)

var (
	dns1123SubdomainPattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)

	dns1123LabelPattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

	dns1035LabelPattern = regexp.MustCompile(`^[a-z]([-a-z0-9]*[a-z0-9])?$`)

	labelValuePattern = regexp.MustCompile(`^(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?$`)

	qualifiedNamePattern = regexp.MustCompile(`^([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*/)?(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])$`)
)

// Resource naming utilities

// SanitizeForKubernetes creates a valid Kubernetes resource name from arbitrary input
func SanitizeForKubernetes(input string) string {
	if input == "" {
		return ""
	}

	// Convert to lowercase
	result := strings.ToLower(input)

	// Replace invalid characters with hyphens
	invalidChars := regexp.MustCompile(`[^a-z0-9-]`)
	result = invalidChars.ReplaceAllString(result, "-")

	// Remove leading/trailing hyphens
	result = strings.Trim(result, "-")

	// Collapse multiple consecutive hyphens
	multipleHyphens := regexp.MustCompile(`-+`)
	result = multipleHyphens.ReplaceAllString(result, "-")

	if len(result) > 0 {
		if !regexp.MustCompile(`^[a-z0-9]`).MatchString(result) {
			result = "x" + result
		}
		if !regexp.MustCompile(`[a-z0-9]$`).MatchString(result) {
			result = result + "x"
		}
	}

	if len(result) > MaxNameLength {
		result = result[:MaxNameLength]
		if !regexp.MustCompile(`[a-z0-9]$`).MatchString(result) {
			result = result[:len(result)-1] + "x"
		}
	}

	return result
}

// ValidateResourceName validates a Kubernetes resource name
func ValidateResourceName(name string) error {
	if name == "" {
		return errors.NewError().Messagef("resource name cannot be empty").WithLocation().Build()
	}

	if len(name) > MaxNameLength {
		return errors.NewError().Messagef("resource name must be at most %d characters", MaxNameLength).WithLocation().Build()
	}

	if !dns1123LabelPattern.MatchString(name) {
		return errors.NewError().Messagef("resource name must be a valid DNS-1123 label: %s", name).WithLocation().Build()
	}

	return nil
}

func ValidateNamespace(namespace string) error {
	if namespace == "" {
		return errors.NewError().Messagef("namespace cannot be empty").WithLocation().Build()
	}

	if len(namespace) > MaxNamespaceLength {
		return errors.NewError().Messagef("namespace must be at most %d characters", MaxNamespaceLength).WithLocation().Build()
	}

	if !dns1123LabelPattern.MatchString(namespace) {
		return errors.NewError().Messagef("namespace must be a valid DNS-1123 label: %s", namespace).WithLocation().Build()
	}

	reservedNamespaces := []string{"kube-system", "kube-public", "kube-node-lease", "default"}
	for _, reserved := range reservedNamespaces {
		if namespace == reserved {
			return errors.NewError().Messagef("namespace %s is reserved", namespace).WithLocation().Build()
		}
	}

	return nil
}

// ValidateServiceName validates a Kubernetes service name
func ValidateServiceName(name string) error {
	if name == "" {
		return errors.NewError().Messagef("service name cannot be empty").WithLocation().Build()
	}

	if len(name) > MaxNameLength {
		return errors.NewError().Messagef("service name must be at most %d characters", MaxNameLength).WithLocation().Build()
	}

	if !dns1035LabelPattern.MatchString(name) {
		return errors.NewError().Messagef("service name must be a valid DNS-1035 label: %s", name).WithLocation().Build()
	}

	return nil
}

// ValidateLabelKey validates a Kubernetes label key
func ValidateLabelKey(key string) error {
	if key == "" {
		return errors.NewError().Messagef("label key cannot be empty").WithLocation().Build()
	}

	parts := strings.Split(key, "/")

	if len(parts) == 1 {
		if len(key) > MaxLabelLength {
			return errors.NewError().Messagef("label key must be at most %d characters", MaxLabelLength).WithLocation().Build()
		}
		if !regexp.MustCompile(`^([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]$`).MatchString(key) {
			return errors.NewError().Messagef("invalid label key format: %s", key).WithLocation().Build()
		}
	} else if len(parts) == 2 {
		prefix, name := parts[0], parts[1]

		if len(prefix) > 253 {
			return errors.NewError().Messagef("label key prefix must be at most 253 characters").WithLocation().Build()
		}
		if len(name) > MaxLabelLength {
			return errors.NewError().Messagef("label key name must be at most %d characters", MaxLabelLength).WithLocation().Build()
		}

		if !dns1123SubdomainPattern.MatchString(prefix) {
			return errors.NewError().Messagef("invalid label key prefix format: %s", prefix).WithLocation().Build()
		}
		if !regexp.MustCompile(`^([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]$`).MatchString(name) {
			return errors.NewError().Messagef("invalid label key name format: %s", name).WithLocation().Build()
		}
	} else {
		return errors.NewError().Messagef("label key can have at most one slash: %s", key).WithLocation().Build()
	}

	return nil
}

func ValidateLabelValue(value string) error {
	if len(value) > MaxLabelLength {
		return errors.NewError().Messagef("label value must be at most %d characters", MaxLabelLength).WithLocation().Build()
	}

	if value != "" && !labelValuePattern.MatchString(value) {
		return errors.NewError().Messagef("invalid label value format: %s", value).WithLocation().Build()
	}

	return nil
}

func ValidateAnnotationKey(key string) error {
	return ValidateLabelKey(key)
}

// ValidateAnnotationValue validates a Kubernetes annotation value
func ValidateAnnotationValue(value string) error {
	if len(value) > MaxAnnotationLength {
		return errors.NewError().Messagef("annotation value must be at most %d bytes", MaxAnnotationLength).WithLocation().Build()
	}

	return nil
}

// SanitizeLabelValue creates a valid label value from arbitrary input
func SanitizeLabelValue(input string) string {
	if input == "" {
		return ""
	}

	validChars := regexp.MustCompile(`[^A-Za-z0-9\-_.]`)
	result := validChars.ReplaceAllString(input, "")

	if len(result) > 0 {
		if !regexp.MustCompile(`^[A-Za-z0-9]`).MatchString(result) {
			result = "x" + result
		}
		if !regexp.MustCompile(`[A-Za-z0-9]$`).MatchString(result) {
			result = result + "x"
		}
	}

	if len(result) > MaxLabelLength {
		result = result[:MaxLabelLength]
		if !regexp.MustCompile(`[A-Za-z0-9]$`).MatchString(result) {
			result = result[:len(result)-1] + "x"
		}
	}

	return result
}

// GenerateResourceName generates a unique resource name
func GenerateResourceName(prefix string) string {
	timestamp := time.Now().Unix()
	sanitizedPrefix := SanitizeForKubernetes(prefix)

	if sanitizedPrefix == "" {
		sanitizedPrefix = "resource"
	}

	name := fmt.Sprintf("%s-%d", sanitizedPrefix, timestamp)

	if len(name) > MaxNameLength {
		prefixLen := MaxNameLength - len(fmt.Sprintf("-%d", timestamp))
		if prefixLen > 0 {
			name = sanitizedPrefix[:prefixLen] + fmt.Sprintf("-%d", timestamp)
		} else {
			name = fmt.Sprintf("res-%d", timestamp)
		}
	}

	return name
}

// CreateStandardLabels creates standard Kubernetes labels
func CreateStandardLabels(appName, version, component string) map[string]string {
	labels := make(map[string]string)

	if appName != "" {
		labels[LabelName] = SanitizeLabelValue(appName)
		labels[LabelApp] = SanitizeLabelValue(appName)
	}

	if version != "" {
		labels[LabelVersion2] = SanitizeLabelValue(version)
		labels[LabelVersion] = SanitizeLabelValue(version)
	}

	if component != "" {
		labels[LabelComponent] = SanitizeLabelValue(component)
	}

	labels[LabelManagedBy] = "container-kit"

	return labels
}

// CreateStandardAnnotations creates standard annotations
func CreateStandardAnnotations(description string) map[string]string {
	annotations := make(map[string]string)

	if description != "" {
		annotations[AnnotationDescription] = description
	}

	annotations[AnnotationCreatedBy] = "container-kit"
	annotations[AnnotationChangeSet] = fmt.Sprintf("%d", time.Now().Unix())

	return annotations
}

// IsNamespaced returns true if the resource type is namespaced
func IsNamespaced(apiVersion, kind string) bool {
	clusterScoped := map[string][]string{
		"v1": {
			"Namespace", "Node", "PersistentVolume", "ClusterRole", "ClusterRoleBinding",
		},
		"rbac.authorization.k8s.io/v1": {
			"ClusterRole", "ClusterRoleBinding",
		},
		"apiextensions.k8s.io/v1": {
			"CustomResourceDefinition",
		},
		"admissionregistration.k8s.io/v1": {
			"ValidatingAdmissionWebhook", "MutatingAdmissionWebhook",
		},
		"storage.k8s.io/v1": {
			"StorageClass", "VolumeAttachment",
		},
		"networking.k8s.io/v1": {
			"IngressClass",
		},
	}

	if kinds, exists := clusterScoped[apiVersion]; exists {
		for _, k := range kinds {
			if k == kind {
				return false
			}
		}
	}

	return true
}

// GetResourceGroup returns the API group for a resource
func GetResourceGroup(apiVersion string) string {
	parts := strings.Split(apiVersion, "/")
	if len(parts) == 1 {
		return ""
	}
	return parts[0]
}

// GetResourceVersion returns the API version for a resource
func GetResourceVersion(apiVersion string) string {
	parts := strings.Split(apiVersion, "/")
	if len(parts) == 1 {
		return parts[0]
	}
	return parts[1]
}

// IsValidDNS1123Label checks if a string is a valid DNS-1123 label
func IsValidDNS1123Label(value string) bool {
	return len(value) <= MaxNameLength && dns1123LabelPattern.MatchString(value)
}

// IsValidDNS1123Subdomain checks if a string is a valid DNS-1123 subdomain
func IsValidDNS1123Subdomain(value string) bool {
	return len(value) <= 253 && dns1123SubdomainPattern.MatchString(value)
}

// IsValidDNS1035Label checks if a string is a valid DNS-1035 label
func IsValidDNS1035Label(value string) bool {
	return len(value) <= MaxNameLength && dns1035LabelPattern.MatchString(value)
}

// ValidatePort validates a Kubernetes port number
func ValidatePort(port int32) error {
	if port < 1 || port > 65535 {
		return errors.NewError().Messagef("port must be between 1 and 65535, got %d", port).WithLocation().Build()
	}
	return nil
}

func ValidatePortName(name string) error {
	if name == "" {
		return nil
	}

	if len(name) > 15 {
		return errors.NewError().Messagef("port name must be at most 15 characters").WithLocation().Build()
	}

	if !regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`).MatchString(name) {
		return errors.NewError().Messagef("invalid port name format: %s", name).WithLocation().Build()
	}

	return nil
}

// ValidateContainerName validates a container name
func ValidateContainerName(name string) error {
	if name == "" {
		return errors.NewError().Messagef("container name cannot be empty").WithLocation().Build()
	}

	if len(name) > MaxNameLength {
		return errors.NewError().Messagef("container name must be at most %d characters", MaxNameLength).WithLocation().Build()
	}

	if !dns1123LabelPattern.MatchString(name) {
		return errors.NewError().Messagef("container name must be a valid DNS-1123 label: %s", name).WithLocation().Build()
	}

	return nil
}

func SanitizeContainerName(input string) string {
	return SanitizeForKubernetes(input)
}

// ValidateEnvVarName validates an environment variable name
func ValidateEnvVarName(name string) error {
	if name == "" {
		return errors.NewError().Messagef("environment variable name cannot be empty").WithLocation().Build()
	}

	if !regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`).MatchString(name) {
		return errors.NewError().Messagef("invalid environment variable name: %s", name).WithLocation().Build()
	}

	return nil
}

func SanitizeEnvVarName(input string) string {
	if input == "" {
		return ""
	}

	result := strings.ToUpper(input)

	invalidChars := regexp.MustCompile(`[^A-Z0-9_]`)
	result = invalidChars.ReplaceAllString(result, "_")

	if len(result) > 0 && !regexp.MustCompile(`^[A-Z_]`).MatchString(result) {
		result = "X_" + result
	}

	multipleUnderscores := regexp.MustCompile(`_+`)
	result = multipleUnderscores.ReplaceAllString(result, "_")

	result = strings.Trim(result, "_")

	return result
}
