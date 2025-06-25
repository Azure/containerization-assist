package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	coredocker "github.com/Azure/container-copilot/pkg/core/docker"
	"github.com/Azure/container-copilot/pkg/mcp/internal/api/contract"
	"github.com/Azure/container-copilot/pkg/mcp/internal/interfaces"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	sessiontypes "github.com/Azure/container-copilot/pkg/mcp/internal/types/session"
	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
	publicutils "github.com/Azure/container-copilot/pkg/mcp/utils"
	"github.com/localrivet/gomcp/server"
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
	BaseAIContextResult      // Embed AI context methods
	Success             bool `json:"success"`

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
	pipelineAdapter mcptypes.PipelineOperations
	sessionManager  mcptypes.ToolSessionManager
	logger          zerolog.Logger
}

// NewAtomicPushImageTool creates a new atomic push image tool
func NewAtomicPushImageTool(adapter mcptypes.PipelineOperations, sessionManager mcptypes.ToolSessionManager, logger zerolog.Logger) *AtomicPushImageTool {
	return &AtomicPushImageTool{
		pipelineAdapter: adapter,
		sessionManager:  sessionManager,
		logger:          logger.With().Str("tool", "atomic_push_image").Logger(),
	}
}

// ExecutePush runs the atomic Docker image push
func (t *AtomicPushImageTool) ExecutePush(ctx context.Context, args AtomicPushImageArgs) (*AtomicPushImageResult, error) {
	startTime := time.Now()

	// Create result object early for error handling
	result := &AtomicPushImageResult{
		BaseToolResponse:    types.NewBaseResponse("atomic_push_image", args.SessionID, args.DryRun),
		BaseAIContextResult: NewBaseAIContextResult("push", false, 0), // Duration and success will be updated later
		SessionID:           args.SessionID,
		ImageRef:            args.ImageRef,
		RegistryURL:         t.extractRegistryURL(args),
		PushContext:         &PushContext{},
	}

	// Direct execution without progress tracking
	return t.executeWithoutProgress(ctx, args, result, startTime)
}

// ExecuteWithContext runs the atomic Docker image push with GoMCP progress tracking
func (t *AtomicPushImageTool) ExecuteWithContext(serverCtx *server.Context, args AtomicPushImageArgs) (*AtomicPushImageResult, error) {
	startTime := time.Now()

	// Create result object early for error handling
	result := &AtomicPushImageResult{
		BaseToolResponse:    types.NewBaseResponse("atomic_push_image", args.SessionID, args.DryRun),
		BaseAIContextResult: NewBaseAIContextResult("push", false, 0), // Duration will be updated later
		SessionID:           args.SessionID,
		ImageRef:            args.ImageRef,
		RegistryURL:         t.extractRegistryURL(args),
		PushContext:         &PushContext{},
	}

	// Create progress adapter for GoMCP using standard push stages
	adapter := NewGoMCPProgressAdapter(serverCtx, interfaces.StandardPushStages())

	// Execute with progress tracking
	ctx := context.Background()
	err := t.executeWithProgress(ctx, args, result, startTime, adapter)

	// Always set total duration
	result.TotalDuration = time.Since(startTime)

	// Complete progress tracking
	if err != nil {
		adapter.Complete("Push failed")
		result.Success = false
		return result, nil // Return result with error info, not the error itself
	} else {
		adapter.Complete("Push completed successfully")
	}

	return result, nil
}

