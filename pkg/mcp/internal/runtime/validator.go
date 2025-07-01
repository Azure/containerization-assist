package runtime

import (
	"context"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/Azure/container-kit/pkg/mcp/validation/core"
	"github.com/Azure/container-kit/pkg/mcp/validation/validators"
)

// BaseValidator interface for runtime validation - bridges to unified validation
type BaseValidator interface {
	Validate(ctx context.Context, input interface{}, options ValidationOptions) (*ValidationResult, error)
	ValidateUnified(ctx context.Context, input interface{}, options *core.ValidationOptions) (*core.ValidationResult, error)

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

// BaseValidatorImpl provides runtime validation with unified validation integration
type BaseValidatorImpl struct {
	Name             string
	Version          string
	unifiedValidator core.Validator // Bridge to unified validation
}

func NewBaseValidator(name, version string) *BaseValidatorImpl {
	return &BaseValidatorImpl{
		Name:    name,
		Version: version,
		// Create a base unified validator for fallback
		unifiedValidator: validators.NewBaseValidator(name, version, []string{"runtime"}),
	}
}

// NewBaseValidatorWithUnified creates a runtime validator with a specific unified validator
func NewBaseValidatorWithUnified(name, version string, unified core.Validator) *BaseValidatorImpl {
	return &BaseValidatorImpl{
		Name:             name,
		Version:          version,
		unifiedValidator: unified,
	}
}

func (v *BaseValidatorImpl) GetName() string {
	return v.Name
}

// ValidateUnified performs validation using the unified validation framework
func (v *BaseValidatorImpl) ValidateUnified(ctx context.Context, input interface{}, options *core.ValidationOptions) (*core.ValidationResult, error) {
	if v.unifiedValidator != nil {
		return v.unifiedValidator.Validate(ctx, input, options), nil
	}

	// Fallback to creating a basic unified validation result
	result := &core.ValidationResult{
		Valid:    true,
		Errors:   make([]*core.ValidationError, 0),
		Warnings: make([]*core.ValidationWarning, 0),
		Metadata: core.ValidationMetadata{
			ValidatedAt:      time.Now(),
			ValidatorName:    v.Name,
			ValidatorVersion: v.Version,
			Context:          make(map[string]interface{}),
		},
		Suggestions: make([]string, 0),
	}

	return result, nil
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

// ValidateUnified performs validation using unified validation framework
func (c *ValidatorChain) ValidateUnified(ctx context.Context, input interface{}, options *core.ValidationOptions) (*core.ValidationResult, error) {
	// Create combined result
	combinedResult := &core.ValidationResult{
		Valid:    true,
		Errors:   make([]*core.ValidationError, 0),
		Warnings: make([]*core.ValidationWarning, 0),
		Metadata: core.ValidationMetadata{
			ValidatedAt:      time.Now(),
			ValidatorName:    "runtime-validator-chain",
			ValidatorVersion: "1.0.0",
			Context:          make(map[string]interface{}),
		},
		Suggestions: make([]string, 0),
	}

	// Run each validator that supports unified validation
	for _, validator := range c.validators {
		if unifiedValidator, ok := validator.(interface {
			ValidateUnified(ctx context.Context, input interface{}, options *core.ValidationOptions) (*core.ValidationResult, error)
		}); ok {
			vResult, err := unifiedValidator.ValidateUnified(ctx, input, options)
			if err != nil {
				return nil, errors.Wrapf(err, "runtime/validator", "unified validator %s failed", validator.GetName())
			}
			combinedResult.Merge(vResult)
		} else {
			// Fallback to legacy validation and convert
			legacyOptions := ValidationOptions{
				Severity:     "medium",
				StrictMode:   options.StrictMode,
				IgnoreRules:  make([]string, 0),
				CustomParams: make(map[string]interface{}),
			}
			legacyResult, err := validator.Validate(ctx, input, legacyOptions)
			if err != nil {
				return nil, errors.Wrapf(err, "runtime/validator", "legacy validator %s failed", validator.GetName())
			}

			// Convert legacy result to unified format
			unifiedResult := convertLegacyToUnified(legacyResult)
			combinedResult.Merge(unifiedResult)
		}
	}

	// Calculate final score and duration
	combinedResult.Duration = time.Since(combinedResult.Metadata.ValidatedAt)

	return combinedResult, nil
}

func (c *ValidatorChain) GetName() string {
	return "ValidatorChain"
}

// convertLegacyToUnified converts legacy ValidationResult to unified ValidationResult
func convertLegacyToUnified(legacy *ValidationResult) *core.ValidationResult {
	if legacy == nil {
		return &core.ValidationResult{
			Valid:       true,
			Errors:      make([]*core.ValidationError, 0),
			Warnings:    make([]*core.ValidationWarning, 0),
			Suggestions: make([]string, 0),
			Metadata: core.ValidationMetadata{
				ValidatedAt:      time.Now(),
				ValidatorName:    "legacy-converter",
				ValidatorVersion: "1.0.0",
				Context:          make(map[string]interface{}),
			},
		}
	}

	unified := &core.ValidationResult{
		Valid:       legacy.IsValid,
		Score:       float64(legacy.Score),
		Errors:      make([]*core.ValidationError, 0),
		Warnings:    make([]*core.ValidationWarning, 0),
		Suggestions: make([]string, 0),
		Metadata: core.ValidationMetadata{
			ValidatedAt:      legacy.Metadata.Timestamp,
			ValidatorName:    legacy.Metadata.ValidatorName,
			ValidatorVersion: legacy.Metadata.ValidatorVersion,
			Context:          legacy.Context,
		},
		Duration: legacy.Metadata.Duration,
	}

	// Convert errors
	for _, legacyErr := range legacy.Errors {
		unifiedErr := &core.ValidationError{
			Code:     legacyErr.Code,
			Message:  legacyErr.Message,
			Type:     convertErrorType(legacyErr.Type),
			Severity: convertSeverity(legacyErr.Severity),
			Field:    legacyErr.Location.Path,
		}
		unified.Errors = append(unified.Errors, unifiedErr)
	}

	// Convert warnings
	for _, legacyWarn := range legacy.Warnings {
		unifiedWarn := &core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     legacyWarn.Code,
				Message:  legacyWarn.Message,
				Type:     convertErrorType(legacyWarn.Type),
				Severity: core.SeverityMedium, // Default warnings to medium
				Field:    legacyWarn.Location.Path,
			},
		}
		unified.Warnings = append(unified.Warnings, unifiedWarn)

		if legacyWarn.Suggestion != "" {
			unified.Suggestions = append(unified.Suggestions, legacyWarn.Suggestion)
		}
	}

	// Set risk level based on errors
	if legacy.CriticalIssues > 0 {
		unified.RiskLevel = "high"
	} else if len(legacy.Errors) > 0 {
		unified.RiskLevel = "medium"
	} else {
		unified.RiskLevel = "low"
	}

	return unified
}

