package conversation

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
	domaintypes "github.com/Azure/container-kit/pkg/mcp/domain/types"
)

// ConversationPromptService defines the interface for prompt processing
// ConversationPromptService - Use services.PromptService for the canonical interface
// This version is simplified for conversation-specific operations
// Deprecated: Use services.PromptService for new code
type ConversationPromptService interface {
	// ProcessPrompt processes a user prompt and returns a conversation response
	ProcessPrompt(ctx context.Context, sessionID, userInput string) (*ConversationResponse, error)

	// SetConversationHandler sets the conversation handler for the service
	SetConversationHandler(handler *ConversationHandler)
}

type PromptServiceImpl struct {
	sessionManager        session.SessionManager
	toolOrchestrator      api.Orchestrator
	preferenceStore       *domaintypes.PreferenceStore
	retryService          ConversationRetryService
	conversationHandler   *ConversationHandler
	smartWorkflowDetector *SmartWorkflowDetector
	logger                *slog.Logger
}

// Type alias for backward compatibility
type PromptManager = PromptServiceImpl
type PromptManagerConfig struct {
	SessionManager   session.SessionManager
	ToolOrchestrator api.Orchestrator
	PreferenceStore  *domaintypes.PreferenceStore
	Logger           *slog.Logger
}

// NewPromptService creates a new conversation prompt service
func NewPromptService(config PromptManagerConfig) ConversationPromptService {
	ps := &PromptServiceImpl{
		sessionManager:   config.SessionManager,
		toolOrchestrator: config.ToolOrchestrator,
		preferenceStore:  config.PreferenceStore,
		retryService:     NewRetryService(config.Logger, nil),
		logger:           config.Logger,
	}
	ps.smartWorkflowDetector = NewSmartWorkflowDetector(ps)

	return ps
}