// executeWithProgress handles the main execution with progress reporting
func (t *AtomicPushImageTool) executeWithProgress(ctx context.Context, args AtomicPushImageArgs, result *AtomicPushImageResult, startTime time.Time, reporter interfaces.ProgressReporter) error {
	// Stage 1: Initialize - Loading session and validating inputs
	reporter.ReportStage(0.1, "Loading session")
	sessionInterface, err := t.sessionManager.GetSession(args.SessionID)
	if err != nil {
		t.logger.Error().Err(err).Str("session_id", args.SessionID).Msg("Failed to get session")
		return types.NewRichError("INVALID_ARGUMENTS", fmt.Sprintf("failed to get session: %v", err), "session_error")
	}
	session := sessionInterface.(*sessiontypes.SessionState)

	// Set session details
	result.SessionID = session.SessionID
	result.WorkspaceDir = t.pipelineAdapter.GetSessionWorkspace(session.SessionID)

	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("image_ref", args.ImageRef).
		Msg("Starting atomic Docker push")

	reporter.ReportStage(0.8, "Session initialized")

	// Handle dry-run
	if args.DryRun {
		result.Success = true
		result.BaseAIContextResult.IsSuccessful = true
		result.PushContext.PushStatus = "dry-run"
		result.PushContext.NextStepSuggestions = []string{
			"This is a dry-run - no actual push was performed",
			"Remove dry_run flag to perform actual push",
		}
		reporter.NextStage("Dry-run completed")
		return nil
	}

	// Stage 2: Authenticate - Authenticating with registry
	reporter.NextStage("Validating prerequisites")
	if err := t.validatePushPrerequisites(result, args); err != nil {
		t.logger.Error().Err(err).
			Str("session_id", session.SessionID).
			Str("image_ref", result.ImageRef).
			Msg("Push prerequisites validation failed")
		return types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("push prerequisites validation failed: %v", err), "validation_error")
	}

	reporter.ReportStage(1.0, "Prerequisites validated")

	// Stage 3: Push - Pushing Docker image layers
	reporter.NextStage("Pushing Docker image")
	return t.performPush(ctx, session, args, result, reporter)
}

// executeWithoutProgress handles execution without progress tracking (fallback)
func (t *AtomicPushImageTool) executeWithoutProgress(ctx context.Context, args AtomicPushImageArgs, result *AtomicPushImageResult, startTime time.Time) (*AtomicPushImageResult, error) {
	// Get session
	sessionInterface, err := t.sessionManager.GetSession(args.SessionID)
	if err != nil {
		t.logger.Error().Err(err).Str("session_id", args.SessionID).Msg("Failed to get session")
		result.Success = false
		result.TotalDuration = time.Since(startTime)
		return result, types.NewRichError("INVALID_ARGUMENTS", fmt.Sprintf("failed to get session: %v", err), "session_error")
	}
	session := sessionInterface.(*sessiontypes.SessionState)

	// Set session details
	result.SessionID = session.SessionID
	result.WorkspaceDir = t.pipelineAdapter.GetSessionWorkspace(session.SessionID)

	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("image_ref", args.ImageRef).
		Msg("Starting atomic Docker push")

	// Handle dry-run
	if args.DryRun {
		result.Success = true
		result.BaseAIContextResult.IsSuccessful = true
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
		t.logger.Error().Err(err).
			Str("session_id", session.SessionID).
			Str("image_ref", result.ImageRef).
			Msg("Push prerequisites validation failed")
		result.Success = false
		result.TotalDuration = time.Since(startTime)
		return result, types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("push prerequisites validation failed: %v", err), "validation_error")
	}

	// Perform the push without progress reporting
	err = t.performPush(ctx, session, args, result, nil)
	result.TotalDuration = time.Since(startTime)

	if err != nil {
		result.Success = false
		return result, nil
	}

	return result, nil
}

// performPush contains the actual push logic that can be used with or without progress reporting
func (t *AtomicPushImageTool) performPush(ctx context.Context, session *sessiontypes.SessionState, args AtomicPushImageArgs, result *AtomicPushImageResult, reporter interfaces.ProgressReporter) error {
	// Report progress if reporter is available
	if reporter != nil {
		reporter.ReportStage(0.1, "Starting image push")
	}

	// Push Docker image using core operations
	pushStartTime := time.Now()
	// PushDockerImage only returns error, not a result
	err := t.pipelineAdapter.PushDockerImage(
		session.SessionID,
		result.ImageRef,
	)
	result.PushDuration = time.Since(pushStartTime)

	if err != nil {
		result.Success = false
		result.PushResult = &coredocker.RegistryPushResult{
			Success:  false,
			ImageRef: result.ImageRef,
			Registry: result.RegistryURL,
		}
		// Log push failure
		t.handlePushError(ctx, err, nil, result)
		return types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("push failed: %v", err), "push_error")
	}

	// Push succeeded since we didn't get an error
	result.PushResult = &coredocker.RegistryPushResult{
		Success:  true,
		ImageRef: result.ImageRef,
		Registry: result.RegistryURL,
	}
	result.Success = true
	result.BaseAIContextResult.IsSuccessful = true
	result.BaseAIContextResult.Duration = result.TotalDuration
	t.analyzePushResults(result)

	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("image_ref", result.ImageRef).
		Str("registry", result.RegistryURL).
		Dur("push_duration", result.PushDuration).
		Msg("Docker push completed successfully")

	if reporter != nil {
		reporter.ReportStage(1.0, "Push completed successfully")
	}

	// Stage 4: Verify - Verifying push results
	if reporter != nil {
		reporter.NextStage("Verifying push results")
	}

	// Generate rich context for Claude reasoning
	t.generatePushContext(result, args)

	if reporter != nil {
		reporter.ReportStage(1.0, "Verification completed")
	}

	// Stage 5: Finalize - Updating session state
	if reporter != nil {
		reporter.NextStage("Finalizing")
	}

	// Update session state
	if err := t.updateSessionState(session, result); err != nil {
		t.logger.Warn().Err(err).Msg("Failed to update session state")
	}

	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("image_ref", result.ImageRef).
		Bool("success", result.Success).
		Msg("Atomic Docker push completed")

	if reporter != nil {
		reporter.ReportStage(1.0, "Push operation completed")
	}

	return nil
}

