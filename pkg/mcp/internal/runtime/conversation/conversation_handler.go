package conversation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/internal/conversation"
	"github.com/Azure/container-kit/pkg/mcp/internal/observability"
	"github.com/Azure/container-kit/pkg/mcp/internal/orchestration"
	"github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/Azure/container-kit/pkg/mcp/internal/utils"
	"github.com/rs/zerolog"
)

// ConversationHandler is a concrete implementation for handling conversations
// without generic type parameters, simplifying the architecture.
type ConversationHandler struct {
	promptManager    *PromptManager
	sessionManager   *session.SessionManager
	toolOrchestrator core.Orchestrator
	preferenceStore  *utils.PreferenceStore
	logger           zerolog.Logger
}

// ConversationHandlerConfig holds configuration for the concrete conversation handler
type ConversationHandlerConfig struct {
	SessionManager     *session.SessionManager
	SessionAdapter     *session.SessionManager // Pre-created session adapter for tools
	PreferenceStore    *utils.PreferenceStore
	PipelineOperations core.PipelineOperations            // Using interface instead of concrete adapter
	ToolOrchestrator   *orchestration.MCPToolOrchestrator // Optional: use existing orchestrator
	Transport          interface{}                        // Accept both core.Transport and internal transport.Transport
	Logger             zerolog.Logger
	Telemetry          *observability.TelemetryManager
}

// NewConversationHandler creates a new concrete conversation handler
func NewConversationHandler(config ConversationHandlerConfig) (*ConversationHandler, error) {
	// Use provided orchestrator or create adapter
	var toolOrchestrator core.Orchestrator
	if config.ToolOrchestrator != nil {
		// Use the provided canonical orchestrator directly
		toolOrchestrator = config.ToolOrchestrator
		config.Logger.Info().Msg("Using provided canonical orchestrator for conversation handler")
	} else {
		return nil, fmt.Errorf("tool orchestrator is required for conversation handler")
	}

	// Create prompt manager
	promptManager := NewPromptManager(PromptManagerConfig{
		SessionManager:   config.SessionManager,
		ToolOrchestrator: toolOrchestrator,
		PreferenceStore:  config.PreferenceStore,
		Logger:           config.Logger,
	})

	handler := &ConversationHandler{
		promptManager:    promptManager,
		sessionManager:   config.SessionManager,
		toolOrchestrator: toolOrchestrator,
		preferenceStore:  config.PreferenceStore,
		logger:           config.Logger,
	}

	// Set the conversation handler in the prompt manager for auto-fix functionality
	promptManager.SetConversationHandler(handler)

	return handler, nil
}

// HandleConversation handles a conversation turn
func (ch *ConversationHandler) HandleConversation(ctx context.Context, args conversation.ChatToolArgs) (*conversation.ChatToolResult, error) {
	if args.Message == "" {
		return nil, fmt.Errorf("message parameter is required")
	}

	// Process the conversation
	response, err := ch.promptManager.ProcessPrompt(ctx, args.SessionID, args.Message)
	if err != nil {
		return &conversation.ChatToolResult{
			Success: false,
			Message: fmt.Sprintf("Failed to process prompt: %v", err),
		}, nil
	}

	// Handle auto-advance if conditions are met
	finalResponse, err := ch.handleAutoAdvance(ctx, response)
	if err != nil {
		ch.logger.Error().Err(err).Msg("Auto-advance failed")
		// Continue with original response even if auto-advance fails
		finalResponse = response
	}

	// Convert response to ChatToolResult format
	result := &conversation.ChatToolResult{
		Success:   true,
		SessionID: finalResponse.SessionID, // Use session ID from response
		Message:   finalResponse.Message,
		Stage:     string(finalResponse.Stage),
		Status:    string(finalResponse.Status),
	}

	if len(finalResponse.Options) > 0 {
		options := make([]map[string]interface{}, len(finalResponse.Options))
		for i, opt := range finalResponse.Options {
			options[i] = map[string]interface{}{
				"id":          opt.ID,
				"label":       opt.Label,
				"description": opt.Description,
				"recommended": opt.Recommended,
			}
		}
		result.Options = options
	}

	if len(finalResponse.NextSteps) > 0 {
		result.NextSteps = finalResponse.NextSteps
	}

	if finalResponse.Progress != nil {
		result.Progress = map[string]interface{}{
			"current_stage": string(finalResponse.Progress.CurrentStage),
			"current_step":  finalResponse.Progress.CurrentStep,
			"total_steps":   finalResponse.Progress.TotalSteps,
			"percentage":    finalResponse.Progress.Percentage,
		}
	}

	return result, nil
}

