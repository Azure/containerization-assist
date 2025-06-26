package customizer

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
)

// SecretCustomizer handles Kubernetes secret customization
type SecretCustomizer struct {
	logger zerolog.Logger
}

// NewSecretCustomizer creates a new secret customizer
func NewSecretCustomizer(logger zerolog.Logger) *SecretCustomizer {
	return &SecretCustomizer{
		logger: logger.With().Str("customizer", "k8s_secret").Logger(),
	}
}

// SecretCustomizationOptions contains options for customizing a secret
type SecretCustomizationOptions struct {
	Namespace string
	Labels    map[string]string
}

// CustomizeSecret customizes a Kubernetes secret manifest
func (c *SecretCustomizer) CustomizeSecret(secretPath string, opts SecretCustomizationOptions) error {
	content, err := os.ReadFile(secretPath)
	if err != nil {
		return fmt.Errorf("reading secret manifest: %w", err)
	}

	var secret map[string]interface{}
	if err := yaml.Unmarshal(content, &secret); err != nil {
		return fmt.Errorf("parsing secret YAML: %w", err)
	}

	// Update namespace
	if opts.Namespace != "" {
		if err := updateNestedValue(secret, opts.Namespace, "metadata", "namespace"); err != nil {
			return fmt.Errorf("updating namespace: %w", err)
		}
	}

	// Update labels with workflow labels
	if len(opts.Labels) > 0 {
		if err := updateLabelsInManifest(secret, opts.Labels); err != nil {
			return fmt.Errorf("updating workflow labels: %w", err)
		}
	}

	// Write the updated secret back to file
	updatedContent, err := yaml.Marshal(secret)
	if err != nil {
		return fmt.Errorf("marshaling updated secret YAML: %w", err)
	}

	if err := os.WriteFile(secretPath, updatedContent, 0644); err != nil {
		return fmt.Errorf("writing updated secret manifest: %w", err)
	}

	c.logger.Debug().
		Str("secret_path", secretPath).
		Msg("Successfully customized secret manifest")

	return nil
}
