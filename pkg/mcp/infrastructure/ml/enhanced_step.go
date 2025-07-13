// Package ml provides enhanced workflow steps with AI-powered error handling.
package ml

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// Step represents a workflow step interface
type Step interface {
	Name() string
	MaxRetries() int
	Execute(ctx context.Context, state interface{}) error
}

// WorkflowState represents the workflow state interface
type WorkflowState interface {
	GetWorkflowID() string
}

// EnhancedStep wraps a regular workflow step with AI-powered error analysis and retry logic
type EnhancedStep struct {
	step         Step
	errorHandler *EnhancedErrorHandler
	maxRetries   int
	logger       *slog.Logger
}

// NewEnhancedStep creates a new enhanced step with error analysis capabilities
func NewEnhancedStep(
	step Step,
	errorHandler *EnhancedErrorHandler,
	logger *slog.Logger,
) *EnhancedStep {
	return &EnhancedStep{
		step:         step,
		errorHandler: errorHandler,
		maxRetries:   step.MaxRetries(),
		logger:       logger.With("enhanced_step", step.Name()),
	}
}

// Name returns the name of the wrapped step
func (e *EnhancedStep) Name() string {
	return e.step.Name()
}

// MaxRetries returns the maximum number of retries for this step
func (e *EnhancedStep) MaxRetries() int {
	return e.maxRetries
}

// Execute runs the step with enhanced error handling and intelligent retry logic
func (e *EnhancedStep) Execute(ctx context.Context, state interface{}) error {
	stepName := e.step.Name()
	e.logger.Info("Starting enhanced step execution", "step", stepName)

	var lastError error
	var lastClassification *ErrorClassification

	for attempt := 1; attempt <= e.maxRetries+1; attempt++ {
		e.logger.Info("Executing step attempt",
			"step", stepName,
			"attempt", attempt,
			"max_attempts", e.maxRetries+1)

		// Execute the actual step
		startTime := time.Now()
		err := e.step.Execute(ctx, state)
		duration := time.Since(startTime)

		if err == nil {
			// Step succeeded
			e.logger.Info("Enhanced step completed successfully",
				"step", stepName,
				"attempt", attempt,
				"duration", duration)

			// Mark any previous errors as resolved
			if lastError != nil && e.errorHandler.recognizer.errorHistory != nil {
				if ws, ok := state.(WorkflowState); ok {
					e.errorHandler.recognizer.errorHistory.MarkResolved(ws.GetWorkflowID(), stepName)
				}
			}

			return nil
		}

		// Step failed - analyze the error
		lastError = err
		e.logger.Error("Step failed, analyzing error",
			"step", stepName,
			"attempt", attempt,
			"error", err.Error(),
			"duration", duration)

		// Get AI analysis of the error
		classification, analyzeErr := e.errorHandler.AnalyzeWorkflowError(
			ctx, err, state, stepName, e.getStepNumber(stepName))

		if analyzeErr != nil {
			e.logger.Error("Error analysis failed", "error", analyzeErr)
			// Continue with basic retry logic if AI analysis fails
			if attempt <= e.maxRetries {
				e.logger.Info("Using basic retry logic due to analysis failure")
				time.Sleep(time.Duration(30*attempt) * time.Second)
				continue
			}
			return err
		}

		lastClassification = classification

		// Decide whether to retry based on AI recommendation
		retryDecision := e.errorHandler.SuggestRetryStrategy(
			ctx, classification, attempt-1, e.maxRetries)

		if !retryDecision.ShouldRetry {
			e.logger.Info("AI recommends not to retry",
				"step", stepName,
				"reasoning", retryDecision.Reasoning)
			break
		}

		// Apply auto-fix if recommended
		if retryDecision.ModifyArgs && classification.AutoFixable {
			autoFixResult, fixErr := e.errorHandler.ApplyAutoFix(ctx, classification, state)
			if fixErr != nil {
				e.logger.Error("Auto-fix failed", "error", fixErr)
			} else if autoFixResult.Applied {
				e.logger.Info("Auto-fix applied",
					"description", autoFixResult.Description,
					"changes", autoFixResult.Changes)
			} else {
				e.logger.Info("Auto-fix not applied", "reason", autoFixResult.Reason)
			}
		}

		// Wait before retry if recommended
		if retryDecision.DelayDuration > 0 {
			e.logger.Info("Waiting before retry",
				"delay", retryDecision.DelayDuration,
				"reasoning", retryDecision.Reasoning)
			time.Sleep(retryDecision.DelayDuration)
		}

		e.logger.Info("Retrying step based on AI recommendation",
			"step", stepName,
			"attempt", attempt+1,
			"reasoning", retryDecision.Reasoning)
	}

	// All retries exhausted
	e.logger.Error("Enhanced step failed after all retries",
		"step", stepName,
		"max_attempts", e.maxRetries+1,
		"final_error", lastError.Error())

	// Return enriched error with AI analysis
	if lastClassification != nil {
		return e.createEnrichedError(lastError, lastClassification, stepName)
	}

	return lastError
}

// getStepNumber maps step names to their position in the workflow
func (e *EnhancedStep) getStepNumber(stepName string) int {
	stepMap := map[string]int{
		"analyze":    1,
		"dockerfile": 2,
		"build":      3,
		"scan":       4,
		"tag":        5,
		"push":       6,
		"manifest":   7,
		"cluster":    8,
		"deploy":     9,
		"verify":     10,
	}

	if num, exists := stepMap[stepName]; exists {
		return num
	}
	return 0
}

// createEnrichedError creates an error with AI analysis information
func (e *EnhancedStep) createEnrichedError(
	originalError error,
	classification *ErrorClassification,
	stepName string,
) error {

	enrichedMsg := fmt.Sprintf(`Step '%s' failed with AI analysis:

Original Error: %s

AI Analysis:
- Category: %s (confidence: %.2f)
- Severity: %s
- Auto-fixable: %v
- Suggested Fix: %s

Patterns: %v`,
		stepName,
		originalError.Error(),
		classification.Category,
		classification.Confidence,
		classification.Severity,
		classification.AutoFixable,
		classification.SuggestedFix,
		classification.Patterns)

	return fmt.Errorf("%s", enrichedMsg)
}

// StepEnhancer provides utilities for enhancing existing workflow steps
type StepEnhancer struct {
	errorHandler *EnhancedErrorHandler
	logger       *slog.Logger
}

// NewStepEnhancer creates a new step enhancer
func NewStepEnhancer(errorHandler *EnhancedErrorHandler, logger *slog.Logger) *StepEnhancer {
	return &StepEnhancer{
		errorHandler: errorHandler,
		logger:       logger,
	}
}

// EnhanceStep wraps a regular step with AI capabilities
func (se *StepEnhancer) EnhanceStep(step Step) Step {
	return NewEnhancedStep(step, se.errorHandler, se.logger)
}

// EnhanceAllSteps enhances a slice of workflow steps
func (se *StepEnhancer) EnhanceAllSteps(steps []Step) []Step {
	enhanced := make([]Step, len(steps))
	for i, step := range steps {
		enhanced[i] = se.EnhanceStep(step)
	}
	return enhanced
}

// CreateEnhancedWorkflow creates a workflow with all steps enhanced for AI error handling
func CreateEnhancedWorkflow(
	originalSteps []Step,
	errorHandler *EnhancedErrorHandler,
	logger *slog.Logger,
) []Step {

	enhancer := NewStepEnhancer(errorHandler, logger)
	return enhancer.EnhanceAllSteps(originalSteps)
}
