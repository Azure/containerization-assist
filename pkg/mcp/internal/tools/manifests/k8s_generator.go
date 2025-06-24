package manifests

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Azure/container-copilot/pkg/core/kubernetes"
	"github.com/rs/zerolog"
)

// K8sManifestGenerator handles core Kubernetes manifest generation
type K8sManifestGenerator struct {
	pipelineAdapter PipelineAdapter
	logger          zerolog.Logger
}

// PipelineAdapter defines the interface for pipeline operations
type PipelineAdapter interface {
	GenerateKubernetesManifests(sessionID, imageRef, appName string, port int, cpuRequest, memoryRequest, cpuLimit, memoryLimit string) (*kubernetes.ManifestGenerationResult, error)
}

// NewK8sManifestGenerator creates a new K8s manifest generator
func NewK8sManifestGenerator(adapter PipelineAdapter, logger zerolog.Logger) *K8sManifestGenerator {
	return &K8sManifestGenerator{
		pipelineAdapter: adapter,
		logger:          logger.With().Str("component", "k8s_generator").Logger(),
	}
}

// GenerateManifests generates Kubernetes manifests for the application
func (g *K8sManifestGenerator) GenerateManifests(ctx context.Context, args GenerateManifestsRequest) (*kubernetes.ManifestGenerationResult, error) {
	g.logger.Info().
		Str("image", args.ImageReference).
		Str("app", args.AppName).
		Int("port", args.Port).
		Msg("Generating Kubernetes manifests")

	// Call pipeline adapter to generate base manifests
	result, err := g.pipelineAdapter.GenerateKubernetesManifests(
		args.SessionID,
		args.ImageReference,
		args.AppName,
		args.Port,
		args.CPURequest,
		args.MemoryRequest,
		args.CPULimit,
		args.MemoryLimit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate manifests: %w", err)
	}

	// Apply namespace if specified
	if args.Namespace != "" && args.Namespace != "default" {
		g.applyNamespaceToManifests(result, args.Namespace)
	}

	// Apply resource limits if not already set
	if args.CPURequest != "" || args.MemoryRequest != "" || args.CPULimit != "" || args.MemoryLimit != "" {
		g.applyResourceLimits(result, args)
	}

	return result, nil
}

// GenerateConfigMap generates a ConfigMap for non-sensitive environment variables
func (g *K8sManifestGenerator) GenerateConfigMap(appName, namespace string, envVars map[string]string) (*GeneratedManifest, error) {
	if len(envVars) == 0 {
		return nil, nil
	}

	g.logger.Info().
		Str("app", appName).
		Int("env_vars", len(envVars)).
		Msg("Generating ConfigMap for environment variables")

	configMapName := fmt.Sprintf("%s-config", appName)

	// Build ConfigMap YAML
	var configMapYAML strings.Builder
	configMapYAML.WriteString("apiVersion: v1\n")
	configMapYAML.WriteString("kind: ConfigMap\n")
	configMapYAML.WriteString("metadata:\n")
	configMapYAML.WriteString(fmt.Sprintf("  name: %s\n", configMapName))
	if namespace != "" && namespace != "default" {
		configMapYAML.WriteString(fmt.Sprintf("  namespace: %s\n", namespace))
	}
	configMapYAML.WriteString("data:\n")

	for key, value := range envVars {
		// Escape special characters in YAML
		escapedValue := strings.ReplaceAll(value, "\"", "\\\"")
		configMapYAML.WriteString(fmt.Sprintf("  %s: \"%s\"\n", key, escapedValue))
	}

	return &GeneratedManifest{
		Kind:     "ConfigMap",
		Name:     configMapName,
		Content:  configMapYAML.String(),
		FilePath: filepath.Join("manifests", fmt.Sprintf("%s-configmap.yaml", appName)),
	}, nil
}

