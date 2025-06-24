package base

import (
	"fmt"
	"strings"
)

// ErrorType defines the type of error
type ErrorType string

const (
	ErrTypeValidation ErrorType = "validation"
	ErrTypeNotFound   ErrorType = "not_found"
	ErrTypeSystem     ErrorType = "system"
	ErrTypeBuild      ErrorType = "build"
	ErrTypeDeployment ErrorType = "deployment"
	ErrTypeSecurity   ErrorType = "security"
	ErrTypeConfig     ErrorType = "configuration"
	ErrTypeNetwork    ErrorType = "network"
	ErrTypePermission ErrorType = "permission"
)

// ErrorSeverity defines the severity of an error
type ErrorSeverity string

const (
	SeverityCritical ErrorSeverity = "critical"
	SeverityHigh     ErrorSeverity = "high"
	SeverityMedium   ErrorSeverity = "medium"
	SeverityLow      ErrorSeverity = "low"
)

// ToolError represents a rich error with context
type ToolError struct {
	Code      string
	Message   string
	Type      ErrorType
	Severity  ErrorSeverity
	Context   ErrorContext
	Cause     error
	Timestamp string
}

// ErrorContext provides additional context for errors
type ErrorContext struct {
	Tool      string
	Operation string
	Stage     string
	SessionID string
	Fields    map[string]interface{}
}

// Error implements the error interface
func (e *ToolError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *ToolError) Unwrap() error {
	return e.Cause
}

// WithContext adds context to the error
func (e *ToolError) WithContext(key string, value interface{}) *ToolError {
	if e.Context.Fields == nil {
		e.Context.Fields = make(map[string]interface{})
	}
	e.Context.Fields[key] = value
	return e
}

// ErrorBuilder provides a fluent interface for building errors
type ErrorBuilder struct {
	err *ToolError
}

// NewErrorBuilder creates a new error builder
func NewErrorBuilder(code, message string) *ErrorBuilder {
	return &ErrorBuilder{
		err: &ToolError{
			Code:     code,
			Message:  message,
			Type:     ErrTypeSystem,
			Severity: SeverityMedium,
			Context: ErrorContext{
				Fields: make(map[string]interface{}),
			},
		},
	}
}

