// Package utils - Error handling utilities
// This file consolidates error handling functions from across pkg/mcp
package utils

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// ErrorType represents different categories of errors
type ErrorType string

const (
	ErrorTypeValidation    ErrorType = "validation"
	ErrorTypeNetwork       ErrorType = "network"
	ErrorTypeFileSystem    ErrorType = "filesystem"
	ErrorTypeDocker        ErrorType = "docker"
	ErrorTypeKubernetes    ErrorType = "kubernetes"
	ErrorTypeConfiguration ErrorType = "configuration"
	ErrorTypeSecurity      ErrorType = "security"
	ErrorTypeInternal      ErrorType = "internal"
	ErrorTypeTimeout       ErrorType = "timeout"
	ErrorTypePermission    ErrorType = "permission"
)

// StructuredError provides rich error context
type StructuredError struct {
	Type        ErrorType         `json:"type"`
	Code        string            `json:"code"`
	Message     string            `json:"message"`
	Details     string            `json:"details,omitempty"`
	Cause       error             `json:"cause,omitempty"`
	Context     map[string]string `json:"context,omitempty"`
	StackTrace  string            `json:"stack_trace,omitempty"`
	Suggestions []string          `json:"suggestions,omitempty"`
}

// Error implements the error interface
func (e *StructuredError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Message, e.Details, e.Code)
	}
	return fmt.Sprintf("%s (%s)", e.Message, e.Code)
}

// Unwrap returns the underlying cause error
func (e *StructuredError) Unwrap() error {
	return e.Cause
}

// NewStructuredError creates a new structured error
func NewStructuredError(errorType ErrorType, code, message string) *StructuredError {
	return &StructuredError{
		Type:    errorType,
		Code:    code,
		Message: message,
		Context: make(map[string]string),
	}
}

// WithDetails adds details to the error
func (e *StructuredError) WithDetails(details string) *StructuredError {
	e.Details = details
	return e
}

// WithCause adds a cause error
func (e *StructuredError) WithCause(cause error) *StructuredError {
	e.Cause = cause
	return e
}

// WithContext adds context information
func (e *StructuredError) WithContext(key, value string) *StructuredError {
	if e.Context == nil {
		e.Context = make(map[string]string)
	}
	e.Context[key] = value
	return e
}

// WithSuggestion adds a suggestion for resolving the error
func (e *StructuredError) WithSuggestion(suggestion string) *StructuredError {
	e.Suggestions = append(e.Suggestions, suggestion)
	return e
}

// WithStackTrace captures the current stack trace
func (e *StructuredError) WithStackTrace() *StructuredError {
	buf := make([]byte, 1024*4)
	n := runtime.Stack(buf, false)
	e.StackTrace = string(buf[:n])
	return e
}

// Common error constructors

// NewValidationError creates a validation error
func NewValidationError(code, message string) *StructuredError {
	return NewStructuredError(ErrorTypeValidation, code, message).
		WithSuggestion("Check the input parameters and try again")
}

// NewNetworkError creates a network error
func NewNetworkError(code, message string) *StructuredError {
	return NewStructuredError(ErrorTypeNetwork, code, message).
		WithSuggestion("Check network connectivity and retry")
}

// NewFileSystemError creates a filesystem error
func NewFileSystemError(code, message string) *StructuredError {
	return NewStructuredError(ErrorTypeFileSystem, code, message).
		WithSuggestion("Check file permissions and disk space")
}

// NewDockerError creates a Docker-related error
func NewDockerError(code, message string) *StructuredError {
	return NewStructuredError(ErrorTypeDocker, code, message).
		WithSuggestion("Check Docker daemon status and configuration")
}

// NewKubernetesError creates a Kubernetes-related error
func NewKubernetesError(code, message string) *StructuredError {
	return NewStructuredError(ErrorTypeKubernetes, code, message).
		WithSuggestion("Check cluster connectivity and RBAC permissions")
}

// NewConfigurationError creates a configuration error
func NewConfigurationError(code, message string) *StructuredError {
	return NewStructuredError(ErrorTypeConfiguration, code, message).
		WithSuggestion("Review configuration settings and documentation")
}

// NewSecurityError creates a security-related error
func NewSecurityError(code, message string) *StructuredError {
	return NewStructuredError(ErrorTypeSecurity, code, message).
		WithSuggestion("Review security policies and permissions")
}

// NewTimeoutError creates a timeout error
func NewTimeoutError(code, message string) *StructuredError {
	return NewStructuredError(ErrorTypeTimeout, code, message).
		WithSuggestion("Increase timeout values or optimize the operation")
}

// NewPermissionError creates a permission error
func NewPermissionError(code, message string) *StructuredError {
	return NewStructuredError(ErrorTypePermission, code, message).
		WithSuggestion("Check file or resource permissions")
}

// Error wrapping utilities

// WrapError wraps an error with additional context
func WrapError(err error, message string) error {
	if err == nil {
		return nil
	}

	return errors.NewError().Message(message).Cause(err).WithLocation(

	// WrapWithCode wraps an error with a code and message
	).Build()
}

func WrapWithCode(err error, code, message string) *StructuredError {
	if err == nil {
		return nil
	}

	return NewStructuredError(ErrorTypeInternal, code, message).WithCause(err)
}

