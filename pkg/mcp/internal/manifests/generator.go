package manifests

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
)

// Generator is the main interface for manifest generation
type Generator interface {
	// GenerateManifests generates Kubernetes manifests based on options
	GenerateManifests(ctx context.Context, opts GenerationOptions) (*GenerationResult, error)

	// ValidateManifests validates generated manifests
	ValidateManifests(ctx context.Context, manifestPath string) (*ValidationSummary, error)
}

// ManifestGenerator implements the Generator interface
type ManifestGenerator struct {
	logger     zerolog.Logger
	writer     *Writer
	validator  *Validator
	strategies map[string]ManifestStrategy
}

// NewManifestGenerator creates a new manifest generator
func NewManifestGenerator(logger zerolog.Logger) *ManifestGenerator {
	gen := &ManifestGenerator{
		logger:     logger.With().Str("component", "manifest_generator").Logger(),
		writer:     NewWriter(logger),
		validator:  NewValidator(logger),
		strategies: make(map[string]ManifestStrategy),
	}

	// Register default strategies
	gen.registerDefaultStrategies()

	return gen
}

// GenerateManifests generates Kubernetes manifests
func (g *ManifestGenerator) GenerateManifests(ctx context.Context, opts GenerationOptions) (*GenerationResult, error) {
	startTime := time.Now()

	result := &GenerationResult{
		Success:        false,
		FilesGenerated: []string{},
		Duration:       0,
		Errors:         []string{},
		Warnings:       []string{},
	}

	g.logger.Info().
		Str("namespace", opts.Namespace).
		Str("image", opts.ImageRef.String()).
		Bool("include_ingress", opts.IncludeIngress).
		Msg("Starting manifest generation")

	// Create output directory
	manifestPath := g.getManifestPath(opts)
	if err := g.writer.EnsureDirectory(manifestPath); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to create manifest directory: %v", err))
		result.Duration = time.Since(startTime)
		return result, err
	}

	result.ManifestPath = manifestPath

	// Generate deployment manifest
	if err := g.generateDeployment(manifestPath, opts); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to generate deployment: %v", err))
		result.Duration = time.Since(startTime)
		return result, err
	}
	result.FilesGenerated = append(result.FilesGenerated, "deployment.yaml")

	// Generate service manifest
	if err := g.generateService(manifestPath, opts); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to generate service: %v", err))
		result.Duration = time.Since(startTime)
		return result, err
	}
	result.FilesGenerated = append(result.FilesGenerated, "service.yaml")

	// Generate ConfigMap if needed
	if g.shouldGenerateConfigMap(opts) {
		if err := g.generateConfigMap(manifestPath, opts); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to generate ConfigMap: %v", err))
		} else {
			result.FilesGenerated = append(result.FilesGenerated, "configmap.yaml")
		}
	}

	// Generate Ingress if requested
	if opts.IncludeIngress {
		if err := g.generateIngress(manifestPath, opts); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to generate Ingress: %v", err))
		} else {
			result.FilesGenerated = append(result.FilesGenerated, "ingress.yaml")
		}
	}

	// Generate secrets if needed
	if len(opts.Secrets) > 0 {
		if err := g.generateSecrets(manifestPath, opts); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to generate secrets: %v", err))
		} else {
			result.FilesGenerated = append(result.FilesGenerated, "secret.yaml")
		}
	}

	result.Success = len(result.Errors) == 0
	result.Duration = time.Since(startTime)

	g.logger.Info().
		Bool("success", result.Success).
		Int("files_generated", len(result.FilesGenerated)).
		Dur("duration", result.Duration).
		Msg("Manifest generation completed")

	return result, nil
}

// ValidateManifests validates the generated manifests
func (g *ManifestGenerator) ValidateManifests(ctx context.Context, manifestPath string) (*ValidationSummary, error) {
	return g.validator.ValidateDirectory(ctx, manifestPath)
}

// RegisterStrategy registers a custom manifest generation strategy
func (g *ManifestGenerator) RegisterStrategy(name string, strategy ManifestStrategy) {
	g.strategies[name] = strategy
}

