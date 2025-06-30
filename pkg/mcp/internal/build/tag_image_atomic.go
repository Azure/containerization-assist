package build

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/core/docker"
	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/internal/observability"

	// "github.com/Azure/container-kit/pkg/mcp/internal/runtime" // Temporarily commented to avoid import cycle

	// mcp import removed - using mcptypes
	"github.com/Azure/container-kit/pkg/mcp/internal/types"

	mcptypes "github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/rs/zerolog"
)

// standardTagStages provides common stages for tag operations
func standardTagStages() []mcptypes.LocalProgressStage {
	return []mcptypes.LocalProgressStage{
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
	SourceImage string `json:"source_image" jsonschema:"required,pattern=^[a-zA-Z0-9][a-zA-Z0-9._/-]*(:([a-zA-Z0-9][a-zA-Z0-9._-]*|latest))?$" description:"The source image to tag (e.g. nginx:latest, myapp:v1.0.0)"`
	TargetImage string `json:"target_image" jsonschema:"required,pattern=^[a-zA-Z0-9][a-zA-Z0-9._/-]*:[a-zA-Z0-9][a-zA-Z0-9._-]*$" description:"The target image name and tag (e.g. myregistry.com/nginx:production)"`
	// Tag configuration
	Force bool `json:"force,omitempty" description:"Force tag even if target tag already exists"`
}

// AtomicTagImageResult defines the response from atomic Docker image tagging
type AtomicTagImageResult struct {
	types.BaseToolResponse
	mcptypes.BaseAIContextResult      // Embedded for AI context methods
	Success                      bool `json:"success"`
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
	pipelineAdapter mcptypes.PipelineOperations
	sessionManager  core.ToolSessionManager
	logger          zerolog.Logger
	analyzer        ToolAnalyzer
	fixingMixin     *AtomicToolFixingMixin
}

// NewAtomicTagImageTool creates a new atomic tag image tool
func NewAtomicTagImageTool(adapter mcptypes.PipelineOperations, sessionManager core.ToolSessionManager, logger zerolog.Logger) *AtomicTagImageTool {
	toolLogger := logger.With().Str("tool", "atomic_tag_image").Logger()
	return &AtomicTagImageTool{
		pipelineAdapter: adapter,
		sessionManager:  sessionManager,
		logger:          toolLogger,
	}
}

// SetAnalyzer sets the analyzer for failure analysis
func (t *AtomicTagImageTool) SetAnalyzer(analyzer ToolAnalyzer) {
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
		progress := observability.NewUnifiedProgressReporter(nil) // No server context in ExecuteWithFixes
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
					return fmt.Errorf("operation failed")
				}
				return nil
			},
		}, progress)
		// Use fixing mixin to handle retries
		startTime := time.Now()
		err := t.fixingMixin.ExecuteWithRetry(ctx, args.SessionID, "/workspace", operation)
		if err != nil {
			if result == nil {
				result = &AtomicTagImageResult{
					BaseToolResponse:    types.NewBaseResponse("atomic_tag_image", args.SessionID, args.DryRun),
					BaseAIContextResult: mcptypes.NewBaseAIContextResult("tag", false, 0),
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
		return fmt.Errorf("source image is required")
	}
	if args.TargetImage == "" {
		return fmt.Errorf("target image is required")
	}
	// Validate image name formats using RichError
	if !t.isValidImageReference(args.SourceImage) {
		return fmt.Errorf("invalid source image reference")
	}
	if !t.isValidImageReference(args.TargetImage) {
		return fmt.Errorf("invalid target image reference")
	}
	return nil
}

// isValidImageReference checks if an image reference is valid
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
	tagArgs, ok := args.(AtomicTagImageArgs)
	if !ok {
		return fmt.Errorf("invalid argument type for atomic_tag_image")
	}
	if tagArgs.SourceImage == "" {
		return fmt.Errorf("validation error")
	}
	if tagArgs.TargetImage == "" {
		return fmt.Errorf("validation error")
	}
	if tagArgs.SessionID == "" {
		return fmt.Errorf("validation error")
	}
	// Validate image reference formats
	if !t.isValidImageReference(tagArgs.SourceImage) {
		return fmt.Errorf("validation error")
	}
	if !t.isValidImageReference(tagArgs.TargetImage) {
		return fmt.Errorf("validation error")
	}
	return nil
}

