package build

import (
	"github.com/Azure/container-copilot/pkg/mcp/internal"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-copilot/pkg/core/docker"
	"github.com/Azure/container-copilot/pkg/mcp/internal/ai_context"
	"github.com/Azure/container-copilot/pkg/mcp/internal/api/contract"
	"github.com/Azure/container-copilot/pkg/mcp/internal/mcperror"
	sessiontypes "github.com/Azure/container-copilot/pkg/mcp/internal/session"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
	"github.com/localrivet/gomcp/server"
	"github.com/rs/zerolog"
)

// standardPullStages provides common stages for pull operations
func standardPullStages() []mcptypes.ProgressStage {
	return []mcptypes.ProgressStage{
		{Name: "Initialize", Weight: 0.10, Description: "Loading session and validating inputs"},
		{Name: "Authenticate", Weight: 0.15, Description: "Authenticating with registry"},
		{Name: "Pull", Weight: 0.60, Description: "Pulling Docker image layers"},
		{Name: "Verify", Weight: 0.10, Description: "Verifying pull results"},
		{Name: "Finalize", Weight: 0.05, Description: "Updating session state"},
	}
}

// AtomicPullImageArgs defines arguments for atomic Docker image pull
type AtomicPullImageArgs struct {
	types.BaseToolArgs

	// Image information
	ImageRef string `json:"image_ref" jsonschema:"required,pattern=^[a-zA-Z0-9][a-zA-Z0-9._/-]*(:([a-zA-Z0-9][a-zA-Z0-9._-]*|latest))?$" description:"The full image reference to pull (e.g. nginx:latest, myregistry.com/app:v1.0.0)"`

	// Pull configuration
	Timeout    int  `json:"timeout,omitempty" jsonschema:"minimum=30,maximum=3600" description:"Pull timeout in seconds (default: 600)"`
	RetryCount int  `json:"retry_count,omitempty" jsonschema:"minimum=0,maximum=10" description:"Number of retry attempts (default: 3)"`
	Force      bool `json:"force,omitempty" description:"Force pull even if image already exists locally"`
}

// AtomicPullImageResult defines the response from atomic Docker image pull
type AtomicPullImageResult struct {
	types.BaseToolResponse
	internal.BaseAIContextResult      // Embedded for AI context methods
	Success             bool `json:"success"`

	// Session context
	SessionID    string `json:"session_id"`
	WorkspaceDir string `json:"workspace_dir"`

	// Pull configuration
	ImageRef string `json:"image_ref"`
	Registry string `json:"registry"`

	// Pull results from core operations
	PullResult *docker.PullResult `json:"pull_result,omitempty"`

	// Timing information
	PullDuration  time.Duration `json:"pull_duration"`
	TotalDuration time.Duration `json:"total_duration"`

	// Rich context for Claude reasoning
	PullContext *PullContext `json:"pull_context"`

	// Rich error information if operation failed
}

// PullContext provides rich context for Claude to reason about
type PullContext struct {
	// Pull analysis
	PullStatus    string  `json:"pull_status"`
	LayersPulled  int     `json:"layers_pulled"`
	LayersCached  int     `json:"layers_cached"`
	PullSizeMB    float64 `json:"pull_size_mb"`
	CacheHitRatio float64 `json:"cache_hit_ratio"`

	// Registry information
	RegistryType     string `json:"registry_type"`
	RegistryEndpoint string `json:"registry_endpoint"`
	AuthMethod       string `json:"auth_method,omitempty"`

	// Error analysis
	ErrorType     string `json:"error_type,omitempty"`
	ErrorCategory string `json:"error_category,omitempty"`
	IsRetryable   bool   `json:"is_retryable"`

	// Next step suggestions
	NextStepSuggestions []string `json:"next_step_suggestions"`
	TroubleshootingTips []string `json:"troubleshooting_tips,omitempty"`
	AuthenticationGuide []string `json:"authentication_guide,omitempty"`
}

// AtomicPullImageTool implements atomic Docker image pull using core operations
type AtomicPullImageTool struct {
	pipelineAdapter mcptypes.PipelineOperations
	sessionManager  mcptypes.ToolSessionManager
	logger          zerolog.Logger
}

