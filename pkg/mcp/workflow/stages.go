package workflow

import (
	"context"

	"github.com/Azure/container-kit/pkg/mcp/errors"
)

// StageValidator handles validation of workflow stages
type StageValidator struct {
	toolRegistry *ToolRegistry
}

// NewStageValidator creates a new stage validator
func NewStageValidator(toolRegistry *ToolRegistry) *StageValidator {
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
func (sv *StageValidator) validateBasicRequirements(_ context.Context, stage *WorkflowStage) error {
	if stage.Name == "" {
		return errors.NewError().
			Message("stage validation failed: missing name").
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Context("stage_id", stage.ID).
			Context("validation_type", "basic_requirements").
			WithLocation().
			Build()
	}

	if len(stage.Tools) == 0 {
		return errors.NewError().
			Message("stage must have at least one tool").
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Context("stage_id", stage.ID).
			Context("stage_name", stage.Name).
			Context("validation_type", "tool_count").
			WithLocation().
			Build()
	}

	return nil
}

func (sv *StageValidator) validateTools(stage *WorkflowStage) error {
	for _, toolName := range stage.Tools {
		if _, err := sv.toolRegistry.GetTool(toolName); err != nil {
			return errors.NewError().
				Messagef("invalid tool reference: %s", toolName).
				Code(errors.CodeToolNotFound).
				Type(errors.ErrTypeTool).
				Cause(err).
				Context("stage_id", stage.ID).
				Context("stage_name", stage.Name).
				Context("tool_name", toolName).
				WithLocation().
				Build()
		}
	}
	return nil
}

// validateTimeout validates timeout configuration
func (sv *StageValidator) validateTimeout(stage *WorkflowStage) error {
	if stage.Timeout != nil && *stage.Timeout <= 0 {
		return errors.NewError().
			Message("timeout must be positive").
			Code(errors.CodeInvalidParameter).
			Type(errors.ErrTypeValidation).
			Context("stage_id", stage.ID).
			Context("stage_name", stage.Name).
			Context("timeout", *stage.Timeout).
			WithLocation().
			Build()
	}
	return nil
}

func (sv *StageValidator) validateRetryPolicy(stage *WorkflowStage) error {
	if stage.RetryPolicy == nil {
		return nil
	}

	policy := stage.RetryPolicy

	if policy.MaxAttempts < 0 {
		return errors.NewError().
			Message("max retry attempts cannot be negative").
			Code(errors.CodeInvalidParameter).
			Type(errors.ErrTypeValidation).
			Context("stage_id", stage.ID).
			Context("stage_name", stage.Name).
			Context("max_attempts", policy.MaxAttempts).
			WithLocation().
			Build()
	}

	if policy.MaxAttempts > 10 {
		return errors.NewError().
			Message("max retry attempts cannot exceed 10").
			Code(errors.CodeInvalidParameter).
			Type(errors.ErrTypeValidation).
			Context("stage_id", stage.ID).
			Context("stage_name", stage.Name).
			Context("max_attempts", policy.MaxAttempts).
			Suggestion("Set max retry attempts to 10 or less").
			WithLocation().
			Build()
	}

	if policy.InitialDelay < 0 {
		return errors.NewError().
			Message("initial delay cannot be negative").
			Code(errors.CodeInvalidParameter).
			Type(errors.ErrTypeValidation).
			Context("stage_id", stage.ID).
			Context("stage_name", stage.Name).
			Context("initial_delay", policy.InitialDelay).
			WithLocation().
			Build()
	}

	if policy.MaxDelay > 0 && policy.MaxDelay < policy.InitialDelay {
		return errors.NewError().
			Message("max delay must be greater than initial delay").
			Code(errors.CodeInvalidParameter).
			Type(errors.ErrTypeValidation).
			Context("stage_id", stage.ID).
			Context("stage_name", stage.Name).
			Context("initial_delay", policy.InitialDelay).
			Context("max_delay", policy.MaxDelay).
			WithLocation().
			Build()
	}

	// BackoffMultiplier validation for exponential backoff
	if policy.BackoffMultiplier <= 0 {
		return errors.NewError().
			Message("backoff multiplier must be positive").
			Code(errors.CodeInvalidParameter).
			Type(errors.ErrTypeValidation).
			Context("stage_id", stage.ID).
			Context("stage_name", stage.Name).
			Context("backoff_multiplier", policy.BackoffMultiplier).
			Suggestion("Set backoff multiplier to a value greater than 0").
			WithLocation().
			Build()
	}

	return nil
}

func (sv *StageValidator) validateConditions(stage *WorkflowStage) error {
	for i, condition := range stage.Conditions {
		if err := sv.validateCondition(&condition, i); err != nil {
			return errors.NewError().
				Messagef("invalid condition %d for stage %s", i, stage.Name).
				Code(errors.CodeValidationFailed).
				Type(errors.ErrTypeValidation).
				Cause(err).
				Context("stage_id", stage.ID).
				Context("stage_name", stage.Name).
				Context("condition_index", i).
				WithLocation().
				Build()
		}
	}
	return nil
}

// validateCondition validates a single condition
func (sv *StageValidator) validateCondition(condition *StageCondition, index int) error {
	if condition.Key == "" {
		return errors.NewError().
			Messagef("condition key is required at index %d", index).
			Code(errors.CodeMissingParameter).
			Type(errors.ErrTypeValidation).
			Context("condition_index", index).
			Context("condition", condition).
			WithLocation().
			Build()
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
		return errors.NewError().
			Messagef("invalid operator '%s' at index %d", condition.Operator, index).
			Code(errors.CodeInvalidParameter).
			Type(errors.ErrTypeValidation).
			Context("condition_index", index).
			Context("operator", condition.Operator).
			Context("valid_operators", []string{"required", "equals", "not_equals", "exists", "not_exists", "contains", "not_contains"}).
			Suggestion("Use one of: required, equals, not_equals, exists, not_exists, contains, not_contains").
			WithLocation().
			Build()
	}

	requiresValue := map[string]bool{
		"equals":       true,
		"not_equals":   true,
		"contains":     true,
		"not_contains": true,
	}

	if requiresValue[condition.Operator] && condition.Value == nil {
		return errors.NewError().
			Messagef("operator '%s' requires a value at index %d", condition.Operator, index).
			Code(errors.CodeMissingParameter).
			Type(errors.ErrTypeValidation).
			Context("condition_index", index).
			Context("operator", condition.Operator).
			Suggestion("Provide a value for this operator").
			WithLocation().
			Build()
	}

	return nil
}

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
		return errors.NewError().
			Messagef("invalid failure action: %s", stage.OnFailure.Action).
			Code(errors.CodeInvalidParameter).
			Type(errors.ErrTypeValidation).
			Context("stage_id", stage.ID).
			Context("stage_name", stage.Name).
			Context("action", stage.OnFailure.Action).
			Context("valid_actions", []string{"retry", "redirect", "skip", "fail"}).
			Suggestion("Use one of: retry, redirect, skip, fail").
			WithLocation().
			Build()
	}

	if stage.OnFailure.Action == "redirect" && stage.OnFailure.RedirectTo == "" {
		return errors.NewError().
			Message("redirect action requires RedirectTo to be specified").
			Code(errors.CodeMissingParameter).
			Type(errors.ErrTypeValidation).
			Context("stage_id", stage.ID).
			Context("stage_name", stage.Name).
			Context("action", stage.OnFailure.Action).
			Suggestion("Specify the RedirectTo field when using redirect action").
			WithLocation().
			Build()
	}

	return nil
}
