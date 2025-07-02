package conversation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/genericutils"
	"github.com/Azure/container-kit/pkg/mcp/core/session"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"

	publicutils "github.com/Azure/container-kit/pkg/mcp/utils"
)

// getIntFromMap safely extracts an int value from a map with JSON number conversion support
func getIntFromMap(m map[string]interface{}, key string) int {
	// Try direct int first
	if val, ok := genericutils.MapGet[int](m, key); ok {
		return val
	}
	// Try float64 (common in JSON)
	if val, ok := genericutils.MapGet[float64](m, key); ok {
		return int(val)
	}
	// Try int64
	if val, ok := genericutils.MapGet[int64](m, key); ok {
		return int(val)
	}
	return 0
}

// getDockerfileBuilt checks if dockerfile has been built (from metadata)
func getDockerfileBuilt(sessionState *session.SessionState) bool {
	if sessionState.Metadata == nil {
		return false
	}
	if built, ok := sessionState.Metadata["dockerfile_built"].(bool); ok {
		return built
	}
	return false
}

// getDockerfileImageID gets the built image ID from metadata
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

// setDockerfileBuilt sets the dockerfile built status in metadata
func setDockerfileBuilt(sessionState *session.SessionState, built bool) {
	if sessionState.Metadata == nil {
		sessionState.Metadata = make(map[string]interface{})
	}
	sessionState.Metadata["dockerfile_built"] = built
}

// setDockerfileImageID sets the built image ID in metadata
func setDockerfileImageID(sessionState *session.SessionState, imageID string) {
	if sessionState.Metadata == nil {
		sessionState.Metadata = make(map[string]interface{})
	}
	sessionState.Metadata["dockerfile_image_id"] = imageID
	sessionState.Metadata["image_id"] = imageID
}

// getImageRefRegistry gets the image registry from metadata
func getImageRefRegistry(sessionState *session.SessionState) string {
	if sessionState.Metadata == nil {
		return ""
	}
	if registry, ok := sessionState.Metadata["image_registry"].(string); ok {
		return registry
	}
	return ""
}

// getImageRefTag gets the image tag from metadata
func getImageRefTag(sessionState *session.SessionState) string {
	if sessionState.Metadata == nil {
		return ""
	}
	if tag, ok := sessionState.Metadata["image_tag"].(string); ok {
		return tag
	}
	return ""
}

// setImageRefRegistry sets the image registry in metadata
func setImageRefRegistry(sessionState *session.SessionState, registry string) {
	if sessionState.Metadata == nil {
		sessionState.Metadata = make(map[string]interface{})
	}
	sessionState.Metadata["image_registry"] = registry
}

// setImageRefTag sets the image tag in metadata
func setImageRefTag(sessionState *session.SessionState, tag string) {
	if sessionState.Metadata == nil {
		sessionState.Metadata = make(map[string]interface{})
	}
	sessionState.Metadata["image_tag"] = tag
}

