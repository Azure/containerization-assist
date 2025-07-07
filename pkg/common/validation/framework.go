package validation

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/common/validation-core/core"
)

// Validator defines the standard interface that all tool validators must implement
// This is the unified interface that bridges tool-specific validation with the core framework
type Validator interface {
	Validate(ctx context.Context, input interface{}) error
}

// ToolValidator defines the interface for tool input validation with rich error reporting
type ToolValidator[T any] interface {
	// ValidateInput performs comprehensive input validation
	ValidateInput(ctx context.Context, input T) *core.Result[T]

	// GetValidationRules returns the validation rules this validator enforces
	GetValidationRules() []ValidationRule

	// GetSupportedInputTypes returns the input types this validator supports
	GetSupportedInputTypes() []string
}

// ValidationRule represents a single validation rule with metadata
type ValidationRule struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Severity    core.ErrorSeverity   `json:"severity"`
	Category    string               `json:"category"`
	Enabled     bool                 `json:"enabled"`
	Config      ValidationRuleConfig `json:"config,omitempty"`
}

// ValidationRuleConfig holds configuration for validation rules
type ValidationRuleConfig struct {
	MaxLength   int                    `json:"max_length,omitempty"`
	MinLength   int                    `json:"min_length,omitempty"`
	Pattern     string                 `json:"pattern,omitempty"`
	Required    bool                   `json:"required,omitempty"`
	CustomRules map[string]interface{} `json:"custom_rules,omitempty"`
}

// ValidationError creates a standardized validation error
type ValidationError struct {
	Field   string
	Code    string
	Message string
	Value   interface{}
}

// Error implements the error interface
func (ve *ValidationError) Error() string {
	if ve.Field != "" {
		return fmt.Sprintf("Field '%s': %s", ve.Field, ve.Message)
	}
	return ve.Message
}

// NewError creates a new validation error
func NewError(field, code, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Code:    code,
		Message: message,
	}
}

// NewRequiredFieldError creates an error for missing required fields
func NewRequiredFieldError(field string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Code:    "REQUIRED_FIELD",
		Message: "is required",
	}
}

// NewInvalidFormatError creates an error for invalid field formats
func NewInvalidFormatError(field, expectedFormat string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Code:    "INVALID_FORMAT",
		Message: fmt.Sprintf("invalid format, expected %s", expectedFormat),
	}
}

// NewInvalidValueError creates an error for invalid field values
func NewInvalidValueError(field string, value interface{}, reason string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Code:    "INVALID_VALUE",
		Message: fmt.Sprintf("invalid value '%v': %s", value, reason),
		Value:   value,
	}
}

// BaseValidator provides common validation functionality
type BaseValidator struct {
	name           string
	version        string
	rules          []ValidationRule
	supportedTypes []string
}

// NewBaseValidator creates a new base validator
func NewBaseValidator(name, version string) *BaseValidator {
	return &BaseValidator{
		name:           name,
		version:        version,
		rules:          make([]ValidationRule, 0),
		supportedTypes: make([]string, 0),
	}
}

// Duplicate method removed - already defined above

// GetName returns the validator name
func (v *BaseValidator) GetName() string {
	return v.name
}

// GetVersion returns the validator version
func (v *BaseValidator) GetVersion() string {
	return v.version
}

// GetValidationRules returns the validation rules
func (v *BaseValidator) GetValidationRules() []ValidationRule {
	return v.rules
}

// GetSupportedInputTypes returns supported input types
func (v *BaseValidator) GetSupportedInputTypes() []string {
	return v.supportedTypes
}

// AddRule adds a validation rule
func (v *BaseValidator) AddRule(rule ValidationRule) {
	v.rules = append(v.rules, rule)
}

// AddSupportedType adds a supported input type
func (v *BaseValidator) AddSupportedType(inputType string) {
	v.supportedTypes = append(v.supportedTypes, inputType)
}

// ValidateRequired checks if required fields are present
func (v *BaseValidator) ValidateRequired(fieldName string, value interface{}) *ValidationError {
	if value == nil {
		return NewRequiredFieldError(fieldName)
	}

	// Check for empty strings
	if str, ok := value.(string); ok && str == "" {
		return NewRequiredFieldError(fieldName)
	}

	return nil
}