// convertErrorType converts legacy error type to unified error type
func convertErrorType(legacyType string) core.ErrorType {
	switch legacyType {
	case "validation":
		return core.ErrTypeValidation
	case "security":
		return core.ErrTypeSecurity
	case "performance":
		return core.ErrTypeSystem // Use system instead of performance
	case "system":
		return core.ErrTypeSystem
	default:
		return core.ErrTypeValidation
	}
}

// convertSeverity converts legacy severity to unified severity
func convertSeverity(legacySeverity string) core.ErrorSeverity {
	switch legacySeverity {
	case "critical":
		return core.SeverityCritical
	case "high":
		return core.SeverityHigh
	case "medium":
		return core.SeverityMedium
	case "low":
		return core.SeverityLow
	default:
		return core.SeverityMedium
	}
}

// RuntimeValidatorRegistry manages runtime validators with unified validation support
type RuntimeValidatorRegistry struct {
	validators      map[string]BaseValidator
	unifiedRegistry core.ValidatorRegistry
}

// NewRuntimeValidatorRegistry creates a new runtime validator registry
func NewRuntimeValidatorRegistry() *RuntimeValidatorRegistry {
	return &RuntimeValidatorRegistry{
		validators:      make(map[string]BaseValidator),
		unifiedRegistry: core.NewValidatorRegistry(),
	}
}

// RegisterValidator registers a runtime validator
func (r *RuntimeValidatorRegistry) RegisterValidator(name string, validator BaseValidator) {
	r.validators[name] = validator

	// If validator supports unified validation, register it with unified registry
	if unifiedValidator, ok := validator.(interface {
		ValidateUnified(ctx context.Context, input interface{}, options *core.ValidationOptions) (*core.ValidationResult, error)
	}); ok {
		// Create wrapper for unified registry
		wrapper := &runtimeValidatorWrapper{
			name:      name,
			validator: unifiedValidator,
		}
		r.unifiedRegistry.Register(name, wrapper) // Error ignored for brevity
	}
}

