package tools

import "fmt"

// Error implements the error interface for ValidationError
func (e ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("validation error on field '%s': %s", e.Field, e.Message)
	}
	return fmt.Sprintf("validation error: %s", e.Message)
}

// NewValidationError creates a new validation error
func NewValidationError(field, message string) error {
	return ValidationError{
		Field:   field,
		Message: message,
	}
}

// NewValidationErrorWithCode creates a new validation error with a code
func NewValidationErrorWithCode(field, message, code string) error {
	return ValidationError{
		Field:   field,
		Message: message,
		Code:    code,
	}
}
