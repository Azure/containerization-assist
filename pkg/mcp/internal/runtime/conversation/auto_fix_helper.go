package conversation

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/core"
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
func (h *AutoFixHelper) AttemptAutoFix(ctx context.Context, response *ConversationResponse, stage core.ConversationStage, err error, state *ConversationState) bool {
	if h.conversationHandler == nil {
		return false
	}

	autoFixResult, autoFixErr := h.conversationHandler.attemptAutoFix(ctx, response.SessionID, types.ConversationStage(stage), err, state)
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

// convertToMCPStage converts internal types.ConversationStage to core.ConversationStage
// This function is deprecated - use convertFromTypesStage instead
func convertToMCPStage(stage types.ConversationStage) core.ConversationStage {
	return convertFromTypesStage(stage)
}

// getSuccessOptions returns appropriate options for successful auto-fix based on stage
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
		// All other stages (PreFlight, Analyze, Dockerfile, Scan, Completed, etc.) use default behavior
		return []Option{
			{ID: "continue", Label: "Continue", Recommended: true},
			{ID: "review", Label: "Review changes"},
		}
	}
}

// getStageDisplayName returns a display name for the stage
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
		// All other stages (PreFlight, Analyze, Dockerfile, Scan, Completed, etc.) return "Operation"
		return "Operation"
	}
}

// getStageErrorPrefix returns an error message prefix for the stage
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
		// All other stages (PreFlight, Analyze, Dockerfile, Scan, Completed, etc.) return "Operation"
		return "Operation"
	}
}
