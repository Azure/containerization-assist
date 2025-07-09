package core

import (
	"sync"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors/codes"
)

// DefaultValidatorRegistry implements ValidatorRegistry
type DefaultValidatorRegistry struct {
	validators map[string]Validator
	mu         sync.RWMutex
}

// NewValidatorRegistry creates a new validator registry
func NewValidatorRegistry() ValidatorRegistry {
	return &DefaultValidatorRegistry{
		validators: make(map[string]Validator),
	}
}

// Register registers a validator
func (r *DefaultValidatorRegistry) Register(name string, validator Validator) error {
	if name == "" {
		return errors.NewError().
			Code(codes.VALIDATION_REQUIRED_MISSING).
			Message("Validator name cannot be empty").
			Context("field", "name").
			Build()
	}
	if validator == nil {
		return errors.NewError().
			Code(codes.VALIDATION_REQUIRED_MISSING).
			Message("Validator cannot be nil").
			Context("parameter", "validator").
			Build()
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.validators[name]; exists {
		return errors.NewError().
			Code(codes.RESOURCE_ALREADY_EXISTS).
			Messagef("Validator '%s' is already registered", name).
			Context("validator_name", name).
			Suggestion("Use a different name or unregister the existing validator first").
			Build()
	}

	r.validators[name] = validator
	return nil
}

// Unregister removes a validator
func (r *DefaultValidatorRegistry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.validators[name]; !exists {
		return errors.NewError().
			Code(codes.RESOURCE_NOT_FOUND).
			Messagef("Validator '%s' is not registered", name).
			Context("validator_name", name).
			Suggestion("Check the validator name or list available validators").
			Build()
	}

	delete(r.validators, name)
	return nil
}

// Get retrieves a validator by name
func (r *DefaultValidatorRegistry) Get(name string) (Validator, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	validator, exists := r.validators[name]
	return validator, exists
}

// List returns all registered validators
func (r *DefaultValidatorRegistry) List() map[string]Validator {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]Validator)
	for name, validator := range r.validators {
		result[name] = validator
	}
	return result
}

// GetByType returns validators that support the given type
func (r *DefaultValidatorRegistry) GetByType(dataType string) []Validator {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []Validator
	for _, validator := range r.validators {
		supportedTypes := validator.GetSupportedTypes()
		for _, supportedType := range supportedTypes {
			if supportedType == dataType {
				result = append(result, validator)
				break
			}
		}
	}
	return result
}

// Clear removes all validators
func (r *DefaultValidatorRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.validators = make(map[string]Validator)
}

// GlobalRegistry is the global validator registry
var GlobalRegistry = NewValidatorRegistry()

// RegisterValidator registers a validator globally
func RegisterValidator(name string, validator Validator) error {
	return GlobalRegistry.Register(name, validator)
}

// GetValidator retrieves a validator by name from the global registry
func GetValidator(name string) (Validator, bool) {
	return GlobalRegistry.Get(name)
}

// ListValidators returns all globally registered validators
func ListValidators() map[string]Validator {
	return GlobalRegistry.List()
}

// GetValidatorsByType returns globally registered validators that support the given type
func GetValidatorsByType(dataType string) []Validator {
	return GlobalRegistry.GetByType(dataType)
}
