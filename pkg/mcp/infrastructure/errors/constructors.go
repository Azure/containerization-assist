// Package errors provides error construction utilities
package errors

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"runtime"
	"strings"
	"time"
)

// ErrorBuilder provides a fluent interface for building structured errors
type ErrorBuilder struct {
	err *StructuredError
}

// NewError creates a new error builder with basic information
func NewError(operation, component, message string) *ErrorBuilder {
	return &ErrorBuilder{
		err: &StructuredError{
			ID:        generateErrorID(),
			Operation: operation,
			Component: component,
			Message:   message,
			Context:   make(map[string]interface{}),
			Timestamp: time.Now(),
			Category:  CategoryInfrastructure, // Default category
			Severity:  SeverityMedium,         // Default severity
		},
	}
}

// WithCategory sets the error category
func (b *ErrorBuilder) WithCategory(category ErrorCategory) *ErrorBuilder {
	b.err.Category = category
	return b
}

// WithSeverity sets the error severity
func (b *ErrorBuilder) WithSeverity(severity ErrorSeverity) *ErrorBuilder {
	b.err.Severity = severity
	return b
}

// WithCause sets the underlying cause
func (b *ErrorBuilder) WithCause(cause error) *ErrorBuilder {
	b.err.Cause = cause
	return b
}

// WithRecoverable marks the error as recoverable
func (b *ErrorBuilder) WithRecoverable(recoverable bool) *ErrorBuilder {
	b.err.Recoverable = recoverable
	return b
}

// WithStacktrace captures the current stack trace
func (b *ErrorBuilder) WithStacktrace() *ErrorBuilder {
	b.err.Stacktrace = captureStacktrace()
	return b
}

// WithContext adds context information
func (b *ErrorBuilder) WithContext(key string, value interface{}) *ErrorBuilder {
	b.err.Context[key] = value
	return b
}

// WithWorkflowID adds workflow context
func (b *ErrorBuilder) WithWorkflowID(workflowID string) *ErrorBuilder {
	b.err.WorkflowID = workflowID
	return b
}

// WithSessionID adds session context
func (b *ErrorBuilder) WithSessionID(sessionID string) *ErrorBuilder {
	b.err.SessionID = sessionID
	return b
}

// WithRetryAfter suggests a retry delay
func (b *ErrorBuilder) WithRetryAfter(delay time.Duration) *ErrorBuilder {
	b.err.RetryAfter = &delay
	return b
}

// Build returns the constructed error
func (b *ErrorBuilder) Build() *StructuredError {
	return b.err
}

// Convenience constructors for common error patterns

// NewInfrastructureError creates an infrastructure-related error
func NewInfrastructureError(operation, component, message string, cause error) *StructuredError {
	return NewError(operation, component, message).
		WithCategory(CategoryInfrastructure).
		WithSeverity(SeverityHigh).
		WithCause(cause).
		Build()
}

// NewValidationError creates a validation error
func NewValidationError(field, message string) *StructuredError {
	return NewError("validate", "validator", message).
		WithCategory(CategoryValidation).
		WithSeverity(SeverityMedium).
		WithContext("field", field).
		Build()
}

// NewSecurityError creates a security-related error
func NewSecurityError(operation, component, message string) *StructuredError {
	return NewError(operation, component, message).
		WithCategory(CategorySecurity).
		WithSeverity(SeverityCritical).
		WithRecoverable(false).
		Build()
}

// NewWorkflowError creates a workflow-related error
func NewWorkflowError(step, message string, cause error) *StructuredError {
	return NewError("execute_step", "workflow", message).
		WithCategory(CategoryWorkflow).
		WithSeverity(SeverityHigh).
		WithCause(cause).
		WithContext("step", step).
		WithRecoverable(true).
		Build()
}

// NewNetworkError creates a network-related error
func NewNetworkError(operation, endpoint, message string, cause error) *StructuredError {
	return NewError(operation, "network", message).
		WithCategory(CategoryNetwork).
		WithSeverity(SeverityHigh).
		WithCause(cause).
		WithContext("endpoint", endpoint).
		WithRecoverable(true).
		WithRetryAfter(time.Second * 5).
		Build()
}

