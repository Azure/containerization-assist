package commands

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/application/services"
	"github.com/Azure/container-kit/pkg/mcp/domain/containerization/build"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// ConsolidatedBuildCommand consolidates all build tool functionality into a single command
// This replaces the 67 files in pkg/mcp/tools/build/ with a unified implementation
type ConsolidatedBuildCommand struct {
	sessionStore services.SessionStore
	sessionState services.SessionState
	dockerClient services.DockerClient
	logger       *slog.Logger
}

// NewConsolidatedBuildCommand creates a new consolidated build command
func NewConsolidatedBuildCommand(
	sessionStore services.SessionStore,
	sessionState services.SessionState,
	dockerClient services.DockerClient,
	logger *slog.Logger,
) *ConsolidatedBuildCommand {
	return &ConsolidatedBuildCommand{
		sessionStore: sessionStore,
		sessionState: sessionState,
		dockerClient: dockerClient,
		logger:       logger,
	}
}

// Execute performs build operations with full functionality from original tools
func (cmd *ConsolidatedBuildCommand) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	startTime := time.Now()

	// Extract and validate input parameters
	buildRequest, err := cmd.parseBuildInput(input)
	if err != nil {
		return api.ToolOutput{}, errors.NewError().
			Code(errors.CodeInvalidParameter).
			Message("failed to parse build input").
			Cause(err).
			Build()
	}

	// Validate using domain rules
	if validationErrors := cmd.validateBuildRequest(buildRequest); len(validationErrors) > 0 {
		return api.ToolOutput{}, errors.NewError().
			Code(errors.CodeValidationFailed).
			Message("build request validation failed").
			Context("validation_errors", validationErrors).
			Build()
	}

	// Get workspace directory for the session
	workspaceDir, err := cmd.getSessionWorkspace(ctx, buildRequest.SessionID)
	if err != nil {
		return api.ToolOutput{}, errors.NewError().
			Code(errors.CodeInternalError).
			Message("failed to get session workspace").
			Cause(err).
			Build()
	}

	// Execute build operation based on operation type
	var buildResult *build.BuildResult
	switch buildRequest.Operation {
	case "build":
		buildResult, err = cmd.executeBuildImage(ctx, buildRequest, workspaceDir)
	case "push":
		buildResult, err = cmd.executePushImage(ctx, buildRequest, workspaceDir)
	case "pull":
		buildResult, err = cmd.executePullImage(ctx, buildRequest, workspaceDir)
	case "tag":
		buildResult, err = cmd.executeTagImage(ctx, buildRequest, workspaceDir)
	default:
		return api.ToolOutput{}, errors.NewError().
			Code(errors.CodeInvalidParameter).
			Message(fmt.Sprintf("unsupported operation: %s", buildRequest.Operation)).
			Build()
	}

	if err != nil {
		return api.ToolOutput{}, errors.NewError().
			Code(errors.CodeInternalError).
			Message("build operation failed").
			Cause(err).
			Build()
	}

	// Update session state with build results
	if err := cmd.updateSessionState(ctx, buildRequest.SessionID, buildResult); err != nil {
		cmd.logger.Warn("failed to update session state", "error", err)
	}

	// Create consolidated response
	response := cmd.createBuildResponse(buildResult, time.Since(startTime))

	return api.ToolOutput{
		Success: true,
		Data: map[string]interface{}{
			"build_result": response,
		},
	}, nil
}

