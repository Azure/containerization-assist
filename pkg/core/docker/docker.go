// Package docker provides core Docker operations extracted from the Container Kit pipeline.
// This package contains mechanical Docker operations without AI dependencies,
// designed to be used by atomic MCP tools that let external AI handle reasoning.
package docker

import (
	"context"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/clients"
	"github.com/Azure/container-kit/pkg/mcp/domain/types"
)

// Manager provides a unified interface to all Docker operations
type Manager struct {
	Builder         *Builder
	TemplateEngine  *TemplateEngine
	RegistryManager *RegistryManager
	Validator       *Validator
	logger          *slog.Logger
}

// NewManager creates a new Docker operations manager
func NewManager(clients *clients.Clients, logger *slog.Logger) *Manager {
	return &Manager{
		Builder:         NewBuilder(clients, logger),
		TemplateEngine:  NewTemplateEngine(logger),
		RegistryManager: NewRegistryManager(clients, logger),
		Validator:       NewValidator(logger),
		logger:          logger.With("component", "docker_manager"),
	}
}

// ContainerizeOptions contains all options for the containerization process
type ContainerizeOptions struct {
	// Template options
	TemplateName string
	Language     string
	Framework    string
	Dependencies []string
	ConfigFiles  []string

	// Build options
	ImageName    string
	Registry     string
	NoCache      bool
	Platform     string
	BuildArgs    map[string]string
	BuildTimeout time.Duration

	// Push options
	AutoPush    bool
	RetryCount  int
	PushTimeout time.Duration
}

// ContainerizationResult contains the complete result of containerization
type ContainerizationResult struct {
	Success bool `json:"success"`

	// Template generation results
	Template *GenerateResult `json:"template,omitempty"`

	// Validation results
	Validation *types.BuildValidationResult `json:"validation,omitempty"`

	// Build results
	Build *BuildResult `json:"build,omitempty"`

	// Push results (if requested)
	Push *RegistryPushResult `json:"push,omitempty"`

	// Overall context
	Duration time.Duration          `json:"duration"`
	Context  map[string]interface{} `json:"context"`
	Error    string                 `json:"error,omitempty"`
}

// Containerize performs the complete containerization workflow
// This is a convenience method that combines template generation, validation, build, and optionally push
func (m *Manager) Containerize(ctx context.Context, targetDir string, options ContainerizeOptions) (*ContainerizationResult, error) {
	startTime := time.Now()

	result := &ContainerizationResult{
		Context: make(map[string]interface{}),
	}

	m.logger.Info("Starting containerization workflow",
		"target_dir", targetDir,
		"template", options.TemplateName,
		"image_name", options.ImageName)

	// Step 1: Generate Dockerfile from template
	if options.TemplateName == "" {
		// Suggest template based on heuristics
		suggestedTemplate, suggestions, err := m.TemplateEngine.SuggestTemplate(
			options.Language,
			options.Framework,
			options.Dependencies,
			options.ConfigFiles,
		)
		if err != nil {
			result.Error = err.Error()
			result.Duration = time.Since(startTime)
			return result, nil
		}

		options.TemplateName = suggestedTemplate
		result.Context["template_suggestions"] = suggestions
		result.Context["template_auto_selected"] = true
	}

	templateResult, err := m.TemplateEngine.GenerateFromTemplate(options.TemplateName, targetDir)
	if err != nil {
		result.Error = err.Error()
		result.Duration = time.Since(startTime)
		return result, nil
	}

	result.Template = templateResult
	if !templateResult.Success {
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Step 2: Validate the generated Dockerfile
	validationResult := m.Validator.ValidateDockerfile(templateResult.Dockerfile)
	result.Validation = validationResult

	if !validationResult.Valid {
		m.logger.Warn("Generated Dockerfile has validation errors",
			"errors", len(validationResult.Errors))
		// Continue anyway - let external AI handle the errors
	}

	// Step 3: Build the Docker image
	buildOptions := BuildOptions{
		ImageName:    options.ImageName,
		Registry:     options.Registry,
		NoCache:      options.NoCache,
		Platform:     options.Platform,
		BuildArgs:    options.BuildArgs,
		BuildTimeout: options.BuildTimeout,
	}

	buildResult, err := m.Builder.BuildImage(ctx, templateResult.Dockerfile, targetDir, buildOptions)
	if err != nil {
		result.Error = err.Error()
		result.Duration = time.Since(startTime)
		return result, nil
	}

	result.Build = buildResult
	if !buildResult.Success {
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Step 4: Push to registry if requested
	if options.AutoPush && options.Registry != "" {
		pushOptions := PushOptions{
			Registry:   options.Registry,
			RetryCount: options.RetryCount,
			Timeout:    options.PushTimeout,
		}

		pushResult, err := m.RegistryManager.PushImage(ctx, buildResult.ImageRef, pushOptions)
		if err != nil {
			result.Error = err.Error()
			result.Duration = time.Since(startTime)
			return result, nil
		}

		result.Push = pushResult
		if !pushResult.Success {
			result.Duration = time.Since(startTime)
			return result, nil
		}
	}

	// Success!
	result.Success = true
	result.Duration = time.Since(startTime)
	result.Context["workflow_completed"] = true
	result.Context["image_ref"] = buildResult.ImageRef

	if result.Push != nil && result.Push.Success {
		result.Context["pushed_to_registry"] = true
	}

	m.logger.Info("Containerization workflow completed successfully",
		"image_ref", buildResult.ImageRef,
		"duration", result.Duration,
		"pushed", result.Push != nil && result.Push.Success)

	return result, nil
}

// CheckPrerequisites verifies that all Docker prerequisites are met
func (m *Manager) CheckPrerequisites(ctx context.Context) error {
	return m.Validator.CheckDockerInstallation()
}

// GetAvailableTemplates returns all available Dockerfile templates
func (m *Manager) GetAvailableTemplates() ([]TemplateInfo, error) {
	return m.TemplateEngine.ListAvailableTemplates()
}

// QuickBuild performs a quick build without template generation
// Useful when you already have a Dockerfile
func (m *Manager) QuickBuild(ctx context.Context, dockerfileContent string, targetDir string, options BuildOptions) (*BuildResult, error) {
	return m.Builder.BuildImage(ctx, dockerfileContent, targetDir, options)
}

// QuickPush performs a quick push of an already built image
func (m *Manager) QuickPush(ctx context.Context, imageRef string, options PushOptions) (*RegistryPushResult, error) {
	return m.RegistryManager.PushImage(ctx, imageRef, options)
}
