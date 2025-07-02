package errors

import (
	"fmt"
	"strings"
)

// Validation error codes
const (
	CodeRequiredField       = "REQUIRED_FIELD"
	CodeInvalidFormat       = "INVALID_FORMAT"
	CodeOutOfRange          = "OUT_OF_RANGE"
	CodeInvalidType         = "INVALID_TYPE"
	CodeDuplicateValue      = "DUPLICATE_VALUE"
	CodeInvalidReference    = "INVALID_REFERENCE"
	CodeConstraintViolation = "CONSTRAINT_VIOLATION"
)

// ValidationError represents a single validation error
type ValidationError struct {
	Field      string      // Field that failed validation
	Value      interface{} // Actual value that failed
	Code       string      // Error code
	Message    string      // Human-readable message
	Constraint string      // Constraint that was violated
	Expected   interface{} // Expected value/format
}

// Error implements the error interface
func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s: %s", e.Field, e.Message)
	}
	return e.Message
}

// ValidationErrors represents multiple validation errors
type ValidationErrors struct {
	errors []ValidationError
}

// NewValidationErrors creates a new validation errors collection
func NewValidationErrors() *ValidationErrors {
	return &ValidationErrors{
		errors: make([]ValidationError, 0),
	}
}

// Add adds a validation error
func (v *ValidationErrors) Add(err ValidationError) {
	v.errors = append(v.errors, err)
}

// AddRequired adds a required field error
func (v *ValidationErrors) AddRequired(field string) {
	v.Add(ValidationError{
		Field:   field,
		Code:    CodeRequiredField,
		Message: fmt.Sprintf("%s is required", field),
	})
}

// AddInvalidFormat adds an invalid format error
func (v *ValidationErrors) AddInvalidFormat(field string, value interface{}, expectedFormat string) {
	v.Add(ValidationError{
		Field:    field,
		Value:    value,
		Code:     CodeInvalidFormat,
		Message:  fmt.Sprintf("%s has invalid format", field),
		Expected: expectedFormat,
	})
}

// AddOutOfRange adds an out of range error
func (v *ValidationErrors) AddOutOfRange(field string, value, min, max interface{}) {
	v.Add(ValidationError{
		Field:    field,
		Value:    value,
		Code:     CodeOutOfRange,
		Message:  fmt.Sprintf("%s is out of range [%v, %v]", field, min, max),
		Expected: fmt.Sprintf("[%v, %v]", min, max),
	})
}

// AddInvalidType adds an invalid type error
func (v *ValidationErrors) AddInvalidType(field string, value interface{}, expectedType string) {
	v.Add(ValidationError{
		Field:    field,
		Value:    value,
		Code:     CodeInvalidType,
		Message:  fmt.Sprintf("%s has invalid type", field),
		Expected: expectedType,
	})
}

// AddDuplicate adds a duplicate value error
func (v *ValidationErrors) AddDuplicate(field string, value interface{}) {
	v.Add(ValidationError{
		Field:   field,
		Value:   value,
		Code:    CodeDuplicateValue,
		Message: fmt.Sprintf("%s contains duplicate value", field),
	})
}

// AddInvalidReference adds an invalid reference error
func (v *ValidationErrors) AddInvalidReference(field string, value interface{}, target string) {
	v.Add(ValidationError{
		Field:    field,
		Value:    value,
		Code:     CodeInvalidReference,
		Message:  fmt.Sprintf("%s references non-existent %s", field, target),
		Expected: target,
	})
}

// AddConstraintViolation adds a constraint violation error
func (v *ValidationErrors) AddConstraintViolation(field string, value interface{}, constraint string) {
	v.Add(ValidationError{
		Field:      field,
		Value:      value,
		Code:       CodeConstraintViolation,
		Message:    fmt.Sprintf("%s violates constraint: %s", field, constraint),
		Constraint: constraint,
	})
}

// HasErrors returns true if there are validation errors
func (v *ValidationErrors) HasErrors() bool {
	return len(v.errors) > 0
}

// Count returns the number of validation errors
func (v *ValidationErrors) Count() int {
	return len(v.errors)
}

// Errors returns all validation errors
func (v *ValidationErrors) Errors() []ValidationError {
	return v.errors
}

// Error implements the error interface
func (v *ValidationErrors) Error() string {
	if len(v.errors) == 0 {
		return ""
	}

	if len(v.errors) == 1 {
		return v.errors[0].Error()
	}

	messages := make([]string, len(v.errors))
	for i, err := range v.errors {
		messages[i] = err.Error()
	}

	return fmt.Sprintf("validation failed with %d errors: %s",
		len(v.errors), strings.Join(messages, "; "))
}

// ToCoreError converts validation errors to a CoreError
func (v *ValidationErrors) ToCoreError(module string) *CoreError {
	if !v.HasErrors() {
		return nil
	}

	err := Validation(module, v.Error())
	err.WithContext("error_count", v.Count())
	err.WithContext("errors", v.errors)

	// Add resolution suggestions
	err.WithResolution(&ErrorResolution{
		ImmediateSteps: []ResolutionStep{
			{
				Step:        1,
				Action:      "Review validation errors",
				Description: "Check each field mentioned in the validation errors",
			},
			{
				Step:        2,
				Action:      "Fix invalid values",
				Description: "Update the values to match the expected format or constraints",
			},
			{
				Step:        3,
				Action:      "Retry the operation",
				Description: "Once all validation errors are fixed, retry the operation",
			},
		},
	})

	return err
}

// Validation helper functions

// ValidateRequired checks if a value is not empty
func ValidateRequired(field string, value interface{}, errors *ValidationErrors) bool {
	if value == nil || value == "" || value == 0 {
		errors.AddRequired(field)
		return false
	}
	return true
}

// ValidateMinLength checks if a string meets minimum length
func ValidateMinLength(field, value string, minLength int, errors *ValidationErrors) bool {
	if len(value) < minLength {
		errors.AddConstraintViolation(field, value, fmt.Sprintf("minimum length %d", minLength))
		return false
	}
	return true
}

// ValidateMaxLength checks if a string is within maximum length
func ValidateMaxLength(field, value string, maxLength int, errors *ValidationErrors) bool {
	if len(value) > maxLength {
		errors.AddConstraintViolation(field, value, fmt.Sprintf("maximum length %d", maxLength))
		return false
	}
	return true
}

// ValidateRange checks if a numeric value is within range
func ValidateRange(field string, value, min, max int, errors *ValidationErrors) bool {
	if value < min || value > max {
		errors.AddOutOfRange(field, value, min, max)
		return false
	}
	return true
}

// ValidateEnum checks if a value is in allowed set
func ValidateEnum(field string, value string, allowed []string, errors *ValidationErrors) bool {
	for _, a := range allowed {
		if value == a {
			return true
		}
	}
	errors.AddConstraintViolation(field, value, fmt.Sprintf("must be one of: %v", allowed))
	return false
}

// ValidatePattern checks if a string matches a pattern
func ValidatePattern(field, value, pattern string, errors *ValidationErrors) bool {
	// Pattern validation would use regexp here
	// For now, this is a placeholder
	return true
}
