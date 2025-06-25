package conversation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	publicutils "github.com/Azure/container-copilot/pkg/mcp/utils"
)

// handleBuildStage handles the Docker image build stage
func (pm *PromptManager) handleBuildStage(ctx context.Context, state *ConversationState, input string) *ConversationResponse {
	// Add progress indicator and stage intro
	progressPrefix := fmt.Sprintf("%s %s\n\n", getStageProgress(types.StageBuild), getStageIntro(types.StageBuild))

	// Check if user wants to skip build
	if strings.Contains(strings.ToLower(input), "skip") {
		state.SetStage(types.StagePush)
		return &ConversationResponse{
			Message: fmt.Sprintf("%sSkipping build stage. Moving to push stage...", progressPrefix),
			Stage:   types.StagePush,
			Status:  ResponseStatusSuccess,
		}
	}

	// Run pre-flight checks for build stage
	if !pm.hasPassedStagePreFlightChecks(state, types.StageBuild) {
		checkResult, err := pm.preFlightChecker.RunStageChecks(ctx, types.StageBuild, state.SessionState)
		if err != nil {
			return &ConversationResponse{
				Message: fmt.Sprintf("%sFailed to run pre-flight checks: %v", progressPrefix, err),
				Stage:   types.StageBuild,
				Status:  ResponseStatusError,
			}
		}

		if !checkResult.Passed {
			response := pm.handleFailedPreFlightChecks(ctx, state, checkResult, types.StageBuild)
			response.Message = fmt.Sprintf("%s%s", progressPrefix, response.Message)
			return response
		}

		// Mark pre-flight checks as passed
		pm.markStagePreFlightPassed(state, types.StageBuild)
	}

	// Check if we need to gather build preferences
	if !state.Dockerfile.Built {
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
		Message: fmt.Sprintf("%sImage built successfully: %s", progressPrefix, state.Dockerfile.ImageID),
		Stage:   types.StageBuild,
		Status:  ResponseStatusSuccess,
	}

	// Check if user has autopilot enabled by looking at their preferences
	hasAutopilot := pm.hasAutopilotEnabled(state)

	if hasAutopilot {
		// Auto-advance to push stage
		response.WithAutoAdvance(types.StagePush, AutoAdvanceConfig{
			DelaySeconds:  2,
			Confidence:    0.9,
			Reason:        "Build successful, proceeding to push stage",
			CanCancel:     true,
			DefaultAction: "push",
		})
		response.Message = response.GetAutoAdvanceMessage()
	} else {
		// Manual mode: ask user for input
		state.SetStage(types.StagePush)
		response.Stage = types.StagePush
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
		Stage:  types.StageBuild,
		Status: ResponseStatusProcessing,
	}

	// Run dry-run build
	params := map[string]interface{}{
		"session_id": state.SessionID,
		"dry_run":    true,
	}

	result, err := pm.toolOrchestrator.ExecuteTool(ctx, "build_image", params, state.SessionState.SessionID)
	if err != nil {
		response.Status = ResponseStatusError
		response.Message = fmt.Sprintf("Failed to preview build: %v", err)
		return response
	}

	// Mark that we've run dry-run
	state.Context["build_dry_run_complete"] = true

	// Format preview
	details, _ := result.(map[string]interface{})
	layers := publicutils.GetIntFromMap(details, "estimated_layers")
	size := int64(publicutils.GetIntFromMap(details, "estimated_size"))
	baseImage := publicutils.GetStringFromMap(details, "base_image")

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
		Stage:   types.StageBuild,
		Status:  ResponseStatusProcessing,
		Message: "Building Docker image... This may take a few minutes.",
	}

	// Prepare build parameters
	imageTag := pm.generateImageTag(state)
	params := map[string]interface{}{
		"session_id": state.SessionID,
		"image_ref":  imageTag,
		"platform":   state.Preferences.Platform,
	}

	if len(state.Preferences.BuildArgs) > 0 {
		params["build_args"] = state.Preferences.BuildArgs
	}

	// Execute build
	startTime := time.Now()
	result, err := pm.toolOrchestrator.ExecuteTool(ctx, "build_image", params, state.SessionState.SessionID)
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
		response.Message = fmt.Sprintf("Build failed: %v\n\nWould you like to:", err)
		response.Options = []Option{
			{ID: "retry", Label: "Retry build"},
			{ID: "logs", Label: "Show build logs"},
			{ID: "modify", Label: "Modify Dockerfile"},
		}
		return response
	}

	toolCall.Result = result
	response.ToolCalls = []ToolCall{toolCall}

	// Extract details from result
	details, _ := result.(map[string]interface{})

	// Update state with build results
	state.Dockerfile.Built = true
	state.Dockerfile.ImageID = imageTag
	now := time.Now()
	state.Dockerfile.BuildTime = &now

	// Add build artifact
	artifact := Artifact{
		Type:    "docker-image",
		Name:    "Docker Image",
		Content: imageTag,
		Stage:   types.StageBuild,
		Metadata: map[string]interface{}{
			"size":     details["size"],
			"layers":   details["layers"],
			"duration": duration.Seconds(),
		},
	}
	state.AddArtifact(artifact)

	// Success - move to push stage
	state.SetStage(types.StagePush)
	response.Status = ResponseStatusSuccess
	response.Message = fmt.Sprintf(
		"✅ Image built successfully!\n\n"+
			"- Tag: %s\n"+
			"- Size: %s\n"+
			"- Build time: %s\n\n"+
			"Would you like to push this image to a registry?",
		imageTag,
		publicutils.FormatBytes(int64(publicutils.GetIntFromMap(details, "size"))),
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
	progressPrefix := fmt.Sprintf("%s %s\n\n", getStageProgress(types.StagePush), getStageIntro(types.StagePush))

	// Check for security scan request
	if strings.Contains(strings.ToLower(input), "scan") {
		response := pm.performSecurityScan(ctx, state)
		response.Message = fmt.Sprintf("%s%s", progressPrefix, response.Message)
		return response
	}

	// Check if user wants to skip push
	if strings.Contains(strings.ToLower(input), "skip") || strings.Contains(strings.ToLower(input), "local") {
		state.SetStage(types.StageManifests)
		return &ConversationResponse{
			Message: fmt.Sprintf("%sKeeping image local. Moving to Kubernetes manifest generation...", progressPrefix),
			Stage:   types.StageManifests,
			Status:  ResponseStatusSuccess,
		}
	}

	// Run pre-flight checks for push stage
	if !pm.hasPassedStagePreFlightChecks(state, types.StagePush) {
		checkResult, err := pm.preFlightChecker.RunStageChecks(ctx, types.StagePush, state.SessionState)
		if err != nil {
			return &ConversationResponse{
				Message: fmt.Sprintf("%sFailed to run pre-flight checks: %v", progressPrefix, err),
				Stage:   types.StagePush,
				Status:  ResponseStatusError,
			}
		}

		if !checkResult.Passed {
			response := pm.handleFailedPreFlightChecks(ctx, state, checkResult, types.StagePush)
			response.Message = fmt.Sprintf("%s%s", progressPrefix, response.Message)
			return response
		}

		// Mark pre-flight checks as passed
		pm.markStagePreFlightPassed(state, types.StagePush)
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
		Stage:   types.StagePush,
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
		Stage:   types.StagePush,
		Status:  ResponseStatusProcessing,
		Message: "Pushing image to registry...",
	}

	// Prepare push parameters
	registry, _ := state.Context["preferred_registry"].(string) //nolint:errcheck // Already validated above
	imageRef := fmt.Sprintf("%s/%s", registry, state.Dockerfile.ImageID)

	params := map[string]interface{}{
		"session_id": state.SessionID,
		"image_ref":  imageRef,
		"source_ref": state.Dockerfile.ImageID,
	}

	// First try dry-run to check access
	dryResult, err := pm.toolOrchestrator.ExecuteTool(ctx, "push_image", params, state.SessionState.SessionID)
	if err != nil {
		// Log dry-run failure but continue
		pm.logger.Debug().Err(err).Msg("Dry-run push failed, proceeding with actual push")
	}
	if dryResult != nil {
		// Check if dry-run failed by examining the result
		if dryResultMap, ok := dryResult.(map[string]interface{}); ok {
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
	result, err := pm.toolOrchestrator.ExecuteTool(ctx, "push_image", params, state.SessionState.SessionID)
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
		response.Message = fmt.Sprintf("Failed to push Docker image: %v", err)
		return response
	}

	toolCall.Result = result
	response.ToolCalls = []ToolCall{toolCall}

	// Update state
	state.Dockerfile.Pushed = true
	state.ImageRef.Registry = registry
	state.ImageRef.Tag = extractTag(imageRef)

	// Success - move to manifests
	state.SetStage(types.StageManifests)
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
