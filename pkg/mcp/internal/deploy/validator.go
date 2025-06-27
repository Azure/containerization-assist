package deploy

import (
	"fmt"
	"regexp"

	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
)

// minInt returns the minimum of two integers
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

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
		return types.NewValidationErrorBuilder("Manifest content is empty", "content", manifest.Content).
			WithOperation("validate_manifest").
			WithStage("content_validation").
			WithRootCause("Manifest file contains no content or failed to load").
			WithImmediateStep(1, "Check file", "Verify the manifest file exists and has content").
			WithImmediateStep(2, "Regenerate", "Use manifest generation tools to create valid content").
			Build()
	}

	// Parse YAML to check structure
	var doc map[string]interface{}
	if err := yaml.Unmarshal([]byte(manifest.Content), &doc); err != nil {
		return types.NewValidationErrorBuilder("Invalid YAML syntax in manifest", "yaml_content", string(manifest.Content[:minInt(100, len(manifest.Content))])).
			WithOperation("validate_manifest").
			WithStage("yaml_parsing").
			WithRootCause(fmt.Sprintf("YAML parsing failed: %v", err)).
			WithImmediateStep(1, "Check syntax", "Validate YAML syntax using a YAML validator").
			WithImmediateStep(2, "Fix indentation", "Ensure proper YAML indentation (spaces, not tabs)").
			WithImmediateStep(3, "Check quotes", "Verify string values are properly quoted").
			Build()
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
		return types.NewValidationErrorBuilder("Missing required field: apiVersion", "apiVersion", nil).
			WithOperation("validate_manifest").
			WithStage("required_fields").
			WithRootCause("Kubernetes manifests must specify an apiVersion").
			WithImmediateStep(1, "Add apiVersion", "Add apiVersion field (e.g., 'apiVersion: apps/v1' for Deployments)").
			WithImmediateStep(2, "Check documentation", "Refer to Kubernetes API documentation for correct apiVersion").
			Build()
	}

	// Check kind
	if docKind, ok := doc["kind"].(string); !ok || docKind != kind {
		return types.NewValidationErrorBuilder("Kind mismatch in manifest", "kind", doc["kind"]).
			WithField("expected", kind).
			WithOperation("validate_manifest").
			WithStage("required_fields").
			WithRootCause(fmt.Sprintf("Expected kind '%s' but found '%v'", kind, doc["kind"])).
			WithImmediateStep(1, "Fix kind", fmt.Sprintf("Set kind to '%s'", kind)).
			WithImmediateStep(2, "Verify resource type", "Ensure you're using the correct Kubernetes resource type").
			Build()
	}

	// Check metadata
	metadata, ok := doc["metadata"].(map[string]interface{})
	if !ok {
		return types.NewValidationErrorBuilder("Missing required field: metadata", "metadata", doc["metadata"]).
			WithOperation("validate_manifest").
			WithStage("required_fields").
			WithRootCause("Kubernetes manifests must have a metadata section").
			WithImmediateStep(1, "Add metadata", "Add metadata section with at least a name field").
			WithImmediateStep(2, "Check structure", "Verify metadata is an object, not a string or array").
			Build()
	}

	// Check name
	if _, ok := metadata["name"]; !ok {
		return types.NewValidationErrorBuilder("Missing required field: metadata.name", "metadata.name", nil).
			WithOperation("validate_manifest").
			WithStage("required_fields").
			WithRootCause("Kubernetes resources must have a name in metadata").
			WithImmediateStep(1, "Add name", "Add 'name' field to metadata section").
			WithImmediateStep(2, "Use valid name", "Ensure name follows Kubernetes naming conventions").
			Build()
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
		return types.NewValidationErrorBuilder("Resource name cannot be empty", "name", name).
			WithOperation("validate_name").
			WithStage("name_validation").
			WithRootCause("Kubernetes resources must have non-empty names").
			WithImmediateStep(1, "Set name", "Provide a valid name for the resource").
			WithImmediateStep(2, "Use convention", "Use lowercase letters, numbers, and hyphens").
			Build()
	}

	if len(name) > 253 {
		return types.NewValidationErrorBuilder("Resource name too long", "name", name).
			WithField("length", len(name)).
			WithField("max_length", 253).
			WithOperation("validate_name").
			WithStage("name_validation").
			WithRootCause("Kubernetes resource names cannot exceed 253 characters").
			WithImmediateStep(1, "Shorten name", "Reduce name to 253 characters or less").
			WithImmediateStep(2, "Use abbreviations", "Consider using abbreviations or shorter identifiers").
			Build()
	}

	// Must consist of lower case alphanumeric characters, '-' or '.'
	validName := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)
	if !validName.MatchString(name) {
		return types.NewValidationErrorBuilder("Invalid resource name format", "name", name).
			WithOperation("validate_name").
			WithStage("name_validation").
			WithRootCause("Name doesn't follow Kubernetes naming conventions").
			WithImmediateStep(1, "Fix format", "Use lowercase letters, numbers, and hyphens only").
			WithImmediateStep(2, "Start/end correctly", "Start and end with alphanumeric characters").
			WithImmediateStep(3, "Remove invalid chars", "Remove uppercase letters, underscores, or special characters").
			Build()
	}

	return nil
}