// handlePushError creates a rich error for push failures
func (t *AtomicPushImageTool) handlePushError(ctx context.Context, err error, pushResult *coredocker.RegistryPushResult, result *AtomicPushImageResult) *types.RichError {
	var richError *types.RichError

	// Check if we have detailed error information from push result
	if pushResult != nil && pushResult.Error != nil {
		errorType := pushResult.Error.Type

		// Handle authentication errors specially
		switch errorType {
		case types.ErrorCategoryAuthError:
			richError = types.NewRichError(types.ErrCodeImagePushFailed, pushResult.Error.Message, types.ErrTypeBuild)
			richError.Context.Operation = types.OperationDockerPush
			richError.Context.Stage = "registry_authentication"
			if richError.Context.Metadata == nil {
				richError.Context.Metadata = types.NewErrorMetadata("", "image_tool", "operation")
			}
			richError.Context.Metadata.AddCustom("registry", result.RegistryURL)
			richError.Context.Metadata.AddCustom("image_ref", result.ImageRef)

			// Add authentication guidance
			if authGuidance, ok := pushResult.Error.Context["auth_guidance"].([]string); ok {
				result.PushContext.AuthenticationGuide = authGuidance
				for _, guide := range authGuidance {
					richError.Resolution.ImmediateSteps = append(richError.Resolution.ImmediateSteps,
						types.ResolutionStep{
							Order:       len(richError.Resolution.ImmediateSteps) + 1,
							Action:      guide,
							Description: "Re-authenticate with the registry",
							Expected:    "Authentication will be refreshed",
						},
					)
				}
			} else {
				// Fallback authentication guidance
				result.PushContext.AuthenticationGuide = []string{
					"Run: docker login " + result.RegistryURL,
					"Check credentials are valid",
				}
			}

			// Add troubleshooting tips for auth errors
			result.PushContext.TroubleshootingTips = append(result.PushContext.TroubleshootingTips,
				"Verify you have push permissions to this registry/repository",
				"Check registry access policies and team permissions",
				"Ensure your account has the required roles",
			)

			richError.Resolution.Prevention = append(richError.Resolution.Prevention,
				"Ensure Docker credentials are up to date before pushing",
				"Use credential helpers for automatic token refresh",
				"Set up registry authentication in CI/CD pipelines",
			)
		case types.NetworkError:
			richError = types.NewRichError(types.ErrCodeImagePushFailed, pushResult.Error.Message, types.ErrTypeBuild)
			richError.Context.Operation = types.OperationDockerPush
			richError.Context.Stage = "registry_communication"

			// Add troubleshooting tips for network errors
			if strings.Contains(strings.ToLower(pushResult.Error.Message), "no such host") {
				result.PushContext.TroubleshootingTips = append(result.PushContext.TroubleshootingTips,
					"Verify registry URL",
					"Check DNS resolution",
				)
			} else {
				result.PushContext.TroubleshootingTips = append(result.PushContext.TroubleshootingTips,
					"Check network connectivity",
					"Retry with increased timeout",
				)
			}
		case types.ErrorCategoryRateLimit:
			richError = types.NewRichError(types.ErrCodeImagePushFailed, pushResult.Error.Message, types.ErrTypeBuild)
			richError.Context.Operation = types.OperationDockerPush
			richError.Context.Stage = "rate_limiting"

			// Add troubleshooting tips for rate limit errors
			result.PushContext.TroubleshootingTips = append(result.PushContext.TroubleshootingTips,
				"Wait before retrying",
				"Consider upgrading plan",
				"Spread pushes over time to avoid rate limits",
			)
		default:
			richError = types.NewRichError(types.ErrCodeImagePushFailed, pushResult.Error.Message, types.ErrTypeBuild)
			richError.Context.Operation = types.OperationDockerPush
			richError.Context.Stage = "image_push"
		}

		// Copy error type to context
		result.PushContext.ErrorType = errorType
		result.PushContext.ErrorCategory = t.categorizeErrorType(errorType)
		result.PushContext.IsRetryable = t.isRetryableError(errorType, pushResult.Error.Message)
	} else {
		// Generic push error
		richError = types.NewRichError(types.ErrCodeImagePushFailed, fmt.Sprintf("Docker push failed: %v", err), types.ErrTypeBuild)
		richError.Context.Operation = types.OperationDockerPush
		richError.Context.Stage = "image_push"

		// Try to categorize based on error message
		if publicutils.IsAuthenticationError(err, "") {
			result.PushContext.ErrorType = types.ErrorCategoryAuthError
			result.PushContext.ErrorCategory = types.OperationAuthentication
			result.PushContext.AuthenticationGuide = publicutils.GetAuthErrorGuidance(result.RegistryURL)
		}
	}

	// Add common context
	if richError.Context.Metadata == nil {
		richError.Context.Metadata = types.NewErrorMetadata("", "image_tool", "operation")
	}
	richError.Context.Metadata.AddCustom("image_ref", result.ImageRef)
	richError.Context.Metadata.AddCustom("registry", result.RegistryURL)
	if result.PushDuration > 0 {
		richError.Context.Metadata.AddCustom("push_duration_seconds", result.PushDuration.Seconds())
	}

	// Add troubleshooting tips
	t.addTroubleshootingTips(result, err)

	return richError
}