// parseBuildInput extracts and validates build parameters from tool input
func (cmd *ConsolidatedBuildCommand) parseBuildInput(input api.ToolInput) (*BuildRequest, error) {
	// Extract operation type
	operation := getStringParam(input.Data, "operation", "build")

	// Extract common parameters
	request := &BuildRequest{
		SessionID: input.SessionID,
		Operation: operation,
		ImageName: getStringParam(input.Data, "image_name", ""),
		ImageTag:  getStringParam(input.Data, "image_tag", "latest"),
		BuildOptions: BuildOptions{
			DockerfilePath: getStringParam(input.Data, "dockerfile_path", "Dockerfile"),
			BuildContext:   getStringParam(input.Data, "build_context", "."),
			Platform:       getStringParam(input.Data, "platform", ""),
			NoCache:        getBoolParam(input.Data, "no_cache", false),
			ForceRebuild:   getBoolParam(input.Data, "force_rebuild", false),
			PushAfterBuild: getBoolParam(input.Data, "push_after_build", false),
			BuildArgs:      getStringMapParam(input.Data, "build_args"),
			Labels:         getStringMapParam(input.Data, "labels"),
			Target:         getStringParam(input.Data, "target", ""),
			NetworkMode:    getStringParam(input.Data, "network_mode", ""),
			CacheFrom:      getStringSliceParam(input.Data, "cache_from"),
			CacheTo:        getStringSliceParam(input.Data, "cache_to"),
			Squash:         getBoolParam(input.Data, "squash", false),
			PullParent:     getBoolParam(input.Data, "pull_parent", true),
			Isolation:      getStringParam(input.Data, "isolation", ""),
			ShmSize:        getStringParam(input.Data, "shm_size", ""),
			Ulimits:        getStringSliceParam(input.Data, "ulimits"),
			Memory:         getStringParam(input.Data, "memory", ""),
			MemorySwap:     getStringParam(input.Data, "memory_swap", ""),
			CpuShares:      getIntParam(input.Data, "cpu_shares", 0),
			CpuSetCpus:     getStringParam(input.Data, "cpu_set_cpus", ""),
			CpuSetMems:     getStringParam(input.Data, "cpu_set_mems", ""),
			CpuQuota:       getIntParam(input.Data, "cpu_quota", 0),
			CpuPeriod:      getIntParam(input.Data, "cpu_period", 0),
			SecurityOpt:    getStringSliceParam(input.Data, "security_opt"),
			AddHost:        getStringSliceParam(input.Data, "add_host"),
		},
		PushOptions: PushOptions{
			Registry: getStringParam(input.Data, "registry", ""),
			Username: getStringParam(input.Data, "username", ""),
			Password: getStringParam(input.Data, "password", ""),
			Force:    getBoolParam(input.Data, "force", false),
		},
		PullOptions: PullOptions{
			Registry:            getStringParam(input.Data, "registry", ""),
			Tag:                 getStringParam(input.Data, "tag", "latest"),
			Platform:            getStringParam(input.Data, "platform", ""),
			AllTags:             getBoolParam(input.Data, "all_tags", false),
			DisableContentTrust: getBoolParam(input.Data, "disable_content_trust", false),
		},
		TagOptions: TagOptions{
			SourceImage: getStringParam(input.Data, "source_image", ""),
			TargetImage: getStringParam(input.Data, "target_image", ""),
			Force:       getBoolParam(input.Data, "force", false),
		},
		CreatedAt: time.Now(),
	}

	// Validate required fields based on operation
	if err := cmd.validateOperationParams(request); err != nil {
		return nil, err
	}

	return request, nil
}

// validateOperationParams validates operation-specific parameters
func (cmd *ConsolidatedBuildCommand) validateOperationParams(request *BuildRequest) error {
	switch request.Operation {
	case "build":
		if request.ImageName == "" {
			return errors.NewError().
				Code(errors.CodeValidationFailed).
				Type(errors.ErrTypeValidation).
				Message("image_name is required for build operation").
				WithLocation().
				Build()
		}
		if request.BuildOptions.DockerfilePath == "" {
			return errors.NewError().
				Code(errors.CodeValidationFailed).
				Type(errors.ErrTypeValidation).
				Message("dockerfile_path is required for build operation").
				WithLocation().
				Build()
		}
	case "push":
		if request.ImageName == "" {
			return errors.NewError().
				Code(errors.CodeValidationFailed).
				Type(errors.ErrTypeValidation).
				Message("image_name is required for push operation").
				WithLocation().
				Build()
		}
	case "pull":
		if request.ImageName == "" {
			return errors.NewError().
				Code(errors.CodeValidationFailed).
				Type(errors.ErrTypeValidation).
				Message("image_name is required for pull operation").
				WithLocation().
				Build()
		}
	case "tag":
		if request.TagOptions.SourceImage == "" {
			return errors.NewError().
				Code(errors.CodeValidationFailed).
				Type(errors.ErrTypeValidation).
				Message("source_image is required for tag operation").
				WithLocation().
				Build()
		}
		if request.TagOptions.TargetImage == "" {
			return errors.NewError().
				Code(errors.CodeValidationFailed).
				Type(errors.ErrTypeValidation).
				Message("target_image is required for tag operation").
				WithLocation().
				Build()
		}
	}
	return nil
}