// Chain creates an error chain with multiple errors
func Chain(errs ...error) error {
	var nonNilErrors []error
	for _, err := range errs {
		if err != nil {
			nonNilErrors = append(nonNilErrors, err)
		}
	}

	if len(nonNilErrors) == 0 {
		return nil
	}

	if len(nonNilErrors) == 1 {
		return nonNilErrors[0]
	}

	var messages []string
	for _, err := range nonNilErrors {
		messages = append(messages, err.Error())
	}

	return errors.NewError().Messagef("multiple errors: %s", strings.Join(messages, "; ")).WithLocation().Build()
}

// IsType checks if an error is of a specific type
func IsType(err error, errorType ErrorType) bool {
	if structured, ok := err.(*StructuredError); ok {
		return structured.Type == errorType
	}
	return false
}

// HasCode checks if an error has a specific code
func HasCode(err error, code string) bool {
	if structured, ok := err.(*StructuredError); ok {
		return structured.Code == code
	}
	return false
}

// IsValidationError checks if an error is a validation error
func IsValidationError(err error) bool {
	return IsType(err, ErrorTypeValidation)
}

// IsNetworkError checks if an error is a network error
func IsNetworkError(err error) bool {
	return IsType(err, ErrorTypeNetwork)
}

// IsFileSystemError checks if an error is a filesystem error
func IsFileSystemError(err error) bool {
	return IsType(err, ErrorTypeFileSystem)
}

// IsDockerError checks if an error is a Docker error
func IsDockerError(err error) bool {
	return IsType(err, ErrorTypeDocker)
}

// IsKubernetesError checks if an error is a Kubernetes error
func IsKubernetesError(err error) bool {
	return IsType(err, ErrorTypeKubernetes)
}

// IsConfigurationError checks if an error is a configuration error
func IsConfigurationError(err error) bool {
	return IsType(err, ErrorTypeConfiguration)
}

// IsSecurityError checks if an error is a security error
func IsSecurityError(err error) bool {
	return IsType(err, ErrorTypeSecurity)
}

// IsTimeoutError checks if an error is a timeout error
func IsTimeoutError(err error) bool {
	return IsType(err, ErrorTypeTimeout)
}

// IsPermissionError checks if an error is a permission error
func IsPermissionError(err error) bool {
	return IsType(err, ErrorTypePermission)
}

// Error formatting utilities

// FormatError formats an error for user display
func FormatError(err error) string {
	if err == nil {
		return ""
	}

	if structured, ok := err.(*StructuredError); ok {
		return structured.Error()
	}

	return err.Error()
}

// FormatErrorWithSuggestions formats an error with suggestions
func FormatErrorWithSuggestions(err error) string {
	if err == nil {
		return ""
	}

	if structured, ok := err.(*StructuredError); ok {
		message := structured.Error()
		if len(structured.Suggestions) > 0 {
			message += "\n\nSuggestions:\n"
			for _, suggestion := range structured.Suggestions {
				message += "  - " + suggestion + "\n"
			}
		}
		return message
	}

	return err.Error()
}

// GetErrorCode extracts the error code from an error
func GetErrorCode(err error) string {
	if structured, ok := err.(*StructuredError); ok {
		return structured.Code
	}
	return "UNKNOWN"
}

// GetErrorType extracts the error type from an error
func GetErrorType(err error) ErrorType {
	if structured, ok := err.(*StructuredError); ok {
		return structured.Type
	}
	return ErrorTypeInternal
}

// GetErrorContext extracts context from an error
func GetErrorContext(err error) map[string]string {
	if structured, ok := err.(*StructuredError); ok {
		return structured.Context
	}
	return nil
}

// Error collection utilities

// ErrorCollector collects multiple errors
type ErrorCollector struct {
	errors []error
}

// NewErrorCollector creates a new error collector
func NewErrorCollector() *ErrorCollector {
	return &ErrorCollector{
		errors: make([]error, 0),
	}
}

// Add adds an error to the collection
func (ec *ErrorCollector) Add(err error) {
	if err != nil {
		ec.errors = append(ec.errors, err)
	}
}

// HasErrors returns true if there are any errors
func (ec *ErrorCollector) HasErrors() bool {
	return len(ec.errors) > 0
}

// Count returns the number of errors
func (ec *ErrorCollector) Count() int {
	return len(ec.errors)
}

// Errors returns all collected errors
func (ec *ErrorCollector) Errors() []error {
	return ec.errors
}

// Error returns a combined error or nil if no errors
func (ec *ErrorCollector) Error() error {
	if len(ec.errors) == 0 {
		return nil
	}

	if len(ec.errors) == 1 {
		return ec.errors[0]
	}

	return Chain(ec.errors...)
}

// FirstError returns the first error or nil
func (ec *ErrorCollector) FirstError() error {
	if len(ec.errors) == 0 {
		return nil
	}
	return ec.errors[0]
}

// LastError returns the last error or nil
func (ec *ErrorCollector) LastError() error {
	if len(ec.errors) == 0 {
		return nil
	}
	return ec.errors[len(ec.errors)-1]
}

// Clear removes all errors
func (ec *ErrorCollector) Clear() {
	ec.errors = ec.errors[:0]
}
