package conversation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/genericutils"
	"github.com/Azure/container-kit/pkg/mcp"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
)

// Helper functions for accessing metadata fields

// getK8sManifestsFromMetadata checks if k8s manifests exist in metadata
func getK8sManifestsFromMetadata(sessionState *mcp.SessionState) map[string]interface{} {
	if sessionState.Metadata == nil {
		return nil
	}
	if manifests, ok := sessionState.Metadata["k8s_manifests"].(map[string]interface{}); ok {
		return manifests
	}
	return nil
}

// getDockerfilePushed checks if dockerfile has been pushed from metadata
func getDockerfilePushed(sessionState *mcp.SessionState) bool {
	if sessionState.Metadata == nil {
		return false
	}
	if pushed, ok := sessionState.Metadata["dockerfile_pushed"].(bool); ok {
		return pushed
	}
	return false
}

// getImageRef constructs the appropriate image reference based on push status
func getImageRef(sessionState *mcp.SessionState) string {
	imageID := getDockerfileImageID(sessionState)
	if getDockerfilePushed(sessionState) {
		registry := getImageRefRegistry(sessionState)
		if registry != "" {
			return fmt.Sprintf("%s/%s", registry, imageID)
		}
	}
	return imageID
}

// setK8sManifest stores a manifest in metadata
func setK8sManifest(sessionState *mcp.SessionState, name string, manifest types.K8sManifest) {
	if sessionState.Metadata == nil {
		sessionState.Metadata = make(map[string]interface{})
	}
	if sessionState.Metadata["k8s_manifests"] == nil {
		sessionState.Metadata["k8s_manifests"] = make(map[string]interface{})
	}
	k8sManifests := sessionState.Metadata["k8s_manifests"].(map[string]interface{})
	k8sManifests[name] = map[string]interface{}{
		"content": manifest.Content,
		"kind":    manifest.Kind,
		"applied": manifest.Applied,
		"status":  manifest.Status,
	}
}

// getK8sManifestsAsTypes converts metadata manifests to types.K8sManifest format
func getK8sManifestsAsTypes(sessionState *mcp.SessionState) map[string]types.K8sManifest {
	result := make(map[string]types.K8sManifest)
	manifestsData := getK8sManifestsFromMetadata(sessionState)
	if manifestsData == nil {
		return result
	}

	for name, manifestData := range manifestsData {
		if manifestMap, ok := manifestData.(map[string]interface{}); ok {
			manifest := types.K8sManifest{}
			if content, ok := manifestMap["content"].(string); ok {
				manifest.Content = content
			}
			if kind, ok := manifestMap["kind"].(string); ok {
				manifest.Kind = kind
			}
			if applied, ok := manifestMap["applied"].(bool); ok {
				manifest.Applied = applied
			}
			if status, ok := manifestMap["status"].(string); ok {
				manifest.Status = status
			}
			if manifest.Content != "" || manifest.Kind != "" {
				result[name] = manifest
			}
		}
	}
	return result
}

// handleManifestsStage handles Kubernetes manifest generation
func (pm *PromptManager) handleManifestsStage(ctx context.Context, state *ConversationState, input string) *ConversationResponse {
	// Add progress indicator and stage intro
	progressPrefix := fmt.Sprintf("%s %s\n\n", getStageProgress(convertFromTypesStage(types.StageManifests)), getStageIntro(convertFromTypesStage(types.StageManifests)))

	// Gather manifest preferences if not set
	appName, _ := state.Context["app_name"].(string) //nolint:errcheck // Will prompt if empty
	if appName == "" {
		response := pm.gatherManifestPreferences(ctx, state, input)
		response.Message = fmt.Sprintf("%s%s", progressPrefix, response.Message)
		return response
	}

	// Check if manifests already generated
	k8sManifests := getK8sManifestsFromMetadata(state.SessionState)
	if len(k8sManifests) > 0 {
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
		Stage:    convertFromTypesStage(types.StageManifests),
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
			Stage:   convertFromTypesStage(types.StageManifests),
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
		Stage:   convertFromTypesStage(types.StageManifests),
		Status:  ResponseStatusWaitingInput,
	}
}