// handleBuildStage handles the Docker image build stage
func (pm *PromptManager) handleBuildStage(ctx context.Context, state *ConversationState, input string) *ConversationResponse {
	// Add progress indicator and stage intro
	progressPrefix := fmt.Sprintf("%s %s\n\n", getStageProgress(convertFromTypesStage(types.StageBuild)), getStageIntro(convertFromTypesStage(types.StageBuild)))

	// Check if user wants to skip build
	if strings.Contains(strings.ToLower(input), "skip") {
		state.SetStage(convertFromTypesStage(types.StagePush))
		return &ConversationResponse{
			Message: fmt.Sprintf("%sSkipping build stage. Moving to push stage...", progressPrefix),
			Stage:   convertFromTypesStage(types.StagePush),
			Status:  ResponseStatusSuccess,
		}
	}

	// Run pre-flight checks for build stage
	if !pm.hasPassedStagePreFlightChecks(state, convertFromTypesStage(types.StageBuild)) {
		checkResult, err := pm.preFlightChecker.RunStageChecks(ctx, string(convertFromTypesStage(types.StageBuild)), state.SessionState)
		if err != nil {
			return &ConversationResponse{
				Message: fmt.Sprintf("%sFailed to run pre-flight checks: %v", progressPrefix, err),
				Stage:   convertFromTypesStage(types.StageBuild),
				Status:  ResponseStatusError,
			}
		}

		if !checkResult.Passed {
			response := pm.handleFailedPreFlightChecks(ctx, state, checkResult, convertFromTypesStage(types.StageBuild))
			response.Message = fmt.Sprintf("%s%s", progressPrefix, response.Message)
			return response
		}

		// Mark pre-flight checks as passed
		pm.markStagePreFlightPassed(state, convertFromTypesStage(types.StageBuild))
	}

	// Check if we need to gather build preferences
	if !getDockerfileBuilt(state.SessionState) {
		// First, offer dry-run
		if !pm.hasRunBuildDryRun(state) {
			return pm.offerBuildDryRun(ctx, state)
		}

		// If user confirmed after dry-run, proceed with actual build
		if strings.Contains(strings.ToLower(input), "yes") || strings.Contains(strings.ToLower(input), "proceed") {
			return pm.executeBuild(ctx, state)
		}
	}

	// Build already complete, determine next action based on user preferences
	response := &ConversationResponse{
		Message: fmt.Sprintf("%sImage built successfully: %s", progressPrefix, getDockerfileImageID(state.SessionState)),
		Stage:   convertFromTypesStage(types.StageBuild),
		Status:  ResponseStatusSuccess,
	}

	// Check if user has autopilot enabled by looking at their preferences
	hasAutopilot := pm.hasAutopilotEnabled(state)

	if hasAutopilot {
		// Auto-advance to push stage
		response.WithAutoAdvance(convertFromTypesStage(types.StagePush), AutoAdvanceConfig{
			DelaySeconds:  2,
			Confidence:    0.9,
			Reason:        "Build successful, proceeding to push stage",
			CanCancel:     true,
			DefaultAction: "push",
		})
		response.Message = response.GetAutoAdvanceMessage()
	} else {
		// Manual mode: ask user for input
		state.SetStage(convertFromTypesStage(types.StagePush))
		response.Stage = convertFromTypesStage(types.StagePush)
		response.WithUserInput()
		response.Message += "\n\nWould you like to push it to a registry?"
		response.Options = []Option{
			{ID: "push", Label: "Yes, push to registry", Recommended: true},
			{ID: "skip", Label: "No, continue with local image"},
		}
	}

	return response
}

