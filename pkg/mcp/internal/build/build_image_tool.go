package build

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/errors/rich"
	"github.com/Azure/container-kit/pkg/mcp/types/tools"
	"github.com/rs/zerolog"
)

// dockerBuildToolImpl implements the strongly-typed Docker build tool
type dockerBuildToolImpl struct {
	pipelineAdapter core.PipelineOperations
	sessionManager  core.ToolSessionManager
	logger          zerolog.Logger
	// Legacy components for backward compatibility
	contextAnalyzer *BuildContextAnalyzer
	validator       *BuildValidatorImpl
	executor        *BuildExecutorService
}

// NewDockerBuildTool creates a new strongly-typed Docker build tool
func NewDockerBuildTool(adapter core.PipelineOperations, sessionManager core.ToolSessionManager, logger zerolog.Logger) DockerBuildTool {
	toolLogger := logger.With().Str("tool", "docker_build").Logger()

	// Initialize legacy components for backward compatibility
	contextAnalyzer := NewBuildContextAnalyzer(toolLogger)
	validator := NewBuildValidator(toolLogger)
	executor := NewBuildExecutor(adapter, sessionManager, toolLogger)

	return &dockerBuildToolImpl{
		pipelineAdapter: adapter,
		sessionManager:  sessionManager,
		logger:          toolLogger,
		contextAnalyzer: contextAnalyzer,
		validator:       validator,
		executor:        executor,
	}
}

// Execute implements tools.Tool[DockerBuildParams, DockerBuildResult]
func (t *dockerBuildToolImpl) Execute(ctx context.Context, params DockerBuildParams) (DockerBuildResult, error) {
	startTime := time.Now()

	// Validate parameters at compile time
	if err := params.Validate(); err != nil {
		return DockerBuildResult{}, rich.NewError().
			Code(rich.CodeInvalidParameter).
			Message("Build parameters validation failed").
			Type(rich.ErrTypeValidation).
			Severity(rich.SeverityMedium).
			Cause(err).
			Context("dockerfile_path", params.DockerfilePath).
			Context("context_path", params.ContextPath).
			Suggestion("Check that dockerfile_path and context_path are provided and valid").
			WithLocation().
			Build()
	}

	// Convert to legacy format for backward compatibility
	legacyArgs := t.convertToLegacyArgs(params)

	// Create a legacy result structure for internal processing
	legacyResult := &AtomicBuildImageResult{
		SessionID: params.SessionID,
		ImageName: extractImageName(params.Tags),
		ImageTag:  extractImageTag(params.Tags),
		Platform:  params.Platform,
		Success:   false,
	}

	// Execute the build using existing infrastructure
	err := t.executor.executeWithProgress(ctx, legacyArgs, legacyResult, startTime, nil)

	// Convert result to strongly-typed format
	result := DockerBuildResult{
		Success:   legacyResult.Success,
		Duration:  time.Since(startTime),
		SessionID: params.SessionID,
		Tags:      params.Tags,
	}

	if legacyResult.BuildResult != nil {
		result.ImageID = legacyResult.BuildResult.ImageID
		result.ImageSize = legacyResult.BuildResult.Size
		result.BuildLog = legacyResult.BuildResult.BuildLogs
		result.CacheHits = legacyResult.BuildResult.CacheStats.Hits
		result.CacheMisses = legacyResult.BuildResult.CacheStats.Misses
	}

	if err != nil {
		// Create RichError with Docker build context
		return result, rich.DockerBuildGenericError(
			extractImageName(params.Tags),
			params.DockerfilePath,
			err,
		)
	}

	if !result.Success {
		// Create error for failed build even without exception
		return result, rich.NewError().
			Code(rich.CodeImageBuildFailed).
			Message("Docker image build failed").
			Type(rich.ErrTypeBusiness).
			Severity(rich.SeverityHigh).
			Context("dockerfile_path", params.DockerfilePath).
			Context("context_path", params.ContextPath).
			Context("image_tags", params.Tags).
			Context("build_duration", result.Duration.String()).
			Suggestion("Check build logs for specific errors").
			WithLocation().
			Build()
	}

	return result, nil
}

