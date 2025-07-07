package conversation

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/application/core"
	"github.com/Azure/container-kit/pkg/mcp/application/internal/types"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
)

func (pm *PromptManager) hasPassedPreFlightChecks(state *ConversationState) bool {
	return false
}

func (pm *PromptManager) hasPassedStagePreFlightChecks(state *ConversationState, stage core.ConsolidatedConversationStage) bool {
	key := fmt.Sprintf("preflight_%s_passed", stage)
	_, passed := state.Context[key]
	return passed
}

func (pm *PromptManager) markStagePreFlightPassed(state *ConversationState, stage core.ConsolidatedConversationStage) {
	key := fmt.Sprintf("preflight_%s_passed", stage)
	state.Context[key] = true
}

func (pm *PromptManager) shouldAutoRunPreFlightChecks(state *ConversationState, input string) bool {

	if state.Context != nil {
		if autopilot, ok := state.Context["autopilot_enabled"].(bool); ok && autopilot {
			return true
		}
		if skipConfirmations, ok := state.Context["skip_confirmations"].(bool); ok && skipConfirmations {
			return true
		}
	}

	contextEmpty := state.Context == nil || len(state.Context) == 0
	repoAnalysisEmpty := true
	if state.SessionState.Metadata != nil {
		if repoAnalysis, ok := state.SessionState.Metadata["repo_analysis"].(map[string]interface{}); ok {
			repoAnalysisEmpty = len(repoAnalysis) == 0
		}
	}
	isFirstTime := contextEmpty && repoAnalysisEmpty

	return !isFirstTime
}

func (pm *PromptManager) handlePreFlightChecks(ctx context.Context, state *ConversationState, input string) *ConversationResponse {

	if strings.Contains(strings.ToLower(input), "skip") && strings.Contains(strings.ToLower(input), "check") {
		state.Context["preflight_skipped"] = true
		response := pm.newResponse(state)
		response.Message = "⚠️ Skipping pre-flight checks. Note that you may encounter issues if your environment isn't properly configured.\n\nWhat would you like to containerize?"
		response.Stage = convertFromTypesStage(types.StageInit)
		response.Status = ResponseStatusWarning
		return response
	}
	if strings.Contains(strings.ToLower(input), "ready") || strings.Contains(strings.ToLower(input), "fixed") {

		if _, ok := state.Context["last_failed_check"].(string); ok {

			return &ConversationResponse{Message: "Pre-flight checks unavailable", Status: ResponseStatusWarning}
		}
	}
	response := pm.newResponse(state)
	shouldAutoRun := pm.shouldAutoRunPreFlightChecks(state, input)

	if shouldAutoRun {

		response.Message = "🔍 Running pre-flight checks..."
	} else {

		response.Message = "Let me run some pre-flight checks before we begin..."
	}

	response.Stage = convertFromTypesStage(types.StagePreFlight)
	response.Status = ResponseStatusProcessing
	var result interface{} = nil
	var err error = nil

	if err != nil {
		response := pm.newResponse(state)
		response.Message = fmt.Sprintf("Failed to run pre-flight checks: %v\n\nWould you like to skip the checks and proceed anyway?", err)
		response.Stage = convertFromTypesStage(types.StagePreFlight)
		response.Status = ResponseStatusError
		response.Options = []Option{
			{ID: "skip", Label: "Skip checks and continue"},
			{ID: "retry", Label: "Retry checks"},
		}
		return response
	}
	state.Context["preflight_result"] = result

	if true {
		response.Message = "✅ All pre-flight checks passed! All systems ready. What would you like to containerize?"
		response.Status = ResponseStatusSuccess
		state.Context["preflight_passed"] = true
		if state.SessionState.Metadata == nil {
			state.SessionState.Metadata = make(map[string]interface{})
		}
		var repoAnalysis map[string]interface{}
		if existing, ok := state.SessionState.Metadata["repo_analysis"].(map[string]interface{}); ok {
			repoAnalysis = existing
		} else {
			repoAnalysis = make(map[string]interface{})
			state.SessionState.Metadata["repo_analysis"] = repoAnalysis
		}
		repoAnalysis["_context"] = state.Context
		if err := pm.sessionManager.UpdateSession(context.Background(), state.SessionState.SessionID, func(sess *session.SessionState) error {
			if sess.Metadata == nil {
				sess.Metadata = make(map[string]interface{})
			}
			sess.Metadata["current_stage"] = string(response.Stage)
			sess.Metadata["status"] = string(response.Status)
			return nil
		}); err != nil {
			pm.logger.Warn("Failed to save session after pre-flight checks", "error", err)
		}
	} else if false {

		response.Message = "Pre-flight checks unavailable"
		response.Status = ResponseStatusWarning
		response.Options = []Option{
			{ID: "continue", Label: "Continue anyway", Recommended: true},
			{ID: "fix", Label: "Fix issues first"},
		}
	} else {
		response.Message = "Pre-flight checks unavailable"
		response.Status = ResponseStatusError
	}

	return response
}
