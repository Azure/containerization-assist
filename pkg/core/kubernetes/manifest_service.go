// Package kubernetes provides core Kubernetes operations extracted from the Container Kit pipeline.
// This package contains mechanical K8s operations without AI dependencies,
// designed to be used by atomic MCP tools.
package kubernetes

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	validation "github.com/Azure/container-kit/pkg/mcp/domain/security"
	"sigs.k8s.io/yaml"
)

// ManifestService provides mechanical Kubernetes manifest operations
type ManifestService interface {
	// GenerateManifests generates Kubernetes manifests from templates
	GenerateManifests(ctx context.Context, options ManifestOptions) (*ManifestGenerationResult, error)

	// DiscoverManifests discovers existing Kubernetes manifests in a directory
	DiscoverManifests(ctx context.Context, directory string) (*ManifestDiscoveryResult, error)

	// ValidateManifests validates Kubernetes manifests
	ValidateManifests(ctx context.Context, manifests []string) (*api.ManifestValidationResult, error)

	// GetAvailableTemplates returns available manifest templates
	GetAvailableTemplates() ([]string, error)
}

// ManifestServiceImpl implements the ManifestService interface
type ManifestServiceImpl struct {
	logger *slog.Logger
}

// NewManifestService creates a new manifest service
func NewManifestService(logger *slog.Logger) ManifestService {
	return &ManifestServiceImpl{
		logger: logger.With("component", "k8s_manifest_service"),
	}
}

