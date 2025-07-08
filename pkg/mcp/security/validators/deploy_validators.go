package validators

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/errors"
	"gopkg.in/yaml.v3"
)

// DeployValidator provides validation specific to deployment operations
type DeployValidator struct {
	unified *UnifiedValidator
}

// NewDeployValidator creates a new deploy validator
func NewDeployValidator() *DeployValidator {
	return &DeployValidator{
		unified: NewUnifiedValidator(),
	}
}

// ValidateDeployArgs validates arguments for Kubernetes deployment
func (dv *DeployValidator) ValidateDeployArgs(ctx context.Context, sessionID string, manifests []string, namespace string) error {
	vctx := NewValidateContext(ctx)

	// Validate session ID
	if err := dv.unified.Input.ValidateSessionID(sessionID); err != nil {
		vctx.AddError(err)
	}

	// Validate manifests
	if err := dv.unified.Manifest.ValidateManifestFiles(manifests); err != nil {
		vctx.AddError(err)
	}

	// Validate namespace if provided
	if namespace != "" {
		if err := dv.unified.Input.ValidateKubernetesName(namespace); err != nil {
			vctx.AddError(err)
		}
	}

	return vctx.GetFirstError()
}

// ValidateKubectlAvailable checks if kubectl is available
func (dv *DeployValidator) ValidateKubectlAvailable() error {
	return dv.unified.System.ValidateCommandAvailable("kubectl")
}

// ValidateManifestContent validates the content of Kubernetes manifests
func (dv *DeployValidator) ValidateManifestContent(manifestPath string) (*ManifestValidationResult, error) {
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, errors.Wrapf(err, "deploy", "failed to read manifest file: %s", manifestPath)
	}

	result := &ManifestValidationResult{
		FilePath: manifestPath,
		Valid:    true,
		Warnings: []string{},
		Errors:   []string{},
	}

	// Parse YAML documents
	decoder := yaml.NewDecoder(strings.NewReader(string(content)))
	docIndex := 0

	for {
		var doc map[string]interface{}
		err := decoder.Decode(&doc)
		if err == io.EOF {
			break
		}
		if err != nil {
			result.Valid = false
			result.Errors = append(result.Errors,
				fmt.Sprintf("Document %d: Invalid YAML: %v", docIndex, err))
			continue
		}

		// Validate document structure
		if err := dv.validateManifestDocument(doc, docIndex, result); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors,
				fmt.Sprintf("Document %d: %v", docIndex, err))
		}

		docIndex++
	}

	if docIndex == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "No valid YAML documents found")
	}

	return result, nil
}

// validateManifestDocument validates a single YAML document
func (dv *DeployValidator) validateManifestDocument(doc map[string]interface{}, docIndex int, result *ManifestValidationResult) error {
	// Check required fields
	apiVersion, ok := doc["apiVersion"].(string)
	if !ok || apiVersion == "" {
		return errors.Validation("deploy", "apiVersion is required")
	}

	kind, ok := doc["kind"].(string)
	if !ok || kind == "" {
		return errors.Validation("deploy", "kind is required")
	}

	metadata, ok := doc["metadata"].(map[string]interface{})
	if !ok {
		return errors.Validation("deploy", "metadata is required")
	}

	name, ok := metadata["name"].(string)
	if !ok || name == "" {
		return errors.Validation("deploy", "metadata.name is required")
	}

	// Validate resource name
	if err := dv.unified.Input.ValidateKubernetesName(name); err != nil {
		return err
	}

	// Add warnings for common issues
	dv.addManifestWarnings(doc, result)

	return nil
}

// addManifestWarnings adds warnings for common manifest issues
func (dv *DeployValidator) addManifestWarnings(doc map[string]interface{}, result *ManifestValidationResult) {
	kind, _ := doc["kind"].(string)

	// Check for missing namespace in namespaced resources
	if dv.isNamespacedResource(kind) {
		if metadata, ok := doc["metadata"].(map[string]interface{}); ok {
			if _, hasNamespace := metadata["namespace"]; !hasNamespace {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("No namespace specified for %s resource", kind))
			}
		}
	}

	// Check for missing resource limits in Deployment/StatefulSet
	if kind == "Deployment" || kind == "StatefulSet" {
		if spec, ok := doc["spec"].(map[string]interface{}); ok {
			if template, ok := spec["template"].(map[string]interface{}); ok {
				if tspec, ok := template["spec"].(map[string]interface{}); ok {
					if containers, ok := tspec["containers"].([]interface{}); ok {
						for _, container := range containers {
							if cont, ok := container.(map[string]interface{}); ok {
								if resources, ok := cont["resources"].(map[string]interface{}); !ok || resources == nil {
									result.Warnings = append(result.Warnings,
										"Container missing resource limits/requests")
									break
								}
							}
						}
					}
				}
			}
		}
	}

	// Check for latest tag usage
	if spec, ok := doc["spec"].(map[string]interface{}); ok {
		dv.checkImageTags(spec, result)
	}
}

