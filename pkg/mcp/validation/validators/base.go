package validators

import (
	"context"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/validation/core"
)

// BaseValidatorImpl provides a base implementation of the Validator interface
type BaseValidatorImpl struct {
	name              string
	version           string
	supportedTypes    []string
	config            map[string]interface{}
	validationContext *core.ValidationContext
}

// NewBaseValidator creates a new base validator
func NewBaseValidator(name, version string, supportedTypes []string) *BaseValidatorImpl {
	return &BaseValidatorImpl{
		name:           name,
		version:        version,
		supportedTypes: supportedTypes,
		config:         make(map[string]interface{}),
	}
}

// GetName returns the name of the validator
func (b *BaseValidatorImpl) GetName() string {
	return b.name
}

// GetVersion returns the version of the validator
func (b *BaseValidatorImpl) GetVersion() string {
	return b.version
}

// GetSupportedTypes returns the types this validator can handle
func (b *BaseValidatorImpl) GetSupportedTypes() []string {
	result := make([]string, len(b.supportedTypes))
	copy(result, b.supportedTypes)
	return result
}

// Validate provides a default implementation that should be overridden
func (b *BaseValidatorImpl) Validate(ctx context.Context, data interface{}, options *core.ValidationOptions) *core.ValidationResult {
	startTime := time.Now()

	result := &core.ValidationResult{
		Valid:    true,
		Errors:   make([]*core.ValidationError, 0),
		Warnings: make([]*core.ValidationWarning, 0),
		Metadata: core.ValidationMetadata{
			ValidatedAt:      startTime,
			ValidatorName:    b.name,
			ValidatorVersion: b.version,
			RulesApplied:     []string{},
			Context:          make(map[string]interface{}),
		},
		Suggestions: make([]string, 0),
	}

	// Add validation context if available
	if b.validationContext != nil {
		result.Metadata.Context["session_id"] = b.validationContext.SessionID
		result.Metadata.Context["tool"] = b.validationContext.Tool
		result.Metadata.Context["operation"] = b.validationContext.Operation
	}

	result.Duration = time.Since(startTime)
	return result
}

// SetContext sets the validation context (implements ContextAware)
func (b *BaseValidatorImpl) SetContext(ctx *core.ValidationContext) {
	b.validationContext = ctx
}

// GetContext returns the validation context (implements ContextAware)
func (b *BaseValidatorImpl) GetContext() *core.ValidationContext {
	return b.validationContext
}

// Configure configures the validator (implements Configurable)
func (b *BaseValidatorImpl) Configure(config map[string]interface{}) error {
	if b.config == nil {
		b.config = make(map[string]interface{})
	}

	for key, value := range config {
		b.config[key] = value
	}

	return nil
}

// GetConfiguration returns the current configuration (implements Configurable)
func (b *BaseValidatorImpl) GetConfiguration() map[string]interface{} {
	result := make(map[string]interface{})
	for key, value := range b.config {
		result[key] = value
	}
	return result
}

// NoOpValidator is a validator that does nothing (useful for testing and placeholders)
type NoOpValidator struct {
	*BaseValidatorImpl
}

// NewNoOpValidator creates a new no-op validator
func NewNoOpValidator() *NoOpValidator {
	return &NoOpValidator{
		BaseValidatorImpl: NewBaseValidator("noop", "1.0.0", []string{"*"}),
	}
}

// AlwaysPassValidator always returns a successful validation result
type AlwaysPassValidator struct {
	*BaseValidatorImpl
}

// NewAlwaysPassValidator creates a validator that always passes
func NewAlwaysPassValidator() *AlwaysPassValidator {
	return &AlwaysPassValidator{
		BaseValidatorImpl: NewBaseValidator("always-pass", "1.0.0", []string{"*"}),
	}
}

// AlwaysFailValidator always returns a failed validation result
type AlwaysFailValidator struct {
	*BaseValidatorImpl
	errorMessage string
}

// NewAlwaysFailValidator creates a validator that always fails
func NewAlwaysFailValidator(errorMessage string) *AlwaysFailValidator {
	if errorMessage == "" {
		errorMessage = "Validation always fails"
	}

	return &AlwaysFailValidator{
		BaseValidatorImpl: NewBaseValidator("always-fail", "1.0.0", []string{"*"}),
		errorMessage:      errorMessage,
	}
}

// Validate always returns a failed result
func (a *AlwaysFailValidator) Validate(ctx context.Context, data interface{}, options *core.ValidationOptions) *core.ValidationResult {
	result := a.BaseValidatorImpl.Validate(ctx, data, options)

	error := core.NewValidationError(
		"ALWAYS_FAIL",
		a.errorMessage,
		core.ErrTypeValidation,
		core.SeverityMedium,
	)

	result.AddError(error)
	return result
}
