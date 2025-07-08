// Package kubernetes provides core Kubernetes operations extracted from the Container Kit pipeline.
// This package contains mechanical K8s operations without AI dependencies,
// designed to be used by atomic MCP tools.
package kubernetes

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/k8s"
	"github.com/Azure/container-kit/pkg/mcp/api"
	validation "github.com/Azure/container-kit/pkg/mcp/security"
	"github.com/rs/zerolog"
	"sigs.k8s.io/yaml"
)

// ManifestManager provides mechanical Kubernetes manifest operations
type ManifestManager struct {
	logger zerolog.Logger
}

// NewManifestManager creates a new manifest manager
func NewManifestManager(logger zerolog.Logger) *ManifestManager {
	return &ManifestManager{
		logger: logger.With().Str("component", "k8s_manifest_manager").Logger(),
	}
}

// ManifestGenerationResult contains the result of manifest generation
type ManifestGenerationResult struct {
	Success      bool                   `json:"success"`
	Manifests    []GeneratedManifest    `json:"manifests"`
	Template     string                 `json:"template"`
	OutputDir    string                 `json:"output_dir"`
	ManifestPath string                 `json:"manifest_path"` // Path to generated manifests
	Duration     time.Duration          `json:"duration"`
	Context      map[string]interface{} `json:"context"`
	Error        *ManifestError         `json:"error,omitempty"`
}

// GeneratedManifest represents a generated Kubernetes manifest
type GeneratedManifest struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Path    string `json:"path"`
	Content string `json:"content"`
	Size    int    `json:"size"`
	Valid   bool   `json:"valid"`
}

// ManifestDiscoveryResult contains discovered manifests
type ManifestDiscoveryResult struct {
	Success   bool                   `json:"success"`
	Manifests []DiscoveredManifest   `json:"manifests"`
	Directory string                 `json:"directory"`
	Context   map[string]interface{} `json:"context"`
	Error     *ManifestError         `json:"error,omitempty"`
}

// DiscoveredManifest represents a discovered Kubernetes manifest
type DiscoveredManifest struct {
	Name             string            `json:"name"`
	Kind             string            `json:"kind"`
	ApiVersion       string            `json:"api_version"`
	Path             string            `json:"path"`
	Size             int64             `json:"size"`
	Valid            bool              `json:"valid"`
	Metadata         map[string]string `json:"metadata"`
	ValidationErrors []string          `json:"validation_errors,omitempty"`
}

// ManifestError provides detailed manifest error information
type ManifestError struct {
	Type         string                 `json:"type"` // "generation_error", "discovery_error", "validation_error"
	Message      string                 `json:"message"`
	Path         string                 `json:"path,omitempty"`
	ManifestName string                 `json:"manifest_name,omitempty"`
	Context      map[string]interface{} `json:"context"`
}

// ManifestOptions contains options for manifest generation
type ManifestOptions struct {
	ImageRef       string
	AppName        string
	Namespace      string
	Port           int
	Replicas       int
	Template       string
	OutputDir      string
	IncludeService bool
	IncludeIngress bool
	Labels         map[string]string
	Annotations    map[string]string
	Resources      *ResourceRequirements
}

// ResourceRequirements defines resource requests and limits
type ResourceRequirements struct {
	Requests *ResourceQuantity `json:"requests,omitempty"`
	Limits   *ResourceQuantity `json:"limits,omitempty"`
}

// ResourceQuantity defines CPU and memory quantities
type ResourceQuantity struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

