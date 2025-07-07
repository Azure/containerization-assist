package build

import (
	"context"
	"fmt"
	"strings"
	"time"

	// mcp import removed - using mcptypes

	coredocker "github.com/Azure/container-kit/pkg/core/docker"
	"github.com/Azure/container-kit/pkg/mcp/application/core"
	internalcommon "github.com/Azure/container-kit/pkg/mcp/domain/common"
	"github.com/Azure/container-kit/pkg/mcp/domain/validation"
	"github.com/Azure/container-kit/pkg/mcp/services"

	// "github.com/Azure/container-kit/pkg/mcp/application/internal/runtime" // Temporarily commented to avoid import cycle
	"github.com/Azure/container-kit/pkg/mcp/domain/types"

	mcptypes "github.com/Azure/container-kit/pkg/mcp/application/core"

	"log/slog"

	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/localrivet/gomcp/server"
)

// Note: Using centralized stage definitions from core.StandardPushStages()
// AtomicPushImageArgs defines arguments for atomic Docker image push
type AtomicPushImageArgs struct {
	types.BaseToolArgs
	// Image information
	ImageRef    string `json:"image_ref" validate:"required,docker_image" description:"Full image reference to push (e.g., myregistry.azurecr.io/myapp:latest)"`
	RegistryURL string `json:"registry_url,omitempty" validate:"omitempty,registry_url" description:"Override registry URL (optional - extracted from image_ref if not provided)"`
	// Push configuration
	Timeout    int  `json:"timeout,omitempty" validate:"omitempty,min=30,max=3600" description:"Push timeout in seconds (default: 600)"`
	RetryCount int  `json:"retry_count,omitempty" validate:"omitempty,min=0,max=10" description:"Number of retry attempts (default: 3)"`
	Force      bool `json:"force,omitempty" description:"Force push even if image already exists"`
}

// AtomicPushImageResult defines the response from atomic Docker image push
type AtomicPushImageResult struct {
	types.BaseToolResponse
	core.BaseAIContextResult      // Embed AI context methods
	Success                  bool `json:"success"`
	// Session context
	SessionID    string `json:"session_id"`
	WorkspaceDir string `json:"workspace_dir"`
	// Push configuration
	ImageRef    string `json:"image_ref"`
	RegistryURL string `json:"registry_url"`
	// Push results from core operations
	PushResult *coredocker.RegistryPushResult `json:"push_result"`
	// Timing information
	PushDuration  time.Duration `json:"push_duration"`
	TotalDuration time.Duration `json:"total_duration"`
	// Rich context for Claude reasoning
	PushContext *PushContext `json:"push_context"`
}

