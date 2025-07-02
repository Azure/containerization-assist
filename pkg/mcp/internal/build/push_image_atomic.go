package build

import (
	"context"
	"fmt"
	"strings"
	"time"

	// mcp import removed - using mcptypes

	coredocker "github.com/Azure/container-kit/pkg/core/docker"
	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/internal/common"
	"github.com/Azure/container-kit/pkg/mcp/internal/observability"

	// "github.com/Azure/container-kit/pkg/mcp/internal/runtime" // Temporarily commented to avoid import cycle
	sessiontypes "github.com/Azure/container-kit/pkg/mcp/core/session"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"

	mcptypes "github.com/Azure/container-kit/pkg/mcp/core"

	"github.com/rs/zerolog"
)

// Note: Using centralized stage definitions from core.StandardPushStages()
// AtomicPushImageArgs defines arguments for atomic Docker image push
type AtomicPushImageArgs struct {
	types.BaseToolArgs
	// Image information
	ImageRef    string `json:"image_ref" jsonschema:"required,pattern=^[a-zA-Z0-9][a-zA-Z0-9._/-]*:[a-zA-Z0-9][a-zA-Z0-9._-]*$" description:"Full image reference to push (e.g., myregistry.azurecr.io/myapp:latest)"`
	RegistryURL string `json:"registry_url,omitempty" jsonschema:"pattern=^[a-zA-Z0-9][a-zA-Z0-9.-]*[a-zA-Z0-9](:[0-9]+)?$" description:"Override registry URL (optional - extracted from image_ref if not provided)"`
	// Push configuration
	Timeout    int  `json:"timeout,omitempty" jsonschema:"minimum=30,maximum=3600" description:"Push timeout in seconds (default: 600)"`
	RetryCount int  `json:"retry_count,omitempty" jsonschema:"minimum=0,maximum=10" description:"Number of retry attempts (default: 3)"`
	Force      bool `json:"force,omitempty" description:"Force push even if image already exists"`
}

// AtomicPushImageResult defines the response from atomic Docker image push
type AtomicPushImageResult struct {
	types.BaseToolResponse
	mcptypes.BaseAIContextResult      // Embed AI context methods
	Success                      bool `json:"success"`
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
	PipelineAdapter mcptypes.PipelineOperations // Exported for direct access
	SessionManager  core.ToolSessionManager     // Exported for direct access
	Logger          zerolog.Logger              // Exported for direct access
	Analyzer        common.FailureAnalyzer      // Exported for direct access
	FixingMixin     *AtomicToolFixingMixin      // Exported for direct access
}

// NewAtomicPushImageTool creates a new atomic push image tool
func NewAtomicPushImageTool(adapter mcptypes.PipelineOperations, sessionManager core.ToolSessionManager, logger zerolog.Logger) *AtomicPushImageTool {
	return &AtomicPushImageTool{
		PipelineAdapter: adapter,
		SessionManager:  sessionManager,
		Logger:          logger.With().Str("tool", "atomic_push_image").Logger(),
	}
}

// Note: Analyzer and FixingMixin fields are exported for direct access
// Use tool.Analyzer = analyzer and tool.FixingMixin = mixin directly

// ExecuteWithFixes runs the atomic Docker image push with automatic fixes
func (t *AtomicPushImageTool) ExecuteWithFixes(ctx context.Context, args AtomicPushImageArgs) (*AtomicPushImageResult, error) {
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
				result, err = t.executeWithoutProgress(ctx, args, nil, time.Now())
				if err != nil {
					return err
				}
				if !result.Success {
					return fmt.Errorf("operation failed")
				}
				return nil
			},
		}, observability.NewUnifiedProgressReporter(nil))
		err := t.FixingMixin.ExecuteWithRetry(ctx, args.SessionID, "/workspace", operation)
		if err != nil {
			if result == nil {
				result = &AtomicPushImageResult{
					BaseToolResponse:    types.NewBaseResponse("atomic_push_image", args.SessionID, args.DryRun),
					BaseAIContextResult: mcptypes.NewBaseAIContextResult("push", false, 0),
					ImageRef:            args.ImageRef,
				}
			}
			result.Success = false
			result.TotalDuration = time.Since(startTime)
			return result, err
		}
		return result, nil
	}
	return t.executeWithoutProgress(ctx, args, nil, time.Now())
}