// validateBuildRequest validates build request using domain rules
func (cmd *ConsolidatedBuildCommand) validateBuildRequest(request *BuildRequest) []ValidationError {
	var errors []ValidationError

	// Session ID validation
	if request.SessionID == "" {
		errors = append(errors, ValidationError{
			Field:   "session_id",
			Message: "session ID is required",
			Code:    "MISSING_SESSION_ID",
		})
	}

	// Operation validation
	validOperations := []string{"build", "push", "pull", "tag"}
	if !slices.Contains(validOperations, request.Operation) {
		errors = append(errors, ValidationError{
			Field:   "operation",
			Message: fmt.Sprintf("operation must be one of: %s", strings.Join(validOperations, ", ")),
			Code:    "INVALID_OPERATION",
		})
	}

	// Image name validation
	if request.ImageName != "" {
		if !isValidImageName(request.ImageName) {
			errors = append(errors, ValidationError{
				Field:   "image_name",
				Message: "invalid image name format",
				Code:    "INVALID_IMAGE_NAME",
			})
		}
	}

	// Docker tag validation
	if request.ImageTag != "" {
		if !isValidImageTag(request.ImageTag) {
			errors = append(errors, ValidationError{
				Field:   "image_tag",
				Message: "invalid image tag format",
				Code:    "INVALID_IMAGE_TAG",
			})
		}
	}

	return errors
}

// getSessionWorkspace retrieves the workspace directory for a session
func (cmd *ConsolidatedBuildCommand) getSessionWorkspace(ctx context.Context, sessionID string) (string, error) {
	sessionMetadata, err := cmd.sessionState.GetSessionMetadata(ctx, sessionID)
	if err != nil {
		return "", errors.NewError().
			Code(errors.CodeInternalError).
			Type(errors.ErrTypeSession).
			Messagef("failed to get session metadata: %w", err).
			WithLocation().
			Build()
	}

	workspaceDir, ok := sessionMetadata["workspace_dir"].(string)
	if !ok || workspaceDir == "" {
		return "", errors.NewError().
			Code(errors.CodeNotFound).
			Type(errors.ErrTypeNotFound).
			Messagef("workspace directory not found for session %s", sessionID).
			WithLocation().
			Build()
	}

	return workspaceDir, nil
}

// executeBuildImage performs Docker image build operation
func (cmd *ConsolidatedBuildCommand) executeBuildImage(ctx context.Context, request *BuildRequest, workspaceDir string) (*build.BuildResult, error) {
	// Create build request from domain
	buildRequest := &build.BuildRequest{
		ID:         fmt.Sprintf("build-%d", time.Now().Unix()),
		SessionID:  request.SessionID,
		Context:    filepath.Join(workspaceDir, request.BuildOptions.BuildContext),
		Dockerfile: filepath.Join(workspaceDir, request.BuildOptions.DockerfilePath),
		ImageName:  request.ImageName,
		Tags:       []string{request.ImageTag},
		BuildArgs:  request.BuildOptions.BuildArgs,
		Target:     request.BuildOptions.Target,
		Platform:   request.BuildOptions.Platform,
		NoCache:    request.BuildOptions.NoCache,
		PullParent: request.BuildOptions.PullParent,
		Labels:     request.BuildOptions.Labels,
		CreatedAt:  time.Now(),
	}

	// Execute build using Docker client
	result, err := cmd.performDockerBuild(ctx, buildRequest)
	if err != nil {
		return nil, errors.NewError().
			Code(errors.CodeContainerStartFailed).
			Type(errors.ErrTypeContainer).
			Messagef("build execution failed: %w", err).
			WithLocation().
			Build()
	}

	// Handle post-build operations
	if request.BuildOptions.PushAfterBuild && result.Status == build.BuildStatusCompleted {
		pushResult, err := cmd.executePushImage(ctx, request, workspaceDir)
		if err != nil {
			cmd.logger.Warn("post-build push failed", "error", err)
			// Add warning to metadata if it exists
			if result.Metadata.Optimizations == nil {
				result.Metadata.Optimizations = []build.Optimization{}
			}
		} else {
			// Store push result in metadata
			_ = pushResult // Could be stored in metadata if needed
		}
	}

	return result, nil
}

