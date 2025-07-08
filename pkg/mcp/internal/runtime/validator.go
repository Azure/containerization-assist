package runtime

import (
	"context"
	"time"

	"github.com/Azure/container-kit/pkg/common/validation-core/core"
	"github.com/Azure/container-kit/pkg/common/validation-core/validators"
	"github.com/Azure/container-kit/pkg/mcp/errors"
)

type BaseValidator interface {
	Validate(ctx context.Context, input interface{}, options ValidationOptions) (*RuntimeValidationResult, error)
	ValidateUnified(ctx context.Context, input interface{}, options *core.ValidationOptions) (*core.NonGenericResult, error)

	GetName() string
}
type ValidationOptions = core.ValidationOptions
type RuntimeValidationResult = core.RuntimeResult
type RuntimeValidationData = core.RuntimeValidationData
type ValidationError = core.Error
type ValidationWarning = core.Warning

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
	Name             string
	Version          string
	unifiedValidator core.Validator
}

func NewBaseValidator(name, version string) *BaseValidatorImpl {
	return &BaseValidatorImpl{
		Name:    name,
		Version: version,

		unifiedValidator: validators.NewBaseValidator(name, version, []string{"runtime"}),
	}
}
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
func (v *BaseValidatorImpl) ValidateUnified(ctx context.Context, input interface{}, options *core.ValidationOptions) (*core.NonGenericResult, error) {
	if v.unifiedValidator != nil {
		return v.unifiedValidator.Validate(ctx, input, options), nil
	}
	result := &core.NonGenericResult{
		Valid:    true,
		Errors:   make([]*core.Error, 0),
		Warnings: make([]*core.Warning, 0),
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

func (v *BaseValidatorImpl) CreateResult() *RuntimeValidationResult {
	return &RuntimeValidationResult{
		Valid: true,
		Score: 100,
		Data: core.RuntimeValidationData{
			ToolName: v.Name,
		},
		Metadata: core.ValidationMetadata{
			ValidatedAt:      time.Now(),
			ValidatorName:    v.Name,
			ValidatorVersion: v.Version,
		},
		Timestamp: time.Now(),
		Errors:    make([]*core.Error, 0),
		Warnings:  make([]*core.Warning, 0),
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
		Errors:   make([]*core.Error, 0),
		Warnings: make([]*core.Warning, 0),
		Data:     core.RuntimeValidationData{},
		Metadata: core.ValidationMetadata{
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
func (c *ValidatorChain) ValidateUnified(ctx context.Context, input interface{}, options *core.ValidationOptions) (*core.NonGenericResult, error) {

	combinedResult := &core.NonGenericResult{
		Valid:    true,
		Errors:   make([]*core.Error, 0),
		Warnings: make([]*core.Warning, 0),
		Metadata: core.ValidationMetadata{
			ValidatedAt:      time.Now(),
			ValidatorName:    "runtime-validator-chain",
			ValidatorVersion: "1.0.0",
			Context:          make(map[string]interface{}),
		},
		Suggestions: make([]string, 0),
	}
	for _, validator := range c.validators {
		if unifiedValidator, ok := validator.(interface {
			ValidateUnified(ctx context.Context, input interface{}, options *core.ValidationOptions) (*core.NonGenericResult, error)
		}); ok {
			vResult, err := unifiedValidator.ValidateUnified(ctx, input, options)
			if err != nil {
				return nil, errors.Wrapf(err, "runtime/validator", "unified validator %s failed", validator.GetName())
			}
			combinedResult.Merge(vResult)
		} else {

			legacyResult, err := validator.Validate(ctx, input, *options)
			if err != nil {
				return nil, errors.Wrapf(err, "runtime/validator", "legacy validator %s failed", validator.GetName())
			}
			unifiedResult := convertLegacyToUnified(legacyResult)
			combinedResult.Merge(unifiedResult)
		}
	}
	combinedResult.Duration = time.Since(combinedResult.Metadata.ValidatedAt)

	return combinedResult, nil
}

func (c *ValidatorChain) GetName() string {
	return "ValidatorChain"
}
func convertLegacyToUnified(legacy *RuntimeValidationResult) *core.NonGenericResult {
	if legacy == nil {
		return &core.NonGenericResult{
			Valid:       true,
			Errors:      make([]*core.Error, 0),
			Warnings:    make([]*core.Warning, 0),
			Suggestions: make([]string, 0),
			Metadata: core.ValidationMetadata{
				ValidatedAt:      time.Now(),
				ValidatorName:    "legacy-converter",
				ValidatorVersion: "1.0.0",
				Context:          make(map[string]interface{}),
			},
		}
	}

	unified := &core.NonGenericResult{
		Valid:       legacy.Valid,
		Score:       float64(legacy.Score),
		Errors:      make([]*core.Error, 0),
		Warnings:    make([]*core.Warning, 0),
		Suggestions: make([]string, 0),
		Metadata: core.ValidationMetadata{
			ValidatedAt:      legacy.Metadata.ValidatedAt,
			ValidatorName:    legacy.Metadata.ValidatorName,
			ValidatorVersion: legacy.Metadata.ValidatorVersion,
			Context:          make(map[string]interface{}),
		},
		Duration: legacy.Duration,
	}
	for _, legacyErr := range legacy.Errors {
		unified.Errors = append(unified.Errors, legacyErr)
	}
	for _, legacyWarn := range legacy.Warnings {
		unified.Warnings = append(unified.Warnings, legacyWarn)
		if len(legacyWarn.Error.Suggestions) > 0 {
			unified.Suggestions = append(unified.Suggestions, legacyWarn.Error.Suggestions...)
		}
	}
	if len(legacy.Errors) > 0 {

		hasCritical := false
		for _, err := range legacy.Errors {
			if err.Severity == core.SeverityHigh || err.Severity == core.SeverityCritical {
				hasCritical = true
				break
			}
		}
		if hasCritical {
			unified.RiskLevel = "high"
		} else {
			unified.RiskLevel = "medium"
		}
	} else {
		unified.RiskLevel = "low"
	}

	return unified
}

type RuntimeValidatorRegistry struct {
	validators      map[string]BaseValidator
	unifiedRegistry core.ValidatorRegistry
}

func NewRuntimeValidatorRegistry() *RuntimeValidatorRegistry {
	return &RuntimeValidatorRegistry{
		validators:      make(map[string]BaseValidator),
		unifiedRegistry: core.NewValidatorRegistry(),
	}
}
func (r *RuntimeValidatorRegistry) RegisterValidator(name string, validator BaseValidator) {
	r.validators[name] = validator
	if unifiedValidator, ok := validator.(interface {
		ValidateUnified(ctx context.Context, input interface{}, options *core.ValidationOptions) (*core.NonGenericResult, error)
	}); ok {

		wrapper := &runtimeValidatorWrapper{
			name:      name,
			validator: unifiedValidator,
		}
		r.unifiedRegistry.Register(name, wrapper)
	}
}
func (r *RuntimeValidatorRegistry) GetValidator(name string) (BaseValidator, bool) {
	validator, exists := r.validators[name]
	return validator, exists
}
func (r *RuntimeValidatorRegistry) GetUnifiedRegistry() core.ValidatorRegistry {
	return r.unifiedRegistry
}
func (r *RuntimeValidatorRegistry) ValidateWithRuntime(ctx context.Context, validatorName string, input interface{}, options ValidationOptions) (*RuntimeValidationResult, error) {
	validator, exists := r.validators[validatorName]
	if !exists {
		return nil, errors.Newf("runtime/validator", errors.CategoryValidation, "validator %s not found", validatorName)
	}

	return validator.Validate(ctx, input, options)
}
func (r *RuntimeValidatorRegistry) ValidateWithUnified(ctx context.Context, validatorName string, input interface{}, options *core.ValidationOptions) (*core.NonGenericResult, error) {

	if unifiedValidator, exists := r.unifiedRegistry.Get(validatorName); exists {
		return unifiedValidator.Validate(ctx, input, options), nil
	}
	runtimeValidator, exists := r.validators[validatorName]
	if !exists {
		return nil, errors.Newf("runtime/validator", errors.CategoryValidation, "validator %s not found", validatorName)
	}

	if unifiedValidator, ok := runtimeValidator.(interface {
		ValidateUnified(ctx context.Context, input interface{}, options *core.ValidationOptions) (*core.NonGenericResult, error)
	}); ok {
		return unifiedValidator.ValidateUnified(ctx, input, options)
	}

	return nil, errors.Newf("runtime/validator", errors.CategoryValidation, "validator %s does not support unified validation", validatorName)
}

type runtimeValidatorWrapper struct {
	name      string
	validator interface {
		ValidateUnified(ctx context.Context, input interface{}, options *core.ValidationOptions) (*core.NonGenericResult, error)
	}
}

func (w *runtimeValidatorWrapper) Validate(ctx context.Context, input interface{}, options *core.ValidationOptions) *core.NonGenericResult {
	result, err := w.validator.ValidateUnified(ctx, input, options)
	if err != nil {

		return &core.NonGenericResult{
			Valid: false,
			Errors: []*core.Error{{
				Code:     "WRAPPER_ERROR",
				Message:  err.Error(),
				Type:     core.ErrTypeSystem,
				Severity: core.SeverityHigh,
			}},
			Warnings: make([]*core.Warning, 0),
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

// DefaultRuntimeRegistry is deprecated - use ValidatorService instead
// var DefaultRuntimeRegistry = NewRuntimeValidatorRegistry() // REMOVED: Global state eliminated

// Deprecated: Use ValidatorService.RegisterValidator instead
func RegisterRuntimeValidator(name string, validator BaseValidator) {
	// NOTE: This function is deprecated. Use ValidatorService for validator management without global state.
	panic("RegisterRuntimeValidator is deprecated - use ValidatorService.RegisterValidator instead")
}

// Deprecated: Use ValidatorService.GetValidator instead
func GetRuntimeValidator(name string) (BaseValidator, bool) {
	// NOTE: This function is deprecated. Use ValidatorService for validator management without global state.
	panic("GetRuntimeValidator is deprecated - use ValidatorService.GetValidator instead")
}

// Deprecated: Use ValidatorService.ValidateWithRuntime instead
func ValidateRuntime(ctx context.Context, validatorName string, input interface{}, options ValidationOptions) (*RuntimeValidationResult, error) {
	// NOTE: This function is deprecated. Use ValidatorService for validator management without global state.
	panic("ValidateRuntime is deprecated - use ValidatorService.ValidateWithRuntime instead")
}

// Deprecated: Use ValidatorService.ValidateWithUnified instead
func ValidateRuntimeUnified(ctx context.Context, validatorName string, input interface{}, options *core.ValidationOptions) (*core.NonGenericResult, error) {
	// NOTE: This function is deprecated. Use ValidatorService for validator management without global state.
	panic("ValidateRuntimeUnified is deprecated - use ValidatorService.ValidateWithUnified instead")
}
