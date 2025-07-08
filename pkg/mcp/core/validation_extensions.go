package core

import (
	"fmt"

	validation "github.com/Azure/container-kit/pkg/mcp/security"
)

// AddError extends the validation.Result to support DockerfileValidationError
func AddErrorToResult(result *validation.Result, err interface{}) {
	switch e := err.(type) {
	case *DockerfileValidationError:
		// Convert DockerfileValidationError to validation.Error
		validationErr := validation.Error{
			Field:    e.Rule, // Use Rule as Field for compatibility
			Message:  e.Message,
			Code:     e.Code,
			Severity: e.Severity,
			Context:  e.Context,
			Path:     e.Rule,
		}

		// Add line/column info to context if present
		if e.Line > 0 {
			validationErr.Context["line"] = fmt.Sprintf("%d", e.Line)
		}
		if e.Column > 0 {
			validationErr.Context["column"] = fmt.Sprintf("%d", e.Column)
		}

		result.AddError(&validationErr)
	default:
		// Fall back to the original AddError implementation
		result.AddError(err)
	}
}

// AddWarning extends the validation.Result to support DockerfileValidationWarning
func AddWarningToResult(result *validation.Result, warn interface{}) {
	switch w := warn.(type) {
	case *DockerfileValidationWarning:
		// Extract line/column/rule from the embedded error if present
		var field, suggestion string
		if w.Error != nil {
			if w.Error.Rule != "" {
				field = w.Error.Rule
			}
			if w.Error.Line > 0 {
				suggestion = fmt.Sprintf("Line %d", w.Error.Line)
				if w.Error.Column > 0 {
					suggestion += fmt.Sprintf(", Column %d", w.Error.Column)
				}
			}
		}

		result.AddWarning(field, w.Message, w.Code, nil, suggestion)
	default:
		// Try to handle as a regular warning
		if warning, ok := warn.(*validation.Warning); ok {
			result.AddWarning(warning.Field, warning.Message, warning.Code, warning.Value, warning.Suggestion)
		}
	}
}

// ExtendedValidationResult wraps validation.Result with additional methods
type ExtendedValidationResult struct {
	*validation.Result
}

// AddError adds an error with support for DockerfileValidationError
func (r *ExtendedValidationResult) AddError(err interface{}) {
	AddErrorToResult(r.Result, err)
}

// AddWarning adds a warning with support for DockerfileValidationWarning
func (r *ExtendedValidationResult) AddWarning(warn interface{}) {
	AddWarningToResult(r.Result, warn)
}

// NewExtendedValidationResult creates a new extended validation result
func NewExtendedValidationResult() *ExtendedValidationResult {
	return &ExtendedValidationResult{
		Result: validation.NewResult(),
	}
}