// NewAtomicPullImageTool creates a new atomic pull image tool
func NewAtomicPullImageTool(adapter mcptypes.PipelineOperations, sessionManager mcptypes.ToolSessionManager, logger zerolog.Logger) *AtomicPullImageTool {
	return &AtomicPullImageTool{
		pipelineAdapter: adapter,
		sessionManager:  sessionManager,
		logger:          logger.With().Str("tool", "atomic_pull_image").Logger(),
	}
}

// ExecutePullImage runs the atomic Docker image pull (legacy method)
func (t *AtomicPullImageTool) ExecutePullImage(ctx context.Context, args AtomicPullImageArgs) (*AtomicPullImageResult, error) {
	startTime := time.Now()

	// Create result object early for error handling
	result := &AtomicPullImageResult{
		BaseToolResponse:    types.NewBaseResponse("atomic_pull_image", args.SessionID, args.DryRun),
		BaseAIContextResult: internal.NewBaseAIContextResult("pull", false, 0), // Will be updated later
		ImageRef:            args.ImageRef,
		PullContext:         &PullContext{},
	}

	// Direct execution without progress tracking
	return t.executeWithoutProgress(ctx, args, result, startTime)
}

// ExecuteWithContext runs the atomic Docker image pull with GoMCP progress tracking
func (t *AtomicPullImageTool) ExecuteWithContext(serverCtx *server.Context, args AtomicPullImageArgs) (*AtomicPullImageResult, error) {
	startTime := time.Now()

	// Create result object early for error handling
	result := &AtomicPullImageResult{
		BaseToolResponse:    types.NewBaseResponse("atomic_pull_image", args.SessionID, args.DryRun),
		BaseAIContextResult: internal.NewBaseAIContextResult("pull", false, 0), // Will be updated later
		ImageRef:            args.ImageRef,
		PullContext:         &PullContext{},
	}

	// Create progress adapter for GoMCP using standard pull stages
	_ = internal.NewGoMCPProgressAdapter(serverCtx, []mcptypes.ProgressStage{{Name: "Initialize", Weight: 0.10, Description: "Loading session"}, {Name: "Pull", Weight: 0.80, Description: "Pulling image"}, {Name: "Finalize", Weight: 0.10, Description: "Updating state"}})

	// Execute with progress tracking
	ctx := context.Background()
	err := t.executeWithProgress(ctx, args, result, startTime, nil)

	// Always set total duration
	result.TotalDuration = time.Since(startTime)

	// Update AI context with final result
	result.BaseAIContextResult = internal.NewBaseAIContextResult("pull", result.Success, result.TotalDuration)

	// Complete progress tracking
	if err != nil {
		t.logger.Info().Msg("Pull failed")
		result.Success = false
		return result, nil // Return result with error info, not the error itself
	} else {
		t.logger.Info().Msg("Pull completed successfully")
	}

	return result, nil
}