// AI Context Interface Implementations

// AI Context methods are now provided by embedded BaseAIContextResult

func (r *AtomicPushImageResult) calculateConfidenceLevel() int {
	confidence := 75 // Base confidence for push operations

	if r.Success {
		confidence += 20
	} else {
		confidence -= 30
	}

	// Higher confidence with registry authentication
	if r.PushContext != nil && r.PushContext.AuthMethod != "" {
		confidence += 10
	}

	// Lower confidence for very slow operations (may indicate issues)
	if r.PushDuration > 15*time.Minute {
		confidence -= 10
	}

	// Ensure bounds
	if confidence > 100 {
		confidence = 100
	}
	if confidence < 0 {
		confidence = 0
	}
	return confidence
}

func (r *AtomicPushImageResult) determineOverallHealth() string {
	score := r.CalculateScore()
	if score >= 80 {
		return types.SeverityExcellent
	} else if score >= 60 {
		return types.SeverityGood
	} else if score >= 40 {
		return "fair"
	} else {
		return types.SeverityPoor
	}
}

// Helper methods

func (t *AtomicPushImageTool) extractRegistryURL(args AtomicPushImageArgs) string {
	if args.RegistryURL != "" {
		return args.RegistryURL
	}

	// Extract from image reference
	parts := strings.Split(args.ImageRef, "/")
	if len(parts) >= 2 {
		firstPart := parts[0]
		// Check if first part looks like a registry (contains dots or localhost with port)
		if strings.Contains(firstPart, ".") || strings.HasPrefix(firstPart, "localhost") {
			return firstPart
		}
	}

	return "docker.io" // Default to Docker Hub
}

func (t *AtomicPushImageTool) validatePushPrerequisites(result *AtomicPushImageResult, args AtomicPushImageArgs) error {
	// Note: Manual validation removed as jsonschema validation handles all requirements
	// jsonschema ensures:
	// - image_ref is required and matches container image pattern
	// - registry_url follows valid hostname pattern
	// - timeout is within reasonable bounds (30-3600 seconds)
	// - retry_count is within safe limits (0-10)

	// Basic image reference validation for user experience
	if !strings.Contains(args.ImageRef, ":") {
		result.PushContext.TroubleshootingTips = append(
			result.PushContext.TroubleshootingTips,
			"Image reference should include a tag (e.g., myapp:latest)",
		)
	}

	return nil
}

