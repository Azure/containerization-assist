package conversation

import (
	"context"
	"fmt"

	"github.com/Azure/container-copilot/pkg/mcp/internal/api/contract"
	"github.com/Azure/container-copilot/pkg/mcp/internal/observability"
	"github.com/Azure/container-copilot/pkg/mcp/internal/orchestration"
	"github.com/Azure/container-copilot/pkg/mcp/internal/runtime"
	sessiontypes "github.com/Azure/container-copilot/pkg/mcp/internal/session"
	"github.com/Azure/container-copilot/pkg/mcp/internal/session/session"
	"github.com/Azure/container-copilot/pkg/mcp/internal/utils"
	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
	"github.com/rs/zerolog"
)

// ConversationHandler is a concrete implementation for handling conversations
// without generic type parameters, simplifying the architecture.
type ConversationHandler struct {
	promptManager    *PromptManager
	sessionManager   *session.SessionManager
	toolOrchestrator mcptypes.ToolOrchestrator
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
	Telemetry          *ops.TelemetryManager
}

// NewConversationHandler creates a new concrete conversation handler
func NewConversationHandler(config ConversationHandlerConfig) (*ConversationHandler, error) {
	// Use provided orchestrator or create adapter
	var toolOrchestrator mcptypes.ToolOrchestrator
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

	return &ConversationHandler{
		promptManager:    promptManager,
		sessionManager:   config.SessionManager,
		toolOrchestrator: toolOrchestrator,
		preferenceStore:  config.PreferenceStore,
		logger:           config.Logger,
	}, nil
}

// HandleConversation handles a conversation turn
func (ch *ConversationHandler) HandleConversation(ctx context.Context, args runtime.ChatToolArgs) (*runtime.ChatToolResult, error) {
	if args.Message == "" {
		return nil, fmt.Errorf("message parameter is required")
	}

	// Process the conversation
	response, err := ch.promptManager.ProcessPrompt(ctx, args.SessionID, args.Message)
	if err != nil {
		return &runtime.ChatToolResult{
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
	result := &runtime.ChatToolResult{
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
	var userPrefs contract.UserPreferences = contract.UserPreferences{
		SkipConfirmations: false,
	}

	// Check if autopilot is enabled in session context
	if sessionID := response.SessionID; sessionID != "" {
		sessionInterface, err := ch.sessionManager.GetSession(sessionID)
		if err == nil && sessionInterface != nil {
			// Type assert to concrete session type
			if session, ok := sessionInterface.(*sessiontypes.SessionState); ok && session.RepoAnalysis != nil {
				if sessionCtx, ok := session.RepoAnalysis["_context"].(map[string]interface{}); ok {
					if autopilotEnabled, exists := sessionCtx["autopilot_enabled"].(bool); exists && autopilotEnabled {
						// Override user preferences when autopilot is explicitly enabled
						userPrefs.SkipConfirmations = true
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
