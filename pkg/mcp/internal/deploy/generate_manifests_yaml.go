package deploy

import (
	"fmt"
	"strings"

	corek8s "github.com/Azure/container-copilot/pkg/core/kubernetes"
)

// generateConfigMapManifest generates a ConfigMap for non-sensitive environment variables
func (t *AtomicGenerateManifestsTool) generateConfigMapManifest(manifestResult *corek8s.ManifestGenerationResult, args AtomicGenerateManifestsArgs) error {
	if len(args.Environment) == 0 {
		t.logger.Debug().Msg("No environment variables to create ConfigMap")
		return nil
	}

	// Create ConfigMap data with non-sensitive environment variables
	configMapData := make(map[string]string)
	for key, value := range args.Environment {
		// Only include non-sensitive variables (avoid secrets, passwords, keys, etc.)
		if !t.isSensitiveKey(key) {
			configMapData[key] = value
		}
	}

	if len(configMapData) == 0 {
		t.logger.Debug().Msg("No non-sensitive environment variables for ConfigMap")
		return nil
	}

	// Generate ConfigMap YAML
	configMapName := fmt.Sprintf("%s-config", args.AppName)
	configMapYAML := t.generateConfigMapYAML(configMapName, args.Namespace, configMapData, args.AppName)

	// Add to manifest result
	configMapManifest := corek8s.GeneratedManifest{
		Name:    configMapName,
		Kind:    "ConfigMap",
		Path:    fmt.Sprintf("k8s/%s-configmap.yaml", args.AppName),
		Content: configMapYAML,
		Size:    len(configMapYAML),
		Valid:   true,
	}
	manifestResult.Manifests = append(manifestResult.Manifests, configMapManifest)

	t.logger.Info().
		Str("configmap_name", configMapName).
		Int("data_entries", len(configMapData)).
		Msg("Generated ConfigMap manifest")

	return nil
}

// generateIngressManifest generates an Ingress resource for the application
func (t *AtomicGenerateManifestsTool) generateIngressManifest(manifestResult *corek8s.ManifestGenerationResult, args AtomicGenerateManifestsArgs) error {
	ingressName := fmt.Sprintf("%s-ingress", args.AppName)
	serviceName := fmt.Sprintf("%s-service", args.AppName)

	// Use provided port or default to 8080
	port := args.Port
	if port == 0 {
		port = 8080
	}

	// Generate Ingress YAML
	ingressYAML := t.generateIngressYAML(ingressName, args.Namespace, serviceName, port, args.AppName)

	// Add to manifest result
	ingressManifest := corek8s.GeneratedManifest{
		Name:    ingressName,
		Kind:    "Ingress",
		Path:    fmt.Sprintf("k8s/%s-ingress.yaml", args.AppName),
		Content: ingressYAML,
		Size:    len(ingressYAML),
		Valid:   true,
	}
	manifestResult.Manifests = append(manifestResult.Manifests, ingressManifest)

	t.logger.Info().
		Str("ingress_name", ingressName).
		Str("service_name", serviceName).
		Int("port", port).
		Msg("Generated Ingress manifest")

	return nil
}

// applyResourceLimits applies CPU and memory limits to existing manifests
func (t *AtomicGenerateManifestsTool) applyResourceLimits(manifestResult *corek8s.ManifestGenerationResult, args AtomicGenerateManifestsArgs) error {
	if args.CPURequest == "" && args.MemoryRequest == "" && args.CPULimit == "" && args.MemoryLimit == "" {
		t.logger.Debug().Msg("No resource limits specified")
		return nil
	}

	// Find and modify the deployment manifest to add resource specifications
	for i, manifest := range manifestResult.Manifests {
		if manifest.Kind == "Deployment" {
			// Add resource limits to the deployment YAML
			updatedContent, err := t.addResourceLimitsToDeployment(manifest.Content, args)
			if err != nil {
				t.logger.Error().Err(err).Msg("Failed to add resource limits to deployment")
				return fmt.Errorf("failed to add resource limits: %w", err)
			}
			
			// Update the manifest with resource limits
			manifestResult.Manifests[i].Content = updatedContent
			manifestResult.Manifests[i].Size = len(updatedContent)
			
			t.logger.Info().
				Str("cpu_request", args.CPURequest).
				Str("memory_request", args.MemoryRequest).
				Str("cpu_limit", args.CPULimit).
				Str("memory_limit", args.MemoryLimit).
				Str("deployment_name", manifest.Name).
				Msg("Applied resource limits to deployment manifest")
			
			return nil
		}
	}

	t.logger.Warn().Msg("No deployment manifest found to apply resource limits")
	return nil
}

// isSensitiveKey checks if an environment variable key indicates sensitive data
func (t *AtomicGenerateManifestsTool) isSensitiveKey(key string) bool {
	keyLower := strings.ToLower(key)
	sensitivePatterns := []string{
		"password", "passwd", "pwd",
		"secret", "key", "token",
		"api_key", "apikey", "auth",
		"credential", "cred",
		"private", "cert", "certificate",
		"oauth", "bearer",
	}

	for _, pattern := range sensitivePatterns {
		if strings.Contains(keyLower, pattern) {
			return true
		}
	}
	return false
}