// handleAutoAdvance checks if auto-advance should occur and executes it
func (ch *ConversationHandler) handleAutoAdvance(ctx context.Context, response *ConversationResponse) (*ConversationResponse, error) {
	if response == nil {
		return response, nil
	}

	// Get user preferences to check auto-advance settings
	var userPrefs types.UserPreferences = types.UserPreferences{
		SkipConfirmations: false,
	}

	// Check if autopilot is enabled in session context
	if sessionID := response.SessionID; sessionID != "" {
		sessionInterface, err := ch.sessionManager.GetSession(sessionID)
		if err == nil && sessionInterface != nil {
			// Type assert to concrete session type
			if sessionState, ok := sessionInterface.(*session.SessionState); ok {
				coreSession := sessionState.ToCoreSessionState()
				if coreSession.Metadata != nil {
					if repoAnalysis, ok := coreSession.Metadata["repo_analysis"].(map[string]interface{}); ok {
						if sessionCtx, ok := repoAnalysis["_context"].(map[string]interface{}); ok {
							if autopilotEnabled, exists := sessionCtx["autopilot_enabled"].(bool); exists && autopilotEnabled {
								// Override user preferences when autopilot is explicitly enabled
								userPrefs.SkipConfirmations = true
							}
						}
					}
				}
			}
		}
	}

	maxAdvanceSteps := 5 // Prevent infinite loops
	currentResponse := response

	for i := 0; i < maxAdvanceSteps; i++ {
		if !currentResponse.ShouldAutoAdvance(userPrefs) {
			break
		}

		ch.logger.Debug().
			Str("session_id", currentResponse.SessionID).
			Str("stage", string(currentResponse.Stage)).
			Msg("Auto-advancing conversation")

		// Execute the auto-advance action
		nextMessage := ""
		if currentResponse.AutoAdvance != nil && currentResponse.AutoAdvance.DefaultAction != "" {
			nextMessage = currentResponse.AutoAdvance.DefaultAction
		} else {
			// Default auto-advance message
			nextMessage = "continue"
		}

		// Process the next step
		nextResponse, err := ch.promptManager.ProcessPrompt(ctx, currentResponse.SessionID, nextMessage)
		if err != nil {
			ch.logger.Error().Err(err).Msg("Auto-advance processing failed")
			return currentResponse, err
		}

		// Update current response
		currentResponse = nextResponse

		// If the new response doesn't support auto-advance, break
		if !currentResponse.CanAutoAdvance() {
			break
		}
	}

	return currentResponse, nil
}

