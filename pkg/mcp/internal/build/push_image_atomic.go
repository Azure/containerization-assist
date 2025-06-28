package build

import (
	"context"
	"fmt"
	"strings"
	"time"

	coredocker "github.com/Azure/container-kit/pkg/core/docker"
	sessiontypes "github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
	publicutils "github.com/Azure/container-kit/pkg/mcp/utils"
	"github.com/localrivet/gomcp/server"
	"github.com/rs/zerolog"
)

type AtomicPushImageArgs struct {
	types.BaseToolArgs

	ImageRef    string `json:"image_ref" jsonschema:"required,pattern=^[a-zA-Z0-9][a-zA-Z0-9._/-]*:[a-zA-Z0-9][a-zA-Z0-9._-]*$" description:"Full image reference to push (e.g., myregistry.azurecr.io/myapp:latest)"`
	RegistryURL string `json:"registry_url,omitempty" jsonschema:"pattern=^[a-zA-Z0-9][a-zA-Z0-9.-]*[a-zA-Z0-9](:[0-9]+)?$" description:"Override registry URL (optional - extracted from image_ref if not provided)"`

	Timeout    int  `json:"timeout,omitempty" jsonschema:"minimum=30,maximum=3600" description:"Push timeout in seconds (default: 600)"`
	RetryCount int  `json:"retry_count,omitempty" jsonschema:"minimum=0,maximum=10" description:"Number of retry attempts (default: 3)"`
	Force      bool `json:"force,omitempty" description:"Force push even if image already exists"`
}

type AtomicPushImageResult struct {
	types.BaseToolResponse
	mcptypes.BaseAIContextResult
	Success bool `json:"success"`

	SessionID    string `json:"session_id"`
	WorkspaceDir string `json:"workspace_dir"`

	ImageRef    string `json:"image_ref"`
	RegistryURL string `json:"registry_url"`

	PushResult *coredocker.RegistryPushResult `json:"push_result"`

	PushDuration  time.Duration `json:"push_duration"`
	TotalDuration time.Duration `json:"total_duration"`

	PushContext *PushContext `json:"push_context"`
}

type PushContext struct {
	PushStatus    string  `json:"push_status"`
	LayersPushed  int     `json:"layers_pushed"`
	LayersCached  int     `json:"layers_cached"`
	PushSizeMB    float64 `json:"push_size_mb"`
	CacheHitRatio float64 `json:"cache_hit_ratio"`

	RegistryType     string `json:"registry_type"`
	RegistryEndpoint string `json:"registry_endpoint"`
	AuthMethod       string `json:"auth_method,omitempty"`

	ErrorType     string `json:"error_type,omitempty"`
	ErrorCategory string `json:"error_category,omitempty"`
	IsRetryable   bool   `json:"is_retryable"`

	NextStepSuggestions []string `json:"next_step_suggestions"`
	TroubleshootingTips []string `json:"troubleshooting_tips,omitempty"`
	AuthenticationGuide []string `json:"authentication_guide,omitempty"`
}

type AtomicPushImageTool struct {
	pipelineAdapter mcptypes.PipelineOperations
	sessionManager  mcptypes.ToolSessionManager
	logger          zerolog.Logger
	analyzer        ToolAnalyzer
	fixingMixin     *AtomicToolFixingMixin
}

func NewAtomicPushImageTool(adapter mcptypes.PipelineOperations, sessionManager mcptypes.ToolSessionManager, logger zerolog.Logger) *AtomicPushImageTool {
	return &AtomicPushImageTool{
		pipelineAdapter: adapter,
		sessionManager:  sessionManager,
		logger:          logger.With().Str("tool", "atomic_push_image").Logger(),
	}
}

func (t *AtomicPushImageTool) SetAnalyzer(analyzer ToolAnalyzer) {
	t.analyzer = analyzer
}

func (t *AtomicPushImageTool) SetFixingMixin(mixin *AtomicToolFixingMixin) {
	t.fixingMixin = mixin
}

func (t *AtomicPushImageTool) ExecuteWithFixes(ctx context.Context, args AtomicPushImageArgs) (*AtomicPushImageResult, error) {
	if t.fixingMixin != nil && !args.DryRun {
		var result *AtomicPushImageResult
		operation := NewPushOperationWrapper(
			func(ctx context.Context) error {
				var err error
				result, err = t.executePushCore(ctx, args)
				if err != nil {
					return err
				}
				if !result.Success {
					return fmt.Errorf("push operation failed")
				}
				return nil
			},
			func() error {
				if t.analyzer != nil {
					return t.analyzer.AnalyzePushFailure(args.ImageRef, args.SessionID)
				}
				return nil
			},
			func() error {
				return nil
			},
		)

		err := t.fixingMixin.ExecuteWithRetry(ctx, args.SessionID, t.pipelineAdapter.GetSessionWorkspace(args.SessionID), operation)
		if err != nil {
			return nil, err
		}
		return result, nil
	}

	return t.executePushCore(ctx, args)
}

