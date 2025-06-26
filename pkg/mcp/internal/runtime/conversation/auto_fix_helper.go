package conversation

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/internal/types"
)

// AutoFixHelper encapsulates the common auto-fix logic pattern used across stages
type AutoFixHelper struct {
	conversationHandler *ConversationHandler
}

// NewAutoFixHelper creates a new auto-fix helper instance
func NewAutoFixHelper(handler *ConversationHandler) *AutoFixHelper {
	return &AutoFixHelper{
		conversationHandler: handler,
	}
}

// AttemptAutoFix attempts to automatically fix an error and updates the response accordingly
// Returns true if the response was modified (either success or with fallback options)
func (h *AutoFixHelper) AttemptAutoFix(ctx context.Context, response *ConversationResponse, stage types.ConversationStage, err error, state *ConversationState) bool {
	if h.conversationHandler == nil {
		return false
	}

	autoFixResult, autoFixErr := h.conversationHandler.attemptAutoFix(ctx, response.SessionID, stage, err, state)
	if autoFixErr != nil || autoFixResult == nil {
		return false
	}

	if autoFixResult.Success {
		// Auto-fix succeeded, update response
		response.Status = ResponseStatusSuccess
		response.Message = fmt.Sprintf("%s issue resolved automatically!\n\nFixes applied: %s",
			getStageDisplayName(stage), strings.Join(autoFixResult.AttemptedFixes, ", "))

		// Set appropriate success options based on stage
		response.Options = h.getSuccessOptions(stage)
		return true
	}

	// Auto-fix failed, show what was attempted and fallback options
	response.Message = fmt.Sprintf("%s failed: %v\n\nAttempted fixes: %s\n\nWould you like to:",
		getStageErrorPrefix(stage), err, strings.Join(autoFixResult.AttemptedFixes, ", "))
	response.Options = autoFixResult.FallbackOptions
	return true
}

// getSuccessOptions returns appropriate options for successful auto-fix based on stage
func (h *AutoFixHelper) getSuccessOptions(stage types.ConversationStage) []Option {
	switch stage {
	case types.StageBuild:
		return []Option{
			{ID: "continue", Label: "Continue to next stage", Recommended: true},
			{ID: "review", Label: "Review changes"},
		}
	case types.StagePush:
		return []Option{
			{ID: "continue", Label: "Continue to manifest generation", Recommended: true},
			{ID: "review", Label: "Review changes"},
		}
	case types.StageManifests:
		return []Option{
			{ID: "continue", Label: "Continue to deployment", Recommended: true},
			{ID: "review", Label: "Review generated manifests"},
		}
	case types.StageDeployment:
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

// getStageDisplayName returns a display name for the stage
func getStageDisplayName(stage types.ConversationStage) string {
	switch stage {
	case types.StageBuild:
		return "Build"
	case types.StagePush:
		return "Push"
	case types.StageManifests:
		return "Manifest generation"
	case types.StageDeployment:
		return "Deployment"
	default:
		return "Operation"
	}
}

// getStageErrorPrefix returns an error message prefix for the stage
func getStageErrorPrefix(stage types.ConversationStage) string {
	switch stage {
	case types.StageBuild:
		return "Build"
	case types.StagePush:
		return "Failed to push Docker image"
	case types.StageManifests:
		return "Failed to generate Kubernetes manifests"
	case types.StageDeployment:
		return "Deployment"
	default:
		return "Operation"
	}
}