// ValidateStringLength validates string length constraints
func (v *BaseValidator) ValidateStringLength(fieldName string, value string, minLen, maxLen int) *ValidationError {
	if len(value) < minLen {
		return NewInvalidValueError(fieldName, value, fmt.Sprintf("minimum length is %d", minLen))
	}
	if maxLen > 0 && len(value) > maxLen {
		return NewInvalidValueError(fieldName, value, fmt.Sprintf("maximum length is %d", maxLen))
	}
	return nil
}

// ValidateAbsolutePath validates that a path is absolute
func (v *BaseValidator) ValidateAbsolutePath(fieldName string, path string) *ValidationError {
	if path == "" {
		return NewRequiredFieldError(fieldName)
	}
	if path[0] != '/' {
		return NewInvalidFormatError(fieldName, "absolute path")
	}
	return nil
}

// CreateResult creates a new validation result with proper metadata (helper functions)
func CreateResult[T any](name, version string) *core.Result[T] {
	return core.NewGenericResult[T](name, version)
}

// CreateResultWithStartTime creates a new validation result and tracks duration (helper functions)
func CreateResultWithStartTime[T any](name, version string, startTime time.Time) *core.Result[T] {
	result := core.NewGenericResult[T](name, version)
	result.Duration = time.Since(startTime)
	return result
}

// ToolInputValidator provides a standard implementation for tool input validation
type ToolInputValidator[T any] struct {
	*BaseValidator
	validateFunc func(ctx context.Context, input T, result *core.Result[T]) error
}

// NewToolInputValidator creates a new tool input validator
func NewToolInputValidator[T any](name, version string, validateFunc func(ctx context.Context, input T, result *core.Result[T]) error) *ToolInputValidator[T] {
	return &ToolInputValidator[T]{
		BaseValidator: NewBaseValidator(name, version),
		validateFunc:  validateFunc,
	}
}

// ValidateInput performs the actual validation
func (v *ToolInputValidator[T]) ValidateInput(ctx context.Context, input T) *core.Result[T] {
	startTime := time.Now()
	result := CreateResultWithStartTime[T](v.name, v.version, startTime)

	if v.validateFunc != nil {
		if err := v.validateFunc(ctx, input, result); err != nil {
			// Convert error to validation error if needed
			if validationErr, ok := err.(*ValidationError); ok {
				coreErr := core.NewError(validationErr.Code, validationErr.Message, core.ErrTypeValidation, core.SeverityMedium)
				if validationErr.Field != "" {
					coreErr.WithField(validationErr.Field)
				}
				result.AddError(coreErr)
			} else {
				// Generic error
				coreErr := core.NewError("VALIDATION_ERROR", err.Error(), core.ErrTypeValidation, core.SeverityMedium)
				result.AddError(coreErr)
			}
		}
	}

	result.Duration = time.Since(startTime)
	return result
}

// Standard validation rule categories
const (
	CategoryRequired    = "required"
	CategoryFormat      = "format"
	CategorySecurity    = "security"
	CategoryBusiness    = "business"
	CategoryPerformance = "performance"
)

// Common validation rules that can be reused across tools
var (
	RequiredFieldRule = ValidationRule{
		Name:        "required_field",
		Description: "Field is required and cannot be empty",
		Severity:    core.SeverityHigh,
		Category:    CategoryRequired,
		Enabled:     true,
	}

	AbsolutePathRule = ValidationRule{
		Name:        "absolute_path",
		Description: "Path must be absolute (start with /)",
		Severity:    core.SeverityMedium,
		Category:    CategoryFormat,
		Enabled:     true,
	}

	NonEmptyStringRule = ValidationRule{
		Name:        "non_empty_string",
		Description: "String must not be empty",
		Severity:    core.SeverityMedium,
		Category:    CategoryRequired,
		Enabled:     true,
	}

	ValidImageTagRule = ValidationRule{
		Name:        "valid_image_tag",
		Description: "Image tag must follow Docker naming conventions",
		Severity:    core.SeverityMedium,
		Category:    CategoryFormat,
		Enabled:     true,
	}
)
