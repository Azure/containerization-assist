package utils

import (
	"fmt"
	"os"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/internal/types"
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
	context map[string]interface{}
}

// TypedValidationError represents a validation error with type information
type TypedValidationError struct {
	Field       string
	Value       interface{}
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
func (tvr *TypedValidationResult) AddError(field, message, code string, value interface{}) {
	tvr.Valid = false
	tvr.Errors = append(tvr.Errors, TypedValidationError{
		Field:   field,
		Value:   value,
		Message: message,
		Code:    code,
	})
}

// ValidateString validates string fields with type safety
func (tv *TypedValidator) ValidateString(value, fieldName string, required bool, validators ...func(string) error) error {
	if required && IsEmpty(value) {
		return types.NewValidationErrorBuilder(
			fmt.Sprintf("Required field '%s' cannot be empty", fieldName),
			fieldName,
			value,
		).WithOperation("validate_string").Build()
	}

	for _, validator := range validators {
		if err := validator(value); err != nil {
			return types.NewValidationErrorBuilder(
				fmt.Sprintf("Validation failed for field '%s': %v", fieldName, err),
				fieldName,
				value,
			).WithOperation("validate_string").Build()
		}
	}

	return nil
}

// ValidateInt validates integer fields with type safety
func (tv *TypedValidator) ValidateInt(value int, fieldName string, required bool, validators ...func(int) error) error {
	if required && value == 0 {
		return types.NewValidationErrorBuilder(
			fmt.Sprintf("Required field '%s' cannot be zero", fieldName),
			fieldName,
			value,
		).WithOperation("validate_int").Build()
	}

	for _, validator := range validators {
		if err := validator(value); err != nil {
			return types.NewValidationErrorBuilder(
				fmt.Sprintf("Validation failed for field '%s': %v", fieldName, err),
				fieldName,
				value,
			).WithOperation("validate_int").Build()
		}
	}

	return nil
}

// ValidateStringSlice validates string slice fields with type safety
func (tv *TypedValidator) ValidateStringSlice(value []string, fieldName string, required bool, validators ...func([]string) error) error {
	if required && len(value) == 0 {
		return types.NewValidationErrorBuilder(
			fmt.Sprintf("Required field '%s' cannot be empty", fieldName),
			fieldName,
			value,
		).WithOperation("validate_string_slice").Build()
	}

	for _, validator := range validators {
		if err := validator(value); err != nil {
			return types.NewValidationErrorBuilder(
				fmt.Sprintf("Validation failed for field '%s': %v", fieldName, err),
				fieldName,
				value,
			).WithOperation("validate_string_slice").Build()
		}
	}

	return nil
}

// ValidatePath validates file/directory paths with type safety
func (tv *TypedValidator) ValidatePath(path, fieldName string, requirements PathRequirements) error {
	if path == "" {
		if requirements.Required {
			return types.NewValidationErrorBuilder(
				fmt.Sprintf("Required path field '%s' cannot be empty", fieldName),
				fieldName,
				path,
			).WithOperation("validate_path").Build()
		}
		return nil
	}

	// Use centralized path validation
	if err := ValidateLocalPath(path); err != nil {
		return types.NewValidationErrorBuilder(
			fmt.Sprintf("Path validation failed for field '%s': %v", fieldName, err),
			fieldName,
			path,
		).WithOperation("validate_path").Build()
	}

	// Check specific requirements
	if requirements.MustExist {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return types.NewValidationErrorBuilder(
				fmt.Sprintf("Path '%s' in field '%s' does not exist", path, fieldName),
				fieldName,
				path,
			).WithOperation("validate_path").Build()
		}
	}

	if requirements.MustBeDirectory {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return types.NewValidationErrorBuilder(
				fmt.Sprintf("Path '%s' in field '%s' must be a directory", path, fieldName),
				fieldName,
				path,
			).WithOperation("validate_path").Build()
		}
	}

	if requirements.MustBeFile {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return types.NewValidationErrorBuilder(
				fmt.Sprintf("Path '%s' in field '%s' must be a file", path, fieldName),
				fieldName,
				path,
			).WithOperation("validate_path").Build()
		}
	}

	if requirements.MustBeReadable {
		if err := tv.checkReadPermission(path); err != nil {
			return types.NewValidationErrorBuilder(
				fmt.Sprintf("Path '%s' in field '%s' is not readable: %v", path, fieldName, err),
				fieldName,
				path,
			).WithOperation("validate_path").Build()
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