// attemptAutoFix attempts automatic error resolution before presenting manual options
func (ch *ConversationHandler) attemptAutoFix(ctx context.Context, sessionID string, stage types.ConversationStage, err error, state *ConversationState) (*AutoFixResult, error) {
	ch.logger.Info().
		Str("session_id", sessionID).
		Str("stage", string(stage)).
		Err(err).
		Msg("Attempting automatic fix before manual intervention")

	// Initialize error router if not already available
	errorRouter := orchestration.NewDefaultErrorRouter(ch.logger)

	// Extract failure analysis if available from the latest tool result
	var failureAnalysis map[string]interface{}
	if latestTurn := state.GetLatestTurn(); latestTurn != nil && len(latestTurn.ToolCalls) > 0 {
		lastToolCall := latestTurn.ToolCalls[len(latestTurn.ToolCalls)-1]
		if lastToolCall.Result != nil {
			// Check if the result contains failure analysis
			if resultMap, ok := lastToolCall.Result.(map[string]interface{}); ok {
				if fa, exists := resultMap["failure_analysis"]; exists && fa != nil {
					failureAnalysis, _ = fa.(map[string]interface{})
				}
			}
		}
	}

	// Create workflow error from the stage error
	workflowError := &orchestration.WorkflowError{
		ID:        fmt.Sprintf("%s_%d", sessionID, time.Now().Unix()),
		StageName: string(stage),
		ToolName:  ch.getToolNameForStage(stage),
		ErrorType: ch.classifyError(err),
		Message:   err.Error(),
		Severity:  ch.getErrorSeverity(err),
		Timestamp: time.Now(),
	}

	// Create workflow session context
	workflowSession := &orchestration.WorkflowSession{
		SessionID: sessionID,
		Context:   make(map[string]interface{}),
		ErrorContext: map[string]interface{}{
			"conversation_stage": string(stage),
			"state":              state,
			"failure_analysis":   failureAnalysis,
		},
	}

	// Attempt to route the error and get an action
	errorAction, err := errorRouter.RouteError(ctx, workflowError, workflowSession)
	if err != nil {
		ch.logger.Error().Err(err).Msg("Error routing failed")
		return &AutoFixResult{
			Success:         false,
			AttemptedFixes:  []string{},
			FallbackOptions: []Option{},
		}, err
	}

	result := &AutoFixResult{
		Success:        false,
		AttemptedFixes: []string{},
	}

	// Handle the error action
	switch errorAction.Action {
	case "retry":
		result.AttemptedFixes = append(result.AttemptedFixes, "Automatic retry with enhanced parameters")
		// NOTE: Actual retry logic implementation deferred to tool orchestrator integration
		success := ch.attemptRetryFix(ctx, sessionID, stage, errorAction)
		result.Success = success

	case "redirect":
		result.AttemptedFixes = append(result.AttemptedFixes, fmt.Sprintf("Cross-tool escalation to %s", errorAction.RedirectTo))
		// NOTE: Redirection logic implementation deferred to tool orchestrator integration
		success := ch.attemptRedirectFix(ctx, sessionID, errorAction.RedirectTo, workflowError)
		result.Success = success

	case "skip":
		result.AttemptedFixes = append(result.AttemptedFixes, "Automatic skip with warning")
		result.Success = true // Skip is considered successful

	case "fail":
		result.AttemptedFixes = append(result.AttemptedFixes, "Analyzed error - manual intervention required")
		result.Success = false
	}

	// Add fallback options based on the stage and error type
	result.FallbackOptions = ch.generateFallbackOptions(stage, err, errorAction)

	ch.logger.Info().
		Bool("success", result.Success).
		Strs("attempted_fixes", result.AttemptedFixes).
		Int("fallback_options", len(result.FallbackOptions)).
		Msg("Auto-fix attempt completed")

	return result, nil
}

// AutoFixResult represents the result of an automatic fix attempt
type AutoFixResult struct {
	Success         bool     `json:"success"`
	AttemptedFixes  []string `json:"attempted_fixes"`
	FallbackOptions []Option `json:"fallback_options"`
	Message         string   `json:"message"`
}

// Helper methods for auto-fix functionality

func (ch *ConversationHandler) getToolNameForStage(stage types.ConversationStage) string {
	switch stage {
	case types.StageDockerfile, types.StageBuild:
		return "build_image"
	case types.StagePush:
		return "push_image"
	case types.StageDeployment:
		return "deploy_kubernetes"
	case types.StageManifests:
		return "generate_manifests"
	default:
		return "unknown"
	}
}

func (ch *ConversationHandler) classifyError(err error) string {
	errMsg := err.Error()
	switch {
	case strings.Contains(errMsg, "build"):
		return "build_error"
	case strings.Contains(errMsg, "push"):
		return "push_error"
	case strings.Contains(errMsg, "deploy"):
		return "deployment_error"
	case strings.Contains(errMsg, "manifest"):
		return "manifest_error"
	case strings.Contains(errMsg, "dockerfile"):
		return "dockerfile_error"
	case strings.Contains(errMsg, "network"):
		return "network_error"
	case strings.Contains(errMsg, "auth") || strings.Contains(errMsg, "authentication"):
		return "authentication_error"
	case strings.Contains(errMsg, "registry"):
		return "registry_error"
	default:
		return "unknown_error"
	}
}

