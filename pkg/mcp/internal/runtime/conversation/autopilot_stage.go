package conversation

import (
	"fmt"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp"
)

// getStageProgress returns a formatted progress indicator for the current stage
func getStageProgress(currentStage mcp.ConversationStage) string {
	// Map the simplified mcp stages to detailed progress
	progressMap := map[mcp.ConversationStage]int{
		mcp.ConversationStageAnalyze: 4, // Maps to analysis stage
		mcp.ConversationStageBuild:   6, // Maps to build stage
		mcp.ConversationStageDeploy:  8, // Maps to deployment stage
		mcp.ConversationStageScan:    9, // Maps to scan stage
	}

	currentStep := 1
	totalSteps := 10

	if step, exists := progressMap[currentStage]; exists {
		currentStep = step
	}

	return fmt.Sprintf("[Step %d/%d]", currentStep, totalSteps)
}

// getStageIntro returns a short introductory message for each stage
func getStageIntro(stage mcp.ConversationStage) string {
	intros := map[mcp.ConversationStage]string{
		mcp.ConversationStageAnalyze: "Analyzing your repository to understand the project structure.",
		mcp.ConversationStageBuild:   "Building your Docker image with the generated Dockerfile.",
		mcp.ConversationStageDeploy:  "Deploying your application to the Kubernetes cluster.",
		mcp.ConversationStageScan:    "Running security scans on your container image.",
	}

	if intro, exists := intros[stage]; exists {
		return intro
	}
	return "Processing your request..."
}

// hasAutopilotEnabled checks if the user has autopilot mode enabled
func (pm *PromptManager) hasAutopilotEnabled(state *ConversationState) bool {
	// Check conversation context for autopilot flag
	if autopilot, ok := state.Context["autopilot_enabled"].(bool); ok && autopilot {
		return true
	}

	// Check if skip confirmations is enabled in preferences
	// This is a simple heuristic - we could enhance this later
	if skipConfirmations, ok := state.Context["skip_confirmations"].(bool); ok && skipConfirmations {
		return true
	}

	// Default to manual mode for safety
	return false
}

// enableAutopilot enables autopilot mode for the conversation
func (pm *PromptManager) enableAutopilot(state *ConversationState) {
	state.Context["autopilot_enabled"] = true
	pm.logger.Info().Str("session_id", state.SessionID).Msg("Autopilot mode enabled")
}

// disableAutopilot disables autopilot mode for the conversation
func (pm *PromptManager) disableAutopilot(state *ConversationState) {
	state.Context["autopilot_enabled"] = false
	pm.logger.Info().Str("session_id", state.SessionID).Msg("Autopilot mode disabled")
}

// handleAutopilotCommands checks for autopilot control commands in user input
func (pm *PromptManager) handleAutopilotCommands(input string, state *ConversationState) *ConversationResponse {
	lowerInput := strings.ToLower(strings.TrimSpace(input))

	switch {
	case lowerInput == "autopilot on" || lowerInput == "enable autopilot":
		pm.enableAutopilot(state)
		return &ConversationResponse{
			Message: "✅ Autopilot mode enabled! I'll proceed through the stages automatically with minimal confirmations.\n\nYou can disable it anytime by typing 'autopilot off'.",
			Stage:   state.CurrentStage,
			Status:  ResponseStatusSuccess,
		}

	case lowerInput == "autopilot off" || lowerInput == "disable autopilot":
		pm.disableAutopilot(state)
		return &ConversationResponse{
			Message: "✅ Autopilot mode disabled. I'll ask for confirmation at each stage.",
			Stage:   state.CurrentStage,
			Status:  ResponseStatusSuccess,
		}

	case lowerInput == "autopilot status":
		enabled := pm.hasAutopilotEnabled(state)
		status := "disabled"
		if enabled {
			status = "enabled"
		}
		return &ConversationResponse{
			Message: fmt.Sprintf("Autopilot mode is currently %s.", status),
			Stage:   state.CurrentStage,
			Status:  ResponseStatusSuccess,
		}

	case lowerInput == "stop":
		pm.disableAutopilot(state)
		return &ConversationResponse{
			Message: "⏸️ Autopilot paused. I'll wait for your confirmation before proceeding to the next stage.",
			Stage:   state.CurrentStage,
			Status:  ResponseStatusSuccess,
		}
	}

	// Not an autopilot command
	return nil
}
