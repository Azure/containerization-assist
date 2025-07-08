package build

import (
	"context"
	"fmt"
	"time"

	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/core"
	core "github.com/Azure/container-kit/pkg/mcp/core/tools"
	"github.com/Azure/container-kit/pkg/mcp/errors"
)

// dockerBuildToolImpl implements the strongly-typed Docker build tool using services
type dockerBuildToolImpl struct {
	pipelineAdapter core.TypedPipelineOperations
	sessionStore    services.SessionStore
	sessionState    services.SessionState
	buildExecutor   services.BuildExecutor
	logger          *slog.Logger
	// Legacy components for backward compatibility
	contextAnalyzer *BuildContextAnalyzer
	validator       *BuildValidatorImpl
	executor        *BuildExecutorService
}

// NewDockerBuildTool creates a new strongly-typed Docker build tool using service container
func NewDockerBuildTool(adapter core.TypedPipelineOperations, container services.ServiceContainer, logger *slog.Logger) api.Tool {
	toolLogger := logger.With("tool", "docker_build")

	// Initialize legacy components for backward compatibility
	contextAnalyzer := NewBuildContextAnalyzer(toolLogger)
	validator := NewBuildValidator(toolLogger)
	executor := NewBuildExecutor(adapter, nil, toolLogger) // Pass nil for session manager in service mode

	return &dockerBuildToolImpl{
		pipelineAdapter: adapter,
		sessionStore:    container.SessionStore(),
		sessionState:    container.SessionState(),
		buildExecutor:   container.BuildExecutor(),
		logger:          toolLogger,
		contextAnalyzer: contextAnalyzer,
		validator:       validator,
		executor:        executor,
	}
}

// Execute implements api.Tool interface
func (t *dockerBuildToolImpl) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	// Extract params from ToolInput
	var buildParams *tools.BuildToolParams
	if rawParams, ok := input.Data["params"]; ok {
		if typedParams, ok := rawParams.(*tools.BuildToolParams); ok {
			buildParams = typedParams
		} else {
			return api.ToolOutput{
					Success: false,
					Error:   "Invalid input type for build tool",
				}, errors.NewError().
					Code(errors.CodeInvalidParameter).
					Message("Invalid input type for build tool").
					Type(errors.ErrTypeValidation).
					Severity(errors.SeverityHigh).
					Context("tool", "docker_build").
					Context("operation", "type_assertion").
					Build()
		}
	} else {
		return api.ToolOutput{
				Success: false,
				Error:   "No params provided",
			}, errors.NewError().
				Code(errors.CodeInvalidParameter).
				Message("No params provided").
				Type(errors.ErrTypeValidation).
				Severity(errors.SeverityHigh).
				Build()
	}

	// Use session ID from input if available
	if input.SessionID != "" {
		buildParams.SessionID = input.SessionID
	}
	startTime := time.Now()

	// Validate parameters at compile time
	if err := buildParams.Validate(); err != nil {
		return api.ToolOutput{
				Success: false,
				Error:   "Build parameters validation failed",
			}, errors.NewError().
				Code(errors.CodeInvalidParameter).
				Message("Build parameters validation failed").
				Type(errors.ErrTypeValidation).
				Severity(errors.SeverityMedium).
				Cause(err).
				Context("dockerfile_path", buildParams.DockerfilePath).
				Context("context_path", buildParams.ContextPath).
				Suggestion("Check that dockerfile_path and context_path are provided and valid").
				WithLocation().
				Build()
	}

	// Convert to internal format for backward compatibility
	dockerParams := t.convertToDockerBuildParams(buildParams)
	legacyArgs := t.convertToLegacyArgs(dockerParams)

	// Create a legacy result structure for internal processing
	legacyResult := &AtomicBuildImageResult{
		SessionID: buildParams.SessionID,
		ImageName: extractImageName(buildParams.Tags),
		ImageTag:  extractImageTag(buildParams.Tags),
		Platform:  buildParams.Platform,
		Success:   false,
	}

	// Execute the build using existing infrastructure
	err := t.executor.executeWithProgress(context.Background(), legacyArgs, legacyResult, startTime, nil)

	// Convert result to strongly-typed format
	result := DockerBuildResult{
		Success:   legacyResult.Success,
		Duration:  time.Since(startTime),
		SessionID: buildParams.SessionID,
		Tags:      buildParams.Tags,
	}

	if legacyResult.BuildResult != nil {
		result.ImageID = legacyResult.BuildResult.ImageID
		result.ImageSize = 0 // Size would come from Docker API in real implementation
		result.BuildLog = legacyResult.BuildResult.Logs
		result.CacheHits = 0   // Cache stats would come from Docker API
		result.CacheMisses = 0 // Cache stats would come from Docker API
	}

	if err != nil {
		// Create RichError with Docker build context
		return api.ToolOutput{
				Success: false,
				Data:    map[string]interface{}{"result": &result},
				Error:   err.Error(),
			}, errors.DockerBuildGenericError(
				"Docker build failed",
				map[string]interface{}{
					"image_name":      extractImageName(buildParams.Tags),
					"dockerfile_path": buildParams.DockerfilePath,
					"error":           err.Error(),
				},
			)
	}

	if !result.Success {
		// Create error for failed build even without exception
		return api.ToolOutput{
				Success: false,
				Data:    map[string]interface{}{"result": &result},
				Error:   "Docker image build failed",
			}, errors.NewError().
				Code(errors.CodeImageBuildFailed).
				Message("Docker image build failed").
				Type(errors.ErrTypeBusiness).
				Severity(errors.SeverityHigh).
				Context("dockerfile_path", buildParams.DockerfilePath).
				Context("context_path", buildParams.ContextPath).
				Context("image_tags", buildParams.Tags).
				Context("build_duration", result.Duration.String()).
				Suggestion("Check build logs for specific errors").
				WithLocation().
				Build()
	}

	return api.ToolOutput{
		Success: true,
		Data:    map[string]interface{}{"result": &result},
	}, nil
}