func (t *AtomicPushImageTool) analyzePushResults(result *AtomicPushImageResult) {
	ctx := result.PushContext
	pushResult := result.PushResult

	if pushResult == nil {
		return
	}

	ctx.PushStatus = "successful"
	ctx.RegistryEndpoint = pushResult.Registry

	// Analyze registry type
	ctx.RegistryType = t.detectRegistryType(pushResult.Registry)

	// Extract context information if available
	if pushResult.Context != nil {
		// Try to extract layer information from context
		if layers, ok := pushResult.Context["layers_pushed"].(int); ok {
			ctx.LayersPushed = layers
		}
		if cached, ok := pushResult.Context["layers_cached"].(int); ok {
			ctx.LayersCached = cached
		}
		if ratio, ok := pushResult.Context["cache_ratio"].(float64); ok {
			ctx.CacheHitRatio = ratio
		}
		if size, ok := pushResult.Context["size_bytes"].(int64); ok {
			ctx.PushSizeMB = float64(size) / (1024 * 1024)
		}
	}
}

func (t *AtomicPushImageTool) generatePushContext(result *AtomicPushImageResult, args AtomicPushImageArgs) {
	ctx := result.PushContext

	// Generate next step suggestions
	if result.Success {
		ctx.NextStepSuggestions = append(ctx.NextStepSuggestions,
			fmt.Sprintf("Image successfully pushed to %s", result.RegistryURL))
		ctx.NextStepSuggestions = append(ctx.NextStepSuggestions,
			"You can now use this image reference in Kubernetes deployments")
		ctx.NextStepSuggestions = append(ctx.NextStepSuggestions,
			fmt.Sprintf("Image reference: %s", result.ImageRef))

		if ctx.CacheHitRatio > 0.5 {
			ctx.NextStepSuggestions = append(ctx.NextStepSuggestions,
				fmt.Sprintf("Good cache efficiency: %.1f%% layers were already in registry", ctx.CacheHitRatio*100))
		}
	} else {
		ctx.NextStepSuggestions = append(ctx.NextStepSuggestions,
			"Push failed - review error details and troubleshooting tips")

		if ctx.IsRetryable {
			ctx.NextStepSuggestions = append(ctx.NextStepSuggestions,
				"This error appears to be temporary - consider retrying")
		}
	}
}

func (t *AtomicPushImageTool) addTroubleshootingTips(result *AtomicPushImageResult, err error) {
	ctx := result.PushContext

	if err == nil {
		return
	}

	errStr := strings.ToLower(err.Error())

	// Network-related issues
	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "connection") {
		ctx.TroubleshootingTips = append(ctx.TroubleshootingTips,
			"Check network connectivity to the registry",
			"Verify registry URL is correct and accessible",
			"Consider increasing timeout if pushing large images")
	}

	// Image not found
	if strings.Contains(errStr, "not found") || strings.Contains(errStr, "no such") {
		ctx.TroubleshootingTips = append(ctx.TroubleshootingTips,
			"Verify the image exists locally with: docker images",
			"Ensure you've built the image before pushing",
			"Check the image name and tag are correct")
	}

	// Rate limiting
	if strings.Contains(errStr, "rate limit") || strings.Contains(errStr, "too many requests") {
		ctx.TroubleshootingTips = append(ctx.TroubleshootingTips,
			"Registry rate limit reached - wait before retrying",
			"Consider using authenticated requests for higher limits",
			"Spread pushes over time to avoid rate limits")
	}

	// Permission issues
	if strings.Contains(errStr, "permission") || strings.Contains(errStr, "access denied") {
		ctx.TroubleshootingTips = append(ctx.TroubleshootingTips,
			"Verify you have push permissions to this registry/repository",
			"Check registry access policies and team permissions",
			"Ensure your account has the required roles")
	}
}

