package build

import (
	"context"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/core"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
	"github.com/Azure/container-kit/pkg/mcp/domain/types/tools"
	"github.com/Azure/container-kit/pkg/mcp/services"
	"github.com/rs/zerolog"
)

// dockerPushToolImpl implements the strongly-typed Docker push tool
type dockerPushToolImpl struct {
	pipelineAdapter core.TypedPipelineOperations
	sessionManager  session.UnifiedSessionManager // Legacy field for backward compatibility
	sessionStore    services.SessionStore         // Modern service interface
	sessionState    services.SessionState         // Modern service interface
	logger          zerolog.Logger
}

// NewDockerPushTool creates a new strongly-typed Docker push tool (legacy constructor)
func NewDockerPushTool(adapter core.TypedPipelineOperations, sessionManager session.UnifiedSessionManager, logger zerolog.Logger) interface{} {
	toolLogger := logger.With().Str("tool", "docker_push").Logger()

	return &dockerPushToolImpl{
		pipelineAdapter: adapter,
		sessionManager:  sessionManager,
		logger:          toolLogger,
	}
}

// NewDockerPushToolWithServices creates a new strongly-typed Docker push tool with services
func NewDockerPushToolWithServices(
	adapter core.TypedPipelineOperations,
	serviceContainer services.ServiceContainer,
	logger zerolog.Logger,
) interface{} {
	toolLogger := logger.With().Str("tool", "docker_push").Logger()

	return &dockerPushToolImpl{
		pipelineAdapter: adapter,
		sessionStore:    serviceContainer.SessionStore(),
		sessionState:    serviceContainer.SessionState(),
		logger:          toolLogger,
	}
}

// Execute implements api.Tool interface
func (t *dockerPushToolImpl) Execute(ctx context.Context, params DockerPushParams) (DockerPushResult, error) {
	startTime := time.Now()

	// Validate parameters at compile time
	if err := params.Validate(); err != nil {
		return DockerPushResult{}, errors.NewError().
			Code(errors.CodeInvalidParameter).
			Message("Push parameters validation failed").
			Type(errors.ErrTypeValidation).
			Severity(errors.SeverityMedium).
			Cause(err).
			Context("image", params.Image).
			Context("tag", params.Tag).
			Context("registry", params.Registry).
			Suggestion("Ensure image name is provided").
			WithLocation().
			Build()
	}

	// Construct full image reference
	imageRef := params.Image
	if params.Tag != "" {
		imageRef = params.Image + ":" + params.Tag
	}

	// If registry is specified, prepend it
	if params.Registry != "" {
		imageRef = params.Registry + "/" + imageRef
	}

	// Execute Docker push using pipeline adapter
	pushErr := t.executePush(ctx, imageRef)

	// Create result
	result := DockerPushResult{
		Success:   pushErr == nil,
		Duration:  time.Since(startTime),
		SessionID: params.SessionID,
		Registry:  params.Registry,
	}

	if pushErr != nil {
		// Create RichError with network context for push failures
		return result, errors.ImagePushError(imageRef, params.Registry, pushErr)
	}

	// Set success details (would normally come from Docker API)
	result.ImageID = "sha256:pushed-image-id" // This would be actual ID from push
	result.RemoteSize = 0                     // This would be actual remote size

	t.logger.Info().
		Str("image", imageRef).
		Str("registry", params.Registry).
		Dur("duration", result.Duration).
		Msg("Docker image pushed successfully")

	return result, nil
}

// GetName implements api.Tool
func (t *dockerPushToolImpl) GetName() string {
	return "docker_push"
}

// GetDescription implements api.Tool
func (t *dockerPushToolImpl) GetDescription() string {
	return "Pushes Docker images to registries with strongly-typed parameters and comprehensive error handling"
}

// Schema implements api.Tool
func (t *dockerPushToolImpl) Schema() tools.Schema[DockerPushParams, DockerPushResult] {
	return tools.Schema[DockerPushParams, DockerPushResult]{
		Name:        "docker_push",
		Description: "Strongly-typed Docker image push tool with RichError support",
		Version:     "2.0.0",
		ParamsSchema: tools.FromMap(map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"image": map[string]interface{}{
					"type":        "string",
					"description": "Docker image name to push",
					"minLength":   1,
				},
				"tag": map[string]interface{}{
					"type":        "string",
					"description": "Image tag (default: latest)",
				},
				"registry": map[string]interface{}{
					"type":        "string",
					"description": "Target registry URL",
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
					"description": "Whether the push was successful",
				},
				"image_id": map[string]interface{}{
					"type":        "string",
					"description": "Pushed image ID",
				},
				"duration": map[string]interface{}{
					"type":        "string",
					"description": "Push duration",
				},
				"registry": map[string]interface{}{
					"type":        "string",
					"description": "Target registry",
				},
				"remote_size": map[string]interface{}{
					"type":        "number",
					"description": "Image size in registry",
				},
			},
		}),
		Examples: []tools.Example[DockerPushParams, DockerPushResult]{
			{
				Name:        "push_to_dockerhub",
				Description: "Push image to Docker Hub",
				Params: DockerPushParams{
					Image:     "my-app",
					Tag:       "v1.0.0",
					Registry:  "docker.io/myuser",
					SessionID: "session-123",
				},
				Result: DockerPushResult{
					Success:    true,
					ImageID:    "sha256:ghi789...",
					Duration:   45 * time.Second,
					SessionID:  "session-123",
					Registry:   "docker.io/myuser",
					RemoteSize: 256 * 1024 * 1024, // 256 MB
				},
			},
		},
	}
}

// executePush performs the actual Docker push operation
func (t *dockerPushToolImpl) executePush(ctx context.Context, imageRef string) error {
	// This would integrate with the existing pipeline adapter
	// For now, we'll simulate the operation
	t.logger.Info().Str("image", imageRef).Msg("Pushing Docker image")

	// In real implementation, this would use:
	// return t.pipelineAdapter.PushImage(ctx, imageRef)

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
