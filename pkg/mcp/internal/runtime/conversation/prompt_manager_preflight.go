package conversation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/observability"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
)

// Pre-flight check methods

func (pm *PromptManager) hasPassedPreFlightChecks(state *ConversationState) bool {
	// Check if pre-flight checks have been run and passed
	if result, ok := state.Context["preflight_result"].(*observability.PreFlightResult); ok {
		// Checks are valid for 1 hour
		if time.Since(result.Timestamp) < 1*time.Hour {
			return result.CanProceed
		}
	}
	return false
}

func (pm *PromptManager) hasPassedStagePreFlightChecks(state *ConversationState, stage types.ConversationStage) bool {
	key := fmt.Sprintf("preflight_%s_passed", stage)
	_, passed := state.Context[key]
	return passed
}

func (pm *PromptManager) markStagePreFlightPassed(state *ConversationState, stage types.ConversationStage) {
	key := fmt.Sprintf("preflight_%s_passed", stage)
	state.Context[key] = true
}

func (pm *PromptManager) shouldAutoRunPreFlightChecks(state *ConversationState, input string) bool {
	// Always auto-run if autopilot mode is enabled
	if state.Context != nil {
		if autopilot, ok := state.Context["autopilot_enabled"].(bool); ok && autopilot {
			return true
		}

		// Always auto-run if skip_confirmations is enabled
		if skipConfirmations, ok := state.Context["skip_confirmations"].(bool); ok && skipConfirmations {
			return true
		}
	}

	// Auto-run by default unless this is the very first interaction
	// (indicated by empty/nil context and empty repo analysis)
	contextEmpty := state.Context == nil || len(state.Context) == 0
	repoAnalysisEmpty := state.RepoAnalysis == nil || len(state.RepoAnalysis) == 0
	isFirstTime := contextEmpty && repoAnalysisEmpty

	// For first-time users, require more explicit confirmation
	// But for returning users, auto-run for smoother experience
	return !isFirstTime
}

func (pm *PromptManager) handleFailedPreFlightChecks(ctx context.Context, state *ConversationState, result *observability.PreFlightResult, stage types.ConversationStage) *ConversationResponse {
	var failedChecks []string
	var suggestions []string

	for _, check := range result.Checks {
		if check.Status == observability.CheckStatusFail {
			failedChecks = append(failedChecks, fmt.Sprintf("❌ %s: %s", check.Name, check.Error))
			if check.RecoveryAction != "" {
				suggestions = append(suggestions, fmt.Sprintf("• %s", check.RecoveryAction))
			}
		}
	}

	message := fmt.Sprintf(
		"Pre-flight checks failed for %s stage:\n\n%s\n\nSuggested actions:\n%s\n\nWould you like to retry after fixing these issues?",
		stage,
		strings.Join(failedChecks, "\n"),
		strings.Join(suggestions, "\n"),
	)

	return &ConversationResponse{
		Message: message,
		Stage:   stage,
		Status:  ResponseStatusError,
		Options: []Option{
			{ID: "retry", Label: "Retry checks", Recommended: true},
			{ID: "skip", Label: "Skip this stage"},
			{ID: "abort", Label: "Cancel workflow"},
		},
	}
}