// GenerateManifests generates Kubernetes manifests from templates
func (mm *ManifestManager) GenerateManifests(ctx context.Context, options ManifestOptions) (*ManifestGenerationResult, error) {
	startTime := time.Now()

	result := &ManifestGenerationResult{
		Manifests: make([]GeneratedManifest, 0),
		Template:  options.Template,
		OutputDir: options.OutputDir,
		Context:   make(map[string]interface{}),
	}

	mm.logger.Info().
		Str("image_ref", options.ImageRef).
		Str("app_name", options.AppName).
		Str("template", options.Template).
		Str("output_dir", options.OutputDir).
		Msg("Starting manifest generation")

	// Validate inputs
	if err := mm.validateGenerationInputs(options); err != nil {
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

	// Prepare output directory
	if err := mm.prepareOutputDirectory(options.OutputDir); err != nil {
		result.Error = &ManifestError{
			Type:    "filesystem_error",
			Message: fmt.Sprintf("Failed to prepare output directory: %v", err),
			Path:    options.OutputDir,
		}
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Determine template to use
	templateName := options.Template
	if templateName == "" {
		templateName = mm.selectDefaultTemplate(options)
	}

	// Generate manifests from template
	manifests, err := mm.generateFromTemplate(templateName, options)
	if err != nil {
		result.Error = &ManifestError{
			Type:    "generation_error",
			Message: fmt.Sprintf("Failed to generate manifests: %v", err),
			Context: map[string]interface{}{
				"template": templateName,
				"options":  options,
			},
		}
		result.Duration = time.Since(startTime)
		return result, nil
	}

	result.Manifests = manifests
	result.Success = true
	result.Duration = time.Since(startTime)
	result.Context = map[string]interface{}{
		"generation_time": result.Duration.Seconds(),
		"manifest_count":  len(manifests),
		"template_used":   templateName,
		"output_dir":      options.OutputDir,
	}

	mm.logger.Info().
		Int("manifest_count", len(manifests)).
		Str("template", templateName).
		Dur("duration", result.Duration).
		Msg("Manifest generation completed successfully")

	return result, nil
}

// DiscoverManifests finds and analyzes existing Kubernetes manifests
func (mm *ManifestManager) DiscoverManifests(ctx context.Context, directory string) (*ManifestDiscoveryResult, error) {
	result := &ManifestDiscoveryResult{
		Manifests: make([]DiscoveredManifest, 0),
		Directory: directory,
		Context:   make(map[string]interface{}),
	}

	mm.logger.Info().Str("directory", directory).Msg("Starting manifest discovery")

	// Validate directory
	if err := mm.validateDirectory(directory); err != nil {
		result.Error = &ManifestError{
			Type:    "validation_error",
			Message: err.Error(),
			Path:    directory,
		}
		return result, nil
	}

	// Use existing K8s package to find manifests
	k8sObjects, err := k8s.FindK8sObjects(directory)
	if err != nil {
		result.Error = &ManifestError{
			Type:    "discovery_error",
			Message: fmt.Sprintf("Failed to discover manifests: %v", err),
			Path:    directory,
		}
		return result, nil
	}

	// Convert to our format
	for _, obj := range k8sObjects {
		manifest := DiscoveredManifest{
			Name:       obj.Metadata.Name,
			Kind:       obj.Kind,
			ApiVersion: obj.ApiVersion,
			Path:       obj.ManifestPath,
			Valid:      true, // Basic validation passed if it was discovered
			Metadata:   make(map[string]string),
		}

		// Get file size
		if stat, err := os.Stat(obj.ManifestPath); err == nil {
			manifest.Size = stat.Size()
		}

		// Extract basic metadata
		if obj.Metadata.Name != "" {
			manifest.Metadata["name"] = obj.Metadata.Name
		}
		// Note: K8sMetadata doesn't have Namespace field currently

		// Add labels if any
		for k, v := range obj.Metadata.Labels {
			manifest.Metadata[fmt.Sprintf("label.%s", k)] = v
		}

		result.Manifests = append(result.Manifests, manifest)
	}

	result.Success = true
	result.Context = map[string]interface{}{
		"manifest_count": len(result.Manifests),
		"directory":      directory,
	}

	mm.logger.Info().
		Int("manifest_count", len(result.Manifests)).
		Str("directory", directory).
		Msg("Manifest discovery completed successfully")

	return result, nil
}

// ValidateManifest validates a single Kubernetes manifest
func (mm *ManifestManager) ValidateManifest(manifestPath string) (*ManifestValidationResult, error) {
	result := &ManifestValidationResult{
		Valid:    true,
		Errors:   make([]api.ValidationError, 0),
		Warnings: make([]api.ValidationWarning, 0),
		Metadata: api.ValidationMetadata{
			ValidatedAt:      time.Now(),
			ValidatorName:    "k8s-manifest-validator",
			ValidatorVersion: "1.0.0",
			Context:          make(map[string]string),
		},
		Details: make(map[string]interface{}),
		Context: make(map[string]string),
	}

	// Read manifest file
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		fileError := api.NewError(
			"MANIFEST_FILE_READ_ERROR",
			fmt.Sprintf("Cannot read manifest file: %v", err),
			validation.ErrTypeCustom,
			validation.SeverityHigh,
		)
		result.Errors = append(result.Errors, *fileError)
		return result, nil
	}

	// Parse as YAML
	var manifest map[string]interface{}
	if err := yaml.Unmarshal(content, &manifest); err != nil {
		yamlError := api.NewError(
			"MANIFEST_YAML_PARSE_ERROR",
			fmt.Sprintf("Invalid YAML format: %v", err),
			validation.ErrTypeValidation,
			validation.SeverityHigh,
		)
		result.Errors = append(result.Errors, *yamlError)
		return result, nil
	}

	// Basic Kubernetes resource validation
	if err := mm.validateK8sResource(manifest); err != nil {
		k8sError := api.NewError(
			"MANIFEST_K8S_VALIDATION_ERROR",
			err.Error(),
			validation.ErrTypeValidation,
			validation.SeverityHigh,
		)
		result.Errors = append(result.Errors, *k8sError)
	}

	result.Valid = len(result.Errors) == 0

	// Store content in validation metadata
	result.Metadata.Context["manifest_path"] = manifestPath
	result.Metadata.Context["content"] = string(content)

	return result, nil
}

// ManifestValidationResult now uses the unified validation framework for deploy domain
type ManifestValidationResult = api.ManifestValidationResult

// ValidationError now uses the unified validation framework
type ValidationError = api.ValidationError

// GetAvailableTemplates returns available manifest templates
func (mm *ManifestManager) GetAvailableTemplates() ([]TemplateInfo, error) {
	// This would list available templates from the embedded templates
	// For now, return the known basic template
	return []TemplateInfo{
		{
			Name:        "basic",
			Description: "Basic Kubernetes deployment with service",
			Files:       []string{"deployment.yaml", "service.yaml"},
		},
	}, nil
}

// TemplateInfo contains information about a manifest template
type TemplateInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Files       []string `json:"files"`
}

