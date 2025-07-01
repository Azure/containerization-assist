package chains

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/validation/core"
)

// CompositeValidator implements ValidatorChain
type CompositeValidator struct {
	name       string
	version    string
	validators []core.Validator
}

// NewCompositeValidator creates a new composite validator
func NewCompositeValidator(name, version string) core.ValidatorChain {
	return &CompositeValidator{
		name:       name,
		version:    version,
		validators: make([]core.Validator, 0),
	}
}

// Validate executes all validators in the chain
func (c *CompositeValidator) Validate(ctx context.Context, data interface{}, options *core.ValidationOptions) *core.ValidationResult {
	startTime := time.Now()

	result := &core.ValidationResult{
		Valid:    true,
		Errors:   make([]*core.ValidationError, 0),
		Warnings: make([]*core.ValidationWarning, 0),
		Metadata: core.ValidationMetadata{
			ValidatedAt:      startTime,
			ValidatorName:    c.name,
			ValidatorVersion: c.version,
			RulesApplied:     make([]string, 0),
		},
		Suggestions: make([]string, 0),
	}

	// Execute each validator in sequence
	for _, validator := range c.validators {
		// Check if we should fail fast
		if options.FailFast && result.HasErrors() {
			break
		}

		// Check if we've hit the max error limit
		if options.MaxErrors > 0 && len(result.Errors) >= options.MaxErrors {
			break
		}

		// Check for context cancellation
		select {
		case <-ctx.Done():
			result.AddError(core.NewValidationError(
				"VALIDATION_CANCELLED",
				"Validation was cancelled",
				core.ErrTypeSystem,
				core.SeverityMedium,
			))
			break
		default:
		}

		// Execute the validator
		validatorResult := validator.Validate(ctx, data, options)
		if validatorResult != nil {
			result.Merge(validatorResult)

			// Add validator name to rules applied
			validatorName := validator.GetName()
			if validatorName != "" {
				result.Metadata.RulesApplied = append(result.Metadata.RulesApplied, validatorName)
			}
		}
	}

	result.Duration = time.Since(startTime)
	return result
}

// GetName returns the name of the composite validator
func (c *CompositeValidator) GetName() string {
	return c.name
}

// GetVersion returns the version of the composite validator
func (c *CompositeValidator) GetVersion() string {
	return c.version
}

// GetSupportedTypes returns all types supported by validators in the chain
func (c *CompositeValidator) GetSupportedTypes() []string {
	typeSet := make(map[string]bool)
	for _, validator := range c.validators {
		for _, t := range validator.GetSupportedTypes() {
			typeSet[t] = true
		}
	}

	types := make([]string, 0, len(typeSet))
	for t := range typeSet {
		types = append(types, t)
	}
	return types
}

// Add adds a validator to the chain
func (c *CompositeValidator) Add(validator core.Validator) core.ValidatorChain {
	if validator != nil {
		c.validators = append(c.validators, validator)
	}
	return c
}

// AddConditional adds a conditional validator to the chain
func (c *CompositeValidator) AddConditional(validator core.ConditionalValidator) core.ValidatorChain {
	if validator != nil {
		c.validators = append(c.validators, &conditionalValidatorWrapper{validator})
	}
	return c
}

// GetValidators returns all validators in the chain
func (c *CompositeValidator) GetValidators() []core.Validator {
	result := make([]core.Validator, len(c.validators))
	copy(result, c.validators)
	return result
}

// Clear removes all validators from the chain
func (c *CompositeValidator) Clear() {
	c.validators = make([]core.Validator, 0)
}

// Chain creates a new validator that runs this validator followed by the next
func (c *CompositeValidator) Chain(next core.Validator) core.Validator {
	newChainInterface := NewCompositeValidator(
		fmt.Sprintf("%s->%s", c.name, next.GetName()),
		c.version,
	)
	newChain, ok := newChainInterface.(*CompositeValidator)
	if !ok {
		// This should never happen since NewCompositeValidator always returns *CompositeValidator
		// Return current validator if type assertion fails
		return c
	}

	// Add all current validators
	for _, validator := range c.validators {
		newChain.Add(validator)
	}

	// Add the next validator
	newChain.Add(next)

	return newChain
}