// executeWithoutProgress handles push execution without progress tracking (fallback)
func (t *AtomicPushImageTool) executeWithoutProgress(ctx context.Context, args AtomicPushImageArgs, result *AtomicPushImageResult, startTime time.Time) (*AtomicPushImageResult, error) {
	// Create result if not provided
	if result == nil {
		result = &AtomicPushImageResult{
			BaseToolResponse:    types.NewBaseResponse("atomic_push_image", args.SessionID, args.DryRun),
			BaseAIContextResult: mcptypes.NewBaseAIContextResult("push", false, 0),
			ImageRef:            args.ImageRef,
			RegistryURL:         args.RegistryURL,
			PushContext:         &PushContext{},
		}
	}

	// Get session
	sessionInterface, err := t.SessionManager.GetSession(args.SessionID)
	if err != nil {
		t.Logger.Error().Err(err).Str("session_id", args.SessionID).Msg("Failed to get session")
		result.Success = false
		result.TotalDuration = time.Since(startTime)
		return result, fmt.Errorf("session not found: %s", args.SessionID)
	}

	sessionState := sessionInterface.(*sessiontypes.SessionState)
	session := sessionState.ToCoreSessionState()
	result.SessionID = session.SessionID
	result.WorkspaceDir = t.PipelineAdapter.GetSessionWorkspace(session.SessionID)

	t.Logger.Info().
		Str("session_id", session.SessionID).
		Str("image_ref", args.ImageRef).
		Msg("Starting atomic Docker push")

	// Handle dry-run
	if args.DryRun {
		result.RegistryURL = t.extractRegistryURL(args.ImageRef, args.RegistryURL)
		result.Success = true
		result.BaseAIContextResult = mcptypes.NewBaseAIContextResult("push", true, result.TotalDuration)
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
		t.Logger.Error().Err(err).
			Str("session_id", session.SessionID).
			Str("image_ref", result.ImageRef).
			Msg("Push prerequisites validation failed")
		result.Success = false
		result.TotalDuration = time.Since(startTime)
		return result, err
	}

	// Perform the push without progress reporting
	err = t.performPush(ctx, session, args, result, nil)
	result.TotalDuration = time.Since(startTime)

	// Update AI context with final result
	result.BaseAIContextResult = mcptypes.NewBaseAIContextResult("push", result.Success, result.TotalDuration)

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
		return fmt.Errorf("image reference is required")
	}

	// Validate image reference format
	if !t.isValidImageReference(args.ImageRef) {
		return fmt.Errorf("invalid image reference format")
	}

	// Extract and validate registry URL
	registryURL := t.extractRegistryURL(args.ImageRef, args.RegistryURL)
	if registryURL == "" {
		return fmt.Errorf("could not extract registry URL from image reference")
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

// executePushWithCallback executes the push operation with progress callback
// ProgressCallback type temporarily defined here to avoid import cycle
type ProgressCallback func(progress float64, message string)

func (t *AtomicPushImageTool) executePushWithCallback(ctx context.Context, args AtomicPushImageArgs, result *AtomicPushImageResult, progress ProgressCallback) error {
	// Get session
	sessionInterface, err := t.SessionManager.GetSession(args.SessionID)
	if err != nil {
		return fmt.Errorf("session not found: %s", args.SessionID)
	}
	sessionState := sessionInterface.(*sessiontypes.SessionState)
	session := sessionState.ToCoreSessionState()

	// Report initial progress
	progress(0.1, "Starting image push operation")

	// Validate inputs
	progress(0.2, "Validating push parameters")
	if err := t.validatePushPrerequisites(result, args); err != nil {
		return err
	}

	// Perform the actual push
	progress(0.3, "Authenticating with registry")

	// Execute push operation
	err = t.performPush(ctx, session, args, result, progress)
	if err != nil {
		return err
	}

	// Complete
	progress(1.0, "Push operation completed successfully")
	result.Success = true

	return nil
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

	// Create push arguments
	pushArgs := map[string]interface{}{
		"imageRef":   args.ImageRef,
		"force":      args.Force,
		"timeout":    args.Timeout,
		"retryCount": args.RetryCount,
	}

	// Report progress
	if progress != nil {
		progress(0.5, fmt.Sprintf("Pushing image %s to %s", args.ImageRef, result.RegistryURL))
	}

	// Use the pipeline adapter to push the image
	pushResult, err := t.PipelineAdapter.PushImage(ctx, session.SessionID, pushArgs)
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
		t.Logger.Error().Err(err).Str("image_ref", args.ImageRef).Msg("Failed to push image")
		return fmt.Errorf("failed to push image: %w", err)
	}

	// Report progress
	if progress != nil {
		progress(0.8, "Finalizing push operation")
	}

	// Success - extract push results
	result.Success = true
	if pushResultTyped, ok := pushResult.(*coredocker.RegistryPushResult); ok {
		result.PushResult = pushResultTyped

		// Extract metrics from context if available
		if pushResultTyped.Context != nil {
			if layersPushed, ok := pushResultTyped.Context["layers_pushed"].(int); ok {
				result.PushContext.LayersPushed = layersPushed
			}
			if layersCached, ok := pushResultTyped.Context["layers_cached"].(int); ok {
				result.PushContext.LayersCached = layersCached
			}
			if bytesTransferred, ok := pushResultTyped.Context["bytes_transferred"].(int64); ok {
				result.PushContext.PushSizeMB = float64(bytesTransferred) / (1024 * 1024)
			}
		}

		// Calculate cache hit ratio if we have layer information
		total := result.PushContext.LayersPushed + result.PushContext.LayersCached
		if total > 0 {
			result.PushContext.CacheHitRatio = float64(result.PushContext.LayersCached) / float64(total)
		}

		// Use the duration from the push result
		result.PushDuration = pushResultTyped.Duration
	}

	result.PushContext.PushStatus = "successful"
	result.PushContext.RegistryType = t.getRegistryType(result.RegistryURL)
	result.PushContext.RegistryEndpoint = result.RegistryURL
	result.PushContext.NextStepSuggestions = []string{
		fmt.Sprintf("Image %s successfully pushed to %s", args.ImageRef, result.RegistryURL),
		"Image is now available for deployment or sharing",
	}

	t.Logger.Info().
		Str("image_ref", args.ImageRef).
		Str("registry", result.RegistryURL).
		Dur("push_duration", result.PushDuration).
		Msg("Push operation completed successfully")

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