// Helper methods

func (mm *ManifestManager) validateGenerationInputs(options ManifestOptions) error {
	if options.ImageRef == "" {
		return fmt.Errorf("image reference is required")
	}

	if options.AppName == "" {
		return fmt.Errorf("application name is required")
	}

	if options.OutputDir == "" {
		return fmt.Errorf("output directory is required")
	}

	return nil
}

func (mm *ManifestManager) validateDirectory(directory string) error {
	if directory == "" {
		return fmt.Errorf("directory path is required")
	}

	stat, err := os.Stat(directory)
	if err != nil {
		return fmt.Errorf("cannot access directory: %v", err)
	}

	if !stat.IsDir() {
		return fmt.Errorf("path is not a directory: %s", directory)
	}

	return nil
}

func (mm *ManifestManager) prepareOutputDirectory(outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}
	return nil
}

func (mm *ManifestManager) selectDefaultTemplate(options ManifestOptions) string {
	// Simple template selection logic
	if options.IncludeService {
		return "basic"
	}
	return "basic" // Default
}

func (mm *ManifestManager) generateFromTemplate(templateName string, options ManifestOptions) ([]GeneratedManifest, error) {
	manifests := make([]GeneratedManifest, 0)

	// Use the existing K8s package to write manifests from template
	if err := k8s.WriteManifestsFromTemplate(k8s.ManifestsBasic, options.OutputDir, k8s.DefaultImageAndTag); err != nil {
		return nil, fmt.Errorf("failed to write manifests from template: %v", err)
	}

	// Discover the generated manifests
	k8sObjects, err := k8s.FindK8sObjects(options.OutputDir)
	if err != nil {
		return nil, fmt.Errorf("failed to find generated manifests: %v", err)
	}

	// Process each generated manifest
	for _, obj := range k8sObjects {
		// Read the content
		content, err := os.ReadFile(obj.ManifestPath)
		if err != nil {
			mm.logger.Warn().Err(err).Str("path", obj.ManifestPath).Msg("Failed to read generated manifest")
			continue
		}

		// Customize the manifest with provided options
		customizedContent, err := mm.customizeManifest(string(content), options)
		if err != nil {
			mm.logger.Warn().Err(err).Str("path", obj.ManifestPath).Msg("Failed to customize manifest")
			customizedContent = string(content) // Use original if customization fails
		}

		// Write the customized manifest back
		if err := os.WriteFile(obj.ManifestPath, []byte(customizedContent), 0644); err != nil {
			mm.logger.Warn().Err(err).Str("path", obj.ManifestPath).Msg("Failed to write customized manifest")
		}

		manifest := GeneratedManifest{
			Name:    obj.Metadata.Name,
			Kind:    obj.Kind,
			Path:    obj.ManifestPath,
			Content: customizedContent,
			Size:    len(customizedContent),
			Valid:   true,
		}

		manifests = append(manifests, manifest)
	}

	return manifests, nil
}