// PushContext provides rich context for Claude to reason about
type PushContext struct {
	// Push analysis
	PushStatus    string  `json:"push_status"`
	LayersPushed  int     `json:"layers_pushed"`
	LayersCached  int     `json:"layers_cached"`
	PushSizeMB    float64 `json:"push_size_mb"`
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

// AtomicPushImageTool implements atomic Docker image push using core operations
type AtomicPushImageTool struct {
	PipelineAdapter mcptypes.TypedPipelineOperations // Exported for direct access
	SessionStore    services.SessionStore            // Focused service interface
	SessionState    services.SessionState            // Focused service interface
	Logger          *slog.Logger                     // Exported for direct access
	Analyzer        *internalcommon.FailureAnalyzer  // Exported for direct access
	FixingMixin     *AtomicToolFixingMixin           // Exported for direct access
}

// NewAtomicPushImageToolWithServices creates a new atomic push image tool using service container
func NewAtomicPushImageToolWithServices(adapter mcptypes.TypedPipelineOperations, serviceContainer services.ServiceContainer, logger *slog.Logger) *AtomicPushImageTool {
	toolLogger := logger.With("tool", "atomic_push_image")

	// Use focused services directly - no wrapper needed!
	return createAtomicPushImageTool(adapter, serviceContainer.SessionStore(), serviceContainer.SessionState(), toolLogger)
}

// createAtomicPushImageTool is the common creation logic
func createAtomicPushImageTool(adapter mcptypes.TypedPipelineOperations, sessionStore services.SessionStore, sessionState services.SessionState, logger *slog.Logger) *AtomicPushImageTool {
	return &AtomicPushImageTool{
		PipelineAdapter: adapter,
		SessionStore:    sessionStore,
		SessionState:    sessionState,
		Logger:          logger,
		Analyzer:        internalcommon.NewFailureAnalyzer(),
	}
}

// Note: Analyzer and FixingMixin fields are exported for direct access
// Use tool.Analyzer = analyzer and tool.FixingMixin = mixin directly

// Validate validates the tool arguments using tag-based validation
func (t *AtomicPushImageTool) Validate(_ context.Context, args interface{}) error {
	// Validate using tag-based validation
	return validation.ValidateTaggedStruct(args)
}

// ExecuteWithContext runs the atomic Docker image push with automatic fixes
func (t *AtomicPushImageTool) ExecuteWithContext(ctx *server.Context, args *AtomicPushImageArgs) (*AtomicPushImageResult, error) {
	if t.FixingMixin != nil && !args.DryRun {
		// Use fixing mixin to handle retries
		var result *AtomicPushImageResult
		startTime := time.Now()
		operation := NewDockerOperation(DockerOperationConfig{
			Type:          OperationPush,
			Name:          args.ImageRef,
			RetryAttempts: 3,
			Timeout:       10 * time.Minute,
			ExecuteFunc: func(ctx context.Context) error {
				var err error
				result, err = t.executeWithoutProgress(context.Background(), *args, nil, time.Now())
				if err != nil {
					return err
				}
				if !result.Success {
					return errors.NewError().Messagef("operation failed").Build()
				}
				return nil
			},
		}, nil) // progress reporter removed
		err := t.FixingMixin.ExecuteWithRetry(context.Background(), args.SessionID, "/workspace", operation)
		if err != nil {
			if result == nil {
				result = &AtomicPushImageResult{
					BaseToolResponse:    types.BaseToolResponse{Success: false, Timestamp: time.Now()},
					BaseAIContextResult: core.NewBaseAIContextResult("push", false, 0),
					ImageRef:            args.ImageRef,
				}
			}
			result.Success = false
			result.TotalDuration = time.Since(startTime)
			return result, err
		}
		return result, nil
	}
	return t.executeWithoutProgress(context.Background(), *args, nil, time.Now())
}

// executeWithoutProgress handles push execution without progress tracking (fallback)
func (t *AtomicPushImageTool) executeWithoutProgress(ctx context.Context, args AtomicPushImageArgs, result *AtomicPushImageResult, startTime time.Time) (*AtomicPushImageResult, error) {
	// Create result if not provided
	if result == nil {
		result = &AtomicPushImageResult{
			BaseToolResponse:    types.BaseToolResponse{Success: false, Timestamp: time.Now()},
			BaseAIContextResult: core.NewBaseAIContextResult("push", false, 0),
			ImageRef:            args.ImageRef,
			RegistryURL:         args.RegistryURL,
			PushContext:         &PushContext{},
		}
	}

	// Get session using focused service interface
	sessionData, err := t.SessionStore.Get(ctx, args.SessionID)
	if err != nil {
		t.Logger.Error("Failed to get session", "error", err, "session_id", args.SessionID)
		result.Success = false
		result.TotalDuration = time.Since(startTime)
		return result, errors.NewError().Messagef("session not found: %s", args.SessionID).WithLocation().Build()
	}

	result.SessionID = sessionData.ID
	result.WorkspaceDir = t.PipelineAdapter.GetSessionWorkspace(sessionData.ID)

	t.Logger.Info("Starting atomic Docker push", "session_id", sessionData.ID, "image_ref", args.ImageRef)

	// Handle dry-run
	if args.DryRun {
		result.RegistryURL = t.extractRegistryURL(args.ImageRef, args.RegistryURL)
		result.Success = true
		result.BaseAIContextResult = core.NewBaseAIContextResult("push", true, result.TotalDuration)
		result.PushContext.PushStatus = "dry-run"
		result.PushContext.NextStepSuggestions = []string{
			"This is a dry-run - no actual push was performed",
			"Remove dry_run flag to perform actual push",
		}
		result.TotalDuration = time.Since(startTime)
		return result, nil
	}

	// Validate prerequisites
	if err := t.validatePushPrerequisites(result, args); err != nil {
		t.Logger.Error("Push prerequisites validation failed", "error", err, "session_id", sessionData.ID, "image_ref", result.ImageRef)
		result.Success = false
		result.TotalDuration = time.Since(startTime)
		return result, err
	}

	// Convert to core.SessionState for compatibility with performPush
	coreSession := &core.SessionState{
		SessionID: sessionData.ID,
		// Add other fields as needed
	}

	// Perform the push without progress reporting
	err = t.performPush(ctx, coreSession, args, result, nil)
	result.TotalDuration = time.Since(startTime)

	// Update AI context with final result
	result.BaseAIContextResult = core.NewBaseAIContextResult("push", result.Success, result.TotalDuration)

	if err != nil {
		result.Success = false
		return result, nil
	}

	return result, nil
}

// validatePushPrerequisites checks if all prerequisites for pushing are met
func (t *AtomicPushImageTool) validatePushPrerequisites(result *AtomicPushImageResult, args AtomicPushImageArgs) error {
	// Basic input validation
	if args.ImageRef == "" {
		return errors.NewError().Messagef("image reference is required").WithLocation(

		// Validate image reference format
		).Build()
	}

	if !t.isValidImageReference(args.ImageRef) {
		return errors.NewError().Messagef("invalid image reference format").WithLocation(

		// Extract and validate registry URL
		).Build()
	}

	registryURL := t.extractRegistryURL(args.ImageRef, args.RegistryURL)
	if registryURL == "" {
		return errors.NewError().Messagef("could not extract registry URL from image reference").WithLocation().Build()
	}
	result.RegistryURL = registryURL

	return nil
}

// isValidImageReference checks if an image reference is valid
func (t *AtomicPushImageTool) isValidImageReference(imageRef string) bool {
	// Basic validation - should contain at least name
	if imageRef == "" {
		return false
	}
	// Should not contain spaces
	if strings.Contains(imageRef, " ") {
		return false
	}
	// Should contain a tag (for push operations, we typically want explicit tags)
	if !strings.Contains(imageRef, ":") {
		return false
	}
	return true
}

// extractRegistryURL extracts the registry URL from an image reference
func (t *AtomicPushImageTool) extractRegistryURL(imageRef string, registryURL string) string {
	// If registry URL is explicitly provided, use it
	if registryURL != "" {
		return registryURL
	}

	// Split by slash to get registry part
	parts := strings.Split(imageRef, "/")
	if len(parts) > 1 && strings.Contains(parts[0], ".") {
		return parts[0] // First part contains registry
	}
	return "docker.io" // Default registry
}

// ProgressCallback type for push operations
type ProgressCallback func(progress float64, message string)

// performPush contains the actual push logic that can be used with or without progress reporting
func (t *AtomicPushImageTool) performPush(ctx context.Context, session *core.SessionState, args AtomicPushImageArgs, result *AtomicPushImageResult, progressCallback interface{}) error {
	// Type assert progress callback if provided
	var progress ProgressCallback
	if progressCallback != nil {
		if pc, ok := progressCallback.(ProgressCallback); ok {
			progress = pc
		}
	}
	// Extract registry from image reference
	result.RegistryURL = t.extractRegistryURL(args.ImageRef, args.RegistryURL)

	// Push Docker image using pipeline adapter
	pushStartTime := time.Now()

	// Report progress if callback provided
	if progress != nil {
		progress(0.4, "Preparing image for push")
	}

	// Report progress
	if progress != nil {
		progress(0.5, fmt.Sprintf("Pushing image %s to %s", args.ImageRef, result.RegistryURL))
	}

	// Use the pipeline adapter to push the image
	// Convert to typed parameters for PushImageTyped
	pushParams := core.PushImageParams{
		ImageRef:   args.ImageRef,
		Registry:   result.RegistryURL,
		Repository: "", // Will be extracted from ImageRef
		Tag:        "", // Will be extracted from ImageRef
	}
	pushResult, err := t.PipelineAdapter.PushImageTyped(ctx, session.SessionID, pushParams)
	result.PushDuration = time.Since(pushStartTime)

	if err != nil {
		result.Success = false
		result.PushContext.PushStatus = "failed"
		result.PushContext.ErrorType = "push_error"
		result.PushContext.IsRetryable = true
		result.PushContext.NextStepSuggestions = []string{
			"Check registry authentication",
			"Verify image exists locally",
			"Check network connectivity to registry",
		}
		t.Logger.Error("Failed to push image", "error", err, "image_ref", args.ImageRef)
		return errors.NewError().Message("failed to push image").Cause(err).WithLocation(

		// Report progress
		).Build()
	}

	if progress != nil {
		progress(0.8, "Finalizing push operation")
	}

	// Success - extract push results
	result.Success = true
	// pushResult is already typed as *core.PushImageResult
	// Extract basic information from the typed result
	if pushResult != nil {
		// The push was successful since no error was returned
		result.PushContext.PushStatus = "successful"
	}
	result.PushContext.RegistryType = t.getRegistryType(result.RegistryURL)
	result.PushContext.RegistryEndpoint = result.RegistryURL
	result.PushContext.NextStepSuggestions = []string{
		fmt.Sprintf("Image %s successfully pushed to %s", args.ImageRef, result.RegistryURL),
		"Image is now available for deployment or sharing",
	}

	t.Logger.Info("Push operation completed successfully", "image_ref", args.ImageRef, "registry", result.RegistryURL, "push_duration", result.PushDuration)

	return nil
}

// getRegistryType determines the type of registry from the URL
func (t *AtomicPushImageTool) getRegistryType(registryURL string) string {
	if strings.Contains(registryURL, "azurecr.io") {
		return "azure_container_registry"
	} else if strings.Contains(registryURL, "amazonaws.com") {
		return "amazon_ecr"
	} else if strings.Contains(registryURL, "gcr.io") || strings.Contains(registryURL, "pkg.dev") {
		return "google_container_registry"
	} else if registryURL == "docker.io" {
		return "docker_hub"
	}
	return "private_registry"
}
