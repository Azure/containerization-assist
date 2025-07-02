package conversation

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/core/session"
	"github.com/Azure/container-kit/pkg/mcp/internal/common/utils"
	obs "github.com/Azure/container-kit/pkg/mcp/internal/observability"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
)

// PromptManager manages conversation flow and tool orchestration
type PromptManager struct {
	sessionManager      *session.SessionManager
	toolOrchestrator    core.Orchestrator
	preFlightChecker    *obs.PreFlightChecker
	preferenceStore     *utils.PreferenceStore
	retryManager        *SimpleRetryManager
	conversationHandler *ConversationHandler
	logger              zerolog.Logger
}

// PromptManagerConfig holds configuration for the prompt manager
type PromptManagerConfig struct {
	SessionManager   *session.SessionManager
	ToolOrchestrator core.Orchestrator
	PreferenceStore  *utils.PreferenceStore
	Logger           zerolog.Logger
}

// NewPromptManager creates a new prompt manager
func NewPromptManager(config PromptManagerConfig) *PromptManager {
	return &PromptManager{
		sessionManager:   config.SessionManager,
		toolOrchestrator: config.ToolOrchestrator,
		preFlightChecker: obs.NewPreFlightChecker(config.Logger),
		preferenceStore:  config.PreferenceStore,
		retryManager:     NewSimpleRetryManager(config.Logger),
		logger:           config.Logger,
	}
}

// SetConversationHandler sets the conversation handler for auto-fix functionality
func (pm *PromptManager) SetConversationHandler(handler *ConversationHandler) {
	pm.conversationHandler = handler
}

// newResponse creates a new ConversationResponse with the session ID set
func (pm *PromptManager) newResponse(state *ConversationState) *ConversationResponse {
	return &ConversationResponse{
		SessionID: state.SessionState.SessionID,
	}
}