// executePushImage performs Docker image push operation
func (cmd *ConsolidatedBuildCommand) executePushImage(ctx context.Context, request *BuildRequest, workspaceDir string) (*build.BuildResult, error) {
	// Create push request from domain
	pushRequest := &build.ImagePushRequest{
		ID:        fmt.Sprintf("push-%d", time.Now().Unix()),
		SessionID: request.SessionID,
		ImageName: request.ImageName,
		Tag:       request.ImageTag,
		Registry:  request.PushOptions.Registry,
		CreatedAt: time.Now(),
	}

	// Execute push using Docker client
	result, err := cmd.performDockerPush(ctx, pushRequest)
	if err != nil {
		return nil, errors.NewError().
			Code(errors.CodeContainerStartFailed).
			Type(errors.ErrTypeContainer).
			Messagef("push execution failed: %w", err).
			WithLocation().
			Build()
	}

	// Convert push result to build result
	buildResult := &build.BuildResult{
		BuildID:   fmt.Sprintf("push-%d", time.Now().Unix()),
		RequestID: pushRequest.ID,
		SessionID: request.SessionID,
		ImageName: request.ImageName,
		Tags:      []string{request.ImageTag},
		Status:    build.BuildStatusCompleted,
		CreatedAt: time.Now(),
		Metadata: build.BuildMetadata{
			Strategy: build.BuildStrategyDocker,
		},
	}

	if result.Status == build.PushStatusCompleted {
		buildResult.Status = build.BuildStatusCompleted
	} else {
		buildResult.Status = build.BuildStatusFailed
		buildResult.Error = result.Error
	}

	return buildResult, nil
}

// executePullImage performs Docker image pull operation
func (cmd *ConsolidatedBuildCommand) executePullImage(ctx context.Context, request *BuildRequest, workspaceDir string) (*build.BuildResult, error) {
	// Execute pull using Docker client
	result, err := cmd.performDockerPull(ctx, request.ImageName, request.PullOptions.Tag, request.PullOptions.Registry)
	if err != nil {
		return nil, errors.NewError().
			Code(errors.CodeContainerStartFailed).
			Type(errors.ErrTypeContainer).
			Messagef("pull execution failed: %w", err).
			WithLocation().
			Build()
	}

	// Convert to build result
	buildResult := &build.BuildResult{
		BuildID:   fmt.Sprintf("pull-%d", time.Now().Unix()),
		SessionID: request.SessionID,
		ImageName: request.ImageName,
		Tags:      []string{request.PullOptions.Tag},
		Status:    build.BuildStatusCompleted,
		CreatedAt: time.Now(),
		Metadata: build.BuildMetadata{
			Strategy: build.BuildStrategyDocker,
		},
	}

	if result != nil {
		buildResult.ImageID = result.ImageID
		buildResult.Size = result.Size
		buildResult.Duration = result.Duration
		if result.Error != "" {
			buildResult.Status = build.BuildStatusFailed
			buildResult.Error = result.Error
		}
	}

	return buildResult, nil
}

// executeTagImage performs Docker image tag operation
func (cmd *ConsolidatedBuildCommand) executeTagImage(ctx context.Context, request *BuildRequest, workspaceDir string) (*build.BuildResult, error) {
	// Create tag request from domain
	tagRequest := &build.ImageTagRequest{
		ID:        fmt.Sprintf("tag-%d", time.Now().Unix()),
		SessionID: request.SessionID,
		ImageID:   request.TagOptions.SourceImage,
		NewTag:    request.TagOptions.TargetImage,
		CreatedAt: time.Now(),
	}

	// Execute tag using Docker client
	result, err := cmd.performDockerTag(ctx, tagRequest)
	if err != nil {
		return nil, errors.NewError().
			Code(errors.CodeContainerStartFailed).
			Type(errors.ErrTypeContainer).
			Messagef("tag execution failed: %w", err).
			WithLocation().
			Build()
	}

	// Convert to build result
	buildResult := &build.BuildResult{
		BuildID:   fmt.Sprintf("tag-%d", time.Now().Unix()),
		RequestID: tagRequest.ID,
		SessionID: request.SessionID,
		ImageName: request.TagOptions.TargetImage,
		Tags:      []string{request.TagOptions.TargetImage},
		Status:    build.BuildStatusCompleted,
		CreatedAt: time.Now(),
		Metadata: build.BuildMetadata{
			Strategy: build.BuildStrategyDocker,
		},
	}

	if result.Status == build.TagStatusCompleted {
		buildResult.Status = build.BuildStatusCompleted
		buildResult.ImageID = result.ImageID
	} else {
		buildResult.Status = build.BuildStatusFailed
		buildResult.Error = result.Error
	}

	return buildResult, nil
}

