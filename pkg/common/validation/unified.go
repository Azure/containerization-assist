package validation

import (
	"context"

	"github.com/Azure/container-kit/pkg/common/interfaces"
	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/validation"
)

// UnifiedValidator provides a unified validation interface
type UnifiedValidator struct {
	registry     validation.ValidatorRegistry
	capabilities []string
}

// NewUnifiedValidator creates a new unified validator
func NewUnifiedValidator(capabilities []string) *UnifiedValidator {
	return &UnifiedValidator{
		registry:     validation.NewValidatorRegistry(),
		capabilities: capabilities,
	}
}

// ValidateInput validates tool input
func (u *UnifiedValidator) ValidateInput(ctx context.Context, _ string, input api.ToolInput) error {
	result := u.registry.ValidateAll(ctx, input, "tool", "input")
	if !result.Valid && len(result.Errors) > 0 {
		return result.Errors[0]
	}
	return nil
}

// ValidateOutput validates tool output
func (u *UnifiedValidator) ValidateOutput(ctx context.Context, _ string, output api.ToolOutput) error {
	result := u.registry.ValidateAll(ctx, output, "tool", "output")
	if !result.Valid && len(result.Errors) > 0 {
		return result.Errors[0]
	}
	return nil
}

// ValidateConfig validates configuration
func (u *UnifiedValidator) ValidateConfig(ctx context.Context, config interface{}) error {
	result := u.registry.ValidateAll(ctx, config, "config", "validation")
	if !result.Valid && len(result.Errors) > 0 {
		return result.Errors[0]
	}
	return nil
}

// ValidateSchema validates schema
func (u *UnifiedValidator) ValidateSchema(ctx context.Context, _ interface{}, data interface{}) error {
	result := u.registry.ValidateAll(ctx, data, "schema", "validation")
	if !result.Valid && len(result.Errors) > 0 {
		return result.Errors[0]
	}
	return nil
}

// ValidateHealth validates health
func (u *UnifiedValidator) ValidateHealth(_ context.Context) []interfaces.ValidationResult {
	return []interfaces.ValidationResult{
		{
			Valid:    true,
			Message:  "Validator is healthy",
			Severity: "info",
		},
	}
}

// GetCapabilities returns validator capabilities
func (u *UnifiedValidator) GetCapabilities() []string {
	return u.capabilities
}
