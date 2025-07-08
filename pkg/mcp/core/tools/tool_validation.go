package core

import (
	"fmt"

	"github.com/Azure/container-kit/pkg/mcp/errors"
)

// Error implements the error interface for ValidationError
func (e ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("validation error on field '%s': %s", e.Field, e.Message)
	}
	return fmt.Sprintf("validation error: %s", e.Message)
}

// NewValidationError creates a new validation error
// Deprecated: Use NewRichValidationError instead for better error context
func NewValidationError(field, message string) error {
	return ValidationError{
		Field:   field,
		Message: message,
	}
}

// NewValidationErrorWithCode creates a new validation error with a code
// Deprecated: Use NewRichValidationErrorWithCode instead for better error context
func NewValidationErrorWithCode(field, message, code string) error {
	return ValidationError{
		Field:   field,
		Message: message,
		Code:    code,
	}
}

// NewRichValidationError creates a new rich validation error (preferred)
func NewRichValidationError(toolName, field, message string) *errors.RichError {
	return errors.ToolValidationError(toolName, field, message, "", nil)
}

// NewRichValidationErrorWithCode creates a new rich validation error with code (preferred)
func NewRichValidationErrorWithCode(toolName, field, message, code string) *errors.RichError {
	return errors.ToolValidationError(toolName, field, message, code, nil)
}