func (t *AtomicPushImageTool) ExecutePush(ctx context.Context, args AtomicPushImageArgs) (*AtomicPushImageResult, error) {
	return t.executePushCore(ctx, args)
}

func (t *AtomicPushImageTool) executePushCore(ctx context.Context, args AtomicPushImageArgs) (*AtomicPushImageResult, error) {
	startTime := time.Now()

	result := &AtomicPushImageResult{
		BaseToolResponse:    types.NewBaseResponse("atomic_push_image", args.SessionID, args.DryRun),
		BaseAIContextResult: mcptypes.NewBaseAIContextResult("push", false, 0),
		SessionID:           args.SessionID,
		ImageRef:            args.ImageRef,
		RegistryURL:         t.extractRegistryURL(args),
		PushContext:         &PushContext{},
	}

	return t.executeWithoutProgress(ctx, args, result, startTime)
}

func (t *AtomicPushImageTool) ExecuteWithContext(serverCtx *server.Context, args AtomicPushImageArgs) (*AtomicPushImageResult, error) {
	startTime := time.Now()

	result := &AtomicPushImageResult{
		BaseToolResponse:    types.NewBaseResponse("atomic_push_image", args.SessionID, args.DryRun),
		BaseAIContextResult: mcptypes.NewBaseAIContextResult("push", false, 0),
		SessionID:           args.SessionID,
		ImageRef:            args.ImageRef,
		RegistryURL:         t.extractRegistryURL(args),
		PushContext:         &PushContext{},
	}

	ctx := context.Background()
	err := t.executeWithProgress(ctx, args, result, startTime, nil)

	result.TotalDuration = time.Since(startTime)

	if err != nil {
		t.logger.Info().Msg("Push failed")
		result.Success = false
		return result, nil
	} else {
		t.logger.Info().Msg("Push completed successfully")
	}

	return result, nil
}

func (t *AtomicPushImageTool) executeWithProgress(ctx context.Context, args AtomicPushImageArgs, result *AtomicPushImageResult, startTime time.Time, reporter interface{}) error {
	t.logger.Info().Msg("Loading session")
	sessionInterface, err := t.sessionManager.GetSession(args.SessionID)
	if err != nil {
		t.logger.Error().Err(err).Str("session_id", args.SessionID).Msg("Failed to get session")
		return types.NewRichError("SESSION_NOT_FOUND", fmt.Sprintf("session not found: %s", args.SessionID), types.ErrTypeSession)
	}
	session := sessionInterface.(*sessiontypes.SessionState)

	result.SessionID = session.SessionID
	result.WorkspaceDir = t.pipelineAdapter.GetSessionWorkspace(session.SessionID)

	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("image_ref", args.ImageRef).
		Msg("Starting atomic Docker push")

	t.logger.Info().Msg("Session initialized")

	if args.DryRun {
		result.Success = true
		result.BaseAIContextResult.IsSuccessful = true
		result.PushContext.PushStatus = "dry-run"
		result.PushContext.NextStepSuggestions = []string{
			"This is a dry-run - no actual push was performed",
			"Remove dry_run flag to perform actual push",
		}
		t.logger.Info().Msg("Dry-run completed")
		return nil
	}

	t.logger.Info().Msg("Validating prerequisites")
	if err := t.validatePushPrerequisites(result, args); err != nil {
		t.logger.Error().Err(err).
			Str("session_id", session.SessionID).
			Str("image_ref", result.ImageRef).
			Msg("Push prerequisites validation failed")
		return types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("push prerequisites validation failed: %v", err), "validation_error")
	}

	t.logger.Info().Msg("Prerequisites validated")

	t.logger.Info().Msg("Pushing Docker image")
	return t.performPush(ctx, session, args, result, reporter)
}

