package deploy

import (
	"fmt"
	"regexp"

	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
)

// ManifestValidator validates Kubernetes manifests
type ManifestValidator struct {
	logger zerolog.Logger
}

// NewManifestValidator creates a new manifest validator
func NewManifestValidator(logger zerolog.Logger) *ManifestValidator {
	return &ManifestValidator{
		logger: logger.With().Str("component", "manifest_validator").Logger(),
	}
}

// ValidateManifest validates a single manifest
func (v *ManifestValidator) ValidateManifest(manifest ManifestFile) error {
	v.logger.Debug().
		Str("kind", manifest.Kind).
		Str("name", manifest.Name).
		Msg("Validating manifest")

	// Basic validation
	if manifest.Content == "" {
		return fmt.Errorf("manifest content is empty")
	}

	// Parse YAML to check structure
	var doc map[string]interface{}
	if err := yaml.Unmarshal([]byte(manifest.Content), &doc); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}

	// Validate required fields
	if err := v.validateRequiredFields(doc, manifest.Kind); err != nil {
		return err
	}

	// Kind-specific validation
	switch manifest.Kind {
	case "Deployment":
		return v.validateDeployment(doc)
	case "Service":
		return v.validateService(doc)
	case "ConfigMap":
		return v.validateConfigMap(doc)
	case "Secret":
		return v.validateSecret(doc)
	case "Ingress":
		return v.validateIngress(doc)
	case "PersistentVolumeClaim":
		return v.validatePVC(doc)
	default:
		v.logger.Warn().Str("kind", manifest.Kind).Msg("Unknown manifest kind, applying basic validation only")
		return nil
	}
}

// ValidateManifests validates multiple manifests
func (v *ManifestValidator) ValidateManifests(manifests []ManifestFile) []ValidationResult {
	v.logger.Info().Int("count", len(manifests)).Msg("Validating manifests")

	results := make([]ValidationResult, len(manifests))

	for i, manifest := range manifests {
		result := ValidationResult{
			ManifestName: manifest.Name,
			Valid:        true,
			Errors:       []string{},
			Warnings:     []string{},
		}

		if err := v.ValidateManifest(manifest); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, err.Error())
		}

		// Additional checks that generate warnings
		warnings := v.checkBestPractices(manifest)
		result.Warnings = append(result.Warnings, warnings...)

		results[i] = result
	}

	// Cross-manifest validation
	v.validateCrossReferences(manifests, results)

	return results
}

// validateRequiredFields checks that required fields are present
func (v *ManifestValidator) validateRequiredFields(doc map[string]interface{}, kind string) error {
	// Check API version
	if _, ok := doc["apiVersion"]; !ok {
		return fmt.Errorf("missing required field: apiVersion")
	}

	// Check kind
	if docKind, ok := doc["kind"].(string); !ok || docKind != kind {
		return fmt.Errorf("kind mismatch: expected %s, got %v", kind, doc["kind"])
	}

	// Check metadata
	metadata, ok := doc["metadata"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("missing required field: metadata")
	}

	// Check name
	if _, ok := metadata["name"]; !ok {
		return fmt.Errorf("missing required field: metadata.name")
	}

	// Validate name format
	if name, ok := metadata["name"].(string); ok {
		if err := v.validateKubernetesName(name); err != nil {
			return fmt.Errorf("invalid name: %w", err)
		}
	}

	return nil
}

// validateKubernetesName validates Kubernetes resource names
func (v *ManifestValidator) validateKubernetesName(name string) error {
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}

	if len(name) > 253 {
		return fmt.Errorf("name too long (max 253 characters)")
	}

	// Must consist of lower case alphanumeric characters, '-' or '.'
	validName := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)
	if !validName.MatchString(name) {
		return fmt.Errorf("name must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character")
	}

	return nil
}

