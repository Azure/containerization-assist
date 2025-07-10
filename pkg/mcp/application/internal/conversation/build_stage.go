package conversation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/genericutils"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"

	publicutils "github.com/Azure/container-kit/pkg/mcp/application/state"
)

func getIntFromMap(m map[string]interface{}, key string) int {
	if val, ok := genericutils.MapGet[int](m, key); ok {
		return val
	}
	if val, ok := genericutils.MapGet[float64](m, key); ok {
		return int(val)
	}
	if val, ok := genericutils.MapGet[int64](m, key); ok {
		return int(val)
	}
	return 0
}

func getDockerfileBuilt(sessionState *session.SessionState) bool {
	if sessionState.Metadata == nil {
		return false
	}
	if built, ok := sessionState.Metadata["dockerfile_built"].(bool); ok {
		return built
	}
	return false
}

func getDockerfileImageID(sessionState *session.SessionState) string {
	if sessionState.Metadata == nil {
		return ""
	}
	if imageID, ok := sessionState.Metadata["dockerfile_image_id"].(string); ok {
		return imageID
	}
	if imageID, ok := sessionState.Metadata["image_id"].(string); ok {
		return imageID
	}
	return ""
}

func setDockerfileBuilt(sessionState *session.SessionState, built bool) {
	if sessionState.Metadata == nil {
		sessionState.Metadata = make(map[string]interface{})
	}
	sessionState.Metadata["dockerfile_built"] = built
}

func setDockerfileImageID(sessionState *session.SessionState, imageID string) {
	if sessionState.Metadata == nil {
		sessionState.Metadata = make(map[string]interface{})
	}
	sessionState.Metadata["dockerfile_image_id"] = imageID
	sessionState.Metadata["image_id"] = imageID
}

func getImageRefRegistry(sessionState *session.SessionState) string {
	if sessionState.Metadata == nil {
		return ""
	}
	if registry, ok := sessionState.Metadata["image_registry"].(string); ok {
		return registry
	}
	return ""
}

func getImageRefTag(sessionState *session.SessionState) string {
	if sessionState.Metadata == nil {
		return ""
	}
	if tag, ok := sessionState.Metadata["image_tag"].(string); ok {
		return tag
	}
	return ""
}

func setImageRefRegistry(sessionState *session.SessionState, registry string) {
	if sessionState.Metadata == nil {
		sessionState.Metadata = make(map[string]interface{})
	}
	sessionState.Metadata["image_registry"] = registry
}

func setImageRefTag(sessionState *session.SessionState, tag string) {
	if sessionState.Metadata == nil {
		sessionState.Metadata = make(map[string]interface{})
	}
	sessionState.Metadata["image_tag"] = tag
}

func (ps *PromptServiceImpl) handleBuildStage(ctx context.Context, state *ConversationState, input string) *ConversationResponse {
	progressPrefix := fmt.Sprintf("%s %s\n\n", getStageProgress(convertFromTypesStage(publicutils.StageBuild)), getStageIntro(convertFromTypesStage(publicutils.StageBuild)))

	if strings.Contains(strings.ToLower(input), "skip") {
		state.SetStage(convertFromTypesStage(publicutils.StagePush))
		return &ConversationResponse{
			Message: fmt.Sprintf("%sSkipping build stage. Moving to push stage...", progressPrefix),
			Stage:   convertFromTypesStage(publicutils.StagePush),
			Status:  ResponseStatusSuccess,
		}
	}

	if !ps.hasPassedStagePreFlightChecks(state, convertFromTypesStage(publicutils.StageBuild)) {
		ps.markStagePreFlightPassed(state, convertFromTypesStage(publicutils.StageBuild))
	}

	if !getDockerfileBuilt(state.SessionState) {
		if !ps.hasRunBuildDryRun(state) {
			return ps.offerBuildDryRun(ctx, state)
		}

		if strings.Contains(strings.ToLower(input), "yes") || strings.Contains(strings.ToLower(input), "proceed") {
			return ps.executeBuild(ctx, state)
		}
	}

	response := &ConversationResponse{
		Message: fmt.Sprintf("%sImage built successfully: %s", progressPrefix, getDockerfileImageID(state.SessionState)),
		Stage:   convertFromTypesStage(publicutils.StageBuild),
		Status:  ResponseStatusSuccess,
	}

	hasAutopilot := ps.hasAutopilotEnabled(state)

	if hasAutopilot {
		response.WithAutoAdvance(convertFromTypesStage(publicutils.StagePush), AutoAdvanceConfig{
			DelaySeconds:  2,
			Confidence:    0.9,
			Reason:        "Build successful, proceeding to push stage",
			CanCancel:     true,
			DefaultAction: "push",
		})
		response.Message = response.GetAutoAdvanceMessage()
	} else {
		state.SetStage(convertFromTypesStage(publicutils.StagePush))
		response.Stage = convertFromTypesStage(publicutils.StagePush)
		response.WithUserInput()
		response.Message += "\n\nWould you like to push it to a registry?"
		response.Options = []Option{
			{ID: "push", Label: "Yes, push to registry", Recommended: true},
			{ID: "skip", Label: "No, continue with local image"},
		}
	}

	return response
}

