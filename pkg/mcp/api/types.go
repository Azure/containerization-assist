package api

import (
	validation "github.com/Azure/container-kit/pkg/mcp/security"
)

type ValidationError = validation.Error
type ValidationWarning = validation.Warning
type ValidationMetadata = validation.Metadata
type ValidationResult = validation.Result
type ManifestValidationResult = validation.Result

// NewError creates a new validation error
func NewError(code, message string, errorType validation.ErrorType, severity validation.ErrorSeverity) *ValidationError {
	return validation.NewError(code, message, errorType, severity)
}
