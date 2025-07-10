package conversation

import (
	"fmt"
	"strings"

	domaintypes "github.com/Azure/container-kit/pkg/mcp/domain/types"
)

func getStageProgress(currentStage domaintypes.ConversationStage) string {
	progressMap := map[domaintypes.ConversationStage]int{
		domaintypes.StageAnalysis:   4,
		domaintypes.StageBuild:      6,
		domaintypes.StageDeployment: 8,
		domaintypes.StageScan:       9,
	}

	currentStep := 1
	totalSteps := 10

	if step, exists := progressMap[currentStage]; exists {
		currentStep = step
	}

	return fmt.Sprintf("[Step %d/%d]", currentStep, totalSteps)
}

func getStageIntro(stage domaintypes.ConversationStage) string {
	intros := map[domaintypes.ConversationStage]string{
		domaintypes.StageAnalysis:   "Analyzing your repository to understand the project structure.",
		domaintypes.StageBuild:      "Building your Docker image with the generated Dockerfile.",
		domaintypes.StageDeployment: "Deploying your application to the Kubernetes cluster.",
		domaintypes.StageScan:       "Running security scans on your container image.",
	}

	if intro, exists := intros[stage]; exists {
		return intro
	}
	return "Processing your request..."
}

func (ps *PromptServiceImpl) hasAutopilotEnabled(state *ConversationState) bool {
	if autopilot, ok := state.Context["autopilot_enabled"].(bool); ok && autopilot {
		return true
	}

	if skipConfirmations, ok := state.Context["skip_confirmations"].(bool); ok && skipConfirmations {
		return true
	}

	return false
}

func (ps *PromptServiceImpl) enableAutopilot(state *ConversationState) {
	state.Context["autopilot_enabled"] = true
	ps.logger.Info("Autopilot mode enabled", "session_id", state.SessionState.SessionID)
}

func (ps *PromptServiceImpl) disableAutopilot(state *ConversationState) {
	state.Context["autopilot_enabled"] = false
	ps.logger.Info("Autopilot mode disabled", "session_id", state.SessionState.SessionID)
}

func (ps *PromptServiceImpl) handleAutopilotCommands(input string, state *ConversationState) *ConversationResponse {
	lowerInput := strings.ToLower(strings.TrimSpace(input))

	switch {
	case lowerInput == "autopilot on" || lowerInput == "enable autopilot":
		ps.enableAutopilot(state)
		return &ConversationResponse{
			Message: "✅ Autopilot mode enabled! I'll proceed through the stages automatically with minimal confirmations.\n\nYou can disable it anytime by typing 'autopilot off'.",
			Stage:   state.CurrentStage,
			Status:  ResponseStatusSuccess,
		}

	case lowerInput == "autopilot off" || lowerInput == "disable autopilot":
		ps.disableAutopilot(state)
		return &ConversationResponse{
			Message: "✅ Autopilot mode disabled. I'll ask for confirmation at each stage.",
			Stage:   state.CurrentStage,
			Status:  ResponseStatusSuccess,
		}

	case lowerInput == "autopilot status":
		enabled := ps.hasAutopilotEnabled(state)
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
		ps.disableAutopilot(state)
		return &ConversationResponse{
			Message: "⏸️ Autopilot paused. I'll wait for your confirmation before proceeding to the next stage.",
			Stage:   state.CurrentStage,
			Status:  ResponseStatusSuccess,
		}
	}

	return nil
}
