package conversation

import (
	"context"
	"fmt"
	"strings"
	"time"

	domaintypes "github.com/Azure/container-kit/pkg/mcp/domain/types"
)

func (ps *PromptServiceImpl) hasRunBuildDryRun(state *ConversationState) bool {
	_, ok := state.Context["build_dry_run_complete"].(bool)
	return ok
}

func (ps *PromptServiceImpl) generateImageTag(state *ConversationState) string {
	appName, _ := state.Context["app_name"].(string)
	if appName == "" {
		appName = "app"
	}

	timestamp := time.Now().Format("20060102-150405")
	return fmt.Sprintf("%s:%s", appName, timestamp)
}

func (ps *PromptServiceImpl) performSecurityScan(ctx context.Context, state *ConversationState) *ConversationResponse {
	response := &ConversationResponse{
		Stage:   convertFromTypesStage(domaintypes.StagePush),
		Status:  ResponseStatusProcessing,
		Message: "Running security scan on image...",
	}

	params := map[string]interface{}{
		"session_id": state.SessionState.SessionID,
		"image_ref":  getDockerfileImageID(state.SessionState),
	}

	resultStruct, err := ps.toolOrchestrator.ExecuteTool(ctx, "scan_image_security", params)
	if err != nil {
		response.Status = ResponseStatusError
		response.Message = fmt.Sprintf("Security scan failed: %v\n\nContinue anyway?", err)
		response.Options = []Option{
			{ID: "push", Label: "Yes, push anyway"},
			{ID: "cancel", Label: "No, cancel push"},
		}
		return response
	}

	if scanResult, ok := resultStruct.(map[string]interface{}); ok {
		vulnerabilities := extractVulnerabilities(scanResult)
		if len(vulnerabilities) > 0 {
			response.Status = ResponseStatusWarning
			response.Message = formatSecurityScanResults(vulnerabilities)
			response.Options = []Option{
				{ID: "push", Label: "Push despite vulnerabilities"},
				{ID: "cancel", Label: "Cancel push"},
			}
		} else {
			response.Status = ResponseStatusSuccess
			response.Message = "✅ Security scan passed! No vulnerabilities found.\n\nProceed with push?"
			response.Options = []Option{
				{ID: "push", Label: "Yes, push to registry", Recommended: true},
				{ID: "cancel", Label: "Cancel"},
			}
		}
	}

	return response
}

func (ps *PromptServiceImpl) reviewManifests(_ context.Context, state *ConversationState, input string) *ConversationResponse {
	if strings.Contains(strings.ToLower(input), "show") || strings.Contains(strings.ToLower(input), "full") {
		var manifestsText strings.Builder
		if state.SessionState.Metadata != nil {
			if k8sManifests, ok := state.SessionState.Metadata["k8s_manifests"].(map[string]interface{}); ok {
				for name, manifestData := range k8sManifests {
					if manifestMap, ok := manifestData.(map[string]interface{}); ok {
						if content, ok := manifestMap["content"].(string); ok {
							manifestsText.WriteString(fmt.Sprintf("# %s\n---\n%s\n\n", name, content))
						}
					}
				}
			}
		}

		return &ConversationResponse{
			Message: fmt.Sprintf("Full Kubernetes manifests:\n\n```yaml\n%s```\n\nReady to deploy?", manifestsText.String()),
			Stage:   convertFromTypesStage(domaintypes.StageManifests),
			Status:  ResponseStatusSuccess,
			Options: []Option{
				{ID: "deploy", Label: "Deploy to Kubernetes", Recommended: true},
				{ID: "modify", Label: "Modify configuration"},
			},
		}
	}

	state.SetStage(convertFromTypesStage(domaintypes.StageDeployment))
	return &ConversationResponse{
		Message: "Manifests are ready. Shall we deploy to Kubernetes?",
		Stage:   convertFromTypesStage(domaintypes.StageDeployment),
		Status:  ResponseStatusSuccess,
		Options: []Option{
			{ID: "deploy", Label: "Yes, deploy", Recommended: true},
			{ID: "dry-run", Label: "Preview first (dry-run)"},
			{ID: "review", Label: "Review manifests again"},
		},
	}
}

func (ps *PromptServiceImpl) suggestAppName(state *ConversationState) string {
	if state.SessionState.RepoURL != "" {
		parts := strings.Split(state.SessionState.RepoURL, "/")
		if len(parts) > 0 {
			name := parts[len(parts)-1]
			name = strings.TrimSuffix(name, ".git")
			name = strings.ToLower(name)
			name = strings.ReplaceAll(name, "_", "-")
			return name
		}
	}

	if state.SessionState.Metadata != nil {
		if repoAnalysis, ok := state.SessionState.Metadata["repo_analysis"].(map[string]interface{}); ok {
			if projectName, ok := repoAnalysis["project_name"].(string); ok {
				return strings.ToLower(strings.ReplaceAll(projectName, "_", "-"))
			}
		}
	}

	return "my-app"
}

func (ps *PromptServiceImpl) formatManifestSummary(manifests map[string]domaintypes.K8sManifest) string {
	var sb strings.Builder
	sb.WriteString("✅ Kubernetes manifests generated:\n\n")

	for name, manifest := range manifests {
		sb.WriteString(fmt.Sprintf("- %s (%s)\n", name, manifest.Kind))
	}

	sb.WriteString("\nKey features:\n")
	sb.WriteString("- Rolling update strategy\n")
	sb.WriteString("- Resource limits configured\n")
	sb.WriteString("- Health checks included\n")
	sb.WriteString("- Service exposed\n")

	return sb.String()
}

