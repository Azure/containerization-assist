package conversation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	sessiontypes "github.com/Azure/container-copilot/pkg/mcp/internal/types/session"
	publicutils "github.com/Azure/container-copilot/pkg/mcp/utils"
)

// Analysis and Dockerfile generation helpers

// startAnalysisWithFormData starts analysis after form data has been applied
func (pm *PromptManager) startAnalysisWithFormData(ctx context.Context, state *ConversationState) *ConversationResponse {
	pm.applyAnalysisFormDataToPreferences(state)
	return pm.startAnalysis(ctx, state, state.RepoURL)
}

// startAnalysis initiates repository analysis
func (pm *PromptManager) startAnalysis(ctx context.Context, state *ConversationState, repoURL string) *ConversationResponse {
	response := &ConversationResponse{
		Stage:  types.StageAnalysis,
		Status: ResponseStatusProcessing,
	}

	// Apply preferences from form data
	pm.applyAnalysisFormDataToPreferences(state)

	// Execute analysis tool
	params := map[string]interface{}{
		"repo_url":       repoURL,
		"session_id":     state.SessionID,
		"skip_file_tree": state.Preferences.SkipFileTree,
	}

	// Add branch if specified
	if branch := GetFormValue(state, "repository_analysis", "branch", ""); branch != nil {
		if branchStr, ok := branch.(string); ok && branchStr != "" {
			params["branch"] = branchStr
		}
	}

	startTime := time.Now()
	result, err := pm.toolOrchestrator.ExecuteTool(ctx, "analyze_repository", params, state.SessionState.SessionID)
	duration := time.Since(startTime)

	toolCall := ToolCall{
		Tool:       "analyze_repository",
		Parameters: params,
		Duration:   duration,
	}

	if err != nil {
		toolCall.Error = &types.ToolError{
			Type:      "analysis_error",
			Message:   fmt.Sprintf("analyze_repository error: %v", err),
			Retryable: true,
			Timestamp: time.Now(),
		}
		response.ToolCalls = []ToolCall{toolCall}
		response.Status = ResponseStatusError
		response.Message = fmt.Sprintf("Failed to analyze repository: %v", err)
		return response
	}

	toolCall.Result = result
	response.ToolCalls = []ToolCall{toolCall}

	// Parse analysis results
	if result != nil {
		if analysis, ok := result.(map[string]interface{}); ok {
			state.RepoAnalysis = analysis

			// Extract key information
			language := publicutils.GetStringFromMap(analysis, "language")
			if language == "" {
				language = "Unknown"
			}
			framework := publicutils.GetStringFromMap(analysis, "framework")
			entryPoints := pm.getStringSliceFromMap(analysis, "entry_points", []string{})

			// Build response message
			var msg strings.Builder
			msg.WriteString("Analysis complete! I found:\n")
			msg.WriteString(fmt.Sprintf("- Language: %s\n", language))
			if framework != "" {
				msg.WriteString(fmt.Sprintf("- Framework: %s\n", framework))
			}
			if len(entryPoints) > 0 {
				msg.WriteString(fmt.Sprintf("- Entry point: %s\n", entryPoints[0]))
			}

			// Add suggestions if available
			if suggestions, ok := analysis["suggestions"].([]interface{}); ok && len(suggestions) > 0 {
				msg.WriteString("\nSuggested optimizations:\n")
				for _, s := range suggestions {
					if str, ok := s.(string); ok {
						msg.WriteString(fmt.Sprintf("- %s\n", str))
					}
				}
			}

			msg.WriteString("\nShall we proceed to create a Dockerfile?")

			response.Message = msg.String()
			response.Status = ResponseStatusSuccess
			response.NextSteps = []string{"Generate Dockerfile", "Review analysis details"}
		}
	}

	return response
}