// validateDeployment validates deployment-specific fields
func (v *ManifestValidator) validateDeployment(doc map[string]interface{}) error {
	spec, ok := doc["spec"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("missing required field: spec")
	}

	// Check replicas
	if replicas, ok := spec["replicas"].(int); ok && replicas < 0 {
		return fmt.Errorf("invalid replicas: must be >= 0")
	}

	// Check selector
	if _, ok := spec["selector"]; !ok {
		return fmt.Errorf("missing required field: spec.selector")
	}

	// Check template
	template, ok := spec["template"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("missing required field: spec.template")
	}

	// Check template.spec
	if templateSpec, ok := template["spec"].(map[string]interface{}); ok {
		// Check containers
		containers, ok := templateSpec["containers"].([]interface{})
		if !ok || len(containers) == 0 {
			return fmt.Errorf("at least one container is required")
		}

		// Validate each container
		for i, container := range containers {
			if err := v.validateContainer(container, i); err != nil {
				return err
			}
		}
	} else {
		return fmt.Errorf("missing required field: spec.template.spec")
	}

	return nil
}

// validateContainer validates container configuration
func (v *ManifestValidator) validateContainer(container interface{}, index int) error {
	cont, ok := container.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid container at index %d", index)
	}

	// Check name
	if _, ok := cont["name"]; !ok {
		return fmt.Errorf("container at index %d missing name", index)
	}

	// Check image
	if _, ok := cont["image"]; !ok {
		return fmt.Errorf("container at index %d missing image", index)
	}

	return nil
}

// validateService validates service-specific fields
func (v *ManifestValidator) validateService(doc map[string]interface{}) error {
	spec, ok := doc["spec"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("missing required field: spec")
	}

	// Check ports
	ports, ok := spec["ports"].([]interface{})
	if !ok || len(ports) == 0 {
		return fmt.Errorf("at least one port is required")
	}

	// Validate each port
	for i, port := range ports {
		if err := v.validateServicePort(port, i); err != nil {
			return err
		}
	}

	// Check selector
	if _, ok := spec["selector"]; !ok {
		return fmt.Errorf("missing required field: spec.selector")
	}

	return nil
}

// validateServicePort validates service port configuration
func (v *ManifestValidator) validateServicePort(port interface{}, index int) error {
	p, ok := port.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid port at index %d", index)
	}

	// Check port number
	if portNum, ok := p["port"].(int); !ok || portNum < 1 || portNum > 65535 {
		return fmt.Errorf("port at index %d has invalid port number", index)
	}

	// Check target port if specified
	if targetPort, ok := p["targetPort"].(int); ok && (targetPort < 1 || targetPort > 65535) {
		return fmt.Errorf("port at index %d has invalid targetPort", index)
	}

	return nil
}

// validateConfigMap validates ConfigMap-specific fields
func (v *ManifestValidator) validateConfigMap(doc map[string]interface{}) error {
	// ConfigMaps must have either data or binaryData
	_, hasData := doc["data"]
	_, hasBinaryData := doc["binaryData"]

	if !hasData && !hasBinaryData {
		return fmt.Errorf("ConfigMap must have either 'data' or 'binaryData'")
	}

	return nil
}

// validateSecret validates Secret-specific fields
func (v *ManifestValidator) validateSecret(doc map[string]interface{}) error {
	// Check type
	secretType, ok := doc["type"].(string)
	if !ok {
		return fmt.Errorf("missing required field: type")
	}

	// Validate known secret types
	validTypes := []string{
		"Opaque",
		"kubernetes.io/service-account-token",
		"kubernetes.io/dockercfg",
		"kubernetes.io/dockerconfigjson",
		"kubernetes.io/basic-auth",
		"kubernetes.io/ssh-auth",
		"kubernetes.io/tls",
	}

	isValidType := false
	for _, validType := range validTypes {
		if secretType == validType {
			isValidType = true
			break
		}
	}

	if !isValidType {
		v.logger.Warn().Str("type", secretType).Msg("Unknown secret type")
	}

	return nil
}

// validateIngress validates Ingress-specific fields
func (v *ManifestValidator) validateIngress(doc map[string]interface{}) error {
	spec, ok := doc["spec"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("missing required field: spec")
	}

	// Check rules
	rules, ok := spec["rules"].([]interface{})
	if !ok || len(rules) == 0 {
		return fmt.Errorf("at least one rule is required")
	}

	return nil
}

// validatePVC validates PersistentVolumeClaim-specific fields
func (v *ManifestValidator) validatePVC(doc map[string]interface{}) error {
	spec, ok := doc["spec"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("missing required field: spec")
	}

	// Check accessModes
	if _, ok := spec["accessModes"]; !ok {
		return fmt.Errorf("missing required field: spec.accessModes")
	}

	// Check resources
	if _, ok := spec["resources"]; !ok {
		return fmt.Errorf("missing required field: spec.resources")
	}

	return nil
}

