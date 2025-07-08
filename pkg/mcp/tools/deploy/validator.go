package deploy

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/core/tools"
	errors "github.com/Azure/container-kit/pkg/mcp/errors"
	"gopkg.in/yaml.v3"
)

// Type definitions are now located in types.go

// minInt function removed - was unused helper function

// Core validation infrastructure is now in core_validator.go

// validateDeployment validates deployment-specific fields
func (v *ManifestValidator) validateDeployment(doc map[string]interface{}) error {
	// Try typed validation first
	if typedSpec, err := v.convertToTypedDeploymentSpec(doc); err == nil {
		return v.validateDeploymentTyped(typedSpec)
	}

	// Fall back to interface{} validation
	return v.validateDeploymentInterface(doc)
}

// validateDeploymentInterface validates deployment using interface{} (legacy)
func (v *ManifestValidator) validateDeploymentInterface(doc map[string]interface{}) error {
	spec, ok := doc["spec"].(map[string]interface{})
	if !ok {
		return errors.NewError().Messagef("missing required field: spec").WithLocation(

		// Check replicas
		).Build()
	}

	if replicas, ok := spec["replicas"].(int); ok && replicas < 0 {
		return errors.NewError().Messagef("invalid replicas count: %d", replicas).WithLocation(

		// Check selector
		).Build()
	}

	if _, ok := spec["selector"]; !ok {
		return errors.NewError().Messagef("missing required field: spec.selector").WithLocation(

		// Check template
		).Build()
	}

	template, ok := spec["template"].(map[string]interface{})
	if !ok {
		return errors.NewError().Messagef("missing required field: spec.template").WithLocation(

		// Check template.spec
		).Build()
	}

	if templateSpec, ok := template["spec"].(map[string]interface{}); ok {
		// Check containers
		containers, ok := templateSpec["containers"].([]interface{})
		if !ok || len(containers) == 0 {
			return errors.NewError().Messagef("at least one container is required").WithLocation(

			// Validate each container
			).Build()
		}

		for i, container := range containers {
			if err := v.validateContainer(container, i); err != nil {
				return err
			}
		}
	} else {
		return errors.NewError().Messagef("missing required field: spec.template.spec").WithLocation().Build(

		// convertToTypedDeploymentSpec converts interface{} to typed deployment spec
		)
	}

	return nil
}

func (v *ManifestValidator) convertToTypedDeploymentSpec(doc map[string]interface{}) (TypedDeploymentSpec, error) {
	var spec TypedDeploymentSpec

	specData, ok := doc["spec"].(map[string]interface{})
	if !ok {
		return spec, errors.NewError().Messagef("spec field not found or not a map").WithLocation(

		// Convert to YAML and back to get typed structure
		).Build()
	}

	yamlData, err := yaml.Marshal(specData)
	if err != nil {
		return spec, err
	}

	err = yaml.Unmarshal(yamlData, &spec)
	return spec, err
}

// validateDeploymentTyped validates typed deployment specification
func (v *ManifestValidator) validateDeploymentTyped(spec TypedDeploymentSpec) error {
	// Check replicas
	if spec.Replicas < 0 {
		return errors.NewError().Messagef("invalid replicas count: %d", spec.Replicas).WithLocation(

		// Check containers
		).Build()
	}

	if len(spec.Template.Spec.Containers) == 0 {
		return errors.NewError().Messagef("at least one container is required").WithLocation(

		// Validate each container
		).Build()
	}

	for i, container := range spec.Template.Spec.Containers {
		if err := v.validateContainerTyped(container, i); err != nil {
			return err
		}
	}

	return nil
}

// validateContainerTyped validates typed container configuration
func (v *ManifestValidator) validateContainerTyped(container TypedContainer, index int) error {
	// Check name
	if container.Name == "" {
		return errors.NewError().Messagef("container at index %d missing name", index).WithLocation(

		// Check image
		).Build()
	}

	if container.Image == "" {
		return errors.NewError().Messagef("container at index %d missing image", index).WithLocation(

		// Validate ports
		).Build()
	}

	for i, port := range container.Ports {
		if err := v.validateContainerPortTyped(port, index, i); err != nil {
			return err
		}
	}

	// Validate image pull policy
	if container.ImagePullPolicy != "" {
		validPolicies := map[string]bool{
			"Always":       true,
			"Never":        true,
			"IfNotPresent": true,
		}
		if !validPolicies[container.ImagePullPolicy] {
			return errors.NewError().Messagef("invalid imagePullPolicy for container %s: %s", container.Name, container.ImagePullPolicy).WithLocation().Build()
		}
	}

	return nil
}