func (mm *ManifestManager) customizeManifest(content string, options ManifestOptions) (string, error) {
	// Simple string replacement for customization
	// In a more sophisticated implementation, this would parse YAML and modify structured data

	result := content

	// Replace image reference
	if options.ImageRef != "" {
		// Look for common image placeholder patterns
		result = strings.ReplaceAll(result, "{{IMAGE}}", options.ImageRef)
		result = strings.ReplaceAll(result, "nginx:latest", options.ImageRef) // Replace default
	}

	// Replace app name
	if options.AppName != "" {
		result = strings.ReplaceAll(result, "{{APP_NAME}}", options.AppName)
		result = strings.ReplaceAll(result, "my-app", options.AppName) // Replace default
	}

	// Replace namespace
	if options.Namespace != "" && options.Namespace != "default" {
		result = strings.ReplaceAll(result, "{{NAMESPACE}}", options.Namespace)
	}

	// Replace port
	if options.Port > 0 {
		result = strings.ReplaceAll(result, "{{PORT}}", fmt.Sprintf("%d", options.Port))
		result = strings.ReplaceAll(result, "port: 80", fmt.Sprintf("port: %d", options.Port))
		result = strings.ReplaceAll(result, "targetPort: 80", fmt.Sprintf("targetPort: %d", options.Port))
	}

	// Replace replicas
	if options.Replicas > 0 {
		result = strings.ReplaceAll(result, "{{REPLICAS}}", fmt.Sprintf("%d", options.Replicas))
		result = strings.ReplaceAll(result, "replicas: 1", fmt.Sprintf("replicas: %d", options.Replicas))
	}

	// Replace resource requirements
	if options.Resources != nil {
		if options.Resources.Requests != nil {
			if options.Resources.Requests.CPU != "" {
				result = strings.ReplaceAll(result, "cpu: \"0.5\"", fmt.Sprintf("cpu: \"%s\"", options.Resources.Requests.CPU))
				result = strings.ReplaceAll(result, "{{CPU_REQUEST}}", options.Resources.Requests.CPU)
			}
			if options.Resources.Requests.Memory != "" {
				result = strings.ReplaceAll(result, "memory: \"0.5Gi\"", fmt.Sprintf("memory: \"%s\"", options.Resources.Requests.Memory))
				result = strings.ReplaceAll(result, "{{MEMORY_REQUEST}}", options.Resources.Requests.Memory)
			}
		}
		if options.Resources.Limits != nil {
			if options.Resources.Limits.CPU != "" {
				result = strings.ReplaceAll(result, "cpu: \"1\"", fmt.Sprintf("cpu: \"%s\"", options.Resources.Limits.CPU))
				result = strings.ReplaceAll(result, "{{CPU_LIMIT}}", options.Resources.Limits.CPU)
			}
			if options.Resources.Limits.Memory != "" {
				result = strings.ReplaceAll(result, "memory: \"1Gi\"", fmt.Sprintf("memory: \"%s\"", options.Resources.Limits.Memory))
				result = strings.ReplaceAll(result, "{{MEMORY_LIMIT}}", options.Resources.Limits.Memory)
			}
		}
	}

	return result, nil
}

func (mm *ManifestManager) validateK8sResource(manifest map[string]interface{}) error {
	// Basic Kubernetes resource validation

	// Check for required fields
	if apiVersion, ok := manifest["apiVersion"].(string); !ok || apiVersion == "" {
		return fmt.Errorf("missing or empty apiVersion field")
	}

	if kind, ok := manifest["kind"].(string); !ok || kind == "" {
		return fmt.Errorf("missing or empty kind field")
	}

	// Check for metadata
	if metadata, ok := manifest["metadata"].(map[string]interface{}); !ok {
		return fmt.Errorf("missing metadata field")
	} else {
		if name, ok := metadata["name"].(string); !ok || name == "" {
			return fmt.Errorf("missing or empty metadata.name field")
		}
	}

	return nil
}
