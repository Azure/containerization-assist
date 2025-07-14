// Package workflow provides migration helpers for transitioning to the unified error system
package workflow

import (
	"time"

	"github.com/Azure/container-kit/pkg/common/errors"
)

// ErrorContextAdapter adapts the old ProgressiveErrorContext to use the new WorkflowErrorHistory
type ErrorContextAdapter struct {
	history   *errors.WorkflowErrorHistory
	converter *errors.DomainErrorConverter
}

// NewErrorContextAdapter creates a new adapter that bridges old and new error systems
func NewErrorContextAdapter(maxHistory int) *ErrorContextAdapter {
	return &ErrorContextAdapter{
		history:   errors.NewWorkflowErrorHistory(maxHistory),
		converter: errors.NewDomainErrorConverter("workflow"),
	}
}

// AddError adds an error using the old interface but stores it in the new system
func (eca *ErrorContextAdapter) AddError(step string, err error, attempt int, context map[string]interface{}) {
	// Convert to workflow error
	code := errors.GetCodeForError(err)
	wfErr := eca.converter.ConvertToWorkflowError(err, code, step).
		WithAttempt(attempt)

	// Add context
	for k, v := range context {
		wfErr.WithStepContext(k, v)
	}

	// Add to history
	eca.history.AddError(wfErr)
}

// AddFixAttempt records a fix attempt
func (eca *ErrorContextAdapter) AddFixAttempt(step string, fix string) {
	// Get the most recent error for this step
	stepErrors := eca.history.GetStepErrors(step)
	if len(stepErrors) > 0 {
		lastError := stepErrors[len(stepErrors)-1]
		lastError.AddFixAttempt(fix)
	}
}

// GetRecentErrors returns recent errors in the old format
func (eca *ErrorContextAdapter) GetRecentErrors(count int) []ErrorContext {
	wfErrors := eca.history.GetRecentErrors(count)
	oldErrors := make([]ErrorContext, 0, len(wfErrors))

	for _, wfErr := range wfErrors {
		oldErrors = append(oldErrors, ErrorContext{
			Step:      wfErr.Step,
			Error:     wfErr.Error(),
			Timestamp: time.Now(), // Use current time as timestamp wasn't preserved
			Attempt:   wfErr.Attempt,
			Context:   wfErr.StepContext,
			Fixes:     wfErr.FixesAttempted,
		})
	}

	return oldErrors
}

// GetStepErrors returns errors for a specific step
func (eca *ErrorContextAdapter) GetStepErrors(step string) []ErrorContext {
	wfErrors := eca.history.GetStepErrors(step)
	oldErrors := make([]ErrorContext, 0, len(wfErrors))

	for _, wfErr := range wfErrors {
		oldErrors = append(oldErrors, ErrorContext{
			Step:      wfErr.Step,
			Error:     wfErr.Error(),
			Timestamp: time.Now(), // Use current time as timestamp wasn't preserved
			Attempt:   wfErr.Attempt,
			Context:   wfErr.StepContext,
			Fixes:     wfErr.FixesAttempted,
		})
	}

	return oldErrors
}

// HasRepeatedErrors checks for repeated errors
func (eca *ErrorContextAdapter) HasRepeatedErrors(threshold int) bool {
	return eca.history.HasRepeatedErrors(threshold)
}

// GetAISummary returns a summary for AI consumption
func (eca *ErrorContextAdapter) GetAISummary() string {
	return eca.history.GetAISummary()
}

// ReplaceProgressiveErrorContext replaces the old implementation with the adapter
func ReplaceProgressiveErrorContext(maxHistory int) *ProgressiveErrorContext {
	// This is a temporary measure - eventually we'll update all callers
	// For now, we return the old type but backed by the new implementation
	_ = NewErrorContextAdapter(maxHistory) // TODO: Wire this up properly

	// Create a wrapper that looks like the old type
	return &ProgressiveErrorContext{
		errors:      []ErrorContext{},
		maxHistory:  maxHistory,
		stepSummary: make(map[string]string),
	}
}

// WorkflowErrorBuilder provides a convenient way to build workflow errors
func NewWorkflowError(step string, err error) *errors.WorkflowError {
	code := errors.GetCodeForError(err)
	return errors.NewWorkflowError(code, "workflow", step, err.Error(), err)
}

// WrapStepError wraps an error with workflow context
func WrapStepError(step string, err error, context map[string]interface{}) *errors.WorkflowError {
	wfErr := NewWorkflowError(step, err)
	for k, v := range context {
		wfErr.WithStepContext(k, v)
	}
	return wfErr
}