func (ps *PromptServiceImpl) offerBuildDryRun(ctx context.Context, state *ConversationState) *ConversationResponse {
	response := &ConversationResponse{
		Stage:  convertFromTypesStage(publicutils.StageBuild),
		Status: ResponseStatusProcessing,
	}

	params := map[string]interface{}{
		"session_id": state.SessionState.SessionID,
		"dry_run":    true,
	}

	result, err := ps.toolOrchestrator.ExecuteTool(ctx, "build_image", params)
	if err != nil {
		response.Status = ResponseStatusError
		response.Message = fmt.Sprintf("Failed to preview build: %v", err)
		return response
	}

	state.Context["build_dry_run_complete"] = true

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		resultMap = make(map[string]interface{})
	}
	var details map[string]interface{}
	if resultField, exists := resultMap["result"]; exists {
		if detailsMap, ok := resultField.(map[string]interface{}); ok {
			details = detailsMap
		} else {
			details = resultMap
		}
	} else {
		details = resultMap
	}
	layers := getIntFromMap(details, "estimated_layers")
	size := int64(getIntFromMap(details, "estimated_size"))
	baseImage := genericutils.MapGetWithDefault[string](details, "base_image", "")

	response.Message = fmt.Sprintf(
		"Build Preview:\n"+
			"- Base image: %s\n"+
			"- Estimated layers: %d\n"+
			"- Estimated size: %s\n\n"+
			"This may take a few minutes. Proceed with the build?",
		baseImage, layers, publicutils.FormatBytes(size))

	response.Status = ResponseStatusSuccess
	response.Options = []Option{
		{ID: "yes", Label: "Yes, build the image", Recommended: true},
		{ID: "modify", Label: "Modify Dockerfile first"},
		{ID: "skip", Label: "Skip build"},
	}

	return response
}

