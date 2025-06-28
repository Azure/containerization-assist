package orchestration

import (
	"context"
	"fmt"

	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
)

// StageValidator handles validation of workflow stages
type StageValidator struct {
	toolRegistry InternalToolRegistry
}

// NewStageValidator creates a new stage validator
func NewStageValidator(toolRegistry InternalToolRegistry) *StageValidator {
	return &StageValidator{
		toolRegistry: toolRegistry,
	}
}

// Validate validates a workflow stage configuration
func (sv *StageValidator) Validate(ctx context.Context, stage *WorkflowStage) error {
	// Basic validation
	if err := sv.validateBasicRequirements(ctx, stage); err != nil {
		return err
	}

	// Tool validation
	if err := sv.validateTools(stage); err != nil {
		return err
	}

	// Timeout validation
	if err := sv.validateTimeout(stage); err != nil {
		return err
	}

	// Retry policy validation
	if err := sv.validateRetryPolicy(stage); err != nil {
		return err
	}

	// Condition validation
	if err := sv.validateConditions(stage); err != nil {
		return err
	}

	// Failure action validation
	if err := sv.validateFailureAction(stage); err != nil {
		return err
	}

	return nil
}

// validateBasicRequirements checks basic stage requirements
func (sv *StageValidator) validateBasicRequirements(ctx context.Context, stage *WorkflowStage) error {
	if stage.Name == "" {
		return mcptypes.NewErrorBuilder("VALIDATION_ERROR", "Stage name is required", "validation_error").
			WithOperation("validate_stage").
			WithStage("basic_requirements").
			WithRootCause("Stage name field is empty").
			WithImmediateStep(1, "Set name", "Provide a descriptive name for the stage").
			WithImmediateStep(2, "Check config", "Verify stage configuration includes name field").
			Build()
	}

	if len(stage.Tools) == 0 {
		return mcptypes.NewErrorBuilder("VALIDATION_ERROR", "Stage must specify at least one tool", "validation_error").
			WithField("stage_name", stage.Name).
			WithOperation("validate_stage").
			WithStage("basic_requirements").
			WithRootCause("Stage has no tools configured").
			WithImmediateStep(1, "Add tools", "Specify at least one tool for the stage to execute").
			WithImmediateStep(2, "Check requirements", "Verify stage requirements and add appropriate tools").
			Build()
	}

	return nil
}

// validateTools validates that all tools exist and are available
func (sv *StageValidator) validateTools(stage *WorkflowStage) error {
	for _, toolName := range stage.Tools {
		if _, err := sv.toolRegistry.GetTool(toolName); err != nil {
			return mcptypes.NewErrorBuilder("VALIDATION_ERROR", "Invalid tool in stage", "validation_error").
				WithField("stage_name", stage.Name).
				WithOperation("validate_stage").
				WithStage("tool_validation").
				WithRootCause(fmt.Sprintf("Tool %s is not available or invalid: %v", toolName, err)).
				WithImmediateStep(1, "Check tool name", "Verify the tool name is spelled correctly").
				WithImmediateStep(2, "Check registration", "Ensure the tool is properly registered").
				WithImmediateStep(3, "Check availability", "Verify tool dependencies are available").
				Build()
		}
	}
	return nil
}

// validateTimeout validates timeout configuration
func (sv *StageValidator) validateTimeout(stage *WorkflowStage) error {
	if stage.Timeout != nil && *stage.Timeout <= 0 {
		return mcptypes.NewErrorBuilder("VALIDATION_ERROR", "Stage timeout must be positive", "validation_error").
			WithField("stage_name", stage.Name).
			WithOperation("validate_stage").
			WithStage("timeout_validation").
			WithRootCause("Timeout value is zero or negative").
			WithImmediateStep(1, "Set positive timeout", "Use a positive timeout value (e.g., 30s, 5m)").
			WithImmediateStep(2, "Remove timeout", "Remove timeout to use default value").
			Build()
	}
	return nil
}

// validateRetryPolicy validates retry policy configuration
func (sv *StageValidator) validateRetryPolicy(stage *WorkflowStage) error {
	if stage.RetryPolicy == nil {
		return nil
	}

	policy := stage.RetryPolicy

	if policy.MaxAttempts < 0 {
		return fmt.Errorf("max retry attempts cannot be negative")
	}

	if policy.MaxAttempts > 10 {
		return fmt.Errorf("max retry attempts cannot exceed 10")
	}

	if policy.InitialDelay < 0 {
		return fmt.Errorf("initial delay cannot be negative")
	}

	if policy.MaxDelay > 0 && policy.MaxDelay < policy.InitialDelay {
		return fmt.Errorf("max delay must be greater than initial delay")
	}

	switch policy.BackoffMode {
	case "", "fixed", "exponential", "linear":
		// Valid modes
	default:
		return fmt.Errorf("invalid backoff mode: %s", policy.BackoffMode)
	}

	if policy.BackoffMode == "exponential" && policy.Multiplier <= 0 {
		return fmt.Errorf("multiplier must be positive for exponential backoff")
	}

	return nil
}

// validateConditions validates stage conditions
func (sv *StageValidator) validateConditions(stage *WorkflowStage) error {
	for i, condition := range stage.Conditions {
		if err := sv.validateCondition(&condition, i); err != nil {
			return fmt.Errorf("invalid condition %d for stage %s: %w", i, stage.Name, err)
		}
	}
	return nil
}

// validateCondition validates a single condition
func (sv *StageValidator) validateCondition(condition *StageCondition, index int) error {
	if condition.Key == "" {
		return fmt.Errorf("condition key is required at index %d", index)
	}

	validOperators := map[string]bool{
		"required":     true,
		"equals":       true,
		"not_equals":   true,
		"exists":       true,
		"not_exists":   true,
		"contains":     true,
		"not_contains": true,
	}

	if !validOperators[condition.Operator] {
		return fmt.Errorf("invalid operator '%s' at index %d", condition.Operator, index)
	}

	// Some operators require a value
	requiresValue := map[string]bool{
		"equals":       true,
		"not_equals":   true,
		"contains":     true,
		"not_contains": true,
	}

	if requiresValue[condition.Operator] && condition.Value == nil {
		return fmt.Errorf("operator '%s' requires a value at index %d", condition.Operator, index)
	}

	return nil
}

// validateFailureAction validates failure action configuration
func (sv *StageValidator) validateFailureAction(stage *WorkflowStage) error {
	if stage.OnFailure == nil {
		return nil
	}

	validActions := map[string]bool{
		"retry":    true,
		"redirect": true,
		"skip":     true,
		"fail":     true,
	}

	if !validActions[stage.OnFailure.Action] {
		return fmt.Errorf("invalid failure action: %s", stage.OnFailure.Action)
	}

	if stage.OnFailure.Action == "redirect" && stage.OnFailure.RedirectTo == "" {
		return fmt.Errorf("redirect action requires RedirectTo to be specified")
	}

	return nil
}