// offerBuildDryRun offers a dry-run preview of the build
func (pm *PromptManager) offerBuildDryRun(ctx context.Context, state *ConversationState) *ConversationResponse {
	response := &ConversationResponse{
		Stage:  convertFromTypesStage(types.StageBuild),
		Status: ResponseStatusProcessing,
	}

	// Run dry-run build
	params := map[string]interface{}{
		"session_id": state.SessionState.SessionID,
		"dry_run":    true,
	}

	result, err := pm.toolOrchestrator.ExecuteTool(ctx, "build_image", params)
	if err != nil {
		response.Status = ResponseStatusError
		response.Message = fmt.Sprintf("Failed to preview build: %v", err)
		return response
	}

	// Mark that we've run dry-run
	state.Context["build_dry_run_complete"] = true

	// Format preview
	resultMap, _ := result.(map[string]interface{})
	var details map[string]interface{}
	if resultField, ok := resultMap["result"]; ok {
		details, _ = resultField.(map[string]interface{})
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

// executeBuild performs the actual Docker build
func (pm *PromptManager) executeBuild(ctx context.Context, state *ConversationState) *ConversationResponse {
	response := &ConversationResponse{
		Stage:   convertFromTypesStage(types.StageBuild),
		Status:  ResponseStatusProcessing,
		Message: "Building Docker image... This may take a few minutes.",
	}

	// Prepare build parameters
	imageTag := pm.generateImageTag(state)
	params := map[string]interface{}{
		"session_id": state.SessionState.SessionID,
		"image_ref":  imageTag,
		"platform":   state.Preferences.Platform,
	}

	if len(state.Preferences.BuildArgs) > 0 {
		params["build_args"] = state.Preferences.BuildArgs
	}

	// Execute build
	startTime := time.Now()
	result, err := pm.toolOrchestrator.ExecuteTool(ctx, "build_image", params)
	duration := time.Since(startTime)

	toolCall := ToolCall{
		Tool:       "build_image",
		Parameters: params,
		Duration:   duration,
	}

	if err != nil {
		toolCall.Error = &types.ToolError{
			Type:      "build_error",
			Message:   fmt.Sprintf("build_image error: %v", err),
			Retryable: true,
			Timestamp: time.Now(),
		}
		response.ToolCalls = []ToolCall{toolCall}
		response.Status = ResponseStatusError

		// Attempt automatic fix before showing manual options
		autoFixHelper := NewAutoFixHelper(pm.conversationHandler)
		if autoFixHelper.AttemptAutoFix(ctx, response, convertFromTypesStage(types.StageBuild), err, state) {
			return response
		}

		// Fallback to original behavior if auto-fix is not available
		response.Message = fmt.Sprintf("Build failed: %v\n\nWould you like to:", err)
		response.Options = []Option{
			{ID: "retry", Label: "Retry build"},
			{ID: "logs", Label: "Show build logs"},
			{ID: "modify", Label: "Modify Dockerfile"},
		}
		return response
	}

	// Extract result data
	resultMap, _ := result.(map[string]interface{})
	if resultField, ok := resultMap["result"]; ok {
		toolCall.Result = resultField
	} else {
		toolCall.Result = result
	}
	response.ToolCalls = []ToolCall{toolCall}

	// Extract details from result
	var details map[string]interface{}
	if resultField, ok := resultMap["result"]; ok {
		details, _ = resultField.(map[string]interface{})
	} else {
		details = resultMap
	}

	// Update state with build results
	setDockerfileBuilt(state.SessionState, true)
	setDockerfileImageID(state.SessionState, imageTag)
	now := time.Now()
	if state.SessionState.Metadata == nil {
		state.SessionState.Metadata = make(map[string]interface{})
	}
	state.SessionState.Metadata["dockerfile_build_time"] = now

	// Add build artifact
	artifact := Artifact{
		Type:    "docker-image",
		Name:    "Docker Image",
		Content: imageTag,
		Stage:   convertFromTypesStage(types.StageBuild),
		Metadata: map[string]interface{}{
			"size":     details["size"],
			"layers":   details["layers"],
			"duration": duration.Seconds(),
		},
	}
	state.AddArtifact(artifact)

	// Success - move to push stage
	state.SetStage(convertFromTypesStage(types.StagePush))
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

// handlePushStage handles the Docker image push stage
func (pm *PromptManager) handlePushStage(ctx context.Context, state *ConversationState, input string) *ConversationResponse {
	// Add progress indicator and stage intro
	progressPrefix := fmt.Sprintf("%s %s\n\n", getStageProgress(convertFromTypesStage(types.StagePush)), getStageIntro(convertFromTypesStage(types.StagePush)))

	// Check for security scan request
	if strings.Contains(strings.ToLower(input), "scan") {
		response := pm.performSecurityScan(ctx, state)
		response.Message = fmt.Sprintf("%s%s", progressPrefix, response.Message)
		return response
	}

	// Check if user wants to skip push
	if strings.Contains(strings.ToLower(input), "skip") || strings.Contains(strings.ToLower(input), "local") {
		state.SetStage(convertFromTypesStage(types.StageManifests))
		return &ConversationResponse{
			Message: fmt.Sprintf("%sKeeping image local. Moving to Kubernetes manifest generation...", progressPrefix),
			Stage:   convertFromTypesStage(types.StageManifests),
			Status:  ResponseStatusSuccess,
		}
	}

	// Run pre-flight checks for push stage
	if !pm.hasPassedStagePreFlightChecks(state, convertFromTypesStage(types.StagePush)) {
		checkResult, err := pm.preFlightChecker.RunStageChecks(ctx, string(convertFromTypesStage(types.StagePush)), state.SessionState)
		if err != nil {
			return &ConversationResponse{
				Message: fmt.Sprintf("%sFailed to run pre-flight checks: %v", progressPrefix, err),
				Stage:   convertFromTypesStage(types.StagePush),
				Status:  ResponseStatusError,
			}
		}

		if !checkResult.Passed {
			response := pm.handleFailedPreFlightChecks(ctx, state, checkResult, convertFromTypesStage(types.StagePush))
			response.Message = fmt.Sprintf("%s%s", progressPrefix, response.Message)
			return response
		}

		// Mark pre-flight checks as passed
		pm.markStagePreFlightPassed(state, convertFromTypesStage(types.StagePush))
	}

	// Check if we need registry information
	registry, ok := state.Context["preferred_registry"].(string)
	if !ok || registry == "" {
		response := pm.gatherRegistryInfo(ctx, state, input)
		response.Message = fmt.Sprintf("%s%s", progressPrefix, response.Message)
		return response
	}

	// Execute push
	response := pm.executePush(ctx, state)
	response.Message = fmt.Sprintf("%s%s", progressPrefix, response.Message)
	return response
}

// gatherRegistryInfo collects registry information
func (pm *PromptManager) gatherRegistryInfo(ctx context.Context, state *ConversationState, input string) *ConversationResponse {
	// Check if input contains registry
	if strings.Contains(input, ".") || strings.Contains(input, "/") {
		state.Context["preferred_registry"] = extractRegistry(input)
		return pm.executePush(ctx, state)
	}

	return &ConversationResponse{
		Message: "Which container registry would you like to use?",
		Stage:   convertFromTypesStage(types.StagePush),
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

// executePush performs the Docker push
func (pm *PromptManager) executePush(ctx context.Context, state *ConversationState) *ConversationResponse {
	response := &ConversationResponse{
		Stage:   convertFromTypesStage(types.StagePush),
		Status:  ResponseStatusProcessing,
		Message: "Pushing image to registry...",
	}

	// Prepare push parameters
	registry, _ := state.Context["preferred_registry"].(string) //nolint:errcheck // Already validated above
	imageRef := fmt.Sprintf("%s/%s", registry, getDockerfileImageID(state.SessionState))

	params := map[string]interface{}{
		"session_id": state.SessionState.SessionID,
		"image_ref":  imageRef,
		"source_ref": getDockerfileImageID(state.SessionState),
	}

	// First try dry-run to check access
	dryResult, err := pm.toolOrchestrator.ExecuteTool(ctx, "push_image", params)
	if err != nil {
		// Log dry-run failure but continue
		pm.logger.Debug().Err(err).Msg("Dry-run push failed, proceeding with actual push")
	}
	if dryResult != nil {
		// Check if dry-run failed by examining the result
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

	// Execute actual push
	startTime := time.Now()
	result, err := pm.toolOrchestrator.ExecuteTool(ctx, "push_image", params)
	duration := time.Since(startTime)

	toolCall := ToolCall{
		Tool:       "push_image",
		Parameters: params,
		Duration:   duration,
	}

	if err != nil {
		toolCall.Error = &types.ToolError{
			Type:      "push_error",
			Message:   fmt.Sprintf("push_image error: %v", err),
			Retryable: true,
			Timestamp: time.Now(),
		}
		response.ToolCalls = []ToolCall{toolCall}
		response.Status = ResponseStatusError

		// Attempt automatic fix before showing manual options
		autoFixHelper := NewAutoFixHelper(pm.conversationHandler)
		if autoFixHelper.AttemptAutoFix(ctx, response, convertFromTypesStage(types.StagePush), err, state) {
			return response
		}

		// Fallback to original behavior if auto-fix is not available
		response.Message = fmt.Sprintf("Failed to push Docker image: %v\n\nWould you like to:", err)
		response.Options = []Option{
			{ID: "retry", Label: "Retry push"},
			{ID: "local", Label: "Skip push, keep local"},
			{ID: "registry", Label: "Change registry"},
		}
		return response
	}

	// Extract result data
	resultMap, _ := result.(map[string]interface{})
	if resultField, ok := resultMap["result"]; ok {
		toolCall.Result = resultField
	} else {
		toolCall.Result = result
	}
	response.ToolCalls = []ToolCall{toolCall}

	// Update state
	if state.SessionState.Metadata == nil {
		state.SessionState.Metadata = make(map[string]interface{})
	}
	state.SessionState.Metadata["dockerfile_pushed"] = true
	setImageRefRegistry(state.SessionState, registry)
	setImageRefTag(state.SessionState, extractTag(imageRef))

	// Success - move to manifests
	state.SetStage(convertFromTypesStage(types.StageManifests))
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