// checkBestPractices checks for best practice violations
func (v *ManifestValidator) checkBestPractices(manifest ManifestFile) []string {
	var warnings []string

	// Parse manifest
	var doc map[string]interface{}
	if err := yaml.Unmarshal([]byte(manifest.Content), &doc); err != nil {
		return warnings
	}

	// Check for labels
	if metadata, ok := doc["metadata"].(map[string]interface{}); ok {
		if _, hasLabels := metadata["labels"]; !hasLabels {
			warnings = append(warnings, "Consider adding labels for better resource management")
		}
	}

	// Deployment-specific checks
	if manifest.Kind == "Deployment" {
		warnings = append(warnings, v.checkDeploymentBestPractices(doc)...)
	}

	// Service-specific checks
	if manifest.Kind == "Service" {
		warnings = append(warnings, v.checkServiceBestPractices(doc)...)
	}

	return warnings
}

// checkDeploymentBestPractices checks deployment best practices
func (v *ManifestValidator) checkDeploymentBestPractices(doc map[string]interface{}) []string {
	var warnings []string

	if spec, ok := doc["spec"].(map[string]interface{}); ok {
		// Check replicas
		if replicas, ok := spec["replicas"].(int); ok && replicas == 1 {
			warnings = append(warnings, "Consider using more than 1 replica for high availability")
		}

		// Check pod template
		if template, ok := spec["template"].(map[string]interface{}); ok {
			if templateSpec, ok := template["spec"].(map[string]interface{}); ok {
				// Check containers
				if containers, ok := templateSpec["containers"].([]interface{}); ok {
					for i, container := range containers {
						if cont, ok := container.(map[string]interface{}); ok {
							// Check resource limits
							if _, hasResources := cont["resources"]; !hasResources {
								warnings = append(warnings, fmt.Sprintf("Container %d: Consider setting resource requests and limits", i))
							}

							// Check liveness/readiness probes
							if _, hasLiveness := cont["livenessProbe"]; !hasLiveness {
								warnings = append(warnings, fmt.Sprintf("Container %d: Consider adding a liveness probe", i))
							}
							if _, hasReadiness := cont["readinessProbe"]; !hasReadiness {
								warnings = append(warnings, fmt.Sprintf("Container %d: Consider adding a readiness probe", i))
							}
						}
					}
				}
			}
		}
	}

	return warnings
}

// checkServiceBestPractices checks service best practices
func (v *ManifestValidator) checkServiceBestPractices(doc map[string]interface{}) []string {
	var warnings []string

	if spec, ok := doc["spec"].(map[string]interface{}); ok {
		// Check service type
		if serviceType, ok := spec["type"].(string); ok && serviceType == "LoadBalancer" {
			warnings = append(warnings, "LoadBalancer services can be expensive in cloud environments")
		}
	}

	return warnings
}

// validateCrossReferences validates references between manifests
func (v *ManifestValidator) validateCrossReferences(manifests []ManifestFile, results []ValidationResult) {
	// Build maps of available resources
	services := make(map[string]bool)
	configMaps := make(map[string]bool)
	secrets := make(map[string]bool)

	for _, manifest := range manifests {
		switch manifest.Kind {
		case "Service":
			services[manifest.Name] = true
		case "ConfigMap":
			configMaps[manifest.Name] = true
		case "Secret":
			secrets[manifest.Name] = true
		}
	}

	// Check references in deployments
	for i, manifest := range manifests {
		if manifest.Kind == "Deployment" {
			var doc map[string]interface{}
			if err := yaml.Unmarshal([]byte(manifest.Content), &doc); err != nil {
				continue
			}

			// Check service references in deployment
			if spec, ok := doc["spec"].(map[string]interface{}); ok {
				if template, ok := spec["template"].(map[string]interface{}); ok {
					if templateSpec, ok := template["spec"].(map[string]interface{}); ok {
						// Check environment references
						if containers, ok := templateSpec["containers"].([]interface{}); ok {
							for _, container := range containers {
								if cont, ok := container.(map[string]interface{}); ok {
									v.checkContainerReferences(cont, configMaps, secrets, &results[i])
								}
							}
						}
					}
				}
			}
		}
	}
}