// GetValidator retrieves a runtime validator by name
func (r *RuntimeValidatorRegistry) GetValidator(name string) (BaseValidator, bool) {
	validator, exists := r.validators[name]
	return validator, exists
}

// GetUnifiedRegistry returns the unified validator registry
func (r *RuntimeValidatorRegistry) GetUnifiedRegistry() core.ValidatorRegistry {
	return r.unifiedRegistry
}

// ValidateWithRuntime performs validation using runtime validator
func (r *RuntimeValidatorRegistry) ValidateWithRuntime(ctx context.Context, validatorName string, input interface{}, options ValidationOptions) (*ValidationResult, error) {
	validator, exists := r.validators[validatorName]
	if !exists {
		return nil, errors.Newf("runtime/validator", errors.CategoryValidation, "validator %s not found", validatorName)
	}

	return validator.Validate(ctx, input, options)
}

// ValidateWithUnified performs validation using unified validation framework
func (r *RuntimeValidatorRegistry) ValidateWithUnified(ctx context.Context, validatorName string, input interface{}, options *core.ValidationOptions) (*core.ValidationResult, error) {
	// First try unified registry
	if unifiedValidator, exists := r.unifiedRegistry.Get(validatorName); exists {
		return unifiedValidator.Validate(ctx, input, options), nil
	}

	// Fallback to runtime validator if it supports unified validation
	runtimeValidator, exists := r.validators[validatorName]
	if !exists {
		return nil, errors.Newf("runtime/validator", errors.CategoryValidation, "validator %s not found", validatorName)
	}

	if unifiedValidator, ok := runtimeValidator.(interface {
		ValidateUnified(ctx context.Context, input interface{}, options *core.ValidationOptions) (*core.ValidationResult, error)
	}); ok {
		return unifiedValidator.ValidateUnified(ctx, input, options)
	}

	return nil, errors.Newf("runtime/validator", errors.CategoryValidation, "validator %s does not support unified validation", validatorName)
}

// runtimeValidatorWrapper wraps runtime validator for unified registry
type runtimeValidatorWrapper struct {
	name      string
	validator interface {
		ValidateUnified(ctx context.Context, input interface{}, options *core.ValidationOptions) (*core.ValidationResult, error)
	}
}

func (w *runtimeValidatorWrapper) Validate(ctx context.Context, input interface{}, options *core.ValidationOptions) *core.ValidationResult {
	result, err := w.validator.ValidateUnified(ctx, input, options)
	if err != nil {
		// Create error result
		return &core.ValidationResult{
			Valid: false,
			Errors: []*core.ValidationError{{
				Code:     "WRAPPER_ERROR",
				Message:  err.Error(),
				Type:     core.ErrTypeSystem,
				Severity: core.SeverityHigh,
			}},
			Warnings: make([]*core.ValidationWarning, 0),
			Metadata: core.ValidationMetadata{
				ValidatedAt:      time.Now(),
				ValidatorName:    w.name,
				ValidatorVersion: "1.0.0",
				Context:          make(map[string]interface{}),
			},
			Suggestions: make([]string, 0),
		}
	}
	return result
}

func (w *runtimeValidatorWrapper) GetName() string {
	return w.name
}

func (w *runtimeValidatorWrapper) GetVersion() string {
	return "1.0.0"
}

func (w *runtimeValidatorWrapper) GetSupportedTypes() []string {
	return []string{"runtime", "wrapper"}
}

// Global runtime validator registry
var DefaultRuntimeRegistry = NewRuntimeValidatorRegistry()

// RegisterRuntimeValidator registers a validator with the default runtime registry
func RegisterRuntimeValidator(name string, validator BaseValidator) {
	DefaultRuntimeRegistry.RegisterValidator(name, validator)
}

// GetRuntimeValidator retrieves a validator from the default runtime registry
func GetRuntimeValidator(name string) (BaseValidator, bool) {
	return DefaultRuntimeRegistry.GetValidator(name)
}

// ValidateRuntime performs runtime validation using the default registry
func ValidateRuntime(ctx context.Context, validatorName string, input interface{}, options ValidationOptions) (*ValidationResult, error) {
	return DefaultRuntimeRegistry.ValidateWithRuntime(ctx, validatorName, input, options)
}

// ValidateRuntimeUnified performs unified validation using the default registry
func ValidateRuntimeUnified(ctx context.Context, validatorName string, input interface{}, options *core.ValidationOptions) (*core.ValidationResult, error) {
	return DefaultRuntimeRegistry.ValidateWithUnified(ctx, validatorName, input, options)
}