// executeWithProgress handles the main execution with progress reporting
func (t *AtomicPullImageTool) executeWithProgress(ctx context.Context, args AtomicPullImageArgs, result *AtomicPullImageResult, startTime time.Time, reporter mcptypes.ProgressReporter) error {
	// Stage 1: Initialize - Loading session and validating inputs
	t.logger.Info().Msg("Loading session")
	sessionInterface, err := t.sessionManager.GetSession(args.SessionID)
	if err != nil {
		t.logger.Error().Err(err).Str("session_id", args.SessionID).Msg("Failed to get session")
		return mcperror.NewSessionNotFound(args.SessionID)
	}
	session := sessionInterface.(*sessiontypes.SessionState)

	// Set session details
	result.SessionID = session.SessionID
	result.WorkspaceDir = t.pipelineAdapter.GetSessionWorkspace(session.SessionID)

	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("image_ref", args.ImageRef).
		Msg("Starting atomic Docker pull")

	t.logger.Info().Msg("Session initialized")

	// Handle dry-run
	if args.DryRun {
		// Extract registry even in dry-run for testing
		result.Registry = t.extractRegistryURL(args.ImageRef)
		result.Success = true
		// Update AI context with success
		result.BaseAIContextResult = internal.NewBaseAIContextResult("pull", true, result.TotalDuration)
		result.PullContext.PullStatus = "dry-run"
		result.PullContext.NextStepSuggestions = []string{
			"This is a dry-run - no actual pull was performed",
			"Remove dry_run flag to perform actual pull",
		}
		t.logger.Info().Msg("Dry-run completed")
		return nil
	}

	// Stage 2: Authenticate - Authenticating with registry
	t.logger.Info().Msg("Validating prerequisites")
	if err := t.validatePullPrerequisites(result, args); err != nil {
		t.logger.Error().Err(err).
			Str("session_id", session.SessionID).
			Str("image_ref", result.ImageRef).
			Msg("Pull prerequisites validation failed")
		return mcperror.NewWithData("prerequisites_validation_failed", "Pull prerequisites validation failed", map[string]interface{}{
			"session_id": session.SessionID,
			"image_ref":  result.ImageRef,
		})
	}

	t.logger.Info().Msg("Prerequisites validated")

	// Stage 3: Pull - Pulling Docker image layers
	t.logger.Info().Msg("Pulling Docker image")
	return t.performPull(ctx, session, args, result, reporter)
}

// executeWithoutProgress handles execution without progress tracking (fallback)
func (t *AtomicPullImageTool) executeWithoutProgress(ctx context.Context, args AtomicPullImageArgs, result *AtomicPullImageResult, startTime time.Time) (*AtomicPullImageResult, error) {
	// Get session
	sessionInterface, err := t.sessionManager.GetSession(args.SessionID)
	if err != nil {
		t.logger.Error().Err(err).Str("session_id", args.SessionID).Msg("Failed to get session")
		result.Success = false
		result.TotalDuration = time.Since(startTime)
		return result, mcperror.NewSessionNotFound(args.SessionID)
	}
	session := sessionInterface.(*sessiontypes.SessionState)

	// Set session details
	result.SessionID = session.SessionID
	result.WorkspaceDir = t.pipelineAdapter.GetSessionWorkspace(session.SessionID)

	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("image_ref", args.ImageRef).
		Msg("Starting atomic Docker pull")

	// Handle dry-run
	if args.DryRun {
		// Extract registry even in dry-run for testing
		result.Registry = t.extractRegistryURL(args.ImageRef)
		result.Success = true
		// Update AI context with success
		result.BaseAIContextResult = internal.NewBaseAIContextResult("pull", true, result.TotalDuration)
		result.PullContext.PullStatus = "dry-run"
		result.PullContext.NextStepSuggestions = []string{
			"This is a dry-run - no actual pull was performed",
			"Remove dry_run flag to perform actual pull",
		}
		result.TotalDuration = time.Since(startTime)
		return result, nil
	}

	// Validate prerequisites
	if err := t.validatePullPrerequisites(result, args); err != nil {
		t.logger.Error().Err(err).
			Str("session_id", session.SessionID).
			Str("image_ref", result.ImageRef).
			Msg("Pull prerequisites validation failed")
		result.Success = false
		result.TotalDuration = time.Since(startTime)
		return result, mcperror.NewWithData("prerequisites_validation_failed", "Pull prerequisites validation failed", map[string]interface{}{
			"session_id": session.SessionID,
			"image_ref":  result.ImageRef,
		})
	}

	// Perform the pull without progress reporting
	err = t.performPull(ctx, session, args, result, nil)
	result.TotalDuration = time.Since(startTime)

	// Update AI context with final result
	result.BaseAIContextResult = internal.NewBaseAIContextResult("pull", result.Success, result.TotalDuration)

	if err != nil {
		result.Success = false
		return result, nil
	}

	return result, nil
}

