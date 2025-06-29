package conversation

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp"
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
func (h *AutoFixHelper) AttemptAutoFix(ctx context.Context, response *ConversationResponse, stage mcp.ConversationStage, err error, state *ConversationState) bool {
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

// convertToMCPStage converts internal types.ConversationStage to mcp.ConversationStage
// This function is deprecated - use convertFromTypesStage instead
func convertToMCPStage(stage types.ConversationStage) mcp.ConversationStage {
	return convertFromTypesStage(stage)
}

// getSuccessOptions returns appropriate options for successful auto-fix based on stage
func (h *AutoFixHelper) getSuccessOptions(stage mcp.ConversationStage) []Option {
	switch stage {
	case mcp.ConversationStageBuild:
		return []Option{
			{ID: "continue", Label: "Continue to next stage", Recommended: true},
			{ID: "review", Label: "Review changes"},
		}
	case mcp.ConversationStagePush:
		return []Option{
			{ID: "continue", Label: "Continue to manifest generation", Recommended: true},
			{ID: "review", Label: "Review changes"},
		}
	case mcp.ConversationStageManifests:
		return []Option{
			{ID: "continue", Label: "Continue to deployment", Recommended: true},
			{ID: "review", Label: "Review changes"},
		}
	case mcp.ConversationStageDeploy:
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
func getStageDisplayName(stage mcp.ConversationStage) string {
	switch stage {
	case mcp.ConversationStageBuild:
		return "Build"
	case mcp.ConversationStagePush:
		return "Push"
	case mcp.ConversationStageManifests:
		return "Manifest generation"
	case mcp.ConversationStageDeploy:
		return "Deployment"
	default:
		// All other stages (PreFlight, Analyze, Dockerfile, Scan, Completed, etc.) return "Operation"
		return "Operation"
	}
}

// getStageErrorPrefix returns an error message prefix for the stage
func getStageErrorPrefix(stage mcp.ConversationStage) string {
	switch stage {
	case mcp.ConversationStageBuild:
		return "Build"
	case mcp.ConversationStagePush:
		return "Failed to push Docker image"
	case mcp.ConversationStageManifests:
		return "Failed to generate Kubernetes manifests"
	case mcp.ConversationStageDeploy:
		return "Deployment"
	default:
		// All other stages (PreFlight, Analyze, Dockerfile, Scan, Completed, etc.) return "Operation"
		return "Operation"
	}
}
