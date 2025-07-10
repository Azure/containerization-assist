package runtime

import (
	"context"
	"sync"

	"github.com/Azure/container-kit/pkg/mcp/domain/security"
)

// ValidatorService provides validator registry management without global state
type ValidatorService struct {
	mu sync.RWMutex
	// registry *RuntimeValidatorRegistry  // TODO: Update when validator.go is migrated
	validators map[string]security.Validator
}

// NewValidatorService creates a new validator service
func NewValidatorService() *ValidatorService {
	return &ValidatorService{
		validators: make(map[string]security.Validator),
	}
}

// RegisterValidator registers a validator with the service
func (vs *ValidatorService) RegisterValidator(name string, validator security.Validator) {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	vs.validators[name] = validator
}

// GetValidator retrieves a validator by name
func (vs *ValidatorService) GetValidator(name string) (security.Validator, bool) {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	validator, exists := vs.validators[name]
	return validator, exists
}

// ValidateWithRuntime validates input using the runtime validation system
func (vs *ValidatorService) ValidateWithRuntime(ctx context.Context, validatorName string, input interface{}, options security.Options) (*security.Result, error) {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	validator, exists := vs.validators[validatorName]
	if !exists {
		result := security.NewResult()
		result.AddError("validator", "validator not found", "VALIDATOR_NOT_FOUND", validatorName, security.SeverityHigh)
		return result, nil
	}

	result := validator.ValidateWithOptions(ctx, input, options)
	return &result, nil
}

// ValidateWithUnified validates input using the unified validation system
func (vs *ValidatorService) ValidateWithUnified(ctx context.Context, validatorName string, input interface{}, options *security.Options) (*security.Result, error) {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	validator, exists := vs.validators[validatorName]
	if !exists {
		result := security.NewResult()
		result.AddError("validator", "validator not found", "VALIDATOR_NOT_FOUND", validatorName, security.SeverityHigh)
		return result, nil
	}

	opts := security.Options{}
	if options != nil {
		opts = *options
	}

	result := validator.ValidateWithOptions(ctx, input, opts)
	return &result, nil
}

// Reset clears all registered validators (useful for testing)
func (vs *ValidatorService) Reset() {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	vs.validators = make(map[string]security.Validator)
}
