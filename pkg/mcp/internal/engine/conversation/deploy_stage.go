package conversation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	publicutils "github.com/Azure/container-copilot/pkg/mcp/utils"
)

// handleManifestsStage handles Kubernetes manifest generation
func (pm *PromptManager) handleManifestsStage(ctx context.Context, state *ConversationState, input string) *ConversationResponse {
	// Add progress indicator and stage intro
	progressPrefix := fmt.Sprintf("%s %s\n\n", getStageProgress(types.StageManifests), getStageIntro(types.StageManifests))

	// Gather manifest preferences if not set
	appName, _ := state.Context["app_name"].(string) //nolint:errcheck // Will prompt if empty
	if appName == "" {
		response := pm.gatherManifestPreferences(ctx, state, input)
		response.Message = fmt.Sprintf("%s%s", progressPrefix, response.Message)
		return response
	}

	// Check if manifests already generated
	if len(state.K8sManifests) > 0 {
		response := pm.reviewManifests(ctx, state, input)
		response.Message = fmt.Sprintf("%s%s", progressPrefix, response.Message)
		return response
	}

	// Generate manifests
	response := pm.generateManifests(ctx, state)
	response.Message = fmt.Sprintf("%s%s", progressPrefix, response.Message)
	return response
}

// gatherManifestPreferences collects Kubernetes deployment preferences
func (pm *PromptManager) gatherManifestPreferences(ctx context.Context, state *ConversationState, input string) *ConversationResponse {
	// Create decision point for app configuration
	decision := &DecisionPoint{
		ID:       "k8s-config",
		Stage:    types.StageManifests,
		Question: "Let's configure your Kubernetes deployment. What should we name the application?",
		Required: true,
	}
	state.SetPendingDecision(decision)

	// If input contains app name, extract it
	if input != "" && !strings.Contains(input, " ") {
		state.Context["app_name"] = strings.ToLower(input)
		state.ResolvePendingDecision(Decision{
			DecisionID:  decision.ID,
			CustomValue: input,
			Timestamp:   time.Now(),
		})

		// Ask for next preference
		return &ConversationResponse{
			Message: fmt.Sprintf("App name set to '%s'. How many replicas would you like?", state.Context["app_name"]),
			Stage:   types.StageManifests,
			Status:  ResponseStatusWaitingInput,
			Options: []Option{
				{ID: "1", Label: "1 replica (development)"},
				{ID: "3", Label: "3 replicas (production)", Recommended: true},
				{ID: "custom", Label: "Custom number"},
			},
		}
	}

	// Suggest app name based on repo
	suggestedName := pm.suggestAppName(state)

	return &ConversationResponse{
		Message: decision.Question + fmt.Sprintf("\n\nSuggested: %s", suggestedName),
		Stage:   types.StageManifests,
		Status:  ResponseStatusWaitingInput,
	}
}

