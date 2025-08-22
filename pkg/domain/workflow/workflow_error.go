// Package workflow provides simple error handling for workflow execution
package workflow

import "fmt"

// WorkflowError represents a simple workflow error with step and attempt information
type WorkflowError struct {
	Step    string
	Attempt int
	Err     error
}

func (w *WorkflowError) Error() string {
	if w.Attempt > 1 {
		return fmt.Sprintf("step '%s' failed on attempt %d: %v", w.Step, w.Attempt, w.Err)
	}
	return fmt.Sprintf("step '%s' failed: %v", w.Step, w.Err)
}

// Unwrap implements the error unwrapping interface for Go 1.13+ error handling.
// This allows errors.Is() and errors.As() to work with WorkflowError.
// Although deadcode reports this as unused, it's required for proper error chain handling.
func (w *WorkflowError) Unwrap() error {
	return w.Err
}

func NewWorkflowError(step string, attempt int, err error) *WorkflowError {
	return &WorkflowError{
		Step:    step,
		Attempt: attempt,
		Err:     err,
	}
}
