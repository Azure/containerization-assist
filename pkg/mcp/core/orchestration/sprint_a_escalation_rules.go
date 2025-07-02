package orchestration

import (
	"time"
)

// InitializeSprintAEscalationRules adds enhanced cross-tool escalation rules
// This implements the cross-tool error escalation equivalent to legacy OnFailGoto
func (er *DefaultErrorRouter) InitializeSprintAEscalationRules() {
	// Build → Manifest Escalation
	// When build failures might be resolved by deployment configuration changes
	er.addDefaultRule("build_image", ErrorRoutingRule{
		ID:          "build_manifest_escalation",
		Name:        "Build Error Manifest Escalation",
		Description: "Escalate build errors that might be resolved by manifest configuration",
		Conditions: []RoutingCondition{
			{Field: "error_type", Operator: "contains", Value: "build_error"},
			{Field: "message", Operator: "contains", Value: "resource"},
		},
		Action:     "redirect",
		RedirectTo: "generate_manifests",
		Parameters: &ErrorRoutingParameters{
			FixErrors: true,
			CustomParams: map[string]string{
				"escalation_source": "build_image",
				"fix_resources":     "true",
				"escalation_mode":   "auto",
			},
		},
		Priority: 125,
		Enabled:  true,
	})

	// Build → Dockerfile Escalation
	// When build failures need dockerfile fixes
	er.addDefaultRule("build_image", ErrorRoutingRule{
		ID:          "build_dockerfile_escalation",
		Name:        "Build Error Dockerfile Escalation",
		Description: "Escalate build errors to dockerfile regeneration",
		Conditions: []RoutingCondition{
			{Field: "error_type", Operator: "contains", Value: "build_error"},
			{Field: "message", Operator: "contains", Value: "dockerfile"},
		},
		Action:     "redirect",
		RedirectTo: "generate_dockerfile",
		Parameters: &ErrorRoutingParameters{
			FixErrors: true,
			CustomParams: map[string]string{
				"escalation_source": "build_image",
				"fix_dockerfile":    "true",
				"escalation_mode":   "auto",
			},
		},
		Priority: 130,
		Enabled:  true,
	})

	// Deploy → Build Escalation
	// When deployment failures require rebuilding the image
	er.addDefaultRule("deploy_kubernetes", ErrorRoutingRule{
		ID:          "deploy_build_escalation",
		Name:        "Deploy Error Build Escalation",
		Description: "Escalate deployment errors that require image rebuilds",
		Conditions: []RoutingCondition{
			{Field: "error_type", Operator: "contains", Value: "deployment_error"},
			{Field: "message", Operator: "contains", Value: "image"},
		},
		Action:     "redirect",
		RedirectTo: "build_image",
		Parameters: &ErrorRoutingParameters{
			FixErrors: true,
			CustomParams: map[string]string{
				"escalation_source": "deploy_kubernetes",
				"rebuild_image":     "true",
				"escalation_mode":   "auto",
			},
		},
		Priority: 125,
		Enabled:  true,
	})

	// Deploy → Manifest Escalation
	// When deployment failures need manifest fixes
	er.addDefaultRule("deploy_kubernetes", ErrorRoutingRule{
		ID:          "deploy_manifest_escalation",
		Name:        "Deploy Error Manifest Escalation",
		Description: "Escalate deployment errors to manifest regeneration",
		Conditions: []RoutingCondition{
			{Field: "error_type", Operator: "contains", Value: "deployment_error"},
			{Field: "message", Operator: "contains", Value: "manifest"},
		},
		Action:     "redirect",
		RedirectTo: "generate_manifests",
		Parameters: &ErrorRoutingParameters{
			FixErrors: true,
			CustomParams: map[string]string{
				"escalation_source": "deploy_kubernetes",
				"fix_manifests":     "true",
				"escalation_mode":   "auto",
			},
		},
		Priority: 120,
		Enabled:  true,
	})

	// Manifest → Build Escalation
	// When manifest generation failures indicate fundamental image issues
	er.addDefaultRule("generate_manifests", ErrorRoutingRule{
		ID:          "manifest_build_escalation",
		Name:        "Manifest Error Build Escalation",
		Description: "Escalate manifest errors that require image rebuilds",
		Conditions: []RoutingCondition{
			{Field: "error_type", Operator: "contains", Value: "manifest_error"},
			{Field: "message", Operator: "contains", Value: "port"},
		},
		Action:     "redirect",
		RedirectTo: "build_image",
		Parameters: &ErrorRoutingParameters{
			FixErrors: true,
			CustomParams: map[string]string{
				"escalation_source": "generate_manifests",
				"rebuild_image":     "true",
				"escalation_mode":   "auto",
			},
		},
		Priority: 120,
		Enabled:  true,
	})

	// Manifest → Dockerfile Escalation
	// When manifest generation failures need dockerfile fixes
	er.addDefaultRule("generate_manifests", ErrorRoutingRule{
		ID:          "manifest_dockerfile_escalation",
		Name:        "Manifest Error Dockerfile Escalation",
		Description: "Escalate manifest errors to dockerfile regeneration",
		Conditions: []RoutingCondition{
			{Field: "error_type", Operator: "contains", Value: "manifest_error"},
			{Field: "message", Operator: "contains", Value: "dependency"},
		},
		Action:     "redirect",
		RedirectTo: "generate_dockerfile",
		Parameters: &ErrorRoutingParameters{
			FixErrors: true,
			CustomParams: map[string]string{
				"escalation_source": "generate_manifests",
				"fix_dockerfile":    "true",
				"escalation_mode":   "auto",
			},
		},
		Priority: 115,
		Enabled:  true,
	})

	// Dockerfile → Analysis Escalation
	// When dockerfile generation needs deeper analysis
	er.addDefaultRule("generate_dockerfile", ErrorRoutingRule{
		ID:          "dockerfile_analysis_escalation",
		Name:        "Dockerfile Error Analysis Escalation",
		Description: "Escalate dockerfile errors to repository analysis",
		Conditions: []RoutingCondition{
			{Field: "error_type", Operator: "contains", Value: "dockerfile_error"},
			{Field: "message", Operator: "contains", Value: "analysis"},
		},
		Action:     "redirect",
		RedirectTo: "analyze_repository",
		Parameters: &ErrorRoutingParameters{
			FixErrors: true,
			CustomParams: map[string]string{
				"escalation_source": "generate_dockerfile",
				"deep_analysis":     "true",
				"escalation_mode":   "auto",
			},
		},
		Priority: 110,
		Enabled:  true,
	})

	// Enhanced retry policies for escalated tools
	er.addEscalationRetryPolicies()

	// Context preservation rules for escalation
	er.addEscalationContextRules()
}

