// Package errors provides workflow-aware error handling extensions
package errors

import (
	"fmt"
	"runtime"
	"strings"
)

// WorkflowError extends Rich error with workflow-specific context
type WorkflowError struct {
	*Rich
	Step           string                 `json:"step"`
	Attempt        int                    `json:"attempt"`
	FixesAttempted []string               `json:"fixes_attempted,omitempty"`
	WorkflowID     string                 `json:"workflow_id,omitempty"`
	StepContext    map[string]interface{} `json:"step_context,omitempty"`
}

// NewWorkflowError creates a workflow-aware error
func NewWorkflowError(code Code, domain, step, msg string, cause error) *WorkflowError {
	rich := New(code, domain, msg, cause)
	return &WorkflowError{
		Rich:        rich,
		Step:        step,
		Attempt:     1,
		StepContext: make(map[string]interface{}),
	}
}

// WithWorkflowID adds workflow ID to the error
func (we *WorkflowError) WithWorkflowID(id string) *WorkflowError {
	we.WorkflowID = id
	return we
}

// WithAttempt sets the attempt number
func (we *WorkflowError) WithAttempt(attempt int) *WorkflowError {
	we.Attempt = attempt
	return we
}

// AddFixAttempt records a fix that was attempted
func (we *WorkflowError) AddFixAttempt(fix string) *WorkflowError {
	we.FixesAttempted = append(we.FixesAttempted, fix)
	return we
}

// WithStepContext adds step-specific context
func (we *WorkflowError) WithStepContext(key string, value interface{}) *WorkflowError {
	we.StepContext[key] = value
	return we
}

// Error implements the error interface with workflow details
func (we *WorkflowError) Error() string {
	base := we.Rich.Error()
	if we.Step != "" {
		base = fmt.Sprintf("[%s] %s", we.Step, base)
	}
	if we.Attempt > 1 {
		base = fmt.Sprintf("%s (attempt %d)", base, we.Attempt)
	}
	return base
}

// WorkflowErrorHistory maintains error history for AI-assisted recovery
type WorkflowErrorHistory struct {
	errors      []*WorkflowError
	maxHistory  int
	stepSummary map[string][]string // Error summaries per step
}

// NewWorkflowErrorHistory creates a new error history tracker
func NewWorkflowErrorHistory(maxHistory int) *WorkflowErrorHistory {
	if maxHistory <= 0 {
		maxHistory = 10
	}
	return &WorkflowErrorHistory{
		errors:      make([]*WorkflowError, 0, maxHistory),
		maxHistory:  maxHistory,
		stepSummary: make(map[string][]string),
	}
}

// AddError adds a workflow error to the history
func (weh *WorkflowErrorHistory) AddError(err *WorkflowError) {
	weh.errors = append(weh.errors, err)

	// Maintain max history
	if len(weh.errors) > weh.maxHistory {
		weh.errors = weh.errors[len(weh.errors)-weh.maxHistory:]
	}

	// Update step summary
	if err.Step != "" {
		weh.stepSummary[err.Step] = append(weh.stepSummary[err.Step], err.Message)
	}
}

// GetRecentErrors returns the most recent errors
func (weh *WorkflowErrorHistory) GetRecentErrors(count int) []*WorkflowError {
	if count > len(weh.errors) {
		count = len(weh.errors)
	}

	start := len(weh.errors) - count
	if start < 0 {
		start = 0
	}

	return weh.errors[start:]
}

// GetStepErrors returns all errors for a specific step
func (weh *WorkflowErrorHistory) GetStepErrors(step string) []*WorkflowError {
	var stepErrors []*WorkflowError
	for _, err := range weh.errors {
		if err.Step == step {
			stepErrors = append(stepErrors, err)
		}
	}
	return stepErrors
}

// HasRepeatedErrors checks if the same error has occurred multiple times
func (weh *WorkflowErrorHistory) HasRepeatedErrors(threshold int) bool {
	errorCounts := make(map[string]int)
	for _, err := range weh.errors {
		key := fmt.Sprintf("%s:%s", err.Step, err.Code)
		errorCounts[key]++
		if errorCounts[key] >= threshold {
			return true
		}
	}
	return false
}

// GetAISummary generates a summary suitable for AI context
func (weh *WorkflowErrorHistory) GetAISummary() string {
	var summary strings.Builder

	summary.WriteString("Error History Summary:\n")

	// Summarize by step
	for step, errors := range weh.stepSummary {
		summary.WriteString(fmt.Sprintf("\n%s (%d errors):\n", step, len(errors)))
		// Show unique errors
		seen := make(map[string]bool)
		for _, err := range errors {
			if !seen[err] {
				summary.WriteString(fmt.Sprintf("  - %s\n", err))
				seen[err] = true
			}
		}
	}

	// Recent errors with fixes attempted
	recent := weh.GetRecentErrors(3)
	if len(recent) > 0 {
		summary.WriteString("\nRecent Errors:\n")
		for _, err := range recent {
			summary.WriteString(fmt.Sprintf("  - [%s] %s\n", err.Step, err.Message))
			if len(err.FixesAttempted) > 0 {
				summary.WriteString(fmt.Sprintf("    Fixes attempted: %s\n", strings.Join(err.FixesAttempted, ", ")))
			}
		}
	}

	return summary.String()
}

// Builder provides a fluent interface for creating workflow errors
type WorkflowErrorBuilder struct {
	err *WorkflowError
}

// NewWorkflowErrorBuilder creates a new error builder
func NewWorkflowErrorBuilder() *WorkflowErrorBuilder {
	return &WorkflowErrorBuilder{
		err: &WorkflowError{
			Rich: &Rich{
				Fields: make(map[string]any),
			},
			StepContext: make(map[string]interface{}),
		},
	}
}

// Code sets the error code
func (b *WorkflowErrorBuilder) Code(code Code) *WorkflowErrorBuilder {
	b.err.Code = code
	// Apply metadata from generated code
	if _, sev, retry, exists := GetCodeMetadata(code); exists {
		b.err.Severity = sev
		b.err.Retryable = retry
	}
	return b
}

// Step sets the workflow step
func (b *WorkflowErrorBuilder) Step(step string) *WorkflowErrorBuilder {
	b.err.Step = step
	return b
}

// Message sets the error message
func (b *WorkflowErrorBuilder) Message(msg string) *WorkflowErrorBuilder {
	b.err.Message = msg
	return b
}

// Domain sets the error domain
func (b *WorkflowErrorBuilder) Domain(domain string) *WorkflowErrorBuilder {
	b.err.Domain = domain
	return b
}

// Cause sets the underlying error
func (b *WorkflowErrorBuilder) Cause(err error) *WorkflowErrorBuilder {
	b.err.Cause = err
	return b
}

// WithContext adds step context
func (b *WorkflowErrorBuilder) WithContext(key string, value interface{}) *WorkflowErrorBuilder {
	b.err.StepContext[key] = value
	return b
}

// Build creates the final error
func (b *WorkflowErrorBuilder) Build() *WorkflowError {
	// Set location
	_, file, line, _ := runtime.Caller(1)
	b.err.Location = fmt.Sprintf("%s:%d", file, line)

	// Set defaults
	if b.err.Severity == 0 {
		b.err.Severity = SeverityMedium
	}
	if b.err.UserFacing == false {
		b.err.UserFacing = true
	}

	return b.err
}
