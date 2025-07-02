package errors

import (
	"errors"
	"fmt"
)

// ErrorCategory represents different types of errors in the MCP system
type ErrorCategory string

const (
	// Validation errors - invalid input or configuration
	CategoryValidation ErrorCategory = "validation"
	// Network errors - connection, timeout, DNS issues
	CategoryNetwork ErrorCategory = "network"
	// Internal errors - unexpected system failures
	CategoryInternal ErrorCategory = "internal"
	// Authorization errors - permission denied, authentication failures
	CategoryAuth ErrorCategory = "auth"
	// Resource errors - not found, already exists, quota exceeded
	CategoryResource ErrorCategory = "resource"
	// Timeout errors - operation timeout
	CategoryTimeout ErrorCategory = "timeout"
	// Configuration errors - invalid or missing configuration
	CategoryConfig ErrorCategory = "config"
)

// MCPError represents a standardized error in the MCP system
type MCPError struct {
	Category    ErrorCategory
	Module      string
	Operation   string
	Message     string
	Cause       error
	Context     map[string]interface{}
	Retryable   bool
	Recoverable bool
}

// Error implements the error interface
func (e *MCPError) Error() string {
	if e.Module != "" {
		return fmt.Sprintf("mcp/%s: %s", e.Module, e.Message)
	}
	return fmt.Sprintf("mcp: %s", e.Message)
}

// Unwrap returns the underlying error for error unwrapping
func (e *MCPError) Unwrap() error {
	return e.Cause
}

// Is checks if the error matches a target error
func (e *MCPError) Is(target error) bool {
	if mcpErr, ok := target.(*MCPError); ok {
		return e.Category == mcpErr.Category && e.Module == mcpErr.Module
	}
	return errors.Is(e.Cause, target)
}

// WithContext adds context information to the error
func (e *MCPError) WithContext(key string, value interface{}) *MCPError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// New creates a new MCPError with the standard format
// Deprecated: Use NewRichValidation, NewRichNetwork, etc. from the migration package instead
func New(module, message string, category ErrorCategory) *MCPError {
	return &MCPError{
		Module:   module,
		Message:  message,
		Category: category,
		Context:  make(map[string]interface{}),
	}
}

// Newf creates a new MCPError with formatted message
func Newf(module string, category ErrorCategory, format string, args ...interface{}) *MCPError {
	return &MCPError{
		Module:   module,
		Message:  fmt.Sprintf(format, args...),
		Category: category,
		Context:  make(map[string]interface{}),
	}
}

// Wrap wraps an existing error with additional context
// Deprecated: Use WrapRich from the migration package instead
func Wrap(err error, module, message string) *MCPError {
	if err == nil {
		return nil
	}

	// If it's already an MCPError, preserve its category and add context
	if mcpErr, ok := err.(*MCPError); ok {
		return &MCPError{
			Category:    mcpErr.Category,
			Module:      module,
			Operation:   mcpErr.Operation,
			Message:     message,
			Cause:       mcpErr,
			Context:     make(map[string]interface{}),
			Retryable:   mcpErr.Retryable,
			Recoverable: mcpErr.Recoverable,
		}
	}

	// For non-MCP errors, categorize as internal by default
	return &MCPError{
		Category: CategoryInternal,
		Module:   module,
		Message:  message,
		Cause:    err,
		Context:  make(map[string]interface{}),
	}
}

// Wrapf wraps an existing error with formatted message
func Wrapf(err error, module, format string, args ...interface{}) *MCPError {
	return Wrap(err, module, fmt.Sprintf(format, args...))
}

// Validation creates a validation error
// Deprecated: Use NewRichValidation from the migration package instead
func Validation(module, message string) *MCPError {
	return New(module, message, CategoryValidation)
}

// Validationf creates a validation error with formatted message
func Validationf(module, format string, args ...interface{}) *MCPError {
	return Newf(module, CategoryValidation, format, args...)
}

// Network creates a network error
// Deprecated: Use NewRichNetwork from the migration package instead
func Network(module, message string) *MCPError {
	return &MCPError{
		Module:    module,
		Message:   message,
		Category:  CategoryNetwork,
		Context:   make(map[string]interface{}),
		Retryable: true, // Network errors are typically retryable
	}
}

// Networkf creates a network error with formatted message
func Networkf(module, format string, args ...interface{}) *MCPError {
	return &MCPError{
		Module:    module,
		Message:   fmt.Sprintf(format, args...),
		Category:  CategoryNetwork,
		Context:   make(map[string]interface{}),
		Retryable: true,
	}
}

// Internal creates an internal error
func Internal(module, message string) *MCPError {
	return New(module, message, CategoryInternal)
}

// Internalf creates an internal error with formatted message
func Internalf(module, format string, args ...interface{}) *MCPError {
	return Newf(module, CategoryInternal, format, args...)
}

// Resource creates a resource error
func Resource(module, message string) *MCPError {
	return New(module, message, CategoryResource)
}

// Resourcef creates a resource error with formatted message
func Resourcef(module, format string, args ...interface{}) *MCPError {
	return Newf(module, CategoryResource, format, args...)
}

// Timeout creates a timeout error
func Timeout(module, message string) *MCPError {
	return &MCPError{
		Module:    module,
		Message:   message,
		Category:  CategoryTimeout,
		Context:   make(map[string]interface{}),
		Retryable: true, // Timeout errors are typically retryable
	}
}

// Timeoutf creates a timeout error with formatted message
func Timeoutf(module, format string, args ...interface{}) *MCPError {
	return &MCPError{
		Module:    module,
		Message:   fmt.Sprintf(format, args...),
		Category:  CategoryTimeout,
		Context:   make(map[string]interface{}),
		Retryable: true,
	}
}

// Config creates a configuration error
func Config(module, message string) *MCPError {
	return New(module, message, CategoryConfig)
}

// Configf creates a configuration error with formatted message
func Configf(module, format string, args ...interface{}) *MCPError {
	return Newf(module, CategoryConfig, format, args...)
}

// Auth creates an authorization error
func Auth(module, message string) *MCPError {
	return New(module, message, CategoryAuth)
}

// Authf creates an authorization error with formatted message
func Authf(module, format string, args ...interface{}) *MCPError {
	return Newf(module, CategoryAuth, format, args...)
}

// IsCategory checks if an error belongs to a specific category
func IsCategory(err error, category ErrorCategory) bool {
	if mcpErr, ok := err.(*MCPError); ok {
		return mcpErr.Category == category
	}
	return false
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	if mcpErr, ok := err.(*MCPError); ok {
		return mcpErr.Retryable
	}
	return false
}

// IsRecoverable checks if an error is recoverable
func IsRecoverable(err error) bool {
	if mcpErr, ok := err.(*MCPError); ok {
		return mcpErr.Recoverable
	}
	return false
}

// GetModule returns the module name from an MCPError
func GetModule(err error) string {
	if mcpErr, ok := err.(*MCPError); ok {
		return mcpErr.Module
	}
	return ""
}

// GetCategory returns the category from an MCPError
func GetCategory(err error) ErrorCategory {
	if mcpErr, ok := err.(*MCPError); ok {
		return mcpErr.Category
	}
	return CategoryInternal
}