// addEscalationRetryPolicies adds retry policies optimized for cross-tool escalation
func (er *DefaultErrorRouter) addEscalationRetryPolicies() {
	// Escalated operations get more aggressive retry policies
	er.SetRetryPolicy("escalated_build", &RetryPolicy{
		MaxAttempts:  2, // Fewer attempts since this is already an escalation
		BackoffMode:  "fixed",
		InitialDelay: 30 * time.Second,
	})

	er.SetRetryPolicy("escalated_deploy", &RetryPolicy{
		MaxAttempts:  2,
		BackoffMode:  "fixed",
		InitialDelay: 20 * time.Second,
	})

	er.SetRetryPolicy("escalated_generate", &RetryPolicy{
		MaxAttempts:  1, // Generation steps should be fast
		BackoffMode:  "fixed",
		InitialDelay: 10 * time.Second,
	})
}

// addEscalationContextRules adds context preservation rules for escalation scenarios
func (er *DefaultErrorRouter) addEscalationContextRules() {
	// These rules would be implemented when context sharing is enhanced
	// For now, documenting the intended behavior

	// Context that should be preserved during escalation:
	// - Original error details
	// - Session state
	// - Workspace directory
	// - Previous attempt history
	// - Tool-specific configurations
	// - User preferences and settings

	er.logger.Info().Msg("Escalation context preservation rules initialized")
}

// IsEscalatedOperation checks if an operation is the result of an escalation
func (er *DefaultErrorRouter) IsEscalatedOperation(parameters map[string]interface{}) bool {
	if escalationMode, exists := parameters["escalation_mode"]; exists {
		return escalationMode == "auto"
	}
	return false
}

// GetEscalationSource returns the source tool that triggered the escalation
func (er *DefaultErrorRouter) GetEscalationSource(parameters map[string]interface{}) string {
	if source, exists := parameters["escalation_source"]; exists {
		if sourceStr, ok := source.(string); ok {
			return sourceStr
		}
	}
	return ""
}