// generateManifests creates Kubernetes manifests
func (pm *PromptManager) generateManifests(ctx context.Context, state *ConversationState) *ConversationResponse {
	response := &ConversationResponse{
		Stage:   types.StageManifests,
		Status:  ResponseStatusProcessing,
		Message: "Generating Kubernetes manifests...",
	}

	// Determine image reference
	imageRef := state.Dockerfile.ImageID
	if state.Dockerfile.Pushed {
		imageRef = fmt.Sprintf("%s/%s", state.ImageRef.Registry, state.Dockerfile.ImageID)
	}

	params := map[string]interface{}{
		"session_id":    state.SessionID,
		"app_name":      state.Context["app_name"],
		"namespace":     state.Preferences.Namespace,
		"image_ref":     imageRef,
		"replicas":      state.Preferences.Replicas,
		"service_type":  state.Preferences.ServiceType,
		"generate_only": true, // Don't deploy yet
	}

	// Add resource limits if specified
	if state.Preferences.ResourceLimits.CPULimit != "" || state.Preferences.ResourceLimits.MemoryLimit != "" {
		params["resources"] = map[string]interface{}{
			"limits": map[string]string{
				"cpu":    state.Preferences.ResourceLimits.CPULimit,
				"memory": state.Preferences.ResourceLimits.MemoryLimit,
			},
			"requests": map[string]string{
				"cpu":    state.Preferences.ResourceLimits.CPURequest,
				"memory": state.Preferences.ResourceLimits.MemoryRequest,
			},
		}
	}

	// Add environment variables from context
	if envVars, ok := state.Context["environment_vars"].(map[string]string); ok && len(envVars) > 0 {
		params["env_vars"] = envVars
	}

	startTime := time.Now()
	result, err := pm.toolOrchestrator.ExecuteTool(ctx, "generate_manifests", params, state.SessionState.SessionID)
	duration := time.Since(startTime)

	toolCall := ToolCall{
		Tool:       "generate_manifests",
		Parameters: params,
		Duration:   duration,
	}

	if err != nil {
		toolCall.Error = &types.ToolError{
			Type:      "generation_error",
			Message:   fmt.Sprintf("generate_manifests error: %v", err),
			Retryable: true,
			Timestamp: time.Now(),
		}
		response.ToolCalls = []ToolCall{toolCall}
		response.Status = ResponseStatusError
		response.Message = fmt.Sprintf("Failed to generate Kubernetes manifests: %v", err)
		return response
	}

	toolCall.Result = result
	response.ToolCalls = []ToolCall{toolCall}

	// Parse manifests from result
	if resultData, ok := result.Result.(map[string]interface{}); ok {
		if manifests, ok := resultData["manifests"].(map[string]interface{}); ok {
			for name, content := range manifests {
			contentStr, ok := content.(string)
			if !ok {
				continue // Skip invalid content
			}
			manifest := types.K8sManifest{
				Name:    name,
				Content: contentStr,
				Kind:    extractKind(contentStr),
			}
			state.K8sManifests[name] = manifest

			// Add as artifact
			artifact := Artifact{
				Type:    "k8s-manifest",
				Name:    fmt.Sprintf("%s (%s)", name, manifest.Kind),
				Content: manifest.Content,
				Stage:   types.StageManifests,
			}
			state.AddArtifact(artifact)
		}
		}
	}

	// Format response with manifest summary
	response.Status = ResponseStatusSuccess
	response.Message = pm.formatManifestSummary(state.K8sManifests)
	response.Options = []Option{
		{ID: "deploy", Label: "Deploy to Kubernetes", Recommended: true},
		{ID: "review", Label: "Show full manifests"},
		{ID: "modify", Label: "Modify configuration"},
		{ID: "dry-run", Label: "Preview deployment (dry-run)"},
	}

	return response
}

// handleDeploymentStage handles Kubernetes deployment
func (pm *PromptManager) handleDeploymentStage(ctx context.Context, state *ConversationState, input string) *ConversationResponse {
	// Add progress indicator and stage intro
	progressPrefix := fmt.Sprintf("%s %s\n\n", getStageProgress(types.StageDeployment), getStageIntro(types.StageDeployment))

	// Check for retry request
	if strings.Contains(strings.ToLower(input), "retry") {
		response := pm.handleDeploymentRetry(ctx, state)
		response.Message = fmt.Sprintf("%s%s", progressPrefix, response.Message)
		return response
	}

	// Check for dry-run request
	if strings.Contains(strings.ToLower(input), "dry") || strings.Contains(strings.ToLower(input), "preview") {
		response := pm.deploymentDryRun(ctx, state)
		response.Message = fmt.Sprintf("%s%s", progressPrefix, response.Message)
		return response
	}

	// Check for logs request (from previous failure)
	if strings.Contains(strings.ToLower(input), "logs") {
		response := pm.showDeploymentLogs(ctx, state)
		response.Message = fmt.Sprintf("%s%s", progressPrefix, response.Message)
		return response
	}

	// Execute deployment
	response := pm.executeDeployment(ctx, state)
	response.Message = fmt.Sprintf("%s%s", progressPrefix, response.Message)
	return response
}