func (ps *PromptServiceImpl) executeBuild(ctx context.Context, state *ConversationState) *ConversationResponse {
	response := &ConversationResponse{
		Stage:   convertFromTypesStage(publicutils.StageBuild),
		Status:  ResponseStatusProcessing,
		Message: "Building Docker image... This may take a few minutes.",
	}

	imageTag := ps.generateImageTag(state)
	params := map[string]interface{}{
		"session_id": state.SessionState.SessionID,
		"image_ref":  imageTag,
		"platform":   state.Preferences.Platform,
	}

	if len(state.Preferences.BuildArgs) > 0 {
		params["build_args"] = state.Preferences.BuildArgs
	}

	startTime := time.Now()
	result, err := ps.toolOrchestrator.ExecuteTool(ctx, "build_image", params)
	duration := time.Since(startTime)

	toolCall := ToolCall{
		Tool:       "build_image",
		Parameters: params,
		Duration:   duration,
	}

	if err != nil {
		toolCall.Error = &publicutils.ToolError{
			Type:      "build_error",
			Message:   fmt.Sprintf("build_image error: %v", err),
			Retryable: true,
			Timestamp: time.Now(),
		}
		response.ToolCalls = []ToolCall{toolCall}
		response.Status = ResponseStatusError

		autoFixHelper := NewAutoFixHelper(ps.conversationHandler)
		if autoFixHelper.AttemptAutoFix(ctx, response, convertFromTypesStage(publicutils.StageBuild), err, state) {
			return response
		}

		response.Message = fmt.Sprintf("Build failed: %v\n\nWould you like to:", err)
		response.Options = []Option{
			{ID: "retry", Label: "Retry build"},
			{ID: "logs", Label: "Show build logs"},
			{ID: "modify", Label: "Modify Dockerfile"},
		}
		return response
	}

	resultMap, _ := result.(map[string]interface{})
	if resultField, ok := resultMap["result"]; ok {
		toolCall.Result = resultField
	} else {
		toolCall.Result = result
	}
	response.ToolCalls = []ToolCall{toolCall}

	var details map[string]interface{}
	if resultField, ok := resultMap["result"]; ok {
		details, _ = resultField.(map[string]interface{})
	} else {
		details = resultMap
	}

	setDockerfileBuilt(state.SessionState, true)
	setDockerfileImageID(state.SessionState, imageTag)
	now := time.Now()
	if state.SessionState.Metadata == nil {
		state.SessionState.Metadata = make(map[string]interface{})
	}
	state.SessionState.Metadata["dockerfile_build_time"] = now

	artifact := Artifact{
		Type:    "docker-image",
		Name:    "Docker Image",
		Content: imageTag,
		Stage:   convertFromTypesStage(publicutils.StageBuild),
		Metadata: map[string]interface{}{
			"size":     details["size"],
			"layers":   details["layers"],
			"duration": duration.Seconds(),
		},
	}
	state.AddArtifact(artifact)

	state.SetStage(convertFromTypesStage(publicutils.StagePush))
	response.Status = ResponseStatusSuccess
	response.Message = fmt.Sprintf(
		"✅ Image built successfully!\n\n"+
			"- Tag: %s\n"+
			"- Size: %s\n"+
			"- Build time: %s\n\n"+
			"Would you like to push this image to a registry?",
		imageTag,
		publicutils.FormatBytes(int64(getIntFromMap(details, "size"))),
		duration.Round(time.Second))

	response.Options = []Option{
		{ID: "push", Label: "Push to registry", Recommended: true},
		{ID: "local", Label: "Keep local only"},
		{ID: "scan", Label: "Security scan first"},
	}

	return response
}

func (ps *PromptServiceImpl) handlePushStage(ctx context.Context, state *ConversationState, input string) *ConversationResponse {
	progressPrefix := fmt.Sprintf("%s %s\n\n", getStageProgress(convertFromTypesStage(publicutils.StagePush)), getStageIntro(convertFromTypesStage(publicutils.StagePush)))

	if strings.Contains(strings.ToLower(input), "scan") {
		response := ps.performSecurityScan(ctx, state)
		response.Message = fmt.Sprintf("%s%s", progressPrefix, response.Message)
		return response
	}

	if strings.Contains(strings.ToLower(input), "skip") || strings.Contains(strings.ToLower(input), "local") {
		state.SetStage(convertFromTypesStage(publicutils.StageManifests))
		return &ConversationResponse{
			Message: fmt.Sprintf("%sKeeping image local. Moving to Kubernetes manifest generation...", progressPrefix),
			Stage:   convertFromTypesStage(publicutils.StageManifests),
			Status:  ResponseStatusSuccess,
		}
	}

	if !ps.hasPassedStagePreFlightChecks(state, convertFromTypesStage(publicutils.StagePush)) {
		ps.markStagePreFlightPassed(state, convertFromTypesStage(publicutils.StagePush))
	}

	registry, ok := state.Context["preferred_registry"].(string)
	if !ok || registry == "" {
		response := ps.gatherRegistryInfo(ctx, state, input)
		response.Message = fmt.Sprintf("%s%s", progressPrefix, response.Message)
		return response
	}

	response := ps.executePush(ctx, state)
	response.Message = fmt.Sprintf("%s%s", progressPrefix, response.Message)
	return response
}

func (ps *PromptServiceImpl) gatherRegistryInfo(ctx context.Context, state *ConversationState, input string) *ConversationResponse {
	if strings.Contains(input, ".") || strings.Contains(input, "/") {
		state.Context["preferred_registry"] = extractRegistry(input)
		return ps.executePush(ctx, state)
	}

	return &ConversationResponse{
		Message: "Which container registry would you like to use?",
		Stage:   convertFromTypesStage(publicutils.StagePush),
		Status:  ResponseStatusWaitingInput,
		Options: []Option{
			{ID: "dockerhub", Label: "Docker Hub (docker.io)"},
			{ID: "gcr", Label: "Google Container Registry (gcr.io)"},
			{ID: "acr", Label: "Azure Container Registry"},
			{ID: "ecr", Label: "Amazon ECR"},
			{ID: "custom", Label: "Custom registry"},
		},
	}
}

