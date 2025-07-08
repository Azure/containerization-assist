package build

import (
	"context"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/core"
	core "github.com/Azure/container-kit/pkg/mcp/core/tools"
	"github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/Azure/container-kit/pkg/mcp/session"
	"github.com/rs/zerolog"
)

// dockerPullToolImpl implements the strongly-typed Docker pull tool
type dockerPullToolImpl struct {
	pipelineAdapter core.TypedPipelineOperations
	sessionManager  session.UnifiedSessionManager // Legacy field for backward compatibility
	sessionStore    services.SessionStore         // Modern service interface
	sessionState    services.SessionState         // Modern service interface
	logger          zerolog.Logger
}

// NewDockerPullTool creates a new strongly-typed Docker pull tool (legacy constructor)
func NewDockerPullTool(adapter core.TypedPipelineOperations, sessionManager session.UnifiedSessionManager, logger zerolog.Logger) interface{} {
	toolLogger := logger.With().Str("tool", "docker_pull").Logger()

	return &dockerPullToolImpl{
		pipelineAdapter: adapter,
		sessionManager:  sessionManager,
		logger:          toolLogger,
	}
}

// NewDockerPullToolWithServices creates a new strongly-typed Docker pull tool using service interfaces
func NewDockerPullToolWithServices(adapter core.TypedPipelineOperations, serviceContainer services.ServiceContainer, logger zerolog.Logger) interface{} {
	toolLogger := logger.With().Str("tool", "docker_pull").Logger()

	return &dockerPullToolImpl{
		pipelineAdapter: adapter,
		sessionStore:    serviceContainer.SessionStore(),
		sessionState:    serviceContainer.SessionState(),
		logger:          toolLogger,
	}
}

// Legacy function removed - use NewDockerPullToolUnified with session.UnifiedSessionManager

// Execute implements api.Tool interface
func (t *dockerPullToolImpl) Execute(ctx context.Context, params DockerPullParams) (DockerPullResult, error) {
	startTime := time.Now()

	// Validate parameters at compile time
	if err := params.Validate(); err != nil {
		return DockerPullResult{}, errors.NewError().
			Code(errors.CodeInvalidParameter).
			Message("Pull parameters validation failed").
			Type(errors.ErrTypeValidation).
			Severity(errors.SeverityMedium).
			Cause(err).
			Context("image", params.Image).
			Context("tag", params.Tag).
			Suggestion("Ensure image name is provided").
			WithLocation().
			Build()
	}

	// Construct full image reference
	imageRef := params.Image
	if params.Tag != "" {
		imageRef = params.Image + ":" + params.Tag
	}

	// Execute Docker pull using pipeline adapter
	pullErr := t.executePull(ctx, imageRef)

	// Create result
	result := DockerPullResult{
		Success:   pullErr == nil,
		Duration:  time.Since(startTime),
		SessionID: params.SessionID,
	}

	if pullErr != nil {
		// Create RichError with network context for pull failures
		return result, errors.ImagePullError(imageRef, pullErr)
	}

	// Set success details (would normally come from Docker API)
	result.ImageID = "sha256:pulled-image-id" // This would be actual ID from pull
	result.ImageSize = 0                      // This would be actual size

	t.logger.Info().
		Str("image", imageRef).
		Dur("duration", result.Duration).
		Msg("Docker image pulled successfully")

	return result, nil
}

// GetName implements api.Tool
func (t *dockerPullToolImpl) GetName() string {
	return "docker_pull"
}

// GetDescription implements api.Tool
func (t *dockerPullToolImpl) GetDescription() string {
	return "Pulls Docker images from registries with strongly-typed parameters and comprehensive error handling"
}

// Schema implements api.Tool
func (t *dockerPullToolImpl) Schema() tools.Schema[DockerPullParams, DockerPullResult] {
	return tools.Schema[DockerPullParams, DockerPullResult]{
		Name:        "docker_pull",
		Description: "Strongly-typed Docker image pull tool with RichError support",
		Version:     "2.0.0",
		ParamsSchema: tools.FromMap(map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"image": map[string]interface{}{
					"type":        "string",
					"description": "Docker image name to pull",
					"minLength":   1,
				},
				"tag": map[string]interface{}{
					"type":        "string",
					"description": "Image tag (default: latest)",
				},
				"platform": map[string]interface{}{
					"type":        "string",
					"description": "Target platform for multi-arch images",
				},
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session ID for tracking",
				},
			},
			"required": []string{"image"},
		}),
		ResultSchema: tools.FromMap(map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"success": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether the pull was successful",
				},
				"image_id": map[string]interface{}{
					"type":        "string",
					"description": "Pulled image ID",
				},
				"duration": map[string]interface{}{
					"type":        "string",
					"description": "Pull duration",
				},
			},
		}),
		Examples: []tools.Example[DockerPullParams, DockerPullResult]{
			{
				Name:        "pull_ubuntu",
				Description: "Pull Ubuntu image from Docker Hub",
				Params: DockerPullParams{
					Image:     "ubuntu",
					Tag:       "22.04",
					SessionID: "session-123",
				},
				Result: DockerPullResult{
					Success:   true,
					ImageID:   "sha256:def456...",
					Duration:  15 * time.Second,
					SessionID: "session-123",
				},
			},
		},
	}
}

// executePull performs the actual Docker pull operation
func (t *dockerPullToolImpl) executePull(ctx context.Context, imageRef string) error {
	// This would integrate with the existing pipeline adapter
	// For now, we'll simulate the operation
	t.logger.Info().Str("image", imageRef).Msg("Pulling Docker image")

	// In real implementation, this would use:
	// return t.pipelineAdapter.PullImage(ctx, imageRef)

	// For demonstration, we'll just validate the image reference format
	if imageRef == "" {
		return errors.NewError().
			Code(errors.CodeInvalidParameter).
			Message("Empty image reference").
			Type(errors.ErrTypeValidation).
			Build()
	}

	return nil // Success for demonstration
}