// GetName implements tools.Tool
func (t *dockerBuildToolImpl) GetName() string {
	return "docker_build"
}

// GetDescription implements tools.Tool
func (t *dockerBuildToolImpl) GetDescription() string {
	return "Builds Docker images with strongly-typed parameters and comprehensive error handling"
}

// GetSchema implements tools.Tool
func (t *dockerBuildToolImpl) GetSchema() tools.Schema[DockerBuildParams, DockerBuildResult] {
	return tools.Schema[DockerBuildParams, DockerBuildResult]{
		Name:        "docker_build",
		Description: "Strongly-typed Docker image build tool with RichError support",
		Version:     "2.0.0",
		ParamsSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"dockerfile_path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the Dockerfile",
					"minLength":   1,
				},
				"context_path": map[string]interface{}{
					"type":        "string",
					"description": "Build context directory path",
					"minLength":   1,
				},
				"build_args": map[string]interface{}{
					"type":        "object",
					"description": "Docker build arguments",
				},
				"tags": map[string]interface{}{
					"type":        "array",
					"description": "Image tags to apply",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"no_cache": map[string]interface{}{
					"type":        "boolean",
					"description": "Build without using cache",
				},
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session ID for workspace management",
				},
			},
			"required": []string{"dockerfile_path", "context_path"},
		},
		ResultSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"success": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether the build was successful",
				},
				"image_id": map[string]interface{}{
					"type":        "string",
					"description": "Built image ID",
				},
				"duration": map[string]interface{}{
					"type":        "string",
					"description": "Build duration",
				},
				"build_log": map[string]interface{}{
					"type":        "array",
					"description": "Build log output",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
			},
		},
		Examples: []tools.Example[DockerBuildParams, DockerBuildResult]{
			{
				Name:        "basic_docker_build",
				Description: "Build a Docker image from a Dockerfile",
				Params: DockerBuildParams{
					DockerfilePath: "./Dockerfile",
					ContextPath:    ".",
					Tags:           []string{"my-app:latest"},
					SessionID:      "session-123",
				},
				Result: DockerBuildResult{
					Success:   true,
					ImageID:   "sha256:abc123...",
					Duration:  30 * time.Second,
					SessionID: "session-123",
					Tags:      []string{"my-app:latest"},
				},
			},
		},
	}
}

// Helper functions for backward compatibility

// convertToLegacyArgs converts new params to legacy format
func (t *dockerBuildToolImpl) convertToLegacyArgs(params DockerBuildParams) AtomicBuildImageArgs {
	imageName, imageTag := extractImageNameAndTag(params.Tags)

	return AtomicBuildImageArgs{
		ImageName:      imageName,
		ImageTag:       imageTag,
		DockerfilePath: params.DockerfilePath,
		BuildContext:   params.ContextPath,
		Platform:       params.Platform,
		NoCache:        params.NoCache,
		BuildArgs:      params.BuildArgs,
	}
}

// extractImageName extracts the image name from tags
func extractImageName(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	// Extract name from first tag (before colon)
	tag := tags[0]
	if colonIndex := fmt.Sprintf("%s", tag); len(colonIndex) > 0 {
		for i, char := range tag {
			if char == ':' {
				return tag[:i]
			}
		}
	}
	return tag
}

// extractImageTag extracts the tag from tags
func extractImageTag(tags []string) string {
	if len(tags) == 0 {
		return "latest"
	}
	// Extract tag from first tag (after colon)
	tag := tags[0]
	for i, char := range tag {
		if char == ':' && i+1 < len(tag) {
			return tag[i+1:]
		}
	}
	return "latest"
}

// extractImageNameAndTag extracts both name and tag
func extractImageNameAndTag(tags []string) (string, string) {
	return extractImageName(tags), extractImageTag(tags)
}