func (ch *ConversationHandler) getErrorSeverity(err error) string {
	errMsg := err.Error()
	switch {
	case strings.Contains(errMsg, "fatal") || strings.Contains(errMsg, "critical"):
		return "critical"
	case strings.Contains(errMsg, "error"):
		return "high"
	case strings.Contains(errMsg, "warning"):
		return "medium"
	default:
		return "high" // Default to high for unknown errors
	}
}

func (ch *ConversationHandler) attemptRetryFix(ctx context.Context, sessionID string, stage types.ConversationStage, action *orchestration.ErrorAction) bool {
	// Get the session to retrieve the last tool call parameters
	sessionInterface, err := ch.sessionManager.GetSession(sessionID)
	if err != nil {
		ch.logger.Error().Err(err).Msg("Failed to get session for retry")
		return false
	}

	// Type assert to get the concrete session type
	internalSession, ok := sessionInterface.(*session.SessionState)
	if !ok {
		ch.logger.Error().Msg("Session type assertion failed during retry")
		return false
	}

	// Create conversation state to access tool call history
	convState := &ConversationState{
		SessionState: internalSession,
		History:      make([]ConversationTurn, 0),
		Context:      make(map[string]interface{}),
	}

	// Load conversation history from session metadata
	if internalSession.Metadata != nil {
		if history, ok := internalSession.Metadata["conversation_history"].([]interface{}); ok {
			for _, turnData := range history {
				if turnMap, ok := turnData.(map[string]interface{}); ok {
					turn := ConversationTurn{}
					if toolCallsData, ok := turnMap["tool_calls"].([]interface{}); ok {
						for _, tcData := range toolCallsData {
							if tcMap, ok := tcData.(map[string]interface{}); ok {
								tc := ToolCall{
									Tool: fmt.Sprintf("%v", tcMap["tool"]),
								}
								if params, ok := tcMap["parameters"].(map[string]interface{}); ok {
									tc.Parameters = params
								}
								turn.ToolCalls = append(turn.ToolCalls, tc)
							}
						}
					}
					convState.History = append(convState.History, turn)
				}
			}
		}
	}

	// Get the last conversation turn with tool calls
	var lastToolCall *ToolCall
	if len(convState.History) > 0 {
		for i := len(convState.History) - 1; i >= 0; i-- {
			turn := convState.History[i]
			if len(turn.ToolCalls) > 0 {
				// Find the tool call that matches our stage
				toolName := ch.getToolNameForStage(stage)
				for _, tc := range turn.ToolCalls {
					if tc.Tool == toolName || strings.Contains(tc.Tool, strings.TrimSuffix(toolName, "_atomic")) {
						lastToolCall = &tc
						break
					}
				}
				if lastToolCall != nil {
					break
				}
			}
		}
	}

	// If we couldn't find the tool call in history, check metadata for common parameters
	if lastToolCall == nil {
		// Build parameters from session metadata based on the stage
		toolName := ch.getToolNameForStage(stage)
		params := make(map[string]interface{})
		params["session_id"] = sessionID

		switch stage {
		case types.StageBuild:
			if internalSession.Metadata != nil {
				if imageRef, ok := internalSession.Metadata["image_ref"].(string); ok {
					params["image_ref"] = imageRef
				}
				if imageName, ok := internalSession.Metadata["image_name"].(string); ok {
					params["image_name"] = imageName
				}
			}
		case types.StageDeployment:
			if internalSession.Metadata != nil {
				if manifestPath, ok := internalSession.Metadata["manifest_path"].(string); ok {
					params["manifest_path"] = manifestPath
				}
				if imageRef, ok := internalSession.Metadata["image_ref"].(string); ok {
					params["image_ref"] = imageRef
				}
			}
		}

		lastToolCall = &ToolCall{
			Tool:       toolName,
			Parameters: params,
		}
	}

	// Apply retry enhancements from the error action
	enhancedParams := make(map[string]interface{})
	for k, v := range lastToolCall.Parameters {
		enhancedParams[k] = v
	}

	// Apply any retry-specific parameters from the action
	if action != nil && action.Parameters != nil {
		for k, v := range action.Parameters {
			enhancedParams[k] = v
		}
	}

	// Extract failure analysis from the last tool result if available
	var failureAnalysisData map[string]interface{}
	if len(convState.History) > 0 {
		latestTurn := convState.History[len(convState.History)-1]
		if len(latestTurn.ToolCalls) > 0 {
			lastToolCall := latestTurn.ToolCalls[len(latestTurn.ToolCalls)-1]
			if lastToolCall.Result != nil {
				if resultMap, ok := lastToolCall.Result.(map[string]interface{}); ok {
					if fa, exists := resultMap["build_failure_analysis"]; exists && fa != nil {
						failureAnalysisData, _ = fa.(map[string]interface{})
					}
				}
			}
		}
	}

	if failureAnalysisData != nil {
		// Apply specific fixes based on failure type
		if failureType, ok := failureAnalysisData["failure_type"].(string); ok {
			switch failureType {
			case "network":
				// Increase timeouts for network issues
				enhancedParams["timeout"] = 300 // 5 minutes
				enhancedParams["retry_count"] = 3
			case "permissions":
				// Force root user for permission issues
				enhancedParams["force_root_user"] = true
			case "resources":
				// Enable cleanup for resource issues
				enhancedParams["force_rm"] = true
				enhancedParams["no_cache"] = true
			case "dockerfile_syntax":
				// Enable validation for dockerfile issues
				enhancedParams["validate_dockerfile"] = true
			}
		}

		// Apply suggested fixes if retry is recommended
		if retryRecommended, ok := failureAnalysisData["retry_recommended"].(bool); ok && retryRecommended {
			enhancedParams["apply_fixes"] = true
		}
	}

	// Track retry state
	if convState.RetryStates == nil {
		convState.RetryStates = make(map[string]*RetryState)
	}

	retryKey := fmt.Sprintf("%s_%s", sessionID, stage)
	retryState, exists := convState.RetryStates[retryKey]
	if !exists {
		retryState = &RetryState{
			Attempts: 0,
		}
		convState.RetryStates[retryKey] = retryState
	}

	// Check retry limit
	maxRetries := 3
	if retryState.Attempts >= maxRetries {
		ch.logger.Warn().
			Int("attempts", retryState.Attempts).
			Msg("Max retry attempts reached")
		return false
	}

	// Update retry state
	retryState.Attempts++
	retryState.LastAttempt = time.Now()

	ch.logger.Info().
		Str("session_id", sessionID).
		Str("tool", lastToolCall.Tool).
		Int("attempt", retryState.Attempts).
		Msg("Executing retry with enhanced parameters")

	// Execute the tool with retry
	result, err := ch.toolOrchestrator.ExecuteTool(ctx, lastToolCall.Tool, enhancedParams)

	if err != nil {
		retryState.LastError = err.Error()
		ch.logger.Error().
			Err(err).
			Int("attempt", retryState.Attempts).
			Msg("Retry attempt failed")
		return false
	}

	// Update session with retry success
	err = ch.sessionManager.UpdateSession(sessionID, func(s interface{}) {
		if sess, ok := s.(*session.SessionState); ok {
			if sess.Metadata == nil {
				sess.Metadata = make(map[string]interface{})
			}
			sess.Metadata["last_retry_success"] = true
			sess.Metadata["retry_result"] = result
		}
	})

	ch.logger.Info().
		Str("session_id", sessionID).
		Str("tool", lastToolCall.Tool).
		Int("attempt", retryState.Attempts).
		Msg("Retry succeeded")

	return true
}

