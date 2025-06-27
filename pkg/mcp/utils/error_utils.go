package utils

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
)

// ErrorUtils provides common error handling utilities
// This file consolidates duplicate error functions found across the codebase

// ErrorWithContext represents an error with additional context information
type ErrorWithContext struct {
	Err       error
	Context   map[string]interface{}
	Operation string
	File      string
	Line      int
}

func (e *ErrorWithContext) Error() string {
	var parts []string

	if e.Operation != "" {
		parts = append(parts, fmt.Sprintf("operation: %s", e.Operation))
	}

	if e.File != "" && e.Line > 0 {
		parts = append(parts, fmt.Sprintf("location: %s:%d", e.File, e.Line))
	}

	if len(e.Context) > 0 {
		var contextParts []string
		for k, v := range e.Context {
			contextParts = append(contextParts, fmt.Sprintf("%s=%v", k, v))
		}
		parts = append(parts, fmt.Sprintf("context: {%s}", strings.Join(contextParts, ", ")))
	}

	if len(parts) > 0 {
		return fmt.Sprintf("%s [%s]", e.Err.Error(), strings.Join(parts, "; "))
	}

	return e.Err.Error()
}

func (e *ErrorWithContext) Unwrap() error {
	return e.Err
}

// WrapErrorWithContext wraps an error with additional context
func WrapErrorWithContext(err error, operation string, context map[string]interface{}) error {
	if err == nil {
		return nil
	}

	_, file, line, _ := runtime.Caller(1)

	return &ErrorWithContext{
		Err:       err,
		Context:   context,
		Operation: operation,
		File:      file,
		Line:      line,
	}
}

// WrapError wraps an error with a message (simple version)
func WrapError(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

// WrapErrorf wraps an error with a formatted message
func WrapErrorf(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	message := fmt.Sprintf(format, args...)
	return fmt.Errorf("%s: %w", message, err)
}

// NewError creates a new error with context
func NewError(message string, context map[string]interface{}) error {
	_, file, line, _ := runtime.Caller(1)

	return &ErrorWithContext{
		Err:     errors.New(message),
		Context: context,
		File:    file,
		Line:    line,
	}
}

// NewErrorf creates a new formatted error with context
func NewErrorf(format string, args ...interface{}) error {
	message := fmt.Sprintf(format, args...)
	return NewError(message, nil)
}

// ErrorChain represents a chain of errors
type ErrorChain struct {
	errors []error
}

func (e *ErrorChain) Error() string {
	if len(e.errors) == 0 {
		return "no errors"
	}

	if len(e.errors) == 1 {
		return e.errors[0].Error()
	}

	var messages []string
	for i, err := range e.errors {
		messages = append(messages, fmt.Sprintf("[%d] %s", i+1, err.Error()))
	}

	return fmt.Sprintf("multiple errors: %s", strings.Join(messages, "; "))
}

func (e *ErrorChain) Unwrap() error {
	if len(e.errors) == 0 {
		return nil
	}
	return e.errors[0]
}

// Add adds an error to the chain
func (e *ErrorChain) Add(err error) {
	if err != nil {
		e.errors = append(e.errors, err)
	}
}

// HasErrors returns true if the chain contains any errors
func (e *ErrorChain) HasErrors() bool {
	return len(e.errors) > 0
}

// Errors returns all errors in the chain
func (e *ErrorChain) Errors() []error {
	return e.errors
}

// ErrorOrNil returns the error chain if it has errors, nil otherwise
func (e *ErrorChain) ErrorOrNil() error {
	if len(e.errors) == 0 {
		return nil
	}
	return e
}

// NewErrorChain creates a new error chain
func NewErrorChain() *ErrorChain {
	return &ErrorChain{
		errors: make([]error, 0),
	}
}

// CombineErrors combines multiple errors into a single error
func CombineErrors(errors ...error) error {
	chain := NewErrorChain()
	for _, err := range errors {
		chain.Add(err)
	}
	return chain.ErrorOrNil()
}

// IsTemporaryError checks if an error is temporary (implements temporary interface)
func IsTemporaryError(err error) bool {
	type temporary interface {
		Temporary() bool
	}

	if temp, ok := err.(temporary); ok {
		return temp.Temporary()
	}

	// Check wrapped errors
	if unwrapped := errors.Unwrap(err); unwrapped != nil {
		return IsTemporaryError(unwrapped)
	}

	return false
}

// IsTimeoutError checks if an error is a timeout error
func IsTimeoutError(err error) bool {
	type timeout interface {
		Timeout() bool
	}

	if temp, ok := err.(timeout); ok {
		return temp.Timeout()
	}

	// Check wrapped errors
	if unwrapped := errors.Unwrap(err); unwrapped != nil {
		return IsTimeoutError(unwrapped)
	}

	// Check for common timeout error messages
	errStr := strings.ToLower(err.Error())
	timeoutKeywords := []string{"timeout", "deadline exceeded", "context deadline exceeded"}
	for _, keyword := range timeoutKeywords {
		if strings.Contains(errStr, keyword) {
			return true
		}
	}

	return false
}

// IsNetworkError checks if an error is a network-related error
func IsNetworkError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	networkKeywords := []string{
		"connection refused",
		"connection reset",
		"network is unreachable",
		"no route to host",
		"dns",
		"dial",
		"lookup",
	}

	for _, keyword := range networkKeywords {
		if strings.Contains(errStr, keyword) {
			return true
		}
	}

	return false
}

