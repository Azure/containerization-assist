package customizer

import (
	"github.com/Azure/container-copilot/pkg/core/kubernetes"
	"github.com/rs/zerolog"
)

// ConfigMapCustomizer handles Kubernetes configmap customization
type ConfigMapCustomizer struct {
	coreCustomizer *kubernetes.ManifestCustomizer
	logger         zerolog.Logger
}

// NewConfigMapCustomizer creates a new configmap customizer
func NewConfigMapCustomizer(logger zerolog.Logger) *ConfigMapCustomizer {
	return &ConfigMapCustomizer{
		coreCustomizer: kubernetes.NewManifestCustomizer(logger),
		logger:         logger.With().Str("customizer", "k8s_configmap").Logger(),
	}
}

// CustomizeConfigMap delegates to the core Kubernetes customizer
func (c *ConfigMapCustomizer) CustomizeConfigMap(configMapPath string, opts kubernetes.CustomizeOptions) error {
	return c.coreCustomizer.CustomizeConfigMap(configMapPath, opts)
}
