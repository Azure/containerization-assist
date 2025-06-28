package runtime

import (
	"context"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/errors"
)

type BaseValidator interface {
	Validate(ctx context.Context, input interface{}, options ValidationOptions) (*ValidationResult, error)

	GetName() string
}

type ValidationOptions struct {
	Severity string

	IgnoreRules []string

	StrictMode bool

	CustomParams map[string]interface{}
}

type ValidationResult struct {
	IsValid bool
	Score   int

	Errors   []ValidationError
	Warnings []ValidationWarning

	TotalIssues    int
	CriticalIssues int

	Context  map[string]interface{}
	Metadata ValidationMetadata
}

type ValidationError struct {
	Code          string
	Type          string
	Message       string
	Severity      string
	Location      ErrorLocation
	Fix           string
	Documentation string
}

type ValidationWarning struct {
	Code       string
	Type       string
	Message    string
	Suggestion string
	Impact     string
	Location   WarningLocation
}

type ErrorLocation struct {
	File   string
	Line   int
	Column int
	Path   string
}

type WarningLocation struct {
	File string
	Line int
	Path string
}

type ValidationMetadata struct {
	ValidatorName    string
	ValidatorVersion string
	Duration         time.Duration
	Timestamp        time.Time
	Parameters       map[string]interface{}
}

type BaseValidatorImpl struct {
	Name    string
	Version string
}

func NewBaseValidator(name, version string) *BaseValidatorImpl {
	return &BaseValidatorImpl{
		Name:    name,
		Version: version,
	}
}

func (v *BaseValidatorImpl) GetName() string {
	return v.Name
}

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

func (r *ValidationResult) AddError(err ValidationError) {
	r.Errors = append(r.Errors, err)
	r.TotalIssues++

	if err.Severity == "critical" || err.Severity == "high" {
		r.CriticalIssues++
	}

	r.IsValid = false
}

func (r *ValidationResult) AddWarning(warn ValidationWarning) {
	r.Warnings = append(r.Warnings, warn)
	r.TotalIssues++
}

func (r *ValidationResult) CalculateScore() {
	score := 100

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

	score -= len(r.Warnings) * 2

	if score < 0 {
		score = 0
	}

	r.Score = score
}

func (r *ValidationResult) Merge(other *ValidationResult) {
	if other == nil {
		return
	}

	r.Errors = append(r.Errors, other.Errors...)
	r.Warnings = append(r.Warnings, other.Warnings...)

	r.TotalIssues += other.TotalIssues
	r.CriticalIssues += other.CriticalIssues

	if !other.IsValid {
		r.IsValid = false
	}

	for k, v := range other.Context {
		r.Context[k] = v
	}
}

func (r *ValidationResult) FilterBySeverity(minSeverity string) {
	severityLevel := GetSeverityLevel(minSeverity)

	filteredErrors := make([]ValidationError, 0)
	for _, err := range r.Errors {
		if GetSeverityLevel(err.Severity) >= severityLevel {
			filteredErrors = append(filteredErrors, err)
		}
	}
	r.Errors = filteredErrors

	r.TotalIssues = len(r.Errors) + len(r.Warnings)
	r.CriticalIssues = 0
	for _, err := range r.Errors {
		if err.Severity == "critical" || err.Severity == "high" {
			r.CriticalIssues++
		}
	}
}

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

type ValidationContext struct {
	SessionID  string
	WorkingDir string
	Options    ValidationOptions
	Logger     interface{}
	StartTime  time.Time
	Custom     map[string]interface{}
}

func NewValidationContext(sessionID, workingDir string, options ValidationOptions) *ValidationContext {
	return &ValidationContext{
		SessionID:  sessionID,
		WorkingDir: workingDir,
		Options:    options,
		StartTime:  time.Now(),
		Custom:     make(map[string]interface{}),
	}
}

func (c *ValidationContext) Duration() time.Duration {
	return time.Since(c.StartTime)
}

type ValidatorChain struct {
	validators []BaseValidator
}

func NewValidatorChain(validators ...BaseValidator) *ValidatorChain {
	return &ValidatorChain{
		validators: validators,
	}
}

func (c *ValidatorChain) Validate(ctx context.Context, input interface{}, options ValidationOptions) (*ValidationResult, error) {
	result := &ValidationResult{
		IsValid:  true,
		Errors:   make([]ValidationError, 0),
		Warnings: make([]ValidationWarning, 0),
		Context:  make(map[string]interface{}),
	}

	for _, validator := range c.validators {
		vResult, err := validator.Validate(ctx, input, options)
		if err != nil {
			return nil, errors.Wrapf(err, "runtime/validator", "validator %s failed", validator.GetName())
		}

		result.Merge(vResult)
	}

	result.CalculateScore()

	return result, nil
}

func (c *ValidatorChain) GetName() string {
	return "ValidatorChain"
}
