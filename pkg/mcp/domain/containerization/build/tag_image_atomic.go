package build

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/core/docker"
	"github.com/Azure/container-kit/pkg/mcp/application/api"

	"github.com/Azure/container-kit/pkg/mcp/application/core"
	internalcommon "github.com/Azure/container-kit/pkg/mcp/domain/common"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
	"github.com/Azure/container-kit/pkg/mcp/domain/validation"
	"github.com/Azure/container-kit/pkg/mcp/services"

	// "github.com/Azure/container-kit/pkg/mcp/application/internal/runtime" // Temporarily commented to avoid import cycle
	// mcp import removed - using mcptypes
	"github.com/Azure/container-kit/pkg/mcp/domain/types"

	mcptypes "github.com/Azure/container-kit/pkg/mcp/application/core"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"log/slog"
)

// standardTagStages provides common stages for tag operations
func standardTagStages() []mcptypes.ConsolidatedLocalProgressStage {
	return []mcptypes.ConsolidatedLocalProgressStage{
		{Name: "Initialize", Weight: 0.10, Description: "Loading session and validating inputs"},
		{Name: "Check", Weight: 0.30, Description: "Checking source image availability"},
		{Name: "Tag", Weight: 0.40, Description: "Tagging Docker image"},
		{Name: "Verify", Weight: 0.15, Description: "Verifying tag operation"},
		{Name: "Finalize", Weight: 0.05, Description: "Updating session state"},
	}
}

// AtomicTagImageArgs defines arguments for atomic Docker image tagging
type AtomicTagImageArgs struct {
	types.BaseToolArgs
	// Image information
	SourceImage string `json:"source_image" validate:"required,docker_image" description:"The source image to tag (e.g. nginx:latest, myapp:v1.0.0)"`
	TargetImage string `json:"target_image" validate:"required,docker_image" description:"The target image name and tag (e.g. myregistry.com/nginx:production)"`
	// Tag configuration
	Force bool `json:"force,omitempty" description:"Force tag even if target tag already exists"`
}

// AtomicTagImageResult defines the response from atomic Docker image tagging
type AtomicTagImageResult struct {
	types.BaseToolResponse
	core.BaseAIContextResult      // Embedded for AI context methods
	Success                  bool `json:"success"`
	// Session context
	SessionID    string `json:"session_id"`
	WorkspaceDir string `json:"workspace_dir"`
	// Tag configuration
	SourceImage string `json:"source_image"`
	TargetImage string `json:"target_image"`
	// Tag results from core operations
	TagResult *docker.TagResult `json:"tag_result,omitempty"`
	// Timing information
	TagDuration   time.Duration `json:"tag_duration"`
	TotalDuration time.Duration `json:"total_duration"`
	// Rich context for Claude reasoning
	TagContext *TagContext `json:"tag_context"`
	// Rich error information if operation failed
}

// TagContext provides rich context for Claude to reason about
type TagContext struct {
	// Tag analysis
	TagStatus         string `json:"tag_status"`
	SourceImageExists bool   `json:"source_image_exists"`
	TargetImageExists bool   `json:"target_image_exists"`
	TagOverwrite      bool   `json:"tag_overwrite"`
	// Registry information
	SourceRegistry string `json:"source_registry"`
	TargetRegistry string `json:"target_registry"`
	SameRegistry   bool   `json:"same_registry"`
	// Error analysis
	ErrorType     string `json:"error_type,omitempty"`
	ErrorCategory string `json:"error_category,omitempty"`
	IsRetryable   bool   `json:"is_retryable"`
	// Next step suggestions
	NextStepSuggestions []string `json:"next_step_suggestions"`
	TroubleshootingTips []string `json:"troubleshooting_tips,omitempty"`
}

// AtomicTagImageTool implements atomic Docker image tagging using core operations
type AtomicTagImageTool struct {
	pipelineAdapter mcptypes.TypedPipelineOperations
	sessionManager  session.UnifiedSessionManager
	logger          *slog.Logger
	analyzer        *internalcommon.FailureAnalyzer
	fixingMixin     *AtomicToolFixingMixin
}

// NewAtomicTagImageToolWithServices creates a new atomic tag image tool using service interfaces
func NewAtomicTagImageToolWithServices(adapter mcptypes.TypedPipelineOperations, serviceContainer services.ServiceContainer, logger *slog.Logger) *AtomicTagImageTool {
	toolLogger := logger.With("tool", "atomic_tag_image")

	return &AtomicTagImageTool{
		pipelineAdapter: adapter,
		sessionManager:  nil, // TODO: Update to use services directly
		logger:          toolLogger,
		analyzer:        internalcommon.NewFailureAnalyzer(),
	}
}

// SetAnalyzer sets the analyzer for failure analysis
func (t *AtomicTagImageTool) SetAnalyzer(analyzer *internalcommon.FailureAnalyzer) {
	t.analyzer = analyzer
}