// performPull contains the actual pull logic that can be used with or without progress reporting
func (t *AtomicPullImageTool) performPull(ctx context.Context, session *sessiontypes.SessionState, args AtomicPullImageArgs, result *AtomicPullImageResult, reporter mcptypes.ProgressReporter) error {
	// Report progress if reporter is available
	// Progress reporting removed

	// Extract registry from image reference
	result.Registry = t.extractRegistryURL(args.ImageRef)

	// Pull Docker image using pipeline adapter
	pullStartTime := time.Now()

	err := t.pipelineAdapter.PullDockerImage(session.SessionID, args.ImageRef)
	result.PullDuration = time.Since(pullStartTime)

	if err != nil {
		result.Success = false
		t.logger.Error().Err(err).Str("image_ref", args.ImageRef).Msg("Failed to pull image")
		return mcperror.NewWithData("image_pull_failed", "Failed to pull image", map[string]interface{}{
			"image_ref":  args.ImageRef,
			"session_id": session.SessionID,
		})
	}

	// Update result with pull operation status
	result.Success = true
	result.PullResult = &docker.PullResult{
		Success:  true,
		ImageRef: args.ImageRef,
		Registry: result.Registry,
	}

	result.PullContext.PullStatus = "successful"
	result.PullContext.NextStepSuggestions = []string{
		fmt.Sprintf("Image %s pulled successfully", args.ImageRef),
		"You can now use this image for building or deployment",
	}

	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("image_ref", result.ImageRef).
		Str("registry", result.Registry).
		Dur("pull_duration", result.PullDuration).
		Msg("Docker pull completed successfully")

	// Progress reporting removed

	// Stage 4: Verify - Verifying pull results
	// Progress reporting removed

	// Generate rich context for Claude reasoning
	t.generatePullContext(result, args)

	// Progress reporting removed

	// Stage 5: Finalize - Updating session state
	// Progress reporting removed

	// Update session state
	if err := t.updateSessionState(session, result); err != nil {
		t.logger.Warn().Err(err).Msg("Failed to update session state")
	}

	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("image_ref", result.ImageRef).
		Bool("success", result.Success).
		Msg("Atomic Docker pull completed")

	// Progress reporting removed

	return nil
}

// Helper methods

func (t *AtomicPullImageTool) extractRegistryURL(imageRef string) string {
	parts := strings.Split(imageRef, "/")
	if len(parts) >= 2 {
		firstPart := parts[0]
		// Check if first part looks like a registry (contains dots or localhost with port)
		if strings.Contains(firstPart, ".") || strings.HasPrefix(firstPart, "localhost") {
			return firstPart
		}
	}
	return "docker.io" // Default to Docker Hub
}

func (t *AtomicPullImageTool) validatePullPrerequisites(result *AtomicPullImageResult, args AtomicPullImageArgs) error {
	// Basic image reference validation for user experience
	if !strings.Contains(args.ImageRef, ":") {
		result.PullContext.TroubleshootingTips = append(
			result.PullContext.TroubleshootingTips,
			"Consider specifying a tag (e.g., myapp:latest) for more predictable pulls",
		)
	}
	return nil
}

func (t *AtomicPullImageTool) generatePullContext(result *AtomicPullImageResult, args AtomicPullImageArgs) {
	ctx := result.PullContext

	// Generate next step suggestions
	if result.Success {
		ctx.NextStepSuggestions = append(ctx.NextStepSuggestions,
			fmt.Sprintf("Image %s pulled successfully", result.ImageRef))
		ctx.NextStepSuggestions = append(ctx.NextStepSuggestions,
			"You can now build containers or deploy applications using this image")
		ctx.NextStepSuggestions = append(ctx.NextStepSuggestions,
			fmt.Sprintf("Image is available locally as: %s", result.ImageRef))
	} else {
		ctx.NextStepSuggestions = append(ctx.NextStepSuggestions,
			"Pull failed - review error details and troubleshooting tips")

		if ctx.IsRetryable {
			ctx.NextStepSuggestions = append(ctx.NextStepSuggestions,
				"This error appears to be temporary - consider retrying")
		}
	}
}

