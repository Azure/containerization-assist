package kubernetes

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
)

// NetworkPolicyCustomizer handles Kubernetes NetworkPolicy customization
type NetworkPolicyCustomizer struct {
	logger zerolog.Logger
}

// NewNetworkPolicyCustomizer creates a new NetworkPolicy customizer
func NewNetworkPolicyCustomizer(logger zerolog.Logger) *NetworkPolicyCustomizer {
	return &NetworkPolicyCustomizer{
		logger: logger.With().Str("customizer", "k8s_networkpolicy").Logger(),
	}
}

// NetworkPolicyCustomizationOptions contains options for customizing a NetworkPolicy
type NetworkPolicyCustomizationOptions struct {
	PolicyTypes []string
	PodSelector map[string]string
	Ingress     []NetworkPolicyIngressRule
	Egress      []NetworkPolicyEgressRule
	Namespace   string
	Labels      map[string]string
	Annotations map[string]string
}

// NetworkPolicyIngressRule represents an ingress rule for NetworkPolicy
type NetworkPolicyIngressRule struct {
	Ports []NetworkPolicyPortRule `json:"ports,omitempty"`
	From  []NetworkPolicyPeerRule `json:"from,omitempty"`
}

// NetworkPolicyEgressRule represents an egress rule for NetworkPolicy
type NetworkPolicyEgressRule struct {
	Ports []NetworkPolicyPortRule `json:"ports,omitempty"`
	To    []NetworkPolicyPeerRule `json:"to,omitempty"`
}

// NetworkPolicyPortRule represents a port rule in NetworkPolicy
type NetworkPolicyPortRule struct {
	Protocol string `json:"protocol,omitempty"`
	Port     string `json:"port,omitempty"`
	EndPort  *int   `json:"endPort,omitempty"`
}

// NetworkPolicyPeerRule represents a peer rule in NetworkPolicy
type NetworkPolicyPeerRule struct {
	PodSelector       map[string]string    `json:"podSelector,omitempty"`
	NamespaceSelector map[string]string    `json:"namespaceSelector,omitempty"`
	IPBlock           *NetworkPolicyIPBlock `json:"ipBlock,omitempty"`
}

// NetworkPolicyIPBlock represents an IP block in NetworkPolicy
type NetworkPolicyIPBlock struct {
	CIDR   string   `json:"cidr"`
	Except []string `json:"except,omitempty"`
}

// CustomizeNetworkPolicy customizes a NetworkPolicy YAML file with the provided options
func (nc *NetworkPolicyCustomizer) CustomizeNetworkPolicy(filePath string, options NetworkPolicyCustomizationOptions) error {
	nc.logger.Debug().
		Str("file_path", filePath).
		Interface("options", options).
		Msg("Customizing NetworkPolicy")

	// Read the existing YAML file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read NetworkPolicy file: %w", err)
	}

	// Parse the YAML
	var networkPolicy map[string]interface{}
	if err := yaml.Unmarshal(content, &networkPolicy); err != nil {
		return fmt.Errorf("failed to parse NetworkPolicy YAML: %w", err)
	}

	// Apply customizations
	if err := nc.applyCustomizations(&networkPolicy, options); err != nil {
		return fmt.Errorf("failed to apply NetworkPolicy customizations: %w", err)
	}

	// Write back the modified YAML
	updatedContent, err := yaml.Marshal(&networkPolicy)
	if err != nil {
		return fmt.Errorf("failed to marshal NetworkPolicy YAML: %w", err)
	}

	if err := os.WriteFile(filePath, updatedContent, 0644); err != nil {
		return fmt.Errorf("failed to write NetworkPolicy file: %w", err)
	}

	nc.logger.Info().
		Str("file_path", filePath).
		Msg("Successfully customized NetworkPolicy")

	return nil
}

