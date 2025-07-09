package conversation

import (
	"context"
	"fmt"
	"strings"
)

type AutoFixHelper struct {
	conversationHandler *ConversationHandler
	retrySystem         *IntelligentRetrySystem
}

func NewAutoFixHelper(handler *ConversationHandler) *AutoFixHelper {
	helper := &AutoFixHelper{
		conversationHandler: handler,
	}

	if handler != nil {
		helper.retrySystem = NewIntelligentRetrySystem(handler.logger)
	}

	return helper
}
func (h *AutoFixHelper) AttemptAutoFix(ctx context.Context, response *ConversationResponse, stage core.ConversationStage, err error, state *ConversationState) bool {
	if h.conversationHandler == nil {
		return false
	}

	autoFixResult, autoFixErr := h.conversationHandler.attemptAutoFix(ctx, response.SessionID, stage, err, state)
	if autoFixErr != nil || autoFixResult == nil {
		return false
	}

	if autoFixResult.Success {
		response.Status = ResponseStatusSuccess
		response.Message = fmt.Sprintf("%s issue resolved automatically!\n\nFixes applied: %s",
			getStageDisplayName(stage), strings.Join(autoFixResult.AttemptedFixes, ", "))

		response.Options = h.getSuccessOptions(stage)
		return true
	}

	h.addIntelligentRetryGuidance(ctx, RetryGuidanceInput{
		Response:      response,
		Stage:         stage,
		Error:         err,
		AutoFixResult: autoFixResult,
		State:         state,
	})
	return true
}

func convertToMCPStage(stage types.ConversationStage) core.ConversationStage {
	return convertFromTypesStage(stage)
}

func (h *AutoFixHelper) getSuccessOptions(stage core.ConversationStage) []Option {
	switch stage {
	case core.ConversationStageBuild:
		return []Option{
			{ID: "continue", Label: "Continue to next stage", Recommended: true},
			{ID: "review", Label: "Review changes"},
		}
	case core.ConversationStagePush:
		return []Option{
			{ID: "continue", Label: "Continue to manifest generation", Recommended: true},
			{ID: "review", Label: "Review changes"},
		}
	case core.ConversationStageManifests:
		return []Option{
			{ID: "continue", Label: "Continue to deployment", Recommended: true},
			{ID: "review", Label: "Review changes"},
		}
	case core.ConversationStageDeploy:
		return []Option{
			{ID: "continue", Label: "Continue to completion", Recommended: true},
			{ID: "review", Label: "Review deployment status"},
		}
	default:
		return []Option{
			{ID: "continue", Label: "Continue", Recommended: true},
			{ID: "review", Label: "Review changes"},
		}
	}
}

func getStageDisplayName(stage core.ConversationStage) string {
	switch stage {
	case core.ConversationStageBuild:
		return "Build"
	case core.ConversationStagePush:
		return "Push"
	case core.ConversationStageManifests:
		return "Manifest generation"
	case core.ConversationStageDeploy:
		return "Deployment"
	default:
		return "Operation"
	}
}

func getStageErrorPrefix(stage core.ConversationStage) string {
	switch stage {
	case core.ConversationStageBuild:
		return "Build"
	case core.ConversationStagePush:
		return "Failed to push Docker image"
	case core.ConversationStageManifests:
		return "Failed to generate Kubernetes manifests"
	case core.ConversationStageDeploy:
		return "Deployment"
	default:
		return "Operation"
	}
}

type RetryGuidanceInput struct {
	Response      *ConversationResponse
	Stage         core.ConversationStage
	Error         error
	AutoFixResult *AutoFixResult
	State         *ConversationState
}