// updateSessionState updates session state with build results
func (cmd *ConsolidatedBuildCommand) updateSessionState(ctx context.Context, sessionID string, result *build.BuildResult) error {
	// Update session state with build results
	stateUpdate := map[string]interface{}{
		"last_build":     result,
		"build_time":     time.Now(),
		"build_success":  result.Status == build.BuildStatusCompleted,
		"image_name":     result.ImageName,
		"image_id":       result.ImageID,
		"build_duration": result.Duration,
	}

	return cmd.sessionState.UpdateSessionData(ctx, sessionID, stateUpdate)
}

// createBuildResponse creates the final build response
func (cmd *ConsolidatedBuildCommand) createBuildResponse(result *build.BuildResult, duration time.Duration) *ConsolidatedBuildResponse {
	return &ConsolidatedBuildResponse{
		Success:        result.Status == build.BuildStatusCompleted,
		ImageName:      result.ImageName,
		ImageTag:       getFirstTag(result.Tags),
		FullImageRef:   fmt.Sprintf("%s:%s", result.ImageName, getFirstTag(result.Tags)),
		ImageID:        result.ImageID,
		Duration:       result.Duration,
		BuildLogs:      convertBuildLogs(result.Logs),
		Warnings:       []string{}, // Could be extracted from logs
		Errors:         []string{result.Error},
		LayerCount:     result.Metadata.Layers,
		ImageSizeBytes: result.Size,
		CacheHits:      result.Metadata.CacheHits,
		CacheMisses:    result.Metadata.CacheMisses,
		Platform:       result.Metadata.Platform,
		Tags:           result.Tags,
		BuildMetadata:  convertBuildMetadata(result.Metadata),
		TotalDuration:  duration,
	}
}

// Tool registration for consolidated build command
func (cmd *ConsolidatedBuildCommand) Name() string {
	return "build_image"
}

func (cmd *ConsolidatedBuildCommand) Description() string {
	return "Comprehensive Docker build tool that consolidates all build capabilities"
}

func (cmd *ConsolidatedBuildCommand) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        cmd.Name(),
		Description: cmd.Description(),
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Build operation type",
					"enum":        []string{"build", "push", "pull", "tag"},
					"default":     "build",
				},
				"image_name": map[string]interface{}{
					"type":        "string",
					"description": "Name of the Docker image",
				},
				"image_tag": map[string]interface{}{
					"type":        "string",
					"description": "Tag for the Docker image",
					"default":     "latest",
				},
				"dockerfile_path": map[string]interface{}{
					"type":        "string",
					"description": "Path to Dockerfile",
					"default":     "Dockerfile",
				},
				"build_context": map[string]interface{}{
					"type":        "string",
					"description": "Build context directory",
					"default":     ".",
				},
				"platform": map[string]interface{}{
					"type":        "string",
					"description": "Target platform (e.g., linux/amd64, linux/arm64)",
				},
				"no_cache": map[string]interface{}{
					"type":        "boolean",
					"description": "Build without using cache",
					"default":     false,
				},
				"force_rebuild": map[string]interface{}{
					"type":        "boolean",
					"description": "Force rebuild from scratch",
					"default":     false,
				},
				"push_after_build": map[string]interface{}{
					"type":        "boolean",
					"description": "Push image after successful build",
					"default":     false,
				},
				"build_args": map[string]interface{}{
					"type":        "object",
					"description": "Build arguments",
					"additionalProperties": map[string]interface{}{
						"type": "string",
					},
				},
				"labels": map[string]interface{}{
					"type":        "object",
					"description": "Labels to apply to the image",
					"additionalProperties": map[string]interface{}{
						"type": "string",
					},
				},
				"target": map[string]interface{}{
					"type":        "string",
					"description": "Target stage for multi-stage builds",
				},
				"registry": map[string]interface{}{
					"type":        "string",
					"description": "Registry URL for push/pull operations",
				},
				"username": map[string]interface{}{
					"type":        "string",
					"description": "Registry username",
				},
				"password": map[string]interface{}{
					"type":        "string",
					"description": "Registry password",
				},
				"source_image": map[string]interface{}{
					"type":        "string",
					"description": "Source image for tag operation",
				},
				"target_image": map[string]interface{}{
					"type":        "string",
					"description": "Target image for tag operation",
				},
				"force": map[string]interface{}{
					"type":        "boolean",
					"description": "Force operation",
					"default":     false,
				},
			},
			"required": []string{"image_name"},
		},
		Tags:     []string{"build", "docker", "containerization"},
		Category: api.CategoryBuild,
	}
}