// Name implements api.Tool interface
func (t *dockerBuildToolImpl) Name() string {
	return "docker_build"
}

// Description provides a description of the tool
func (t *dockerBuildToolImpl) Description() string {
	return "Builds Docker images with strongly-typed parameters and comprehensive error handling using session-based context"
}

// Schema implements api.Tool interface
func (t *dockerBuildToolImpl) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        "docker_build",
		Description: "Builds Docker images with strongly-typed parameters and comprehensive error handling using session-based context",
		Version:     "2.0.0",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"params": map[string]interface{}{
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
			},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"result": map[string]interface{}{
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
			},
		},
	}
}

// GetLegacySchema implements tools.Tool
func (t *dockerBuildToolImpl) GetLegacySchema() tools.Schema[DockerBuildParams, DockerBuildResult] {
	return tools.Schema[DockerBuildParams, DockerBuildResult]{
		Name:        "docker_build",
		Description: "Strongly-typed Docker image build tool with RichError support",
		Version:     "2.0.0",
		ParamsSchema: tools.FromMap(map[string]interface{}{
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
		}),
		ResultSchema: tools.FromMap(map[string]interface{}{
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
		}),
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

// convertToDockerBuildParams converts tools.BuildToolParams to internal DockerBuildParams
func (t *dockerBuildToolImpl) convertToDockerBuildParams(params *tools.BuildToolParams) DockerBuildParams {
	return DockerBuildParams{
		DockerfilePath: params.DockerfilePath,
		ContextPath:    params.ContextPath,
		BuildArgs:      params.BuildArgs,
		Tags:           params.Tags,
		NoCache:        params.NoCache,
		SessionID:      params.SessionID,
		Target:         params.Target,
		Platform:       params.Platform,
		BuildKit:       false, // Default to false for now
	}
}

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
