package validation

import (
	"context"

	"github.com/Azure/container-kit/pkg/common/interfaces"
	"github.com/Azure/container-kit/pkg/mcp/application/api"
	domainvalidation "github.com/Azure/container-kit/pkg/mcp/domain/validation"
)

// validatorRegistryAdapter adapts domain validation registry to api interface
type validatorRegistryAdapter struct {
	domainRegistry domainvalidation.ValidatorRegistry
}

// Register adapts domain validator to api interface
func (a *validatorRegistryAdapter) Register(validator api.DomainValidator[interface{}]) error {
	// Create adapter for domain validator
	domainValidator := &domainValidatorAdapter{apiValidator: validator}
	return a.domainRegistry.Register(domainValidator)
}

// Unregister forwards to domain registry
func (a *validatorRegistryAdapter) Unregister(name string) error {
	return a.domainRegistry.Unregister(name)
}

// GetValidators converts domain validators to api interface
func (a *validatorRegistryAdapter) GetValidators(domain, category string) []api.DomainValidator[interface{}] {
	domainValidators := a.domainRegistry.GetValidators(domain, category)
	result := make([]api.DomainValidator[interface{}], len(domainValidators))
	for i, v := range domainValidators {
		result[i] = &apiValidatorAdapter{domainValidator: v}
	}
	return result
}

// GetDomainValidators converts domain validators to api interface
func (a *validatorRegistryAdapter) GetDomainValidators(domain string) []api.DomainValidator[interface{}] {
	domainValidators := a.domainRegistry.GetDomainValidators(domain)
	result := make([]api.DomainValidator[interface{}], len(domainValidators))
	for i, v := range domainValidators {
		result[i] = &apiValidatorAdapter{domainValidator: v}
	}
	return result
}

// ValidateAll converts result types
func (a *validatorRegistryAdapter) ValidateAll(ctx context.Context, data interface{}, domain, category string) api.ValidationResult {
	domainResult := a.domainRegistry.ValidateAll(ctx, data, domain, category)
	return api.ValidationResult{
		Valid:    domainResult.Valid,
		Errors:   domainResult.Errors,
		Warnings: domainResult.Warnings,
		Context:  api.ValidationContext(domainResult.Context),
	}
}

// ListValidators converts validator info
func (a *validatorRegistryAdapter) ListValidators() []api.ValidatorInfo {
	domainInfos := a.domainRegistry.ListValidators()
	result := make([]api.ValidatorInfo, len(domainInfos))
	for i, info := range domainInfos {
		result[i] = api.ValidatorInfo{
			Name:         info.Name,
			Domain:       info.Domain,
			Category:     info.Category,
			Priority:     info.Priority,
			Dependencies: info.Dependencies,
		}
	}
	return result
}

// domainValidatorAdapter adapts api validator to domain interface
type domainValidatorAdapter struct {
	apiValidator api.DomainValidator[interface{}]
}

func (d *domainValidatorAdapter) Validate(ctx context.Context, value interface{}) domainvalidation.ValidationResult {
	apiResult := d.apiValidator.Validate(ctx, value)
	return domainvalidation.ValidationResult{
		Valid:    apiResult.Valid,
		Errors:   apiResult.Errors,
		Warnings: apiResult.Warnings,
		Context:  domainvalidation.ValidationContext(apiResult.Context),
	}
}

func (d *domainValidatorAdapter) Name() string           { return d.apiValidator.Name() }
func (d *domainValidatorAdapter) Domain() string         { return d.apiValidator.Domain() }
func (d *domainValidatorAdapter) Category() string       { return d.apiValidator.Category() }
func (d *domainValidatorAdapter) Priority() int          { return d.apiValidator.Priority() }
func (d *domainValidatorAdapter) Dependencies() []string { return d.apiValidator.Dependencies() }

// apiValidatorAdapter adapts domain validator to api interface
type apiValidatorAdapter struct {
	domainValidator domainvalidation.DomainValidator[interface{}]
}

func (a *apiValidatorAdapter) Validate(ctx context.Context, value interface{}) api.ValidationResult {
	domainResult := a.domainValidator.Validate(ctx, value)
	return api.ValidationResult{
		Valid:    domainResult.Valid,
		Errors:   domainResult.Errors,
		Warnings: domainResult.Warnings,
		Context:  api.ValidationContext(domainResult.Context),
	}
}

func (a *apiValidatorAdapter) Name() string           { return a.domainValidator.Name() }
func (a *apiValidatorAdapter) Domain() string         { return a.domainValidator.Domain() }
func (a *apiValidatorAdapter) Category() string       { return a.domainValidator.Category() }
func (a *apiValidatorAdapter) Priority() int          { return a.domainValidator.Priority() }
func (a *apiValidatorAdapter) Dependencies() []string { return a.domainValidator.Dependencies() }

// UnifiedValidator provides a unified validation interface
type UnifiedValidator struct {
	registry     api.ValidatorRegistry
	capabilities []string
}

// NewUnifiedValidator creates a new unified validator
func NewUnifiedValidator(capabilities []string) *UnifiedValidator {
	domainRegistry := domainvalidation.NewValidatorRegistry()
	return &UnifiedValidator{
		registry:     &validatorRegistryAdapter{domainRegistry: domainRegistry},
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
