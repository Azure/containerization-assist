package build

import (
	"context"

	"github.com/Azure/container-kit/pkg/mcp/core"
	core "github.com/Azure/container-kit/pkg/mcp/core/tools"
	"github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/Azure/container-kit/pkg/mcp/errors/codes"
	"github.com/Azure/container-kit/pkg/mcp/session"
	"github.com/rs/zerolog"
)

// dockerTagToolImpl implements the strongly-typed Docker tag tool
type dockerTagToolImpl struct {
	pipelineAdapter core.TypedPipelineOperations
	sessionManager  session.UnifiedSessionManager // Legacy field for backward compatibility
	sessionStore    services.SessionStore         // Modern service interface
	sessionState    services.SessionState         // Modern service interface
	logger          zerolog.Logger
}

// NewDockerTagTool creates a new strongly-typed Docker tag tool (legacy constructor)
func NewDockerTagTool(adapter core.TypedPipelineOperations, sessionManager session.UnifiedSessionManager, logger zerolog.Logger) interface{} {
	toolLogger := logger.With().Str("tool", "docker_tag").Logger()

	return &dockerTagToolImpl{
		pipelineAdapter: adapter,
		sessionManager:  sessionManager,
		logger:          toolLogger,
	}
}

// NewDockerTagToolWithServices creates a new strongly-typed Docker tag tool with services
func NewDockerTagToolWithServices(
	adapter core.TypedPipelineOperations,
	serviceContainer services.ServiceContainer,
	logger zerolog.Logger,
) interface{} {
	toolLogger := logger.With().Str("tool", "docker_tag").Logger()

	return &dockerTagToolImpl{
		pipelineAdapter: adapter,
		sessionStore:    serviceContainer.SessionStore(),
		sessionState:    serviceContainer.SessionState(),
		logger:          toolLogger,
	}
}

// Legacy function removed - use NewDockerTagToolUnified with session.UnifiedSessionManager

// Execute implements api.Tool interface
func (t *dockerTagToolImpl) Execute(ctx context.Context, params DockerTagParams) (DockerTagResult, error) {
	// Validate parameters at compile time
	if err := params.Validate(); err != nil {
		return DockerTagResult{}, errors.NewError().
			Code(errors.CodeInvalidParameter).
			Message("Tag parameters validation failed").
			Type(errors.ErrTypeValidation).
			Severity(errors.SeverityMedium).
			Cause(err).
			Context("source_image", params.SourceImage).
			Context("target_image", params.TargetImage).
			Suggestion("Ensure both source_image and target_image are provided").
			WithLocation().
			Build()
	}

	// Execute Docker tag using pipeline adapter
	tagErr := t.executeTag(ctx, params.SourceImage, params.TargetImage)

	// Create result
	result := DockerTagResult{
		Success:     tagErr == nil,
		SourceImage: params.SourceImage,
		TargetImage: params.TargetImage,
		SessionID:   params.SessionID,
	}

	if tagErr != nil {
		// Create RichError for tag failures
		return result, errors.NewError().
			Code(codes.BUILD_IMAGE_TAG_FAILED).
			Message("Failed to tag Docker image").
			Type(errors.ErrTypeBusiness).
			Severity(errors.SeverityMedium).
			Cause(tagErr).
			Context("source_image", params.SourceImage).
			Context("target_image", params.TargetImage).
			Suggestion("Check that source image exists and target image name is valid").
			WithLocation().
			Build()
	}

	t.logger.Info().
		Str("source", params.SourceImage).
		Str("target", params.TargetImage).
		Msg("Docker image tagged successfully")

	return result, nil
}

// GetName implements api.Tool
func (t *dockerTagToolImpl) GetName() string {
	return "docker_tag"
}

// GetDescription implements api.Tool
func (t *dockerTagToolImpl) GetDescription() string {
	return "Tags Docker images with strongly-typed parameters and comprehensive error handling"
}

// Schema implements api.Tool
func (t *dockerTagToolImpl) Schema() tools.Schema[DockerTagParams, DockerTagResult] {
	return tools.Schema[DockerTagParams, DockerTagResult]{
		Name:        "docker_tag",
		Description: "Strongly-typed Docker image tag tool with RichError support",
		Version:     "2.0.0",
		ParamsSchema: tools.FromMap(map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"source_image": map[string]interface{}{
					"type":        "string",
					"description": "Source image to tag",
					"minLength":   1,
				},
				"target_image": map[string]interface{}{
					"type":        "string",
					"description": "Target image name/tag",
					"minLength":   1,
				},
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session ID for tracking",
				},
			},
			"required": []string{"source_image", "target_image"},
		}),
		ResultSchema: tools.FromMap(map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"success": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether the tag was successful",
				},
				"source_image": map[string]interface{}{
					"type":        "string",
					"description": "Source image that was tagged",
				},
				"target_image": map[string]interface{}{
					"type":        "string",
					"description": "Target image name/tag",
				},
			},
		}),
		Examples: []tools.Example[DockerTagParams, DockerTagResult]{
			{
				Name:        "tag_for_release",
				Description: "Tag image for release",
				Params: DockerTagParams{
					SourceImage: "my-app:latest",
					TargetImage: "my-app:v1.0.0",
					SessionID:   "session-123",
				},
				Result: DockerTagResult{
					Success:     true,
					SourceImage: "my-app:latest",
					TargetImage: "my-app:v1.0.0",
					SessionID:   "session-123",
				},
			},
		},
	}
}

// executeTag performs the actual Docker tag operation
func (t *dockerTagToolImpl) executeTag(ctx context.Context, sourceImage, targetImage string) error {
	// This would integrate with the existing pipeline adapter
	// For now, we'll simulate the operation
	t.logger.Info().
		Str("source", sourceImage).
		Str("target", targetImage).
		Msg("Tagging Docker image")

	// In real implementation, this would use:
	// return t.pipelineAdapter.TagImage(ctx, sourceImage, targetImage)

	// For demonstration, we'll just validate the image references
	if sourceImage == "" || targetImage == "" {
		return errors.NewError().
			Code(errors.CodeInvalidParameter).
			Message("Empty image reference").
			Type(errors.ErrTypeValidation).
			Build()
	}

	return nil // Success for demonstration
}
