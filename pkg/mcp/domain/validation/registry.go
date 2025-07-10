package validation

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// ValidatorRegistryImpl provides a thread-safe registry for domain validators
// with dependency resolution and execution ordering
type ValidatorRegistryImpl struct {
	validators map[string]DomainValidator[interface{}]
	mu         sync.RWMutex
}

// NewValidatorRegistry creates a new validator registry
func NewValidatorRegistry() ValidatorRegistry {
	return &ValidatorRegistryImpl{
		validators: make(map[string]DomainValidator[interface{}]),
	}
}

// Register adds a domain validator to the registry
func (r *ValidatorRegistryImpl) Register(validator DomainValidator[interface{}]) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := validator.Name()
	if name == "" {
		return errors.NewValidationFailed("validator.name", "validator name cannot be empty")
	}

	// Check if validator is already registered
	if _, exists := r.validators[name]; exists {
		return errors.NewValidationFailed("validator.registration",
			fmt.Sprintf("validator '%s' is already registered", name))
	}

	// Validate dependencies exist
	for _, dep := range validator.Dependencies() {
		if _, exists := r.validators[dep]; !exists {
			return errors.NewValidationFailed("validator.dependencies",
				fmt.Sprintf("dependency '%s' not found for validator '%s'", dep, name))
		}
	}

	r.validators[name] = validator
	return nil
}

// Unregister removes a validator from the registry
func (r *ValidatorRegistryImpl) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.validators[name]; !exists {
		return errors.NewValidationFailed("validator.unregister",
			fmt.Sprintf("validator '%s' not found", name))
	}

	// Check if any other validators depend on this one
	for _, validator := range r.validators {
		for _, dep := range validator.Dependencies() {
			if dep == name {
				return errors.NewValidationFailed("validator.dependencies",
					fmt.Sprintf("cannot unregister '%s': validator '%s' depends on it",
						name, validator.Name()))
			}
		}
	}

	delete(r.validators, name)
	return nil
}

// GetValidators returns validators filtered by domain and category
func (r *ValidatorRegistryImpl) GetValidators(domain, category string) []DomainValidator[interface{}] {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []DomainValidator[interface{}]
	for _, validator := range r.validators {
		if validator.Domain() == domain && validator.Category() == category {
			result = append(result, validator)
		}
	}

	// Sort by priority (higher priority first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Priority() > result[j].Priority()
	})

	return result
}

// GetDomainValidators returns all validators for a domain
func (r *ValidatorRegistryImpl) GetDomainValidators(domain string) []DomainValidator[interface{}] {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []DomainValidator[interface{}]
	for _, validator := range r.validators {
		if validator.Domain() == domain {
			result = append(result, validator)
		}
	}

	// Sort by priority (higher priority first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Priority() > result[j].Priority()
	})

	return result
}

// ValidateAll executes all applicable validators for domain and category
func (r *ValidatorRegistryImpl) ValidateAll(ctx context.Context, data interface{}, domain, category string) ValidationResult {
	validators := r.GetValidators(domain, category)

	if len(validators) == 0 {
		return ValidationResult{
			Valid:    true,
			Errors:   make([]error, 0),
			Warnings: make([]string, 0),
		}
	}

	// Resolve dependencies and create execution order
	orderedValidators, err := r.resolveDependencies(validators)
	if err != nil {
		return ValidationResult{
			Valid:  false,
			Errors: []error{err},
		}
	}

	// Execute validators in dependency order
	result := ValidationResult{
		Valid:    true,
		Errors:   make([]error, 0),
		Warnings: make([]string, 0),
	}

	for _, validator := range orderedValidators {
		validationResult := validator.Validate(ctx, data)

		// Collect errors and warnings
		result.Errors = append(result.Errors, validationResult.Errors...)
		result.Warnings = append(result.Warnings, validationResult.Warnings...)

		// Update overall validity
		if !validationResult.Valid {
			result.Valid = false
		}
	}

	return result
}

// ListValidators returns information about all registered validators
func (r *ValidatorRegistryImpl) ListValidators() []ValidatorInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []ValidatorInfo
	for _, validator := range r.validators {
		info := ValidatorInfo{
			Name:         validator.Name(),
			Domain:       validator.Domain(),
			Category:     validator.Category(),
			Priority:     validator.Priority(),
			Dependencies: validator.Dependencies(),
		}
		result = append(result, info)
	}

	// Sort by domain, then category, then priority
	sort.Slice(result, func(i, j int) bool {
		if result[i].Domain != result[j].Domain {
			return result[i].Domain < result[j].Domain
		}
		if result[i].Category != result[j].Category {
			return result[i].Category < result[j].Category
		}
		return result[i].Priority > result[j].Priority
	})

	return result
}

// resolveDependencies performs topological sort to resolve validator dependencies
func (r *ValidatorRegistryImpl) resolveDependencies(validators []DomainValidator[interface{}]) ([]DomainValidator[interface{}], error) {
	// Create dependency graph
	validatorMap := make(map[string]DomainValidator[interface{}])
	inDegree := make(map[string]int)
	graph := make(map[string][]string)

	// Initialize
	for _, validator := range validators {
		name := validator.Name()
		validatorMap[name] = validator
		inDegree[name] = 0
		graph[name] = make([]string, 0)
	}

	// Build dependency graph
	for _, validator := range validators {
		name := validator.Name()
		for _, dep := range validator.Dependencies() {
			// Only consider dependencies within our validator set
			if _, exists := validatorMap[dep]; exists {
				graph[dep] = append(graph[dep], name)
				inDegree[name]++
			}
		}
	}

	// Topological sort using Kahn's algorithm
	var result []DomainValidator[interface{}]
	queue := make([]string, 0)

	// Find validators with no dependencies
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}

	for len(queue) > 0 {
		// Dequeue
		current := queue[0]
		queue = queue[1:]

		result = append(result, validatorMap[current])

		// Process dependents
		for _, dependent := range graph[current] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	// Check for circular dependencies
	if len(result) != len(validators) {
		return nil, errors.NewValidationFailed("validator.dependencies",
			"circular dependency detected in validator dependencies")
	}

	return result, nil
}

// GetValidatorByName returns a specific validator by name (for testing/debugging)
func (r *ValidatorRegistryImpl) GetValidatorByName(name string) (DomainValidator[interface{}], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	validator, exists := r.validators[name]
	if !exists {
		return nil, errors.NewValidationFailed("validator.lookup",
			fmt.Sprintf("validator '%s' not found", name))
	}

	return validator, nil
}

// Count returns the number of registered validators
func (r *ValidatorRegistryImpl) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.validators)
}

// Clear removes all validators (for testing)
func (r *ValidatorRegistryImpl) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.validators = make(map[string]DomainValidator[interface{}])
}