// executeTagWithCallback executes the tag operation with progress callback
// ProgressCallback type temporarily defined here to avoid import cycle
type TagProgressCallback func(progress float64, message string)

func (t *AtomicTagImageTool) executeTagWithCallback(ctx context.Context, args AtomicTagImageArgs, result *AtomicTagImageResult, progress TagProgressCallback) error {
	// Get session
	sessionInterface, err := t.sessionManager.GetSession(args.SessionID)
	if err != nil {
		return fmt.Errorf("session not found: %s", args.SessionID)
	}
	session := sessionInterface.(*core.SessionState)

	// Report initial progress
	progress(0.1, "Starting image tag operation")

	// Validate inputs
	progress(0.2, "Validating tag parameters")
	if err := t.validateTagPrerequisites(result, args); err != nil {
		return err
	}

	// Perform the actual tag
	progress(0.3, fmt.Sprintf("Tagging %s as %s", args.SourceImage, args.TargetImage))

	// Create tag arguments
	tagArgs := map[string]interface{}{
		"sourceImage": args.SourceImage,
		"targetImage": args.TargetImage,
		"force":       args.Force,
	}

	// Report progress
	progress(0.5, "Executing tag operation")

	// Use the pipeline adapter to tag the image
	tagResult, err := t.pipelineAdapter.TagImage(ctx, session.SessionID, tagArgs)
	if err != nil {
		return fmt.Errorf("failed to tag image: %w", err)
	}

	// Extract results
	if tagResultTyped, ok := tagResult.(*docker.TagResult); ok {
		result.TagResult = tagResultTyped
		result.Success = true
		progress(0.9, "Tag operation completed")
	}

	// Complete
	progress(1.0, "Image tagged successfully")

	return nil
}

// Execute implements SimpleTool interface with generic signature
func (t *AtomicTagImageTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	tagArgs, ok := args.(AtomicTagImageArgs)
	if !ok {
		return nil, fmt.Errorf("invalid argument type for atomic_tag_image")
	}
	// Call the typed Execute method
	return t.ExecuteTyped(ctx, tagArgs)
}