func (t *AtomicPullImageTool) updateSessionState(session *sessiontypes.SessionState, result *AtomicPullImageResult) error {
	// Update session with pull results
	if session.Metadata == nil {
		session.Metadata = make(map[string]interface{})
	}

	// Update metadata for pull tracking
	session.Metadata["last_pulled_image"] = result.ImageRef
	session.Metadata["last_pull_registry"] = result.Registry
	session.Metadata["last_pull_success"] = result.Success

	if result.Success && result.PullResult != nil {
		session.Metadata["pull_duration_seconds"] = result.PullDuration.Seconds()
		if result.PullContext.CacheHitRatio > 0 {
			session.Metadata["pull_cache_ratio"] = result.PullContext.CacheHitRatio
		}
	}

	session.UpdateLastAccessed()

	return t.sessionManager.UpdateSession(session.SessionID, func(s *sessiontypes.SessionState) {
		*s = *session
	})
}

// GenerateRecommendations implements ai_context.Recommendable
func (r *AtomicPullImageResult) GenerateRecommendations() []ai_context.Recommendation {
	recommendations := []ai_context.Recommendation{}

	if !r.Success {
		recommendations = append(recommendations, ai_context.Recommendation{
			Priority:    "high",
			Category:    "troubleshooting",
			Title:       "Resolve Image Pull Failure",
			Description: "Investigate and fix the image pull failure",
			Reasoning:   []string{"Application cannot run without the required container image"},
			Impact:      "high",
			Benefits:    []string{"Enables successful container deployment and application startup"},
		})
	}

	// Registry optimization
	if r.PullContext != nil && r.PullContext.CacheHitRatio < 0.5 {
		recommendations = append(recommendations, ai_context.Recommendation{
			Priority:    "medium",
			Category:    "optimization",
			Title:       "Optimize Image Caching",
			Description: "Implement image caching strategies to improve pull performance",
			Reasoning:   []string{"Better caching reduces pull times and bandwidth usage"},
			Impact:      "medium",
			Benefits:    []string{"Faster deployments and reduced network overhead"},
		})
	}

	return recommendations
}

// CreateRemediationPlan implements ai_context.Recommendable
func (r *AtomicPullImageResult) CreateRemediationPlan() *ai_context.RemediationPlan {
	plan := &ai_context.RemediationPlan{
		Priority: "low",
		Steps:    []ai_context.RemediationStep{},
	}

	if !r.Success {
		plan.Priority = "high"

		// Step 1: Check image availability
		plan.Steps = append(plan.Steps, ai_context.RemediationStep{
			Action:      "verify",
			Description: "Verify image exists in registry",
			Command:     fmt.Sprintf("docker manifest inspect %s", r.ImageRef),
		})

		// Step 2: Check authentication
		plan.Steps = append(plan.Steps, ai_context.RemediationStep{
			Action:      "authenticate",
			Description: "Verify registry authentication",
			Command:     fmt.Sprintf("docker login %s", r.Registry),
		})

		// Step 3: Retry pull
		plan.Steps = append(plan.Steps, ai_context.RemediationStep{
			Action:      "retry",
			Description: "Retry image pull with verbose output",
			Command:     fmt.Sprintf("docker pull %s", r.ImageRef),
		})
	}

	return plan
}

// GetAlternativeStrategies implements ai_context.Recommendable
func (r *AtomicPullImageResult) GetAlternativeStrategies() []ai_context.AlternativeStrategy {
	alternatives := []ai_context.AlternativeStrategy{}

	// Registry mirrors
	alternatives = append(alternatives, ai_context.AlternativeStrategy{
		Name:        "Registry Mirror",
		Description: "Use a registry mirror or pull-through cache",
		Benefits:    []string{"Faster pulls", "Reduced bandwidth", "Better availability"},
		Drawbacks:   []string{"Additional infrastructure", "Potential sync delays"},
		Complexity:  "medium",
		Timeline:    "days",
	})

	// Pre-pulling
	alternatives = append(alternatives, ai_context.AlternativeStrategy{
		Name:        "Image Pre-pulling",
		Description: "Pre-pull images to all nodes using DaemonSet",
		Benefits:    []string{"Faster pod startup", "Predictable behavior", "Reduced pull failures"},
		Drawbacks:   []string{"Storage overhead", "Update complexity", "Network load"},
		Complexity:  "medium",
		Timeline:    "hours",
	})

	return alternatives
}

// Tool interface implementation (unified interface)

