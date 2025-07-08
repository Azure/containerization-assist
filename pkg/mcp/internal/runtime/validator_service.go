package runtime

import (
	"context"
	"sync"

	"github.com/Azure/container-kit/pkg/common/validation-core/core"
)

// ValidatorService provides validator registry management without global state
type ValidatorService struct {
	mu       sync.RWMutex
	registry *RuntimeValidatorRegistry
}

// NewValidatorService creates a new validator service
func NewValidatorService() *ValidatorService {
	return &ValidatorService{
		registry: NewRuntimeValidatorRegistry(),
	}
}

// RegisterValidator registers a validator with the service
func (vs *ValidatorService) RegisterValidator(name string, validator BaseValidator) {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	vs.registry.RegisterValidator(name, validator)
}

// GetValidator retrieves a validator by name
func (vs *ValidatorService) GetValidator(name string) (BaseValidator, bool) {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return vs.registry.GetValidator(name)
}

// ValidateWithRuntime validates input using the runtime validation system
func (vs *ValidatorService) ValidateWithRuntime(ctx context.Context, validatorName string, input interface{}, options ValidationOptions) (*RuntimeValidationResult, error) {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return vs.registry.ValidateWithRuntime(ctx, validatorName, input, options)
}

// ValidateWithUnified validates input using the unified validation system
func (vs *ValidatorService) ValidateWithUnified(ctx context.Context, validatorName string, input interface{}, options *core.ValidationOptions) (*core.NonGenericResult, error) {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return vs.registry.ValidateWithUnified(ctx, validatorName, input, options)
}

// Reset clears all registered validators (useful for testing)
func (vs *ValidatorService) Reset() {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	vs.registry = NewRuntimeValidatorRegistry()
}
