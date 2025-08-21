package errors

import (
	"fmt"
)

// Error represents a structured error with code and context
type Error struct {
	Code    Code
	Domain  string
	Message string
	Cause   error
}

// New creates a new error with the given code, domain, message, and optional cause
func New(code Code, domain string, message string, cause error) *Error {
	return &Error{
		Code:    code,
		Domain:  domain,
		Message: message,
		Cause:   cause,
	}
}

// Error implements the error interface
func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s:%s] %s: %v", e.Domain, e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s:%s] %s", e.Domain, e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *Error) Unwrap() error {
	return e.Cause
}

// Is checks if the error matches the target error
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	return e.Code == t.Code
}