// SetFixingMixin sets the fixing mixin for automatic error recovery
func (t *AtomicTagImageTool) SetFixingMixin(mixin *AtomicToolFixingMixin) {
	t.fixingMixin = mixin
}

// ExecuteWithFixes runs the atomic Docker image tag with automatic fixes
func (t *AtomicTagImageTool) ExecuteWithFixes(ctx context.Context, args AtomicTagImageArgs) (*AtomicTagImageResult, error) {
	if t.fixingMixin != nil && !args.DryRun {
		// Create wrapper operation for tag process
		var result *AtomicTagImageResult
		// Progress tracking infrastructure removed
		operation := NewDockerOperation(DockerOperationConfig{
			Type:          OperationTag,
			Name:          fmt.Sprintf("%s->%s", args.SourceImage, args.TargetImage),
			RetryAttempts: 3,
			Timeout:       2 * time.Minute, // Tag operations are typically fast
			ExecuteFunc: func(ctx context.Context) error {
				var err error
				result, err = t.executeWithoutProgress(ctx, args, nil, time.Now())
				if err != nil {
					return err
				}
				if !result.Success {
					return errors.NewError().Messagef("operation failed").Build()
				}
				return nil
			},
		}, nil) // progress reporter removed
		// Use fixing mixin to handle retries
		startTime := time.Now()
		err := t.fixingMixin.ExecuteWithRetry(ctx, args.SessionID, "/workspace", operation)
		if err != nil {
			if result == nil {
				result = &AtomicTagImageResult{
					BaseToolResponse:    types.BaseToolResponse{Success: false, Timestamp: time.Now()},
					BaseAIContextResult: core.NewBaseAIContextResult("tag", false, 0),
					SourceImage:         args.SourceImage,
					TargetImage:         args.TargetImage,
				}
			}
			result.Success = false
			result.TotalDuration = time.Since(startTime)
			return result, err
		}
		return result, nil
	}
	// Fallback to direct execution without progress tracking
	return t.ExecuteTyped(ctx, args)
}

// validateTagPrerequisites checks if all prerequisites for tagging are met
func (t *AtomicTagImageTool) validateTagPrerequisites(result *AtomicTagImageResult, args AtomicTagImageArgs) error {
	// Basic input validation using RichError
	if args.SourceImage == "" {
		return errors.NewError().Messagef("source image is required").WithLocation().Build()
	}
	if args.TargetImage == "" {
		return errors.NewError().Messagef("target image is required").WithLocation(

		// Validate image name formats using RichError
		).Build()
	}

	if !t.isValidImageReference(args.SourceImage) {
		return errors.NewError().Messagef("invalid source image reference").WithLocation().Build()
	}
	if !t.isValidImageReference(args.TargetImage) {
		return errors.NewError().Messagef("invalid target image reference").WithLocation().Build(

		// isValidImageReference checks if an image reference is valid
		)
	}
	return nil
}

func (t *AtomicTagImageTool) isValidImageReference(imageRef string) bool {
	// Basic validation - should contain at least name
	if imageRef == "" {
		return false
	}
	// Should not contain spaces
	if strings.Contains(imageRef, " ") {
		return false
	}
	// Should not start or end with special characters
	if strings.HasPrefix(imageRef, "-") || strings.HasSuffix(imageRef, "-") {
		return false
	}
	return true
}

// extractRegistryURL extracts the registry URL from an image reference
func (t *AtomicTagImageTool) extractRegistryURL(imageRef string) string {
	// Split by slash to get registry part
	parts := strings.Split(imageRef, "/")
	if len(parts) > 1 && strings.Contains(parts[0], ".") {
		return parts[0] // First part contains registry
	}
	return "docker.io" // Default registry
}

// Validate validates the tool arguments
func (t *AtomicTagImageTool) Validate(ctx context.Context, args interface{}) error {
	// Validate using tag-based validation
	return validation.ValidateTaggedStruct(args)
}

// Execute implements SimpleTool interface with generic signature
func (t *AtomicTagImageTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	tagArgs, ok := args.(AtomicTagImageArgs)
	if !ok {
		return nil, errors.NewError().Messagef("invalid argument type for atomic_tag_image").WithLocation(

		// Call the typed Execute method
		).Build()
	}

	return t.ExecuteTyped(ctx, tagArgs)
}

// Tool interface implementation (unified interface)
// GetMetadata returns comprehensive tool metadata
func (t *AtomicTagImageTool) GetMetadata() api.ToolMetadata {
	return api.ToolMetadata{
		Name:         "atomic_tag_image",
		Description:  "Tags Docker images with new names for versioning, environment promotion, or registry organization",
		Version:      "1.0.0",
		Category:     api.ToolCategory("docker"),
		Tags:         []string{"docker", "tag", "versioning"},
		Status:       api.ToolStatus("active"),
		Dependencies: []string{"docker"},
		Capabilities: []string{
			"supports_dry_run",
		},
		Requirements: []string{"docker_daemon"},
		RegisteredAt: time.Now(),
		LastModified: time.Now(),
	}
}

