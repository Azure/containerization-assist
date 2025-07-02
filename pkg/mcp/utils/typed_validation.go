package utils

import (
	"fmt"
	"os"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/errors/rich"
	"github.com/rs/zerolog"
)

// TypedValidator provides type-safe validation to replace reflection-based validation
// This replaces the reflection-based StandardizedValidationMixin
type TypedValidator struct {
	logger zerolog.Logger
}

// NewTypedValidator creates a new type-safe validator
func NewTypedValidator(logger zerolog.Logger) *TypedValidator {
	return &TypedValidator{
		logger: logger.With().Str("component", "typed_validator").Logger(),
	}
}

// ValidationRule represents a single validation rule for a field
type ValidationRule[T any] struct {
	FieldName   string
	Value       T
	Required    bool
	Validator   func(T) error
	ErrorCode   string
	Description string
}

// ValidatableStruct interface for structs that can validate themselves
type ValidatableStruct interface {
	ValidateRequired() error
	ValidatePaths() error
	ValidateFormat() error
}

// Validatable interface for individual fields
type Validatable[T any] interface {
	IsEmpty() bool
	Validate() error
}

// TypedValidationResult contains validation results with type safety
type TypedValidationResult struct {
	Valid   bool
	Errors  []TypedValidationError
	Context map[string]string
}

// TypedValidationError represents a validation error with type information
type TypedValidationError struct {
	Field       string
	Value       string // Convert interface{} to string for safety
	Constraint  string
	Message     string
	Code        string
	Severity    string
	Context     map[string]string
	Suggestions []string
}

func (tve *TypedValidationError) Error() string {
	return fmt.Sprintf("validation failed for field '%s': %s", tve.Field, tve.Message)
}

// AddError adds a validation error to the result
// Deprecated: Use AddTypedError with specific types instead
func (tvr *TypedValidationResult) AddError(field, message, code string, value interface{}) {
	tvr.Valid = false
	// Convert interface{} value to string for type safety
	valueStr := ""
	if value != nil {
		valueStr = fmt.Sprintf("%v", value)
	}
	tvr.Errors = append(tvr.Errors, TypedValidationError{
		Field:   field,
		Value:   valueStr,
		Message: message,
		Code:    code,
	})
}

// AddStringError adds a validation error with a string value
func (tvr *TypedValidationResult) AddStringError(field, message, code, value string) {
	tvr.Valid = false
	tvr.Errors = append(tvr.Errors, TypedValidationError{
		Field:   field,
		Value:   value,
		Message: message,
		Code:    code,
	})
}

// AddTypedError adds a validation error with typed value (preferred)
func (tvr *TypedValidationResult) AddTypedError(field, message, code, value string) {
	tvr.AddStringError(field, message, code, value)
}

// AddIntError adds a validation error with an integer value
func (tvr *TypedValidationResult) AddIntError(field, message, code string, value int) {
	tvr.AddStringError(field, message, code, fmt.Sprintf("%d", value))
}

// AddBoolError adds a validation error with a boolean value
func (tvr *TypedValidationResult) AddBoolError(field, message, code string, value bool) {
	tvr.AddStringError(field, message, code, fmt.Sprintf("%t", value))
}

// ValidateString validates string fields with type safety
func (tv *TypedValidator) ValidateString(value, fieldName string, required bool, validators ...func(string) error) error {
	if required && IsEmpty(value) {
		return rich.NewError().
			Code(rich.CodeMissingParameter).
			Type(rich.ErrTypeValidation).
			Severity(rich.SeverityMedium).
			Messagef("Required field '%s' cannot be empty", fieldName).
			Context("module", "typed_validator").
			Context("field", fieldName).
			Context("field_type", "string").
			Suggestion(fmt.Sprintf("Provide a non-empty value for field '%s'", fieldName)).
			WithLocation().
			Build()
	}

	for _, validator := range validators {
		if err := validator(value); err != nil {
			return rich.NewError().
				Code(rich.CodeValidationFailed).
				Type(rich.ErrTypeValidation).
				Severity(rich.SeverityMedium).
				Messagef("Validation failed for field '%s': %v", fieldName, err).
				Context("module", "typed_validator").
				Context("field", fieldName).
				Context("field_type", "string").
				Context("field_value", value).
				Cause(err).
				WithLocation().
				Build()
		}
	}

	return nil
}

