// Package validation provides a unified validation framework for Container Kit
package validation

import (
	"fmt"
	"strings"
)

// ValidationResult represents a unified validation result across all domains
type ValidationResult struct {
	Valid    bool                   `json:"valid"`
	Errors   []ValidationError      `json:"errors"`
	Warnings []ValidationWarning    `json:"warnings"`
	Context  map[string]interface{} `json:"context,omitempty"`
}

// ValidationError represents a validation error with structured information
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
	Level   string `json:"level,omitempty"` // error, warning, info
}

// ValidationWarning represents a validation warning
type ValidationWarning struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// NewValidationResult creates a new validation result
func NewValidationResult() *ValidationResult {
	return &ValidationResult{
		Valid:    true,
		Errors:   make([]ValidationError, 0),
		Warnings: make([]ValidationWarning, 0),
		Context:  make(map[string]interface{}),
	}
}

// AddError adds a validation error and marks the result as invalid
func (r *ValidationResult) AddError(field, message, code string) {
	r.Valid = false
	r.Errors = append(r.Errors, ValidationError{
		Field:   field,
		Message: message,
		Code:    code,
		Level:   "error",
	})
}

// AddErrorf adds a formatted validation error
func (r *ValidationResult) AddErrorf(field, code, format string, args ...interface{}) {
	r.AddError(field, fmt.Sprintf(format, args...), code)
}

// AddWarning adds a validation warning
func (r *ValidationResult) AddWarning(field, message, code string) {
	r.Warnings = append(r.Warnings, ValidationWarning{
		Field:   field,
		Message: message,
		Code:    code,
	})
}

// AddWarningf adds a formatted validation warning
func (r *ValidationResult) AddWarningf(field, code, format string, args ...interface{}) {
	r.AddWarning(field, fmt.Sprintf(format, args...), code)
}

// SetContext adds context information to the validation result
func (r *ValidationResult) SetContext(key string, value interface{}) {
	if r.Context == nil {
		r.Context = make(map[string]interface{})
	}
	r.Context[key] = value
}

// HasErrors returns true if there are validation errors
func (r *ValidationResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// HasWarnings returns true if there are validation warnings
func (r *ValidationResult) HasWarnings() bool {
	return len(r.Warnings) > 0
}

// ErrorCount returns the number of validation errors
func (r *ValidationResult) ErrorCount() int {
	return len(r.Errors)
}

// WarningCount returns the number of validation warnings
func (r *ValidationResult) WarningCount() int {
	return len(r.Warnings)
}

// Merge combines another validation result into this one
func (r *ValidationResult) Merge(other *ValidationResult) {
	if other == nil {
		return
	}

	// If other has errors, this result becomes invalid
	if !other.Valid {
		r.Valid = false
	}

	// Merge errors
	r.Errors = append(r.Errors, other.Errors...)

	// Merge warnings
	r.Warnings = append(r.Warnings, other.Warnings...)

	// Merge context
	if other.Context != nil {
		if r.Context == nil {
			r.Context = make(map[string]interface{})
		}
		for k, v := range other.Context {
			r.Context[k] = v
		}
	}
}

// Summary returns a human-readable summary of the validation result
func (r *ValidationResult) Summary() string {
	if r.Valid && len(r.Warnings) == 0 {
		return "Validation passed"
	}

	var parts []string

	if !r.Valid {
		parts = append(parts, fmt.Sprintf("%d error(s)", len(r.Errors)))
	}

	if len(r.Warnings) > 0 {
		parts = append(parts, fmt.Sprintf("%d warning(s)", len(r.Warnings)))
	}

	status := "passed"
	if !r.Valid {
		status = "failed"
	}

	return fmt.Sprintf("Validation %s with %s", status, strings.Join(parts, " and "))
}

// FirstError returns the first validation error, if any
func (r *ValidationResult) FirstError() *ValidationError {
	if len(r.Errors) > 0 {
		return &r.Errors[0]
	}
	return nil
}

// GetErrorsForField returns all errors for a specific field
func (r *ValidationResult) GetErrorsForField(field string) []ValidationError {
	var fieldErrors []ValidationError
	for _, err := range r.Errors {
		if err.Field == field {
			fieldErrors = append(fieldErrors, err)
		}
	}
	return fieldErrors
}

// GetWarningsForField returns all warnings for a specific field
func (r *ValidationResult) GetWarningsForField(field string) []ValidationWarning {
	var fieldWarnings []ValidationWarning
	for _, warning := range r.Warnings {
		if warning.Field == field {
			fieldWarnings = append(fieldWarnings, warning)
		}
	}
	return fieldWarnings
}
