package conversation

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/core"
	"github.com/Azure/container-kit/pkg/mcp/application/internal/types"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/processing"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
)

type PromptManager struct {
	sessionManager        *session.SessionManager
	toolOrchestrator      core.ToolOrchestrator
	preferenceStore       *processing.PreferenceStore
	retryManager          *SimpleRetryManager
	conversationHandler   *ConversationHandler
	smartWorkflowDetector *SmartWorkflowDetector
	logger                *slog.Logger
}
type PromptManagerConfig struct {
	SessionManager   *session.SessionManager
	ToolOrchestrator core.ToolOrchestrator
	PreferenceStore  *processing.PreferenceStore
	Logger           *slog.Logger
}

func NewPromptManager(config PromptManagerConfig) *PromptManager {
	pm := &PromptManager{
		sessionManager:   config.SessionManager,
		toolOrchestrator: config.ToolOrchestrator,

		preferenceStore: config.PreferenceStore,
		retryManager:    NewSimpleRetryManager(config.Logger),
		logger:          config.Logger,
	}
	pm.smartWorkflowDetector = NewSmartWorkflowDetector(pm)

	return pm
}
func (pm *PromptManager) SetConversationHandler(handler *ConversationHandler) {
	pm.conversationHandler = handler
}
func (pm *PromptManager) newResponse(state *ConversationState) *ConversationResponse {
	return &ConversationResponse{
		SessionID: state.SessionState.SessionID,
	}
}
func (pm *PromptManager) ProcessPrompt(ctx context.Context, sessionID, userInput string) (*ConversationResponse, error) {

	convState, err := pm.initializeConversationState(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if earlyResponse := pm.handleEarlyCases(ctx, convState, userInput); earlyResponse != nil {
		return earlyResponse, nil
	}
	response := pm.processMainConversation(ctx, convState, userInput)
	return pm.finalizeConversation(convState, response, userInput)
}
func getUserIDFromContext(ctx context.Context) string {
	if userID, ok := ctx.Value("user_id").(string); ok {
		return userID
	}
	return ""
}
func (pm *PromptManager) initializeConversationState(ctx context.Context, sessionID string) (*ConversationState, error) {

	sessionInterface, err := pm.sessionManager.GetOrCreateSessionTyped(sessionID)
	if err != nil {
		return nil, errors.NewError().Message("failed to get session").Cause(err).WithLocation().Build()
	}

	internalSession := &session.SessionState{
		SessionID:    sessionInterface.SessionID,
		CreatedAt:    sessionInterface.CreatedAt,
		LastAccessed: sessionInterface.UpdatedAt,
		ExpiresAt:    sessionInterface.ExpiresAt,
		WorkspaceDir: sessionInterface.WorkspaceDir,
		RepoURL:      sessionInterface.RepoURL,
	}
	if internalSession == nil {
		return nil, errors.NewError().Messagef("session type assertion failed: expected *session.SessionState, got %T", sessionInterface).WithLocation().Build()
	}

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
	pm.restoreStateFromSession(convState, internalSession)
	pm.applyUserPreferences(ctx, convState)

	return convState, nil
}
func (pm *PromptManager) restoreStateFromSession(convState *ConversationState, internalSession *session.SessionState) {
	if internalSession.Metadata == nil {
		return
	}
	if repoAnalysis, ok := internalSession.Metadata["repo_analysis"].(map[string]interface{}); ok {
		if ctx, ok := repoAnalysis["_context"].(map[string]interface{}); ok {
			convState.Context = ctx
		}
	}
	if history, ok := internalSession.Metadata["conversation_history"].([]interface{}); ok {
		for _, turnData := range history {
			if turnMap, ok := turnData.(map[string]interface{}); ok {
				turn := ConversationTurn{
					ID:        fmt.Sprintf("%v", turnMap["id"]),
					UserInput: fmt.Sprintf("%v", turnMap["user_input"]),
					Assistant: fmt.Sprintf("%v", turnMap["assistant"]),
				}
				if stage, ok := turnMap["stage"].(string); ok {
					turn.Stage = core.ConsolidatedConversationStage(stage)
				}
				if ts, ok := turnMap["timestamp"].(string); ok {
					turn.Timestamp, _ = time.Parse(time.RFC3339, ts)
				}
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
	if stage, ok := internalSession.Metadata["current_stage"].(string); ok {
		convState.CurrentStage = core.ConsolidatedConversationStage(stage)
	}
}
func (pm *PromptManager) applyUserPreferences(ctx context.Context, convState *ConversationState) {
	userID := getUserIDFromContext(ctx)
	if userID != "" && pm.preferenceStore != nil {
		if err := pm.preferenceStore.ApplyPreferencesToSession(userID, &convState.Preferences); err != nil {
			pm.logger.Warn("Failed to apply user preferences", "error", err)
		}
	}
}
func (pm *PromptManager) handleEarlyCases(ctx context.Context, convState *ConversationState, userInput string) *ConversationResponse {

	if convState.CurrentStage == core.ConversationStagePreFlight && !pm.hasPassedPreFlightChecks(convState) {
		return pm.handlePreFlightChecks(ctx, convState, userInput)
	}
	if convState.PendingDecision != nil {
		response := pm.handlePendingDecision(ctx, convState, userInput)
		turn := ConversationTurn{
			UserInput: userInput,
			Stage:     convState.CurrentStage,
			Assistant: response.Message,
		}
		convState.AddConversationTurn(turn)
		return response
	}
	if autopilotResponse := pm.handleAutopilotCommands(userInput, convState); autopilotResponse != nil {
		turn := ConversationTurn{
			UserInput: userInput,
			Stage:     convState.CurrentStage,
			Assistant: autopilotResponse.Message,
		}
		convState.AddConversationTurn(turn)
		return autopilotResponse
	}

	return nil
}
func (pm *PromptManager) processMainConversation(ctx context.Context, convState *ConversationState, userInput string) *ConversationResponse {

	internalStage := mapMCPStageToDetailedStage(convState.CurrentStage, convState.Context)

	switch internalStage {
	case types.StageWelcome:

		if len(convState.History) == 0 && userInput != "" {
			return pm.smartWorkflowDetector.HandleSmartWorkflow(ctx, convState, userInput)
		} else {
			return pm.handleWelcomeStage(ctx, convState, userInput)
		}
	case types.StageInit:
		return pm.handleInitStage(ctx, convState, userInput)
	case types.StageAnalysis:
		return pm.handleAnalysisStage(ctx, convState, userInput)
	case types.StageDockerfile:
		return pm.handleDockerfileStage(ctx, convState, userInput)
	case types.StageBuild:
		return pm.handleBuildStage(ctx, convState, userInput)
	case types.StagePush:
		return pm.handlePushStage(ctx, convState, userInput)
	case types.StageManifests:
		return pm.handleManifestsStage(ctx, convState, userInput)
	case types.StageDeployment:
		return pm.handleDeploymentStage(ctx, convState, userInput)
	case types.StageCompleted:
		return pm.handleCompletedStage(ctx, convState, userInput)
	default:
		response := &ConversationResponse{
			Message: "I'm not sure what stage we're in. Let's start over. What would you like to containerize?",
			Stage:   convertFromTypesStage(types.StageInit),
			Status:  ResponseStatusError,
		}
		convState.SetStage(convertFromTypesStage(types.StageInit))
		return response
	}
}
func (pm *PromptManager) finalizeConversation(convState *ConversationState, response *ConversationResponse, userInput string) (*ConversationResponse, error) {

	turn := ConversationTurn{
		UserInput: userInput,
		Stage:     convState.CurrentStage,
		Assistant: response.Message,
	}
	if response.ToolCalls != nil {
		turn.ToolCalls = response.ToolCalls
	}
	convState.AddConversationTurn(turn)
	err := pm.sessionManager.UpdateSession(context.Background(), convState.SessionState.SessionID, func(sess *session.SessionState) error {
		if sess.Metadata == nil {
			sess.Metadata = make(map[string]interface{})
		}
		sess.Metadata["current_stage"] = string(response.Stage)
		sess.Metadata["current_status"] = string(response.Status)
		return nil
	})
	if err != nil {
		pm.logger.Warn("Failed to update session", "error", err)
	}
	response.SessionID = convState.SessionState.SessionID

	return response, nil
}