// ProcessPrompt processes a user prompt and returns a response
func (pm *PromptManager) ProcessPrompt(ctx context.Context, sessionID, userInput string) (*ConversationResponse, error) {
	// Get or create conversation state
	sessionInterface, err := pm.sessionManager.GetOrCreateSession(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Type assert to internal session type and work with it directly
	internalSession, ok := sessionInterface.(*session.SessionState)
	if !ok {
		return nil, fmt.Errorf("session type assertion failed: expected *session.SessionState, got %T", sessionInterface)
	}

	// Create conversation state from internal session state (no conversion needed)
	convState := &ConversationState{
		SessionState: internalSession,
		CurrentStage: core.ConversationStagePreFlight,
		History:      make([]ConversationTurn, 0),
		Preferences: types.UserPreferences{
			Namespace:          "default",
			Replicas:           1,
			ServiceType:        "ClusterIP",
			IncludeHealthCheck: true,
		},
		Context:   make(map[string]interface{}),
		Artifacts: make(map[string]Artifact),
	}

	// Restore context and history from session if available
	if internalSession.Metadata != nil {
		// Restore conversation context
		if repoAnalysis, ok := internalSession.Metadata["repo_analysis"].(map[string]interface{}); ok {
			if ctx, ok := repoAnalysis["_context"].(map[string]interface{}); ok {
				convState.Context = ctx
			}
		}

		// Restore conversation history
		if history, ok := internalSession.Metadata["conversation_history"].([]interface{}); ok {
			for _, turnData := range history {
				if turnMap, ok := turnData.(map[string]interface{}); ok {
					turn := ConversationTurn{
						ID:        fmt.Sprintf("%v", turnMap["id"]),
						UserInput: fmt.Sprintf("%v", turnMap["user_input"]),
						Assistant: fmt.Sprintf("%v", turnMap["assistant"]),
					}

					// Parse stage
					if stage, ok := turnMap["stage"].(string); ok {
						turn.Stage = core.ConversationStage(stage)
					}

					// Parse timestamp
					if ts, ok := turnMap["timestamp"].(string); ok {
						turn.Timestamp, _ = time.Parse(time.RFC3339, ts)
					}

					// Parse tool calls
					if toolCallsData, ok := turnMap["tool_calls"].([]interface{}); ok {
						for _, tcData := range toolCallsData {
							if tcMap, ok := tcData.(map[string]interface{}); ok {
								tc := ToolCall{
									Tool: fmt.Sprintf("%v", tcMap["tool"]),
								}
								if params, ok := tcMap["parameters"].(map[string]interface{}); ok {
									tc.Parameters = params
								}
								if result := tcMap["result"]; result != nil {
									tc.Result = result
								}
								if duration, ok := tcMap["duration"].(float64); ok {
									tc.Duration = time.Duration(duration) * time.Millisecond
								}
								turn.ToolCalls = append(turn.ToolCalls, tc)
							}
						}
					}

					convState.History = append(convState.History, turn)
				}
			}
		}

		// Restore current stage from metadata if available
		if stage, ok := internalSession.Metadata["current_stage"].(string); ok {
			convState.CurrentStage = core.ConversationStage(stage)
		}
	}

	// Apply user preferences if available
	userID := getUserIDFromContext(ctx)
	if userID != "" && pm.preferenceStore != nil {
		if err := pm.preferenceStore.ApplyPreferencesToSession(userID, &convState.Preferences); err != nil {
			pm.logger.Warn().Err(err).Msg("Failed to apply user preferences")
		}
	}

	// Check if pre-flight checks are needed
	if convState.CurrentStage == core.ConversationStagePreFlight && !pm.hasPassedPreFlightChecks(convState) {
		response := pm.handlePreFlightChecks(ctx, convState, userInput)
		return response, nil
	}

	// Create conversation turn
	turn := ConversationTurn{
		UserInput: userInput,
		Stage:     convState.CurrentStage,
	}

	// Check for pending decisions
	if convState.PendingDecision != nil {
		response := pm.handlePendingDecision(ctx, convState, userInput)
		turn.Assistant = response.Message
		convState.AddConversationTurn(turn)
		return response, nil
	}

	// Check for autopilot control commands first
	if autopilotResponse := pm.handleAutopilotCommands(userInput, convState); autopilotResponse != nil {
		turn.Assistant = autopilotResponse.Message
		convState.AddConversationTurn(turn)
		return autopilotResponse, nil
	}

	// Route based on current stage and input
	var response *ConversationResponse

	// Convert core.ConversationStage to types.ConversationStage for internal use
	internalStage := mapMCPStageToDetailedStage(convState.CurrentStage, convState.Context)

	switch internalStage {
	case types.StageWelcome:
		response = pm.handleWelcomeStage(ctx, convState, userInput)
	case types.StageInit:
		response = pm.handleInitStage(ctx, convState, userInput)
	case types.StageAnalysis:
		response = pm.handleAnalysisStage(ctx, convState, userInput)
	case types.StageDockerfile:
		response = pm.handleDockerfileStage(ctx, convState, userInput)
	case types.StageBuild:
		response = pm.handleBuildStage(ctx, convState, userInput)
	case types.StagePush:
		response = pm.handlePushStage(ctx, convState, userInput)
	case types.StageManifests:
		response = pm.handleManifestsStage(ctx, convState, userInput)
	case types.StageDeployment:
		response = pm.handleDeploymentStage(ctx, convState, userInput)
	case types.StageCompleted:
		response = pm.handleCompletedStage(ctx, convState, userInput)
	default:
		response = &ConversationResponse{
			Message: "I'm not sure what stage we're in. Let's start over. What would you like to containerize?",
			Stage:   convertFromTypesStage(types.StageInit),
			Status:  ResponseStatusError,
		}
		convState.SetStage(convertFromTypesStage(types.StageInit))
	}

	// Add tool calls to turn if any were made
	if response.ToolCalls != nil {
		turn.ToolCalls = response.ToolCalls
	}

	// Record the turn
	turn.Assistant = response.Message
	convState.AddConversationTurn(turn)

	// Update session
	err = pm.sessionManager.UpdateSession(sessionID, func(s interface{}) {
		if sess, ok := s.(*session.SessionState); ok {
			if sess.Metadata == nil {
				sess.Metadata = make(map[string]interface{})
			}
			sess.Metadata["current_stage"] = string(response.Stage)
			sess.Metadata["current_status"] = string(response.Status)
		}
	})
	if err != nil {
		pm.logger.Warn().Err(err).Msg("Failed to update session")
	}

	// Ensure response has the session ID
	response.SessionID = convState.SessionState.SessionID

	return response, nil
}

// getUserIDFromContext extracts user ID from context
func getUserIDFromContext(ctx context.Context) string {
	if userID, ok := ctx.Value("user_id").(string); ok {
		return userID
	}
	return ""
}
