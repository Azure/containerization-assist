package core

import (
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
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

// Error represents a single validation error with rich context
type Error struct {
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

// Warning represents a validation warning
type Warning struct {
	*Error
}

// NewError creates a new validation error
func NewError(code, message string, errorType ErrorType, severity ErrorSeverity) *Error {
	return &Error{
		Code:      code,
		Message:   message,
		Type:      errorType,
		Severity:  severity,
		Context:   make(map[string]interface{}),
		Timestamp: time.Now(),
	}
}

// NewFieldError creates a field-specific validation error
func NewFieldError(field, message string) *Error {
	return &Error{
		Code:      "FIELD_VALIDATION_ERROR",
		Message:   fmt.Sprintf("Field '%s': %s", field, message),
		Type:      ErrTypeValidation,
		Severity:  SeverityMedium,
		Field:     field,
		Context:   map[string]interface{}{"field": field},
		Timestamp: time.Now(),
	}
}

// NewLineError creates a line-specific validation error
func NewLineError(line int, message, rule string) *Error {
	return &Error{
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

// NewWarning creates a new validation warning
func NewWarning(code, message string) *Warning {
	return &Warning{
		Error: &Error{
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
func (e *Error) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("[%s] Field '%s': %s", e.Severity, e.Field, e.Message)
	}
	if e.Line > 0 {
		return fmt.Sprintf("[%s] Line %d: %s", e.Severity, e.Line, e.Message)
	}
	return fmt.Sprintf("[%s] %s", e.Severity, e.Message)
}

// WithField adds field information to the error
func (e *Error) WithField(field string) *Error {
	e.Field = field
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context["field"] = field
	return e
}

// WithLine adds line information to the error
func (e *Error) WithLine(line int) *Error {
	e.Line = line
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context["line"] = line
	return e
}

// WithColumn adds column information to the error
func (e *Error) WithColumn(column int) *Error {
	e.Column = column
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context["column"] = column
	return e
}

// WithRule adds rule information to the error
func (e *Error) WithRule(rule string) *Error {
	e.Rule = rule
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context["rule"] = rule
	return e
}

// WithContext adds context information to the error
func (e *Error) WithContext(key string, value interface{}) *Error {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// WithSuggestion adds a suggestion to the error
func (e *Error) WithSuggestion(suggestion string) *Error {
	e.Suggestions = append(e.Suggestions, suggestion)
	return e
}

// Result represents the unified result of a validation operation
// Generic type T allows for domain-specific data to be included in validation results
type Result[T any] struct {
	Valid       bool               `json:"valid"`
	Errors      []*Error           `json:"errors,omitempty"`
	Warnings    []*Warning         `json:"warnings,omitempty"`
	Data        T                  `json:"data,omitempty"`       // Domain-specific validation data
	Score       float64            `json:"score,omitempty"`      // 0-100 validation score
	RiskLevel   string             `json:"risk_level,omitempty"` // low, medium, high, critical
	Metadata    ValidationMetadata `json:"metadata"`
	Duration    time.Duration      `json:"duration"`
	Suggestions []string           `json:"suggestions,omitempty"`
	Timestamp   time.Time          `json:"timestamp"` // When validation was performed
}

// NonGenericResult provides backward compatibility for non-generic usage
type NonGenericResult = Result[interface{}]

// Domain-specific validation result type aliases for easy migration
type BuildResult = Result[BuildValidationData]
type DeployResult = Result[DeployValidationData]
type SecurityResult = Result[SecurityValidationData]
type SessionResult = Result[SessionValidationData]
type RuntimeResult = Result[RuntimeValidationData]

// Domain-specific validation data types
type BuildValidationData struct {
	DockerfilePath   string                 `json:"dockerfile_path,omitempty"`
	ImageName        string                 `json:"image_name,omitempty"`
	ImageTag         string                 `json:"image_tag,omitempty"`
	BuildContext     map[string]interface{} `json:"build_context,omitempty"`
	SecurityFindings []SecurityFinding      `json:"security_findings,omitempty"`
}

type DeployValidationData struct {
	ManifestPath string                 `json:"manifest_path,omitempty"`
	Namespace    string                 `json:"namespace,omitempty"`
	Resources    []KubernetesResource   `json:"resources,omitempty"`
	ClusterInfo  map[string]interface{} `json:"cluster_info,omitempty"`
	HealthChecks []HealthCheck          `json:"health_checks,omitempty"`
}

type SecurityValidationData struct {
	ScanType         string            `json:"scan_type,omitempty"`
	Vulnerabilities  []Vulnerability   `json:"vulnerabilities,omitempty"`
	PolicyViolations []PolicyViolation `json:"policy_violations,omitempty"`
	ComplianceChecks []ComplianceCheck `json:"compliance_checks,omitempty"`
}

type SessionValidationData struct {
	SessionID     string                 `json:"session_id,omitempty"`
	StateInfo     map[string]interface{} `json:"state_info,omitempty"`
	ToolsExecuted []string               `json:"tools_executed,omitempty"`
}

type RuntimeValidationData struct {
	ToolName         string                 `json:"tool_name,omitempty"`
	PerformanceStats map[string]interface{} `json:"performance_stats,omitempty"`
	ResourceUsage    map[string]interface{} `json:"resource_usage,omitempty"`
}

// Supporting types for domain-specific data
type SecurityFinding struct {
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
	Line        int    `json:"line,omitempty"`
}

type KubernetesResource struct {
	APIVersion string `json:"api_version"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Namespace  string `json:"namespace,omitempty"`
}

type HealthCheck struct {
	Type   string `json:"type"`
	Status string `json:"status"`
	URL    string `json:"url,omitempty"`
}

type Vulnerability struct {
	ID          string `json:"id"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
	Package     string `json:"package,omitempty"`
}

type PolicyViolation struct {
	PolicyName string `json:"policy_name"`
	Rule       string `json:"rule"`
	Message    string `json:"message"`
}

type ComplianceCheck struct {
	Standard string `json:"standard"`
	Control  string `json:"control"`
	Status   string `json:"status"`
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
func (r *Result[T]) HasErrors() bool {
	return len(r.Errors) > 0
}

// HasWarnings returns true if the result contains any warnings
func (r *Result[T]) HasWarnings() bool {
	return len(r.Warnings) > 0
}

// HasCriticalErrors returns true if the result contains critical errors
func (r *Result[T]) HasCriticalErrors() bool {
	for _, err := range r.Errors {
		if err.Severity == SeverityCritical {
			return true
		}
	}
	return false
}

// ErrorCount returns the total number of errors
func (r *Result[T]) ErrorCount() int {
	return len(r.Errors)
}

// WarningCount returns the total number of warnings
func (r *Result[T]) WarningCount() int {
	return len(r.Warnings)
}

// ErrorsBySeverity returns errors grouped by severity
func (r *Result[T]) ErrorsBySeverity() map[ErrorSeverity][]*Error {
	grouped := make(map[ErrorSeverity][]*Error)
	for _, err := range r.Errors {
		grouped[err.Severity] = append(grouped[err.Severity], err)
	}
	return grouped
}

// AddError adds an error to the result
func (r *Result[T]) AddError(err *Error) {
	r.Errors = append(r.Errors, err)
	r.Valid = false
}

// AddWarning adds a warning to the result
func (r *Result[T]) AddWarning(warning *Warning) {
	r.Warnings = append(r.Warnings, warning)
}

// AddFieldError adds a field validation error
func (r *Result[T]) AddFieldError(field, message string) {
	r.AddError(NewFieldError(field, message))
}

// AddLineError adds a line validation error
func (r *Result[T]) AddLineError(line int, message, rule string) {
	r.AddError(NewLineError(line, message, rule))
}

// AddSuggestion adds a suggestion to the result
func (r *Result[T]) AddSuggestion(suggestion string) {
	r.Suggestions = append(r.Suggestions, suggestion)
}

// SetData sets the domain-specific validation data
func (r *Result[T]) SetData(data T) {
	r.Data = data
}

// GetData returns the domain-specific validation data
func (r *Result[T]) GetData() T {
	return r.Data
}

// Merge combines two validation results
func (r *Result[T]) Merge(other *Result[T]) {
	r.Errors = append(r.Errors, other.Errors...)
	r.Warnings = append(r.Warnings, other.Warnings...)
	r.Valid = r.Valid && other.Valid
	r.Suggestions = append(r.Suggestions, other.Suggestions...)
	r.Duration += other.Duration
}

// String returns a string representation of the validation result
func (r *Result[T]) String() string {
	if r.Valid {
		return fmt.Sprintf("Validation passed with %d warnings", len(r.Warnings))
	}
	return fmt.Sprintf("Validation failed with %d errors and %d warnings", len(r.Errors), len(r.Warnings))
}

// Error returns a string representation of all errors
func (r *Result[T]) Error() string {
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

// ============================================================================
// Factory Functions for Domain-Specific Result Types
// ============================================================================

// NewBuildResult creates a new build validation result
func NewBuildResult(validatorName, validatorVersion string) *BuildResult {
	return &BuildResult{
		Valid:     true,
		Errors:    []*Error{},
		Warnings:  []*Warning{},
		Data:      BuildValidationData{},
		Score:     100.0,
		RiskLevel: "low",
		Metadata: ValidationMetadata{
			ValidatedAt:      time.Now(),
			ValidatorName:    validatorName,
			ValidatorVersion: validatorVersion,
			Context:          make(map[string]interface{}),
		},
		Duration:    0,
		Suggestions: []string{},
		Timestamp:   time.Now(),
	}
}

// NewDeployResult creates a new deploy validation result
func NewDeployResult(validatorName, validatorVersion string) *DeployResult {
	return &DeployResult{
		Valid:     true,
		Errors:    []*Error{},
		Warnings:  []*Warning{},
		Data:      DeployValidationData{},
		Score:     100.0,
		RiskLevel: "low",
		Metadata: ValidationMetadata{
			ValidatedAt:      time.Now(),
			ValidatorName:    validatorName,
			ValidatorVersion: validatorVersion,
			Context:          make(map[string]interface{}),
		},
		Duration:    0,
		Suggestions: []string{},
		Timestamp:   time.Now(),
	}
}

// NewSecurityResult creates a new security validation result
func NewSecurityResult(validatorName, validatorVersion string) *SecurityResult {
	return &SecurityResult{
		Valid:     true,
		Errors:    []*Error{},
		Warnings:  []*Warning{},
		Data:      SecurityValidationData{},
		Score:     100.0,
		RiskLevel: "low",
		Metadata: ValidationMetadata{
			ValidatedAt:      time.Now(),
			ValidatorName:    validatorName,
			ValidatorVersion: validatorVersion,
			Context:          make(map[string]interface{}),
		},
		Duration:    0,
		Suggestions: []string{},
		Timestamp:   time.Now(),
	}
}

// NewSessionResult creates a new session validation result
func NewSessionResult(validatorName, validatorVersion string) *SessionResult {
	return &SessionResult{
		Valid:     true,
		Errors:    []*Error{},
		Warnings:  []*Warning{},
		Data:      SessionValidationData{},
		Score:     100.0,
		RiskLevel: "low",
		Metadata: ValidationMetadata{
			ValidatedAt:      time.Now(),
			ValidatorName:    validatorName,
			ValidatorVersion: validatorVersion,
			Context:          make(map[string]interface{}),
		},
		Duration:    0,
		Suggestions: []string{},
		Timestamp:   time.Now(),
	}
}

// NewRuntimeResult creates a new runtime validation result
func NewRuntimeResult(validatorName, validatorVersion string) *RuntimeResult {
	return &RuntimeResult{
		Valid:     true,
		Errors:    []*Error{},
		Warnings:  []*Warning{},
		Data:      RuntimeValidationData{},
		Score:     100.0,
		RiskLevel: "low",
		Metadata: ValidationMetadata{
			ValidatedAt:      time.Now(),
			ValidatorName:    validatorName,
			ValidatorVersion: validatorVersion,
			Context:          make(map[string]interface{}),
		},
		Duration:    0,
		Suggestions: []string{},
		Timestamp:   time.Now(),
	}
}

// NewGenericResult creates a new generic Result with any data type
func NewGenericResult[T any](validatorName, validatorVersion string) *Result[T] {
	return &Result[T]{
		Valid:    true,
		Errors:   []*Error{},
		Warnings: []*Warning{},
		Data:     *new(T), // Initialize with zero value of T
		Score:    100,
		Metadata: ValidationMetadata{
			ValidatedAt:      time.Now(),
			ValidatorName:    validatorName,
			ValidatorVersion: validatorVersion,
			Context:          make(map[string]interface{}),
			RulesApplied:     []string{},
		},
		Duration:    0,
		Suggestions: []string{},
		Timestamp:   time.Now(),
	}
}

// NewNonGenericResult creates a new NonGenericResult
func NewNonGenericResult(validatorName, validatorVersion string) *NonGenericResult {
	return &NonGenericResult{
		Valid:    true,
		Errors:   []*Error{},
		Warnings: []*Warning{},
		Data:     interface{}(nil),
		Score:    100,
		Metadata: ValidationMetadata{
			ValidatedAt:      time.Now(),
			ValidatorName:    validatorName,
			ValidatorVersion: validatorVersion,
			Context:          make(map[string]interface{}),
			RulesApplied:     []string{},
		},
		Duration:    0,
		Suggestions: []string{},
		Timestamp:   time.Now(),
	}
}

// ============================================================================
// Rich Error Integration with GAMMA's Error System
// ============================================================================

// NewErrorFromRichError creates a Error from GAMMA's RichError
func NewErrorFromRichError(richErr *errors.RichError) *Error {
	return &Error{
		Code:        string(richErr.Code),
		Message:     richErr.Message,
		Type:        mapErrorTypeFromRich(richErr.Type),
		Severity:    mapSeverityFromRich(richErr.Severity),
		Context:     richErr.Context,
		Timestamp:   time.Now(),
		Suggestions: []string{}, // Rich error suggestions can be added here
	}
}

// ToRichError converts a Error to GAMMA's RichError
func (e *Error) ToRichError() *errors.RichError {
	return &errors.RichError{
		Code:     errors.ErrorCode(e.Code),
		Message:  e.Message,
		Type:     mapErrorTypeToRich(e.Type),
		Severity: mapSeverityToRich(e.Severity),
		Context:  e.Context,
	}
}

// Domain-specific error factories integrating with Rich Error System
func NewBuildError(code errors.ErrorCode, message, field string) *Error {
	result := NewErrorFromRichError(&errors.RichError{
		Code:     code,
		Message:  message,
		Type:     errors.ErrTypeContainer,
		Severity: errors.SeverityMedium,
		Context:  map[string]interface{}{"field": field, "domain": "build"},
	}).WithField(field)
	// Add suggestion
	result.Suggestions = append(result.Suggestions, "Check build configuration and Dockerfile syntax")
	return result
}

func NewDeployError(code errors.ErrorCode, message, field string) *Error {
	result := NewErrorFromRichError(&errors.RichError{
		Code:     code,
		Message:  message,
		Type:     errors.ErrTypeKubernetes,
		Severity: errors.SeverityMedium,
		Context:  map[string]interface{}{"field": field, "domain": "deploy"},
	}).WithField(field)
	// Add suggestion
	result.Suggestions = append(result.Suggestions, "Check Kubernetes manifest and cluster configuration")
	return result
}

func NewSecurityError(code errors.ErrorCode, message, field string) *Error {
	result := NewErrorFromRichError(&errors.RichError{
		Code:     code,
		Message:  message,
		Type:     errors.ErrTypeSecurity,
		Severity: errors.SeverityHigh,
		Context:  map[string]interface{}{"field": field, "domain": "security"},
	}).WithField(field)
	// Add suggestion
	result.Suggestions = append(result.Suggestions, "Review security policies and scan configurations")
	return result
}

// Helper functions to map between error types and severities
func mapErrorTypeFromRich(richType errors.ErrorType) ErrorType {
	switch richType {
	case errors.ErrTypeValidation:
		return ErrTypeValidation
	case errors.ErrTypeContainer:
		return ErrTypeBuild
	case errors.ErrTypeKubernetes:
		return ErrTypeDeployment
	case errors.ErrTypeSecurity:
		return ErrTypeSecurity
	case errors.ErrTypeNetwork:
		return ErrTypeNetwork
	case errors.ErrTypePermission:
		return ErrTypePermission
	case errors.ErrTypeConfiguration:
		return ErrTypeConfig
	default:
		return ErrTypeSystem
	}
}

func mapErrorTypeToRich(errType ErrorType) errors.ErrorType {
	switch errType {
	case ErrTypeValidation:
		return errors.ErrTypeValidation
	case ErrTypeBuild:
		return errors.ErrTypeContainer
	case ErrTypeDeployment:
		return errors.ErrTypeKubernetes
	case ErrTypeSecurity:
		return errors.ErrTypeSecurity
	case ErrTypeNetwork:
		return errors.ErrTypeNetwork
	case ErrTypePermission:
		return errors.ErrTypePermission
	case ErrTypeConfig:
		return errors.ErrTypeConfiguration
	default:
		return errors.ErrTypeSystem
	}
}

func mapSeverityFromRich(richSeverity errors.ErrorSeverity) ErrorSeverity {
	switch richSeverity {
	case errors.SeverityLow:
		return SeverityLow
	case errors.SeverityMedium:
		return SeverityMedium
	case errors.SeverityHigh:
		return SeverityHigh
	case errors.SeverityCritical:
		return SeverityCritical
	default:
		return SeverityInfo
	}
}

func mapSeverityToRich(severity ErrorSeverity) errors.ErrorSeverity {
	switch severity {
	case SeverityLow:
		return errors.SeverityLow
	case SeverityMedium:
		return errors.SeverityMedium
	case SeverityHigh:
		return errors.SeverityHigh
	case SeverityCritical:
		return errors.SeverityCritical
	default:
		return errors.SeverityLow
	}
}
