package conversation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/types"
)

// Helper methods for extracting user input preferences

func (pm *PromptManager) extractRepositoryReference(input string) string {
	// Look for common repository patterns
	patterns := []string{
		`https?://github\.com/[\w-]+/[\w-]+`,
		`git@github\.com:[\w-]+/[\w-]+\.git`,
		`/[\w/\-\.]+`,
		`\.{1,2}/[\w/\-\.]+`,
	}

	for _, pattern := range patterns {
		if match := findPattern(input, pattern); match != "" {
			return match
		}
	}

	return ""
}

func (pm *PromptManager) extractDockerfilePreferences(state *ConversationState, input string) {
	lower := strings.ToLower(input)

	if strings.Contains(lower, "size") || strings.Contains(lower, "small") {
		state.Preferences.Optimization = "size"
	} else if strings.Contains(lower, "security") || strings.Contains(lower, "secure") {
		state.Preferences.Optimization = "security"
	} else if strings.Contains(lower, "speed") || strings.Contains(lower, "fast") {
		state.Preferences.Optimization = "speed"
	}

	if strings.Contains(lower, "health") || strings.Contains(lower, "healthcheck") {
		state.Preferences.IncludeHealthCheck = true
	}
}

func (pm *PromptManager) getStringSliceFromMap(m map[string]interface{}, key string, defaultValue []string) []string {
	if val, ok := m[key].([]interface{}); ok {
		result := make([]string, 0, len(val))
		for _, v := range val {
			if s, ok := v.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return defaultValue
}

// handlePendingDecision processes user input for a pending decision
func (pm *PromptManager) handlePendingDecision(ctx context.Context, state *ConversationState, input string) *ConversationResponse {
	decision := state.PendingDecision

	// Match input to options
	var selectedOption *Option
	lower := strings.ToLower(input)

	for _, opt := range decision.Options {
		if strings.Contains(lower, strings.ToLower(opt.ID)) ||
			strings.Contains(lower, strings.ToLower(opt.Label)) {
			selectedOption = &opt
			break
		}
	}

	// If no match and there's a default, use it
	if selectedOption == nil && decision.Default != "" {
		for _, opt := range decision.Options {
			if opt.ID == decision.Default {
				selectedOption = &opt
				break
			}
		}
	}

	// Apply the decision
	if selectedOption != nil {
		userDecision := Decision{
			DecisionID: decision.ID,
			OptionID:   selectedOption.ID,
			Timestamp:  time.Now(),
		}

		// Apply preferences based on decision
		if values, ok := selectedOption.Value.(map[string]interface{}); ok {
			for k, v := range values {
				switch k {
				case "optimization":
					if opt, ok := v.(string); ok {
						state.Preferences.Optimization = opt
					}
				case "include_health_check":
					if healthCheck, ok := v.(bool); ok {
						state.Preferences.IncludeHealthCheck = healthCheck
					}
				}
			}
		}

		state.ResolvePendingDecision(userDecision)
	}

	// Continue with the stage
	switch state.CurrentStage {
	case types.StageDockerfile:
		return pm.generateDockerfile(ctx, state)
	default:
		return &ConversationResponse{
			Message: "Let's continue...",
			Stage:   state.CurrentStage,
			Status:  ResponseStatusSuccess,
		}
	}
}

// Summary and export functions

func (pm *PromptManager) generateSummary(ctx context.Context, state *ConversationState) *ConversationResponse {
	var summary strings.Builder
	summary.WriteString("📊 Deployment Summary\n")
	summary.WriteString("===================\n\n")

	// Application details
	if appName, ok := state.Context["app_name"].(string); ok {
		summary.WriteString(fmt.Sprintf("**Application**: %s\n", appName))
	}
	summary.WriteString(fmt.Sprintf("**Namespace**: %s\n", state.Preferences.Namespace))
	summary.WriteString(fmt.Sprintf("**Replicas**: %d\n\n", state.Preferences.Replicas))

	// Docker details
	summary.WriteString("**Docker Image**\n")
	if getDockerfilePushed(state.SessionState) {
		summary.WriteString(fmt.Sprintf("- Registry: %s\n", getImageRefRegistry(state.SessionState)))
		summary.WriteString(fmt.Sprintf("- Tag: %s\n", getImageRefTag(state.SessionState)))
	} else {
		summary.WriteString(fmt.Sprintf("- Local image: %s\n", getDockerfileImageID(state.SessionState)))
	}
	summary.WriteString(fmt.Sprintf("- Optimization: %s\n", state.Preferences.Optimization))
	summary.WriteString(fmt.Sprintf("- Health check: %v\n\n", state.Preferences.IncludeHealthCheck))

	// Kubernetes resources
	summary.WriteString("**Kubernetes Resources**\n")
	manifests := getK8sManifestsAsTypes(state.SessionState)
	for name, manifest := range manifests {
		summary.WriteString(fmt.Sprintf("- %s (%s)\n", name, manifest.Kind))
	}

	// Artifacts
	summary.WriteString("\n**Generated Artifacts**\n")
	for _, artifact := range state.Artifacts {
		summary.WriteString(fmt.Sprintf("- %s: %s\n", artifact.Type, artifact.Name))
	}

	return &ConversationResponse{
		Message: summary.String(),
		Stage:   types.StageCompleted,
		Status:  ResponseStatusSuccess,
	}
}

func (pm *PromptManager) exportArtifacts(ctx context.Context, state *ConversationState) *ConversationResponse {
	// In a real implementation, this would export all artifacts to a directory
	// For now, we'll just list them
	var exports strings.Builder
	exports.WriteString("📦 Exportable Artifacts\n")
	exports.WriteString("=====================\n\n")

	for _, artifact := range state.Artifacts {
		exports.WriteString(fmt.Sprintf("**%s** (%s)\n", artifact.Name, artifact.Type))
		exports.WriteString("```\n")
		// Truncate content for display
		content := artifact.Content
		if len(content) > 500 {
			content = content[:500] + "\n... (truncated)"
		}
		exports.WriteString(content)
		exports.WriteString("\n```\n\n")
	}

	exports.WriteString("\nTo save these artifacts, you can copy them from the output above.")

	return &ConversationResponse{
		Message: exports.String(),
		Stage:   types.StageCompleted,
		Status:  ResponseStatusSuccess,
	}
}

// findPattern is a helper to find patterns in input
func findPattern(input, pattern string) string {
	// This is a simplified pattern matcher
	// In production, you'd use proper regex
	if strings.Contains(input, "github.com") {
		parts := strings.Fields(input)
		for _, part := range parts {
			if strings.Contains(part, "github.com") {
				return strings.TrimSpace(part)
			}
		}
	}
	return ""
}