// NewDockerError creates a Docker-related error
func NewDockerError(operation, message string, cause error) *StructuredError {
	return NewError(operation, "docker", message).
		WithCategory(CategoryDocker).
		WithSeverity(SeverityHigh).
		WithCause(cause).
		WithRecoverable(true).
		Build()
}

// NewKubernetesError creates a Kubernetes-related error
func NewKubernetesError(operation, resource, message string, cause error) *StructuredError {
	return NewError(operation, "kubernetes", message).
		WithCategory(CategoryKubernetes).
		WithSeverity(SeverityHigh).
		WithCause(cause).
		WithContext("resource", resource).
		WithRecoverable(true).
		Build()
}

// NewAIError creates an AI/ML-related error
func NewAIError(operation, model, message string, cause error) *StructuredError {
	return NewError(operation, "ai", message).
		WithCategory(CategoryAI).
		WithSeverity(SeverityMedium).
		WithCause(cause).
		WithContext("model", model).
		WithRecoverable(true).
		WithRetryAfter(time.Second * 10).
		Build()
}

// NewTemplateError creates a template-related error
func NewTemplateError(templateID, operation, message string, cause error) *StructuredError {
	return NewError(operation, "template", message).
		WithCategory(CategoryTemplate).
		WithSeverity(SeverityMedium).
		WithCause(cause).
		WithContext("template_id", templateID).
		WithRecoverable(false).
		Build()
}

// NewFileSystemError creates a filesystem-related error
func NewFileSystemError(operation, path, message string, cause error) *StructuredError {
	return NewError(operation, "filesystem", message).
		WithCategory(CategoryFileSystem).
		WithSeverity(SeverityHigh).
		WithCause(cause).
		WithContext("path", path).
		WithRecoverable(true).
		Build()
}

// NewDatabaseError creates a database-related error
func NewDatabaseError(operation, message string, cause error) *StructuredError {
	return NewError(operation, "database", message).
		WithCategory(CategoryDatabase).
		WithSeverity(SeverityHigh).
		WithCause(cause).
		WithRecoverable(true).
		WithRetryAfter(time.Second * 3).
		Build()
}

// Utility functions

// generateErrorID creates a unique error identifier
func generateErrorID() string {
	bytes := make([]byte, 4)
	rand.Read(bytes)
	return "err_" + hex.EncodeToString(bytes)
}

// captureStacktrace captures the current call stack
func captureStacktrace() string {
	const maxDepth = 20
	var lines []string

	for i := 2; i < maxDepth; i++ { // Skip generateStacktrace and caller
		_, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}

		// Clean up file path to show only relevant parts
		if idx := strings.LastIndex(file, "/pkg/mcp/"); idx != -1 {
			file = file[idx+1:]
		}

		lines = append(lines, fmt.Sprintf("%s:%d", file, line))
	}

	return strings.Join(lines, " -> ")
}

// Wrap wraps an existing error in a structured error
func Wrap(err error, operation, component, message string) *StructuredError {
	if err == nil {
		return nil
	}

	// If it's already a structured error, preserve its properties
	if structErr, ok := err.(*StructuredError); ok {
		return NewError(operation, component, message).
			WithCause(structErr).
			WithCategory(structErr.Category).
			WithSeverity(structErr.Severity).
			WithWorkflowID(structErr.WorkflowID).
			WithSessionID(structErr.SessionID).
			Build()
	}

	// Otherwise create a new structured error
	return NewError(operation, component, message).
		WithCause(err).
		Build()
}

// WrapContext wraps an error with additional context
func WrapContext(err error, operation, component, message string, context map[string]interface{}) *StructuredError {
	structErr := Wrap(err, operation, component, message)
	if structErr == nil {
		return nil
	}

	for k, v := range context {
		structErr.WithContext(k, v)
	}

	return structErr
}