func (ps *PromptServiceImpl) executePush(ctx context.Context, state *ConversationState) *ConversationResponse {
	response := &ConversationResponse{
		Stage:   convertFromTypesStage(publicutils.StagePush),
		Status:  ResponseStatusProcessing,
		Message: "Pushing image to registry...",
	}

	registry, _ := state.Context["preferred_registry"].(string)
	imageRef := fmt.Sprintf("%s/%s", registry, getDockerfileImageID(state.SessionState))

	params := map[string]interface{}{
		"session_id": state.SessionState.SessionID,
		"image_ref":  imageRef,
		"source_ref": getDockerfileImageID(state.SessionState),
	}

	dryResult, err := ps.toolOrchestrator.ExecuteTool(ctx, "push_image", params)
	if err != nil {
		ps.logger.Debug("Dry-run push failed, proceeding with actual push", "error", err)
	}
	if dryResult != nil {
		var dryResultMap map[string]interface{}
		if resultMap, ok := dryResult.(map[string]interface{}); ok {
			if resultField, ok := resultMap["result"]; ok {
				dryResultMap, _ = resultField.(map[string]interface{})
			} else {
				dryResultMap = resultMap
			}
		}
		if dryResultMap != nil {
			if success, ok := dryResultMap["success"].(bool); ok && !success {
				errorMsg := "unknown error"
				if errStr, ok := dryResultMap["error"].(string); ok {
					errorMsg = errStr
				}
				response.Status = ResponseStatusError
				response.Message = fmt.Sprintf("Registry access check failed: %s\n\nPlease authenticate with:\ndocker login %s",
					errorMsg, registry)
				response.Options = []Option{
					{ID: "retry", Label: "I've authenticated, retry"},
					{ID: "skip", Label: "Skip push"},
				}
				return response
			}
		}
	}

	startTime := time.Now()
	result, err := ps.toolOrchestrator.ExecuteTool(ctx, "push_image", params)
	duration := time.Since(startTime)

	toolCall := ToolCall{
		Tool:       "push_image",
		Parameters: params,
		Duration:   duration,
	}

	if err != nil {
		toolCall.Error = &publicutils.ToolError{
			Type:      "push_error",
			Message:   fmt.Sprintf("push_image error: %v", err),
			Retryable: true,
			Timestamp: time.Now(),
		}
		response.ToolCalls = []ToolCall{toolCall}
		response.Status = ResponseStatusError

		autoFixHelper := NewAutoFixHelper(ps.conversationHandler)
		if autoFixHelper.AttemptAutoFix(ctx, response, convertFromTypesStage(publicutils.StagePush), err, state) {
			return response
		}

		response.Message = fmt.Sprintf("Failed to push Docker image: %v\n\nWould you like to:", err)
		response.Options = []Option{
			{ID: "retry", Label: "Retry push"},
			{ID: "local", Label: "Skip push, keep local"},
			{ID: "registry", Label: "Change registry"},
		}
		return response
	}

	resultMap, _ := result.(map[string]interface{})
	if resultField, ok := resultMap["result"]; ok {
		toolCall.Result = resultField
	} else {
		toolCall.Result = result
	}
	response.ToolCalls = []ToolCall{toolCall}

	if state.SessionState.Metadata == nil {
		state.SessionState.Metadata = make(map[string]interface{})
	}
	state.SessionState.Metadata["dockerfile_pushed"] = true
	setImageRefRegistry(state.SessionState, registry)
	setImageRefTag(state.SessionState, extractTag(imageRef))

	state.SetStage(convertFromTypesStage(publicutils.StageManifests))
	response.Status = ResponseStatusSuccess
	response.Message = fmt.Sprintf(
		"✅ Image pushed successfully!\n\n"+
			"- Registry: %s\n"+
			"- Image: %s\n"+
			"- Push time: %s\n\n"+
			"Now let's create Kubernetes manifests for deployment.",
		registry, imageRef, duration.Round(time.Second))

	return response
}