func (t *AtomicPushImageTool) updateSessionState(session *sessiontypes.SessionState, result *AtomicPushImageResult) error {
	// Update session with push results
	if session.Metadata == nil {
		session.Metadata = make(map[string]interface{})
	}

	// Update session state fields - use modern field and maintain legacy compatibility
	if result.Success {
		session.Dockerfile.Pushed = true
		// session.ImageRef is types.ImageReference, not string
		// Store in metadata instead
	}

	// Update metadata for backward compatibility and additional details
	session.Metadata["last_pushed_image"] = result.ImageRef
	session.Metadata["last_push_registry"] = result.RegistryURL
	session.Metadata["last_push_success"] = result.Success
	session.Metadata["pushed_image_ref"] = result.ImageRef
	session.Metadata["registry_url"] = result.RegistryURL
	session.Metadata["push_success"] = result.Success

	if result.Success && result.PushResult != nil {
		session.Metadata["push_duration_seconds"] = result.PushDuration.Seconds()
		session.Metadata["push_duration"] = result.PushDuration.Seconds()
		if result.PushContext.CacheHitRatio > 0 {
			session.Metadata["push_cache_ratio"] = result.PushContext.CacheHitRatio
		}
	}

	session.UpdateLastAccessed()

	// UpdateSession expects interface{} for updateFunc
	updateFunc := func(sessionInterface interface{}) {
		if s, ok := sessionInterface.(*sessiontypes.SessionState); ok {
			*s = *session
		}
	}
	return t.sessionManager.UpdateSession(session.SessionID, updateFunc)
}

func (t *AtomicPushImageTool) detectRegistryType(registry string) string {
	switch {
	case strings.Contains(registry, "azurecr.io"):
		return "Azure Container Registry"
	case strings.Contains(registry, "gcr.io") || strings.Contains(registry, "pkg.dev"):
		return "Google Container Registry"
	case strings.Contains(registry, "amazonaws.com"):
		return "Amazon ECR"
	case registry == "docker.io" || strings.Contains(registry, "docker.com"):
		return "Docker Hub"
	case strings.Contains(registry, "quay.io"):
		return "Quay.io"
	case strings.Contains(registry, "localhost") || strings.Contains(registry, "127.0.0.1"):
		return "Local Registry"
	default:
		return "Private Registry"
	}
}

func (t *AtomicPushImageTool) categorizeErrorType(errorType string) string {
	switch errorType {
	case types.ErrorCategoryAuthError:
		return types.OperationAuthentication
	case types.NetworkError:
		return "connectivity"
	case "not_found":
		return "missing_resource"
	case "push_error":
		return "operation_failed"
	case types.ErrorCategoryRateLimit:
		return types.ErrorCategoryRateLimit
	default:
		return types.ErrorCategoryUnknown
	}
}

func (t *AtomicPushImageTool) isRetryableError(errorType, message string) bool {
	// Authentication errors are not retryable without fixing credentials
	if errorType == types.ErrorCategoryAuthError {
		return false
	}

	// Network errors are usually retryable, except for DNS resolution failures
	if errorType == types.NetworkError {
		msgLower := strings.ToLower(message)
		if strings.Contains(msgLower, "no such host") {
			return false
		}
		return true
	}

	// Check message for temporary conditions
	msgLower := strings.ToLower(message)
	temporaryIndicators := []string{
		"timeout",
		"temporary",
		"rate limit",
		"too many requests",
		"connection reset",
		"connection refused",
		"502",
		"503",
		"504",
	}

	for _, indicator := range temporaryIndicators {
		if strings.Contains(msgLower, indicator) {
			return true
		}
	}

	return false
}

// validateImageReference validates the format of a Docker image reference
func (t *AtomicPushImageTool) validateImageReference(imageRef string) error {
	// Check for obviously invalid characters
	if strings.Contains(imageRef, "//") {
		return types.NewRichError("INVALID_ARGUMENTS", "image reference contains invalid double slashes", "invalid_format")
	}

	// Check for multiple consecutive colons
	if strings.Contains(imageRef, "::") {
		return types.NewRichError("INVALID_ARGUMENTS", "image reference contains invalid double colons", "invalid_format")
	}

	// Basic format validation - should be [registry/]name:tag
	// Split by colon to separate name and tag
	parts := strings.Split(imageRef, ":")
	if len(parts) > 2 {
		return types.NewRichError("INVALID_ARGUMENTS", "image reference has too many colons", "invalid_format")
	}

	if len(parts) == 2 {
		namepart := parts[0]
		tag := parts[1]

		// Tag cannot be empty
		if tag == "" {
			return types.NewRichError("INVALID_ARGUMENTS", "image tag cannot be empty", "invalid_tag")
		}

		// Tag cannot contain slashes
		if strings.Contains(tag, "/") {
			return types.NewRichError("INVALID_ARGUMENTS", "image tag cannot contain slashes", "invalid_tag")
		}

		// Name part validation
		if namepart == "" {
			return types.NewRichError("INVALID_ARGUMENTS", "image name cannot be empty", "invalid_name")
		}
	}

	return nil
}