func (t *AtomicPushImageTool) executeWithoutProgress(ctx context.Context, args AtomicPushImageArgs, result *AtomicPushImageResult, startTime time.Time) (*AtomicPushImageResult, error) {
	sessionInterface, err := t.sessionManager.GetSession(args.SessionID)
	if err != nil {
		t.logger.Error().Err(err).Str("session_id", args.SessionID).Msg("Failed to get session")
		result.Success = false
		result.TotalDuration = time.Since(startTime)
		return result, types.NewRichError("SESSION_NOT_FOUND", fmt.Sprintf("session not found: %s", args.SessionID), types.ErrTypeSession)
	}
	session := sessionInterface.(*sessiontypes.SessionState)

	result.SessionID = session.SessionID
	result.WorkspaceDir = t.pipelineAdapter.GetSessionWorkspace(session.SessionID)

	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("image_ref", args.ImageRef).
		Msg("Starting atomic Docker push")

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

	if err := t.validatePushPrerequisites(result, args); err != nil {
		t.logger.Error().Err(err).
			Str("session_id", session.SessionID).
			Str("image_ref", result.ImageRef).
			Msg("Push prerequisites validation failed")
		result.Success = false
		result.TotalDuration = time.Since(startTime)
		return result, types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("push prerequisites validation failed: %v", err), "validation_error")
	}

	err = t.performPush(ctx, session, args, result, nil)
	result.TotalDuration = time.Since(startTime)

	if err != nil {
		result.Success = false
		return result, nil
	}

	return result, nil
}