// GetMetadata returns comprehensive tool metadata
func (t *AtomicPullImageTool) GetMetadata() mcptypes.ToolMetadata {
	return mcptypes.ToolMetadata{
		Name:         "atomic_pull_image",
		Description:  "Pulls Docker images from container registries with authentication support and detailed progress tracking",
		Version:      "1.0.0",
		Category:     "docker",
		Dependencies: []string{"docker"},
		Capabilities: []string{
			"supports_streaming",
		},
		Requirements: []string{"docker_daemon"},
		Parameters: map[string]string{
			"image_ref":   "required - Full image reference to pull",
			"timeout":     "optional - Pull timeout in seconds",
			"retry_count": "optional - Number of retry attempts",
			"force":       "optional - Force pull even if image exists",
		},
		Examples: []mcptypes.ToolExample{
			{
				Name:        "basic_pull",
				Description: "Pull a Docker image from registry",
				Input: map[string]interface{}{
					"session_id": "session-123",
					"image_ref":  "nginx:latest",
				},
				Output: map[string]interface{}{
					"success":       true,
					"image_ref":     "nginx:latest",
					"pull_duration": "30s",
				},
			},
		},
	}
}

// Validate validates the tool arguments (unified interface)
func (t *AtomicPullImageTool) Validate(ctx context.Context, args interface{}) error {
	pullArgs, ok := args.(AtomicPullImageArgs)
	if !ok {
		return mcperror.NewWithData("invalid_arguments", "Invalid argument type for atomic_pull_image", map[string]interface{}{
			"expected": "AtomicPullImageArgs",
			"received": fmt.Sprintf("%T", args),
		})
	}

	if pullArgs.ImageRef == "" {
		return mcperror.NewWithData("missing_required_field", "ImageRef is required", map[string]interface{}{
			"field": "image_ref",
		})
	}

	if pullArgs.SessionID == "" {
		return mcperror.NewWithData("missing_required_field", "SessionID is required", map[string]interface{}{
			"field": "session_id",
		})
	}

	return nil
}

// Execute implements unified Tool interface
func (t *AtomicPullImageTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	pullArgs, ok := args.(AtomicPullImageArgs)
	if !ok {
		return nil, mcperror.NewWithData("invalid_arguments", "Invalid argument type for atomic_pull_image", map[string]interface{}{
			"expected": "AtomicPullImageArgs",
			"received": fmt.Sprintf("%T", args),
		})
	}

	// Call the typed Execute method
	return t.ExecuteTyped(ctx, pullArgs)
}

// Legacy interface methods for backward compatibility

// GetName returns the tool name (legacy SimpleTool compatibility)
func (t *AtomicPullImageTool) GetName() string {
	return t.GetMetadata().Name
}

// GetDescription returns the tool description (legacy SimpleTool compatibility)
func (t *AtomicPullImageTool) GetDescription() string {
	return t.GetMetadata().Description
}

// GetVersion returns the tool version (legacy SimpleTool compatibility)
func (t *AtomicPullImageTool) GetVersion() string {
	return t.GetMetadata().Version
}

// GetCapabilities returns the tool capabilities (legacy SimpleTool compatibility)
func (t *AtomicPullImageTool) GetCapabilities() contract.ToolCapabilities {
	return contract.ToolCapabilities{
		SupportsDryRun:    true,
		SupportsStreaming: true,
		IsLongRunning:     true,
		RequiresAuth:      false,
	}
}

// ExecuteTyped provides the original typed execute method
func (t *AtomicPullImageTool) ExecuteTyped(ctx context.Context, args AtomicPullImageArgs) (*AtomicPullImageResult, error) {
	startTime := time.Now()

	// Create result object early for error handling
	result := &AtomicPullImageResult{
		BaseToolResponse:    types.NewBaseResponse("atomic_pull_image", args.SessionID, args.DryRun),
		BaseAIContextResult: internal.NewBaseAIContextResult("pull", false, 0), // Will be updated later
		ImageRef:            args.ImageRef,
		PullContext:         &PullContext{},
	}

	// Direct execution without progress tracking
	return t.executeWithoutProgress(ctx, args, result, startTime)
}