// GenerateManifests generates Kubernetes manifests from templates
func (s *ManifestServiceImpl) GenerateManifests(_ context.Context, options ManifestOptions) (*ManifestGenerationResult, error) {
	startTime := time.Now()

	result := &ManifestGenerationResult{
		Template:  options.Template,
		OutputDir: options.OutputDir,
		Context:   make(map[string]interface{}),
		Manifests: make([]GeneratedManifest, 0),
	}

	s.logger.Info("Starting manifest generation",
		"template", options.Template,
		"app_name", options.AppName,
		"namespace", options.Namespace)

	// Validate inputs
	if err := s.validateGenerateInputs(options); err != nil {
		result.Error = &ManifestError{
			Type:    "validation_error",
			Message: err.Error(),
			Context: map[string]interface{}{
				"options": options,
			},
		}
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Create output directory
	if err := os.MkdirAll(options.OutputDir, 0755); err != nil {
		result.Error = &ManifestError{
			Type:    "directory_error",
			Message: fmt.Sprintf("Failed to create output directory: %v", err),
			Path:    options.OutputDir,
			Context: map[string]interface{}{
				"options": options,
			},
		}
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Generate manifests based on template
	manifests, err := s.generateFromTemplate(options)
	if err != nil {
		result.Error = &ManifestError{
			Type:    "generation_error",
			Message: fmt.Sprintf("Failed to generate manifests: %v", err),
			Context: map[string]interface{}{
				"options": options,
			},
		}
		result.Duration = time.Since(startTime)
		return result, nil
	}

	result.Manifests = manifests
	result.Success = true
	result.Duration = time.Since(startTime)

	// Set manifest path to first manifest or output directory
	if len(manifests) > 0 {
		result.ManifestPath = manifests[0].Path
	} else {
		result.ManifestPath = options.OutputDir
	}

	s.logger.Info("Manifest generation completed",
		"manifests_count", len(manifests),
		"duration", result.Duration,
		"output_dir", options.OutputDir)

	return result, nil
}

// DiscoverManifests discovers existing Kubernetes manifests in a directory
func (s *ManifestServiceImpl) DiscoverManifests(_ context.Context, directory string) (*ManifestDiscoveryResult, error) {
	result := &ManifestDiscoveryResult{
		Directory: directory,
		Context:   make(map[string]interface{}),
		Manifests: make([]DiscoveredManifest, 0),
	}

	s.logger.Info("Discovering manifests", "directory", directory)

	// Validate directory exists
	if _, err := os.Stat(directory); os.IsNotExist(err) {
		result.Error = &ManifestError{
			Type:    "directory_error",
			Message: fmt.Sprintf("Directory does not exist: %s", directory),
			Path:    directory,
			Context: map[string]interface{}{
				"directory": directory,
			},
		}
		return result, nil
	}

	// Discover YAML files
	manifests, err := s.discoverYAMLFiles(directory)
	if err != nil {
		result.Error = &ManifestError{
			Type:    "discovery_error",
			Message: fmt.Sprintf("Failed to discover manifests: %v", err),
			Path:    directory,
			Context: map[string]interface{}{
				"directory": directory,
			},
		}
		return result, nil
	}

	result.Manifests = manifests
	result.Success = true

	s.logger.Info("Manifest discovery completed",
		"manifests_count", len(manifests),
		"directory", directory)

	return result, nil
}

// ValidateManifests validates Kubernetes manifests
func (s *ManifestServiceImpl) ValidateManifests(_ context.Context, manifests []string) (*api.ManifestValidationResult, error) {
	result := &api.ManifestValidationResult{
		Valid:   true,
		Errors:  make([]validation.Error, 0),
		Context: make(map[string]string),
	}

	s.logger.Info("Validating manifests", "count", len(manifests))

	for _, manifest := range manifests {
		if err := s.validateManifest(manifest); err != nil {
			result.Valid = false
			validationErr := validation.Error{
				Message: err.Error(),
				Field:   "manifest",
				Code:    "MANIFEST_VALIDATION_ERROR",
			}
			result.Errors = append(result.Errors, validationErr)
		}
	}

	s.logger.Info("Manifest validation completed",
		"valid", result.Valid,
		"errors", len(result.Errors))

	return result, nil
}

// GetAvailableTemplates returns available manifest templates
func (s *ManifestServiceImpl) GetAvailableTemplates() ([]string, error) {
	// This would typically read from embedded templates or a template directory
	templates := []string{
		"deployment",
		"service",
		"ingress",
		"configmap",
		"secret",
		"full-stack",
	}

	s.logger.Debug("Available templates", "templates", templates)
	return templates, nil
}

// Helper methods

func (s *ManifestServiceImpl) validateGenerateInputs(options ManifestOptions) error {
	if options.AppName == "" {
		return fmt.Errorf("app name is required")
	}
	if options.ImageRef == "" {
		return fmt.Errorf("image reference is required")
	}
	if options.Namespace == "" {
		return fmt.Errorf("namespace is required")
	}
	if options.OutputDir == "" {
		return fmt.Errorf("output directory is required")
	}
	return nil
}

func (s *ManifestServiceImpl) generateFromTemplate(options ManifestOptions) ([]GeneratedManifest, error) {
	// This is a simplified implementation
	// In practice, this would use actual Kubernetes template generation

	manifests := make([]GeneratedManifest, 0)

	// Generate deployment manifest
	deploymentContent := s.generateDeploymentManifest(options)
	deploymentPath := fmt.Sprintf("%s/deployment.yaml", options.OutputDir)

	if err := os.WriteFile(deploymentPath, []byte(deploymentContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to write deployment manifest: %v", err)
	}

	manifests = append(manifests, GeneratedManifest{
		Name:    fmt.Sprintf("%s-deployment", options.AppName),
		Kind:    "Deployment",
		Path:    deploymentPath,
		Content: deploymentContent,
		Size:    len(deploymentContent),
		Valid:   true,
	})

	// Generate service manifest if requested
	if options.IncludeService {
		serviceContent := s.generateServiceManifest(options)
		servicePath := fmt.Sprintf("%s/service.yaml", options.OutputDir)

		if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
			return nil, fmt.Errorf("failed to write service manifest: %v", err)
		}

		manifests = append(manifests, GeneratedManifest{
			Name:    fmt.Sprintf("%s-service", options.AppName),
			Kind:    "Service",
			Path:    servicePath,
			Content: serviceContent,
			Size:    len(serviceContent),
			Valid:   true,
		})
	}

	return manifests, nil
}

func (s *ManifestServiceImpl) generateDeploymentManifest(options ManifestOptions) string {
	// Simplified deployment manifest template
	return fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: %s
spec:
  replicas: %d
  selector:
    matchLabels:
      app: %s
  template:
    metadata:
      labels:
        app: %s
    spec:
      containers:
      - name: %s
        image: %s
        ports:
        - containerPort: %d
`, options.AppName, options.Namespace, options.Replicas, options.AppName, options.AppName, options.AppName, options.ImageRef, options.Port)
}

func (s *ManifestServiceImpl) generateServiceManifest(options ManifestOptions) string {
	// Simplified service manifest template
	return fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: %s
  namespace: %s
spec:
  selector:
    app: %s
  ports:
  - protocol: TCP
    port: %d
    targetPort: %d
  type: ClusterIP
`, options.AppName, options.Namespace, options.AppName, options.Port, options.Port)
}

func (s *ManifestServiceImpl) discoverYAMLFiles(directory string) ([]DiscoveredManifest, error) {
	// This is a simplified implementation
	// In practice, this would recursively discover and parse YAML files
	manifests := make([]DiscoveredManifest, 0)

	// Placeholder implementation
	s.logger.Debug("Discovering YAML files", "directory", directory)

	return manifests, nil
}

func (s *ManifestServiceImpl) validateManifest(manifest string) error {
	// This is a simplified implementation
	// In practice, this would validate Kubernetes manifest syntax and semantics

	if manifest == "" {
		return fmt.Errorf("manifest content is empty")
	}

	// Try to parse as YAML
	var obj interface{}
	if err := yaml.Unmarshal([]byte(manifest), &obj); err != nil {
		return fmt.Errorf("invalid YAML: %v", err)
	}

	return nil
}

// Backward compatibility adapter
type ManifestManagerAdapter struct {
	service ManifestService
}

// NewManifestManagerAdapter creates a new adapter that wraps the service
func NewManifestManagerAdapter(_ *slog.Logger) *ManifestManagerAdapter {
	// Use default slog logger
	slogLogger := slog.Default()

	return &ManifestManagerAdapter{
		service: NewManifestService(slogLogger),
	}
}

// GenerateManifests delegates to the service
func (a *ManifestManagerAdapter) GenerateManifests(ctx context.Context, options ManifestOptions) (*ManifestGenerationResult, error) {
	return a.service.GenerateManifests(ctx, options)
}

// DiscoverManifests delegates to the service
func (a *ManifestManagerAdapter) DiscoverManifests(ctx context.Context, directory string) (*ManifestDiscoveryResult, error) {
	return a.service.DiscoverManifests(ctx, directory)
}

// ValidateManifests delegates to the service
func (a *ManifestManagerAdapter) ValidateManifests(ctx context.Context, manifests []string) (*api.ManifestValidationResult, error) {
	return a.service.ValidateManifests(ctx, manifests)
}

// GetAvailableTemplates delegates to the service
func (a *ManifestManagerAdapter) GetAvailableTemplates() ([]string, error) {
	return a.service.GetAvailableTemplates()
}
