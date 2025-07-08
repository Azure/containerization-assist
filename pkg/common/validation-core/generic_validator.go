package validation

import (
	"fmt"
	"strings"

	errors "github.com/Azure/container-kit/pkg/mcp/errors"
	mcperrors "github.com/Azure/container-kit/pkg/mcp/errors"
)

// FieldValidator provides generic field validation without reflection
type FieldValidator[T any] struct {
	fieldName string
	value     T
	required  bool
	validator func(T) error
}

// NewFieldValidator creates a new field validator
func NewFieldValidator[T any](fieldName string, value T) *FieldValidator[T] {
	return &FieldValidator[T]{
		fieldName: fieldName,
		value:     value,
	}
}

// Required marks the field as required
func (fv *FieldValidator[T]) Required() *FieldValidator[T] {
	fv.required = true
	return fv
}

// WithValidator adds a custom validation function
func (fv *FieldValidator[T]) WithValidator(validator func(T) error) *FieldValidator[T] {
	fv.validator = validator
	return fv
}

// Validate performs the validation
func (fv *FieldValidator[T]) Validate() error {
	// Check if value is zero/empty
	var zero T
	if fv.required && isZeroValue(fv.value, zero) {
		return mcperrors.NewError().Messagef("required field %s is missing or empty", fv.fieldName).WithLocation(

		// Run custom validator if provided
		).Build()
	}

	if fv.validator != nil {
		if err := fv.validator(fv.value); err != nil {
			return mcperrors.NewError().Messagef("validation error for field %s: %w", fv.fieldName, err).WithLocation().Build()
		}
	}

	return nil
}

// StructValidator provides validation for entire structs without reflection
type StructValidator struct {
	errors []error
}

// NewStructValidator creates a new struct validator
func NewStructValidator() *StructValidator {
	return &StructValidator{
		errors: make([]error, 0),
	}
}

// ValidateField validates a single field
func (sv *StructValidator) ValidateField(fieldName string, value interface{}, required bool, validator func(interface{}) error) {
	// Handle string validation (most common case)
	if strVal, ok := value.(string); ok {
		if required && strings.TrimSpace(strVal) == "" {
			sv.errors = append(sv.errors, mcperrors.NewError().Messagef("required field %s is missing or empty", fieldName).WithLocation().Build())
			return
		}
	} else if required && isNilInterface(value) {
		sv.errors = append(sv.errors, mcperrors.NewError().Messagef("required field %s is missing or empty", fieldName).WithLocation().Build(

		// Run custom validator if provided
		))
		return
	}

	if validator != nil && value != nil {
		if err := validator(value); err != nil {
			sv.errors = append(sv.errors, mcperrors.NewError().Messagef("validation error for field %s: %w", fieldName, err).WithLocation(

			// ValidateString validates a string field
			).Build())
		}
	}
}

func (sv *StructValidator) ValidateString(fieldName, value string, required bool) {
	if required && strings.TrimSpace(value) == "" {
		sv.errors = append(sv.errors, mcperrors.NewError().Messagef("required field %s is missing or empty", fieldName).WithLocation(

		// ValidateStringWithPattern validates a string field with a pattern
		).Build())
	}
}

func (sv *StructValidator) ValidateStringWithPattern(fieldName, value string, required bool, pattern func(string) bool, patternDesc string) {
	if required && strings.TrimSpace(value) == "" {
		sv.errors = append(sv.errors, mcperrors.NewError().Messagef("required field %s is missing or empty", fieldName).WithLocation().Build())
		return
	}

	if value != "" && pattern != nil && !pattern(value) {
		sv.errors = append(sv.errors, mcperrors.NewError().Messagef("field %s does not match required pattern: %s", fieldName, patternDesc).WithLocation(

		// ValidateSlice validates a slice field (non-generic version)
		).Build())
	}
}

func (sv *StructValidator) ValidateSlice(fieldName string, length int, required bool) {
	if required && length == 0 {
		sv.errors = append(sv.errors, mcperrors.NewError().Messagef("required field %s cannot be empty", fieldName).WithLocation(

		// ValidateMap validates a map field (non-generic version)
		).Build())
	}
}

func (sv *StructValidator) ValidateMap(fieldName string, length int, required bool) {
	if required && length == 0 {
		sv.errors = append(sv.errors, mcperrors.NewError().Messagef("required field %s cannot be empty", fieldName).WithLocation(

		// HasErrors returns true if there are validation errors
		).Build())
	}
}

func (sv *StructValidator) HasErrors() bool {
	return len(sv.errors) > 0
}

// GetErrors returns all validation errors
func (sv *StructValidator) GetErrors() []error {
	return sv.errors
}

// GetError returns a combined error or nil
func (sv *StructValidator) GetError() error {
	if !sv.HasErrors() {
		return nil
	}

	if len(sv.errors) == 1 {
		return sv.errors[0]
	}

	// Combine multiple errors
	var msgs []string
	for _, err := range sv.errors {
		msgs = append(msgs, err.Error())
	}
	return mcperrors.NewError().Messagef("multiple validation errors: %s", strings.Join(msgs, "; ")).WithLocation(

	// ValidateRequiredFieldsGeneric provides a generic replacement for ValidateRequiredFields
	).Build()
}

func ValidateRequiredFieldsGeneric(validator func(*StructValidator)) error {
	sv := NewStructValidator()
	validator(sv)
	return sv.GetError()
}

// ValidateOptionalFieldsGeneric provides a generic replacement for ValidateOptionalFields
func ValidateOptionalFieldsGeneric(validator func(*StructValidator)) error {
	sv := NewStructValidator()
	validator(sv)
	return sv.GetError()
}

// Helper functions

// isZeroValue checks if a value is the zero value for its type
func isZeroValue[T any](value, zero T) bool {
	// This is a simplified check - in practice, you'd use more sophisticated comparison
	return fmt.Sprintf("%v", value) == fmt.Sprintf("%v", zero)
}

// isNilInterface checks if an interface value is nil
func isNilInterface(value interface{}) bool {
	if value == nil {
		return true
	}

	// Check for typed nil (e.g., (*string)(nil))
	switch v := value.(type) {
	case *string:
		return v == nil
	case *int:
		return v == nil
	case *bool:
		return v == nil
	case []string:
		return v == nil
	case []int:
		return v == nil
	case map[string]interface{}:
		return v == nil
	default:
		return false
	}
}

// Example usage functions for common validations

// ValidateSessionID validates a session ID field
func ValidateSessionID(sessionID string) error {
	return ValidateRequiredFieldsGeneric(func(sv *StructValidator) {
		sv.ValidateString("session_id", sessionID, true)
	})
}

// ValidateImageReference validates an image reference field
func ValidateImageReference(imageRef string) error {
	return ValidateRequiredFieldsGeneric(func(sv *StructValidator) {
		sv.ValidateStringWithPattern("image_ref", imageRef, true,
			func(s string) bool {
				// Basic image reference validation
				return s != "" && !strings.Contains(s, " ")
			},
			"valid Docker image reference",
		)
	})
}

// ValidateToolArguments provides generic validation for tool arguments
type ToolArgumentsValidator interface {
	Validate() error
}

// ValidateToolArgs validates tool arguments that implement the validator interface
func ValidateToolArgs(args ToolArgumentsValidator) error {
	if args == nil {
		return errors.NewError().Messagef("arguments cannot be nil").Build()
	}
	return args.Validate()
}
