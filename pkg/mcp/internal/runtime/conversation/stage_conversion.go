package conversation

import (
	"github.com/Azure/container-kit/pkg/mcp"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
)

// convertFromTypesStage converts internal types.ConversationStage to mcp.ConversationStage
func convertFromTypesStage(stage types.ConversationStage) mcp.ConversationStage {
	switch stage {
	case types.StagePreFlight:
		return mcp.ConversationStagePreFlight
	case types.StageWelcome, types.StageInit:
		// Welcome and init stages map to preflight
		return mcp.ConversationStagePreFlight
	case types.StageAnalysis:
		return mcp.ConversationStageAnalyze
	case types.StageDockerfile:
		return mcp.ConversationStageAnalyze // Maps to analyze since it's pre-build
	case types.StageBuild:
		return mcp.ConversationStageBuild
	case types.StagePush:
		return mcp.ConversationStagePush
	case types.StageManifests:
		return mcp.ConversationStageManifests
	case types.StageDeployment:
		return mcp.ConversationStageDeploy
	case types.StageCompleted:
		return mcp.ConversationStageCompleted
	default:
		// Default to analyze for unknown stages
		return mcp.ConversationStageAnalyze
	}
}

// mapMCPStageToDetailedStage provides a reverse mapping for internal use
// This maps the simplified mcp stages to more detailed internal stages
func mapMCPStageToDetailedStage(mcpStage mcp.ConversationStage, context map[string]interface{}) types.ConversationStage {
	// Check context for more specific stage information
	if context != nil {
		if detailedStage, ok := context["detailed_stage"].(string); ok {
			// Try to parse the detailed stage
			switch types.ConversationStage(detailedStage) {
			case types.StageWelcome, types.StagePreFlight, types.StageInit,
				types.StageAnalysis, types.StageDockerfile, types.StageBuild,
				types.StagePush, types.StageManifests, types.StageDeployment,
				types.StageCompleted:
				return types.ConversationStage(detailedStage)
			}
		}
	}

	// Fall back to basic mapping
	switch mcpStage {
	case mcp.ConversationStagePreFlight:
		return types.StagePreFlight
	case mcp.ConversationStageAnalyze:
		return types.StageAnalysis
	case mcp.ConversationStageDockerfile:
		return types.StageDockerfile
	case mcp.ConversationStageBuild:
		return types.StageBuild
	case mcp.ConversationStagePush:
		return types.StagePush
	case mcp.ConversationStageManifests:
		return types.StageManifests
	case mcp.ConversationStageDeploy:
		return types.StageDeployment
	case mcp.ConversationStageCompleted:
		return types.StageCompleted
	case mcp.ConversationStageScan:
		// Scan doesn't have a direct mapping in types, treat as completed
		return types.StageCompleted
	default:
		return types.StageInit
	}
}
