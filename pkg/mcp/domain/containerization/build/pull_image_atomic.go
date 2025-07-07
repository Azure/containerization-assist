package build

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/core/docker"
	"github.com/Azure/container-kit/pkg/mcp/application/api"

	"github.com/Azure/container-kit/pkg/mcp/application/core"
	"github.com/Azure/container-kit/pkg/mcp/domain/types"
	"github.com/Azure/container-kit/pkg/mcp/domain/validation"
	"github.com/Azure/container-kit/pkg/mcp/services"

	// mcp import removed - using mcptypes
	mcptypes "github.com/Azure/container-kit/pkg/mcp/application/core"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
	internaltypes "github.com/Azure/container-kit/pkg/mcp/domain/types"
	"github.com/Azure/container-kit/pkg/mcp/infra/diagnostics"
	"log/slog"
)

// AtomicPullImageArgs defines arguments for atomic Docker image pull
type AtomicPullImageArgs struct {
	internaltypes.BaseToolArgs
	// Image information
	ImageRef string `json:"image_ref" validate:"required,docker_image" description:"The full image reference to pull (e.g. nginx:latest, myregistry.com/app:v1.0.0)"`
	// Pull configuration
	Timeout    int  `json:"timeout,omitempty" validate:"omitempty,min=30,max=3600" description:"Pull timeout in seconds (default: 600)"`
	RetryCount int  `json:"retry_count,omitempty" validate:"omitempty,min=0,max=10" description:"Number of retry attempts (default: 3)"`
	Force      bool `json:"force,omitempty" description:"Force pull even if image already exists locally"`
	DryRun     bool `json:"dry_run,omitempty" description:"Preview changes without executing"`
}

// AtomicPullImageResult defines the response from atomic Docker image pull
type AtomicPullImageResult struct {
	types.BaseToolResponse
	core.BaseAIContextResult      // Embedded for AI context methods
	Success                  bool `json:"success"`
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
	pipelineAdapter mcptypes.TypedPipelineOperations
	sessionStore    services.SessionStore // Focused service interface
	sessionState    services.SessionState // Focused service interface
	logger          *slog.Logger
	analyzer        diagnostics.FailureAnalyzer
	fixingMixin     *AtomicToolFixingMixin
}

// NewAtomicPullImageToolWithServices creates a new atomic pull image tool using service container
func NewAtomicPullImageToolWithServices(adapter mcptypes.TypedPipelineOperations, serviceContainer services.ServiceContainer, logger *slog.Logger) *AtomicPullImageTool {
	toolLogger := logger.With("tool", "atomic_pull_image")

	// Use focused services directly - no wrapper needed!
	return createAtomicPullImageTool(adapter, serviceContainer.SessionStore(), serviceContainer.SessionState(), toolLogger)
}

// createAtomicPullImageTool is the common creation logic
func createAtomicPullImageTool(adapter mcptypes.TypedPipelineOperations, sessionStore services.SessionStore, sessionState services.SessionState, logger *slog.Logger) *AtomicPullImageTool {
	return &AtomicPullImageTool{
		pipelineAdapter: adapter,
		sessionStore:    sessionStore,
		sessionState:    sessionState,
		logger:          logger,
	}
}

// SetAnalyzer sets the analyzer for failure analysis
func (t *AtomicPullImageTool) SetAnalyzer(analyzer diagnostics.FailureAnalyzer) {
	t.analyzer = analyzer
}

// SetFixingMixin sets the fixing mixin for automatic error recovery
func (t *AtomicPullImageTool) SetFixingMixin(mixin *AtomicToolFixingMixin) {
	t.fixingMixin = mixin
}

// ExecuteWithFixes runs the atomic Docker image pull with automatic fixes
func (t *AtomicPullImageTool) ExecuteWithFixes(ctx context.Context, args AtomicPullImageArgs) (*AtomicPullImageResult, error) {
	if t.fixingMixin != nil && !args.DryRun {
		// Create wrapper operation for pull process
		var result *AtomicPullImageResult
		// Progress tracking infrastructure removed
		operation := NewDockerOperation(DockerOperationConfig{
			Type:          OperationPull,
			Name:          args.ImageRef,
			RetryAttempts: 3,
			Timeout:       5 * time.Minute,
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
		}, nil) // progress parameter removed
		if err := operation.Execute(ctx); err != nil {
			return nil, err
		}
		return result, nil
	}
	return t.executeWithoutProgress(ctx, args, nil, time.Now())
}

