package conversation

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/domain/shared"
)

func (ps *PromptServiceImpl) handleWelcomeStage(_ context.Context, state *ConversationState, input string) *ConversationResponse {

	progressPrefix := fmt.Sprintf("%s %s\n\n", getStageProgress(convertFromTypesStage(shared.StageWelcome)), getStageIntro(convertFromTypesStage(shared.StageWelcome)))
	if input == "" {

		return &ConversationResponse{
			Message: fmt.Sprintf(`%s🎉 Welcome to Container Kit! I'm here to help you containerize your application.

I'll guide you through:
• 🔍 Analyzing your code
• 📦 Creating a Dockerfile
• 🏗️ Building your container image
• ☸️ Generating Kubernetes manifests
• 🚀 Deploying to your cluster

How would you like to proceed?`, progressPrefix),
			Stage:  convertFromTypesStage(shared.StageWelcome),
			Status: ResponseStatusWaitingInput,
			Options: []Option{
				{
					ID:          "interactive",
					Label:       "Interactive Mode - Guide me step by step",
					Description: "I'll ask for your input at each stage",
					Recommended: true,
				},
				{
					ID:          "autopilot",
					Label:       "Autopilot Mode - Automate the workflow",
					Description: "I'll make sensible defaults and proceed automatically",
				},
			},
		}
	}
	lowerInput := strings.ToLower(strings.TrimSpace(input))

	if strings.Contains(lowerInput, "interactive") || strings.Contains(lowerInput, "guide") || input == "1" {

		state.SetStage(convertFromTypesStage(shared.StageInit))
		return &ConversationResponse{
			Message: fmt.Sprintf("%sGreat! I'll guide you through each step. Let's start by analyzing your repository.\n\nCould you provide the repository URL or local path?", progressPrefix),
			Stage:   convertFromTypesStage(shared.StageInit),
			Status:  ResponseStatusWaitingInput,
			Options: []Option{
				{
					ID:          "github",
					Label:       "GitHub URL",
					Description: "e.g., https://github.com/user/repo",
				},
				{
					ID:          "local",
					Label:       "Local Path",
					Description: "e.g., /path/to/your/project",
				},
			},
		}
	}

	if strings.Contains(lowerInput, "autopilot") || strings.Contains(lowerInput, "automate") || input == "2" {

		ps.enableAutopilot(state)
		state.Context["skip_confirmations"] = true
		state.SetStage(convertFromTypesStage(shared.StageInit))

		return &ConversationResponse{
			Message: fmt.Sprintf(`%s🤖 Autopilot mode enabled! I'll proceed automatically with smart defaults.

You can still:
• Type 'stop' or 'wait' to pause at any time
• Type 'autopilot off' to switch back to interactive mode

Now, please provide your repository URL or local path:`, progressPrefix),
			Stage:  convertFromTypesStage(shared.StageInit),
			Status: ResponseStatusWaitingInput,
		}
	}
	return &ConversationResponse{
		Message: fmt.Sprintf("%sPlease choose how you'd like to proceed:", progressPrefix),
		Stage:   convertFromTypesStage(shared.StageWelcome),
		Status:  ResponseStatusWaitingInput,
		Options: []Option{
			{
				ID:          "interactive",
				Label:       "Interactive Mode - Guide me step by step",
				Recommended: true,
			},
			{
				ID:    "autopilot",
				Label: "Autopilot Mode - Automate the workflow",
			},
		},
	}
}
func (ps *PromptServiceImpl) handleInitStage(ctx context.Context, state *ConversationState, input string) *ConversationResponse {

	progressPrefix := fmt.Sprintf("%s %s\n\n", getStageProgress(convertFromTypesStage(shared.StageInit)), getStageIntro(convertFromTypesStage(shared.StageInit)))
	repoRef := ps.extractRepositoryReference(input)

	if repoRef == "" {

		return &ConversationResponse{
			Message: fmt.Sprintf("%sI'll help you containerize your application. Could you provide the repository URL or local path?", progressPrefix),
			Stage:   convertFromTypesStage(shared.StageInit),
			Status:  ResponseStatusWaitingInput,
			Options: []Option{
				{
					ID:          "github",
					Label:       "GitHub URL",
					Description: "e.g., https://github.com/user/repo",
				},
				{
					ID:          "local",
					Label:       "Local Path",
					Description: "e.g., /path/to/your/project",
				},
			},
		}
	}
	state.SessionState.RepoURL = repoRef
	state.SetStage(convertFromTypesStage(shared.StageAnalysis))

	state.Context["autopilot_enabled"] = true
	return ps.startAnalysis(ctx, state, repoRef)
}
func (ps *PromptServiceImpl) handleAnalysisStage(ctx context.Context, state *ConversationState, input string) *ConversationResponse {

	progressPrefix := fmt.Sprintf("%s %s\n\n", getStageProgress(convertFromTypesStage(shared.StageAnalysis)), getStageIntro(convertFromTypesStage(shared.StageAnalysis)))
	repoAnalysisEmpty := true
	repoURL := ""
	if state.SessionState.Metadata != nil {
		if repoAnalysis, ok := state.SessionState.Metadata["repo_analysis"].(map[string]interface{}); ok {
			repoAnalysisEmpty = len(repoAnalysis) == 0
		}
		if url, ok := state.SessionState.Metadata["repo_url"].(string); ok {
			repoURL = url
		}
	}
	if repoAnalysisEmpty && repoURL != "" {

		if completed, ok := state.Context["repository_analysis_completed"].(bool); ok && completed {

			return ps.startAnalysis(ctx, state, repoURL)
		}
		if input != "" && !ps.isFirstAnalysisPrompt(state) {
			if formResponse, err := ParseFormResponse(input, "repository_analysis"); err == nil {
				form := NewRepositoryAnalysisForm()
				if err := form.ApplyFormResponse(formResponse, state); err == nil {

					return ps.startAnalysisWithFormData(ctx, state)
				}
			}
			ps.extractAnalysisPreferences(state, input)
		}
		if ps.hasAutopilotEnabled(state) {

			smartDefaults := &FormResponse{
				FormID: "repository_analysis",
				Values: map[string]interface{}{
					"branch":         "main",
					"skip_file_tree": false,
					"optimization":   "balanced",
				},
				Skipped: false,
			}

			form := NewRepositoryAnalysisForm()
			if err := form.ApplyFormResponse(smartDefaults, state); err != nil {
				ps.logger.Warn("Failed to apply smart defaults for repository analysis", "error", err)
			}

			return ps.startAnalysis(ctx, state, repoURL)
		}
		if !ps.hasAnalysisFormPresented(state) {
			state.Context["analysis_form_presented"] = true
			form := NewRepositoryAnalysisForm()

			response := &ConversationResponse{
				Message: fmt.Sprintf("%sLet's configure how to analyze your repository. You can provide specific settings or type 'skip' to use defaults:", progressPrefix),
				Stage:   convertFromTypesStage(shared.StageAnalysis),
				Status:  ResponseStatusWaitingInput,
				Form:    form,
			}

			return response
		}
	}
	repoAnalysisExists := false
	if state.SessionState.Metadata != nil {
		if repoAnalysis, ok := state.SessionState.Metadata["repo_analysis"].(map[string]interface{}); ok {
			repoAnalysisExists = len(repoAnalysis) > 0
		}
	}
	if repoAnalysisExists {
		state.SetStage(convertFromTypesStage(shared.StageDockerfile))

		if ps.hasAutopilotEnabled(state) {

			response := &ConversationResponse{
				Message: fmt.Sprintf("%sRepository analysis complete. Proceeding to Dockerfile generation...", progressPrefix),
				Stage:   convertFromTypesStage(shared.StageAnalysis),
				Status:  ResponseStatusSuccess,
			}

			return response.WithAutoAdvance(convertFromTypesStage(shared.StageDockerfile), AutoAdvanceConfig{
				DelaySeconds:  2,
				Confidence:    0.9,
				Reason:        "Analysis complete, proceeding to Dockerfile generation",
				CanCancel:     true,
				DefaultAction: "dockerfile",
			})
		}

		return &ConversationResponse{
			Message: fmt.Sprintf("%sAnalysis is complete. Shall we proceed to create a Dockerfile?", progressPrefix),
			Stage:   convertFromTypesStage(shared.StageAnalysis),
			Status:  ResponseStatusWaitingInput,
			Options: []Option{
				{
					ID:          "proceed",
					Label:       "Yes, create Dockerfile",
					Recommended: true,
				},
				{
					ID:    "review",
					Label: "Show me the analysis first",
				},
			},
		}
	}
	return ps.startAnalysis(ctx, state, state.SessionState.RepoURL)
}
func (ps *PromptServiceImpl) handleDockerfileStage(ctx context.Context, state *ConversationState, input string) *ConversationResponse {

	progressPrefix := fmt.Sprintf("%s %s\n\n", getStageProgress(convertFromTypesStage(shared.StageDockerfile)), getStageIntro(convertFromTypesStage(shared.StageDockerfile)))
	dockerfileContent := ""
	if state.SessionState.Metadata != nil {
		if content, ok := state.SessionState.Metadata["dockerfile_content"].(string); ok {
			dockerfileContent = content
		}
	}
	if state.PendingDecision == nil && dockerfileContent == "" {
		if completed, ok := state.Context["dockerfile_config_completed"].(bool); ok && completed {

			return ps.generateDockerfile(ctx, state)
		}
		if input != "" && !ps.isFirstDockerfilePrompt(state) {
			if formResponse, err := ParseFormResponse(input, "dockerfile_config"); err == nil {
				form := NewDockerfileConfigForm()
				if err := form.ApplyFormResponse(formResponse, state); err == nil {

					return ps.generateDockerfileWithFormData(ctx, state)
				}
			}
			ps.extractDockerfilePreferences(state, input)
			if ps.hasDockerfilePreferences(state) {
				return ps.generateDockerfile(ctx, state)
			}
		}
		form := NewDockerfileConfigForm()
		if ps.hasAutopilotEnabled(state) {

			smartDefaults := &FormResponse{
				FormID: "dockerfile_config",
				Values: map[string]interface{}{
					"optimization":         "size",
					"include_health_check": true,
					"platform":             "",
				},
				Skipped: false,
			}

			if err := form.ApplyFormResponse(smartDefaults, state); err != nil {
				ps.logger.Warn("Failed to apply smart defaults for Dockerfile", "error", err)
			}

			response := &ConversationResponse{
				Message: fmt.Sprintf("%sUsing smart defaults for Dockerfile configuration...", progressPrefix),
				Stage:   convertFromTypesStage(shared.StageDockerfile),
				Status:  ResponseStatusProcessing,
			}

			return response.WithAutoAdvance(convertFromTypesStage(shared.StageBuild), AutoAdvanceConfig{
				DelaySeconds:  1,
				Confidence:    0.85,
				Reason:        "Applied smart Dockerfile defaults",
				CanCancel:     true,
				DefaultAction: "generate",
			})
		}
		state.Context["dockerfile_form_presented"] = true

		response := &ConversationResponse{
			Message: fmt.Sprintf("%sLet's configure your Dockerfile. You can provide specific settings or type 'skip' to use smart defaults:", progressPrefix),
			Stage:   convertFromTypesStage(shared.StageDockerfile),
			Status:  ResponseStatusWaitingInput,
			Form:    form,
		}

		return response
	}
	return ps.generateDockerfile(ctx, state)
}
func (ps *PromptServiceImpl) handleCompletedStage(ctx context.Context, state *ConversationState, input string) *ConversationResponse {

	lowerInput := strings.ToLower(strings.TrimSpace(input))

	if strings.Contains(lowerInput, "summary") {
		return ps.generateSummary(ctx, state)
	}

	if strings.Contains(lowerInput, "export") {
		return ps.exportArtifacts(ctx, state)
	}

	if strings.Contains(lowerInput, "help") || strings.Contains(lowerInput, "next") {

		appName := "unknown-app"
		if name, ok := state.Context["app_name"].(string); ok && name != "" {
			appName = name
		}

		return &ConversationResponse{
			Message: `Your containerization is complete! Here are your next steps:

1. **Access your application**:
   ` + "`kubectl port-forward -n " + state.Preferences.Namespace + " svc/" + appName + "-service 8080:80`" + `

2. **Monitor your deployment**:
   ` + "`kubectl get pods -n " + state.Preferences.Namespace + " -w`" + `

3. **View logs**:
   ` + "`kubectl logs -n " + state.Preferences.Namespace + " -l app=" + appName + "`" + `

What else would you like to know?`,
			Stage:  convertFromTypesStage(shared.StageCompleted),
			Status: ResponseStatusSuccess,
			Options: []Option{
				{ID: "summary", Label: "Show deployment summary"},
				{ID: "export", Label: "Export all artifacts"},
				{ID: "new", Label: "Start a new project"},
			},
		}
	}
	return &ConversationResponse{
		Message: "Your containerization journey is complete! 🎉\n\nType 'help' for next steps or 'summary' for a deployment overview.",
		Stage:   convertFromTypesStage(shared.StageCompleted),
		Status:  ResponseStatusSuccess,
	}
}
