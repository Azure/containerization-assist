// Package errors provides structured error handling patterns for Container Kit MCP
package errors

import (
	"fmt"
	"log/slog"
	"time"
)

// ErrorSeverity represents the severity level of an error
type ErrorSeverity string

const (
	SeverityCritical ErrorSeverity = "critical"
	SeverityHigh     ErrorSeverity = "high"
	SeverityMedium   ErrorSeverity = "medium"
	SeverityLow      ErrorSeverity = "low"
	SeverityInfo     ErrorSeverity = "info"
)

// ErrorCategory represents the category of error for better classification
type ErrorCategory string

const (
	// Infrastructure categories
	CategoryInfrastructure ErrorCategory = "infrastructure"
	CategoryValidation     ErrorCategory = "validation"
	CategorySecurity       ErrorCategory = "security"
	CategoryNetwork        ErrorCategory = "network"
	CategoryFileSystem     ErrorCategory = "filesystem"
	CategoryDatabase       ErrorCategory = "database"

	// Business logic categories
	CategoryWorkflow ErrorCategory = "workflow"
	CategoryTemplate ErrorCategory = "template"
	CategorySampling ErrorCategory = "sampling"
	CategoryAnalysis ErrorCategory = "analysis"

	// External categories
	CategoryDocker     ErrorCategory = "docker"
	CategoryKubernetes ErrorCategory = "kubernetes"
	CategoryAI         ErrorCategory = "ai"
)

// StructuredError provides a unified error interface for all Container Kit operations
type StructuredError struct {
	// Core identification
	ID        string `json:"id"`        // Unique error identifier
	Operation string `json:"operation"` // Operation that failed
	Component string `json:"component"` // Component where error occurred
	Message   string `json:"message"`   // Human-readable message

	// Classification
	Category ErrorCategory `json:"category"` // Error category
	Severity ErrorSeverity `json:"severity"` // Error severity

	// Technical details
	Cause      error                  `json:"-"`                    // Underlying cause (not serialized)
	Context    map[string]interface{} `json:"context"`              // Additional context
	Stacktrace string                 `json:"stacktrace,omitempty"` // Stack trace if available

	// Recovery information
	Recoverable bool           `json:"recoverable"`           // Can this be automatically recovered?
	RetryAfter  *time.Duration `json:"retry_after,omitempty"` // Suggested retry delay

	// Metadata
	Timestamp  time.Time `json:"timestamp"`             // When error occurred
	WorkflowID string    `json:"workflow_id,omitempty"` // Associated workflow
	SessionID  string    `json:"session_id,omitempty"`  // Associated session
}

// Error implements the error interface
func (e *StructuredError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s:%s] %s failed in %s: %s (caused by: %v)",
			e.Category, e.Severity, e.Operation, e.Component, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s:%s] %s failed in %s: %s",
		e.Category, e.Severity, e.Operation, e.Component, e.Message)
}

// Unwrap implements error unwrapping for Go 1.13+
func (e *StructuredError) Unwrap() error {
	return e.Cause
}

// IsRecoverable returns whether this error can be automatically recovered from
func (e *StructuredError) IsRecoverable() bool {
	return e.Recoverable
}

// GetRetryAfter returns the suggested retry delay
func (e *StructuredError) GetRetryAfter() *time.Duration {
	return e.RetryAfter
}

// WithContext adds context information to the error
func (e *StructuredError) WithContext(key string, value interface{}) *StructuredError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// WithWorkflowID adds workflow context
func (e *StructuredError) WithWorkflowID(workflowID string) *StructuredError {
	e.WorkflowID = workflowID
	return e
}

// WithSessionID adds session context
func (e *StructuredError) WithSessionID(sessionID string) *StructuredError {
	e.SessionID = sessionID
	return e
}

// WithRetryAfter suggests a retry delay
func (e *StructuredError) WithRetryAfter(delay time.Duration) *StructuredError {
	e.RetryAfter = &delay
	return e
}

// LogStructured logs the error with structured context using slog
func (e *StructuredError) LogStructured(logger *slog.Logger) {
	// Build structured log arguments
	args := []interface{}{
		"error_id", e.ID,
		"operation", e.Operation,
		"component", e.Component,
		"category", string(e.Category),
		"severity", string(e.Severity),
		"recoverable", e.Recoverable,
		"timestamp", e.Timestamp,
	}

	// Add workflow and session context if present
	if e.WorkflowID != "" {
		args = append(args, "workflow_id", e.WorkflowID)
	}
	if e.SessionID != "" {
		args = append(args, "session_id", e.SessionID)
	}

	// Add retry information if present
	if e.RetryAfter != nil {
		args = append(args, "retry_after", *e.RetryAfter)
	}

	// Add context fields
	for k, v := range e.Context {
		args = append(args, k, v)
	}

	// Add cause if present
	if e.Cause != nil {
		args = append(args, "cause", e.Cause.Error())
	}

	// Add stacktrace if present
	if e.Stacktrace != "" {
		args = append(args, "stacktrace", e.Stacktrace)
	}

	// Log at appropriate level based on severity
	switch e.Severity {
	case SeverityCritical, SeverityHigh:
		logger.Error(e.Message, args...)
	case SeverityMedium:
		logger.Warn(e.Message, args...)
	case SeverityLow, SeverityInfo:
		logger.Info(e.Message, args...)
	default:
		logger.Error(e.Message, args...)
	}
}
