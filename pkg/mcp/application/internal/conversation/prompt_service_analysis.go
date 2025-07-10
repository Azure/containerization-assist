package conversation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/genericutils"
	validationCore "github.com/Azure/container-kit/pkg/mcp/domain/security"
)

func (ps *PromptServiceImpl) startAnalysisWithFormData(ctx context.Context, state *ConversationState) *ConversationResponse {
	ps.applyAnalysisFormDataToPreferences(state)
	return ps.startAnalysis(ctx, state, state.SessionState.RepoURL)
}
func (ps *PromptServiceImpl) startAnalysis(ctx context.Context, state *ConversationState, repoURL string) *ConversationResponse {
	response := &ConversationResponse{
		Stage:  convertFromTypesStage(shared.StageAnalysis),
		Status: ResponseStatusProcessing,
	}
	ps.applyAnalysisFormDataToPreferences(state)
	params := map[string]interface{}{
		"repo_url":       repoURL,
		"session_id":     state.SessionState.SessionID,
		"skip_file_tree": state.Preferences.SkipFileTree,
	}
	if branch := GetFormValue(state, "repository_analysis", "branch", ""); branch != nil {
		if branchStr, ok := branch.(string); ok && branchStr != "" {
			params["branch"] = branchStr
		}
	}

	startTime := time.Now()
	resultStruct, err := ps.toolOrchestrator.ExecuteTool(ctx, "analyze_repository", params)
	duration := time.Since(startTime)

	toolCall := ToolCall{
		Tool:       "analyze_repository",
		Parameters: params,
		Duration:   duration,
	}

	if err != nil {
		toolCall.Error = &shared.ToolError{
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

	toolCall.Result = resultStruct
	response.ToolCalls = []ToolCall{toolCall}
	if resultStruct != nil {
		if analysis, ok := resultStruct.(map[string]interface{}); ok {
			if state.SessionState.Metadata == nil {
				state.SessionState.Metadata = make(map[string]interface{})
			}
			state.SessionState.Metadata["repo_analysis"] = analysis
			language := genericutils.MapGetWithDefault[string](analysis, "language", "")
			if language == "" {
				language = "Unknown"
			}
			framework := genericutils.MapGetWithDefault[string](analysis, "framework", "")
			entryPoints := ps.getStringSliceFromMap(analysis, "entry_points", []string{})
			var msg strings.Builder
			msg.WriteString("Analysis complete! I found:\n")
			msg.WriteString(fmt.Sprintf("- Language: %s\n", language))
			if framework != "" {
				msg.WriteString(fmt.Sprintf("- Framework: %s\n", framework))
			}
			if len(entryPoints) > 0 {
				msg.WriteString(fmt.Sprintf("- Entry point: %s\n", entryPoints[0]))
			}
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
func (ps *PromptServiceImpl) generateDockerfile(ctx context.Context, state *ConversationState) *ConversationResponse {
	response := &ConversationResponse{
		Stage:  convertFromTypesStage(shared.StageDockerfile),
		Status: ResponseStatusProcessing,
	}

	params := map[string]interface{}{
		"session_id":           state.SessionState.SessionID,
		"optimization":         state.Preferences.Optimization,
		"include_health_check": state.Preferences.IncludeHealthCheck,
	}

	if state.Preferences.BaseImage != "" {
		params["base_image"] = state.Preferences.BaseImage
	}

	startTime := time.Now()
	resultStruct, err := ps.toolOrchestrator.ExecuteTool(ctx, "generate_dockerfile", params)
	duration := time.Since(startTime)

	toolCall := ToolCall{
		Tool:       "generate_dockerfile",
		Parameters: params,
		Duration:   duration,
	}

	if err != nil {
		toolCall.Error = &shared.ToolError{
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

	toolCall.Result = resultStruct
	response.ToolCalls = []ToolCall{toolCall}
	if resultStruct != nil {
		if dockerResult, ok := resultStruct.(map[string]interface{}); ok {
			content := genericutils.MapGetWithDefault[string](dockerResult, "content", "")
			if content != "" {
				if state.SessionState.Metadata == nil {
					state.SessionState.Metadata = make(map[string]interface{})
				}
				state.SessionState.Metadata["dockerfile_content"] = content
				path := genericutils.MapGetWithDefault[string](dockerResult, "file_path", "")
				if path == "" {
					path = "Dockerfile"
				}
				state.SessionState.Metadata["dockerfile_path"] = path
				if validationData, ok := dockerResult["validation"].(map[string]interface{}); ok {

					validation := validationCore.NewSessionResult("prompt-manager", "1.0.0")
					validation.Valid = genericutils.MapGetWithDefault[bool](validationData, "valid", false)
					if errors, ok := validationData["github.com/Azure/container-kit/pkg/mcp/application/internal"].([]interface{}); ok {
						for _, err := range errors {
							if errMap, ok := err.(map[string]interface{}); ok {
								msg := genericutils.MapGetWithDefault[string](errMap, "message", "")
								if msg != "" {

									validationErr := validationCore.NewError(
										"ANALYSIS_ERROR",
										msg,
										validationCore.ErrTypeValidation,
										validationCore.SeverityHigh,
									)
									validation.AddError(validationErr)
								}
							}
						}
					}

					if warnings, ok := validationData["warnings"].([]interface{}); ok {
						for _, warn := range warnings {
							if warnMap, ok := warn.(map[string]interface{}); ok {
								msg := genericutils.MapGetWithDefault[string](warnMap, "message", "")
								if msg != "" {

									validation.AddWarning(
										"",
										msg,
										"ANALYSIS_WARNING",
										nil,
										"",
									)
								}
							}
						}
					}

					state.SessionState.Metadata["dockerfile_validation_result"] = validation
				}
				artifact := Artifact{
					Type:    "dockerfile",
					Name:    path,
					Content: content,
					Stage:   convertFromTypesStage(shared.StageDockerfile),
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
				state.SetStage(convertFromTypesStage(shared.StageBuild))
			}
		}
	}

	return response
}
func (ps *PromptServiceImpl) generateDockerfileWithFormData(ctx context.Context, state *ConversationState) *ConversationResponse {

	ps.applyFormDataToPreferences(state)
	state.Context["dockerfile_config_completed"] = true
	return ps.generateDockerfile(ctx, state)
}

func (ps *PromptServiceImpl) isFirstDockerfilePrompt(state *ConversationState) bool {
	_, presented := state.Context["dockerfile_form_presented"]
	return !presented
}

func (ps *PromptServiceImpl) hasDockerfilePreferences(state *ConversationState) bool {

	return state.Preferences.Optimization != "" ||
		state.Preferences.BaseImage != "" ||
		state.Context["dockerfile_config_completed"] != nil
}

func (ps *PromptServiceImpl) isFirstAnalysisPrompt(state *ConversationState) bool {
	_, presented := state.Context["analysis_form_presented"]
	return !presented
}

func (ps *PromptServiceImpl) hasAnalysisFormPresented(state *ConversationState) bool {
	_, presented := state.Context["analysis_form_presented"]
	return presented
}

func (ps *PromptServiceImpl) applyFormDataToPreferences(state *ConversationState) {

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

func (ps *PromptServiceImpl) applyAnalysisFormDataToPreferences(state *ConversationState) {

	if optimization := GetFormValue(state, "repository_analysis", "optimization", ""); optimization != nil {
		if opt, ok := optimization.(string); ok && opt != "" {
			state.Preferences.Optimization = opt
		}
	}
	if skipTree := GetFormValue(state, "repository_analysis", "skip_file_tree", false); skipTree != nil {
		if skip, ok := skipTree.(bool); ok {
			state.Preferences.SkipFileTree = skip
		}
	}
}

func (ps *PromptServiceImpl) extractAnalysisPreferences(state *ConversationState, input string) {
	lower := strings.ToLower(input)
	if strings.Contains(lower, "branch") {
		parts := strings.Split(input, " ")
		for i, part := range parts {
			if strings.Contains(part, "branch") && i+1 < len(parts) {
				state.Context["preferred_branch"] = parts[i+1]
				break
			}
		}
	}
	if strings.Contains(lower, "size") || strings.Contains(lower, "small") {
		state.Preferences.Optimization = "size"
	} else if strings.Contains(lower, "security") {
		state.Preferences.Optimization = "security"
	}
}
