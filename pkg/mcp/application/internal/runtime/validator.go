package runtime

import (
	"context"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/security"
)

type BaseValidator interface {
	Validate(ctx context.Context, input interface{}, options ValidationOptions) (*RuntimeValidationResult, error)
	ValidateUnified(ctx context.Context, input interface{}, options *security.Options) (*security.Result, error)

	GetName() string
}

// ValidationOptions represents validation configuration
type ValidationOptions struct {
	StrictMode      bool                   `json:"strict_mode"`
	MaxErrors       int                    `json:"max_errors"`
	SkipFields      []string               `json:"skip_fields,omitempty"`
	IncludeWarnings bool                   `json:"include_warnings"`
	Context         map[string]interface{} `json:"context,omitempty"`
	Timeout         time.Duration          `json:"timeout,omitempty"`
	FailFast        bool                   `json:"fail_fast"`
}

// RuntimeValidationResult represents the result of runtime validation
type RuntimeValidationResult struct {
	Valid       bool                  `json:"valid"`
	Score       float64               `json:"score"`
	Errors      []security.Error      `json:"errors,omitempty"`
	Warnings    []security.Warning    `json:"warnings,omitempty"`
	Data        RuntimeValidationData `json:"data"`
	Metadata    ValidationMetadata    `json:"metadata"`
	Timestamp   time.Time             `json:"timestamp"`
	Suggestions []string              `json:"suggestions,omitempty"`
}

// RuntimeValidationData represents data specific to runtime validation
type RuntimeValidationData struct {
	ToolName   string                 `json:"tool_name"`
	SessionID  string                 `json:"session_id,omitempty"`
	WorkingDir string                 `json:"working_dir,omitempty"`
	Context    map[string]interface{} `json:"context,omitempty"`
}

type ValidationError = security.Error
type ValidationWarning = security.Warning

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
	ValidatedAt      time.Time
	ValidatorName    string
	ValidatorVersion string
	Duration         time.Duration
	Timestamp        time.Time
	Parameters       map[string]interface{}
}
type BaseValidatorImpl struct {
	Name              string
	Version           string
	standardValidator security.Validator
}

func NewBaseValidator(name, version string) *BaseValidatorImpl {
	return &BaseValidatorImpl{
		Name:              name,
		Version:           version,
		standardValidator: nil, // TODO: Create appropriate security validator if needed
	}
}

func NewBaseValidatorWithStandard(name, version string, standard security.Validator) *BaseValidatorImpl {
	return &BaseValidatorImpl{
		Name:              name,
		Version:           version,
		standardValidator: standard,
	}
}

func (v *BaseValidatorImpl) GetName() string {
	return v.Name
}
func (v *BaseValidatorImpl) ValidateUnified(ctx context.Context, input interface{}, options *security.Options) (*security.Result, error) {
	if v.standardValidator != nil {
		result := v.standardValidator.ValidateWithOptions(ctx, input, *options)
		return &result, nil
	}

	result := security.NewSessionResult(v.Name, v.Version)
	return result, nil
}