// generateManifests creates Kubernetes manifests
func (pm *PromptManager) generateManifests(ctx context.Context, state *ConversationState) *ConversationResponse {
	response := &ConversationResponse{
		Stage:   convertFromTypesStage(types.StageManifests),
		Status:  ResponseStatusProcessing,
		Message: "Generating Kubernetes manifests...",
	}

	// Determine image reference
	imageRef := getImageRef(state.SessionState)

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
	req := mcp.ToolExecutionRequest{
		ToolName: "generate_manifests",
		Args:     params,
		Metadata: map[string]interface{}{"session_id": state.SessionState.SessionID},
	}
	resultStruct, err := pm.toolOrchestrator.ExecuteTool(ctx, req)
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

		// Attempt automatic fix before showing manual options
		autoFixHelper := NewAutoFixHelper(pm.conversationHandler)
		if autoFixHelper.AttemptAutoFix(ctx, response, convertFromTypesStage(types.StageManifests), err, state) {
			return response
		}

		// Fallback to original behavior if auto-fix is not available
		response.Message = fmt.Sprintf("Failed to generate Kubernetes manifests: %v\n\nWould you like to:", err)
		response.Options = []Option{
			{ID: "retry", Label: "Retry manifest generation"},
			{ID: "manual", Label: "Create manifests manually"},
			{ID: "skip", Label: "Skip and use existing manifests"},
		}
		return response
	}

	toolCall.Result = resultStruct.Result
	response.ToolCalls = []ToolCall{toolCall}

	// Parse manifests from result
	if resultData, ok := resultStruct.Result.(map[string]interface{}); ok {
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
				setK8sManifest(state.SessionState, name, manifest)

				// Add as artifact
				artifact := Artifact{
					Type:    "k8s-manifest",
					Name:    fmt.Sprintf("%s (%s)", name, manifest.Kind),
					Content: manifest.Content,
					Stage:   convertFromTypesStage(types.StageManifests),
				}
				state.AddArtifact(artifact)
			}
		}
	}

	// Format response with manifest summary
	response.Status = ResponseStatusSuccess
	response.Message = pm.formatManifestSummary(getK8sManifestsAsTypes(state.SessionState))
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
	progressPrefix := fmt.Sprintf("%s %s\n\n", getStageProgress(convertFromTypesStage(types.StageDeployment)), getStageIntro(convertFromTypesStage(types.StageDeployment)))

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
		Stage:   convertFromTypesStage(types.StageDeployment),
		Status:  ResponseStatusProcessing,
		Message: "Running deployment preview (dry-run)...",
	}

	// Determine image reference
	imageRef := getImageRef(state.SessionState)

	params := map[string]interface{}{
		"session_id": state.SessionID,
		"app_name":   state.Context["app_name"],
		"namespace":  state.Preferences.Namespace,
		"image_ref":  imageRef,
		"dry_run":    true,
	}

	req := mcp.ToolExecutionRequest{
		ToolName: "deploy_kubernetes",
		Args:     params,
		Metadata: map[string]interface{}{"session_id": state.SessionState.SessionID},
	}
	resultStruct, err := pm.toolOrchestrator.ExecuteTool(ctx, req)
	if err != nil {
		response.Status = ResponseStatusError
		response.Message = fmt.Sprintf("Dry-run failed: %v", err)
		return response
	}

	// Extract the dry-run preview from the result
	if toolResult, ok := resultStruct.Result.(map[string]interface{}); ok {
		dryRunPreview := genericutils.MapGetWithDefault[string](toolResult, "dry_run_preview", "")
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
		Stage:   convertFromTypesStage(types.StageDeployment),
		Status:  ResponseStatusProcessing,
		Message: "Deploying to Kubernetes cluster...",
	}

	// Determine image reference
	imageRef := getImageRef(state.SessionState)

	params := map[string]interface{}{
		"session_id":     state.SessionID,
		"app_name":       state.Context["app_name"],
		"namespace":      state.Preferences.Namespace,
		"image_ref":      imageRef,
		"wait_for_ready": true, // Default to waiting for readiness
		"timeout":        300,  // 5 minutes
	}

	startTime := time.Now()
	req := mcp.ToolExecutionRequest{
		ToolName: "deploy_kubernetes",
		Args:     params,
		Metadata: map[string]interface{}{"session_id": state.SessionState.SessionID},
	}
	resultStruct, err := pm.toolOrchestrator.ExecuteTool(ctx, req)
	duration := time.Since(startTime)

	toolCall := ToolCall{
		Tool:       "deploy_kubernetes",
		Parameters: params,
		Duration:   duration,
	}

	if err != nil {
		toolCall.Error = &types.ToolError{
			Type:      "deployment_error",
			Message:   fmt.Sprintf("deploy_kubernetes error: %v", err),
			Retryable: true,
			Timestamp: time.Now(),
		}
		response.ToolCalls = []ToolCall{toolCall}
		response.Status = ResponseStatusError

		// Attempt automatic fix before showing manual options
		autoFixHelper := NewAutoFixHelper(pm.conversationHandler)
		if autoFixHelper.AttemptAutoFix(ctx, response, convertFromTypesStage(types.StageDeployment), err, state) {
			return response
		}

		// Fallback to original behavior if auto-fix is not available
		// Check if rollback is available
		hasLastKnownGood := false
		if state.SessionState.Metadata != nil {
			if _, ok := state.SessionState.Metadata["last_known_good"]; ok {
				hasLastKnownGood = true
			}
		}
		if hasLastKnownGood && state.Preferences.AutoRollback {
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
			response.Message = fmt.Sprintf("Deployment failed: %v\n\nWould you like to:", err)
			response.Options = []Option{
				{ID: "logs", Label: "Show pod logs"},
				{ID: "retry", Label: "Retry deployment"},
				{ID: "modify", Label: "Modify manifests"},
			}
		}

		return response
	}

	toolCall.Result = resultStruct.Result
	response.ToolCalls = []ToolCall{toolCall}

	// Mark manifests as deployed
	manifests := getK8sManifestsAsTypes(state.SessionState)
	for name, manifest := range manifests {
		manifest.Applied = true
		manifest.Status = "deployed"
		setK8sManifest(state.SessionState, name, manifest)
	}

	// Check health if requested
	waitForReady, _ := state.Context["wait_for_ready"].(bool)   //nolint:errcheck // Defaults to true
	if waitForReady || state.Context["wait_for_ready"] == nil { // Default to true
		return pm.checkDeploymentHealth(ctx, state, resultStruct.Result)
	}

	// Success - move to completed
	state.SetStage(convertFromTypesStage(types.StageCompleted))
	response.Status = ResponseStatusSuccess
	response.Message = pm.formatDeploymentSuccess(state, duration)

	return response
}

// checkDeploymentHealth verifies deployment health
func (pm *PromptManager) checkDeploymentHealth(ctx context.Context, state *ConversationState, deployResult interface{}) *ConversationResponse {
	response := &ConversationResponse{
		Stage:   convertFromTypesStage(types.StageDeployment),
		Status:  ResponseStatusProcessing,
		Message: "Checking deployment health...",
	}

	params := map[string]interface{}{
		"session_id": state.SessionID,
		"app_name":   state.Context["app_name"],
		"namespace":  state.Preferences.Namespace,
		"timeout":    60, // 1 minute for health check
	}

	req := mcp.ToolExecutionRequest{
		ToolName: "check_health",
		Args:     params,
		Metadata: map[string]interface{}{"session_id": state.SessionState.SessionID},
	}
	_, err := pm.toolOrchestrator.ExecuteTool(ctx, req)
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
	state.SetStage(convertFromTypesStage(types.StageCompleted))
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
			Stage:  convertFromTypesStage(types.StageDeployment),
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
