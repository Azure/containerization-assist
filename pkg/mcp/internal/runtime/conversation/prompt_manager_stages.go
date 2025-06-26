package conversation

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/internal/types"
)

// handleWelcomeStage handles the welcome stage where users choose their workflow mode
func (pm *PromptManager) handleWelcomeStage(ctx context.Context, state *ConversationState, input string) *ConversationResponse {
	// Add progress indicator and stage intro
	progressPrefix := fmt.Sprintf("%s %s\n\n", getStageProgress(types.StageWelcome), getStageIntro(types.StageWelcome))

	// Check if this is the first interaction
	if input == "" {
		// Present welcome message with mode selection
		return &ConversationResponse{
			Message: fmt.Sprintf(`%s🎉 Welcome to Container Kit! I'm here to help you containerize your application.

I'll guide you through:
• 🔍 Analyzing your code
• 📦 Creating a Dockerfile
• 🏗️ Building your container image
• ☸️ Generating Kubernetes manifests
• 🚀 Deploying to your cluster

How would you like to proceed?`, progressPrefix),
			Stage:  types.StageWelcome,
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

	// Process mode selection
	lowerInput := strings.ToLower(strings.TrimSpace(input))

	if strings.Contains(lowerInput, "interactive") || strings.Contains(lowerInput, "guide") || input == "1" {
		// Interactive mode - default behavior
		state.SetStage(types.StageInit)
		return &ConversationResponse{
			Message: fmt.Sprintf("%sGreat! I'll guide you through each step. Let's start by analyzing your repository.\n\nCould you provide the repository URL or local path?", progressPrefix),
			Stage:   types.StageInit,
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
		// Enable autopilot mode
		pm.enableAutopilot(state)
		state.Context["skip_confirmations"] = true
		state.SetStage(types.StageInit)

		return &ConversationResponse{
			Message: fmt.Sprintf(`%s🤖 Autopilot mode enabled! I'll proceed automatically with smart defaults.

You can still:
• Type 'stop' or 'wait' to pause at any time
• Type 'autopilot off' to switch back to interactive mode

Now, please provide your repository URL or local path:`, progressPrefix),
			Stage:  types.StageInit,
			Status: ResponseStatusWaitingInput,
		}
	}

	// If input doesn't match expected options, re-prompt
	return &ConversationResponse{
		Message: fmt.Sprintf("%sPlease choose how you'd like to proceed:", progressPrefix),
		Stage:   types.StageWelcome,
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

// handleInitStage handles the initial stage of the conversation
func (pm *PromptManager) handleInitStage(ctx context.Context, state *ConversationState, input string) *ConversationResponse {
	// Add progress indicator and stage intro
	progressPrefix := fmt.Sprintf("%s %s\n\n", getStageProgress(types.StageInit), getStageIntro(types.StageInit))

	// Check if input contains a repository reference
	repoRef := pm.extractRepositoryReference(input)

	if repoRef == "" {
		// Ask for repository
		return &ConversationResponse{
			Message: fmt.Sprintf("%sI'll help you containerize your application. Could you provide the repository URL or local path?", progressPrefix),
			Stage:   types.StageInit,
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

	// We have a repository, move to analysis
	state.RepoURL = repoRef
	state.SetStage(types.StageAnalysis)

	// Enable autopilot mode when URL is provided directly
	// This allows the conversation to automatically proceed through all stages
	state.Context["autopilot_enabled"] = true

	// Start analysis
	return pm.startAnalysis(ctx, state, repoRef)
}

// handleAnalysisStage handles the repository analysis stage
func (pm *PromptManager) handleAnalysisStage(ctx context.Context, state *ConversationState, input string) *ConversationResponse {
	// Add progress indicator and stage intro
	progressPrefix := fmt.Sprintf("%s %s\n\n", getStageProgress(types.StageAnalysis), getStageIntro(types.StageAnalysis))

	// Check if we need to gather analysis preferences using structured form
	if len(state.RepoAnalysis) == 0 && state.RepoURL != "" {
		// Check if we already have analysis config completed
		if completed, ok := state.Context["repository_analysis_completed"].(bool); ok && completed {
			// Start analysis with gathered preferences
			return pm.startAnalysis(ctx, state, state.RepoURL)
		}

		// Check if user provided form response
		if input != "" && !pm.isFirstAnalysisPrompt(state) {
			if formResponse, err := ParseFormResponse(input, "repository_analysis"); err == nil {
				form := NewRepositoryAnalysisForm()
				if err := form.ApplyFormResponse(formResponse, state); err == nil {
					// Form processed successfully, proceed with analysis
					return pm.startAnalysisWithFormData(ctx, state)
				}
			}

			// Try to extract preferences from natural language input
			pm.extractAnalysisPreferences(state, input)
		}

		// Check for autopilot mode
		if pm.hasAutopilotEnabled(state) {
			// Auto-fill with smart defaults
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
				pm.logger.Warn().Err(err).Msg("Failed to apply smart defaults for repository analysis")
			}

			return pm.startAnalysis(ctx, state, state.RepoURL)
		}

		// Manual mode: present form to user
		if !pm.hasAnalysisFormPresented(state) {
			state.Context["analysis_form_presented"] = true
			form := NewRepositoryAnalysisForm()

			response := &ConversationResponse{
				Message: fmt.Sprintf("%sLet's configure how to analyze your repository. You can provide specific settings or type 'skip' to use defaults:", progressPrefix),
				Stage:   types.StageAnalysis,
				Status:  ResponseStatusWaitingInput,
				Form:    form,
			}

			return response
		}
	}

	// If analysis is complete, ask about moving to Dockerfile
	if len(state.RepoAnalysis) > 0 {
		state.SetStage(types.StageDockerfile)

		if pm.hasAutopilotEnabled(state) {
			// Auto-advance to Dockerfile stage
			response := &ConversationResponse{
				Message: fmt.Sprintf("%sRepository analysis complete. Proceeding to Dockerfile generation...", progressPrefix),
				Stage:   types.StageAnalysis,
				Status:  ResponseStatusSuccess,
			}

			return response.WithAutoAdvance(types.StageDockerfile, AutoAdvanceConfig{
				DelaySeconds:  2,
				Confidence:    0.9,
				Reason:        "Analysis complete, proceeding to Dockerfile generation",
				CanCancel:     true,
				DefaultAction: "dockerfile",
			})
		} else {
			return &ConversationResponse{
				Message: fmt.Sprintf("%sAnalysis is complete. Shall we proceed to create a Dockerfile?", progressPrefix),
				Stage:   types.StageAnalysis,
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
	}

	// Start or retry analysis
	return pm.startAnalysis(ctx, state, state.RepoURL)
}

// handleDockerfileStage handles Dockerfile generation
func (pm *PromptManager) handleDockerfileStage(ctx context.Context, state *ConversationState, input string) *ConversationResponse {
	// Add progress indicator and stage intro
	progressPrefix := fmt.Sprintf("%s %s\n\n", getStageProgress(types.StageDockerfile), getStageIntro(types.StageDockerfile))

	// Check if we need to gather preferences using structured form
	if state.PendingDecision == nil && state.Dockerfile.Content == "" {

		// Check if we already have Dockerfile config completed
		if completed, ok := state.Context["dockerfile_config_completed"].(bool); ok && completed {
			// Generate Dockerfile with gathered preferences
			return pm.generateDockerfile(ctx, state)
		}

		// Check if user provided form response
		if input != "" && !pm.isFirstDockerfilePrompt(state) {
			if formResponse, err := ParseFormResponse(input, "dockerfile_config"); err == nil {
				form := NewDockerfileConfigForm()
				if err := form.ApplyFormResponse(formResponse, state); err == nil {
					// Form processed successfully, proceed with generation
					return pm.generateDockerfileWithFormData(ctx, state)
				}
			}

			// Try to extract preferences from natural language input
			pm.extractDockerfilePreferences(state, input)

			// If we got some preferences, proceed
			if pm.hasDockerfilePreferences(state) {
				return pm.generateDockerfile(ctx, state)
			}
		}

		// Present structured form for Dockerfile configuration
		form := NewDockerfileConfigForm()

		// Check if user has autopilot enabled for smart defaults
		if pm.hasAutopilotEnabled(state) {
			// Auto-fill form with smart defaults and proceed
			smartDefaults := &FormResponse{
				FormID: "dockerfile_config",
				Values: map[string]interface{}{
					"optimization":         "size",
					"include_health_check": true,
					"platform":             "", // auto-detect
				},
				Skipped: false,
			}

			if err := form.ApplyFormResponse(smartDefaults, state); err != nil {
				pm.logger.Warn().Err(err).Msg("Failed to apply smart defaults for Dockerfile")
			}

			response := &ConversationResponse{
				Message: fmt.Sprintf("%sUsing smart defaults for Dockerfile configuration...", progressPrefix),
				Stage:   types.StageDockerfile,
				Status:  ResponseStatusProcessing,
			}

			return response.WithAutoAdvance(types.StageBuild, AutoAdvanceConfig{
				DelaySeconds:  1,
				Confidence:    0.85,
				Reason:        "Applied smart Dockerfile defaults",
				CanCancel:     true,
				DefaultAction: "generate",
			})
		}

		// Manual mode: present form to user
		state.Context["dockerfile_form_presented"] = true

		response := &ConversationResponse{
			Message: fmt.Sprintf("%sLet's configure your Dockerfile. You can provide specific settings or type 'skip' to use smart defaults:", progressPrefix),
			Stage:   types.StageDockerfile,
			Status:  ResponseStatusWaitingInput,
			Form:    form,
		}

		return response
	}

	// Generate Dockerfile
	return pm.generateDockerfile(ctx, state)
}

// handleCompletedStage handles the completed stage
func (pm *PromptManager) handleCompletedStage(ctx context.Context, state *ConversationState, input string) *ConversationResponse {
	// Check for follow-up actions
	lowerInput := strings.ToLower(strings.TrimSpace(input))

	if strings.Contains(lowerInput, "summary") {
		return pm.generateSummary(ctx, state)
	}

	if strings.Contains(lowerInput, "export") {
		return pm.exportArtifacts(ctx, state)
	}

	if strings.Contains(lowerInput, "help") || strings.Contains(lowerInput, "next") {
		return &ConversationResponse{
			Message: `Your containerization is complete! Here are your next steps:

1. **Access your application**:
   ` + "`kubectl port-forward -n " + state.Preferences.Namespace + " svc/" + state.Context["app_name"].(string) + "-service 8080:80`" + `

2. **Monitor your deployment**:
   ` + "`kubectl get pods -n " + state.Preferences.Namespace + " -w`" + `

3. **View logs**:
   ` + "`kubectl logs -n " + state.Preferences.Namespace + " -l app=" + state.Context["app_name"].(string) + "`" + `

What else would you like to know?`,
			Stage:  types.StageCompleted,
			Status: ResponseStatusSuccess,
			Options: []Option{
				{ID: "summary", Label: "Show deployment summary"},
				{ID: "export", Label: "Export all artifacts"},
				{ID: "new", Label: "Start a new project"},
			},
		}
	}

	// Default completed message
	return &ConversationResponse{
		Message: "Your containerization journey is complete! 🎉\n\nType 'help' for next steps or 'summary' for a deployment overview.",
		Stage:   types.StageCompleted,
		Status:  ResponseStatusSuccess,
	}
}
