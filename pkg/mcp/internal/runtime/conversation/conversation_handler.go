package conversation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/conversation"
	"github.com/Azure/container-kit/pkg/mcp/internal/observability"
	"github.com/Azure/container-kit/pkg/mcp/internal/orchestration"
	"github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/Azure/container-kit/pkg/mcp/internal/utils"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
	"github.com/rs/zerolog"
)

// ConversationHandler is a concrete implementation for handling conversations
// without generic type parameters, simplifying the architecture.
type ConversationHandler struct {
	promptManager    *PromptManager
	sessionManager   *session.SessionManager
	toolOrchestrator orchestration.InternalToolOrchestrator
	preferenceStore  *utils.PreferenceStore
	logger           zerolog.Logger
}

// ConversationHandlerConfig holds configuration for the concrete conversation handler
type ConversationHandlerConfig struct {
	SessionManager     *session.SessionManager
	SessionAdapter     *session.SessionManager // Pre-created session adapter for tools
	PreferenceStore    *utils.PreferenceStore
	PipelineOperations mcptypes.PipelineOperations        // Using interface instead of concrete adapter
	ToolOrchestrator   *orchestration.MCPToolOrchestrator // Optional: use existing orchestrator
	Transport          interface{}                        // Accept both mcptypes.Transport and internal transport.Transport
	Logger             zerolog.Logger
	Telemetry          *observability.TelemetryManager
}

// NewConversationHandler creates a new concrete conversation handler
func NewConversationHandler(config ConversationHandlerConfig) (*ConversationHandler, error) {
	// Use provided orchestrator or create adapter
	var toolOrchestrator orchestration.InternalToolOrchestrator
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
			if session, ok := sessionInterface.(*mcptypes.SessionState); ok && session.Metadata != nil {
				if repoAnalysis, ok := session.Metadata["repo_analysis"].(map[string]interface{}); ok {
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

func (ch *ConversationHandler) attemptRetryFix(_ context.Context, sessionID string, stage types.ConversationStage, _ *orchestration.ErrorAction) bool {
	// NOTE: Actual retry logic implementation deferred to tool orchestrator integration
	ch.logger.Info().
		Str("session_id", sessionID).
		Str("stage", string(stage)).
		Msg("Attempting retry fix")
	return false // Placeholder - would implement actual retry
}

func (ch *ConversationHandler) attemptRedirectFix(_ context.Context, sessionID string, redirectTo string, workflowError *orchestration.WorkflowError) bool {
	// NOTE: Actual redirection logic implementation deferred to tool orchestrator integration
	ch.logger.Info().
		Str("session_id", sessionID).
		Str("redirect_to", redirectTo).
		Str("from_tool", workflowError.ToolName).
		Msg("Attempting redirect fix")
	return false // Placeholder - would implement actual redirection
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
