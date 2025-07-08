package config

import (
	"fmt"

	"github.com/Azure/container-kit/pkg/mcp/errors"
)

// ValidationError represents a configuration validation error
// DEPRECATED: Use errors.ValidationError directly instead
func NewValidationError(field, message string) error {
	return errors.ValidationError(
		errors.CodeValidationFailed,
		fmt.Sprintf("validation error for field '%s': %s", field, message),
		nil,
	).WithContext("field", field)
}