func (h *AutoFixHelper) addIntelligentRetryGuidance(ctx context.Context, input RetryGuidanceInput) {
	if h.retrySystem == nil {
		input.Response.Message = fmt.Sprintf("%s failed: %v\n\nWould you like to:", getStageErrorPrefix(input.Stage), input.Error)
		input.Response.Options = input.AutoFixResult.FallbackOptions
		return
	}

	retryCtx := h.buildRetryContext(input.Response.SessionID, input.Error, input.AutoFixResult, input.State)

	guidance := h.retrySystem.GenerateRetryGuidance(ctx, retryCtx)

	errorRecovery := &ErrorRecoveryGuidance{
		ErrorType:          h.classifyError(input.Error, input.Stage),
		AttemptCount:       retryCtx.AttemptCount,
		ProgressAssessment: guidance.ProgressAssessment,
		FocusAreas:         guidance.FocusAreas,
		RecommendedTools:   guidance.SpecificTools,
		NextSteps:          guidance.NextSteps,
		SuccessIndicators:  guidance.SuccessIndicators,
		AvoidRepeating:     guidance.AvoidRepeating,
		IsProgressive:      retryCtx.AttemptCount > 1,
	}

	message := fmt.Sprintf("🔧 **%s Error Recovery Assistance**\n\n", getStageDisplayName(input.Stage))
	message += fmt.Sprintf("**Error**: %v\n\n", input.Error)
	message += fmt.Sprintf("**Auto-fixes attempted**: %s\n\n", strings.Join(input.AutoFixResult.AttemptedFixes, ", "))

	if retryCtx.AttemptCount > 1 {
		message += fmt.Sprintf("**Attempt %d**: Let's take a more systematic approach.\n\n", retryCtx.AttemptCount)
	}

	message += "**Intelligent Guidance**:\n"
	message += fmt.Sprintf("- **Error Type**: %s\n", errorRecovery.ErrorType)
	message += fmt.Sprintf("- **Focus**: %s\n", strings.Join(guidance.FocusAreas, ", "))
	message += fmt.Sprintf("- **Recommended Tools**: %s\n", strings.Join(guidance.SpecificTools, ", "))

	input.Response.Message = message
	input.Response.WithErrorRecovery(errorRecovery)
	input.Response.Options = input.AutoFixResult.FallbackOptions
}

func (h *AutoFixHelper) buildRetryContext(sessionID string, err error, autoFixResult *AutoFixResult, state *ConversationState) *RetryContext {
	attemptCount := 0
	var previousAttempts []RetryAttempt

	for i, turn := range state.History {
		if strings.Contains(strings.ToLower(turn.Assistant), "error") || strings.Contains(strings.ToLower(turn.Assistant), "failed") {
			attemptCount++

			attempt := RetryAttempt{
				AttemptNumber: i + 1,
				Approach:      fmt.Sprintf("Auto-fix attempt %d", attemptCount),
				Result:        "Failed",
				FixApplied:    strings.Join(autoFixResult.AttemptedFixes, ", "),
			}
			previousAttempts = append(previousAttempts, attempt)
		}
	}

	return &RetryContext{
		SessionID:        sessionID,
		OriginalError:    err.Error(),
		AttemptCount:     attemptCount,
		PreviousAttempts: previousAttempts,
		ProjectContext:   h.buildRepositoryContext(state),
	}
}

func (h *AutoFixHelper) classifyError(err error, stage core.ConversationStage) string {
	errorMsg := strings.ToLower(err.Error())

	switch stage {
	case core.ConversationStageBuild:
		if strings.Contains(errorMsg, "copy failed") || strings.Contains(errorMsg, "no such file") {
			return "Docker Build - File Not Found"
		} else if strings.Contains(errorMsg, "command failed") || strings.Contains(errorMsg, "non-zero code") {
			return "Docker Build - Command Execution"
		} else if strings.Contains(errorMsg, "permission denied") {
			return "Docker Build - Permission Error"
		} else if strings.Contains(errorMsg, "package not found") || strings.Contains(errorMsg, "module not found") {
			return "Docker Build - Dependency Error"
		}
		return "Docker Build - General Error"
	case core.ConversationStagePush:
		return "Docker Push Error"
	case core.ConversationStageDeploy:
		return "Kubernetes Deployment Error"
	default:
		return "General Error"
	}
}

func (h *AutoFixHelper) buildRepositoryContext(state *ConversationState) *RepositoryContext {
	ctx := &RepositoryContext{
		WorkspaceDir: "/workspace",
	}

	for _, artifact := range state.Artifacts {
		if artifact.Type == "dockerfile" {
		} else if artifact.Type == "analysis" {
		}
	}

	return ctx
}