// ExecuteTyped provides the original typed execute method
func (t *AtomicTagImageTool) ExecuteTyped(ctx context.Context, args AtomicTagImageArgs) (*AtomicTagImageResult, error) {
	return t.ExecuteTag(ctx, args)
}

// ExecuteTag implements the core tag functionality
func (t *AtomicTagImageTool) ExecuteTag(ctx context.Context, args AtomicTagImageArgs) (*AtomicTagImageResult, error) {
	startTime := time.Now()

	// Create result object
	result := &AtomicTagImageResult{
		BaseToolResponse:    types.BaseToolResponse{Success: false, Timestamp: time.Now()},
		BaseAIContextResult: core.NewBaseAIContextResult("tag", false, 0),
		SourceImage:         args.SourceImage,
		TargetImage:         args.TargetImage,
		TagContext:          &TagContext{},
	}

	// Get session
	sessionState, err := t.sessionManager.GetSession(ctx, args.SessionID)
	if err != nil {
		t.logger.Error("Failed to get session", "error", err, "session_id", args.SessionID)
		result.Success = false
		result.TotalDuration = time.Since(startTime)
		return result, errors.NewError().Messagef("session not found: %s", args.SessionID).Build()
	}
	session := sessionState.ToCoreSessionState()
	result.SessionID = session.SessionID
	result.WorkspaceDir = t.pipelineAdapter.GetSessionWorkspace(session.SessionID)

	// Handle dry-run
	if args.DryRun {
		result.Success = true
		result.TagContext.TagStatus = "dry-run"
		result.TagContext.NextStepSuggestions = []string{
			"This is a dry-run - no actual tag was performed",
			"Remove dry_run flag to perform actual tag",
		}
		result.TotalDuration = time.Since(startTime)
		result.BaseAIContextResult = core.NewBaseAIContextResult("tag", true, result.TotalDuration)
		return result, nil
	}

	// Validate prerequisites
	if err := t.validateTagPrerequisites(result, args); err != nil {
		result.Success = false
		result.TotalDuration = time.Since(startTime)
		return result, err
	}

	// Perform the tag operation
	tagStartTime := time.Now()
	// Convert to typed parameters for TagImageTyped
	tagParams := core.TagImageParams{
		SourceImage: args.SourceImage,
		TargetImage: args.TargetImage,
	}
	_, err = t.pipelineAdapter.TagImageTyped(ctx, session.SessionID, tagParams)
	result.TagDuration = time.Since(tagStartTime)
	result.TotalDuration = time.Since(startTime)

	if err != nil {
		result.Success = false
		t.logger.Error("Failed to tag image",
			"error", err,
			"source_image", args.SourceImage,
			"target_image", args.TargetImage)
		result.BaseAIContextResult = core.NewBaseAIContextResult("tag", false, result.TotalDuration)
		return result, err
	}

	// Success
	result.Success = true
	result.TagResult = &docker.TagResult{
		Success:     true,
		SourceImage: args.SourceImage,
		TargetImage: args.TargetImage,
	}
	result.TagContext.TagStatus = "successful"
	result.TagContext.NextStepSuggestions = []string{
		fmt.Sprintf("Image %s successfully tagged as %s", args.SourceImage, args.TargetImage),
		"You can now push the tagged image to a registry or use it in deployments",
	}

	result.BaseAIContextResult = core.NewBaseAIContextResult("tag", true, result.TotalDuration)

	t.logger.Info("Tag operation completed successfully",
		"source_image", args.SourceImage,
		"target_image", args.TargetImage,
		"tag_duration", result.TagDuration)

	return result, nil
}