// Tool interface implementation (unified interface)
// GetMetadata returns comprehensive tool metadata
func (t *AtomicTagImageTool) GetMetadata() core.ToolMetadata {
	return core.ToolMetadata{
		Name:         "atomic_tag_image",
		Description:  "Tags Docker images with new names for versioning, environment promotion, or registry organization",
		Version:      "1.0.0",
		Category:     "docker",
		Dependencies: []string{"docker"},
		Capabilities: []string{
			"supports_dry_run",
		},
		Requirements: []string{"docker_daemon"},
		Parameters: map[string]string{
			"source_image": "required - Source image to tag",
			"target_image": "required - Target image name and tag",
			"force":        "optional - Force tag even if target exists",
		},
		Examples: []mcptypes.ToolExample{
			{
				Name:        "basic_tag",
				Description: "Tag a Docker image with new name",
				Input: map[string]interface{}{
					"session_id":   "session-123",
					"source_image": "myapp:latest",
					"target_image": "myregistry.azurecr.io/myapp:v1.0.0",
				},
				Output: map[string]interface{}{
					"success":      true,
					"source_image": "myapp:latest",
					"target_image": "myregistry.azurecr.io/myapp:v1.0.0",
				},
			},
		},
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
		BaseToolResponse:    types.NewBaseResponse("atomic_tag_image", args.SessionID, args.DryRun),
		BaseAIContextResult: mcptypes.NewBaseAIContextResult("tag", false, 0),
		SourceImage:         args.SourceImage,
		TargetImage:         args.TargetImage,
		TagContext:          &TagContext{},
	}

	// Get session
	sessionInterface, err := t.sessionManager.GetSession(args.SessionID)
	if err != nil {
		t.logger.Error().Err(err).Str("session_id", args.SessionID).Msg("Failed to get session")
		result.Success = false
		result.TotalDuration = time.Since(startTime)
		return result, fmt.Errorf("session not found: %s", args.SessionID)
	}

	session := sessionInterface.(*core.SessionState)
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
		result.BaseAIContextResult = mcptypes.NewBaseAIContextResult("tag", true, result.TotalDuration)
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
	tagArgs := map[string]interface{}{
		"sourceImage": args.SourceImage,
		"targetImage": args.TargetImage,
	}
	_, err = t.pipelineAdapter.TagImage(ctx, session.SessionID, tagArgs)
	result.TagDuration = time.Since(tagStartTime)
	result.TotalDuration = time.Since(startTime)

	if err != nil {
		result.Success = false
		t.logger.Error().Err(err).
			Str("source_image", args.SourceImage).
			Str("target_image", args.TargetImage).
			Msg("Failed to tag image")
		result.BaseAIContextResult = mcptypes.NewBaseAIContextResult("tag", false, result.TotalDuration)
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

	result.BaseAIContextResult = mcptypes.NewBaseAIContextResult("tag", true, result.TotalDuration)

	t.logger.Info().
		Str("source_image", args.SourceImage).
		Str("target_image", args.TargetImage).
		Dur("tag_duration", result.TagDuration).
		Msg("Tag operation completed successfully")

	return result, nil
}

// executeWithoutProgress handles tag execution without progress tracking (fallback)
func (t *AtomicTagImageTool) executeWithoutProgress(ctx context.Context, args AtomicTagImageArgs, result *AtomicTagImageResult, startTime time.Time) (*AtomicTagImageResult, error) {
	// Create result if not provided
	if result == nil {
		result = &AtomicTagImageResult{
			BaseToolResponse:    types.NewBaseResponse("atomic_tag_image", args.SessionID, args.DryRun),
			BaseAIContextResult: mcptypes.NewBaseAIContextResult("tag", false, 0),
			SourceImage:         args.SourceImage,
			TargetImage:         args.TargetImage,
			TagContext:          &TagContext{},
		}
	}

	// Get session
	sessionInterface, err := t.sessionManager.GetSession(args.SessionID)
	if err != nil {
		t.logger.Error().Err(err).Str("session_id", args.SessionID).Msg("Failed to get session")
		result.Success = false
		result.TotalDuration = time.Since(startTime)
		return result, fmt.Errorf("session not found: %s", args.SessionID)
	}

	session := sessionInterface.(*core.SessionState)
	result.SessionID = session.SessionID
	result.WorkspaceDir = t.pipelineAdapter.GetSessionWorkspace(session.SessionID)

	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("source_image", args.SourceImage).
		Str("target_image", args.TargetImage).
		Msg("Starting atomic Docker tag")

	// Handle dry-run
	if args.DryRun {
		result.Success = true
		result.BaseAIContextResult = mcptypes.NewBaseAIContextResult("tag", true, result.TotalDuration)
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
		t.logger.Error().Err(err).
			Str("session_id", session.SessionID).
			Str("source_image", args.SourceImage).
			Str("target_image", args.TargetImage).
			Msg("Tag prerequisites validation failed")
		result.Success = false
		result.TotalDuration = time.Since(startTime)
		return result, err
	}

	// Perform the tag without progress reporting
	err = t.performTag(ctx, session, args, result, nil)
	result.TotalDuration = time.Since(startTime)

	// Update AI context with final result
	result.BaseAIContextResult = mcptypes.NewBaseAIContextResult("tag", result.Success, result.TotalDuration)

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

	// Create tag arguments
	tagArgs := map[string]interface{}{
		"sourceImage": args.SourceImage,
		"targetImage": args.TargetImage,
		"force":       args.Force,
	}

	// Use the pipeline adapter to tag the image
	_, err := t.pipelineAdapter.TagImage(ctx, session.SessionID, tagArgs)
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
		t.logger.Error().Err(err).
			Str("source_image", args.SourceImage).
			Str("target_image", args.TargetImage).
			Msg("Failed to tag image")
		return fmt.Errorf("failed to tag image: %w", err)
	}

	// Success - create tag result
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

	t.logger.Info().
		Str("source_image", args.SourceImage).
		Str("target_image", args.TargetImage).
		Dur("tag_duration", result.TagDuration).
		Msg("Tag operation completed successfully")

	return nil
}

// AI Context is provided by embedded internal.BaseAIContextResult
