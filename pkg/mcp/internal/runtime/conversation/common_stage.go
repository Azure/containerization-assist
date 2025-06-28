package conversation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/types"
)

// Common helper methods used across different stages

// hasRunBuildDryRun checks if build dry-run has been completed
func (pm *PromptManager) hasRunBuildDryRun(state *ConversationState) bool {
	_, ok := state.Context["build_dry_run_complete"].(bool)
	return ok
}

// generateImageTag generates a unique image tag
func (pm *PromptManager) generateImageTag(state *ConversationState) string {
	appName, _ := state.Context["app_name"].(string) //nolint:errcheck // Has default
	if appName == "" {
		appName = "app"
	}

	// Use timestamp for unique tag
	timestamp := time.Now().Format("20060102-150405")
	return fmt.Sprintf("%s:%s", appName, timestamp)
}

// performSecurityScan performs a security scan on the built image
func (pm *PromptManager) performSecurityScan(ctx context.Context, state *ConversationState) *ConversationResponse {
	response := &ConversationResponse{
		Stage:   types.StagePush,
		Status:  ResponseStatusProcessing,
		Message: "Running security scan on image...",
	}

	params := map[string]interface{}{
		"session_id": state.SessionID,
		"image_ref":  getDockerfileImageID(state.SessionState),
	}

	result, err := pm.toolOrchestrator.ExecuteTool(ctx, "scan_image_security", params, state.SessionState.SessionID)
	if err != nil {
		response.Status = ResponseStatusError
		response.Message = fmt.Sprintf("Security scan failed: %v\n\nContinue anyway?", err)
		response.Options = []Option{
			{ID: "push", Label: "Yes, push anyway"},
			{ID: "cancel", Label: "No, cancel push"},
		}
		return response
	}

	// Format scan results
	if scanResult, ok := result.(map[string]interface{}); ok {
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

// reviewManifests handles manifest review requests
func (pm *PromptManager) reviewManifests(ctx context.Context, state *ConversationState, input string) *ConversationResponse {
	if strings.Contains(strings.ToLower(input), "show") || strings.Contains(strings.ToLower(input), "full") {
		// Show full manifests
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
			Stage:   types.StageManifests,
			Status:  ResponseStatusSuccess,
			Options: []Option{
				{ID: "deploy", Label: "Deploy to Kubernetes", Recommended: true},
				{ID: "modify", Label: "Modify configuration"},
			},
		}
	}

	// Already have manifests, ask about deployment
	state.SetStage(types.StageDeployment)
	return &ConversationResponse{
		Message: "Manifests are ready. Shall we deploy to Kubernetes?",
		Stage:   types.StageDeployment,
		Status:  ResponseStatusSuccess,
		Options: []Option{
			{ID: "deploy", Label: "Yes, deploy", Recommended: true},
			{ID: "dry-run", Label: "Preview first (dry-run)"},
			{ID: "review", Label: "Review manifests again"},
		},
	}
}

// suggestAppName suggests an application name based on repository info
func (pm *PromptManager) suggestAppName(state *ConversationState) string {
	// Try to extract from repo URL
	if state.RepoURL != "" {
		parts := strings.Split(state.RepoURL, "/")
		if len(parts) > 0 {
			name := parts[len(parts)-1]
			name = strings.TrimSuffix(name, ".git")
			name = strings.ToLower(name)
			name = strings.ReplaceAll(name, "_", "-")
			return name
		}
	}

	// Try to extract from repo analysis
	if state.SessionState.Metadata != nil {
		if repoAnalysis, ok := state.SessionState.Metadata["repo_analysis"].(map[string]interface{}); ok {
			if projectName, ok := repoAnalysis["project_name"].(string); ok {
				return strings.ToLower(strings.ReplaceAll(projectName, "_", "-"))
			}
		}
	}

	return "my-app"
}

// formatManifestSummary formats a summary of generated manifests
func (pm *PromptManager) formatManifestSummary(manifests map[string]types.K8sManifest) string {
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

// formatDeploymentSuccess formats a deployment success message
func (pm *PromptManager) formatDeploymentSuccess(state *ConversationState, duration time.Duration) string {
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

// showDeploymentLogs shows logs from failed deployment
func (pm *PromptManager) showDeploymentLogs(ctx context.Context, state *ConversationState) *ConversationResponse {
	response := &ConversationResponse{
		Stage:   types.StageDeployment,
		Status:  ResponseStatusProcessing,
		Message: "Fetching deployment logs...",
	}

	params := map[string]interface{}{
		"session_id":   state.SessionID,
		"app_name":     state.Context["app_name"],
		"namespace":    state.Preferences.Namespace,
		"include_logs": true,
		"log_lines":    100,
	}

	result, err := pm.toolOrchestrator.ExecuteTool(ctx, "check_health", params, state.SessionState.SessionID)
	if err != nil {
		response.Status = ResponseStatusError
		response.Message = fmt.Sprintf("Failed to fetch logs: %v", err)
		return response
	}

	// Extract logs from result
	if healthResult, ok := result.(map[string]interface{}); ok {
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

// Helper functions for working with data

// extractRegistry extracts registry URL from user input
func extractRegistry(input string) string {
	// Check for common registries
	if strings.Contains(input, types.DefaultRegistry) || strings.Contains(input, "dockerhub") {
		return types.DefaultRegistry
	}
	if strings.Contains(input, "gcr.io") {
		return "gcr.io"
	}
	if strings.Contains(input, "acr") && strings.Contains(input, "azurecr.io") {
		return input // Full ACR URL
	}
	if strings.Contains(input, "ecr") && strings.Contains(input, "amazonaws.com") {
		return input // Full ECR URL
	}

	// If it looks like a registry URL, use it
	if strings.Contains(input, ".") && (strings.Contains(input, ":") || strings.Count(input, "/") <= 1) {
		return strings.Split(input, "/")[0]
	}

	// Default to docker.io
	return types.DefaultRegistry
}

// extractTag extracts tag from image reference
func extractTag(imageRef string) string {
	// Look for tag after colon
	parts := strings.Split(imageRef, ":")
	if len(parts) > 1 {
		// Handle case where there's a port in registry
		lastPart := parts[len(parts)-1]
		if !strings.Contains(lastPart, "/") {
			return lastPart
		}
	}
	return "latest"
}

// extractKind extracts Kubernetes resource kind from manifest content
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

// extractVulnerabilities extracts vulnerability information from scan results
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

// formatSecurityScanResults formats vulnerability scan results
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
