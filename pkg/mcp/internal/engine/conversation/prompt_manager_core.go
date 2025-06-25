package conversation

import (
	"context"
	"fmt"

	"github.com/Azure/container-copilot/pkg/mcp/internal/ops"
	"github.com/Azure/container-copilot/pkg/mcp/internal/orchestration"
	"github.com/Azure/container-copilot/pkg/mcp/internal/store/preference"
	"github.com/Azure/container-copilot/pkg/mcp/internal/store/session"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	sessiontypes "github.com/Azure/container-copilot/pkg/mcp/internal/types/session"
	"github.com/rs/zerolog"
)

// PromptManager manages conversation flow and tool orchestration
type PromptManager struct {
	sessionManager   *session.SessionManager
	toolOrchestrator orchestration.ToolOrchestrator
	preFlightChecker *ops.PreFlightChecker
	preferenceStore  *preference.PreferenceStore
	retryManager     *SimpleRetryManager
	logger           zerolog.Logger
}

// PromptManagerConfig holds configuration for the prompt manager
type PromptManagerConfig struct {
	SessionManager   *session.SessionManager
	ToolOrchestrator orchestration.ToolOrchestrator
	PreferenceStore  *preference.PreferenceStore
	Logger           zerolog.Logger
}

// NewPromptManager creates a new prompt manager
func NewPromptManager(config PromptManagerConfig) *PromptManager {
	return &PromptManager{
		sessionManager:   config.SessionManager,
		toolOrchestrator: config.ToolOrchestrator,
		preFlightChecker: ops.NewPreFlightChecker(config.Logger),
		preferenceStore:  config.PreferenceStore,
		retryManager:     NewSimpleRetryManager(config.Logger),
		logger:           config.Logger,
	}
}

// newResponse creates a new ConversationResponse with the session ID set
func (pm *PromptManager) newResponse(state *ConversationState) *ConversationResponse {
	return &ConversationResponse{
		SessionID: state.SessionID,
	}
}

// ProcessPrompt processes a user prompt and returns a response
func (pm *PromptManager) ProcessPrompt(ctx context.Context, sessionID, userInput string) (*ConversationResponse, error) {
	// Get or create conversation state
	sessionInterface, err := pm.sessionManager.GetOrCreateSession(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Type assert to concrete session type
	session, ok := sessionInterface.(*sessiontypes.SessionState)
	if !ok {
		return nil, fmt.Errorf("session type assertion failed")
	}

	// Create conversation state from session state
	convState := &ConversationState{
		SessionState: session,
		CurrentStage: types.StageInit,
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

	// Restore context from session if available
	if session.RepoAnalysis != nil {
		if ctx, ok := session.RepoAnalysis["_context"].(map[string]interface{}); ok {
			convState.Context = ctx
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
	if convState.CurrentStage == types.StageInit && !pm.hasPassedPreFlightChecks(convState) {
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

	switch convState.CurrentStage {
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
			Stage:   types.StageInit,
			Status:  ResponseStatusError,
		}
		convState.SetStage(types.StageInit)
	}

	// Add tool calls to turn if any were made
	if response.ToolCalls != nil {
		turn.ToolCalls = response.ToolCalls
	}

	// Record the turn
	turn.Assistant = response.Message
	convState.AddConversationTurn(turn)

	// Update session
	err = pm.sessionManager.UpdateSession(sessionID, func(s *sessiontypes.SessionState) {
		// Copy conversation state back
		*s = *convState.SessionState
	})
	if err != nil {
		pm.logger.Warn().Err(err).Msg("Failed to update session")
	}

	// Ensure response has the session ID
	response.SessionID = convState.SessionID

	return response, nil
}

// getUserIDFromContext extracts user ID from context
func getUserIDFromContext(ctx context.Context) string {
	if userID, ok := ctx.Value("user_id").(string); ok {
		return userID
	}
	return ""
}
