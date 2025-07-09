package domain

import (
	validation "github.com/Azure/container-kit/pkg/mcp/domain/security"
)

// DockerfileValidationError represents a validation error with line/column/rule information
type DockerfileValidationError struct {
	validation.Error
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
	Rule   string `json:"rule,omitempty"`
}

// WithLine sets the line number for the error
func (e *DockerfileValidationError) WithLine(line int) *DockerfileValidationError {
	e.Line = line
	return e
}

// WithColumn sets the column number for the error
func (e *DockerfileValidationError) WithColumn(column int) *DockerfileValidationError {
	e.Column = column
	return e
}

// WithRule sets the rule for the error
func (e *DockerfileValidationError) WithRule(rule string) *DockerfileValidationError {
	e.Rule = rule
	return e
}

// NewError creates a new validation error with builder pattern support
func NewError(code, message string, errorType ErrorType, severity ErrorSeverity) *DockerfileValidationError {
	return &DockerfileValidationError{
		Error: validation.Error{
			Code:     code,
			Message:  message,
			Severity: validation.ErrorSeverity(severity),
			Context:  make(map[string]string),
		},
	}
}

// NewWarning creates a new validation warning
func NewWarning(code, message string) *DockerfileValidationWarning {
	return &DockerfileValidationWarning{
		Warning: validation.Warning{
			Code:    code,
			Message: message,
			Context: make(map[string]string),
		},
		Error: &DockerfileValidationError{
			Error: validation.Error{
				Code:     code,
				Message:  message,
				Severity: validation.SeverityLow,
				Context:  make(map[string]string),
			},
		},
	}
}

// DockerfileValidationWarning represents a validation warning with error-like capabilities
type DockerfileValidationWarning struct {
	validation.Warning
	Error *DockerfileValidationError `json:"error,omitempty"`
}

// ErrorType represents the type of error
type ErrorType string

// Error type constants
const (
	ErrTypeValidation    ErrorType = "validation"
	ErrTypeConstraint    ErrorType = "constraint"
	ErrTypeRequired      ErrorType = "required"
	ErrTypeFormat        ErrorType = "format"
	ErrTypeRange         ErrorType = "range"
	ErrTypeCustom        ErrorType = "custom"
	ErrTypeSecurity      ErrorType = "security"
	ErrTypeSyntax        ErrorType = "syntax"
	ErrTypeBestPractice  ErrorType = "best_practice"
	ErrTypeOptimization  ErrorType = "optimization"
	ErrTypeDeprecation   ErrorType = "deprecation"
	ErrTypeConfiguration ErrorType = "configuration"
	ErrTypeBuild         ErrorType = "build"
)