func (pm *PromptManager) handlePreFlightChecks(ctx context.Context, state *ConversationState, input string) *ConversationResponse {
	// Check if user wants to skip pre-flight checks
	if strings.Contains(strings.ToLower(input), "skip") && strings.Contains(strings.ToLower(input), "check") {
		state.Context["preflight_skipped"] = true
		response := pm.newResponse(state)
		response.Message = "⚠️ Skipping pre-flight checks. Note that you may encounter issues if your environment isn't properly configured.\n\nWhat would you like to containerize?"
		response.Stage = types.StageInit
		response.Status = ResponseStatusWarning
		return response
	}

	// Check if this is a retry after fixing an issue
	if strings.Contains(strings.ToLower(input), "ready") || strings.Contains(strings.ToLower(input), "fixed") {
		// Re-run the failed check
		if lastFailed, ok := state.Context["last_failed_check"].(string); ok {
			return pm.rerunSingleCheck(ctx, state, lastFailed)
		}
	}

	// Auto-run pre-flight checks unless user explicitly opted out
	response := pm.newResponse(state)

	// Check if we should skip confirmation prompt
	shouldAutoRun := pm.shouldAutoRunPreFlightChecks(state, input)

	if shouldAutoRun {
		// Auto-run without confirmation
		response.Message = "🔍 Running pre-flight checks..."
	} else {
		// Show traditional confirmation prompt for first-time users
		response.Message = "Let me run some pre-flight checks before we begin..."
	}

	response.Stage = types.StagePreFlight
	response.Status = ResponseStatusProcessing

	result, err := pm.preFlightChecker.RunChecks(ctx)
	if err != nil {
		response := pm.newResponse(state)
		response.Message = fmt.Sprintf("Failed to run pre-flight checks: %v\n\nWould you like to skip the checks and proceed anyway?", err)
		response.Stage = types.StagePreFlight
		response.Status = ResponseStatusError
		response.Options = []Option{
			{ID: "skip", Label: "Skip checks and continue"},
			{ID: "retry", Label: "Retry checks"},
		}
		return response
	}

	// Store result
	state.Context["preflight_result"] = result

	// Format results
	if result.Passed {
		response.Message = "✅ All pre-flight checks passed! All systems ready. What would you like to containerize?"
		response.Status = ResponseStatusSuccess
		state.Context["preflight_passed"] = true

		// Save context to session state
		if state.RepoAnalysis == nil {
			state.RepoAnalysis = make(map[string]interface{})
		}
		state.RepoAnalysis["_context"] = state.Context

		// Save session to persist the context
		if err := pm.sessionManager.UpdateSession(state.SessionID, func(s interface{}) {
			if sess, ok := s.(*mcptypes.SessionState); ok {
				sess.CurrentStage = string(response.Stage)
				sess.Status = string(response.Status)
			}
		}); err != nil {
			pm.logger.Warn().Err(err).Msg("Failed to save session after pre-flight checks")
		}
	} else if result.CanProceed {
		response.Message = pm.formatPreFlightWarnings(result)
		response.Status = ResponseStatusWarning
		response.Options = []Option{
			{ID: "continue", Label: "Continue anyway", Recommended: true},
			{ID: "fix", Label: "Fix issues first"},
		}
	} else {
		// Critical failures
		response.Message = pm.formatPreFlightErrors(result)
		response.Status = ResponseStatusError

		// Find first critical failure for recovery
		for _, check := range result.Checks {
			if check.Status == observability.CheckStatusFail && check.Category != "optional" {
				state.Context["last_failed_check"] = check.Name
				response.Options = pm.getRecoveryOptions(check)
				break
			}
		}
	}

	return response
}

func (pm *PromptManager) rerunSingleCheck(ctx context.Context, state *ConversationState, checkName string) *ConversationResponse {
	result, err := pm.preFlightChecker.RunSingleCheck(ctx, checkName)
	if err != nil {
		return &ConversationResponse{
			Message: fmt.Sprintf("Failed to run check: %v", err),
			Stage:   types.StageInit,
			Status:  ResponseStatusError,
		}
	}

	if result.Status == observability.CheckStatusPass {
		// Check passed, run all checks again
		return pm.handlePreFlightChecks(ctx, state, "")
	}

	// Still failing
	return &ConversationResponse{
		Message: fmt.Sprintf("❌ %s check still failing: %s\n\n%s", result.Name, result.Message, result.RecoveryAction),
		Stage:   types.StageInit,
		Status:  ResponseStatusError,
		Options: []Option{
			{ID: "retry", Label: "I've fixed it, try again"},
			{ID: "skip", Label: "Skip this check"},
			{ID: "help", Label: "I need help"},
		},
	}
}

func (pm *PromptManager) formatPreFlightWarnings(result *observability.PreFlightResult) string {
	var sb strings.Builder
	sb.WriteString("⚠️ Pre-flight checks completed with warnings:\n\n")

	for _, check := range result.Checks {
		if check.Status == observability.CheckStatusWarning {
			sb.WriteString(fmt.Sprintf("• %s: %s\n", check.Name, check.Message))
		}
	}

	sb.WriteString("\nThese are optional and you can proceed, but some features may be limited.")
	return sb.String()
}

func (pm *PromptManager) formatPreFlightErrors(result *observability.PreFlightResult) string {
	var sb strings.Builder
	sb.WriteString("❌ Pre-flight checks failed. The following issues must be resolved:\n\n")

	for _, check := range result.Checks {
		if check.Status == observability.CheckStatusFail {
			sb.WriteString(fmt.Sprintf("• %s: %s\n", check.Name, check.Message))
			if check.RecoveryAction != "" {
				sb.WriteString(fmt.Sprintf("  → %s\n", check.RecoveryAction))
			}
		}
	}

	return sb.String()
}

func (pm *PromptManager) getRecoveryOptions(check observability.CheckResult) []Option {
	options := []Option{
		{ID: "fixed", Label: "I've fixed it, try again"},
	}

	switch check.Name {
	case "docker_daemon":
		options = append(options, Option{
			ID:    "kind",
			Label: "Use local Kind cluster instead",
		})
	case "kubernetes_context":
		options = append(options, Option{
			ID:    "skip_deploy",
			Label: "Just build, don't deploy",
		})
	}

	options = append(options, Option{
		ID:    "skip_all",
		Label: "Skip all checks (not recommended)",
	})

	return options
}