// conditionalValidatorWrapper wraps a ConditionalValidator to implement Validator
type conditionalValidatorWrapper struct {
	validator core.ConditionalValidator
}

func (w *conditionalValidatorWrapper) Validate(ctx context.Context, data interface{}, options *core.ValidationOptions) *core.ValidationResult {
	if w.validator.ShouldValidate(ctx, data, options) {
		return w.validator.Validate(ctx, data, options)
	}

	// Return empty result if validation should be skipped
	return &core.ValidationResult{
		Valid: true,
		Metadata: core.ValidationMetadata{
			ValidatedAt:      time.Now(),
			ValidatorName:    w.validator.GetName(),
			ValidatorVersion: w.validator.GetVersion(),
		},
	}
}

func (w *conditionalValidatorWrapper) GetName() string {
	return w.validator.GetName()
}

func (w *conditionalValidatorWrapper) GetVersion() string {
	return w.validator.GetVersion()
}

func (w *conditionalValidatorWrapper) GetSupportedTypes() []string {
	return w.validator.GetSupportedTypes()
}

// ParallelValidator executes validators in parallel
type ParallelValidator struct {
	name       string
	version    string
	validators []core.Validator
}

// NewParallelValidator creates a new parallel validator
func NewParallelValidator(name, version string) *ParallelValidator {
	return &ParallelValidator{
		name:       name,
		version:    version,
		validators: make([]core.Validator, 0),
	}
}

// Validate executes all validators in parallel
func (p *ParallelValidator) Validate(ctx context.Context, data interface{}, options *core.ValidationOptions) *core.ValidationResult {
	startTime := time.Now()

	result := &core.ValidationResult{
		Valid:    true,
		Errors:   make([]*core.ValidationError, 0),
		Warnings: make([]*core.ValidationWarning, 0),
		Metadata: core.ValidationMetadata{
			ValidatedAt:      startTime,
			ValidatorName:    p.name,
			ValidatorVersion: p.version,
			RulesApplied:     make([]string, 0),
		},
		Suggestions: make([]string, 0),
	}

	if len(p.validators) == 0 {
		result.Duration = time.Since(startTime)
		return result
	}

	// Create channels for results
	resultChan := make(chan *core.ValidationResult, len(p.validators))

	// Start all validators in parallel
	for _, validator := range p.validators {
		go func(v core.Validator) {
			validatorResult := v.Validate(ctx, data, options)
			resultChan <- validatorResult
		}(validator)
	}

	// Collect all results
	for i := 0; i < len(p.validators); i++ {
		select {
		case validatorResult := <-resultChan:
			if validatorResult != nil {
				result.Merge(validatorResult)
			}
		case <-ctx.Done():
			result.AddError(core.NewValidationError(
				"VALIDATION_CANCELLED",
				"Parallel validation was cancelled",
				core.ErrTypeSystem,
				core.SeverityMedium,
			))
			break
		}
	}

	result.Duration = time.Since(startTime)
	return result
}

// GetName returns the name of the parallel validator
func (p *ParallelValidator) GetName() string {
	return p.name
}

// GetVersion returns the version of the parallel validator
func (p *ParallelValidator) GetVersion() string {
	return p.version
}

// GetSupportedTypes returns all types supported by validators in parallel
func (p *ParallelValidator) GetSupportedTypes() []string {
	typeSet := make(map[string]bool)
	for _, validator := range p.validators {
		for _, t := range validator.GetSupportedTypes() {
			typeSet[t] = true
		}
	}

	types := make([]string, 0, len(typeSet))
	for t := range typeSet {
		types = append(types, t)
	}
	return types
}

// Add adds a validator to the parallel execution group
func (p *ParallelValidator) Add(validator core.Validator) *ParallelValidator {
	if validator != nil {
		p.validators = append(p.validators, validator)
	}
	return p
}

// GetValidators returns all validators in the parallel group
func (p *ParallelValidator) GetValidators() []core.Validator {
	result := make([]core.Validator, len(p.validators))
	copy(result, p.validators)
	return result
}

// Clear removes all validators from the parallel group
func (p *ParallelValidator) Clear() {
	p.validators = make([]core.Validator, 0)
}