// WithType sets the error type
func (b *ErrorBuilder) WithType(errType ErrorType) *ErrorBuilder {
	b.err.Type = errType
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

// WithTool sets the tool name
func (b *ErrorBuilder) WithTool(tool string) *ErrorBuilder {
	b.err.Context.Tool = tool
	return b
}

// WithOperation sets the operation
func (b *ErrorBuilder) WithOperation(operation string) *ErrorBuilder {
	b.err.Context.Operation = operation
	return b
}

// WithStage sets the stage
func (b *ErrorBuilder) WithStage(stage string) *ErrorBuilder {
	b.err.Context.Stage = stage
	return b
}

// WithSessionID sets the session ID
func (b *ErrorBuilder) WithSessionID(sessionID string) *ErrorBuilder {
	b.err.Context.SessionID = sessionID
	return b
}

// WithField adds a context field
func (b *ErrorBuilder) WithField(key string, value interface{}) *ErrorBuilder {
	b.err.Context.Fields[key] = value
	return b
}

// Build returns the constructed error
func (b *ErrorBuilder) Build() *ToolError {
	return b.err
}

// Common error constructors

// NewValidationError creates a validation error
func NewValidationError(field, message string) *ToolError {
	return NewErrorBuilder("VALIDATION_ERROR", message).
		WithType(ErrTypeValidation).
		WithField("field", field).
		Build()
}

// NewNotFoundError creates a not found error
func NewNotFoundError(resource, identifier string) *ToolError {
	return NewErrorBuilder("NOT_FOUND", fmt.Sprintf("%s not found: %s", resource, identifier)).
		WithType(ErrTypeNotFound).
		WithField("resource", resource).
		WithField("identifier", identifier).
		Build()
}

// NewSystemError creates a system error
func NewSystemError(operation string, cause error) *ToolError {
	return NewErrorBuilder("SYSTEM_ERROR", fmt.Sprintf("system error during %s", operation)).
		WithType(ErrTypeSystem).
		WithCause(cause).
		WithOperation(operation).
		Build()
}

// NewBuildError creates a build error
func NewBuildError(stage, message string) *ToolError {
	return NewErrorBuilder("BUILD_ERROR", message).
		WithType(ErrTypeBuild).
		WithStage(stage).
		Build()
}

// ValidationErrorSet represents a collection of validation errors
type ValidationErrorSet struct {
	errors []*ToolError
}

// NewValidationErrorSet creates a new validation error set
func NewValidationErrorSet() *ValidationErrorSet {
	return &ValidationErrorSet{
		errors: make([]*ToolError, 0),
	}
}

// Add adds an error to the set
func (s *ValidationErrorSet) Add(err *ToolError) {
	s.errors = append(s.errors, err)
}

// AddField adds a field validation error
func (s *ValidationErrorSet) AddField(field, message string) {
	s.Add(NewValidationError(field, message))
}

// HasErrors returns true if there are any errors
func (s *ValidationErrorSet) HasErrors() bool {
	return len(s.errors) > 0
}

// Count returns the number of errors
func (s *ValidationErrorSet) Count() int {
	return len(s.errors)
}

// Errors returns all errors
func (s *ValidationErrorSet) Errors() []*ToolError {
	return s.errors
}

// Error implements the error interface
func (s *ValidationErrorSet) Error() string {
	if len(s.errors) == 0 {
		return ""
	}

	messages := make([]string, len(s.errors))
	for i, err := range s.errors {
		messages[i] = err.Error()
	}

	return fmt.Sprintf("validation failed with %d errors: %s",
		len(s.errors), strings.Join(messages, "; "))
}

// ErrorHandler provides error handling utilities
type ErrorHandler struct {
	logger interface{} // zerolog.Logger
}

// NewErrorHandler creates a new error handler
func NewErrorHandler(logger interface{}) *ErrorHandler {
	return &ErrorHandler{
		logger: logger,
	}
}

// Handle handles an error based on its type and severity
func (h *ErrorHandler) Handle(err error) error {
	if err == nil {
		return nil
	}

	// Check if it's a ToolError
	if toolErr, ok := err.(*ToolError); ok {
		// Log based on severity
		switch toolErr.Severity {
		case SeverityCritical, SeverityHigh:
			// Would log as error
		case SeverityMedium:
			// Would log as warning
		case SeverityLow:
			// Would log as info
		}

		return toolErr
	}

	// Wrap unknown errors
	return NewSystemError("unknown", err)
}

// IsRetryable determines if an error is retryable
func (h *ErrorHandler) IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check if it's a ToolError
	if toolErr, ok := err.(*ToolError); ok {
		// Network and system errors are often retryable
		switch toolErr.Type {
		case ErrTypeNetwork, ErrTypeSystem:
			return true
		case ErrTypePermission, ErrTypeValidation:
			return false
		default:
			// Check specific error codes
			return h.isRetryableCode(toolErr.Code)
		}
	}

	// Check error message for common retryable patterns
	errMsg := err.Error()
	retryablePatterns := []string{
		"timeout", "connection refused", "temporary failure",
		"resource temporarily unavailable", "deadlock",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(strings.ToLower(errMsg), pattern) {
			return true
		}
	}

	return false
}

func (h *ErrorHandler) isRetryableCode(code string) bool {
	retryableCodes := map[string]bool{
		"TIMEOUT":            true,
		"CONNECTION_REFUSED": true,
		"RESOURCE_BUSY":      true,
		"RATE_LIMITED":       true,
	}

	return retryableCodes[code]
}