// Tool interface implementation (unified interface)

// GetMetadata returns comprehensive tool metadata
func (t *AtomicPushImageTool) GetMetadata() mcptypes.ToolMetadata {
	return mcptypes.ToolMetadata{
		Name:         "atomic_push_image",
		Description:  "Pushes Docker images to container registries with authentication support, retry logic, and progress tracking",
		Version:      "1.0.0",
		Category:     "docker",
		Dependencies: []string{"docker"},
		Capabilities: []string{
			"supports_dry_run",
			"supports_streaming",
		},
		Requirements: []string{"docker_daemon", "registry_access"},
		Parameters: map[string]string{
			"image_ref":    "required - Full image reference to push",
			"registry_url": "optional - Override registry URL",
			"timeout":      "optional - Push timeout in seconds",
			"retry_count":  "optional - Number of retry attempts",
			"force":        "optional - Force push even if image exists",
		},
		Examples: []mcptypes.ToolExample{
			{
				Name:        "basic_push",
				Description: "Push a Docker image to registry",
				Input: map[string]interface{}{
					"session_id": "session-123",
					"image_ref":  "myregistry.azurecr.io/myapp:v1.0.0",
				},
				Output: map[string]interface{}{
					"success":       true,
					"image_ref":     "myregistry.azurecr.io/myapp:v1.0.0",
					"push_duration": "45s",
				},
			},
		},
	}
}

// Validate validates the tool arguments (unified interface)
func (t *AtomicPushImageTool) Validate(ctx context.Context, args interface{}) error {
	pushArgs, ok := args.(AtomicPushImageArgs)
	if !ok {
		return types.NewValidationErrorBuilder("Invalid argument type for atomic_push_image", "args", args).
			WithField("expected", "AtomicPushImageArgs").
			WithField("received", fmt.Sprintf("%T", args)).
			Build()
	}

	if pushArgs.ImageRef == "" {
		return types.NewValidationErrorBuilder("ImageRef is required", "image_ref", pushArgs.ImageRef).
			WithField("field", "image_ref").
			Build()
	}

	if pushArgs.SessionID == "" {
		return types.NewValidationErrorBuilder("SessionID is required", "session_id", pushArgs.SessionID).
			WithField("field", "session_id").
			Build()
	}

	// Validate image reference format
	if err := t.validateImageReference(pushArgs.ImageRef); err != nil {
		return types.NewValidationErrorBuilder("Invalid image reference format", "image_ref", pushArgs.ImageRef).
			WithField("error", err.Error()).
			Build()
	}

	return nil
}

// Execute implements unified Tool interface
func (t *AtomicPushImageTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	pushArgs, ok := args.(AtomicPushImageArgs)
	if !ok {
		return nil, types.NewValidationErrorBuilder("Invalid argument type for atomic_push_image", "args", args).
			WithField("expected", "AtomicPushImageArgs").
			WithField("received", fmt.Sprintf("%T", args)).
			Build()
	}

	// Call the typed Execute method
	return t.ExecuteTyped(ctx, pushArgs)
}

// Legacy interface methods for backward compatibility

// GetName returns the tool name (legacy SimpleTool compatibility)
func (t *AtomicPushImageTool) GetName() string {
	return t.GetMetadata().Name
}

// GetDescription returns the tool description (legacy SimpleTool compatibility)
func (t *AtomicPushImageTool) GetDescription() string {
	return t.GetMetadata().Description
}

// GetVersion returns the tool version (legacy SimpleTool compatibility)
func (t *AtomicPushImageTool) GetVersion() string {
	return t.GetMetadata().Version
}

// GetCapabilities returns the tool capabilities (legacy SimpleTool compatibility)
func (t *AtomicPushImageTool) GetCapabilities() contract.ToolCapabilities {
	return contract.ToolCapabilities{
		SupportsDryRun:    true,
		SupportsStreaming: true,
		IsLongRunning:     true,
		RequiresAuth:      true,
	}
}

// ExecuteTyped provides the original typed execute method
func (t *AtomicPushImageTool) ExecuteTyped(ctx context.Context, args AtomicPushImageArgs) (*AtomicPushImageResult, error) {
	return t.ExecutePush(ctx, args)
}
