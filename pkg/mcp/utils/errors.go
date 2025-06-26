package utils

import "fmt"

// WrapError consistently wraps errors with operation context
func WrapError(err error, operation string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("failed to %s: %w", operation, err)
}

// WrapErrorf wraps errors with formatted operation context
func WrapErrorf(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	operation := fmt.Sprintf(format, args...)
	return fmt.Errorf("failed to %s: %w", operation, err)
}

// NewError creates a new error with context
func NewError(operation, message string) error {
	return fmt.Errorf("failed to %s: %s", operation, message)
}

// NewErrorf creates a new error with formatted context
func NewErrorf(operation, format string, args ...interface{}) error {
	message := fmt.Sprintf(format, args...)
	return fmt.Errorf("failed to %s: %s", operation, message)
}
