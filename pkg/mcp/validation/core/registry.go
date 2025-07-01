package core

import (
	"fmt"
	"sync"
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
		return fmt.Errorf("validator name cannot be empty")
	}
	if validator == nil {
		return fmt.Errorf("validator cannot be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.validators[name]; exists {
		return fmt.Errorf("validator '%s' is already registered", name)
	}

	r.validators[name] = validator
	return nil
}

// Unregister removes a validator
func (r *DefaultValidatorRegistry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.validators[name]; !exists {
		return fmt.Errorf("validator '%s' is not registered", name)
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