func (ch *ConversationHandler) attemptRedirectFix(ctx context.Context, sessionID string, redirectTo string, workflowError *orchestration.WorkflowError) bool {
	// Redirect means executing a different tool to handle the error
	ch.logger.Info().
		Str("session_id", sessionID).
		Str("redirect_to", redirectTo).
		Str("from_tool", workflowError.ToolName).
		Msg("Attempting redirect fix")

	// Get the session to access metadata
	sessionInterface, err := ch.sessionManager.GetSession(sessionID)
	if err != nil {
		ch.logger.Error().Err(err).Msg("Failed to get session for redirect")
		return false
	}

	// Type assert to get the concrete session type
	internalSession, ok := sessionInterface.(*session.SessionState)
	if !ok {
		ch.logger.Error().Msg("Session type assertion failed during redirect")
		return false
	}

	// Build parameters for the redirect tool based on the error context
	redirectParams := make(map[string]interface{})
	redirectParams["session_id"] = sessionID

	// Add common parameters from session metadata
	if internalSession.Metadata != nil {
		// Common parameters that might be needed by redirect tools
		if imageRef, ok := internalSession.Metadata["image_ref"].(string); ok {
			redirectParams["image_ref"] = imageRef
		}
		if appName, ok := internalSession.Metadata["app_name"].(string); ok {
			redirectParams["app_name"] = appName
		}
		if namespace, ok := internalSession.Metadata["namespace"].(string); ok {
			redirectParams["namespace"] = namespace
		}
	}

	// Handle specific redirect scenarios
	switch redirectTo {
	case "validate_dockerfile", "validate_dockerfile_atomic":
		// Redirecting to dockerfile validation
		if dockerfilePath, ok := internalSession.Metadata["dockerfile_path"].(string); ok {
			redirectParams["dockerfile_path"] = dockerfilePath
		}
		redirectParams["generate_fixes"] = true

	case "generate_dockerfile":
		// Redirecting to dockerfile generation (regenerate)
		redirectParams["force_regenerate"] = true
		if optimization, ok := internalSession.Metadata["optimization"].(string); ok {
			redirectParams["optimization"] = optimization
		}

	case "scan_image_security", "scan_image_security_atomic":
		// Redirecting to security scan
		redirectParams["fail_on_critical"] = false // Don't fail on redirect

	case "check_health", "check_health_atomic":
		// Redirecting to health check
		redirectParams["include_logs"] = true
		redirectParams["log_lines"] = 50
	}

	// Add error context to help the redirect tool
	redirectParams["error_context"] = map[string]interface{}{
		"original_tool": workflowError.ToolName,
		"error_message": workflowError.Message,
		"error_type":    workflowError.ErrorType,
		"is_redirect":   true,
	}

	ch.logger.Info().
		Str("redirect_tool", redirectTo).
		Interface("params", redirectParams).
		Msg("Executing redirect tool")

	// Execute the redirect tool
	result, err := ch.toolOrchestrator.ExecuteTool(ctx, redirectTo, redirectParams)

	if err != nil {
		ch.logger.Error().
			Err(err).
			Str("redirect_tool", redirectTo).
			Msg("Redirect fix failed")
		return false
	}

	// Update session with redirect result
	err = ch.sessionManager.UpdateSession(sessionID, func(s interface{}) {
		if sess, ok := s.(*session.SessionState); ok {
			if sess.Metadata == nil {
				sess.Metadata = make(map[string]interface{})
			}
			sess.Metadata["last_redirect_success"] = true
			sess.Metadata["redirect_result"] = result
			sess.Metadata["redirect_from"] = workflowError.ToolName
			sess.Metadata["redirect_to"] = redirectTo
		}
	})

	ch.logger.Info().
		Str("session_id", sessionID).
		Str("redirect_to", redirectTo).
		Msg("Redirect fix succeeded")

	return true
}

func (ch *ConversationHandler) generateFallbackOptions(stage types.ConversationStage, _ error, action *orchestration.ErrorAction) []Option {
	var options []Option

	// Always provide a retry option
	options = append(options, Option{
		ID:    "retry",
		Label: "Retry operation",
	})

	// Stage-specific options
	switch stage {
	case types.StageBuild:
		options = append(options, Option{
			ID:    "logs",
			Label: "Show build logs",
		})
		options = append(options, Option{
			ID:    "modify",
			Label: "Modify Dockerfile",
		})

	case types.StageDeployment:
		options = append(options, Option{
			ID:    "manifests",
			Label: "Review manifests",
		})
		options = append(options, Option{
			ID:    "rebuild",
			Label: "Rebuild image",
		})

	case types.StageManifests:
		options = append(options, Option{
			ID:    "regenerate",
			Label: "Regenerate manifests",
		})
	}

	// Add skip option for non-critical errors
	if action != nil && action.Action != "fail" {
		options = append(options, Option{
			ID:    "skip",
			Label: "Skip this stage",
		})
	}

	return options
}