// executeWithoutProgress handles execution without progress tracking (fallback)
func (t *AtomicPullImageTool) executeWithoutProgress(ctx context.Context, args AtomicPullImageArgs, result *AtomicPullImageResult, startTime time.Time) (*AtomicPullImageResult, error) {
	// Get session using services
	sessionData, err := t.sessionStore.Get(ctx, args.SessionID)
	if err != nil {
		t.logger.Error("Failed to get session", "error", err, "session_id", args.SessionID)
		result.Success = false
		result.TotalDuration = time.Since(startTime)
		return result, diagnostics.NewSessionNotFound(args.SessionID)
	}
	session := sessionData
	// Set session details
	result.SessionID = session.ID
	result.WorkspaceDir = t.pipelineAdapter.GetSessionWorkspace(session.ID)
	t.logger.Info("Starting atomic Docker pull",
		"session_id", session.ID,
		"image_ref", args.ImageRef)
	// Handle dry-run
	if args.DryRun {
		// Extract registry even in dry-run for testing
		result.Registry = t.extractRegistryURL(args.ImageRef)
		result.Success = true
		// Update AI context with success
		result.BaseAIContextResult = core.NewBaseAIContextResult("pull", true, result.TotalDuration)
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
		t.logger.Error("Pull prerequisites validation failed",
			"error", err,
			"session_id", session.ID,
			"image_ref", result.ImageRef)
		result.Success = false
		result.TotalDuration = time.Since(startTime)
		return result, errors.NewError().Messagef("pull prerequisites validation failed: session_id=%s, image_ref=%s", session.ID, result.ImageRef).WithLocation(

		// Perform the pull without progress reporting
		).Build()
	}

	// Convert api.Session to core.SessionState
	coreSession := &core.SessionState{
		SessionID: session.ID,
		Metadata:  session.Metadata,
	}
	err = t.performPull(ctx, coreSession, args, result, nil)
	result.TotalDuration = time.Since(startTime)
	// Update AI context with final result
	result.BaseAIContextResult = core.NewBaseAIContextResult("pull", result.Success, result.TotalDuration)
	if err != nil {
		result.Success = false
		return result, nil
	}
	return result, nil
}

// performPull contains the actual pull logic that can be used with or without progress reporting
func (t *AtomicPullImageTool) performPull(ctx context.Context, session *core.SessionState, args AtomicPullImageArgs, result *AtomicPullImageResult, reporter interface{}) error {
	// Report progress if reporter is available
	// Progress reporting removed
	// Extract registry from image reference
	result.Registry = t.extractRegistryURL(args.ImageRef)
	// Pull Docker image using pipeline adapter
	pullStartTime := time.Now()
	// Convert to typed parameters for PullImageTyped
	pullParams := core.PullImageParams{
		ImageRef: args.ImageRef,
		Platform: "", // Default platform
	}
	_, err := t.pipelineAdapter.PullImageTyped(ctx, session.SessionID, pullParams)
	result.PullDuration = time.Since(pullStartTime)
	if err != nil {
		result.Success = false
		t.logger.Error("Failed to pull image", "error", err, "image_ref", args.ImageRef)
		return errors.NewError().Messagef("failed to pull image %s for session %s", args.ImageRef, session.SessionID).WithLocation(

		// Update result with pull operation status
		).Build()
	}

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
	t.logger.Info("Docker pull completed successfully",
		"session_id", session.SessionID,
		"image_ref", result.ImageRef,
		"registry", result.Registry,
		"pull_duration", result.PullDuration)
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
		t.logger.Warn("Failed to update session state", "error", err)
	}
	t.logger.Info("Atomic Docker pull completed",
		"session_id", session.SessionID,
		"image_ref", result.ImageRef,
		"success", result.Success)
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
func (t *AtomicPullImageTool) updateSessionState(session *core.SessionState, result *AtomicPullImageResult) error {
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
	session.UpdatedAt = time.Now()
	// Use the correct interface method for session updates
	return t.pipelineAdapter.UpdateSessionState(session.SessionID, func(s *core.SessionState) {
		*s = *session
	})
}