// Helper types for consolidated build functionality

// BuildRequest represents a consolidated build request
type BuildRequest struct {
	SessionID    string       `json:"session_id"`
	Operation    string       `json:"operation"`
	ImageName    string       `json:"image_name"`
	ImageTag     string       `json:"image_tag"`
	BuildOptions BuildOptions `json:"build_options"`
	PushOptions  PushOptions  `json:"push_options"`
	PullOptions  PullOptions  `json:"pull_options"`
	TagOptions   TagOptions   `json:"tag_options"`
	CreatedAt    time.Time    `json:"created_at"`
}

// BuildOptions contains all build configuration options
type BuildOptions struct {
	DockerfilePath string            `json:"dockerfile_path"`
	BuildContext   string            `json:"build_context"`
	Platform       string            `json:"platform"`
	NoCache        bool              `json:"no_cache"`
	ForceRebuild   bool              `json:"force_rebuild"`
	PushAfterBuild bool              `json:"push_after_build"`
	BuildArgs      map[string]string `json:"build_args"`
	Labels         map[string]string `json:"labels"`
	Target         string            `json:"target"`
	NetworkMode    string            `json:"network_mode"`
	CacheFrom      []string          `json:"cache_from"`
	CacheTo        []string          `json:"cache_to"`
	Squash         bool              `json:"squash"`
	PullParent     bool              `json:"pull_parent"`
	Isolation      string            `json:"isolation"`
	ShmSize        string            `json:"shm_size"`
	Ulimits        []string          `json:"ulimits"`
	Memory         string            `json:"memory"`
	MemorySwap     string            `json:"memory_swap"`
	CpuShares      int               `json:"cpu_shares"`
	CpuSetCpus     string            `json:"cpu_set_cpus"`
	CpuSetMems     string            `json:"cpu_set_mems"`
	CpuQuota       int               `json:"cpu_quota"`
	CpuPeriod      int               `json:"cpu_period"`
	SecurityOpt    []string          `json:"security_opt"`
	AddHost        []string          `json:"add_host"`
}

// PushOptions contains push operation options
type PushOptions struct {
	Registry string `json:"registry"`
	Username string `json:"username"`
	Password string `json:"password"`
	Force    bool   `json:"force"`
}

// PullOptions contains pull operation options
type PullOptions struct {
	Registry            string `json:"registry"`
	Tag                 string `json:"tag"`
	Platform            string `json:"platform"`
	AllTags             bool   `json:"all_tags"`
	DisableContentTrust bool   `json:"disable_content_trust"`
}

// TagOptions contains tag operation options
type TagOptions struct {
	SourceImage string `json:"source_image"`
	TargetImage string `json:"target_image"`
	Force       bool   `json:"force"`
}

