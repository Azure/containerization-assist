package kubernetes

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
)

// IngressCustomizer handles Kubernetes ingress customization
type IngressCustomizer struct {
	logger zerolog.Logger
}

// NewIngressCustomizer creates a new ingress customizer
func NewIngressCustomizer(logger zerolog.Logger) *IngressCustomizer {
	return &IngressCustomizer{
		logger: logger.With().Str("customizer", "k8s_ingress").Logger(),
	}
}

// IngressCustomizationOptions contains options for customizing an ingress
type IngressCustomizationOptions struct {
	IngressHosts []IngressHost
	IngressTLS   []IngressTLS
	IngressClass string
	Namespace    string
	Labels       map[string]string
	Annotations  map[string]string
}

// IngressHost represents ingress host configuration
type IngressHost struct {
	Host  string        `json:"host"`
	Paths []IngressPath `json:"paths"`
}

// IngressPath represents a path configuration for an ingress host
type IngressPath struct {
	Path        string `json:"path"`
	PathType    string `json:"path_type,omitempty"`
	ServiceName string `json:"service_name"`
	ServicePort int    `json:"service_port"`
}

// IngressTLS represents TLS configuration for ingress
type IngressTLS struct {
	Hosts      []string `json:"hosts"`
	SecretName string   `json:"secret_name"`
}

// CustomizeIngress customizes a Kubernetes ingress manifest
func (c *IngressCustomizer) CustomizeIngress(ingressPath string, opts IngressCustomizationOptions) error {
	content, err := os.ReadFile(ingressPath)
	if err != nil {
		return fmt.Errorf("reading ingress manifest: %w", err)
	}

	var ingress map[string]interface{}
	if err := yaml.Unmarshal(content, &ingress); err != nil {
		return fmt.Errorf("parsing ingress YAML: %w", err)
	}

	// Update ingress class if specified
	if opts.IngressClass != "" {
		if err := updateNestedValue(ingress, opts.IngressClass, "spec", "ingressClassName"); err != nil {
			return fmt.Errorf("updating ingress class: %w", err)
		}
		c.logger.Debug().Str("class", opts.IngressClass).Msg("Updated ingress class")
	}

	// Update ingress hosts and paths
	if len(opts.IngressHosts) > 0 {
		if err := c.updateIngressRules(ingress, opts.IngressHosts); err != nil {
			return fmt.Errorf("updating ingress rules: %w", err)
		}
	}

	// Update TLS configuration
	if len(opts.IngressTLS) > 0 {
		if err := c.updateIngressTLS(ingress, opts.IngressTLS); err != nil {
			return fmt.Errorf("updating ingress TLS: %w", err)
		}
	}

	// Update namespace
	if opts.Namespace != "" {
		if err := updateNestedValue(ingress, opts.Namespace, "metadata", "namespace"); err != nil {
			return fmt.Errorf("updating namespace: %w", err)
		}
	}

	// Update labels
	if len(opts.Labels) > 0 {
		if err := updateLabelsInManifest(ingress, opts.Labels); err != nil {
			return fmt.Errorf("updating labels: %w", err)
		}
	}

	// Update annotations
	if len(opts.Annotations) > 0 {
		if err := c.updateAnnotations(ingress, opts.Annotations); err != nil {
			return fmt.Errorf("updating annotations: %w", err)
		}
	}

	// Write the updated ingress back to file
	updatedContent, err := yaml.Marshal(ingress)
	if err != nil {
		return fmt.Errorf("marshaling updated ingress YAML: %w", err)
	}

	if err := os.WriteFile(ingressPath, updatedContent, 0644); err != nil {
		return fmt.Errorf("writing updated ingress manifest: %w", err)
	}

	c.logger.Debug().
		Str("ingress_path", ingressPath).
		Msg("Successfully customized ingress manifest")

	return nil
}

// updateIngressRules updates the rules in an ingress manifest
func (c *IngressCustomizer) updateIngressRules(ingress map[string]interface{}, hosts []IngressHost) error {
	rules := make([]interface{}, len(hosts))

	for i, host := range hosts {
		rule := map[string]interface{}{
			"host": host.Host,
		}

		paths := make([]interface{}, len(host.Paths))
		for j, path := range host.Paths {
			pathConfig := map[string]interface{}{
				"path": path.Path,
				"backend": map[string]interface{}{
					"service": map[string]interface{}{
						"name": path.ServiceName,
						"port": map[string]interface{}{
							"number": path.ServicePort,
						},
					},
				},
			}

			if path.PathType != "" {
				pathConfig["pathType"] = path.PathType
			} else {
				pathConfig["pathType"] = "Prefix" // Default path type
			}

			paths[j] = pathConfig
		}

		rule["http"] = map[string]interface{}{
			"paths": paths,
		}

		rules[i] = rule
	}

	if err := updateNestedValue(ingress, rules, "spec", "rules"); err != nil {
		return fmt.Errorf("updating ingress rules: %w", err)
	}

	c.logger.Debug().
		Int("host_count", len(hosts)).
		Msg("Updated ingress rules")

	return nil
}

// updateIngressTLS updates the TLS configuration in an ingress manifest
func (c *IngressCustomizer) updateIngressTLS(ingress map[string]interface{}, tlsConfigs []IngressTLS) error {
	tls := make([]interface{}, len(tlsConfigs))

	for i, tlsConfig := range tlsConfigs {
		tlsEntry := map[string]interface{}{
			"hosts":      tlsConfig.Hosts,
			"secretName": tlsConfig.SecretName,
		}
		tls[i] = tlsEntry
	}

	if err := updateNestedValue(ingress, tls, "spec", "tls"); err != nil {
		return fmt.Errorf("updating TLS configuration: %w", err)
	}

	c.logger.Debug().
		Int("tls_count", len(tlsConfigs)).
		Msg("Updated ingress TLS configuration")

	return nil
}

// updateAnnotations updates annotations in a manifest
func (c *IngressCustomizer) updateAnnotations(manifest map[string]interface{}, annotations map[string]string) error {
	if len(annotations) == 0 {
		return nil
	}

	// Get existing metadata
	metadata, exists := manifest["metadata"]
	if !exists {
		metadata = make(map[string]interface{})
		manifest["metadata"] = metadata
	}

	metadataMap, ok := metadata.(map[string]interface{})
	if !ok {
		return fmt.Errorf("metadata is not a map")
	}

	// Get existing annotations
	existingAnnotations, exists := metadataMap["annotations"]
	if !exists {
		existingAnnotations = make(map[string]interface{})
		metadataMap["annotations"] = existingAnnotations
	}

	annotationsMap, ok := existingAnnotations.(map[string]interface{})
	if !ok {
		annotationsMap = make(map[string]interface{})
		metadataMap["annotations"] = annotationsMap
	}

	// Add new annotations
	for k, v := range annotations {
		annotationsMap[k] = v
	}

	return nil
}