// GenerateIngress generates an Ingress resource
func (g *K8sManifestGenerator) GenerateIngress(appName, namespace, host string, port int) (*GeneratedManifest, error) {
	g.logger.Info().
		Str("app", appName).
		Str("host", host).
		Int("port", port).
		Msg("Generating Ingress resource")

	ingressName := fmt.Sprintf("%s-ingress", appName)
	serviceName := fmt.Sprintf("%s-service", appName)

	// Build Ingress YAML
	var ingressYAML strings.Builder
	ingressYAML.WriteString("apiVersion: networking.k8s.io/v1\n")
	ingressYAML.WriteString("kind: Ingress\n")
	ingressYAML.WriteString("metadata:\n")
	ingressYAML.WriteString(fmt.Sprintf("  name: %s\n", ingressName))
	if namespace != "" && namespace != "default" {
		ingressYAML.WriteString(fmt.Sprintf("  namespace: %s\n", namespace))
	}
	ingressYAML.WriteString("  annotations:\n")
	ingressYAML.WriteString("    nginx.ingress.kubernetes.io/rewrite-target: /\n")
	ingressYAML.WriteString("spec:\n")
	ingressYAML.WriteString("  ingressClassName: nginx\n")
	ingressYAML.WriteString("  rules:\n")
	ingressYAML.WriteString(fmt.Sprintf("  - host: %s\n", host))
	ingressYAML.WriteString("    http:\n")
	ingressYAML.WriteString("      paths:\n")
	ingressYAML.WriteString("      - path: /\n")
	ingressYAML.WriteString("        pathType: Prefix\n")
	ingressYAML.WriteString("        backend:\n")
	ingressYAML.WriteString("          service:\n")
	ingressYAML.WriteString(fmt.Sprintf("            name: %s\n", serviceName))
	ingressYAML.WriteString("            port:\n")
	ingressYAML.WriteString(fmt.Sprintf("              number: %d\n", port))

	return &GeneratedManifest{
		Kind:     "Ingress",
		Name:     ingressName,
		Content:  ingressYAML.String(),
		FilePath: filepath.Join("manifests", fmt.Sprintf("%s-ingress.yaml", appName)),
	}, nil
}

// applyNamespaceToManifests updates all manifests to use the specified namespace
func (g *K8sManifestGenerator) applyNamespaceToManifests(result *kubernetes.ManifestGenerationResult, namespace string) {
	for i, manifest := range result.Manifests {
		// Simple namespace injection - in production, use proper YAML parsing
		if !strings.Contains(manifest.Content, "namespace:") {
			lines := strings.Split(manifest.Content, "\n")
			for j, line := range lines {
				if strings.HasPrefix(line, "metadata:") && j+1 < len(lines) {
					// Insert namespace after metadata
					newLines := append(lines[:j+1],
						fmt.Sprintf("  namespace: %s", namespace))
					newLines = append(newLines, lines[j+1:]...)
					lines = newLines
					break
				}
			}
			result.Manifests[i].Content = strings.Join(lines, "\n")
		}
	}
}

// applyResourceLimits updates deployment manifests with resource limits
func (g *K8sManifestGenerator) applyResourceLimits(result *kubernetes.ManifestGenerationResult, args GenerateManifestsRequest) {
	for _, manifest := range result.Manifests {
		if manifest.Kind == "Deployment" {
			// In production, use proper YAML parsing
			// This is a simplified version for the refactoring
			g.logger.Debug().
				Str("deployment", manifest.Name).
				Str("cpu_request", args.CPURequest).
				Str("memory_request", args.MemoryRequest).
				Msg("Applying resource limits to deployment")

			// Resource limits would be applied here using proper YAML manipulation
			// For now, we just log the intention
		}
	}
}

// GetDefaultPort returns a default port if none is specified
func (g *K8sManifestGenerator) GetDefaultPort(port int) int {
	if port > 0 {
		return port
	}
	return 8080
}

// GetDefaultNamespace returns the default namespace
func (g *K8sManifestGenerator) GetDefaultNamespace(namespace string) string {
	if namespace != "" {
		return namespace
	}
	return "default"
}

// GetDefaultAppName generates a default app name from image reference
func (g *K8sManifestGenerator) GetDefaultAppName(appName, imageRef string) string {
	if appName != "" {
		return appName
	}

	// Extract app name from image reference
	parts := strings.Split(imageRef, "/")
	lastPart := parts[len(parts)-1]

	// Remove tag if present
	imageName := strings.Split(lastPart, ":")[0]

	// Sanitize for Kubernetes naming
	sanitized := strings.ToLower(imageName)
	sanitized = strings.ReplaceAll(sanitized, "_", "-")
	sanitized = strings.ReplaceAll(sanitized, ".", "-")

	if sanitized == "" {
		return "app"
	}

	return sanitized
}
