package workflow

import (
	"context"
	"fmt"

	"github.com/Azure/container-kit/pkg/mcp/application/orchestration/execution"
	"github.com/Azure/container-kit/pkg/mcp/application/orchestration/registry"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// StageValidator handles validation of workflow stages
type StageValidator struct {
	toolRegistry *registry.ToolRegistry
}

// NewStageValidator creates a new stage validator
func NewStageValidator(toolRegistry *registry.ToolRegistry) *StageValidator {
	return &StageValidator{
		toolRegistry: toolRegistry,
	}
}

// Validate validates a workflow stage configuration
func (sv *StageValidator) Validate(ctx context.Context, stage *execution.WorkflowStage) error {
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
func (sv *StageValidator) validateBasicRequirements(_ context.Context, stage *execution.WorkflowStage) error {
	if stage.Name == "" {
		return errors.NewError().Messagef("validation error").Build()
	}

	if len(stage.Tools) == 0 {
		return errors.NewError().Messagef("validation error").Build(

		// validateTools validates that all tools exist and are available
		)
	}

	return nil
}

func (sv *StageValidator) validateTools(stage *execution.WorkflowStage) error {
	for _, toolName := range stage.Tools {
		if _, err := sv.toolRegistry.GetTool(toolName); err != nil {
			return errors.NewError().Messagef("validation error").Build()
		}
	}
	return nil
}

// validateTimeout validates timeout configuration
func (sv *StageValidator) validateTimeout(stage *execution.WorkflowStage) error {
	if stage.Timeout != nil && *stage.Timeout <= 0 {
		return errors.NewError().Messagef("validation error").Build(

		// validateRetryPolicy validates retry policy configuration
		)
	}
	return nil
}

func (sv *StageValidator) validateRetryPolicy(stage *execution.WorkflowStage) error {
	if stage.RetryPolicy == nil {
		return nil
	}

	policy := stage.RetryPolicy

	if policy.MaxAttempts < 0 {
		return errors.NewError().Messagef("max retry attempts cannot be negative").Build()
	}

	if policy.MaxAttempts > 10 {
		return errors.NewError().Messagef("max retry attempts cannot exceed 10").Build()
	}

	if policy.InitialDelay < 0 {
		return errors.NewError().Messagef("initial delay cannot be negative").Build()
	}

	if policy.MaxDelay > 0 && policy.MaxDelay < policy.InitialDelay {
		return errors.NewError().Messagef("max delay must be greater than initial delay").Build()
	}

	switch policy.BackoffMode {
	case "", "fixed", "exponential", "linear":
		// Valid modes
	default:
		return errors.NewError().Messagef("invalid backoff mode: %s", policy.BackoffMode).Build()
	}

	if policy.BackoffMode == "exponential" && policy.Multiplier <= 0 {
		return errors.NewError().Messagef("multiplier must be positive for exponential backoff").Build(

		// validateConditions validates stage conditions
		)
	}

	return nil
}

func (sv *StageValidator) validateConditions(stage *execution.WorkflowStage) error {
	for i, condition := range stage.Conditions {
		if err := sv.validateCondition(&condition, i); err != nil {
			return errors.NewError().Message(fmt.Sprintf("invalid condition %d for stage %s", i, stage.Name)).Cause(err).Build()
		}
	}
	return nil
}

// validateCondition validates a single condition
func (sv *StageValidator) validateCondition(condition *execution.StageCondition, index int) error {
	if condition.Key == "" {
		return errors.NewError().Messagef("condition key is required at index %d", index).Build()
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
		return errors.NewError().Messagef("invalid operator '%s' at index %d", condition.Operator, index).WithLocation(

		// Some operators require a value
		).Build()
	}

	requiresValue := map[string]bool{
		"equals":       true,
		"not_equals":   true,
		"contains":     true,
		"not_contains": true,
	}

	if requiresValue[condition.Operator] && condition.Value == nil {
		return errors.NewError().Messagef("operator '%s' requires a value at index %d", condition.Operator, index).Build(

		// validateFailureAction validates failure action configuration
		)
	}

	return nil
}

func (sv *StageValidator) validateFailureAction(stage *execution.WorkflowStage) error {
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
		return errors.NewError().Messagef("invalid failure action: %s", stage.OnFailure.Action).Build()
	}

	if stage.OnFailure.Action == "redirect" && stage.OnFailure.RedirectTo == "" {
		return errors.NewError().Messagef("redirect action requires RedirectTo to be specified").Build()
	}

	return nil
}