// ExtractRootCause extracts the root cause from a wrapped error
func ExtractRootCause(err error) error {
	for {
		unwrapped := errors.Unwrap(err)
		if unwrapped == nil {
			return err
		}
		err = unwrapped
	}
}

// ErrorMatcher provides a way to match errors by type or message
type ErrorMatcher func(error) bool

// MatchErrorByMessage creates an error matcher that matches by message content
func MatchErrorByMessage(substring string) ErrorMatcher {
	return func(err error) bool {
		if err == nil {
			return false
		}
		return strings.Contains(strings.ToLower(err.Error()), strings.ToLower(substring))
	}
}

// MatchErrorByType creates an error matcher that matches by error type
func MatchErrorByType[T error](target T) ErrorMatcher {
	return func(err error) bool {
		if err == nil {
			return false
		}

		// Check if err is of type T
		var targetType T
		return errors.As(err, &targetType)
	}
}

// RecoverError safely recovers from a panic and converts it to an error
func RecoverError() error {
	if r := recover(); r != nil {
		switch v := r.(type) {
		case error:
			return WrapError(v, "recovered from panic")
		case string:
			return NewError(fmt.Sprintf("recovered from panic: %s", v), nil)
		default:
			return NewError(fmt.Sprintf("recovered from panic: %v", v), nil)
		}
	}
	return nil
}

// SafeErrorString safely extracts error message, handling nil errors
func SafeErrorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// ErrorSeverity represents the severity level of an error
type ErrorSeverity int

const (
	ErrorSeverityInfo ErrorSeverity = iota
	ErrorSeverityWarning
	ErrorSeverityError
	ErrorSeverityCritical
)

func (s ErrorSeverity) String() string {
	switch s {
	case ErrorSeverityInfo:
		return "info"
	case ErrorSeverityWarning:
		return "warning"
	case ErrorSeverityError:
		return "error"
	case ErrorSeverityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// ClassifiedError represents an error with a severity classification
type ClassifiedError struct {
	Err      error
	Severity ErrorSeverity
	Category string
	Code     string
}

func (e *ClassifiedError) Error() string {
	return fmt.Sprintf("[%s:%s] %s", e.Severity, e.Category, e.Err.Error())
}

func (e *ClassifiedError) Unwrap() error {
	return e.Err
}

// ClassifyError creates a classified error
func ClassifyError(err error, severity ErrorSeverity, category, code string) error {
	if err == nil {
		return nil
	}

	return &ClassifiedError{
		Err:      err,
		Severity: severity,
		Category: category,
		Code:     code,
	}
}
