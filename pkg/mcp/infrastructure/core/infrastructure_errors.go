// Package core provides structured error handling patterns for infrastructure operations
package core

import (
	"fmt"
	"log/slog"
)

// InfrastructureError represents a structured error for infrastructure operations
type InfrastructureError struct {
	Operation   string                 `json:"operation"`
	Component   string                 `json:"component"`
	Cause       error                  `json:"-"`
	Recoverable bool                   `json:"recoverable"`
	Context     map[string]interface{} `json:"context,omitempty"`
	Message     string                 `json:"message"`
}

func (e *InfrastructureError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s failed in %s: %s (caused by: %v)", e.Operation, e.Component, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s failed in %s: %s", e.Operation, e.Component, e.Message)
}

func (e *InfrastructureError) Unwrap() error {
	return e.Cause
}

// IsRecoverable returns whether this error can be automatically recovered from
func (e *InfrastructureError) IsRecoverable() bool {
	return e.Recoverable
}

func NewInfrastructureError(operation, component, message string, cause error, recoverable bool) *InfrastructureError {
	return &InfrastructureError{
		Operation:   operation,
		Component:   component,
		Message:     message,
		Cause:       cause,
		Recoverable: recoverable,
		Context:     make(map[string]interface{}),
	}
}

// WithContext adds context information to the error
func (e *InfrastructureError) WithContext(key string, value interface{}) *InfrastructureError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// LogWithContext logs the error with structured context
func (e *InfrastructureError) LogWithContext(logger *slog.Logger) {
	// Build args slice for logger
	args := []interface{}{
		"operation", e.Operation,
		"component", e.Component,
		"message", e.Message,
		"recoverable", e.Recoverable,
	}

	// Add context fields
	for k, v := range e.Context {
		args = append(args, k, v)
	}

	// Add cause if present
	if e.Cause != nil {
		args = append(args, "cause", e.Cause.Error())
	}

	if e.Recoverable {
		logger.Warn("Infrastructure operation failed (recoverable)", args...)
	} else {
		logger.Error("Infrastructure operation failed", args...)
	}
}

// Common error patterns for infrastructure operations

// ErrImageNotFound represents a common Docker image not found error
var ErrImageNotFound = fmt.Errorf("image not found")

// ErrResourceNotFound represents a common Kubernetes resource not found error
var ErrResourceNotFound = fmt.Errorf("resource not found")

// ErrPermissionDenied represents a common permission denied error
var ErrPermissionDenied = fmt.Errorf("permission denied")

// IsImageNotFound checks if an error indicates an image was not found
func IsImageNotFound(err error) bool {
	if err == nil {
		return false
	}

	// Check for common Docker image not found patterns
	errStr := err.Error()
	return contains(errStr, "No such image") ||
		contains(errStr, "image not found") ||
		contains(errStr, "pull access denied") ||
		contains(errStr, "repository does not exist")
}

// IsResourceNotFound checks if an error indicates a Kubernetes resource was not found
func IsResourceNotFound(err error) bool {
	if err == nil {
		return false
	}

	// Check for common Kubernetes not found patterns
	errStr := err.Error()
	return contains(errStr, "not found") ||
		contains(errStr, "NotFound") ||
		contains(errStr, "does not exist")
}

// IsPermissionDenied checks if an error indicates permission was denied
func IsPermissionDenied(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	return contains(errStr, "permission denied") ||
		contains(errStr, "access denied") ||
		contains(errStr, "forbidden") ||
		contains(errStr, "unauthorized")
}

// contains is a helper function for case-insensitive string matching
func contains(s, substr string) bool {
	// Simple case-insensitive contains check
	s, substr = toLowerCase(s), toLowerCase(substr)
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) && findSubstring(s, substr)))
}

func toLowerCase(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
