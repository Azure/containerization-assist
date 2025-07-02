package utils

import (
	"context"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/validation/core"
	"github.com/Azure/container-kit/pkg/mcp/validation/validators"
	"github.com/rs/zerolog"
)

// StandardizedValidationMixin provides common validation utilities for atomic tools
type StandardizedValidationMixin struct {
	logger            zerolog.Logger
	formatValidator   *validators.FormatValidator
	securityValidator *validators.SecurityValidator
	imageValidator    *validators.ImageValidator
}

// NewStandardizedValidationMixin creates a new standardized validation mixin
func NewStandardizedValidationMixin(logger zerolog.Logger) *StandardizedValidationMixin {
	return &StandardizedValidationMixin{
		logger:            logger,
		formatValidator:   validators.NewFormatValidator(),
		securityValidator: validators.NewSecurityValidator(),
		imageValidator:    validators.NewImageValidator(),
	}
}

// ValidationResult represents a simplified validation result for compatibility
type ValidationResult struct {
	errors []ValidationError
}

// ValidationError represents a validation error
type ValidationError struct {
	Message string
	Field   string
}

// HasErrors returns true if there are validation errors
func (r *ValidationResult) HasErrors() bool {
	return len(r.errors) > 0
}

// GetFirstError returns the first validation error
func (r *ValidationResult) GetFirstError() *ValidationError {
	if len(r.errors) > 0 {
		return &r.errors[0]
	}
	return nil
}

// GetErrors returns all validation errors
func (r *ValidationResult) GetErrors() []ValidationError {
	return r.errors
}

// Errors property for backward compatibility
func (r *ValidationResult) Errors() []ValidationError {
	return r.errors
}

// StandardValidateRequiredFields validates that required fields are not empty
func (m *StandardizedValidationMixin) StandardValidateRequiredFields(data interface{}, requiredFields []string) *ValidationResult {
	result := &ValidationResult{errors: make([]ValidationError, 0)}

	// Use format validator to check required fields
	ctx := context.Background()
	options := core.NewValidationOptions()
	formatResult := m.formatValidator.Validate(ctx, data, options)

	// Convert unified validation result to simple result
	for _, err := range formatResult.Errors {
		for _, field := range requiredFields {
			if err.Field == field || strings.Contains(err.Message, field) {
				result.errors = append(result.errors, ValidationError{
					Message: err.Message,
					Field:   err.Field,
				})
			}
		}
	}

	return result
}

// StandardValidateImageRef validates a Docker image reference
func (m *StandardizedValidationMixin) StandardValidateImageRef(imageRef, fieldName string) *ValidationResult {
	result := &ValidationResult{errors: make([]ValidationError, 0)}

	if imageRef == "" {
		result.errors = append(result.errors, ValidationError{
			Message: "Image reference cannot be empty",
			Field:   fieldName,
		})
		return result
	}

	// Use image validator to validate the reference
	ctx := context.Background()
	options := core.NewValidationOptions()
	imageResult := m.imageValidator.Validate(ctx, imageRef, options)

	// Convert unified validation result to simple result
	for _, err := range imageResult.Errors {
		result.errors = append(result.errors, ValidationError{
			Message: err.Message,
			Field:   fieldName,
		})
	}

	return result
}