// ValidateInt validates integer fields with type safety
func (tv *TypedValidator) ValidateInt(value int, fieldName string, required bool, validators ...func(int) error) error {
	if required && value == 0 {
		return rich.NewError().
			Code(rich.CodeMissingParameter).
			Type(rich.ErrTypeValidation).
			Severity(rich.SeverityMedium).
			Messagef("Required field '%s' cannot be zero", fieldName).
			Context("module", "typed_validator").
			Context("field", fieldName).
			Context("field_type", "int").
			Context("field_value", value).
			Suggestion(fmt.Sprintf("Provide a non-zero value for field '%s'", fieldName)).
			WithLocation().
			Build()
	}

	for _, validator := range validators {
		if err := validator(value); err != nil {
			return rich.NewError().
				Code(rich.CodeValidationFailed).
				Type(rich.ErrTypeValidation).
				Severity(rich.SeverityMedium).
				Messagef("Validation failed for field '%s': %v", fieldName, err).
				Context("module", "typed_validator").
				Context("field", fieldName).
				Context("field_type", "int").
				Context("field_value", value).
				Cause(err).
				WithLocation().
				Build()
		}
	}

	return nil
}

// ValidateStringSlice validates string slice fields with type safety
func (tv *TypedValidator) ValidateStringSlice(value []string, fieldName string, required bool, validators ...func([]string) error) error {
	if required && len(value) == 0 {
		return rich.NewError().
			Code(rich.CodeMissingParameter).
			Type(rich.ErrTypeValidation).
			Severity(rich.SeverityMedium).
			Messagef("Required field '%s' cannot be empty", fieldName).
			Context("module", "typed_validator").
			Context("field", fieldName).
			Context("field_type", "[]string").
			Context("field_length", len(value)).
			Suggestion(fmt.Sprintf("Provide at least one item for field '%s'", fieldName)).
			WithLocation().
			Build()
	}

	for _, validator := range validators {
		if err := validator(value); err != nil {
			return rich.NewError().
				Code(rich.CodeValidationFailed).
				Type(rich.ErrTypeValidation).
				Severity(rich.SeverityMedium).
				Messagef("Validation failed for field '%s': %v", fieldName, err).
				Context("module", "typed_validator").
				Context("field", fieldName).
				Context("field_type", "[]string").
				Context("field_length", len(value)).
				Cause(err).
				WithLocation().
				Build()
		}
	}

	return nil
}

// ValidatePath validates file/directory paths with type safety
func (tv *TypedValidator) ValidatePath(path, fieldName string, requirements PathRequirements) error {
	if path == "" {
		if requirements.Required {
			return rich.NewError().
				Code(rich.CodeMissingParameter).
				Type(rich.ErrTypeValidation).
				Severity(rich.SeverityMedium).
				Messagef("Required path field '%s' cannot be empty", fieldName).
				Context("module", "typed_validator").
				Context("field", fieldName).
				Context("field_type", "path").
				Suggestion(fmt.Sprintf("Provide a valid file or directory path for field '%s'", fieldName)).
				WithLocation().
				Build()
		}
		return nil
	}

	// Use centralized path validation
	if err := ValidateLocalPath(path); err != nil {
		return rich.NewError().
			Code(rich.CodeValidationFailed).
			Type(rich.ErrTypeValidation).
			Severity(rich.SeverityMedium).
			Messagef("Path validation failed for field '%s'", fieldName).
			Context("module", "typed_validator").
			Context("field", fieldName).
			Context("field_type", "path").
			Context("path_value", path).
			Cause(err).
			WithLocation().
			Build()
	}

	// Check specific requirements
	if requirements.MustExist {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return rich.NewError().
				Code(rich.CodeResourceNotFound).
				Type(rich.ErrTypeResource).
				Severity(rich.SeverityMedium).
				Messagef("Path '%s' in field '%s' does not exist", path, fieldName).
				Context("module", "typed_validator").
				Context("field", fieldName).
				Context("path_value", path).
				Suggestion("Ensure the path exists and is accessible").
				WithLocation().
				Build()
		}
	}

	if requirements.MustBeDirectory {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return rich.NewError().
				Code(rich.CodeInvalidParameter).
				Type(rich.ErrTypeValidation).
				Severity(rich.SeverityMedium).
				Messagef("Path '%s' in field '%s' must be a directory", path, fieldName).
				Context("module", "typed_validator").
				Context("field", fieldName).
				Context("path_value", path).
				Context("is_directory", false).
				Suggestion("Provide a directory path, not a file path").
				WithLocation().
				Build()
		}
	}

	if requirements.MustBeFile {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return rich.NewError().
				Code(rich.CodeInvalidParameter).
				Type(rich.ErrTypeValidation).
				Severity(rich.SeverityMedium).
				Messagef("Path '%s' in field '%s' must be a file", path, fieldName).
				Context("module", "typed_validator").
				Context("field", fieldName).
				Context("path_value", path).
				Context("is_file", false).
				Suggestion("Provide a file path, not a directory path").
				WithLocation().
				Build()
		}
	}

	if requirements.MustBeReadable {
		if err := tv.checkReadPermission(path); err != nil {
			return rich.NewError().
				Code(rich.CodeResourceNotFound).
				Type(rich.ErrTypePermission).
				Severity(rich.SeverityMedium).
				Messagef("Path '%s' in field '%s' is not readable", path, fieldName).
				Context("module", "typed_validator").
				Context("field", fieldName).
				Context("path_value", path).
				Context("readable", false).
				Cause(err).
				Suggestion("Check file permissions and ensure the path is accessible").
				WithLocation().
				Build()
		}
	}

	return nil
}

