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

	// Resource limits would be applied to the deployment manifest
	// This is a placeholder for now - in a full implementation, you would parse and modify
	// the existing deployment YAML to add resource specifications

	t.logger.Info().
		Str("cpu_request", args.CPURequest).
		Str("memory_request", args.MemoryRequest).
		Str("cpu_limit", args.CPULimit).
		Str("memory_limit", args.MemoryLimit).
		Msg("Would apply resource limits to deployment (not yet implemented)")

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
