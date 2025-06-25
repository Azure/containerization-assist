package errors

import (
	"fmt"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/workflow"
	"github.com/rs/zerolog"
)

// RedirectionManager handles error redirection planning
type RedirectionManager struct {
	logger zerolog.Logger
}

// NewRedirectionManager creates a new redirection manager
func NewRedirectionManager(logger zerolog.Logger) *RedirectionManager {
	return &RedirectionManager{
		logger: logger.With().Str("component", "redirection_manager").Logger(),
	}
}

// ValidateRedirectTarget validates that a redirect target is valid and available
func (rm *RedirectionManager) ValidateRedirectTarget(redirectTo string, workflowError *workflow.WorkflowError) error {
	if redirectTo == "" {
		return fmt.Errorf("redirect target cannot be empty")
	}

	// Define valid redirect targets and their conditions
	validRedirectTargets := map[string][]string{
		"validate_dockerfile":   {"build_image", "generate_dockerfile"},
		"fix_manifests":         {"deploy_kubernetes", "generate_manifests"},
		"retry_authentication":  {"*"}, // Can be used from any stage
		"manual_intervention":   {"*"}, // Can be used from any stage
		"cleanup_resources":     {"build_image", "deploy_kubernetes"},
		"alternative_registry":  {"push_image", "pull_image"},
		"security_scan_bypass":  {"scan_image_security"},
		"dependency_resolution": {"analyze_repository", "build_image"},
	}

	if allowedStages, exists := validRedirectTargets[redirectTo]; exists {
		// Check if redirection is allowed from current stage
		for _, allowedStage := range allowedStages {
			if allowedStage == "*" || allowedStage == workflowError.StageName {
				rm.logger.Debug().
					Str("redirect_to", redirectTo).
					Str("from_stage", workflowError.StageName).
					Msg("Redirect target validated successfully")
				return nil
			}
		}
		return fmt.Errorf("redirect to '%s' not allowed from stage '%s'", redirectTo, workflowError.StageName)
	}

	rm.logger.Warn().
		Str("redirect_to", redirectTo).
		Msg("Unknown redirect target, allowing with warning")
	return nil
}

// CreateRedirectionPlan creates a detailed plan for error redirection
func (rm *RedirectionManager) CreateRedirectionPlan(
	redirectTo string,
	workflowError *workflow.WorkflowError,
	session *workflow.WorkflowSession,
) (*RedirectionPlan, error) {
	plan := &RedirectionPlan{
		SourceStage:         workflowError.StageName,
		TargetStage:         redirectTo,
		RedirectionType:     "error_recovery",
		CreatedAt:           time.Now(),
		EstimatedDuration:   30 * time.Second, // Default estimate
		ContextPreservation: true,
		Parameters:          make(map[string]interface{}),
	}

	// Customize plan based on redirect target
	switch redirectTo {
	case "validate_dockerfile":
		plan.EstimatedDuration = 60 * time.Second
		plan.Parameters["validation_mode"] = "strict"
		plan.Parameters["fix_errors"] = true
		plan.Parameters["preserve_build_context"] = true
		plan.RequiredContext = []string{"dockerfile_path", "build_context"}
		plan.ExpectedOutcome = "Fixed Dockerfile with validation passing"

	case "fix_manifests":
		plan.EstimatedDuration = 45 * time.Second
		plan.Parameters["validation_mode"] = "comprehensive"
		plan.Parameters["apply_best_practices"] = true
		plan.RequiredContext = []string{"manifest_files", "target_namespace"}
		plan.ExpectedOutcome = "Valid Kubernetes manifests ready for deployment"

	case "retry_authentication":
		plan.EstimatedDuration = 15 * time.Second
		plan.Parameters["clear_cached_credentials"] = true
		plan.Parameters["prompt_for_new_credentials"] = true
		plan.RequiredContext = []string{"auth_context"}
		plan.ExpectedOutcome = "Refreshed authentication credentials"

	case "cleanup_resources":
		plan.EstimatedDuration = 30 * time.Second
		plan.Parameters["cleanup_scope"] = "session"
		plan.Parameters["preserve_artifacts"] = true
		plan.RequiredContext = []string{"resource_inventory"}
		plan.ExpectedOutcome = "Cleaned up resources with available capacity"

	case "manual_intervention":
		plan.EstimatedDuration = 5 * time.Minute // Assume manual action takes longer
		plan.Parameters["pause_workflow"] = true
		plan.Parameters["create_intervention_request"] = true
		plan.InterventionRequired = true
		plan.ExpectedOutcome = "Manual resolution of the issue"

	default:
		rm.logger.Warn().
			Str("redirect_to", redirectTo).
			Msg("Using default redirection plan for unknown target")
		plan.Parameters["generic_redirection"] = true
	}

	// Add error context to plan
	plan.OriginalError = &RedirectionErrorContext{
		ErrorID:      workflowError.ID,
		ErrorType:    workflowError.ErrorType,
		ErrorMessage: workflowError.Message,
		Severity:     workflowError.Severity,
		Timestamp:    workflowError.Timestamp,
	}

	// Validate that required context is available
	for _, requiredKey := range plan.RequiredContext {
		if _, exists := session.SharedContext[requiredKey]; !exists {
			rm.logger.Warn().
				Str("required_key", requiredKey).
				Str("redirect_to", redirectTo).
				Msg("Required context missing for redirection")
			plan.MissingContext = append(plan.MissingContext, requiredKey)
		}
	}

	rm.logger.Info().
		Str("source_stage", plan.SourceStage).
		Str("target_stage", plan.TargetStage).
		Dur("estimated_duration", plan.EstimatedDuration).
		Int("missing_context_count", len(plan.MissingContext)).
		Msg("Created redirection plan")

	return plan, nil
}