// checkImageTags recursively checks for image tags in the spec
func (dv *DeployValidator) checkImageTags(obj interface{}, result *ManifestValidationResult) {
	switch v := obj.(type) {
	case map[string]interface{}:
		for key, value := range v {
			if key == "image" {
				if imageStr, ok := value.(string); ok {
					if strings.Contains(imageStr, ":latest") || !strings.Contains(imageStr, ":") {
						result.Warnings = append(result.Warnings,
							"Using 'latest' tag or no tag is not recommended for production")
					}
				}
			} else {
				dv.checkImageTags(value, result)
			}
		}
	case []interface{}:
		for _, item := range v {
			dv.checkImageTags(item, result)
		}
	}
}

// isNamespacedResource checks if a Kubernetes resource is namespaced
func (dv *DeployValidator) isNamespacedResource(kind string) bool {
	namespacedResources := map[string]bool{
		"Deployment":            true,
		"StatefulSet":           true,
		"DaemonSet":             true,
		"ReplicaSet":            true,
		"Pod":                   true,
		"Service":               true,
		"ConfigMap":             true,
		"Secret":                true,
		"PersistentVolumeClaim": true,
		"Ingress":               true,
		"NetworkPolicy":         true,
		"Role":                  true,
		"RoleBinding":           true,
		"ServiceAccount":        true,
		"Job":                   true,
		"CronJob":               true,
	}
	return namespacedResources[kind]
}

// ValidateDeploymentStrategy validates deployment strategy configuration
func (dv *DeployValidator) ValidateDeploymentStrategy(strategy string, timeout time.Duration) error {
	validStrategies := map[string]bool{
		"rolling":    true,
		"recreate":   true,
		"blue-green": true,
	}

	if strategy != "" && !validStrategies[strategy] {
		return errors.Validationf("deploy", "invalid deployment strategy: %s", strategy)
	}

	if timeout <= 0 {
		return errors.Validation("deploy", "deployment timeout must be positive")
	}

	if timeout > 30*time.Minute {
		return errors.Validation("deploy", "deployment timeout too long (max 30 minutes)")
	}

	return nil
}

// ManifestValidationResult represents the result of manifest validation
type ManifestValidationResult struct {
	FilePath string   `json:"file_path"`
	Valid    bool     `json:"valid"`
	Warnings []string `json:"warnings"`
	Errors   []string `json:"errors"`
}

// ManifestGenerator provides utilities for generating Kubernetes manifests
type ManifestGenerator struct {
	validator *DeployValidator
}

// NewManifestGenerator creates a new manifest generator
func NewManifestGenerator() *ManifestGenerator {
	return &ManifestGenerator{
		validator: NewDeployValidator(),
	}
}

// ValidateManifestTemplate validates a manifest template before processing
func (mg *ManifestGenerator) ValidateManifestTemplate(templatePath string, data interface{}) error {
	// Check if template file exists
	if err := mg.validator.unified.FileSystem.ValidateFileExists(templatePath); err != nil {
		return err
	}

	// Check template file extension
	ext := filepath.Ext(templatePath)
	if ext != ".yaml" && ext != ".yml" && ext != ".tmpl" {
		return errors.Validationf("deploy", "invalid template file extension: %s", ext)
	}

	// Basic validation of template data
	if data == nil {
		return errors.Validation("deploy", "template data cannot be nil")
	}

	return nil
}

// ValidateGeneratedManifests validates generated manifests before deployment
func (mg *ManifestGenerator) ValidateGeneratedManifests(manifestDir string) ([]*ManifestValidationResult, error) {
	// Check if manifest directory exists
	if err := mg.validator.unified.FileSystem.ValidateDirectoryExists(manifestDir); err != nil {
		return nil, err
	}

	// Find all YAML files
	var results []*ManifestValidationResult

	err := filepath.Walk(manifestDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			ext := filepath.Ext(path)
			if ext == ".yaml" || ext == ".yml" {
				result, err := mg.validator.ValidateManifestContent(path)
				if err != nil {
					return err
				}
				results = append(results, result)
			}
		}

		return nil
	})

	if err != nil {
		return nil, errors.Wrapf(err, "deploy", "failed to validate generated manifests")
	}

	return results, nil
}

// HealthCheckValidator provides validation for health checks and readiness
type HealthCheckValidator struct {
	unified *UnifiedValidator
}

// NewHealthCheckValidator creates a new health check validator
func NewHealthCheckValidator() *HealthCheckValidator {
	return &HealthCheckValidator{
		unified: NewUnifiedValidator(),
	}
}

// ValidateHealthCheckArgs validates arguments for health checking
func (hcv *HealthCheckValidator) ValidateHealthCheckArgs(sessionID, namespace, deployment string) error {
	if err := hcv.unified.Input.ValidateSessionID(sessionID); err != nil {
		return err
	}

	if namespace != "" {
		if err := hcv.unified.Input.ValidateKubernetesName(namespace); err != nil {
			return err
		}
	}

	if deployment != "" {
		if err := hcv.unified.Input.ValidateKubernetesName(deployment); err != nil {
			return err
		}
	}

	return nil
}

// ValidateHealthEndpoint validates a health check endpoint URL
func (hcv *HealthCheckValidator) ValidateHealthEndpoint(endpoint string) error {
	if endpoint == "" {
		return errors.Validation("health", "health endpoint URL is required")
	}

	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		return errors.Validation("health", "health endpoint must be a valid HTTP/HTTPS URL")
	}

	return nil
}