func (ps *PromptServiceImpl) formatDeploymentSuccess(state *ConversationState, duration time.Duration) string {
	var sb strings.Builder

	sb.WriteString("🎉 Deployment completed successfully!\n\n")
	sb.WriteString(fmt.Sprintf("Application: %s\n", state.Context["app_name"]))
	sb.WriteString(fmt.Sprintf("Namespace: %s\n", state.Preferences.Namespace))
	sb.WriteString(fmt.Sprintf("Deployment time: %s\n", duration.Round(time.Second)))
	sb.WriteString("\nResources created:\n")

	if state.SessionState.Metadata != nil {
		if k8sManifests, ok := state.SessionState.Metadata["k8s_manifests"].(map[string]interface{}); ok {
			for name, manifestData := range k8sManifests {
				if manifestMap, ok := manifestData.(map[string]interface{}); ok {
					if kind, ok := manifestMap["kind"].(string); ok {
						sb.WriteString(fmt.Sprintf("- %s (%s)\n", name, kind))
					}
				}
			}
		}
	}

	sb.WriteString("\nTo access your application:\n")
	sb.WriteString(fmt.Sprintf("kubectl port-forward -n %s svc/%s-service 8080:80\n",
		state.Preferences.Namespace, state.Context["app_name"]))

	sb.WriteString("\nYour containerization journey is complete! 🚀")

	return sb.String()
}

func (ps *PromptServiceImpl) showDeploymentLogs(ctx context.Context, state *ConversationState) *ConversationResponse {
	response := &ConversationResponse{
		Stage:   convertFromTypesStage(domaintypes.StageDeployment),
		Status:  ResponseStatusProcessing,
		Message: "Fetching deployment logs...",
	}

	params := map[string]interface{}{
		"session_id":   state.SessionState.SessionID,
		"app_name":     state.Context["app_name"],
		"namespace":    state.Preferences.Namespace,
		"include_logs": true,
		"log_lines":    100,
	}

	resultStruct, err := ps.toolOrchestrator.ExecuteTool(ctx, "check_health", params)
	if err != nil {
		response.Status = ResponseStatusError
		response.Message = fmt.Sprintf("Failed to fetch logs: %v", err)
		return response
	}

	if healthResult, ok := resultStruct.(map[string]interface{}); ok {
		if logs, ok := healthResult["logs"].(string); ok && logs != "" {
			response.Status = ResponseStatusSuccess
			response.Message = fmt.Sprintf("Pod logs:\n\n```\n%s\n```\n\nBased on these logs, what would you like to do?", logs)
			response.Options = []Option{
				{ID: "retry", Label: "Retry deployment"},
				{ID: "modify", Label: "Modify configuration"},
				{ID: "rollback", Label: "Rollback if available"},
			}
		} else {
			response.Status = ResponseStatusWarning
			response.Message = "No logs available. The pods may not have started yet."
		}
	}

	return response
}

func extractRegistry(input string) string {
	if strings.Contains(input, internal.DefaultRegistry) || strings.Contains(input, "dockerhub") {
		return internal.DefaultRegistry
	}
	if strings.Contains(input, "gcr.io") {
		return "gcr.io"
	}
	if strings.Contains(input, "acr") && strings.Contains(input, "azurecr.io") {
		return input
	}
	if strings.Contains(input, "ecr") && strings.Contains(input, "amazonaws.com") {
		return input
	}

	if strings.Contains(input, ".") && (strings.Contains(input, ":") || strings.Count(input, "/") <= 1) {
		return strings.Split(input, "/")[0]
	}

	return internal.DefaultRegistry
}

func extractTag(imageRef string) string {
	parts := strings.Split(imageRef, ":")
	if len(parts) > 1 {
		lastPart := parts[len(parts)-1]
		if !strings.Contains(lastPart, "/") {
			return lastPart
		}
	}
	return "latest"
}

func extractKind(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "kind:") {
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return "Unknown"
}

func extractVulnerabilities(scanResult map[string]interface{}) []map[string]interface{} {
	if vulns, ok := scanResult["vulnerabilities"].([]interface{}); ok {
		vulnerabilities := make([]map[string]interface{}, 0, len(vulns))
		for _, v := range vulns {
			if vuln, ok := v.(map[string]interface{}); ok {
				vulnerabilities = append(vulnerabilities, vuln)
			}
		}
		return vulnerabilities
	}
	return nil
}

func formatSecurityScanResults(vulnerabilities []map[string]interface{}) string {
	var critical, high, medium, low int
	for _, vuln := range vulnerabilities {
		if severity, ok := vuln["severity"].(string); ok {
			switch strings.ToLower(severity) {
			case "critical":
				critical++
			case "high":
				high++
			case "medium":
				medium++
			case "low":
				low++
			}
		}
	}

	return fmt.Sprintf(
		"⚠️ Security scan found vulnerabilities:\n\n"+
			"- Critical: %d\n"+
			"- High: %d\n"+
			"- Medium: %d\n"+
			"- Low: %d\n\n"+
			"Would you like to proceed with push?",
		critical, high, medium, low)
}