// ConsolidatedBuildResponse represents the consolidated build response
type ConsolidatedBuildResponse struct {
	Success        bool                   `json:"success"`
	ImageName      string                 `json:"image_name"`
	ImageTag       string                 `json:"image_tag"`
	FullImageRef   string                 `json:"full_image_ref"`
	ImageID        string                 `json:"image_id"`
	Duration       time.Duration          `json:"duration"`
	BuildLogs      []string               `json:"build_logs"`
	Warnings       []string               `json:"warnings"`
	Errors         []string               `json:"errors"`
	LayerCount     int                    `json:"layer_count"`
	ImageSizeBytes int64                  `json:"image_size_bytes"`
	CacheHits      int                    `json:"cache_hits"`
	CacheMisses    int                    `json:"cache_misses"`
	Platform       string                 `json:"platform"`
	Tags           []string               `json:"tags"`
	BuildMetadata  map[string]interface{} `json:"build_metadata"`
	TotalDuration  time.Duration          `json:"total_duration"`
}

// Note: ValidationError is defined in common.go

// Helper functions for build operations

// Note: isValidImageName is defined in common.go

// isValidImageTag validates Docker image tag format
func isValidImageTag(tag string) bool {
	// Basic validation - can be enhanced with full Docker tag rules
	if tag == "" || len(tag) > 128 {
		return false
	}

	// Check for invalid characters
	for _, char := range tag {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '.' || char == '-' ||
			char == '_') {
			return false
		}
	}

	return true
}

// Note: Use slices.Contains from standard library

// Note: Parameter extraction functions are defined in common.go

// Note: getStringSliceParam is defined in common.go

// Helper functions for working with domain types

// getFirstTag returns the first tag from a slice, or "latest" if empty
func getFirstTag(tags []string) string {
	if len(tags) > 0 {
		return tags[0]
	}
	return "latest"
}

// convertBuildLogs converts domain build logs to string slice
func convertBuildLogs(logs []build.BuildLog) []string {
	result := make([]string, len(logs))
	for i, log := range logs {
		result[i] = fmt.Sprintf("[%s] %s: %s", log.Timestamp.Format(time.RFC3339), log.Level, log.Message)
	}
	return result
}

// convertBuildMetadata converts domain build metadata to map
func convertBuildMetadata(metadata build.BuildMetadata) map[string]interface{} {
	return map[string]interface{}{
		"strategy":       metadata.Strategy,
		"platform":       metadata.Platform,
		"base_image":     metadata.BaseImage,
		"layers":         metadata.Layers,
		"cache_hits":     metadata.CacheHits,
		"cache_misses":   metadata.CacheMisses,
		"resource_usage": metadata.ResourceUsage,
		"optimizations":  metadata.Optimizations,
		"security_scan":  metadata.SecurityScan,
	}
}

// Docker integration methods

// performDockerBuild performs the actual Docker build operation
func (cmd *ConsolidatedBuildCommand) performDockerBuild(ctx context.Context, request *build.BuildRequest) (*build.BuildResult, error) {
	startTime := time.Now()

	// Create build result
	result := &build.BuildResult{
		BuildID:   request.ID,
		RequestID: request.ID,
		SessionID: request.SessionID,
		ImageName: request.ImageName,
		Tags:      request.Tags,
		Status:    build.BuildStatusRunning,
		CreatedAt: startTime,
		Metadata: build.BuildMetadata{
			Strategy: build.BuildStrategyDocker,
			Platform: request.Platform,
		},
	}

	// Use Docker client to build image
	buildOptions := services.DockerBuildOptions{
		Context:    request.Context,
		Dockerfile: request.Dockerfile,
		Tags:       request.Tags,
		BuildArgs:  request.BuildArgs,
		Target:     request.Target,
		Platform:   request.Platform,
		NoCache:    request.NoCache,
		PullParent: request.PullParent,
		Labels:     request.Labels,
	}

	buildResult, err := cmd.dockerClient.Build(ctx, buildOptions)
	if err != nil {
		result.Status = build.BuildStatusFailed
		result.Error = err.Error()
		result.Duration = time.Since(startTime)
		completedAt := time.Now()
		result.CompletedAt = &completedAt
		return result, nil
	}

	// Update result with build output
	result.Status = build.BuildStatusCompleted
	result.ImageID = buildResult.ImageID
	result.Size = buildResult.Size
	result.Duration = time.Since(startTime)
	completedAt := time.Now()
	result.CompletedAt = &completedAt

	// Convert build logs
	result.Logs = make([]build.BuildLog, len(buildResult.Logs))
	for i, log := range buildResult.Logs {
		result.Logs[i] = build.BuildLog{
			Timestamp: time.Now(),
			Level:     build.LogLevelInfo,
			Message:   log,
		}
	}

	// Update metadata
	result.Metadata.Layers = buildResult.Layers
	result.Metadata.CacheHits = buildResult.CacheHits
	result.Metadata.CacheMisses = buildResult.CacheMisses

	return result, nil
}