// generateConfigMapYAML creates ConfigMap YAML content
func (t *AtomicGenerateManifestsTool) generateConfigMapYAML(name, namespace string, data map[string]string, appName string) string {
	yaml := fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: %s
  namespace: %s
  labels:
    app: %s
    component: config
data:
`, name, namespace, appName)

	for key, value := range data {
		yaml += fmt.Sprintf("  %s: %q\n", key, value)
	}

	yaml += `
# Instructions:
# 1. This ConfigMap contains non-sensitive environment variables
# 2. Reference in your deployment using:
#    envFrom:
#    - configMapRef:
#        name: ` + name + `
# 3. Or reference individual keys using:
#    env:
#    - name: KEY_NAME
#      valueFrom:
#        configMapKeyRef:
#          name: ` + name + `
#          key: KEY_NAME
`

	return yaml
}

// generateIngressYAML creates Ingress YAML content
func (t *AtomicGenerateManifestsTool) generateIngressYAML(name, namespace, serviceName string, port int, appName string) string {
	host := fmt.Sprintf("%s.example.com", appName)

	yaml := fmt.Sprintf(`apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: %s
  namespace: %s
  labels:
    app: %s
    component: ingress
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
    # Add more annotations as needed for your ingress controller
spec:
  rules:
  - host: %s
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: %s
            port:
              number: %d
  # Uncomment for TLS support:
  # tls:
  # - hosts:
  #   - %s
  #   secretName: %s-tls

# Instructions:
# 1. Update the host (%s) to your actual domain
# 2. Configure your DNS to point to your ingress controller
# 3. For TLS, uncomment the tls section and create a TLS secret
# 4. Adjust annotations for your specific ingress controller
# 5. Apply: kubectl apply -f this-file.yaml
`, name, namespace, appName, host, serviceName, port, host, name, host)

	return yaml
}

// addResourceLimitsToDeployment adds resource limits to a deployment YAML
func (t *AtomicGenerateManifestsTool) addResourceLimitsToDeployment(deploymentYAML string, args AtomicGenerateManifestsArgs) (string, error) {
	// Build resource specification
	resourcesYAML := t.buildResourcesYAML(args)
	if resourcesYAML == "" {
		return deploymentYAML, nil // No resources to add
	}

	// Find the containers section and add resources
	lines := strings.Split(deploymentYAML, "\n")
	var result []string
	inContainers := false
	containerIndent := ""
	resourcesAdded := false

	for i, line := range lines {
		result = append(result, line)
		
		// Look for containers section
		if strings.Contains(line, "containers:") {
			inContainers = true
			continue
		}
		
		// If we're in containers and find a container definition
		if inContainers && strings.Contains(line, "- name:") {
			// Determine indentation
			containerIndent = strings.Repeat(" ", len(line)-len(strings.TrimLeft(line, " ")))
			
			// Look ahead to find the end of this container spec
			// Add resources before the next container or end of containers
			j := i + 1
			for j < len(lines) {
				nextLine := lines[j]
				if strings.TrimSpace(nextLine) == "" {
					j++
					continue
				}
				
				// If we hit another container or end of containers section, add resources here
				if strings.Contains(nextLine, "- name:") || 
				   (!strings.HasPrefix(nextLine, containerIndent) && strings.TrimSpace(nextLine) != "") {
					// Insert resources before this line
					resourceLines := strings.Split(resourcesYAML, "\n")
					for _, resLine := range resourceLines {
						if strings.TrimSpace(resLine) != "" {
							result = append(result, containerIndent+"  "+resLine)
						}
					}
					resourcesAdded = true
					break
				}
				j++
			}
			
			// If we reached the end without finding another container, add resources at the end
			if !resourcesAdded && j >= len(lines) {
				resourceLines := strings.Split(resourcesYAML, "\n")
				for _, resLine := range resourceLines {
					if strings.TrimSpace(resLine) != "" {
						result = append(result, containerIndent+"  "+resLine)
					}
				}
				resourcesAdded = true
			}
		}
	}

	return strings.Join(result, "\n"), nil
}

// buildResourcesYAML creates the resources YAML block
func (t *AtomicGenerateManifestsTool) buildResourcesYAML(args AtomicGenerateManifestsArgs) string {
	hasRequests := args.CPURequest != "" || args.MemoryRequest != ""
	hasLimits := args.CPULimit != "" || args.MemoryLimit != ""
	
	if !hasRequests && !hasLimits {
		return ""
	}
	
	var yaml strings.Builder
	yaml.WriteString("resources:\n")
	
	if hasRequests {
		yaml.WriteString("  requests:\n")
		if args.CPURequest != "" {
			yaml.WriteString(fmt.Sprintf("    cpu: %s\n", args.CPURequest))
		}
		if args.MemoryRequest != "" {
			yaml.WriteString(fmt.Sprintf("    memory: %s\n", args.MemoryRequest))
		}
	}
	
	if hasLimits {
		yaml.WriteString("  limits:\n")
		if args.CPULimit != "" {
			yaml.WriteString(fmt.Sprintf("    cpu: %s\n", args.CPULimit))
		}
		if args.MemoryLimit != "" {
			yaml.WriteString(fmt.Sprintf("    memory: %s\n", args.MemoryLimit))
		}
	}
	
	return yaml.String()
}
