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

// TypedMCPError represents a type-safe error with structured context
type TypedMCPError struct {
	Category      ErrorCategory
	Module        string
	Operation     string
	Message       string
	Cause         error
	StringFields  map[string]string  `json:"string_fields,omitempty"`
	NumberFields  map[string]float64 `json:"number_fields,omitempty"`
	BooleanFields map[string]bool    `json:"boolean_fields,omitempty"`
	Retryable     bool
	Recoverable   bool
}

// Error implements the error interface
func (e *TypedMCPError) Error() string {
	if e.Module != "" {
		return fmt.Sprintf("mcp/%s: %s", e.Module, e.Message)
	}
	return fmt.Sprintf("mcp: %s", e.Message)
}

// Unwrap returns the underlying error for error unwrapping
func (e *TypedMCPError) Unwrap() error {
	return e.Cause
}

// Is checks if the error matches a target error
func (e *TypedMCPError) Is(target error) bool {
	if typedErr, ok := target.(*TypedMCPError); ok {
		return e.Category == typedErr.Category && e.Module == typedErr.Module
	}
	if mcpErr, ok := target.(*MCPError); ok {
		return e.Category == mcpErr.Category && e.Module == mcpErr.Module
	}
	return errors.Is(e.Cause, target)
}

// WithStringContext adds string context information to the error
func (e *TypedMCPError) WithStringContext(key, value string) *TypedMCPError {
	if e.StringFields == nil {
		e.StringFields = make(map[string]string)
	}
	e.StringFields[key] = value
	return e
}

// WithNumberContext adds numeric context information to the error
func (e *TypedMCPError) WithNumberContext(key string, value float64) *TypedMCPError {
	if e.NumberFields == nil {
		e.NumberFields = make(map[string]float64)
	}
	e.NumberFields[key] = value
	return e
}

// WithBooleanContext adds boolean context information to the error
func (e *TypedMCPError) WithBooleanContext(key string, value bool) *TypedMCPError {
	if e.BooleanFields == nil {
		e.BooleanFields = make(map[string]bool)
	}
	e.BooleanFields[key] = value
	return e
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

// NewTypedError creates a new TypedMCPError with the standard format
func NewTypedError(module, message string, category ErrorCategory) *TypedMCPError {
	return &TypedMCPError{
		Module:        module,
		Message:       message,
		Category:      category,
		StringFields:  make(map[string]string),
		NumberFields:  make(map[string]float64),
		BooleanFields: make(map[string]bool),
	}
}

// NewTypedValidation creates a typed validation error
func NewTypedValidation(module, message string) *TypedMCPError {
	return NewTypedError(module, message, CategoryValidation)
}

// NewTypedNetwork creates a typed network error
func NewTypedNetwork(module, message string) *TypedMCPError {
	err := NewTypedError(module, message, CategoryNetwork)
	err.Retryable = true // Network errors are typically retryable
	return err
}

// WrapTyped wraps an existing error with additional context
func WrapTyped(err error, module, message string) *TypedMCPError {
	if err == nil {
		return nil
	}

	typedErr := &TypedMCPError{
		Module:        module,
		Message:       message,
		Cause:         err,
		StringFields:  make(map[string]string),
		NumberFields:  make(map[string]float64),
		BooleanFields: make(map[string]bool),
	}

	// If it's already a TypedMCPError, preserve its category
	if existingTyped, ok := err.(*TypedMCPError); ok {
		typedErr.Category = existingTyped.Category
		typedErr.Operation = existingTyped.Operation
		typedErr.Retryable = existingTyped.Retryable
		typedErr.Recoverable = existingTyped.Recoverable
		return typedErr
	}

	// If it is a legacy MCPError, preserve its category
	if mcpErr, ok := err.(*MCPError); ok {
		typedErr.Category = mcpErr.Category
		typedErr.Operation = mcpErr.Operation
		typedErr.Retryable = mcpErr.Retryable
		typedErr.Recoverable = mcpErr.Recoverable
		return typedErr
	}

	// For non-MCP errors, categorize as internal by default
	typedErr.Category = CategoryInternal
	return typedErr
}

// New creates a new MCPError
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

// Validationf creates a validation error with formatted message
func Validationf(module, format string, args ...interface{}) *MCPError {
	return Newf(module, CategoryValidation, format, args...)
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

// Network creates a network error
func Network(module, message string) *MCPError {
	return &MCPError{
		Module:    module,
		Message:   message,
		Category:  CategoryNetwork,
		Context:   make(map[string]interface{}),
		Retryable: true,
	}
}

// Internal creates an internal error
func Internal(module, message string) *MCPError {
	return New(module, message, CategoryInternal)
}

// Validation creates a validation error
func Validation(module, message string) *MCPError {
	return New(module, message, CategoryValidation)
}

// Wrap wraps an existing error with additional context
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
