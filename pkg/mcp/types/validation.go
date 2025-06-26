package types

import (
	"fmt"
	"strings"
)

// ErrorType defines the type of error
type ErrorType string

const (
	ErrTypeValidation ErrorType = "validation"
	ErrTypeNotFound   ErrorType = "not_found"
	ErrTypeSystem     ErrorType = "system"
	ErrTypeBuild      ErrorType = "build"
	ErrTypeDeployment ErrorType = "deployment"
	ErrTypeSecurity   ErrorType = "security"
	ErrTypeConfig     ErrorType = "configuration"
	ErrTypeNetwork    ErrorType = "network"
	ErrTypePermission ErrorType = "permission"
)

// ErrorSeverity defines the severity of an error
type ErrorSeverity string

const (
	SeverityCritical ErrorSeverity = "critical"
	SeverityHigh     ErrorSeverity = "high"
	SeverityMedium   ErrorSeverity = "medium"
	SeverityLow      ErrorSeverity = "low"
)

// ToolError represents a rich error with context
type ToolError struct {
	Code      string
	Message   string
	Type      ErrorType
	Severity  ErrorSeverity
	Context   ErrorContext
	Cause     error
	Timestamp string
}

// ErrorContext provides additional context for errors
type ErrorContext struct {
	Tool      string
	Operation string
	Stage     string
	SessionID string
	Fields    map[string]interface{}
}

// Error implements the error interface
func (e *ToolError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// ValidationErrorSet groups validation errors
type ValidationErrorSet struct {
	errors []*ToolError
}

// NewValidationErrorSet creates a new validation error set
func NewValidationErrorSet() *ValidationErrorSet {
	return &ValidationErrorSet{
		errors: make([]*ToolError, 0),
	}
}

// Add adds an error to the set
func (s *ValidationErrorSet) Add(err *ToolError) {
	s.errors = append(s.errors, err)
}

// AddField adds a field validation error
func (s *ValidationErrorSet) AddField(field, message string) {
	s.Add(NewValidationError(field, message))
}

// NewValidationError creates a new validation error
func NewValidationError(field, message string) *ToolError {
	return &ToolError{
		Code:     "VALIDATION_ERROR",
		Message:  fmt.Sprintf("Field '%s': %s", field, message),
		Type:     ErrTypeValidation,
		Severity: SeverityMedium,
		Context: ErrorContext{
			Fields: map[string]interface{}{
				"field": field,
			},
		},
	}
}

// HasErrors returns true if there are any errors
func (s *ValidationErrorSet) HasErrors() bool {
	return len(s.errors) > 0
}

// Errors returns all errors
func (s *ValidationErrorSet) Errors() []*ToolError {
	return s.errors
}

// Error returns a string representation of all errors
func (s *ValidationErrorSet) Error() string {
	if !s.HasErrors() {
		return ""
	}

	var messages []string
	for _, err := range s.errors {
		messages = append(messages, err.Error())
	}
	return strings.Join(messages, "; ")
}

// ValidationOptions provides options for validation
type ValidationOptions struct {
	StrictMode bool
	MaxErrors  int
	SkipFields []string
}

// ValidationResult represents the result of a validation operation
type ValidationResult struct {
	Valid    bool
	Errors   []*ToolError
	Warnings []*ToolError
	Metadata ValidationMetadata
}

// ValidationMetadata contains metadata about the validation
type ValidationMetadata struct {
	ValidatedAt string
	Duration    string
	Rules       []string
	Version     string
}

// BaseValidator defines the interface for validators
type BaseValidator interface {
	Validate(data interface{}, options ValidationOptions) *ValidationResult
	GetName() string
	GetVersion() string
}