// deploymentDryRun performs a dry-run deployment
func (pm *PromptManager) deploymentDryRun(ctx context.Context, state *ConversationState) *ConversationResponse {
	response := &ConversationResponse{
		Stage:   types.StageDeployment,
		Status:  ResponseStatusProcessing,
		Message: "Running deployment preview (dry-run)...",
	}

	// Determine image reference
	imageRef := state.Dockerfile.ImageID
	if state.Dockerfile.Pushed {
		imageRef = fmt.Sprintf("%s/%s", state.ImageRef.Registry, state.Dockerfile.ImageID)
	}

	params := map[string]interface{}{
		"session_id": state.SessionID,
		"app_name":   state.Context["app_name"],
		"namespace":  state.Preferences.Namespace,
		"image_ref":  imageRef,
		"dry_run":    true,
	}

	result, err := pm.toolOrchestrator.ExecuteTool(ctx, "deploy_kubernetes_atomic", params, state.SessionState.SessionID)
	if err != nil {
		response.Status = ResponseStatusError
		response.Message = fmt.Sprintf("Dry-run failed: %v", err)
		return response
	}

	// Extract the dry-run preview from the result
	if toolResult, ok := result.Result.(map[string]interface{}); ok {
		dryRunPreview := publicutils.GetStringFromMap(toolResult, "dry_run_preview")
		if dryRunPreview == "" {
			dryRunPreview = "No changes detected - resources are already up to date"
		}

		// Show kubectl diff preview
		response.Status = ResponseStatusSuccess
		response.Message = fmt.Sprintf(
			"Deployment Preview (dry-run):\n\n```diff\n%s\n```\n\n"+
				"This shows what would change. Proceed with actual deployment?",
			dryRunPreview)
	} else {
		response.Status = ResponseStatusSuccess
		response.Message = "Dry-run completed but preview not available. Proceed with actual deployment?"
	}

	response.Options = []Option{
		{ID: "deploy", Label: "Yes, deploy", Recommended: true},
		{ID: "cancel", Label: "No, cancel"},
	}

	return response
}

// executeDeployment performs the actual Kubernetes deployment
func (pm *PromptManager) executeDeployment(ctx context.Context, state *ConversationState) *ConversationResponse {
	response := &ConversationResponse{
		Stage:   types.StageDeployment,
		Status:  ResponseStatusProcessing,
		Message: "Deploying to Kubernetes cluster...",
	}

	// Determine image reference
	imageRef := state.Dockerfile.ImageID
	if state.Dockerfile.Pushed {
		imageRef = fmt.Sprintf("%s/%s", state.ImageRef.Registry, state.Dockerfile.ImageID)
	}

	params := map[string]interface{}{
		"session_id":     state.SessionID,
		"app_name":       state.Context["app_name"],
		"namespace":      state.Preferences.Namespace,
		"image_ref":      imageRef,
		"wait_for_ready": true, // Default to waiting for readiness
		"timeout":        300,  // 5 minutes
	}

	startTime := time.Now()
	result, err := pm.toolOrchestrator.ExecuteTool(ctx, "deploy_kubernetes_atomic", params, state.SessionState.SessionID)
	duration := time.Since(startTime)

	toolCall := ToolCall{
		Tool:       "deploy_kubernetes_atomic",
		Parameters: params,
		Duration:   duration,
	}

	if err != nil {
		toolCall.Error = &types.ToolError{
			Type:      "deployment_error",
			Message:   fmt.Sprintf("deploy_kubernetes_atomic error: %v", err),
			Retryable: true,
			Timestamp: time.Now(),
		}
		response.ToolCalls = []ToolCall{toolCall}
		response.Status = ResponseStatusError

		// Check if rollback is available
		if state.LastKnownGood != nil && state.Preferences.AutoRollback {
			response.Message = fmt.Sprintf(
				"Deployment failed: %v\n\n"+
					"Auto-rollback is available. What would you like to do?",
				err)
			response.Options = []Option{
				{ID: "rollback", Label: "Rollback to previous version", Recommended: true},
				{ID: "logs", Label: "Show pod logs"},
				{ID: "retry", Label: "Retry deployment"},
			}
		} else {
			response.Message = fmt.Sprintf("Deployment failed: %v", err)
			response.Options = []Option{
				{ID: "logs", Label: "Show pod logs"},
				{ID: "retry", Label: "Retry deployment"},
				{ID: "modify", Label: "Modify manifests"},
			}
		}

		return response
	}

	toolCall.Result = result
	response.ToolCalls = []ToolCall{toolCall}

	// Mark manifests as deployed
	for name, manifest := range state.K8sManifests {
		manifest.Applied = true
		manifest.Status = "deployed"
		state.K8sManifests[name] = manifest
	}

	// Check health if requested
	waitForReady, _ := state.Context["wait_for_ready"].(bool)   //nolint:errcheck // Defaults to true
	if waitForReady || state.Context["wait_for_ready"] == nil { // Default to true
		return pm.checkDeploymentHealth(ctx, state, result)
	}

	// Success - move to completed
	state.SetStage(types.StageCompleted)
	response.Status = ResponseStatusSuccess
	response.Message = pm.formatDeploymentSuccess(state, duration)

	return response
}

