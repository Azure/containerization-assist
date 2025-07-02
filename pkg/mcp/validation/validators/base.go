package validators

import (
	"context"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/validation/core"
)

// BaseValidatorImpl provides a base implementation of the Validator interface
type BaseValidatorImpl struct {
	Name              string                  // Validator name (exported for direct access)
	Version           string                  // Validator version (exported for direct access)
	SupportedTypes    []string                // Types this validator can handle (exported for direct access)
	ValidationContext *core.ValidationContext // Validation context (exported for direct access)
	config            map[string]interface{}  // Internal configuration (kept private)
}

// NewBaseValidator creates a new base validator
func NewBaseValidator(name, version string, supportedTypes []string) *BaseValidatorImpl {
	return &BaseValidatorImpl{
		Name:           name,
		Version:        version,
		SupportedTypes: supportedTypes,
		config:         make(map[string]interface{}),
	}
}

// GetSupportedTypes returns a copy of the supported types for safety
// Note: For performance-critical code, access SupportedTypes field directly
func (b *BaseValidatorImpl) GetSupportedTypes() []string {
	result := make([]string, len(b.SupportedTypes))
	copy(result, b.SupportedTypes)
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
			ValidatorName:    b.Name,
			ValidatorVersion: b.Version,
			RulesApplied:     []string{},
			Context:          make(map[string]interface{}),
		},
		Suggestions: make([]string, 0),
	}

	// Add validation context if available
	if b.ValidationContext != nil {
		result.Metadata.Context["session_id"] = b.ValidationContext.SessionID
		result.Metadata.Context["tool"] = b.ValidationContext.Tool
		result.Metadata.Context["operation"] = b.ValidationContext.Operation
	}

	result.Duration = time.Since(startTime)
	return result
}

// Interface compliance methods (required by core.Validator interface)
// Note: Direct field access is preferred when not constrained by interfaces

// GetName returns the validator name (required by core.Validator interface)
func (b *BaseValidatorImpl) GetName() string {
	return b.Name
}

// GetVersion returns the validator version (required by core.Validator interface)
func (b *BaseValidatorImpl) GetVersion() string {
	return b.Version
}

// Note: ValidationContext field is now exported for direct access
// Use validator.ValidationContext = ctx instead of SetContext(ctx) when not constrained by interfaces

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