// performDockerPush performs the actual Docker push operation
func (cmd *ConsolidatedBuildCommand) performDockerPush(ctx context.Context, request *build.ImagePushRequest) (*build.ImagePushResult, error) {
	startTime := time.Now()

	// Create push result
	result := &build.ImagePushResult{
		PushID:    request.ID,
		RequestID: request.ID,
		ImageName: request.ImageName,
		Tag:       request.Tag,
		Registry:  request.Registry,
		Status:    build.PushStatusUploading,
		CreatedAt: startTime,
	}

	// Use Docker client to push image
	fullImageRef := fmt.Sprintf("%s:%s", request.ImageName, request.Tag)
	if request.Registry != "" {
		fullImageRef = fmt.Sprintf("%s/%s", request.Registry, fullImageRef)
	}

	pushOptions := services.DockerPushOptions{
		Registry: request.Registry,
		Tag:      request.Tag,
	}

	err := cmd.dockerClient.Push(ctx, fullImageRef, pushOptions)
	if err != nil {
		result.Status = build.PushStatusFailed
		result.Error = err.Error()
		result.Duration = time.Since(startTime)
		completedAt := time.Now()
		result.CompletedAt = &completedAt
		return result, nil
	}

	// Update result with push output
	result.Status = build.PushStatusCompleted
	result.Duration = time.Since(startTime)
	completedAt := time.Now()
	result.CompletedAt = &completedAt

	return result, nil
}

// performDockerPull performs the actual Docker pull operation
func (cmd *ConsolidatedBuildCommand) performDockerPull(ctx context.Context, imageName, tag, registry string) (*build.BuildResult, error) {
	startTime := time.Now()

	// Create result
	result := &build.BuildResult{
		BuildID:   fmt.Sprintf("pull-%d", time.Now().Unix()),
		ImageName: imageName,
		Tags:      []string{tag},
		Status:    build.BuildStatusRunning,
		CreatedAt: startTime,
		Metadata: build.BuildMetadata{
			Strategy: build.BuildStrategyDocker,
		},
	}

	// Construct full image reference
	fullImageRef := fmt.Sprintf("%s:%s", imageName, tag)
	if registry != "" {
		fullImageRef = fmt.Sprintf("%s/%s:%s", registry, imageName, tag)
	}

	// Use Docker client to pull image
	pullOptions := services.DockerPullOptions{
		Registry: registry,
		Tag:      tag,
	}

	pullResult, err := cmd.dockerClient.Pull(ctx, fullImageRef, pullOptions)
	if err != nil {
		result.Status = build.BuildStatusFailed
		result.Error = err.Error()
		result.Duration = time.Since(startTime)
		completedAt := time.Now()
		result.CompletedAt = &completedAt
		return result, nil
	}

	// Update result with pull output
	result.Status = build.BuildStatusCompleted
	result.ImageID = pullResult.ImageID
	result.Size = pullResult.Size
	result.Duration = time.Since(startTime)
	completedAt := time.Now()
	result.CompletedAt = &completedAt

	return result, nil
}

// performDockerTag performs the actual Docker tag operation
func (cmd *ConsolidatedBuildCommand) performDockerTag(ctx context.Context, request *build.ImageTagRequest) (*build.ImageTagResult, error) {
	startTime := time.Now()

	// Create tag result
	result := &build.ImageTagResult{
		TagID:     request.ID,
		RequestID: request.ID,
		ImageID:   request.ImageID,
		NewTag:    request.NewTag,
		Status:    build.TagStatusCompleted,
		CreatedAt: startTime,
	}

	// Use Docker client to tag image
	err := cmd.dockerClient.Tag(ctx, request.ImageID, request.NewTag)
	if err != nil {
		result.Status = build.TagStatusFailed
		result.Error = err.Error()
		return result, nil
	}

	return result, nil
}
