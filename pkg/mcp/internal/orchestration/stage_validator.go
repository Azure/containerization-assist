package orchestration

import (
	"fmt"

	"github.com/Azure/container-copilot/pkg/mcp/internal/workflow"
	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
)

// StageValidator handles validation of workflow stages
type StageValidator struct {
	toolRegistry mcptypes.ToolRegistry
}

// NewStageValidator creates a new stage validator
func NewStageValidator(toolRegistry mcptypes.ToolRegistry) *StageValidator {
	return &StageValidator{
		toolRegistry: toolRegistry,
	}
}

// Validate validates a workflow stage configuration
func (sv *StageValidator) Validate(stage *workflow.WorkflowStage) error {
	// Basic validation
	if err := sv.validateBasicRequirements(stage); err != nil {
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
func (sv *StageValidator) validateBasicRequirements(stage *workflow.WorkflowStage) error {
	if stage.Name == "" {
		return fmt.Errorf("stage name is required")
	}

	if len(stage.Tools) == 0 {
		return fmt.Errorf("stage must specify at least one tool")
	}

	return nil
}

// validateTools validates that all tools exist and are available
func (sv *StageValidator) validateTools(stage *workflow.WorkflowStage) error {
	for _, toolName := range stage.Tools {
		if _, err := sv.toolRegistry.Create(toolName); err != nil {
			return fmt.Errorf("invalid tool %s in stage %s: %w", toolName, stage.Name, err)
		}
	}
	return nil
}

// validateTimeout validates timeout configuration
func (sv *StageValidator) validateTimeout(stage *workflow.WorkflowStage) error {
	if stage.Timeout != nil && *stage.Timeout <= 0 {
		return fmt.Errorf("stage timeout must be positive")
	}
	return nil
}

// validateRetryPolicy validates retry policy configuration
func (sv *StageValidator) validateRetryPolicy(stage *workflow.WorkflowStage) error {
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
func (sv *StageValidator) validateConditions(stage *workflow.WorkflowStage) error {
	for i, condition := range stage.Conditions {
		if err := sv.validateCondition(&condition, i); err != nil {
			return fmt.Errorf("invalid condition %d for stage %s: %w", i, stage.Name, err)
		}
	}
	return nil
}

// validateCondition validates a single condition
func (sv *StageValidator) validateCondition(condition *workflow.StageCondition, index int) error {
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
func (sv *StageValidator) validateFailureAction(stage *workflow.WorkflowStage) error {
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
