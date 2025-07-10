package conversation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/genericutils"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
)

func getK8sManifestsFromMetadata(sessionState *session.SessionState) map[string]interface{} {
	if sessionState.Metadata == nil {
		return nil
	}
	if manifests, ok := sessionState.Metadata["k8s_manifests"].(map[string]interface{}); ok {
		return manifests
	}
	return nil
}
func getDockerfilePushed(sessionState *session.SessionState) bool {
	if sessionState.Metadata == nil {
		return false
	}
	if pushed, ok := sessionState.Metadata["dockerfile_pushed"].(bool); ok {
		return pushed
	}
	return false
}
func getImageRef(sessionState *session.SessionState) string {
	imageID := getDockerfileImageID(sessionState)
	if getDockerfilePushed(sessionState) {
		registry := getImageRefRegistry(sessionState)
		if registry != "" {
			return fmt.Sprintf("%s/%s", registry, imageID)
		}
	}
	return imageID
}
func setK8sManifest(sessionState *session.SessionState, name string, manifest shared.K8sManifest) {
	if sessionState.Metadata == nil {
		sessionState.Metadata = make(map[string]interface{})
	}
	if sessionState.Metadata["k8s_manifests"] == nil {
		sessionState.Metadata["k8s_manifests"] = make(map[string]interface{})
	}
	k8sManifests, ok := sessionState.Metadata["k8s_manifests"].(map[string]interface{})
	if !ok {

		k8sManifests = make(map[string]interface{})
		sessionState.Metadata["k8s_manifests"] = k8sManifests
	}
	k8sManifests[name] = map[string]interface{}{
		"content": manifest.Content,
		"kind":    manifest.Kind,
		"applied": manifest.Applied,
		"status":  manifest.Status,
	}
}
func getK8sManifestsAsTypes(sessionState *session.SessionState) map[string]shared.K8sManifest {
	result := make(map[string]shared.K8sManifest)
	manifestsData := getK8sManifestsFromMetadata(sessionState)
	if manifestsData == nil {
		return result
	}

	for name, manifestData := range manifestsData {
		if manifestMap, ok := manifestData.(map[string]interface{}); ok {
			manifest := shared.K8sManifest{}
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
func (ps *PromptServiceImpl) handleManifestsStage(ctx context.Context, state *ConversationState, input string) *ConversationResponse {

	progressPrefix := fmt.Sprintf("%s %s\n\n", getStageProgress(convertFromTypesStage(shared.StageManifests)), getStageIntro(convertFromTypesStage(shared.StageManifests)))
	appName, ok := state.Context["app_name"].(string)
	if !ok || appName == "" {
		response := ps.gatherManifestPreferences(ctx, state, input)
		response.Message = fmt.Sprintf("%s%s", progressPrefix, response.Message)
		return response
	}
	k8sManifests := getK8sManifestsFromMetadata(state.SessionState)
	if len(k8sManifests) > 0 {
		response := ps.reviewManifests(ctx, state, input)
		response.Message = fmt.Sprintf("%s%s", progressPrefix, response.Message)
		return response
	}
	response := ps.generateManifests(ctx, state)
	response.Message = fmt.Sprintf("%s%s", progressPrefix, response.Message)
	return response
}
func (ps *PromptServiceImpl) gatherManifestPreferences(_ context.Context, state *ConversationState, input string) *ConversationResponse {

	decision := &DecisionPoint{
		ID:       "k8s-config",
		Stage:    convertFromTypesStage(shared.StageManifests),
		Question: "Let's configure your Kubernetes deployment. What should we name the application?",
		Required: true,
	}
	state.SetPendingDecision(decision)
	if input != "" && !strings.Contains(input, " ") {
		state.Context["app_name"] = strings.ToLower(input)
		state.ResolvePendingDecision(Decision{
			DecisionID:  decision.ID,
			CustomValue: input,
			Timestamp:   time.Now(),
		})
		return &ConversationResponse{
			Message: fmt.Sprintf("App name set to '%s'. How many replicas would you like?", state.Context["app_name"]),
			Stage:   convertFromTypesStage(shared.StageManifests),
			Status:  ResponseStatusWaitingInput,
			Options: []Option{
				{ID: "1", Label: "1 replica (development)"},
				{ID: "3", Label: "3 replicas (production)", Recommended: true},
				{ID: "custom", Label: "Custom number"},
			},
		}
	}
	suggestedName := ps.suggestAppName(state)

	return &ConversationResponse{
		Message: decision.Question + fmt.Sprintf("\n\nSuggested: %s", suggestedName),
		Stage:   convertFromTypesStage(shared.StageManifests),
		Status:  ResponseStatusWaitingInput,
	}
}
func (ps *PromptServiceImpl) generateManifests(ctx context.Context, state *ConversationState) *ConversationResponse {
	response := &ConversationResponse{
		Stage:   convertFromTypesStage(shared.StageManifests),
		Status:  ResponseStatusProcessing,
		Message: "Generating Kubernetes manifests...",
	}
	imageRef := getImageRef(state.SessionState)

	params := map[string]interface{}{
		"session_id":    state.SessionState.SessionID,
		"app_name":      state.Context["app_name"],
		"namespace":     state.Preferences.Namespace,
		"image_ref":     imageRef,
		"replicas":      state.Preferences.Replicas,
		"service_type":  state.Preferences.ServiceType,
		"generate_only": true,
	}
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
	if envVars, ok := state.Context["environment_vars"].(map[string]string); ok && len(envVars) > 0 {
		params["env_vars"] = envVars
	}

	startTime := time.Now()
	resultStruct, err := ps.toolOrchestrator.ExecuteTool(ctx, "generate_manifests", params)
	duration := time.Since(startTime)

	toolCall := ToolCall{
		Tool:       "generate_manifests",
		Parameters: params,
		Duration:   duration,
	}

	if err != nil {
		toolCall.Error = &shared.ToolError{
			Type:      "generation_error",
			Message:   fmt.Sprintf("generate_manifests error: %v", err),
			Retryable: true,
			Timestamp: time.Now(),
		}
		response.ToolCalls = []ToolCall{toolCall}
		response.Status = ResponseStatusError
		autoFixHelper := NewAutoFixHelper(ps.conversationHandler)
		if autoFixHelper.AttemptAutoFix(ctx, response, convertFromTypesStage(shared.StageManifests), err, state) {
			return response
		}
		response.Message = fmt.Sprintf("Failed to generate Kubernetes manifests: %v\n\nWould you like to:", err)
		response.Options = []Option{
			{ID: "retry", Label: "Retry manifest generation"},
			{ID: "manual", Label: "Create manifests manually"},
			{ID: "skip", Label: "Skip and use existing manifests"},
		}
		return response
	}

	toolCall.Result = resultStruct
	response.ToolCalls = []ToolCall{toolCall}
	if resultData, ok := resultStruct.(map[string]interface{}); ok {
		if manifests, ok := resultData["manifests"].(map[string]interface{}); ok {
			for name, content := range manifests {
				contentStr, ok := content.(string)
				if !ok {
					continue
				}
				manifest := shared.K8sManifest{
					Name:    name,
					Content: contentStr,
					Kind:    extractKind(contentStr),
				}
				setK8sManifest(state.SessionState, name, manifest)
				artifact := Artifact{
					Type:    "k8s-manifest",
					Name:    fmt.Sprintf("%s (%s)", name, manifest.Kind),
					Content: manifest.Content,
					Stage:   convertFromTypesStage(shared.StageManifests),
				}
				state.AddArtifact(artifact)
			}
		}
	}
	response.Status = ResponseStatusSuccess
	response.Message = ps.formatManifestSummary(getK8sManifestsAsTypes(state.SessionState))
	response.Options = []Option{
		{ID: "deploy", Label: "Deploy to Kubernetes", Recommended: true},
		{ID: "review", Label: "Show full manifests"},
		{ID: "modify", Label: "Modify configuration"},
		{ID: "dry-run", Label: "Preview deployment (dry-run)"},
	}

	return response
}
func (ps *PromptServiceImpl) handleDeploymentStage(ctx context.Context, state *ConversationState, input string) *ConversationResponse {

	progressPrefix := fmt.Sprintf("%s %s\n\n", getStageProgress(convertFromTypesStage(shared.StageDeployment)), getStageIntro(convertFromTypesStage(shared.StageDeployment)))
	if strings.Contains(strings.ToLower(input), "retry") {
		response := ps.handleDeploymentRetry(ctx, state)
		response.Message = fmt.Sprintf("%s%s", progressPrefix, response.Message)
		return response
	}
	if strings.Contains(strings.ToLower(input), "dry") || strings.Contains(strings.ToLower(input), "preview") {
		response := ps.deploymentDryRun(ctx, state)
		response.Message = fmt.Sprintf("%s%s", progressPrefix, response.Message)
		return response
	}
	if strings.Contains(strings.ToLower(input), "logs") {
		response := ps.showDeploymentLogs(ctx, state)
		response.Message = fmt.Sprintf("%s%s", progressPrefix, response.Message)
		return response
	}
	response := ps.executeDeployment(ctx, state)
	response.Message = fmt.Sprintf("%s%s", progressPrefix, response.Message)
	return response
}
func (ps *PromptServiceImpl) deploymentDryRun(ctx context.Context, state *ConversationState) *ConversationResponse {
	response := &ConversationResponse{
		Stage:   convertFromTypesStage(shared.StageDeployment),
		Status:  ResponseStatusProcessing,
		Message: "Running deployment preview (dry-run)...",
	}
	imageRef := getImageRef(state.SessionState)

	params := map[string]interface{}{
		"session_id": state.SessionState.SessionID,
		"app_name":   state.Context["app_name"],
		"namespace":  state.Preferences.Namespace,
		"image_ref":  imageRef,
		"dry_run":    true,
	}

	resultStruct, err := ps.toolOrchestrator.ExecuteTool(ctx, "deploy_kubernetes", params)
	if err != nil {
		response.Status = ResponseStatusError
		response.Message = fmt.Sprintf("Dry-run failed: %v", err)
		return response
	}
	if toolResult, ok := resultStruct.(map[string]interface{}); ok {
		dryRunPreview := genericutils.MapGetWithDefault[string](toolResult, "dry_run_preview", "")
		if dryRunPreview == "" {
			dryRunPreview = "No changes detected - resources are already up to date"
		}
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
func (ps *PromptServiceImpl) executeDeployment(ctx context.Context, state *ConversationState) *ConversationResponse {
	response := &ConversationResponse{
		Stage:   convertFromTypesStage(shared.StageDeployment),
		Status:  ResponseStatusProcessing,
		Message: "Deploying to Kubernetes cluster...",
	}
	imageRef := getImageRef(state.SessionState)

	params := map[string]interface{}{
		"session_id":     state.SessionState.SessionID,
		"app_name":       state.Context["app_name"],
		"namespace":      state.Preferences.Namespace,
		"image_ref":      imageRef,
		"wait_for_ready": true,
		"timeout":        300,
	}

	startTime := time.Now()
	resultStruct, err := ps.toolOrchestrator.ExecuteTool(ctx, "deploy_kubernetes", params)
	duration := time.Since(startTime)

	toolCall := ToolCall{
		Tool:       "deploy_kubernetes",
		Parameters: params,
		Duration:   duration,
	}

	if err != nil {
		toolCall.Error = &shared.ToolError{
			Type:      "deployment_error",
			Message:   fmt.Sprintf("deploy_kubernetes error: %v", err),
			Retryable: true,
			Timestamp: time.Now(),
		}
		response.ToolCalls = []ToolCall{toolCall}
		response.Status = ResponseStatusError
		autoFixHelper := NewAutoFixHelper(ps.conversationHandler)
		if autoFixHelper.AttemptAutoFix(ctx, response, convertFromTypesStage(shared.StageDeployment), err, state) {
			return response
		}

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

	toolCall.Result = resultStruct
	response.ToolCalls = []ToolCall{toolCall}
	manifests := getK8sManifestsAsTypes(state.SessionState)
	for name, manifest := range manifests {
		manifest.Applied = true
		manifest.Status = "deployed"
		setK8sManifest(state.SessionState, name, manifest)
	}
	waitForReady, ok := state.Context["wait_for_ready"].(bool)
	if !ok || waitForReady || state.Context["wait_for_ready"] == nil {
		return ps.checkDeploymentHealth(ctx, state, resultStruct)
	}
	state.SetStage(convertFromTypesStage(shared.StageCompleted))
	response.Status = ResponseStatusSuccess
	response.Message = ps.formatDeploymentSuccess(state, duration)

	return response
}
func (ps *PromptServiceImpl) checkDeploymentHealth(ctx context.Context, state *ConversationState, _ interface{}) *ConversationResponse {
	response := &ConversationResponse{
		Stage:   convertFromTypesStage(shared.StageDeployment),
		Status:  ResponseStatusProcessing,
		Message: "Checking deployment health...",
	}

	params := map[string]interface{}{
		"session_id": state.SessionState.SessionID,
		"app_name":   state.Context["app_name"],
		"namespace":  state.Preferences.Namespace,
		"timeout":    60,
	}

	_, err := ps.toolOrchestrator.ExecuteTool(ctx, "check_health", params)
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
	state.SetStage(convertFromTypesStage(shared.StageCompleted))
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
func (ps *PromptServiceImpl) handleDeploymentRetry(ctx context.Context, state *ConversationState) *ConversationResponse {

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
			Stage:  convertFromTypesStage(shared.StageDeployment),
			Status: ResponseStatusError,
			Options: []Option{
				{ID: "modify", Label: "Modify manifests"},
				{ID: "rebuild", Label: "Rebuild image"},
			},
		}
	}
	state.Context["deployment_retry_count"] = retryCount + 1
	delay := time.Duration(retryCount+1) * 2 * time.Second
	time.Sleep(delay)

	return ps.executeDeployment(ctx, state)
}