// executeWithoutProgress handles tag execution without progress tracking (fallback)
func (t *AtomicTagImageTool) executeWithoutProgress(ctx context.Context, args AtomicTagImageArgs, result *AtomicTagImageResult, startTime time.Time) (*AtomicTagImageResult, error) {
	// Create result if not provided
	if result == nil {
		result = &AtomicTagImageResult{
			BaseToolResponse:    types.BaseToolResponse{Success: false, Timestamp: time.Now()},
			BaseAIContextResult: core.NewBaseAIContextResult("tag", false, 0),
			SourceImage:         args.SourceImage,
			TargetImage:         args.TargetImage,
			TagContext:          &TagContext{},
		}
	}

	// Get session
	sessionState, err := t.sessionManager.GetSession(ctx, args.SessionID)
	if err != nil {
		t.logger.Error("Failed to get session", "error", err, "session_id", args.SessionID)
		result.Success = false
		result.TotalDuration = time.Since(startTime)
		return result, errors.NewError().Messagef("session not found: %s", args.SessionID).WithLocation().Build()
	}
	session := sessionState.ToCoreSessionState()
	result.SessionID = session.SessionID
	result.WorkspaceDir = t.pipelineAdapter.GetSessionWorkspace(session.SessionID)

	t.logger.Info("Starting atomic Docker tag",
		"session_id", session.SessionID,
		"source_image", args.SourceImage,
		"target_image", args.TargetImage)

	// Handle dry-run
	if args.DryRun {
		result.Success = true
		result.BaseAIContextResult = core.NewBaseAIContextResult("tag", true, result.TotalDuration)
		result.TagContext.TagStatus = "dry-run"
		result.TagContext.SourceRegistry = t.extractRegistryURL(args.SourceImage)
		result.TagContext.TargetRegistry = t.extractRegistryURL(args.TargetImage)
		result.TagContext.SameRegistry = result.TagContext.SourceRegistry == result.TagContext.TargetRegistry
		result.TagContext.NextStepSuggestions = []string{
			"This is a dry-run - no actual tag was performed",
			"Remove dry_run flag to perform actual tag",
		}
		result.TotalDuration = time.Since(startTime)
		return result, nil
	}

	// Validate prerequisites
	if err := t.validateTagPrerequisites(result, args); err != nil {
		t.logger.Error("Tag prerequisites validation failed",
			"error", err,
			"session_id", session.SessionID,
			"source_image", args.SourceImage,
			"target_image", args.TargetImage)
		result.Success = false
		result.TotalDuration = time.Since(startTime)
		return result, err
	}

	// Perform the tag without progress reporting
	err = t.performTag(ctx, session, args, result, nil)
	result.TotalDuration = time.Since(startTime)

	// Update AI context with final result
	result.BaseAIContextResult = core.NewBaseAIContextResult("tag", result.Success, result.TotalDuration)

	if err != nil {
		result.Success = false
		return result, nil
	}

	return result, nil
}

// performTag contains the actual tag logic that can be used with or without progress reporting
func (t *AtomicTagImageTool) performTag(ctx context.Context, session *core.SessionState, args AtomicTagImageArgs, result *AtomicTagImageResult, reporter interface{}) error {
	// Extract registry information
	result.TagContext.SourceRegistry = t.extractRegistryURL(args.SourceImage)
	result.TagContext.TargetRegistry = t.extractRegistryURL(args.TargetImage)
	result.TagContext.SameRegistry = result.TagContext.SourceRegistry == result.TagContext.TargetRegistry

	// Tag Docker image using pipeline adapter
	tagStartTime := time.Now()

	// Convert to typed parameters for TagImageTyped
	tagParams := core.TagImageParams{
		SourceImage: args.SourceImage,
		TargetImage: args.TargetImage,
	}

	// Use the pipeline adapter to tag the image
	_, err := t.pipelineAdapter.TagImageTyped(ctx, session.SessionID, tagParams)
	result.TagDuration = time.Since(tagStartTime)

	if err != nil {
		result.Success = false
		result.TagContext.TagStatus = "failed"
		result.TagContext.ErrorType = "tag_error"
		result.TagContext.IsRetryable = true
		result.TagContext.NextStepSuggestions = []string{
			"Check that source image exists locally",
			"Verify target image name format",
			"Check if target tag already exists (use force flag if needed)",
		}
		t.logger.Error("Failed to tag image",
			"error", err,
			"source_image", args.SourceImage,
			"target_image", args.TargetImage)
		return errors.NewError().Message("failed to tag image").Cause(err).WithLocation(

		// Success - create tag result
		).Build()
	}

	result.Success = true
	result.TagResult = &docker.TagResult{
		Success:     true,
		SourceImage: args.SourceImage,
		TargetImage: args.TargetImage,
	}

	result.TagContext.TagStatus = "successful"
	result.TagContext.SourceImageExists = true
	result.TagContext.TargetImageExists = true
	result.TagContext.TagOverwrite = args.Force
	result.TagContext.NextStepSuggestions = []string{
		fmt.Sprintf("Image %s successfully tagged as %s", args.SourceImage, args.TargetImage),
		"Tagged image is now available for use or push to registry",
	}

	// Add registry-specific suggestions
	if !result.TagContext.SameRegistry {
		result.TagContext.NextStepSuggestions = append(result.TagContext.NextStepSuggestions,
			fmt.Sprintf("Consider pushing %s to %s registry", args.TargetImage, result.TagContext.TargetRegistry))
	}

	t.logger.Info("Tag operation completed successfully",
		"source_image", args.SourceImage,
		"target_image", args.TargetImage,
		"tag_duration", result.TagDuration)

	return nil
}

// AI Context is provided by embedded core.BaseAIContextResult