// validateDeployment validates deployment-specific fields
func (v *ManifestValidator) validateDeployment(doc map[string]interface{}) error {
	spec, ok := doc["spec"].(map[string]interface{})
	if !ok {
		return types.NewValidationErrorBuilder("Missing required field: spec", "spec", doc["spec"]).
			WithOperation("validate_deployment").
			WithStage("spec_validation").
			WithRootCause("Deployment manifests must have a spec section").
			WithImmediateStep(1, "Add spec", "Add spec section to deployment").
			WithImmediateStep(2, "Include required fields", "Add replicas, selector, and template to spec").
			Build()
	}

	// Check replicas
	if replicas, ok := spec["replicas"].(int); ok && replicas < 0 {
		return types.NewValidationErrorBuilder("Invalid replicas count", "replicas", replicas).
			WithOperation("validate_deployment").
			WithStage("replicas_validation").
			WithRootCause("Replica count cannot be negative").
			WithImmediateStep(1, "Set positive value", "Use 0 or higher for replicas count").
			WithImmediateStep(2, "Scale appropriately", "Consider resource availability when setting replicas").
			Build()
	}

	// Check selector
	if _, ok := spec["selector"]; !ok {
		return types.NewValidationErrorBuilder("Missing required field: spec.selector", "selector", nil).
			WithOperation("validate_deployment").
			WithStage("selector_validation").
			WithRootCause("Deployments must specify a selector to match pods").
			WithImmediateStep(1, "Add selector", "Add matchLabels selector to spec").
			WithImmediateStep(2, "Match template labels", "Ensure selector matches pod template labels").
			Build()
	}

	// Check template
	template, ok := spec["template"].(map[string]interface{})
	if !ok {
		return types.NewValidationErrorBuilder("Missing required field: spec.template", "template", spec["template"]).
			WithOperation("validate_deployment").
			WithStage("template_validation").
			WithRootCause("Deployments must specify a pod template").
			WithImmediateStep(1, "Add template", "Add pod template to spec").
			WithImmediateStep(2, "Include metadata", "Add metadata with labels to template").
			WithImmediateStep(3, "Include spec", "Add container spec to template").
			Build()
	}

	// Check template.spec
	if templateSpec, ok := template["spec"].(map[string]interface{}); ok {
		// Check containers
		containers, ok := templateSpec["containers"].([]interface{})
		if !ok || len(containers) == 0 {
			return types.NewValidationErrorBuilder("At least one container is required", "containers", templateSpec["containers"]).
				WithOperation("validate_deployment").
				WithStage("container_validation").
				WithRootCause("Pod templates must specify at least one container").
				WithImmediateStep(1, "Add container", "Add at least one container to the containers array").
				WithImmediateStep(2, "Specify image", "Ensure each container has a valid image reference").
				WithImmediateStep(3, "Set name", "Give each container a unique name").
				Build()
		}

		// Validate each container
		for i, container := range containers {
			if err := v.validateContainer(container, i); err != nil {
				return err
			}
		}
	} else {
		return types.NewValidationErrorBuilder("Missing required field: spec.template.spec", "template.spec", template["spec"]).
			WithOperation("validate_deployment").
			WithStage("template_validation").
			WithRootCause("Pod templates must have a spec section").
			WithImmediateStep(1, "Add template spec", "Add spec section to pod template").
			WithImmediateStep(2, "Include containers", "Add containers array to template spec").
			Build()
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
