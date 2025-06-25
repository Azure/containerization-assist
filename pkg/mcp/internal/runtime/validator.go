package runtime

import (
	"context"
	"fmt"
	"time"
)

// BaseValidator defines the base interface for all validators
type BaseValidator interface {
	// Validate performs validation and returns a result
	Validate(ctx context.Context, input interface{}, options ValidationOptions) (*ValidationResult, error)

	// GetName returns the validator name
	GetName() string
}

// ValidationOptions provides common options for validation
type ValidationOptions struct {
	// Severity level for filtering issues
	Severity string

	// Rules to ignore during validation
	IgnoreRules []string

	// Enable strict validation mode
	StrictMode bool

	// Custom validation parameters
	CustomParams map[string]interface{}
}

// ValidationResult represents the result of validation
type ValidationResult struct {
	// Overall validation status
	IsValid bool
	Score   int // 0-100

	// Issues found during validation
	Errors   []ValidationError
	Warnings []ValidationWarning

	// Summary statistics
	TotalIssues    int
	CriticalIssues int

	// Additional context
	Context  map[string]interface{}
	Metadata ValidationMetadata
}

// ValidationError represents a validation error
type ValidationError struct {
	Code          string
	Type          string
	Message       string
	Severity      string // critical, high, medium, low
	Location      ErrorLocation
	Fix           string
	Documentation string
}

// ValidationWarning represents a validation warning
type ValidationWarning struct {
	Code       string
	Type       string
	Message    string
	Suggestion string
	Impact     string // performance, security, maintainability, etc.
	Location   WarningLocation
}

// ErrorLocation provides location information for an error
type ErrorLocation struct {
	File   string
	Line   int
	Column int
	Path   string // JSON path or similar
}

// WarningLocation provides location information for a warning
type WarningLocation struct {
	File string
	Line int
	Path string
}

// ValidationMetadata provides metadata about the validation
type ValidationMetadata struct {
	ValidatorName    string
	ValidatorVersion string
	Duration         time.Duration
	Timestamp        time.Time
	Parameters       map[string]interface{}
}

// BaseValidator provides common functionality for validators
type BaseValidatorImpl struct {
	Name    string
	Version string
}

// NewBaseValidator creates a new base validator
func NewBaseValidator(name, version string) *BaseValidatorImpl {
	return &BaseValidatorImpl{
		Name:    name,
		Version: version,
	}
}

// GetName returns the validator name
func (v *BaseValidatorImpl) GetName() string {
	return v.Name
}

// CreateResult creates a new validation result with metadata
func (v *BaseValidatorImpl) CreateResult() *ValidationResult {
	return &ValidationResult{
		IsValid:  true,
		Score:    100,
		Errors:   make([]ValidationError, 0),
		Warnings: make([]ValidationWarning, 0),
		Context:  make(map[string]interface{}),
		Metadata: ValidationMetadata{
			ValidatorName:    v.Name,
			ValidatorVersion: v.Version,
			Timestamp:        time.Now(),
			Parameters:       make(map[string]interface{}),
		},
	}
}

// AddError adds an error to the validation result
func (r *ValidationResult) AddError(err ValidationError) {
	r.Errors = append(r.Errors, err)
	r.TotalIssues++

	if err.Severity == "critical" || err.Severity == "high" {
		r.CriticalIssues++
	}

	// Update validity
	r.IsValid = false
}

// AddWarning adds a warning to the validation result
func (r *ValidationResult) AddWarning(warn ValidationWarning) {
	r.Warnings = append(r.Warnings, warn)
	r.TotalIssues++
}

// CalculateScore calculates the validation score based on issues
func (r *ValidationResult) CalculateScore() {
	score := 100

	// Deduct points for errors
	for _, err := range r.Errors {
		switch err.Severity {
		case "critical":
			score -= 20
		case "high":
			score -= 15
		case "medium":
			score -= 10
		case "low":
			score -= 5
		}
	}

	// Deduct points for warnings (less severe)
	score -= len(r.Warnings) * 2

	// Ensure score doesn't go below 0
	if score < 0 {
		score = 0
	}

	r.Score = score
}

// Merge merges another validation result into this one
func (r *ValidationResult) Merge(other *ValidationResult) {
	if other == nil {
		return
	}

	// Merge errors and warnings
	r.Errors = append(r.Errors, other.Errors...)
	r.Warnings = append(r.Warnings, other.Warnings...)

	// Update counts
	r.TotalIssues += other.TotalIssues
	r.CriticalIssues += other.CriticalIssues

	// Update validity
	if !other.IsValid {
		r.IsValid = false
	}

	// Merge context
	for k, v := range other.Context {
		r.Context[k] = v
	}
}

// FilterBySeverity filters issues by minimum severity level
func (r *ValidationResult) FilterBySeverity(minSeverity string) {
	severityLevel := GetSeverityLevel(minSeverity)

	// Filter errors
	filteredErrors := make([]ValidationError, 0)
	for _, err := range r.Errors {
		if GetSeverityLevel(err.Severity) >= severityLevel {
			filteredErrors = append(filteredErrors, err)
		}
	}
	r.Errors = filteredErrors

	// Recalculate counts
	r.TotalIssues = len(r.Errors) + len(r.Warnings)
	r.CriticalIssues = 0
	for _, err := range r.Errors {
		if err.Severity == "critical" || err.Severity == "high" {
			r.CriticalIssues++
		}
	}
}

// GetSeverityLevel returns numeric severity level
func GetSeverityLevel(severity string) int {
	switch severity {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

// ValidationContext provides context for validation operations
type ValidationContext struct {
	SessionID  string
	WorkingDir string
	Options    ValidationOptions
	Logger     interface{} // zerolog.Logger
	StartTime  time.Time
	Custom     map[string]interface{}
}

// NewValidationContext creates a new validation context
func NewValidationContext(sessionID, workingDir string, options ValidationOptions) *ValidationContext {
	return &ValidationContext{
		SessionID:  sessionID,
		WorkingDir: workingDir,
		Options:    options,
		StartTime:  time.Now(),
		Custom:     make(map[string]interface{}),
	}
}

// Duration returns the elapsed time since validation started
func (c *ValidationContext) Duration() time.Duration {
	return time.Since(c.StartTime)
}

// ValidatorChain allows chaining multiple validators
type ValidatorChain struct {
	validators []BaseValidator
}

// NewValidatorChain creates a new validator chain
func NewValidatorChain(validators ...BaseValidator) *ValidatorChain {
	return &ValidatorChain{
		validators: validators,
	}
}

// Validate runs all validators in the chain
func (c *ValidatorChain) Validate(ctx context.Context, input interface{}, options ValidationOptions) (*ValidationResult, error) {
	result := &ValidationResult{
		IsValid:  true,
		Errors:   make([]ValidationError, 0),
		Warnings: make([]ValidationWarning, 0),
		Context:  make(map[string]interface{}),
	}

	// Run each validator
	for _, validator := range c.validators {
		vResult, err := validator.Validate(ctx, input, options)
		if err != nil {
			return nil, fmt.Errorf("validator %s failed: %w", validator.GetName(), err)
		}

		// Merge results
		result.Merge(vResult)
	}

	// Calculate final score
	result.CalculateScore()

	return result, nil
}

// GetName returns the chain name
func (c *ValidatorChain) GetName() string {
	return "ValidatorChain"
}
