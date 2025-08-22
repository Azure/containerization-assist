// Package docker provides core Docker operations extracted from the Containerization Assist pipeline.
// This package contains mechanical Docker operations without AI dependencies,
// designed to be used by atomic MCP tools that let external AI handle reasoning.
package container

import (
	"context"
	"log/slog"
	"os/exec"
	"time"
)

// Service provides a unified interface to all Docker operations
type Service interface {
	// Containerize performs the complete containerization workflow
	Containerize(ctx context.Context, targetDir string, options ContainerizeOptions) (*ContainerizationResult, error)

	// CheckPrerequisites verifies that all Docker prerequisites are met
	CheckPrerequisites(ctx context.Context) error

	// GetAvailableTemplates returns all available Dockerfile templates
	GetAvailableTemplates() ([]TemplateInfo, error)

	// QuickBuild performs a quick build without template generation
	QuickBuild(ctx context.Context, dockerfileContent string, targetDir string, options BuildOptions) (*BuildResult, error)

	// QuickPush performs a quick push of an already built image
	QuickPush(ctx context.Context, imageRef string, options PushOptions) (*RegistryPushResult, error)

	// QuickPull performs a quick pull of an image
	QuickPull(ctx context.Context, imageRef string) (*PullResult, error)
}

// ServiceImpl implements the Docker service interface
type ServiceImpl struct {
	Builder           *Builder
	TemplateEngine    *TemplateEngine
	RegistryManager   *RegistryManager
	HadolintValidator *HadolintValidator
	logger            *slog.Logger
}

// NewService creates a new Docker operations service
func NewService(docker DockerClient, logger *slog.Logger) Service {
	return &ServiceImpl{
		Builder:           NewBuilder(docker, logger),
		TemplateEngine:    NewTemplateEngine(logger),
		RegistryManager:   NewRegistryManager(docker, logger),
		HadolintValidator: NewHadolintValidator(logger),
	}
}

// ServiceImpl methods implementing the Service interface

// Containerize performs the complete containerization workflow
func (s *ServiceImpl) Containerize(ctx context.Context, targetDir string, options ContainerizeOptions) (*ContainerizationResult, error) {
	startTime := time.Now()

	result := &ContainerizationResult{
		Context: make(map[string]interface{}),
	}

	// Step 1: Generate Dockerfile from template
	if options.TemplateName == "" {
		// Suggest template based on heuristics
		suggestedTemplate, suggestions, err := s.TemplateEngine.SuggestTemplate(
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

	templateResult, err := s.TemplateEngine.GenerateFromTemplate(options.TemplateName, targetDir)
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
	validationResult, _ := s.HadolintValidator.ValidateWithHadolint(ctx, templateResult.Dockerfile)
	result.Validation = validationResult

	if !validationResult.Valid {
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

	buildResult, err := s.Builder.BuildImage(ctx, templateResult.Dockerfile, targetDir, buildOptions)
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

		pushResult, err := s.RegistryManager.PushImage(ctx, buildResult.ImageRef, pushOptions)
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

	return result, nil
}

// CheckPrerequisites verifies that all Docker prerequisites are met
func (s *ServiceImpl) CheckPrerequisites(_ context.Context) error {
	// Simple Docker check - verify docker command is available
	_, err := exec.LookPath("docker")
	return err
}

// GetAvailableTemplates returns all available Dockerfile templates
func (s *ServiceImpl) GetAvailableTemplates() ([]TemplateInfo, error) {
	return s.TemplateEngine.ListAvailableTemplates()
}

// QuickBuild performs a quick build without template generation
func (s *ServiceImpl) QuickBuild(ctx context.Context, dockerfileContent string, targetDir string, options BuildOptions) (*BuildResult, error) {
	return s.Builder.BuildImage(ctx, dockerfileContent, targetDir, options)
}

// QuickPush performs a quick push of an already built image
func (s *ServiceImpl) QuickPush(ctx context.Context, imageRef string, options PushOptions) (*RegistryPushResult, error) {
	return s.RegistryManager.PushImage(ctx, imageRef, options)
}

// QuickPull performs a quick pull of an image
func (s *ServiceImpl) QuickPull(ctx context.Context, imageRef string) (*PullResult, error) {
	return s.RegistryManager.PullImage(ctx, imageRef)
}