// performPush contains the actual push logic that can be used with or without progress reporting
func (t *AtomicPushImageTool) performPush(ctx context.Context, session *sessiontypes.SessionState, args AtomicPushImageArgs, result *AtomicPushImageResult, reporter interface{}) error {
	// Report progress if reporter is available
	// Progress reporting removed

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

		// Detect error type for proper error construction
		errorType := types.ErrorCategoryUnknown
		if strings.Contains(strings.ToLower(err.Error()), "authentication") ||
			strings.Contains(strings.ToLower(err.Error()), "login") ||
			strings.Contains(strings.ToLower(err.Error()), "auth") ||
			strings.Contains(strings.ToLower(err.Error()), "denied") {
			errorType = types.ErrorCategoryAuthError
		} else if strings.Contains(strings.ToLower(err.Error()), "network") ||
			strings.Contains(strings.ToLower(err.Error()), "timeout") ||
			strings.Contains(strings.ToLower(err.Error()), "no such host") {
			errorType = types.NetworkError
		} else if strings.Contains(strings.ToLower(err.Error()), "rate limit") ||
			strings.Contains(strings.ToLower(err.Error()), "toomanyrequests") {
			errorType = types.ErrorCategoryRateLimit
		}

		result.PushResult = &coredocker.RegistryPushResult{
			Success:  false,
			ImageRef: result.ImageRef,
			Registry: result.RegistryURL,
			Error: &coredocker.RegistryError{
				Type:     errorType,
				Message:  err.Error(),
				ImageRef: result.ImageRef,
				Registry: result.RegistryURL,
				Output:   err.Error(),
			},
		}
		// Log push failure
		t.handlePushError(ctx, err, result.PushResult, result)
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

	// Progress reporting removed

	// Stage 4: Verify - Verifying push results
	// Progress reporting removed

	// Generate rich context for Claude reasoning
	t.generatePushContext(result, args)

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
		Msg("Atomic Docker push completed")

	// Progress reporting removed

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

// AI Context methods are now provided by embedded internal.BaseAIContextResult

func (r *AtomicPushImageResult) calculateConfidenceLevel() int {
	confidence := 75 // Base confidence for push operations

	if r.Success {
		confidence += 20
	} else {
		confidence -= 30
	}

	if r.PushContext != nil && r.PushContext.AuthMethod != "" {
		confidence += 10
	}

	if r.PushDuration > 15*time.Minute {
		confidence -= 10
	}

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

func (t *AtomicPushImageTool) extractRegistryURL(args AtomicPushImageArgs) string {
	if args.RegistryURL != "" {
		return args.RegistryURL
	}

	parts := strings.Split(args.ImageRef, "/")
	if len(parts) >= 2 {
		firstPart := parts[0]
		if strings.Contains(firstPart, ".") || strings.HasPrefix(firstPart, "localhost") {
			return firstPart
		}
	}

	return "docker.io"
}

func (t *AtomicPushImageTool) validatePushPrerequisites(result *AtomicPushImageResult, args AtomicPushImageArgs) error {
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

	ctx.RegistryType = t.detectRegistryType(pushResult.Registry)

	if pushResult.Context != nil {
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

	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "connection") {
		ctx.TroubleshootingTips = append(ctx.TroubleshootingTips,
			"Check network connectivity to the registry",
			"Verify registry URL is correct and accessible",
			"Consider increasing timeout if pushing large images")
	}

	if strings.Contains(errStr, "not found") || strings.Contains(errStr, "no such") {
		ctx.TroubleshootingTips = append(ctx.TroubleshootingTips,
			"Verify the image exists locally with: docker images",
			"Ensure you've built the image before pushing",
			"Check the image name and tag are correct")
	}

	if strings.Contains(errStr, "rate limit") || strings.Contains(errStr, "too many requests") {
		ctx.TroubleshootingTips = append(ctx.TroubleshootingTips,
			"Registry rate limit reached - wait before retrying",
			"Consider using authenticated requests for higher limits",
			"Spread pushes over time to avoid rate limits")
	}

	if strings.Contains(errStr, "permission") || strings.Contains(errStr, "access denied") {
		ctx.TroubleshootingTips = append(ctx.TroubleshootingTips,
			"Verify you have push permissions to this registry/repository",
			"Check registry access policies and team permissions",
			"Ensure your account has the required roles")
	}
}

func (t *AtomicPushImageTool) updateSessionState(session *sessiontypes.SessionState, result *AtomicPushImageResult) error {
	if session.Metadata == nil {
		session.Metadata = make(map[string]interface{})
	}

	if result.Success {
		session.Dockerfile.Pushed = true
	}

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

	updateFunc := func(s interface{}) {
		if sess, ok := s.(*sessiontypes.SessionState); ok {
			*sess = *session
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
	if errorType == types.ErrorCategoryAuthError {
		return false
	}

	if errorType == types.NetworkError {
		msgLower := strings.ToLower(message)
		if strings.Contains(msgLower, "no such host") {
			return false
		}
		return true
	}

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

func (t *AtomicPushImageTool) validateImageReference(imageRef string) error {
	if strings.Contains(imageRef, "//") {
		return types.NewRichError("INVALID_ARGUMENTS", fmt.Sprintf("image reference '%s' contains invalid double slashes. Format should be: [registry/]name:tag", imageRef), "invalid_format")
	}

	if strings.Contains(imageRef, "::") {
		return types.NewRichError("INVALID_ARGUMENTS", fmt.Sprintf("image reference '%s' contains invalid double colons. Format should be: [registry/]name:tag", imageRef), "invalid_format")
	}

	parts := strings.Split(imageRef, ":")
	if len(parts) > 2 {
		return types.NewRichError("INVALID_ARGUMENTS", fmt.Sprintf("image reference '%s' has too many colons. Format should be: [registry/]name:tag", imageRef), "invalid_format")
	}

	if len(parts) == 2 {
		namepart := parts[0]
		tag := parts[1]

		if tag == "" {
			return types.NewRichError("INVALID_ARGUMENTS", fmt.Sprintf("image tag cannot be empty in '%s'. Format should be: [registry/]name:tag", imageRef), "invalid_tag")
		}

		// Tag cannot contain slashes
		if strings.Contains(tag, "/") {
			return types.NewRichError("INVALID_ARGUMENTS", fmt.Sprintf("image tag '%s' cannot contain slashes in '%s'. Use registry/repository format for repository names", tag, imageRef), "invalid_tag")
		}

		// Name part validation
		if namepart == "" {
			return types.NewRichError("INVALID_ARGUMENTS", fmt.Sprintf("image name cannot be empty in '%s'. Format should be: [registry/]name:tag", imageRef), "invalid_name")
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

	if err := t.validateImageReference(pushArgs.ImageRef); err != nil {
		return types.NewValidationErrorBuilder("Invalid image reference format", "image_ref", pushArgs.ImageRef).
			WithField("error", err.Error()).
			Build()
	}

	return nil
}

func (t *AtomicPushImageTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	pushArgs, ok := args.(AtomicPushImageArgs)
	if !ok {
		return nil, types.NewValidationErrorBuilder("Invalid argument type for atomic_push_image", "args", args).
			WithField("expected", "AtomicPushImageArgs").
			WithField("received", fmt.Sprintf("%T", args)).
			Build()
	}

	return t.ExecuteTyped(ctx, pushArgs)
}

func (t *AtomicPushImageTool) GetName() string {
	return t.GetMetadata().Name
}

func (t *AtomicPushImageTool) GetDescription() string {
	return t.GetMetadata().Description
}

func (t *AtomicPushImageTool) GetVersion() string {
	return t.GetMetadata().Version
}

func (t *AtomicPushImageTool) GetCapabilities() types.ToolCapabilities {
	return types.ToolCapabilities{
		SupportsDryRun:    true,
		SupportsStreaming: true,
		IsLongRunning:     true,
		RequiresAuth:      true,
	}
}

func (t *AtomicPushImageTool) ExecuteTyped(ctx context.Context, args AtomicPushImageArgs) (*AtomicPushImageResult, error) {
	return t.ExecutePush(ctx, args)
}