// validateContainerPortTyped validates typed container port
func (v *ManifestValidator) validateContainerPortTyped(port TypedContainerPort, containerIndex, portIndex int) error {
	// Validate port number
	if port.ContainerPort <= 0 || port.ContainerPort > 65535 {
		return errors.NewError().Messagef("invalid container port %d for container at index %d, port at index %d", port.ContainerPort, containerIndex, portIndex).WithLocation(

		// Validate protocol
		).Build()
	}

	if port.Protocol != "" && port.Protocol != "TCP" && port.Protocol != "UDP" {
		return errors.NewError().Messagef("invalid protocol %s for container at index %d, port at index %d", port.Protocol, containerIndex, portIndex).WithLocation().Build(

		// validateContainer validates container configuration
		)
	}

	return nil
}

func (v *ManifestValidator) validateContainer(container interface{}, index int) error {
	cont, ok := container.(map[string]interface{})
	if !ok {
		return errors.NewError().Messagef("invalid container at index %d", index).WithLocation(

		// Check name
		).Build()
	}

	if _, ok := cont["name"]; !ok {
		return errors.NewError().Messagef("container at index %d missing name", index).WithLocation(

		// Check image
		).Build()
	}

	if _, ok := cont["image"]; !ok {
		return errors.NewError().Messagef("container at index %d missing image", index).WithLocation().Build(

		// validateService validates service-specific fields
		)
	}

	return nil
}

func (v *ManifestValidator) validateService(doc map[string]interface{}) error {
	spec, ok := doc["spec"].(map[string]interface{})
	if !ok {
		return errors.NewError().Messagef("missing required field: spec").WithLocation(

		// Check ports
		).Build()
	}

	ports, ok := spec["ports"].([]interface{})
	if !ok || len(ports) == 0 {
		return errors.NewError().Messagef("at least one port is required").WithLocation(

		// Validate each port
		).Build()
	}

	for i, port := range ports {
		if err := v.validateServicePort(port, i); err != nil {
			return err
		}
	}

	// Check selector
	if _, ok := spec["selector"]; !ok {
		return errors.NewError().Messagef("missing required field: spec.selector").WithLocation().Build(

		// validateServicePort validates service port configuration
		)
	}

	return nil
}

func (v *ManifestValidator) validateServicePort(port interface{}, index int) error {
	p, ok := port.(map[string]interface{})
	if !ok {
		return errors.NewError().Messagef("invalid port at index %d", index).WithLocation(

		// Check port number
		).Build()
	}

	if portNum, ok := p["port"].(int); !ok || portNum < 1 || portNum > 65535 {
		return errors.NewError().Messagef("port at index %d has invalid port number", index).WithLocation(

		// Check target port if specified
		).Build()
	}

	if targetPort, ok := p["targetPort"].(int); ok && (targetPort < 1 || targetPort > 65535) {
		return errors.NewError().Messagef("port at index %d has invalid targetPort", index).WithLocation().Build(

		// validateConfigMap validates ConfigMap-specific fields
		)
	}

	return nil
}

func (v *ManifestValidator) validateConfigMap(doc map[string]interface{}) error {
	// ConfigMaps must have either data or binaryData
	_, hasData := doc["data"]
	_, hasBinaryData := doc["binaryData"]

	if !hasData && !hasBinaryData {
		return errors.NewError().Messagef("ConfigMap must have either 'data' or 'binaryData'").WithLocation().Build(

		// validateSecret validates Secret-specific fields
		)
	}

	return nil
}

func (v *ManifestValidator) validateSecret(doc map[string]interface{}) error {
	// Check type
	secretType, ok := doc["type"].(string)
	if !ok {
		return errors.NewError().Messagef("missing required field: type").WithLocation(

		// Validate known secret types
		).Build()
	}

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
		v.logger.Warn("Unknown secret type", "type", secretType)
	}

	return nil
}

