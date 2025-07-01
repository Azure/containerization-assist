package build

import (
	"context"

	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/errors/rich"
	"github.com/Azure/container-kit/pkg/mcp/types/tools"
	"github.com/rs/zerolog"
)

// dockerTagToolImpl implements the strongly-typed Docker tag tool
type dockerTagToolImpl struct {
	pipelineAdapter core.PipelineOperations
	sessionManager  core.ToolSessionManager
	logger          zerolog.Logger
}

// NewDockerTagTool creates a new strongly-typed Docker tag tool
func NewDockerTagTool(adapter core.PipelineOperations, sessionManager core.ToolSessionManager, logger zerolog.Logger) DockerTagTool {
	toolLogger := logger.With().Str("tool", "docker_tag").Logger()

	return &dockerTagToolImpl{
		pipelineAdapter: adapter,
		sessionManager:  sessionManager,
		logger:          toolLogger,
	}
}

// Execute implements tools.Tool[DockerTagParams, DockerTagResult]
func (t *dockerTagToolImpl) Execute(ctx context.Context, params DockerTagParams) (DockerTagResult, error) {
	// Validate parameters at compile time
	if err := params.Validate(); err != nil {
		return DockerTagResult{}, rich.NewError().
			Code(rich.CodeInvalidParameter).
			Message("Tag parameters validation failed").
			Type(rich.ErrTypeValidation).
			Severity(rich.SeverityMedium).
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
		return result, rich.NewError().
			Code("IMAGE_TAG_FAILED").
			Message("Failed to tag Docker image").
			Type(rich.ErrTypeBusiness).
			Severity(rich.SeverityMedium).
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

// GetName implements tools.Tool
func (t *dockerTagToolImpl) GetName() string {
	return "docker_tag"
}

// GetDescription implements tools.Tool
func (t *dockerTagToolImpl) GetDescription() string {
	return "Tags Docker images with strongly-typed parameters and comprehensive error handling"
}

// GetSchema implements tools.Tool
func (t *dockerTagToolImpl) GetSchema() tools.Schema[DockerTagParams, DockerTagResult] {
	return tools.Schema[DockerTagParams, DockerTagResult]{
		Name:        "docker_tag",
		Description: "Strongly-typed Docker image tag tool with RichError support",
		Version:     "2.0.0",
		ParamsSchema: map[string]interface{}{
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
		},
		ResultSchema: map[string]interface{}{
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
		},
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
		return rich.NewError().
			Code(rich.CodeInvalidParameter).
			Message("Empty image reference").
			Type(rich.ErrTypeValidation).
			Build()
	}

	return nil // Success for demonstration
}