// NewPromptManager creates a new prompt manager (backward compatibility)
func NewPromptManager(config PromptManagerConfig) *PromptManager {
	ps := &PromptServiceImpl{
		sessionManager:   config.SessionManager,
		toolOrchestrator: config.ToolOrchestrator,
		preferenceStore:  config.PreferenceStore,
		retryService:     NewRetryService(config.Logger, nil),
		logger:           config.Logger,
	}
	ps.smartWorkflowDetector = NewSmartWorkflowDetector(ps)

	return ps
}
func (ps *PromptServiceImpl) SetConversationHandler(handler *ConversationHandler) {
	ps.conversationHandler = handler
}
func (ps *PromptServiceImpl) newResponse(state *ConversationState) *ConversationResponse {
	return &ConversationResponse{
		SessionID: state.SessionState.SessionID,
	}
}
func (ps *PromptServiceImpl) ProcessPrompt(ctx context.Context, sessionID, userInput string) (*ConversationResponse, error) {
	convState, err := ps.initializeConversationState(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if earlyResponse := ps.handleEarlyCases(ctx, convState, userInput); earlyResponse != nil {
		return earlyResponse, nil
	}
	response := ps.processMainConversation(ctx, convState, userInput)
	return ps.finalizeConversation(convState, response, userInput)
}
func getUserIDFromContext(ctx context.Context) string {
	if userID, ok := ctx.Value("user_id").(string); ok {
		return userID
	}
	return ""
}
func (ps *PromptServiceImpl) initializeConversationState(ctx context.Context, sessionID string) (*ConversationState, error) {
	sessionInterface, err := ps.sessionManager.GetOrCreateSessionTyped(sessionID)
	if err != nil {
		return nil, errors.NewError().Message("failed to get session").Cause(err).WithLocation().Build()
	}

	internalSession := &session.SessionState{
		SessionID:    sessionInterface.SessionID,
		CreatedAt:    sessionInterface.CreatedAt,
		UpdatedAt:    sessionInterface.UpdatedAt,
		ExpiresAt:    sessionInterface.ExpiresAt,
		WorkspaceDir: sessionInterface.WorkspaceDir,
		RepoURL:      sessionInterface.RepoURL,
	}

	convState := &ConversationState{
		SessionState: internalSession,
		CurrentStage: domaintypes.StagePreFlight,
		History:      make([]ConversationTurn, 0),
		Preferences: domaintypes.UserPreferences{
			Namespace:          "default",
			Replicas:           1,
			ServiceType:        "ClusterIP",
			IncludeHealthCheck: true,
		},
		Context:   make(map[string]interface{}),
		Artifacts: make(map[string]Artifact),
	}
	ps.restoreStateFromSession(convState, internalSession)
	ps.applyUserPreferences(ctx, convState)

	return convState, nil
}
func (ps *PromptServiceImpl) restoreStateFromSession(convState *ConversationState, internalSession *session.SessionState) {
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
					turn.Stage = domaintypes.ConversationStage(stage)
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
		convState.CurrentStage = domaintypes.ConversationStage(stage)
	}
}
func (ps *PromptServiceImpl) applyUserPreferences(ctx context.Context, convState *ConversationState) {
	userID := getUserIDFromContext(ctx)
	if userID != "" && ps.preferenceStore != nil {
		if err := ps.preferenceStore.ApplyPreferencesToSession(userID, &convState.Preferences); err != nil {
			ps.logger.Warn("Failed to apply user preferences", "error", err)
		}
	}
}
func (ps *PromptServiceImpl) handleEarlyCases(ctx context.Context, convState *ConversationState, userInput string) *ConversationResponse {
	if convState.CurrentStage == domaintypes.StagePreFlight && !ps.hasPassedPreFlightChecks(convState) {
		return ps.handlePreFlightChecks(ctx, convState, userInput)
	}
	if convState.PendingDecision != nil {
		response := ps.handlePendingDecision(ctx, convState, userInput)
		turn := ConversationTurn{
			UserInput: userInput,
			Stage:     convState.CurrentStage,
			Assistant: response.Message,
		}
		convState.AddConversationTurn(turn)
		return response
	}
	if autopilotResponse := ps.handleAutopilotCommands(userInput, convState); autopilotResponse != nil {
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
func (ps *PromptServiceImpl) processMainConversation(ctx context.Context, convState *ConversationState, userInput string) *ConversationResponse {
	internalStage := mapMCPStageToDetailedStage(convState.CurrentStage, convState.Context)

	switch internalStage {
	case domaintypes.StageWelcome:
		if len(convState.History) == 0 && userInput != "" {
			return ps.smartWorkflowDetector.HandleSmartWorkflow(ctx, convState, userInput)
		} else {
			return ps.handleWelcomeStage(ctx, convState, userInput)
		}
	case domaintypes.StageInit:
		return ps.handleInitStage(ctx, convState, userInput)
	case domaintypes.StageAnalysis:
		return ps.handleAnalysisStage(ctx, convState, userInput)
	case domaintypes.StageDockerfile:
		return ps.handleDockerfileStage(ctx, convState, userInput)
	case domaintypes.StageBuild:
		return ps.handleBuildStage(ctx, convState, userInput)
	case domaintypes.StagePush:
		return ps.handlePushStage(ctx, convState, userInput)
	case domaintypes.StageManifests:
		return ps.handleManifestsStage(ctx, convState, userInput)
	case domaintypes.StageDeployment:
		return ps.handleDeploymentStage(ctx, convState, userInput)
	case domaintypes.StageCompleted:
		return ps.handleCompletedStage(ctx, convState, userInput)
	default:
		response := &ConversationResponse{
			Message: "I'm not sure what stage we're in. Let's start over. What would you like to containerize?",
			Stage:   convertFromTypesStage(domaintypes.StageInit),
			Status:  ResponseStatusError,
		}
		convState.SetStage(convertFromTypesStage(domaintypes.StageInit))
		return response
	}
}
func (ps *PromptServiceImpl) finalizeConversation(convState *ConversationState, response *ConversationResponse, userInput string) (*ConversationResponse, error) {
	turn := ConversationTurn{
		UserInput: userInput,
		Stage:     convState.CurrentStage,
		Assistant: response.Message,
	}
	if response.ToolCalls != nil {
		turn.ToolCalls = response.ToolCalls
	}
	convState.AddConversationTurn(turn)
	err := ps.sessionManager.UpdateSession(context.Background(), convState.SessionState.SessionID, func(sess *session.SessionState) error {
		if sess.Metadata == nil {
			sess.Metadata = make(map[string]interface{})
		}
		sess.Metadata["current_stage"] = string(response.Stage)
		sess.Metadata["current_status"] = string(response.Status)
		return nil
	})
	if err != nil {
		ps.logger.Warn("Failed to update session", "error", err)
	}
	response.SessionID = convState.SessionState.SessionID

	return response, nil
}
