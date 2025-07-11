package kubernetes

import (
	"fmt"
	"os"

	mcperrors "github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
)

// ManifestCustomizer provides operations for customizing Kubernetes manifests
type ManifestCustomizer struct {
	logger zerolog.Logger
}

// NewManifestCustomizer creates a new manifest customizer
func NewManifestCustomizer(logger zerolog.Logger) *ManifestCustomizer {
	return &ManifestCustomizer{
		logger: logger,
	}
}

// CustomizeOptions contains options for customizing manifests
type CustomizeOptions struct {
	ImageRef    string            `json:"image_ref"`
	AppName     string            `json:"app_name,omitempty"`
	Namespace   string            `json:"namespace,omitempty"`
	Replicas    int               `json:"replicas,omitempty"`
	Port        int               `json:"port,omitempty"`
	ServiceType string            `json:"service_type,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	EnvVars     map[string]string `json:"env_vars,omitempty"`
}

// CustomizeDeployment updates a deployment manifest with the provided options
func (mc *ManifestCustomizer) CustomizeDeployment(manifestPath string, options CustomizeOptions) error {
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return mcperrors.New(mcperrors.CodeDeploymentFailed, "core", "reading deployment manifest", err)
	}

	var deployment map[string]interface{}
	if err := yaml.Unmarshal(content, &deployment); err != nil {
		return mcperrors.New(mcperrors.CodeInternalError, "k8s", "parsing deployment YAML", err)
	}

	if options.ImageRef != "" {
		if err := mc.updateImageInDeployment(deployment, options.ImageRef); err != nil {
			return mcperrors.New(mcperrors.CodeDockerfileSyntaxError, "k8s", "updating image reference", err)
		}
	}

	if options.AppName != "" {
		if err := mc.updateAppNameInDeployment(deployment, options.AppName); err != nil {
			return mcperrors.New(mcperrors.CodeOperationFailed, "k8s", "updating app name", err)
		}
	}

	if options.Namespace != "" {
		if err := mc.updateNamespaceInManifest(deployment, options.Namespace); err != nil {
			return mcperrors.New(mcperrors.CodeOperationFailed, "k8s", "updating namespace", err)
		}
	}

	if options.Replicas > 0 {
		if err := mc.updateReplicasInDeployment(deployment, options.Replicas); err != nil {
			return mcperrors.New(mcperrors.CodeOperationFailed, "k8s", "updating replicas", err)
		}
	}

	if options.Port > 0 {
		if err := mc.updatePortInDeployment(deployment, options.Port); err != nil {
			return mcperrors.New(mcperrors.CodeOperationFailed, "k8s", "updating port", err)
		}
	}

	if len(options.Labels) > 0 {
		if err := mc.updateLabelsInDeployment(deployment, options.Labels); err != nil {
			return mcperrors.New(mcperrors.CodeOperationFailed, "k8s", "updating labels", err)
		}
	}

	if len(options.Annotations) > 0 {
		if err := mc.updateAnnotationsInDeployment(deployment, options.Annotations); err != nil {
			return mcperrors.New(mcperrors.CodeOperationFailed, "k8s", "updating annotations", err)
		}
	}

	if len(options.EnvVars) > 0 {
		if err := mc.updateEnvVarsInDeployment(deployment, options.EnvVars); err != nil {
			return mcperrors.New(mcperrors.CodeOperationFailed, "k8s", "updating environment variables", err)
		}
	}

	updatedContent, err := yaml.Marshal(deployment)
	if err != nil {
		return mcperrors.New(mcperrors.CodeDeploymentFailed, "core", "marshaling updated deployment YAML", err)
	}

	if err := os.WriteFile(manifestPath, updatedContent, 0644); err != nil {
		return mcperrors.New(mcperrors.CodeDeploymentFailed, "core", "writing updated deployment manifest", err)
	}

	mc.logger.Debug().
		Str("manifest_path", manifestPath).
		Str("image_ref", options.ImageRef).
		Msg("Successfully customized deployment manifest")

	return nil
}

// CustomizeService updates a service manifest with the provided options
func (mc *ManifestCustomizer) CustomizeService(manifestPath string, options CustomizeOptions) error {
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return mcperrors.New(mcperrors.CodeIoError, "core", "reading service manifest", err)
	}

	var service map[string]interface{}
	if err := yaml.Unmarshal(content, &service); err != nil {
		return mcperrors.New(mcperrors.CodeInternalError, "k8s", "parsing service YAML", err)
	}

	if options.AppName != "" {
		if err := mc.updateAppNameInService(service, options.AppName); err != nil {
			return mcperrors.New(mcperrors.CodeOperationFailed, "k8s", "updating app name", err)
		}
	}

	if options.Namespace != "" {
		if err := mc.updateNamespaceInManifest(service, options.Namespace); err != nil {
			return mcperrors.New(mcperrors.CodeOperationFailed, "k8s", "updating namespace", err)
		}
	}

	if options.ServiceType != "" {
		if err := mc.updateServiceType(service, options.ServiceType); err != nil {
			return mcperrors.New(mcperrors.CodeOperationFailed, "k8s", "updating service type", err)
		}
	}

	if options.Port > 0 {
		if err := mc.updatePortInService(service, options.Port); err != nil {
			return mcperrors.New(mcperrors.CodeOperationFailed, "k8s", "updating port", err)
		}
	}

	if len(options.Labels) > 0 {
		if err := mc.updateLabelsInService(service, options.Labels); err != nil {
			return mcperrors.New(mcperrors.CodeOperationFailed, "k8s", "updating labels", err)
		}
	}

	updatedContent, err := yaml.Marshal(service)
	if err != nil {
		return mcperrors.New(mcperrors.CodeInternalError, "core", "marshaling updated service YAML", err)
	}

	if err := os.WriteFile(manifestPath, updatedContent, 0644); err != nil {
		return mcperrors.New(mcperrors.CodeInternalError, "core", "writing updated service manifest", err)
	}

	mc.logger.Debug().
		Str("manifest_path", manifestPath).
		Str("service_type", options.ServiceType).
		Msg("Successfully customized service manifest")

	return nil
}

// Helper methods for updating specific fields in manifests

func (mc *ManifestCustomizer) updateImageInDeployment(deployment map[string]interface{}, imageRef string) error {
	return mc.updateNestedValue(deployment, imageRef, "spec", "template", "spec", "containers", 0, "image")
}

func (mc *ManifestCustomizer) updateAppNameInDeployment(deployment map[string]interface{}, appName string) error {
	// Update metadata.name
	if err := mc.updateNestedValue(deployment, appName, "metadata", "name"); err != nil {
		return err
	}

	// Update spec.selector.matchLabels.app
	if err := mc.updateNestedValue(deployment, appName, "spec", "selector", "matchLabels", "app"); err != nil {
		return err
	}

	// Update spec.template.metadata.labels.app
	return mc.updateNestedValue(deployment, appName, "spec", "template", "metadata", "labels", "app")
}

func (mc *ManifestCustomizer) updateAppNameInService(service map[string]interface{}, appName string) error {
	// Update metadata.name
	if err := mc.updateNestedValue(service, appName, "metadata", "name"); err != nil {
		return err
	}

	// Update spec.selector.app
	return mc.updateNestedValue(service, appName, "spec", "selector", "app")
}

func (mc *ManifestCustomizer) updateNamespaceInManifest(manifest map[string]interface{}, namespace string) error {
	return mc.updateNestedValue(manifest, namespace, "metadata", "namespace")
}

func (mc *ManifestCustomizer) updateReplicasInDeployment(deployment map[string]interface{}, replicas int) error {
	return mc.updateNestedValue(deployment, replicas, "spec", "replicas")
}

func (mc *ManifestCustomizer) updatePortInDeployment(deployment map[string]interface{}, port int) error {
	return mc.updateNestedValue(deployment, port, "spec", "template", "spec", "containers", 0, "ports", 0, "containerPort")
}

func (mc *ManifestCustomizer) updatePortInService(service map[string]interface{}, port int) error {
	// Update spec.ports[0].port
	if err := mc.updateNestedValue(service, port, "spec", "ports", 0, "port"); err != nil {
		return err
	}

	// Update spec.ports[0].targetPort
	return mc.updateNestedValue(service, port, "spec", "ports", 0, "targetPort")
}

func (mc *ManifestCustomizer) updateServiceType(service map[string]interface{}, serviceType string) error {
	return mc.updateNestedValue(service, serviceType, "spec", "type")
}

func (mc *ManifestCustomizer) updateLabelsInDeployment(deployment map[string]interface{}, labels map[string]string) error {
	// Update metadata.labels
	if err := mc.updateNestedMap(deployment, labels, "metadata", "labels"); err != nil {
		return err
	}

	// Update spec.template.metadata.labels
	return mc.updateNestedMap(deployment, labels, "spec", "template", "metadata", "labels")
}

func (mc *ManifestCustomizer) updateLabelsInService(service map[string]interface{}, labels map[string]string) error {
	return mc.updateNestedMap(service, labels, "metadata", "labels")
}

func (mc *ManifestCustomizer) updateAppNameInConfigMap(configMap map[string]interface{}, appName string) error {
	return mc.updateNestedValue(configMap, appName, "metadata", "name")
}

func (mc *ManifestCustomizer) updateLabelsInConfigMap(configMap map[string]interface{}, labels map[string]string) error {
	return mc.updateNestedMap(configMap, labels, "metadata", "labels")
}

func (mc *ManifestCustomizer) updateDataInConfigMap(configMap map[string]interface{}, data map[string]string) error {
	return mc.updateNestedMap(configMap, data, "data")
}

func (mc *ManifestCustomizer) updateAppNameInIngress(ingress map[string]interface{}, appName string) error {
	// Update metadata.name
	if err := mc.updateNestedValue(ingress, appName, "metadata", "name"); err != nil {
		return err
	}

	// Update spec.rules[0].http.paths[0].backend.service.name (assuming networking.k8s.io/v1)
	return mc.updateNestedValue(ingress, appName, "spec", "rules", 0, "http", "paths", 0, "backend", "service", "name")
}

func (mc *ManifestCustomizer) updateLabelsInIngress(ingress map[string]interface{}, labels map[string]string) error {
	return mc.updateNestedMap(ingress, labels, "metadata", "labels")
}

func (mc *ManifestCustomizer) updateAnnotationsInIngress(ingress map[string]interface{}, annotations map[string]string) error {
	return mc.updateNestedMap(ingress, annotations, "metadata", "annotations")
}

func (mc *ManifestCustomizer) updatePortInIngress(ingress map[string]interface{}, port int) error {
	// Update spec.rules[0].http.paths[0].backend.service.port.number (assuming networking.k8s.io/v1)
	return mc.updateNestedValue(ingress, port, "spec", "rules", 0, "http", "paths", 0, "backend", "service", "port", "number")
}

func (mc *ManifestCustomizer) updateAnnotationsInDeployment(deployment map[string]interface{}, annotations map[string]string) error {
	// Update metadata.annotations
	if err := mc.updateNestedMap(deployment, annotations, "metadata", "annotations"); err != nil {
		return err
	}

	// Update spec.template.metadata.annotations
	return mc.updateNestedMap(deployment, annotations, "spec", "template", "metadata", "annotations")
}

func (mc *ManifestCustomizer) updateEnvVarsInDeployment(deployment map[string]interface{}, envVars map[string]string) error {
	// Navigate to spec.template.spec.containers[0].env
	containers, err := mc.getNestedValue(deployment, "spec", "template", "spec", "containers")
	if err != nil {
		return err
	}

	containerSlice, ok := containers.([]interface{})
	if !ok || len(containerSlice) == 0 {
		return fmt.Errorf("no containers found in deployment")
	}

	container, ok := containerSlice[0].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid container structure")
	}

	// Create or update env array
	var envArray []interface{}
	if env, exists := container["env"]; exists {
		if envSlice, ok := env.([]interface{}); ok {
			envArray = envSlice
		}
	}

	// Add new environment variables
	for key, value := range envVars {
		envVar := map[string]interface{}{
			"name":  key,
			"value": value,
		}
		envArray = append(envArray, envVar)
	}

	container["env"] = envArray
	return nil
}

// CustomizeConfigMap updates a configmap manifest with the provided options
func (mc *ManifestCustomizer) CustomizeConfigMap(manifestPath string, options CustomizeOptions) error {
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return mcperrors.New(mcperrors.CodeIoError, "core", "reading configmap manifest", err)
	}

	var configMap map[string]interface{}
	if err := yaml.Unmarshal(content, &configMap); err != nil {
		return mcperrors.New(mcperrors.CodeInternalError, "k8s", "parsing configmap YAML", err)
	}

	if options.AppName != "" {
		if err := mc.updateAppNameInConfigMap(configMap, options.AppName); err != nil {
			return mcperrors.New(mcperrors.CodeOperationFailed, "k8s", "updating app name", err)
		}
	}

	if options.Namespace != "" {
		if err := mc.updateNamespaceInManifest(configMap, options.Namespace); err != nil {
			return mcperrors.New(mcperrors.CodeOperationFailed, "k8s", "updating namespace", err)
		}
	}

	if len(options.Labels) > 0 {
		if err := mc.updateLabelsInConfigMap(configMap, options.Labels); err != nil {
			return mcperrors.New(mcperrors.CodeOperationFailed, "k8s", "updating labels", err)
		}
	}

	if len(options.EnvVars) > 0 {
		if err := mc.updateDataInConfigMap(configMap, options.EnvVars); err != nil {
			return mcperrors.New(mcperrors.CodeOperationFailed, "k8s", "updating configmap data", err)
		}
	}

	updatedContent, err := yaml.Marshal(configMap)
	if err != nil {
		return mcperrors.New(mcperrors.CodeInternalError, "core", "marshaling updated configmap YAML", err)
	}

	if err := os.WriteFile(manifestPath, updatedContent, 0644); err != nil {
		return mcperrors.New(mcperrors.CodeInternalError, "core", "writing updated configmap manifest", err)
	}

	mc.logger.Debug().
		Str("manifest_path", manifestPath).
		Msg("Successfully customized configmap manifest")

	return nil
}

// CustomizeIngress updates an ingress manifest with the provided options
func (mc *ManifestCustomizer) CustomizeIngress(manifestPath string, options CustomizeOptions) error {
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return mcperrors.New(mcperrors.CodeIoError, "core", "reading ingress manifest", err)
	}

	var ingress map[string]interface{}
	if err := yaml.Unmarshal(content, &ingress); err != nil {
		return mcperrors.New(mcperrors.CodeInternalError, "k8s", "parsing ingress YAML", err)
	}

	if options.AppName != "" {
		if err := mc.updateAppNameInIngress(ingress, options.AppName); err != nil {
			return mcperrors.New(mcperrors.CodeOperationFailed, "k8s", "updating app name", err)
		}
	}

	if options.Namespace != "" {
		if err := mc.updateNamespaceInManifest(ingress, options.Namespace); err != nil {
			return mcperrors.New(mcperrors.CodeOperationFailed, "k8s", "updating namespace", err)
		}
	}

	if len(options.Labels) > 0 {
		if err := mc.updateLabelsInIngress(ingress, options.Labels); err != nil {
			return mcperrors.New(mcperrors.CodeOperationFailed, "k8s", "updating labels", err)
		}
	}

	if len(options.Annotations) > 0 {
		if err := mc.updateAnnotationsInIngress(ingress, options.Annotations); err != nil {
			return mcperrors.New(mcperrors.CodeOperationFailed, "k8s", "updating annotations", err)
		}
	}

	if options.Port > 0 {
		if err := mc.updatePortInIngress(ingress, options.Port); err != nil {
			return mcperrors.New(mcperrors.CodeOperationFailed, "k8s", "updating port", err)
		}
	}

	updatedContent, err := yaml.Marshal(ingress)
	if err != nil {
		return mcperrors.New(mcperrors.CodeInternalError, "core", "marshaling updated ingress YAML", err)
	}

	if err := os.WriteFile(manifestPath, updatedContent, 0644); err != nil {
		return mcperrors.New(mcperrors.CodeInternalError, "core", "writing updated ingress manifest", err)
	}

	mc.logger.Debug().
		Str("manifest_path", manifestPath).
		Msg("Successfully customized ingress manifest")

	return nil
}

// Generic helper methods for navigating and updating nested YAML structures

func (mc *ManifestCustomizer) updateNestedValue(obj interface{}, value interface{}, path ...interface{}) error {
	if len(path) == 0 {
		return fmt.Errorf("empty path provided")
	}

	current := obj
	for i, key := range path[:len(path)-1] {
		switch curr := current.(type) {
		case map[string]interface{}:
			keyStr, ok := key.(string)
			if !ok {
				return fmt.Errorf("non-string key at path index %d", i)
			}
			if next, exists := curr[keyStr]; exists {
				current = next
			} else {
				// Create missing intermediate maps
				newMap := make(map[string]interface{})
				curr[keyStr] = newMap
				current = newMap
			}
		case []interface{}:
			keyInt, ok := key.(int)
			if !ok {
				return fmt.Errorf("non-integer key for array at path index %d", i)
			}
			if keyInt < len(curr) {
				current = curr[keyInt]
			} else {
				return fmt.Errorf("array index %d out of bounds at path index %d", keyInt, i)
			}
		default:
			return fmt.Errorf("cannot navigate through non-map/non-array at path index %d", i)
		}
	}

	// Set the final value
	finalKey := path[len(path)-1]
	switch curr := current.(type) {
	case map[string]interface{}:
		keyStr, ok := finalKey.(string)
		if !ok {
			return fmt.Errorf("non-string final key")
		}
		curr[keyStr] = value
	case []interface{}:
		keyInt, ok := finalKey.(int)
		if !ok {
			return fmt.Errorf("non-integer final key for array")
		}
		if keyInt < len(curr) {
			curr[keyInt] = value
		} else {
			return fmt.Errorf("array index %d out of bounds for final key", keyInt)
		}
	default:
		return fmt.Errorf("cannot set value on non-map/non-array")
	}

	return nil
}

func (mc *ManifestCustomizer) updateNestedMap(obj interface{}, values map[string]string, path ...interface{}) error {
	if len(path) == 0 {
		return fmt.Errorf("empty path provided")
	}

	targetMap, err := mc.getNestedValue(obj, path...)
	if err != nil {
		// If the path doesn't exist, create it
		if err := mc.createNestedPath(obj, path...); err != nil {
			return err
		}
		targetMap, err = mc.getNestedValue(obj, path...)
		if err != nil {
			return err
		}
	}

	targetMapTyped, ok := targetMap.(map[string]interface{})
	if !ok {
		// Create new map if target is not a map
		newMap := make(map[string]interface{})
		if err := mc.updateNestedValue(obj, newMap, path...); err != nil {
			return err
		}
		targetMapTyped = newMap
	}

	// Update the map with new values
	for key, value := range values {
		targetMapTyped[key] = value
	}

	return nil
}

func (mc *ManifestCustomizer) getNestedValue(obj interface{}, path ...interface{}) (interface{}, error) {
	current := obj
	for i, key := range path {
		switch curr := current.(type) {
		case map[string]interface{}:
			keyStr, ok := key.(string)
			if !ok {
				return nil, fmt.Errorf("non-string key at path index %d", i)
			}
			if next, exists := curr[keyStr]; exists {
				current = next
			} else {
				return nil, fmt.Errorf("key %s not found at path index %d", keyStr, i)
			}
		case []interface{}:
			keyInt, ok := key.(int)
			if !ok {
				return nil, fmt.Errorf("non-integer key for array at path index %d", i)
			}
			if keyInt < len(curr) {
				current = curr[keyInt]
			} else {
				return nil, fmt.Errorf("array index %d out of bounds at path index %d", keyInt, i)
			}
		default:
			return nil, fmt.Errorf("cannot navigate through non-map/non-array at path index %d", i)
		}
	}
	return current, nil
}

func (mc *ManifestCustomizer) createNestedPath(obj interface{}, path ...interface{}) error {
	if len(path) == 0 {
		return nil
	}

	current := obj
	for i, key := range path[:len(path)-1] {
		switch curr := current.(type) {
		case map[string]interface{}:
			keyStr, ok := key.(string)
			if !ok {
				return fmt.Errorf("non-string key at path index %d", i)
			}
			if next, exists := curr[keyStr]; exists {
				current = next
			} else {
				newMap := make(map[string]interface{})
				curr[keyStr] = newMap
				current = newMap
			}
		default:
			return fmt.Errorf("cannot create path through non-map at path index %d", i)
		}
	}

	// Create the final key
	finalKey := path[len(path)-1]
	if currMap, ok := current.(map[string]interface{}); ok {
		if keyStr, ok := finalKey.(string); ok {
			if _, exists := currMap[keyStr]; !exists {
				currMap[keyStr] = make(map[string]interface{})
			}
		}
	}

	return nil
}
