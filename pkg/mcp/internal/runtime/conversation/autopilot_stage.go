package conversation

import (
	"fmt"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/core"
)

func getStageProgress(currentStage core.ConversationStage) string {
	progressMap := map[core.ConversationStage]int{
		core.ConversationStageAnalyze: 4,
		core.ConversationStageBuild:   6,
		core.ConversationStageDeploy:  8,
		core.ConversationStageScan:    9,
	}

	currentStep := 1
	totalSteps := 10

	if step, exists := progressMap[currentStage]; exists {
		currentStep = step
	}

	return fmt.Sprintf("[Step %d/%d]", currentStep, totalSteps)
}

func getStageIntro(stage core.ConversationStage) string {
	intros := map[core.ConversationStage]string{
		core.ConversationStageAnalyze: "Analyzing your repository to understand the project structure.",
		core.ConversationStageBuild:   "Building your Docker image with the generated Dockerfile.",
		core.ConversationStageDeploy:  "Deploying your application to the Kubernetes cluster.",
		core.ConversationStageScan:    "Running security scans on your container image.",
	}

	if intro, exists := intros[stage]; exists {
		return intro
	}
	return "Processing your request..."
}

func (pm *PromptManager) hasAutopilotEnabled(state *ConversationState) bool {
	if autopilot, ok := state.Context["autopilot_enabled"].(bool); ok && autopilot {
		return true
	}

	if skipConfirmations, ok := state.Context["skip_confirmations"].(bool); ok && skipConfirmations {
		return true
	}

	return false
}

func (pm *PromptManager) enableAutopilot(state *ConversationState) {
	state.Context["autopilot_enabled"] = true
	pm.logger.Info("Autopilot mode enabled", "session_id", state.SessionState.SessionID)
}

func (pm *PromptManager) disableAutopilot(state *ConversationState) {
	state.Context["autopilot_enabled"] = false
	pm.logger.Info("Autopilot mode disabled", "session_id", state.SessionState.SessionID)
}

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

	return nil
}