// applyCustomizations applies the customization options to the NetworkPolicy
func (nc *NetworkPolicyCustomizer) applyCustomizations(networkPolicy *map[string]interface{}, options NetworkPolicyCustomizationOptions) error {
	np := *networkPolicy

	// Ensure spec exists
	if _, exists := np["spec"]; !exists {
		np["spec"] = make(map[string]interface{})
	}
	spec := np["spec"].(map[string]interface{})

	// Apply policy types
	if len(options.PolicyTypes) > 0 {
		spec["policyTypes"] = options.PolicyTypes
	}

	// Apply pod selector
	if len(options.PodSelector) > 0 {
		if _, exists := spec["podSelector"]; !exists {
			spec["podSelector"] = make(map[string]interface{})
		}
		podSelector := spec["podSelector"].(map[string]interface{})
		podSelector["matchLabels"] = options.PodSelector
	}

	// Apply ingress rules
	if len(options.Ingress) > 0 {
		ingressRules := make([]interface{}, len(options.Ingress))
		for i, rule := range options.Ingress {
			ingressRule := make(map[string]interface{})
			
			// Add ports
			if len(rule.Ports) > 0 {
				ports := make([]interface{}, len(rule.Ports))
				for j, port := range rule.Ports {
					portMap := make(map[string]interface{})
					if port.Protocol != "" {
						portMap["protocol"] = port.Protocol
					}
					if port.Port != "" {
						portMap["port"] = port.Port
					}
					if port.EndPort != nil {
						portMap["endPort"] = *port.EndPort
					}
					ports[j] = portMap
				}
				ingressRule["ports"] = ports
			}
			
			// Add from rules
			if len(rule.From) > 0 {
				fromRules := make([]interface{}, len(rule.From))
				for j, from := range rule.From {
					fromMap := make(map[string]interface{})
					if len(from.PodSelector) > 0 {
						fromMap["podSelector"] = map[string]interface{}{
							"matchLabels": from.PodSelector,
						}
					}
					if len(from.NamespaceSelector) > 0 {
						fromMap["namespaceSelector"] = map[string]interface{}{
							"matchLabels": from.NamespaceSelector,
						}
					}
					if from.IPBlock != nil {
						ipBlock := map[string]interface{}{
							"cidr": from.IPBlock.CIDR,
						}
						if len(from.IPBlock.Except) > 0 {
							ipBlock["except"] = from.IPBlock.Except
						}
						fromMap["ipBlock"] = ipBlock
					}
					fromRules[j] = fromMap
				}
				ingressRule["from"] = fromRules
			}
			
			ingressRules[i] = ingressRule
		}
		spec["ingress"] = ingressRules
	}

	// Apply egress rules
	if len(options.Egress) > 0 {
		egressRules := make([]interface{}, len(options.Egress))
		for i, rule := range options.Egress {
			egressRule := make(map[string]interface{})
			
			// Add ports
			if len(rule.Ports) > 0 {
				ports := make([]interface{}, len(rule.Ports))
				for j, port := range rule.Ports {
					portMap := make(map[string]interface{})
					if port.Protocol != "" {
						portMap["protocol"] = port.Protocol
					}
					if port.Port != "" {
						portMap["port"] = port.Port
					}
					if port.EndPort != nil {
						portMap["endPort"] = *port.EndPort
					}
					ports[j] = portMap
				}
				egressRule["ports"] = ports
			}
			
			// Add to rules
			if len(rule.To) > 0 {
				toRules := make([]interface{}, len(rule.To))
				for j, to := range rule.To {
					toMap := make(map[string]interface{})
					if len(to.PodSelector) > 0 {
						toMap["podSelector"] = map[string]interface{}{
							"matchLabels": to.PodSelector,
						}
					}
					if len(to.NamespaceSelector) > 0 {
						toMap["namespaceSelector"] = map[string]interface{}{
							"matchLabels": to.NamespaceSelector,
						}
					}
					if to.IPBlock != nil {
						ipBlock := map[string]interface{}{
							"cidr": to.IPBlock.CIDR,
						}
						if len(to.IPBlock.Except) > 0 {
							ipBlock["except"] = to.IPBlock.Except
						}
						toMap["ipBlock"] = ipBlock
					}
					toRules[j] = toMap
				}
				egressRule["to"] = toRules
			}
			
			egressRules[i] = egressRule
		}
		spec["egress"] = egressRules
	}

	// Apply labels to metadata
	if len(options.Labels) > 0 {
		nc.applyLabels(&np, options.Labels)
	}

	// Apply annotations to metadata
	if len(options.Annotations) > 0 {
		nc.applyAnnotations(&np, options.Annotations)
	}

	return nil
}

// applyLabels applies labels to the NetworkPolicy metadata
func (nc *NetworkPolicyCustomizer) applyLabels(networkPolicy *map[string]interface{}, labels map[string]string) {
	np := *networkPolicy
	
	if _, exists := np["metadata"]; !exists {
		np["metadata"] = make(map[string]interface{})
	}
	metadata := np["metadata"].(map[string]interface{})
	
	if _, exists := metadata["labels"]; !exists {
		metadata["labels"] = make(map[string]interface{})
	}
	metadataLabels := metadata["labels"].(map[string]interface{})
	
	for key, value := range labels {
		metadataLabels[key] = value
	}
}

// applyAnnotations applies annotations to the NetworkPolicy metadata
func (nc *NetworkPolicyCustomizer) applyAnnotations(networkPolicy *map[string]interface{}, annotations map[string]string) {
	np := *networkPolicy
	
	if _, exists := np["metadata"]; !exists {
		np["metadata"] = make(map[string]interface{})
	}
	metadata := np["metadata"].(map[string]interface{})
	
	if _, exists := metadata["annotations"]; !exists {
		metadata["annotations"] = make(map[string]interface{})
	}
	metadataAnnotations := metadata["annotations"].(map[string]interface{})
	
	for key, value := range annotations {
		metadataAnnotations[key] = value
	}
}