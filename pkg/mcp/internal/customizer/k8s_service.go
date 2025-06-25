package kubernetes

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
)

// ServiceCustomizer handles Kubernetes service customization
type ServiceCustomizer struct {
	logger zerolog.Logger
}

// NewServiceCustomizer creates a new service customizer
func NewServiceCustomizer(logger zerolog.Logger) *ServiceCustomizer {
	return &ServiceCustomizer{
		logger: logger.With().Str("customizer", "k8s_service").Logger(),
	}
}

// ServiceCustomizationOptions contains options for customizing a service
type ServiceCustomizationOptions struct {
	ServiceType     string
	ServicePorts    []ServicePort
	LoadBalancerIP  string
	SessionAffinity string
	Namespace       string
	Labels          map[string]string
}

// ServicePort represents a Kubernetes service port configuration
type ServicePort struct {
	Name       string `json:"name,omitempty"`
	Port       int    `json:"port"`
	TargetPort int    `json:"target_port,omitempty"`
	NodePort   int    `json:"node_port,omitempty"`
	Protocol   string `json:"protocol,omitempty"`
}

// CustomizeService customizes a Kubernetes service manifest
func (c *ServiceCustomizer) CustomizeService(servicePath string, opts ServiceCustomizationOptions) error {
	content, err := os.ReadFile(servicePath)
	if err != nil {
		return fmt.Errorf("reading service manifest: %w", err)
	}

	var service map[string]interface{}
	if err := yaml.Unmarshal(content, &service); err != nil {
		return fmt.Errorf("parsing service YAML: %w", err)
	}

	// Update service spec
	spec, exists := service["spec"]
	if !exists {
		spec = make(map[string]interface{})
		service["spec"] = spec
	}

	specMap, ok := spec.(map[string]interface{})
	if !ok {
		return fmt.Errorf("service spec is not a map")
	}

	// Update service type
	if opts.ServiceType != "" {
		if err := updateNestedValue(service, opts.ServiceType, "spec", "type"); err != nil {
			return fmt.Errorf("updating service type: %w", err)
		}
		c.logger.Debug().Str("type", opts.ServiceType).Msg("Updated service type")
	}

	// Update service ports
	if len(opts.ServicePorts) > 0 {
		if err := c.updateServicePorts(service, opts.ServicePorts); err != nil {
			return fmt.Errorf("updating service ports: %w", err)
		}
	}

	// Add LoadBalancer IP if specified
	if opts.LoadBalancerIP != "" && opts.ServiceType == "LoadBalancer" {
		specMap["loadBalancerIP"] = opts.LoadBalancerIP
		c.logger.Debug().Str("ip", opts.LoadBalancerIP).Msg("Added LoadBalancer IP")
	}

	// Add session affinity if specified
	if opts.SessionAffinity != "" {
		specMap["sessionAffinity"] = opts.SessionAffinity
		c.logger.Debug().Str("affinity", opts.SessionAffinity).Msg("Added session affinity")
	}

	// Update namespace
	if opts.Namespace != "" {
		if err := updateNestedValue(service, opts.Namespace, "metadata", "namespace"); err != nil {
			return fmt.Errorf("updating namespace: %w", err)
		}
	}

	// Update labels
	if len(opts.Labels) > 0 {
		if err := updateLabelsInManifest(service, opts.Labels); err != nil {
			return fmt.Errorf("updating labels: %w", err)
		}
	}

	// Write the updated service back to file
	updatedContent, err := yaml.Marshal(service)
	if err != nil {
		return fmt.Errorf("marshaling updated service YAML: %w", err)
	}

	if err := os.WriteFile(servicePath, updatedContent, 0644); err != nil {
		return fmt.Errorf("writing updated service manifest: %w", err)
	}

	c.logger.Debug().
		Str("service_path", servicePath).
		Msg("Successfully customized service manifest")

	return nil
}

// updateServicePorts updates the ports in a service manifest
func (c *ServiceCustomizer) updateServicePorts(service map[string]interface{}, servicePorts []ServicePort) error {
	ports := make([]interface{}, len(servicePorts))

	for i, sp := range servicePorts {
		port := map[string]interface{}{
			"port":       sp.Port,
			"targetPort": sp.TargetPort,
		}

		if sp.Name != "" {
			port["name"] = sp.Name
		}

		if sp.Protocol != "" {
			port["protocol"] = sp.Protocol
		} else {
			port["protocol"] = "TCP" // Default protocol
		}

		if sp.NodePort > 0 {
			port["nodePort"] = sp.NodePort
		}

		ports[i] = port
	}

	if err := updateNestedValue(service, ports, "spec", "ports"); err != nil {
		return fmt.Errorf("updating service ports: %w", err)
	}

	c.logger.Debug().
		Int("port_count", len(servicePorts)).
		Msg("Updated service ports")

	return nil
}