func (v *BaseValidatorImpl) CreateResult() *RuntimeValidationResult {
	return &RuntimeValidationResult{
		Valid: true,
		Score: 100,
		Data: RuntimeValidationData{
			ToolName: v.Name,
		},
		Metadata: ValidationMetadata{
			ValidatedAt:      time.Now(),
			ValidatorName:    v.Name,
			ValidatorVersion: v.Version,
		},
		Timestamp: time.Now(),
		Errors:    make([]security.Error, 0),
		Warnings:  make([]security.Warning, 0),
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

func (c *ValidatorChain) Validate(ctx context.Context, input interface{}, options ValidationOptions) (*RuntimeValidationResult, error) {
	result := &RuntimeValidationResult{
		Valid:    true,
		Errors:   make([]security.Error, 0),
		Warnings: make([]security.Warning, 0),
		Data:     RuntimeValidationData{},
		Metadata: ValidationMetadata{
			ValidatedAt:      time.Now(),
			ValidatorName:    "validator-chain",
			ValidatorVersion: "1.0.0",
		},
		Timestamp: time.Now(),
	}

	for _, validator := range c.validators {
		vResult, err := validator.Validate(ctx, input, options)
		if err != nil {
			return nil, errors.Wrapf(err, "runtime/validator", "validator %s failed", validator.GetName())
		}
		result.Valid = result.Valid && vResult.Valid
		result.Errors = append(result.Errors, vResult.Errors...)
		result.Warnings = append(result.Warnings, vResult.Warnings...)
		if vResult.Score < result.Score {
			result.Score = vResult.Score
		}
	}
	if len(result.Errors) > 0 {
		result.Score = float64(100 - (len(result.Errors) * 10) - (len(result.Warnings) * 2))
		if result.Score < 0 {
			result.Score = 0
		}
	}

	return result, nil
}
func (c *ValidatorChain) ValidateUnified(ctx context.Context, input interface{}, options *security.Options) (*security.Result, error) {
	combinedResult := security.NewSessionResult("runtime-validator-chain", "1.0.0")
	for _, validator := range c.validators {
		if unifiedValidator, ok := validator.(interface {
			ValidateUnified(ctx context.Context, input interface{}, options *security.Options) (*security.Result, error)
		}); ok {
			vResult, err := unifiedValidator.ValidateUnified(ctx, input, options)
			if err != nil {
				return nil, errors.Wrapf(err, "runtime/validator", "unified validator %s failed", validator.GetName())
			}
			// Merge results (simplified - security.Result doesn't have Merge method)
			combinedResult.Valid = combinedResult.Valid && vResult.Valid
			combinedResult.Errors = append(combinedResult.Errors, vResult.Errors...)
			combinedResult.Warnings = append(combinedResult.Warnings, vResult.Warnings...)
		} else {
			// Convert security.Options to ValidationOptions for legacy compatibility
			context := make(map[string]interface{})
			for k, v := range options.Context {
				context[k] = v
			}
			legacyOptions := ValidationOptions{
				StrictMode:      options.StrictMode,
				MaxErrors:       options.MaxErrors,
				SkipFields:      options.SkipFields,
				IncludeWarnings: options.IncludeWarnings,
				Context:         context,
				Timeout:         options.Timeout,
				FailFast:        options.FailFast,
			}
			legacyResult, err := validator.Validate(ctx, input, legacyOptions)
			if err != nil {
				return nil, errors.Wrapf(err, "runtime/validator", "legacy validator %s failed", validator.GetName())
			}
			// Convert legacy result to security result
			combinedResult.Valid = combinedResult.Valid && legacyResult.Valid
			combinedResult.Errors = append(combinedResult.Errors, legacyResult.Errors...)
			combinedResult.Warnings = append(combinedResult.Warnings, legacyResult.Warnings...)
		}
	}
	combinedResult.Duration = time.Since(combinedResult.Metadata.ValidatedAt)

	return combinedResult, nil
}

func (c *ValidatorChain) GetName() string {
	return "ValidatorChain"
}

// convertLegacyToStandard function removed - no longer needed with new validation system

type RuntimeValidatorRegistry struct {
	validators map[string]BaseValidator
	// unifiedRegistry removed - no longer needed with new validation system
}

func NewRuntimeValidatorRegistry() *RuntimeValidatorRegistry {
	return &RuntimeValidatorRegistry{
		validators: make(map[string]BaseValidator),
	}
}
func (r *RuntimeValidatorRegistry) RegisterValidator(name string, validator BaseValidator) {
	r.validators[name] = validator
	// Unified registry integration removed - no longer needed with new validation system
}
func (r *RuntimeValidatorRegistry) GetValidator(name string) (BaseValidator, bool) {
	validator, exists := r.validators[name]
	return validator, exists
}

// GetUnifiedRegistry removed - no longer needed with new validation system

func (r *RuntimeValidatorRegistry) ValidateWithRuntime(ctx context.Context, validatorName string, input interface{}, options ValidationOptions) (*RuntimeValidationResult, error) {
	validator, exists := r.validators[validatorName]
	if !exists {
		return nil, errors.Newf("runtime/validator", errors.CategoryValidation, "validator %s not found", validatorName)
	}

	return validator.Validate(ctx, input, options)
}

func (r *RuntimeValidatorRegistry) ValidateWithUnified(ctx context.Context, validatorName string, input interface{}, options *security.Options) (*security.Result, error) {
	runtimeValidator, exists := r.validators[validatorName]
	if !exists {
		return nil, errors.Newf("runtime/validator", errors.CategoryValidation, "validator %s not found", validatorName)
	}

	if unifiedValidator, ok := runtimeValidator.(interface {
		ValidateUnified(ctx context.Context, input interface{}, options *security.Options) (*security.Result, error)
	}); ok {
		return unifiedValidator.ValidateUnified(ctx, input, options)
	}

	return nil, errors.Newf("runtime/validator", errors.CategoryValidation, "validator %s does not support unified validation", validatorName)
}

// runtimeValidatorWrapper removed - no longer needed with new validation system

// Legacy validator functions have been removed. Use ValidatorService instead.