// GenerateRecommendations implements ai_context.Recommendable
func (r *AtomicPullImageResult) GenerateRecommendations() []types.Recommendation {
	var recommendations []types.Recommendation

	// Add recommendations based on pull result
	if !r.Success {
		recommendations = append(recommendations, types.Recommendation{
			Type:        "error",
			Title:       "Pull Failed",
			Description: "Image pull failed - consider checking image name and registry credentials",
			Priority:    1, // high priority
			Action:      "verify_image_registry",
		})
	}

	// Add performance recommendations
	if r.PullDuration > 0 && r.PullDuration.Minutes() > 2 {
		recommendations = append(recommendations, types.Recommendation{
			Type:        "performance",
			Title:       "Slow Pull Performance",
			Description: "Image pull took longer than expected - consider using a local registry",
			Priority:    2, // medium priority
			Action:      "optimize_registry_location",
		})
	}

	return recommendations
}

// CreateRemediationPlan implements ai_context.Recommendable
// Returns nil until AI context types are fully defined in mcptypes
func (r *AtomicPullImageResult) CreateRemediationPlan() interface{} {
	// Implementation pending AI context type definitions
	return nil
}

// GetAlternativeStrategies implements ai_context.Recommendable
func (r *AtomicPullImageResult) GetAlternativeStrategies() []types.AlternativeStrategy {
	var strategies []types.AlternativeStrategy

	// Add alternative strategies based on pull failure
	if !r.Success {
		strategies = append(strategies, types.AlternativeStrategy{
			Name:        "Use Different Registry",
			Description: "Try pulling from an alternative container registry",
			Priority:    1, // high priority
			Pros:        []string{"Different registry may have the image", "Faster pull if closer"},
			Cons:        []string{"May require different authentication", "Image might be different version"},
		})

		strategies = append(strategies, types.AlternativeStrategy{
			Name:        "Build Locally",
			Description: "Build the image locally instead of pulling",
			Priority:    2, // medium priority
			Pros:        []string{"Can customize build process", "Guaranteed local availability"},
			Cons:        []string{"Takes longer than pulling", "Requires Dockerfile"},
		})
	}

	return strategies
}

// Tool interface implementation (unified interface)
// GetMetadata returns comprehensive tool metadata
func (t *AtomicPullImageTool) GetMetadata() api.ToolMetadata {
	return api.ToolMetadata{
		Name:         "atomic_pull_image",
		Description:  "Pulls Docker images from container registries with authentication support and detailed progress tracking",
		Version:      "1.0.0",
		Category:     api.ToolCategory("docker"),
		Tags:         []string{"docker", "pull", "registry"},
		Status:       api.ToolStatus("active"),
		Dependencies: []string{"docker"},
		Capabilities: []string{
			"supports_streaming",
		},
		Requirements: []string{"docker_daemon"},
		RegisteredAt: time.Now(),
		LastModified: time.Now(),
	}
}

// Validate validates the tool arguments (unified interface)
func (t *AtomicPullImageTool) Validate(ctx context.Context, args interface{}) error {
	// Validate using tag-based validation
	return validation.ValidateTaggedStruct(args)
}

func (t *AtomicPullImageTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	pullArgs, ok := args.(AtomicPullImageArgs)
	if !ok {
		return nil, errors.NewError().Messagef("invalid argument type for atomic_pull_image: expected AtomicPullImageArgs, received %T", args).WithLocation(

		// Call the typed Execute method
		).Build()
	}

	return t.ExecuteTyped(ctx, pullArgs)
}

// ExecuteTyped provides the original typed execute method
func (t *AtomicPullImageTool) ExecuteTyped(ctx context.Context, args AtomicPullImageArgs) (*AtomicPullImageResult, error) {
	startTime := time.Now()
	// Create result object early for error handling
	result := &AtomicPullImageResult{
		BaseToolResponse: types.BaseToolResponse{
			Success:   false, // Will be updated on success
			Timestamp: time.Now(),
		},
		BaseAIContextResult: core.NewBaseAIContextResult("pull", false, 0), // Will be updated later
		ImageRef:            args.ImageRef,
		PullContext:         &PullContext{},
	}
	// Direct execution without progress tracking
	return t.executeWithoutProgress(ctx, args, result, startTime)
}
