package orchestration

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
)

// ConditionalExecutor handles conditional execution of workflow stage tools
type ConditionalExecutor struct {
	logger       zerolog.Logger
	baseExecutor Executor // Can be sequential or parallel
}

// NewConditionalExecutor creates a new conditional executor
func NewConditionalExecutor(logger zerolog.Logger, baseExecutor Executor) *ConditionalExecutor {
	return &ConditionalExecutor{
		logger:       logger.With().Str("executor", "conditional").Logger(),
		baseExecutor: baseExecutor,
	}
}

// Execute runs tools based on condition evaluation
func (ce *ConditionalExecutor) Execute(
	ctx context.Context,
	stage *WorkflowStage,
	session *WorkflowSession,
	toolNames []string,
	executeToolFunc ExecuteToolFunc,
) (*ExecutionResult, error) {
	ce.logger.Debug().
		Str("stage_name", stage.Name).
		Int("condition_count", len(stage.Conditions)).
		Msg("Evaluating stage conditions")

	// Evaluate conditions
	if !ce.evaluateConditions(stage.Conditions, session) {
		ce.logger.Info().
			Str("stage_name", stage.Name).
			Msg("Stage conditions not met, skipping execution")

		return &ExecutionResult{
			Success:   true,
			Results:   map[string]interface{}{"skipped": true, "reason": "conditions not met"},
			Artifacts: []WorkflowArtifact{},
			Metrics: map[string]interface{}{
				"execution_type": "conditional",
				"skipped":        true,
			},
		}, nil
	}

	ce.logger.Info().
		Str("stage_name", stage.Name).
		Msg("Stage conditions met, proceeding with execution")

	// Conditions met, execute using base executor
	result, err := ce.baseExecutor.Execute(ctx, stage, session, toolNames, executeToolFunc)

	// Add conditional execution metadata to metrics
	if result.Metrics == nil {
		result.Metrics = make(map[string]interface{})
	}
	result.Metrics["conditional_execution"] = true
	result.Metrics["conditions_evaluated"] = len(stage.Conditions)

	return result, err
}

// evaluateConditions checks if all conditions are met
func (ce *ConditionalExecutor) evaluateConditions(conditions []StageCondition, session *WorkflowSession) bool {
	if len(conditions) == 0 {
		return true
	}

	for _, condition := range conditions {
		if !ce.evaluateCondition(&condition, session) {
			ce.logger.Debug().
				Str("condition_key", condition.Key).
				Str("operator", condition.Operator).
				Interface("expected_value", condition.Value).
				Msg("Condition not met")
			return false
		}
	}

	return true
}

// evaluateCondition checks a single condition
func (ce *ConditionalExecutor) evaluateCondition(condition *StageCondition, session *WorkflowSession) bool {
	// Get value from shared context
	value, exists := session.SharedContext[condition.Key]

	switch condition.Operator {
	case "required", "exists":
		return exists

	case "not_exists":
		return !exists

	case "equals":
		if !exists {
			return false
		}
		return ce.compareValues(value, condition.Value)

	case "not_equals":
		if !exists {
			return true
		}
		return !ce.compareValues(value, condition.Value)

	case "contains":
		if !exists {
			return false
		}
		return ce.containsValue(value, condition.Value)

	case "not_contains":
		if !exists {
			return true
		}
		return !ce.containsValue(value, condition.Value)

	default:
		ce.logger.Warn().
			Str("operator", condition.Operator).
			Msg("Unknown condition operator, evaluating to false")
		return false
	}
}

// compareValues compares two values for equality
func (ce *ConditionalExecutor) compareValues(actual, expected interface{}) bool {
	// Simple equality check
	// In a production system, this would handle type conversions more robustly
	return fmt.Sprintf("%v", actual) == fmt.Sprintf("%v", expected)
}

// containsValue checks if actual contains expected
func (ce *ConditionalExecutor) containsValue(actual, expected interface{}) bool {
	// Convert to strings for simple contains check
	actualStr := fmt.Sprintf("%v", actual)
	expectedStr := fmt.Sprintf("%v", expected)

	// Check if actual string contains expected string
	return actualStr != "" && expectedStr != "" &&
		len(actualStr) >= len(expectedStr) &&
		actualStr[0:len(expectedStr)] == expectedStr
}