// checkDeploymentHealth verifies deployment health
func (pm *PromptManager) checkDeploymentHealth(ctx context.Context, state *ConversationState, deployResult *ToolResult) *ConversationResponse {
	response := &ConversationResponse{
		Stage:   types.StageDeployment,
		Status:  ResponseStatusProcessing,
		Message: "Checking deployment health...",
	}

	params := map[string]interface{}{
		"session_id": state.SessionID,
		"app_name":   state.Context["app_name"],
		"namespace":  state.Preferences.Namespace,
		"timeout":    60, // 1 minute for health check
	}

	_, err := pm.toolOrchestrator.ExecuteTool(ctx, "check_health_atomic", params, state.SessionState.SessionID)
	if err != nil {
		response.Status = ResponseStatusWarning
		response.Message = fmt.Sprintf(
			"⚠️ Deployment succeeded but health check failed: %v\n\n"+
				"The pods may still be starting up. You can:",
			err)
		response.Options = []Option{
			{ID: "wait", Label: "Wait and check again"},
			{ID: "logs", Label: "Show pod logs"},
			{ID: "continue", Label: "Continue anyway"},
		}
		return response
	}

	// Health check passed
	state.SetStage(types.StageCompleted)
	response.Status = ResponseStatusSuccess
	response.Message = fmt.Sprintf(
		"✅ Deployment successful and healthy!\n\n"+
			"Your application is now running:\n"+
			"- Namespace: %s\n"+
			"- Replicas: %d (all healthy)\n"+
			"- Service: %s\n\n"+
			"You can access your application using:\n"+
			"kubectl port-forward -n %s svc/%s 8080:80",
		state.Preferences.Namespace,
		state.Preferences.Replicas,
		fmt.Sprintf("%s-service", state.Context["app_name"]),
		state.Preferences.Namespace,
		fmt.Sprintf("%s-service", state.Context["app_name"]))

	return response
}

// handleDeploymentRetry handles deployment retry requests
func (pm *PromptManager) handleDeploymentRetry(ctx context.Context, state *ConversationState) *ConversationResponse {
	// Check retry count
	retryCount := 0
	if count, ok := state.Context["deployment_retry_count"].(int); ok {
		retryCount = count
	}

	if retryCount >= 3 {
		return &ConversationResponse{
			Message: "Maximum retry attempts (3) reached. Consider:\n" +
				"- Checking your Kubernetes cluster connectivity\n" +
				"- Reviewing the manifest configuration\n" +
				"- Checking if the image exists and is accessible",
			Stage:  types.StageDeployment,
			Status: ResponseStatusError,
			Options: []Option{
				{ID: "modify", Label: "Modify manifests"},
				{ID: "rebuild", Label: "Rebuild image"},
			},
		}
	}

	// Increment retry count
	state.Context["deployment_retry_count"] = retryCount + 1

	// Retry deployment with exponential backoff
	delay := time.Duration(retryCount+1) * 2 * time.Second
	time.Sleep(delay)

	return pm.executeDeployment(ctx, state)
}
