// Package security - Unified security and validation framework for pkg/mcp
// This package consolidates scattered validation utilities and provides a unified interface
package security

import (
	"context"
	"fmt"
	"time"
)

// ValidationResult represents the result of a validation operation
type ValidationResult[T any] struct {
	Valid    bool                   `json:"valid"`
	Data     T                      `json:"data"`
	Errors   []ValidationError      `json:"errors,omitempty"`
	Warnings []ValidationWarning    `json:"warnings,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	Context  map[string]interface{} `json:"context,omitempty"`
	Duration time.Duration          `json:"duration,omitempty"`
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string                 `json:"field"`
	Message string                 `json:"message"`
	Code    string                 `json:"code"`
	Value   interface{}            `json:"value,omitempty"`
	Context map[string]interface{} `json:"context,omitempty"`
}

// ValidationWarning represents a validation warning
type ValidationWarning struct {
	Field   string                 `json:"field"`
	Message string                 `json:"message"`
	Code    string                 `json:"code"`
	Value   interface{}            `json:"value,omitempty"`
	Context map[string]interface{} `json:"context,omitempty"`
}

// Validator is the unified interface for all validators
// This consolidates all scattered validator interfaces across pkg/mcp
type Validator interface {
	// Name returns the validator's unique identifier
	Name() string

	// Validate performs validation on the provided data
	Validate(ctx context.Context, data any) Result

	// ValidateWithOptions performs validation with additional options
	ValidateWithOptions(ctx context.Context, data any, opts Options) Result

	// GetSupportedTypes returns the data types this validator can handle
	GetSupportedTypes() []string

	// GetVersion returns the validator version
	GetVersion() string
}

// TypedValidator provides type-safe validation for specific data types
type TypedValidator[T any] interface {
	// Name returns the validator's unique identifier
	Name() string

	// Validate performs type-safe validation
	Validate(ctx context.Context, data T) ValidationResult[T]

	// ValidateWithOptions performs validation with additional options
	ValidateWithOptions(ctx context.Context, data T, opts Options) ValidationResult[T]

	// GetVersion returns the validator version
	GetVersion() string
}

// Result represents the outcome of validation
// This maintains compatibility with existing code while encouraging migration to ValidationResult[T]
type Result struct {
	// Valid indicates if validation passed
	Valid bool `json:"valid"`

	// Errors contains validation errors
	Errors []Error `json:"errors,omitempty"`

	// Warnings contains validation warnings
	Warnings []Warning `json:"warnings,omitempty"`

	// Score represents the validation score (0-100, 100 being perfect)
	Score int `json:"score"`

	// Details contains additional validation details
	Details map[string]interface{} `json:"details,omitempty"`

	// Context provides additional context about the validation
	Context map[string]string `json:"context,omitempty"`

	// Duration tracks how long validation took
	Duration time.Duration `json:"duration,omitempty"`

	// Metadata contains validation metadata
	Metadata Metadata `json:"metadata,omitempty"`
}

// Error represents a validation error with rich context
type Error struct {
	// Field is the field that failed validation
	Field string `json:"field"`

	// Message is the human-readable error message
	Message string `json:"message"`

	// Code is the error code for programmatic handling
	Code string `json:"code"`

	// Value is the actual value that failed validation
	Value interface{} `json:"value,omitempty"`

	// Severity indicates the error severity level
	Severity ErrorSeverity `json:"severity"`

	// Context provides additional error context
	Context map[string]string `json:"context,omitempty"`

	// Path provides the full path to the field (for nested structures)
	Path string `json:"path,omitempty"`

	// Constraint describes the constraint that was violated
	Constraint string `json:"constraint,omitempty"`
}

// Warning represents a validation warning
type Warning struct {
	// Field is the field that triggered the warning
	Field string `json:"field"`

	// Message is the human-readable warning message
	Message string `json:"message"`

	// Code is the warning code for programmatic handling
	Code string `json:"code"`

	// Value is the value that triggered the warning
	Value interface{} `json:"value,omitempty"`

	// Suggestion provides a suggestion for improvement
	Suggestion string `json:"suggestion,omitempty"`

	// Context provides additional warning context
	Context map[string]string `json:"context,omitempty"`

	// Path provides the full path to the field (for nested structures)
	Path string `json:"path,omitempty"`
}

// Error implements the error interface for ValidationError
func (e *Error) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("%s: %s (field: %s, severity: %s)", e.Path, e.Message, e.Field, e.Severity)
	}
	return fmt.Sprintf("%s (field: %s, severity: %s)", e.Message, e.Field, e.Severity)
}

// ErrorSeverity represents the severity of a validation error
type ErrorSeverity string

const (
	SeverityLow      ErrorSeverity = "low"
	SeverityMedium   ErrorSeverity = "medium"
	SeverityHigh     ErrorSeverity = "high"
	SeverityCritical ErrorSeverity = "critical"
)

// Options provides options for validation behavior
type Options struct {
	// StrictMode enables strict validation (fail on warnings)
	StrictMode bool `json:"strict_mode"`

	// MaxErrors limits the number of errors to collect (0 = unlimited)
	MaxErrors int `json:"max_errors"`

	// SkipFields specifies fields to skip during validation
	SkipFields []string `json:"skip_fields,omitempty"`

	// IncludeWarnings controls whether warnings are included
	IncludeWarnings bool `json:"include_warnings"`

	// Context provides additional context for validation
	Context map[string]string `json:"context,omitempty"`

	// Timeout sets a timeout for validation operations
	Timeout time.Duration `json:"timeout,omitempty"`

	// FailFast stops validation on first error
	FailFast bool `json:"fail_fast"`
}

// Metadata contains metadata about the validation process
type Metadata struct {
	ValidatedAt      time.Time         `json:"validated_at"`
	ValidatorName    string            `json:"validator_name"`
	ValidatorVersion string            `json:"validator_version"`
	Duration         time.Duration     `json:"duration"`
	RulesApplied     []string          `json:"rules_applied,omitempty"`
	Context          map[string]string `json:"context,omitempty"`
}

// FieldConstraint represents a validation constraint for a field
type FieldConstraint struct {
	// Type specifies the constraint type (required, min, max, pattern, etc.)
	Type string `json:"type"`

	// Value specifies the constraint value
	Value interface{} `json:"value,omitempty"`

	// Message provides a custom error message
	Message string `json:"message,omitempty"`

	// Severity specifies the error severity if constraint is violated
	Severity ErrorSeverity `json:"severity,omitempty"`
}

// Rule represents a validation rule that can be applied to data
type Rule struct {
	// Name is the rule identifier
	Name string `json:"name"`

	// Description describes what the rule validates
	Description string `json:"description"`

	// Field specifies the field path this rule applies to
	Field string `json:"field"`

	// Constraints specifies the validation constraints
	Constraints []FieldConstraint `json:"constraints"`

	// Condition specifies when this rule should be applied (optional)
	Condition string `json:"condition,omitempty"`

	// Enabled controls whether this rule is active
	Enabled bool `json:"enabled"`
}

// NewResult creates a new validation result
func NewResult() *Result {
	return &Result{
		Valid:    true,
		Errors:   make([]Error, 0),
		Warnings: make([]Warning, 0),
		Score:    100,
		Details:  make(map[string]interface{}),
		Context:  make(map[string]string),
		Metadata: Metadata{
			ValidatedAt: time.Now(),
		},
	}
}

// AddError adds a validation error (supports both ValidationError pointer and individual parameters)
func (vr *Result) AddError(args ...interface{}) {
	if len(args) == 1 {
		// Single ValidationError pointer
		if err, ok := args[0].(*Error); ok {
			vr.Errors = append(vr.Errors, *err)
			vr.Valid = false
			// Adjust score based on severity
			switch err.Severity {
			case SeverityCritical:
				vr.Score = maximum(0, vr.Score-30)
			case SeverityHigh:
				vr.Score = maximum(0, vr.Score-20)
			case SeverityMedium:
				vr.Score = maximum(0, vr.Score-10)
			case SeverityLow:
				vr.Score = maximum(0, vr.Score-5)
			}
			return
		}
	}

	// Legacy signature: field, message, code, value, severity
	if len(args) == 5 {
		field, _ := args[0].(string)
		message, _ := args[1].(string)
		code, _ := args[2].(string)
		value := args[3]
		severity, _ := args[4].(ErrorSeverity)

		vr.Errors = append(vr.Errors, Error{
			Field:    field,
			Message:  message,
			Code:     code,
			Value:    value,
			Severity: severity,
			Context:  make(map[string]string),
		})
		vr.Valid = false

		// Adjust score based on severity
		switch severity {
		case SeverityCritical:
			vr.Score = maximum(0, vr.Score-30)
		case SeverityHigh:
			vr.Score = maximum(0, vr.Score-20)
		case SeverityMedium:
			vr.Score = maximum(0, vr.Score-10)
		case SeverityLow:
			vr.Score = maximum(0, vr.Score-5)
		}
	}
}

// AddWarning adds a validation warning
func (vr *Result) AddWarning(field, message, code string, value interface{}, suggestion string) {
	vr.Warnings = append(vr.Warnings, Warning{
		Field:      field,
		Message:    message,
		Code:       code,
		Value:      value,
		Suggestion: suggestion,
		Context:    make(map[string]string),
	})

	// Warnings slightly reduce score but don't invalidate
	vr.Score = maximum(0, vr.Score-2)
}

// HasErrors returns true if there are any validation errors
func (vr *Result) HasErrors() bool {
	return len(vr.Errors) > 0
}

// HasWarnings returns true if there are any validation warnings
func (vr *Result) HasWarnings() bool {
	return len(vr.Warnings) > 0
}

// HasCriticalErrors returns true if there are any critical errors
func (vr *Result) HasCriticalErrors() bool {
	for _, err := range vr.Errors {
		if err.Severity == SeverityCritical {
			return true
		}
	}
	return false
}

// GetErrorCount returns the number of errors by severity
func (vr *Result) GetErrorCount(severity ErrorSeverity) int {
	count := 0
	for _, err := range vr.Errors {
		if err.Severity == severity {
			count++
		}
	}
	return count
}

// Error implements the error interface for ValidationResult
func (vr *Result) Error() string {
	if vr.Valid {
		return ""
	}

	if len(vr.Errors) == 1 {
		return vr.Errors[0].Message
	}

	return fmt.Sprintf("validation failed with %d errors", len(vr.Errors))
}

// ToTypedResult converts ValidationResult to ValidationResult[T]
func ToTypedResult[T any](vr *Result, data T) ValidationResult[T] {
	// Convert context from map[string]string to map[string]interface{}
	context := make(map[string]interface{})
	for k, v := range vr.Context {
		context[k] = v
	}

	result := ValidationResult[T]{
		Valid:    vr.Valid,
		Data:     data,
		Errors:   make([]ValidationError, len(vr.Errors)),
		Warnings: make([]ValidationWarning, len(vr.Warnings)),
		Context:  context,
		Duration: vr.Duration,
	}

	// Convert errors
	for i, err := range vr.Errors {
		errContext := make(map[string]interface{})
		for k, v := range err.Context {
			errContext[k] = v
		}
		result.Errors[i] = ValidationError{
			Code:    err.Code,
			Message: err.Message,
			Field:   err.Field,
			Context: errContext,
		}
	}

	// Convert warnings
	for i, warn := range vr.Warnings {
		warnContext := make(map[string]interface{})
		for k, v := range warn.Context {
			warnContext[k] = v
		}
		result.Warnings[i] = ValidationWarning{
			Code:    warn.Code,
			Message: warn.Message,
			Field:   warn.Field,
			Context: warnContext,
		}
	}

	return result
}

// NewSessionResult creates a new validation result for a session
func NewSessionResult(validatorName, validatorVersion string) *Result {
	return &Result{
		Valid:    true,
		Errors:   make([]Error, 0),
		Warnings: make([]Warning, 0),
		Score:    100,
		Details:  make(map[string]interface{}),
		Context:  make(map[string]string),
		Metadata: Metadata{
			ValidatedAt:      time.Now(),
			ValidatorName:    validatorName,
			ValidatorVersion: validatorVersion,
		},
	}
}

// ErrorType constants for validation errors
type ErrorType string

const (
	ErrTypeValidation ErrorType = "validation"
	ErrTypeConstraint ErrorType = "constraint"
	ErrTypeRequired   ErrorType = "required"
	ErrTypeFormat     ErrorType = "format"
	ErrTypeRange      ErrorType = "range"
	ErrTypeCustom     ErrorType = "custom"
)

// NewError creates a new validation error
func NewError(code, message string, _ ErrorType, severity ErrorSeverity) *Error {
	return &Error{
		Code:     code,
		Message:  message,
		Severity: severity,
		Context:  make(map[string]string),
	}
}

// NewWarning creates a new validation warning
func NewWarning(code, message string) *Warning {
	return &Warning{
		Code:    code,
		Message: message,
		Context: make(map[string]string),
	}
}

// helper function for maximum
func maximum(a, b int) int {
	if a > b {
		return a
	}
	return b
}
