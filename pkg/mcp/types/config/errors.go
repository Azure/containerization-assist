package config

import (
	"fmt"

	"github.com/Azure/container-kit/pkg/mcp/errors/rich"
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
}

// Error implements the error interface
func (ve *ValidationError) Error() string {
	return fmt.Sprintf("validation error for field '%s': %s", ve.Field, ve.Message)
}

// NewValidationError creates a new validation error
// Deprecated: Use NewRichValidationError instead for better error context
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}

// NewRichValidationError creates a new rich validation error (preferred)
func NewRichValidationError(field, message string) *rich.RichError {
	return NewRichConfigValidationError(field, message)
}