// generateDockerfile creates the Dockerfile
func (pm *PromptManager) generateDockerfile(ctx context.Context, state *ConversationState) *ConversationResponse {
	response := &ConversationResponse{
		Stage:  types.StageDockerfile,
		Status: ResponseStatusProcessing,
	}

	params := map[string]interface{}{
		"session_id":           state.SessionID,
		"optimization":         state.Preferences.Optimization,
		"include_health_check": state.Preferences.IncludeHealthCheck,
	}

	if state.Preferences.BaseImage != "" {
		params["base_image"] = state.Preferences.BaseImage
	}

	startTime := time.Now()
	result, err := pm.toolOrchestrator.ExecuteTool(ctx, "generate_dockerfile", params, state.SessionState.SessionID)
	duration := time.Since(startTime)

	toolCall := ToolCall{
		Tool:       "generate_dockerfile",
		Parameters: params,
		Duration:   duration,
	}

	if err != nil {
		toolCall.Error = &types.ToolError{
			Type:      "generation_error",
			Message:   fmt.Sprintf("generate_dockerfile error: %v", err),
			Retryable: true,
			Timestamp: time.Now(),
		}
		response.ToolCalls = []ToolCall{toolCall}
		response.Status = ResponseStatusError
		response.Message = fmt.Sprintf("Failed to generate Dockerfile: %v", err)
		return response
	}

	toolCall.Result = result
	response.ToolCalls = []ToolCall{toolCall}

	// Parse Dockerfile result
	if result != nil {
		if dockerResult, ok := result.(map[string]interface{}); ok {
			content := publicutils.GetStringFromMap(dockerResult, "content")
			if content != "" {
				state.Dockerfile.Content = content
				path := publicutils.GetStringFromMap(dockerResult, "file_path")
				if path == "" {
					path = "Dockerfile"
				}
				state.Dockerfile.Path = path

				// Check for validation results
				if validationData, ok := dockerResult["validation"].(map[string]interface{}); ok {
					// Convert validation result to simplified format for storage
					validation := &sessiontypes.ValidationResult{
						Valid:       publicutils.GetBoolFromMap(validationData, "valid"),
						ValidatedAt: time.Now(),
					}

					// Count errors and warnings
					if errors, ok := validationData["errors"].([]interface{}); ok {
						validation.ErrorCount = len(errors)
						for _, err := range errors {
							if errMap, ok := err.(map[string]interface{}); ok {
								msg := publicutils.GetStringFromMap(errMap, "message")
								if msg != "" {
									validation.Errors = append(validation.Errors, msg)
								}
							}
						}
					}

					if warnings, ok := validationData["warnings"].([]interface{}); ok {
						validation.WarningCount = len(warnings)
						for _, warn := range warnings {
							if warnMap, ok := warn.(map[string]interface{}); ok {
								msg := publicutils.GetStringFromMap(warnMap, "message")
								if msg != "" {
									validation.Warnings = append(validation.Warnings, msg)
								}
							}
						}
					}

					state.Dockerfile.ValidationResult = validation
				}

				// Add Dockerfile artifact
				artifact := Artifact{
					Type:    "dockerfile",
					Name:    path,
					Content: content,
					Stage:   types.StageDockerfile,
				}
				state.AddArtifact(artifact)

				response.Message = fmt.Sprintf("✅ Dockerfile created successfully!\n\n"+
					"Optimized for: %s\n"+
					"Health check: %v\n\n"+
					"Ready to build the Docker image?",
					state.Preferences.Optimization,
					state.Preferences.IncludeHealthCheck)

				response.Status = ResponseStatusSuccess
				response.NextSteps = []string{"Build Docker image", "Review Dockerfile"}

				// Move to next stage
				state.SetStage(types.StageBuild)
			}
		}
	}

	return response
}

// generateDockerfileWithFormData processes form data and generates Dockerfile
func (pm *PromptManager) generateDockerfileWithFormData(ctx context.Context, state *ConversationState) *ConversationResponse {
	// Apply form data to preferences
	pm.applyFormDataToPreferences(state)

	// Mark config as completed
	state.Context["dockerfile_config_completed"] = true

	// Generate dockerfile with the preferences
	return pm.generateDockerfile(ctx, state)
}

// Form data helper functions

func (pm *PromptManager) isFirstDockerfilePrompt(state *ConversationState) bool {
	_, presented := state.Context["dockerfile_form_presented"]
	return !presented
}

func (pm *PromptManager) hasDockerfilePreferences(state *ConversationState) bool {
	// Check if we have any Dockerfile preferences set
	return state.Preferences.Optimization != "" ||
		state.Preferences.BaseImage != "" ||
		state.Context["dockerfile_config_completed"] != nil
}

func (pm *PromptManager) isFirstAnalysisPrompt(state *ConversationState) bool {
	_, presented := state.Context["analysis_form_presented"]
	return !presented
}

func (pm *PromptManager) hasAnalysisFormPresented(state *ConversationState) bool {
	_, presented := state.Context["analysis_form_presented"]
	return presented
}

// Apply form data helper functions

func (pm *PromptManager) applyFormDataToPreferences(state *ConversationState) {
	// Check for Dockerfile config form responses
	if optimization := GetFormValue(state, "dockerfile_config", "optimization", ""); optimization != nil {
		if opt, ok := optimization.(string); ok && opt != "" {
			state.Preferences.Optimization = opt
		}
	}

	if healthCheck := GetFormValue(state, "dockerfile_config", "include_health_check", true); healthCheck != nil {
		if hc, ok := healthCheck.(bool); ok {
			state.Preferences.IncludeHealthCheck = hc
		}
	}

	if baseImage := GetFormValue(state, "dockerfile_config", "base_image", ""); baseImage != nil {
		if img, ok := baseImage.(string); ok && img != "" {
			state.Preferences.BaseImage = img
		}
	}
}

func (pm *PromptManager) applyAnalysisFormDataToPreferences(state *ConversationState) {
	// Apply optimization preference if provided
	if optimization := GetFormValue(state, "repository_analysis", "optimization", ""); optimization != nil {
		if opt, ok := optimization.(string); ok && opt != "" {
			state.Preferences.Optimization = opt
		}
	}

	// Apply skip_file_tree preference
	if skipTree := GetFormValue(state, "repository_analysis", "skip_file_tree", false); skipTree != nil {
		if skip, ok := skipTree.(bool); ok {
			state.Preferences.SkipFileTree = skip
		}
	}
}

func (pm *PromptManager) extractAnalysisPreferences(state *ConversationState, input string) {
	lower := strings.ToLower(input)

	// Extract branch preference
	if strings.Contains(lower, "branch") {
		parts := strings.Split(input, " ")
		for i, part := range parts {
			if strings.Contains(part, "branch") && i+1 < len(parts) {
				state.Context["preferred_branch"] = parts[i+1]
				break
			}
		}
	}

	// Extract optimization preference
	if strings.Contains(lower, "size") || strings.Contains(lower, "small") {
		state.Preferences.Optimization = "size"
	} else if strings.Contains(lower, "security") {
		state.Preferences.Optimization = "security"
	}
}
