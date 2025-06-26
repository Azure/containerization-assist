package customizer

import (
	"github.com/Azure/container-kit/pkg/core/kubernetes"
	"github.com/rs/zerolog"
)

// DeploymentCustomizer handles Kubernetes deployment customization
type DeploymentCustomizer struct {
	coreCustomizer *kubernetes.ManifestCustomizer
	logger         zerolog.Logger
}

// NewDeploymentCustomizer creates a new deployment customizer
func NewDeploymentCustomizer(logger zerolog.Logger) *DeploymentCustomizer {
	return &DeploymentCustomizer{
		coreCustomizer: kubernetes.NewManifestCustomizer(logger),
		logger:         logger.With().Str("customizer", "k8s_deployment").Logger(),
	}
}

// CustomizeDeployment delegates to the core Kubernetes customizer
func (c *DeploymentCustomizer) CustomizeDeployment(deploymentPath string, opts kubernetes.CustomizeOptions) error {
	return c.coreCustomizer.CustomizeDeployment(deploymentPath, opts)
}