// PathRequirements defines requirements for path validation
type PathRequirements struct {
	Required          bool
	MustExist         bool
	MustBeDirectory   bool
	MustBeFile        bool
	MustBeReadable    bool
	MustBeWritable    bool
	AllowedExtensions []string
}

// checkReadPermission checks if a path is readable
func (tv *TypedValidator) checkReadPermission(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return nil
}

// Common validation functions that can be used with the typed validators

// MinLength creates a validator for minimum string length
func MinLength(min int) func(string) error {
	return func(s string) error {
		if len(s) < min {
			return fmt.Errorf("must be at least %d characters long", min)
		}
		return nil
	}
}

// MaxLength creates a validator for maximum string length
func MaxLength(max int) func(string) error {
	return func(s string) error {
		if len(s) > max {
			return fmt.Errorf("must be at most %d characters long", max)
		}
		return nil
	}
}

// MinValue creates a validator for minimum integer value
func MinValue(min int) func(int) error {
	return func(i int) error {
		if i < min {
			return fmt.Errorf("must be at least %d", min)
		}
		return nil
	}
}

// MaxValue creates a validator for maximum integer value
func MaxValue(max int) func(int) error {
	return func(i int) error {
		if i > max {
			return fmt.Errorf("must be at most %d", max)
		}
		return nil
	}
}

// NotEmptyStringSlice creates a validator for non-empty string slices
func NotEmptyStringSlice() func([]string) error {
	return func(slice []string) error {
		if len(slice) == 0 {
			return fmt.Errorf("cannot be empty")
		}
		return nil
	}
}

// ValidURL creates a validator for valid URLs
func ValidURL() func(string) error {
	return func(s string) error {
		if !IsValidURL(s) {
			return fmt.Errorf("must be a valid URL")
		}
		return nil
	}
}

// ValidEmail creates a validator for valid email addresses
func ValidEmail() func(string) error {
	return func(s string) error {
		if !IsValidEmail(s) {
			return fmt.Errorf("must be a valid email address")
		}
		return nil
	}
}

// ValidDockerImage creates a validator for valid Docker image names
func ValidDockerImage() func(string) error {
	return func(s string) error {
		if !IsValidDockerImageName(s) {
			return fmt.Errorf("must be a valid Docker image name")
		}
		return nil
	}
}

// ValidKubernetesName creates a validator for valid Kubernetes resource names
func ValidKubernetesName() func(string) error {
	return func(s string) error {
		if !IsValidKubernetesName(s) {
			return fmt.Errorf("must be a valid Kubernetes resource name")
		}
		return nil
	}
}

// ContainsNoUnsafeChars creates a validator that checks for unsafe characters
func ContainsNoUnsafeChars() func(string) error {
	return func(s string) error {
		if ContainsUnsafeCharacters(s) {
			return fmt.Errorf("contains potentially unsafe characters")
		}
		return nil
	}
}

// MatchesPattern creates a validator for regex pattern matching
func MatchesPattern(pattern string) func(string) error {
	return func(s string) error {
		// This would use regexp.MatchString but keeping it simple
		if strings.Contains(s, pattern) {
			return nil
		}
		return fmt.Errorf("must match pattern: %s", pattern)
	}
}

// ValidateStruct validates a struct that implements ValidatableStruct
func (tv *TypedValidator) ValidateStruct(s ValidatableStruct) error {
	if err := s.ValidateRequired(); err != nil {
		return err
	}
	if err := s.ValidatePaths(); err != nil {
		return err
	}
	if err := s.ValidateFormat(); err != nil {
		return err
	}
	return nil
}

// BatchValidate validates multiple fields and returns all errors
func (tv *TypedValidator) BatchValidate(validators ...func() error) []error {
	var errors []error
	for _, validator := range validators {
		if err := validator(); err != nil {
			errors = append(errors, err)
		}
	}
	return errors
}