// validateIngress validates Ingress-specific fields
func (v *ManifestValidator) validateIngress(doc map[string]interface{}) error {
	spec, ok := doc["spec"].(map[string]interface{})
	if !ok {
		return errors.NewError().Messagef("missing required field: spec").WithLocation(

		// Check rules
		).Build()
	}

	rules, ok := spec["rules"].([]interface{})
	if !ok || len(rules) == 0 {
		return errors.NewError().Messagef("at least one rule is required").WithLocation().Build(

		// validatePVC validates PersistentVolumeClaim-specific fields
		)
	}

	return nil
}

func (v *ManifestValidator) validatePVC(doc map[string]interface{}) error {
	spec, ok := doc["spec"].(map[string]interface{})
	if !ok {
		return errors.NewError().Messagef("missing required field: spec").WithLocation(

		// Check accessModes
		).Build()
	}

	if _, ok := spec["accessModes"]; !ok {
		return errors.NewError().Messagef("missing required field: spec.accessModes").WithLocation(

		// Check resources
		).Build()
	}

	if _, ok := spec["resources"]; !ok {
		return errors.NewError().Messagef("missing required field: spec.resources").WithLocation().Build(

		// ===============================
		// Typed Validation Methods (New Type-Safe Approach)
		// ===============================
		)
	}

	return nil
}

// validateDeploymentTypedSpec validates Deployment using typed structures
func (v *ManifestValidator) validateDeploymentTypedSpec(typedDoc *tools.TypedValidationDocument) error {
	if len(typedDoc.Spec) == 0 {
		return errors.NewError().Messagef("missing required field: spec").WithLocation(

		// Parse spec into typed deployment spec
		).Build()
	}

	var spec tools.TypedValidationSpec
	if err := json.Unmarshal(typedDoc.Spec, &spec); err != nil {
		return errors.NewError().Message("invalid deployment spec format").Cause(err).WithLocation(

		// Validate replicas with type safety
		).Build()
	}

	if spec.Replicas != nil && *spec.Replicas < 0 {
		return errors.NewError().Messagef("invalid replicas count: %d", *spec.Replicas).WithLocation(

		// Validate selector
		).Build()
	}

	if spec.Selector == nil {
		return errors.NewError().Messagef("missing required field: spec.selector").WithLocation(

		// Validate template
		).Build()
	}

	if spec.Template == nil {
		return errors.NewError().Messagef("missing required field: spec.template").WithLocation().Build()
	}

	if spec.Template.Spec == nil {
		return errors.NewError().Messagef("missing required field: spec.template.spec").WithLocation(

		// Validate containers
		).Build()
	}

	if len(spec.Template.Spec.Containers) == 0 {
		return errors.NewError().Messagef("at least one container is required").WithLocation(

		// Validate each container with type safety
		).Build()
	}

	for i, container := range spec.Template.Spec.Containers {
		if err := v.validateTypedContainer(container, fmt.Sprintf("spec.template.spec.containers[%d]", i)); err != nil {
			return err
		}
	}

	return nil
}

// validateServiceTypedSpec validates Service using typed structures
func (v *ManifestValidator) validateServiceTypedSpec(typedDoc *tools.TypedValidationDocument) error {
	if len(typedDoc.Spec) == 0 {
		return errors.NewError().Messagef("missing required field: spec").WithLocation(

		// Parse spec into typed service spec
		).Build()
	}

	var spec tools.TypedValidationSpec
	if err := json.Unmarshal(typedDoc.Spec, &spec); err != nil {
		return errors.NewError().Message("invalid service spec format").Cause(err).WithLocation(

		// Validate ports
		).Build()
	}

	if len(spec.Ports) == 0 {
		return errors.NewError().Messagef("at least one port is required").WithLocation(

		// Validate each port with type safety
		).Build()
	}

	for i, port := range spec.Ports {
		if err := v.validatePortTyped(port, fmt.Sprintf("spec.ports[%d]", i)); err != nil {
			return err
		}
	}

	// Validate service type
	if spec.Type != "" {
		validTypes := []string{"ClusterIP", "NodePort", "LoadBalancer", "ExternalName"}
		isValid := false
		for _, validType := range validTypes {
			if spec.Type == validType {
				isValid = true
				break
			}
		}
		if !isValid {
			return errors.NewError().Messagef("invalid service type: %s", spec.Type).WithLocation().Build()
		}
	}

	return nil
}