// registerDefaultStrategies registers the default manifest generation strategies
func (g *ManifestGenerator) registerDefaultStrategies() {
	// Strategies will be registered externally to avoid import cycles
	g.logger.Debug().Msg("Default strategies registration placeholder")
}

// Helper methods

func (g *ManifestGenerator) getManifestPath(opts GenerationOptions) string {
	// Use specified output path, or default to "./manifests"
	if opts.OutputPath != "" {
		return opts.OutputPath
	}
	return "./manifests"
}

func (g *ManifestGenerator) shouldGenerateConfigMap(opts GenerationOptions) bool {
	return len(opts.Environment) > 0 || len(opts.ConfigMapData) > 0 || len(opts.ConfigMapFiles) > 0
}

func (g *ManifestGenerator) generateDeployment(manifestPath string, opts GenerationOptions) error {
	strategy, exists := g.strategies["deployment"]
	if !exists {
		// Fall back to basic template-based generation
		return g.writer.WriteDeploymentTemplate(manifestPath, opts)
	}

	context := g.buildTemplateContext(opts)
	content, err := strategy.GenerateManifest(opts, context)
	if err != nil {
		return err
	}

	return g.writer.WriteFile(filepath.Join(manifestPath, "deployment.yaml"), content)
}

func (g *ManifestGenerator) generateService(manifestPath string, opts GenerationOptions) error {
	strategy, exists := g.strategies["service"]
	if !exists {
		// Fall back to basic template-based generation
		return g.writer.WriteServiceTemplate(manifestPath, opts)
	}

	context := g.buildTemplateContext(opts)
	content, err := strategy.GenerateManifest(opts, context)
	if err != nil {
		return err
	}

	return g.writer.WriteFile(filepath.Join(manifestPath, "service.yaml"), content)
}

func (g *ManifestGenerator) generateConfigMap(manifestPath string, opts GenerationOptions) error {
	strategy, exists := g.strategies["configmap"]
	if !exists {
		// Fall back to basic template-based generation
		return g.writer.WriteConfigMapTemplate(manifestPath, opts)
	}

	context := g.buildTemplateContext(opts)
	content, err := strategy.GenerateManifest(opts, context)
	if err != nil {
		return err
	}

	return g.writer.WriteFile(filepath.Join(manifestPath, "configmap.yaml"), content)
}

func (g *ManifestGenerator) generateIngress(manifestPath string, opts GenerationOptions) error {
	strategy, exists := g.strategies["ingress"]
	if !exists {
		// Fall back to basic template-based generation
		return g.writer.WriteIngressTemplate(manifestPath, opts)
	}

	context := g.buildTemplateContext(opts)
	content, err := strategy.GenerateManifest(opts, context)
	if err != nil {
		return err
	}

	return g.writer.WriteFile(filepath.Join(manifestPath, "ingress.yaml"), content)
}

func (g *ManifestGenerator) generateSecrets(manifestPath string, opts GenerationOptions) error {
	strategy, exists := g.strategies["secret"]
	if !exists {
		// Fall back to basic template-based generation
		return g.writer.WriteSecretTemplate(manifestPath, opts)
	}

	context := g.buildTemplateContext(opts)
	content, err := strategy.GenerateManifest(opts, context)
	if err != nil {
		return err
	}

	return g.writer.WriteFile(filepath.Join(manifestPath, "secret.yaml"), content)
}

func (g *ManifestGenerator) buildTemplateContext(opts GenerationOptions) TemplateContext {
	// Build template context from options
	// This is a simplified version - could be enhanced based on repository analysis
	return TemplateContext{
		HasDatabase: len(opts.Secrets) > 0,
		IsWebApp:    opts.IncludeIngress,
		Port:        g.extractPortFromService(opts),
	}
}

func (g *ManifestGenerator) extractPortFromService(opts GenerationOptions) int {
	if len(opts.ServicePorts) > 0 {
		return opts.ServicePorts[0].Port
	}
	return 8080 // Default port
}
