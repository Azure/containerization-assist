package build

import (
	"context"
	"fmt"
	"github.com/Azure/container-copilot/pkg/mcp/internal"
	"strings"
	"time"

	"github.com/Azure/container-copilot/pkg/core/docker"
	"github.com/Azure/container-copilot/pkg/mcp/internal/api/contract"
	"github.com/Azure/container-copilot/pkg/mcp/internal/constants"
	sessiontypes "github.com/Azure/container-copilot/pkg/mcp/internal/session"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
	"github.com/localrivet/gomcp/server"
	"github.com/rs/zerolog"
)

// standardTagStages provides common stages for tag operations
func standardTagStages() []mcptypes.ProgressStage {
	return []mcptypes.ProgressStage{
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
	internal.BaseAIContextResult      // Embedded for AI context methods
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
	sessionManager  mcptypes.ToolSessionManager
	logger          zerolog.Logger
}

// NewAtomicTagImageTool creates a new atomic tag image tool
func NewAtomicTagImageTool(adapter mcptypes.PipelineOperations, sessionManager mcptypes.ToolSessionManager, logger zerolog.Logger) *AtomicTagImageTool {
	toolLogger := logger.With().Str("tool", "atomic_tag_image").Logger()
	return &AtomicTagImageTool{
		pipelineAdapter: adapter,
		sessionManager:  sessionManager,
		logger:          toolLogger,
	}
}

// ExecuteTag runs the atomic Docker image tag operation
func (t *AtomicTagImageTool) ExecuteTag(ctx context.Context, args AtomicTagImageArgs) (*AtomicTagImageResult, error) {
	startTime := time.Now()

	// Create result object early for error handling
	result := &AtomicTagImageResult{
		BaseToolResponse:    types.NewBaseResponse("atomic_tag_image", args.SessionID, args.DryRun),
		BaseAIContextResult: internal.NewBaseAIContextResult("tag", false, 0), // Will be updated later
		SessionID:           args.SessionID,
		SourceImage:         args.SourceImage,
		TargetImage:         args.TargetImage,
		TagContext:          &TagContext{},
	}

	// Direct execution without progress tracking
	err := t.executeWithoutProgress(ctx, args, result, startTime)
	result.TotalDuration = time.Since(startTime)

	// Update AI context with final result
	result.BaseAIContextResult = internal.NewBaseAIContextResult("tag", result.Success, result.TotalDuration)

	if err != nil {
		result.Success = false
	}

	return result, nil
}

// ExecuteWithContext runs the atomic Docker image tag with GoMCP progress tracking
func (t *AtomicTagImageTool) ExecuteWithContext(serverCtx *server.Context, args AtomicTagImageArgs) (*AtomicTagImageResult, error) {
	startTime := time.Now()

	// Create result object early for error handling
	result := &AtomicTagImageResult{
		BaseToolResponse:    types.NewBaseResponse("atomic_tag_image", args.SessionID, args.DryRun),
		BaseAIContextResult: internal.NewBaseAIContextResult("tag", false, 0), // Will be updated later
		SessionID:           args.SessionID,
		SourceImage:         args.SourceImage,
		TargetImage:         args.TargetImage,
		TagContext:          &TagContext{},
	}

	// Create progress adapter for GoMCP using standard tag stages
	_ = internal.NewGoMCPProgressAdapter(serverCtx, standardTagStages())

	// Execute with progress tracking
	ctx := context.Background()
	err := t.executeWithProgress(ctx, args, result, startTime, nil)

	// Always set total duration
	result.TotalDuration = time.Since(startTime)

	// Update AI context with final result
	result.BaseAIContextResult = internal.NewBaseAIContextResult("tag", result.Success, result.TotalDuration)

	// Complete progress tracking
	if err != nil {
		t.logger.Info().Msg("Tag failed")
		result.Success = false
		return result, nil // Return result with error info, not the error itself
	} else {
		t.logger.Info().Msg("Tag completed successfully")
	}

	return result, nil
}

// executeWithProgress runs the tag operation with progress reporting
func (t *AtomicTagImageTool) executeWithProgress(ctx context.Context, args AtomicTagImageArgs, result *AtomicTagImageResult, startTime time.Time, reporter mcptypes.ProgressReporter) error {
	return t.performTag(ctx, nil, args, result, reporter)
}

// executeWithoutProgress runs the tag operation without progress reporting
func (t *AtomicTagImageTool) executeWithoutProgress(ctx context.Context, args AtomicTagImageArgs, result *AtomicTagImageResult, startTime time.Time) error {
	// Stage 1: Initialize - Loading session and validating inputs
	t.logger.Info().Msg("Starting tag operation without progress tracking")

	// Get session
	sessionInterface, err := t.sessionManager.GetSession(args.SessionID)
	if err != nil {
		t.logger.Error().Err(err).Str("session_id", args.SessionID).Msg("Failed to get session")
		result.Success = false
		result.TotalDuration = time.Since(startTime)
		return types.NewSessionError(args.SessionID, "tag_image").
			WithStage("session_load").
			WithTool("tag_image_atomic").
			WithField("source_image", args.SourceImage).
			WithField("target_image", args.TargetImage).
			WithRootCause("Session ID does not exist or has expired").
			WithCommand(2, "Create new session", "Create a new session if the current one is invalid", "analyze_repository --repo_path /path/to/repo", "New session created").
			Build()
	}
	session := sessionInterface.(*sessiontypes.SessionState)

	// Set session details
	result.SessionID = session.SessionID // Use compatibility method
	result.WorkspaceDir = t.pipelineAdapter.GetSessionWorkspace(session.SessionID)

	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("source_image", args.SourceImage).
		Str("target_image", args.TargetImage).
		Msg("Starting atomic Docker tag")

	// Handle dry-run
	if args.DryRun {
		result.Success = true
		result.TagContext.TagStatus = "dry_run_successful"
		result.TagContext.NextStepSuggestions = []string{
			"This is a dry-run - no actual tag was performed",
			"Remove dry_run flag to perform actual tag operation",
			fmt.Sprintf("Would tag %s as %s", args.SourceImage, args.TargetImage),
		}
		result.TotalDuration = time.Since(startTime)
		return nil
	}

	// Validate prerequisites
	if err := t.validateTagPrerequisites(result, args); err != nil {
		t.logger.Error().Err(err).
			Str("source_image", args.SourceImage).
			Str("target_image", args.TargetImage).
			Str("session_id", session.SessionID).
			Msg("Tag prerequisites validation failed")
		result.Success = false
		result.TotalDuration = time.Since(startTime)
		return err // Already a RichError from validateTagPrerequisites
	}

	// Perform the tag without progress reporting
	err = t.performTag(ctx, session, args, result, nil)
	result.TotalDuration = time.Since(startTime)

	if err != nil {
		result.Success = false
		return types.NewBuildError("Docker tag operation failed", args.SessionID, args.TargetImage).
			WithStage("tag_execution").
			WithTool("tag_image_atomic").
			WithField("source_image", args.SourceImage).
			WithField("target_image", args.TargetImage).
			WithRootCause("Docker daemon or image repository error").
			WithImmediateStep(1, "Check Docker daemon", "Verify Docker daemon is running and accessible").
			WithImmediateStep(2, "Verify source image", "Ensure the source image exists and is pullable").
			WithCommand(3, "Test Docker connection", "Check basic Docker functionality", "docker version", "Docker version information displayed").
			Build()
	}

	result.Success = true
	return nil
}

// performTag executes the actual Docker tag operation
func (t *AtomicTagImageTool) performTag(ctx context.Context, session *sessiontypes.SessionState, args AtomicTagImageArgs, result *AtomicTagImageResult, reporter mcptypes.ProgressReporter) error {
	// Get session if not provided
	if session == nil {
		var err error
		sessionInterface, err := t.sessionManager.GetSession(args.SessionID)
		if err == nil {
			session = sessionInterface.(*sessiontypes.SessionState)
		}
		if err != nil {
			t.logger.Error().Err(err).Str("session_id", args.SessionID).Msg("Failed to get session")
			return types.NewSessionError(args.SessionID, "tag_image").
				WithStage("session_load").
				WithTool("tag_image_atomic").
				WithRootCause("Session ID does not exist or has expired").
				WithCommand(2, "Create new session", "Create a new session if the current one is invalid", "analyze_repository --repo_path /path/to/repo", "New session created").
				Build()
		}
	}

	// Stage 1: Initialize
	// Progress reporting removed

	// Set session details
	result.SessionID = session.SessionID // Use compatibility method
	result.WorkspaceDir = t.pipelineAdapter.GetSessionWorkspace(session.SessionID)

	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("source_image", args.SourceImage).
		Str("target_image", args.TargetImage).
		Msg("Starting atomic Docker tag")

	// Stage 2: Check source image
	// Progress reporting removed

	// Extract registry information for context
	result.TagContext.SourceRegistry = t.extractRegistryURL(args.SourceImage)
	result.TagContext.TargetRegistry = t.extractRegistryURL(args.TargetImage)
	result.TagContext.SameRegistry = result.TagContext.SourceRegistry == result.TagContext.TargetRegistry

	// Stage 3: Tag Docker image using pipeline adapter
	// Progress reporting removed

	tagStartTime := time.Now()

	err := t.pipelineAdapter.TagDockerImage(session.SessionID, args.SourceImage, args.TargetImage)
	result.TagDuration = time.Since(tagStartTime)

	if err != nil {
		result.Success = false
		t.logger.Error().Err(err).
			Str("source_image", args.SourceImage).
			Str("target_image", args.TargetImage).
			Msg("Failed to tag image")
		return err
	}

	// Update result with tag operation details
	result.Success = true
	result.TagResult = &docker.TagResult{
		Success:     true,
		SourceImage: args.SourceImage,
		TargetImage: args.TargetImage,
	}

	result.TagContext.TagStatus = "successful"
	result.TagContext.NextStepSuggestions = []string{
		fmt.Sprintf("Image %s successfully tagged as %s", args.SourceImage, args.TargetImage),
		"You can now use the new tag for deployment or pushing",
		fmt.Sprintf("New tag available: %s", args.TargetImage),
	}

	// Stage 4: Verify operation
	// Progress reporting removed

	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("source_image", result.SourceImage).
		Str("target_image", result.TargetImage).
		Dur("tag_duration", result.TagDuration).
		Bool("success", result.Success).
		Msg("Completed atomic Docker tag")

	// Stage 5: Finalize
	// Progress reporting removed

	// Update session state
	session.UpdateLastAccessed()

	// Save session state
	return t.sessionManager.UpdateSession(session.SessionID, func(s interface{}) {
		if sess, ok := s.(*sessiontypes.SessionState); ok {
			*sess = *session
		}
	})
}

// validateTagPrerequisites checks if all prerequisites for tagging are met
func (t *AtomicTagImageTool) validateTagPrerequisites(result *AtomicTagImageResult, args AtomicTagImageArgs) error {
	// Basic input validation using RichError
	if args.SourceImage == "" {
		return types.NewValidationErrorBuilder("Source image reference is required", "source_image", args.SourceImage).
			WithOperation("tag_image").
			WithStage("input_validation").
			WithImmediateStep(1, "Provide source image", "Specify a valid Docker image reference like 'nginx:latest'").
			Build()
	}
	if args.TargetImage == "" {
		return types.NewValidationErrorBuilder("Target image reference is required", "target_image", args.TargetImage).
			WithOperation("tag_image").
			WithStage("input_validation").
			WithImmediateStep(1, "Provide target image", "Specify a target image name with tag like 'myregistry.com/nginx:production'").
			Build()
	}

	// Validate image name formats using RichError
	if !t.isValidImageReference(args.SourceImage) {
		return types.NewValidationErrorBuilder("Invalid source image reference format", "source_image", args.SourceImage).
			WithOperation("tag_image").
			WithStage("format_validation").
			WithRootCause("Image reference does not match required Docker naming conventions").
			WithImmediateStep(1, "Fix image format", "Use format: [registry/]name[:tag] (e.g., nginx:latest)").
			Build()
	}
	if !t.isValidImageReference(args.TargetImage) {
		return types.NewValidationErrorBuilder("Invalid target image reference format", "target_image", args.TargetImage).
			WithOperation("tag_image").
			WithStage("format_validation").
			WithRootCause("Image reference does not match required Docker naming conventions").
			WithImmediateStep(1, "Fix image format", "Use format: [registry/]name:tag (e.g., myregistry.com/nginx:production)").
			Build()
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
		return types.NewValidationErrorBuilder("Invalid argument type for atomic_tag_image", "args", args).
			WithField("expected", "AtomicTagImageArgs").
			WithField("received", fmt.Sprintf("%T", args)).
			Build()
	}

	if tagArgs.SourceImage == "" {
		return types.NewValidationErrorBuilder("SourceImage is required", "source_image", tagArgs.SourceImage).
			WithField("field", "source_image").
			Build()
	}

	if tagArgs.TargetImage == "" {
		return types.NewValidationErrorBuilder("TargetImage is required", "target_image", tagArgs.TargetImage).
			WithField("field", "target_image").
			Build()
	}

	if tagArgs.SessionID == "" {
		return types.NewValidationErrorBuilder("SessionID is required", "session_id", tagArgs.SessionID).
			WithField("field", "session_id").
			Build()
	}

	// Validate image reference formats
	if !t.isValidImageReference(tagArgs.SourceImage) {
		return types.NewValidationErrorBuilder("Invalid source image reference", "source_image", tagArgs.SourceImage).
			WithField("field", "source_image").
			Build()
	}

	if !t.isValidImageReference(tagArgs.TargetImage) {
		return types.NewValidationErrorBuilder("Invalid target image reference", "target_image", tagArgs.TargetImage).
			WithField("field", "target_image").
			Build()
	}

	return nil
}

// Execute implements SimpleTool interface with generic signature
func (t *AtomicTagImageTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	tagArgs, ok := args.(AtomicTagImageArgs)
	if !ok {
		return nil, types.NewValidationErrorBuilder("Invalid argument type for atomic_tag_image", "args", args).
			WithField("expected", "AtomicTagImageArgs").
			WithField("received", fmt.Sprintf("%T", args)).
			Build()
	}

	// Call the typed Execute method
	return t.ExecuteTyped(ctx, tagArgs)
}

// Tool interface implementation (unified interface)

// GetMetadata returns comprehensive tool metadata
func (t *AtomicTagImageTool) GetMetadata() mcptypes.ToolMetadata {
	return mcptypes.ToolMetadata{
		Name:         "atomic_tag_image",
		Description:  "Tags Docker images with new names for versioning, environment promotion, or registry organization",
		Version:      constants.AtomicToolVersion,
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

// Legacy interface methods for backward compatibility

// GetName returns the tool name (legacy SimpleTool compatibility)
func (t *AtomicTagImageTool) GetName() string {
	return t.GetMetadata().Name
}

// GetDescription returns the tool description (legacy SimpleTool compatibility)
func (t *AtomicTagImageTool) GetDescription() string {
	return t.GetMetadata().Description
}

// GetVersion returns the tool version (legacy SimpleTool compatibility)
func (t *AtomicTagImageTool) GetVersion() string {
	return t.GetMetadata().Version
}

// GetCapabilities returns the tool capabilities (legacy SimpleTool compatibility)
func (t *AtomicTagImageTool) GetCapabilities() contract.ToolCapabilities {
	return contract.ToolCapabilities{
		SupportsDryRun:    true,
		SupportsStreaming: true,
		IsLongRunning:     false,
		RequiresAuth:      false,
	}
}

// ExecuteTyped provides the original typed execute method
func (t *AtomicTagImageTool) ExecuteTyped(ctx context.Context, args AtomicTagImageArgs) (*AtomicTagImageResult, error) {
	return t.ExecuteTag(ctx, args)
}

// AI Context is provided by embedded internal.BaseAIContextResult