// validateConfigMapTypedSpec validates ConfigMap using typed structures
func (v *ManifestValidator) validateConfigMapTypedSpec(typedDoc *tools.TypedValidationDocument) error {
	// ConfigMaps must have either data or stringData
	hasData := len(typedDoc.Data) > 0
	hasStringData := len(typedDoc.StringData) > 0
	hasBinaryData := len(typedDoc.BinaryData) > 0

	if !hasData && !hasStringData && !hasBinaryData {
		return errors.NewError().Messagef("ConfigMap must have either 'data', 'stringData', or 'binaryData'").WithLocation().Build(

		// validateSecretTypedSpec validates Secret using typed structures
		)
	}

	return nil
}

func (v *ManifestValidator) validateSecretTypedSpec(typedDoc *tools.TypedValidationDocument) error {
	// Secrets should have type field
	var secretData struct {
		Type string `json:"type"`
	}

	// Parse any raw fields for type
	for k, v := range typedDoc.RawFields {
		if k == "type" {
			if err := json.Unmarshal(v, &secretData.Type); err == nil {
				break
			}
		}
	}

	if secretData.Type == "" {
		v.logger.Warn("Secret missing type field")
	} else {
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
			if secretData.Type == validType {
				isValidType = true
				break
			}
		}

		if !isValidType {
			v.logger.Warn("Unknown secret type", "type", secretData.Type)
		}
	}

	return nil
}

// validateTypedContainer validates a container with type safety using new typed structures
func (v *ManifestValidator) validateTypedContainer(container tools.TypedValidationContainer, path string) error {
	if container.Name == "" {
		return errors.NewError().Messagef("%s: missing required field: name", path).WithLocation().Build()
	}

	if container.Image == "" {
		return errors.NewError().Messagef("%s: missing required field: image", path).WithLocation(

		// Validate image format (basic check)
		).Build()
	}

	if !v.isValidImageReference(container.Image) {
		return errors.NewError().Messagef("%s: invalid image reference: %s", path, container.Image).WithLocation(

		// Validate ports
		).Build()
	}

	for i, port := range container.Ports {
		if port.ContainerPort <= 0 || port.ContainerPort > 65535 {
			return errors.NewError().Messagef("%s.ports[%d]: invalid containerPort: %d", path, i, port.ContainerPort).WithLocation(

			// Validate environment variables
			).Build()
		}
	}

	for i, env := range container.Env {
		if env.Name == "" {
			return errors.NewError().Messagef("%s.env[%d]: missing required field: name", path, i).WithLocation().Build()
		}
	}

	return nil
}

// validatePortTyped validates a service port with type safety
func (v *ManifestValidator) validatePortTyped(port tools.TypedValidationPort, path string) error {
	if port.Port <= 0 || port.Port > 65535 {
		return errors.NewError().Messagef("%s: invalid port: %d", path, port.Port).WithLocation(

		// Validate protocol
		).Build()
	}

	if port.Protocol != "" {
		validProtocols := []string{"TCP", "UDP", "SCTP"}
		isValid := false
		for _, validProtocol := range validProtocols {
			if port.Protocol == validProtocol {
				isValid = true
				break
			}
		}
		if !isValid {
			return errors.NewError().Messagef("%s: invalid protocol: %s", path, port.Protocol).WithLocation().Build()
		}
	}

	return nil
}

// isValidImageReference performs basic validation of container image reference
func (v *ManifestValidator) isValidImageReference(image string) bool {
	if image == "" {
		return false
	}

	// Basic checks for image format
	// Should contain at least image name, optionally registry/namespace/name:tag
	parts := strings.Split(image, "/")
	if len(parts) == 0 {
		return false
	}

	// Last part should be image name (potentially with tag)
	lastPart := parts[len(parts)-1]
	if lastPart == "" {
		return false
	}

	// Check for invalid characters (basic validation)
	if strings.Contains(image, " ") {
		return false
	}

	return true
}
