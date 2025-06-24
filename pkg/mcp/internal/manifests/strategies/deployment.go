package strategies

import (
	"fmt"
	"strings"

	manifests "github.com/Azure/container-copilot/pkg/mcp/internal/manifests"
	"gopkg.in/yaml.v3"
)

// DeploymentStrategy implements manifest generation for Kubernetes Deployments
type DeploymentStrategy struct{}

// NewDeploymentStrategy creates a new deployment strategy
func NewDeploymentStrategy() *DeploymentStrategy {
	return &DeploymentStrategy{}
}

// GenerateManifest generates a Kubernetes Deployment manifest
func (s *DeploymentStrategy) GenerateManifest(options manifests.GenerationOptions, context manifests.TemplateContext) ([]byte, error) {
	deployment := s.buildDeploymentManifest(options, context)
	return yaml.Marshal(deployment)
}

// GetManifestType returns the manifest type
func (s *DeploymentStrategy) GetManifestType() string {
	return "Deployment"
}

// ValidateOptions validates the deployment-specific options
func (s *DeploymentStrategy) ValidateOptions(options manifests.GenerationOptions) error {
	if options.ImageRef.String() == "" {
		return fmt.Errorf("image reference is required for deployment")
	}

	if options.Replicas < 0 {
		return fmt.Errorf("replicas must be non-negative")
	}

	return nil
}

// buildDeploymentManifest builds the deployment manifest structure
func (s *DeploymentStrategy) buildDeploymentManifest(options manifests.GenerationOptions, context manifests.TemplateContext) map[string]interface{} {
	// Set defaults
	replicas := options.Replicas
	if replicas == 0 {
		replicas = 1
	}

	namespace := options.Namespace
	if namespace == "" {
		namespace = "default"
	}

	appName := s.extractAppName(options.ImageRef.String())

	// Build labels
	labels := map[string]interface{}{
		"app": appName,
	}

	// Add workflow labels
	for k, v := range options.WorkflowLabels {
		labels[k] = v
	}

	// Build container spec
	container := map[string]interface{}{
		"name":  appName,
		"image": options.ImageRef.String(),
	}

	// Add environment variables
	if len(options.Environment) > 0 {
		env := make([]interface{}, 0, len(options.Environment))
		for k, v := range options.Environment {
			env = append(env, map[string]interface{}{
				"name":  k,
				"value": v,
			})
		}
		container["env"] = env
	}

	// Add resource requirements
	if s.hasResourceRequirements(options.Resources) {
		container["resources"] = s.buildResourceRequirements(options.Resources)
	}

	// Add ports if we can infer them
	if context.Port > 0 {
		container["ports"] = []interface{}{
			map[string]interface{}{
				"containerPort": context.Port,
				"protocol":      "TCP",
			},
		}
	}

	// Build the full deployment manifest
	deployment := map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"name":      appName,
			"namespace": namespace,
			"labels":    labels,
		},
		"spec": map[string]interface{}{
			"replicas": replicas,
			"selector": map[string]interface{}{
				"matchLabels": map[string]interface{}{
					"app": appName,
				},
			},
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{
						"app": appName,
					},
				},
				"spec": map[string]interface{}{
					"containers": []interface{}{container},
				},
			},
		},
	}

	return deployment
}

// extractAppName extracts application name from image reference
func (s *DeploymentStrategy) extractAppName(imageRef string) string {
	// Simple extraction - just take the last part after /
	parts := strings.Split(imageRef, "/")
	appName := parts[len(parts)-1]

	// Remove tag if present
	if colonIndex := strings.LastIndex(appName, ":"); colonIndex != -1 {
		appName = appName[:colonIndex]
	}

	// Remove digest if present
	if atIndex := strings.LastIndex(appName, "@"); atIndex != -1 {
		appName = appName[:atIndex]
	}

	return appName
}

// hasResourceRequirements checks if resource requirements are specified
func (s *DeploymentStrategy) hasResourceRequirements(resources manifests.ResourceRequests) bool {
	return resources.CPU != "" || resources.Memory != "" || resources.Storage != ""
}

// buildResourceRequirements builds the resource requirements structure
func (s *DeploymentStrategy) buildResourceRequirements(resources manifests.ResourceRequests) map[string]interface{} {
	resourceReq := make(map[string]interface{})

	if resources.CPU != "" || resources.Memory != "" {
		requests := make(map[string]interface{})
		limits := make(map[string]interface{})

		if resources.CPU != "" {
			requests["cpu"] = resources.CPU
			limits["cpu"] = resources.CPU
		}

		if resources.Memory != "" {
			requests["memory"] = resources.Memory
			limits["memory"] = resources.Memory
		}

		resourceReq["requests"] = requests
		resourceReq["limits"] = limits
	}

	return resourceReq
}
