package core

import (
	"fmt"
	"strings"
	"time"
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
	ErrTypeSyntax     ErrorType = "syntax"
	ErrTypeFormat     ErrorType = "format"
	ErrTypeCompliance ErrorType = "compliance"
)

// ErrorSeverity defines the severity of an error
type ErrorSeverity string

const (
	SeverityCritical ErrorSeverity = "critical"
	SeverityHigh     ErrorSeverity = "high"
	SeverityMedium   ErrorSeverity = "medium"
	SeverityLow      ErrorSeverity = "low"
	SeverityInfo     ErrorSeverity = "info"
)

// ValidationError represents a single validation error with rich context
type ValidationError struct {
	Code        string                 `json:"code"`
	Message     string                 `json:"message"`
	Type        ErrorType              `json:"type"`
	Severity    ErrorSeverity          `json:"severity"`
	Field       string                 `json:"field,omitempty"`
	Line        int                    `json:"line,omitempty"`
	Column      int                    `json:"column,omitempty"`
	Rule        string                 `json:"rule,omitempty"`
	Context     map[string]interface{} `json:"context,omitempty"`
	Suggestions []string               `json:"suggestions,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
}

// ValidationWarning represents a validation warning
type ValidationWarning struct {
	*ValidationError
}

// NewValidationError creates a new validation error
func NewValidationError(code, message string, errorType ErrorType, severity ErrorSeverity) *ValidationError {
	return &ValidationError{
		Code:      code,
		Message:   message,
		Type:      errorType,
		Severity:  severity,
		Context:   make(map[string]interface{}),
		Timestamp: time.Now(),
	}
}

// NewFieldValidationError creates a field-specific validation error
func NewFieldValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Code:      "FIELD_VALIDATION_ERROR",
		Message:   fmt.Sprintf("Field '%s': %s", field, message),
		Type:      ErrTypeValidation,
		Severity:  SeverityMedium,
		Field:     field,
		Context:   map[string]interface{}{"field": field},
		Timestamp: time.Now(),
	}
}

// NewLineValidationError creates a line-specific validation error
func NewLineValidationError(line int, message, rule string) *ValidationError {
	return &ValidationError{
		Code:      "LINE_VALIDATION_ERROR",
		Message:   message,
		Type:      ErrTypeValidation,
		Severity:  SeverityMedium,
		Line:      line,
		Rule:      rule,
		Context:   map[string]interface{}{"line": line, "rule": rule},
		Timestamp: time.Now(),
	}
}

// NewValidationWarning creates a new validation warning
func NewValidationWarning(code, message string) *ValidationWarning {
	return &ValidationWarning{
		ValidationError: &ValidationError{
			Code:      code,
			Message:   message,
			Type:      ErrTypeValidation,
			Severity:  SeverityLow,
			Context:   make(map[string]interface{}),
			Timestamp: time.Now(),
		},
	}
}

// Error implements the error interface
func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("[%s] Field '%s': %s", e.Severity, e.Field, e.Message)
	}
	if e.Line > 0 {
		return fmt.Sprintf("[%s] Line %d: %s", e.Severity, e.Line, e.Message)
	}
	return fmt.Sprintf("[%s] %s", e.Severity, e.Message)
}

// WithField adds field information to the error
func (e *ValidationError) WithField(field string) *ValidationError {
	e.Field = field
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context["field"] = field
	return e
}

// WithLine adds line information to the error
func (e *ValidationError) WithLine(line int) *ValidationError {
	e.Line = line
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context["line"] = line
	return e
}

// WithColumn adds column information to the error
func (e *ValidationError) WithColumn(column int) *ValidationError {
	e.Column = column
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context["column"] = column
	return e
}

// WithRule adds rule information to the error
func (e *ValidationError) WithRule(rule string) *ValidationError {
	e.Rule = rule
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context["rule"] = rule
	return e
}

// WithContext adds context information to the error
func (e *ValidationError) WithContext(key string, value interface{}) *ValidationError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// WithSuggestion adds a suggestion to the error
func (e *ValidationError) WithSuggestion(suggestion string) *ValidationError {
	e.Suggestions = append(e.Suggestions, suggestion)
	return e
}

// ValidationResult represents the unified result of a validation operation
type ValidationResult struct {
	Valid       bool                 `json:"valid"`
	Errors      []*ValidationError   `json:"errors,omitempty"`
	Warnings    []*ValidationWarning `json:"warnings,omitempty"`
	Score       float64              `json:"score,omitempty"`      // 0-100 validation score
	RiskLevel   string               `json:"risk_level,omitempty"` // low, medium, high, critical
	Metadata    ValidationMetadata   `json:"metadata"`
	Duration    time.Duration        `json:"duration"`
	Suggestions []string             `json:"suggestions,omitempty"`
}

// ValidationMetadata contains metadata about the validation
type ValidationMetadata struct {
	ValidatedAt      time.Time              `json:"validated_at"`
	ValidatorName    string                 `json:"validator_name"`
	ValidatorVersion string                 `json:"validator_version"`
	RulesApplied     []string               `json:"rules_applied,omitempty"`
	Context          map[string]interface{} `json:"context,omitempty"`
}

// HasErrors returns true if the result contains any errors
func (r *ValidationResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// HasWarnings returns true if the result contains any warnings
func (r *ValidationResult) HasWarnings() bool {
	return len(r.Warnings) > 0
}

// HasCriticalErrors returns true if the result contains critical errors
func (r *ValidationResult) HasCriticalErrors() bool {
	for _, err := range r.Errors {
		if err.Severity == SeverityCritical {
			return true
		}
	}
	return false
}

// ErrorCount returns the total number of errors
func (r *ValidationResult) ErrorCount() int {
	return len(r.Errors)
}

// WarningCount returns the total number of warnings
func (r *ValidationResult) WarningCount() int {
	return len(r.Warnings)
}

// ErrorsBySeverity returns errors grouped by severity
func (r *ValidationResult) ErrorsBySeverity() map[ErrorSeverity][]*ValidationError {
	grouped := make(map[ErrorSeverity][]*ValidationError)
	for _, err := range r.Errors {
		grouped[err.Severity] = append(grouped[err.Severity], err)
	}
	return grouped
}

// AddError adds an error to the result
func (r *ValidationResult) AddError(err *ValidationError) {
	r.Errors = append(r.Errors, err)
	r.Valid = false
}

// AddWarning adds a warning to the result
func (r *ValidationResult) AddWarning(warning *ValidationWarning) {
	r.Warnings = append(r.Warnings, warning)
}

// AddFieldError adds a field validation error
func (r *ValidationResult) AddFieldError(field, message string) {
	r.AddError(NewFieldValidationError(field, message))
}

// AddLineError adds a line validation error
func (r *ValidationResult) AddLineError(line int, message, rule string) {
	r.AddError(NewLineValidationError(line, message, rule))
}

// AddSuggestion adds a suggestion to the result
func (r *ValidationResult) AddSuggestion(suggestion string) {
	r.Suggestions = append(r.Suggestions, suggestion)
}

// Merge combines two validation results
func (r *ValidationResult) Merge(other *ValidationResult) {
	r.Errors = append(r.Errors, other.Errors...)
	r.Warnings = append(r.Warnings, other.Warnings...)
	r.Valid = r.Valid && other.Valid
	r.Suggestions = append(r.Suggestions, other.Suggestions...)
	r.Duration += other.Duration
}

// String returns a string representation of the validation result
func (r *ValidationResult) String() string {
	if r.Valid {
		return fmt.Sprintf("Validation passed with %d warnings", len(r.Warnings))
	}
	return fmt.Sprintf("Validation failed with %d errors and %d warnings", len(r.Errors), len(r.Warnings))
}

// Error returns a string representation of all errors
func (r *ValidationResult) Error() string {
	if r.Valid {
		return ""
	}

	var messages []string
	for _, err := range r.Errors {
		messages = append(messages, err.Error())
	}
	return strings.Join(messages, "; ")
}

// ValidationOptions provides configuration for validation operations
type ValidationOptions struct {
	StrictMode      bool                   `json:"strict_mode"`
	MaxErrors       int                    `json:"max_errors"`
	SkipFields      []string               `json:"skip_fields,omitempty"`
	SkipRules       []string               `json:"skip_rules,omitempty"`
	EnabledRules    []string               `json:"enabled_rules,omitempty"`
	Context         map[string]interface{} `json:"context,omitempty"`
	FailFast        bool                   `json:"fail_fast"`
	IncludeWarnings bool                   `json:"include_warnings"`
	Timeout         time.Duration          `json:"timeout,omitempty"`
}

// NewValidationOptions creates default validation options
func NewValidationOptions() *ValidationOptions {
	return &ValidationOptions{
		StrictMode:      false,
		MaxErrors:       100,
		SkipFields:      []string{},
		SkipRules:       []string{},
		EnabledRules:    []string{},
		Context:         make(map[string]interface{}),
		FailFast:        false,
		IncludeWarnings: true,
		Timeout:         30 * time.Second,
	}
}

// WithStrictMode enables or disables strict mode
func (o *ValidationOptions) WithStrictMode(strict bool) *ValidationOptions {
	o.StrictMode = strict
	return o
}

// WithMaxErrors sets the maximum number of errors
func (o *ValidationOptions) WithMaxErrors(max int) *ValidationOptions {
	o.MaxErrors = max
	return o
}

// WithFailFast enables or disables fail-fast mode
func (o *ValidationOptions) WithFailFast(failFast bool) *ValidationOptions {
	o.FailFast = failFast
	return o
}

// WithTimeout sets the validation timeout
func (o *ValidationOptions) WithTimeout(timeout time.Duration) *ValidationOptions {
	o.Timeout = timeout
	return o
}

// WithContext adds context to validation options
func (o *ValidationOptions) WithContext(key string, value interface{}) *ValidationOptions {
	if o.Context == nil {
		o.Context = make(map[string]interface{})
	}
	o.Context[key] = value
	return o
}

// ShouldSkipField returns true if the field should be skipped
func (o *ValidationOptions) ShouldSkipField(field string) bool {
	for _, skip := range o.SkipFields {
		if skip == field {
			return true
		}
	}
	return false
}

// ShouldSkipRule returns true if the rule should be skipped
func (o *ValidationOptions) ShouldSkipRule(rule string) bool {
	for _, skip := range o.SkipRules {
		if skip == rule {
			return true
		}
	}
	return false
}

// IsRuleEnabled returns true if the rule is enabled
func (o *ValidationOptions) IsRuleEnabled(rule string) bool {
	if len(o.EnabledRules) == 0 {
		return true // If no specific rules enabled, all are enabled by default
	}
	for _, enabled := range o.EnabledRules {
		if enabled == rule {
			return true
		}
	}
	return false
}
