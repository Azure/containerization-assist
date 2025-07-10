package validation

import (
	"context"
)

// Validator defines the core validation interface
type Validator[T any] interface {
	// Validate validates a value and returns validation result
	Validate(ctx context.Context, value T) ValidationResult

	// Name returns the validator name for error reporting
	Name() string
}

// ValidationResult holds validation outcome
type ValidationResult struct {
	Valid    bool
	Errors   []error
	Warnings []string
	Context  ValidationContext
}

// ValidationContext provides validation execution context
type ValidationContext struct {
	Field    string
	Path     string
	Metadata map[string]interface{}
}

// ValidatorChain allows composing multiple validators
type ValidatorChain[T any] struct {
	validators []Validator[T]
	strategy   ChainStrategy
}

// ChainStrategy defines how validators are executed
type ChainStrategy int

const (
	// StopOnFirstError stops chain on first validation error
	StopOnFirstError ChainStrategy = iota
	// ContinueOnError continues chain collecting all errors
	ContinueOnError
	// StopOnFirstWarning stops chain on first warning
	StopOnFirstWarning
)

// DomainValidator extends basic validation with domain-specific metadata
type DomainValidator[T any] interface {
	Validator[T]

	// Domain returns the validation domain (e.g., "kubernetes", "docker", "security")
	Domain() string

	// Category returns the validation category (e.g., "manifest", "config", "policy")
	Category() string

	// Priority returns validation priority for ordering (higher = earlier)
	Priority() int

	// Dependencies returns validator names this depends on
	Dependencies() []string
}

// ValidatorRegistry manages domain validators with dependency resolution
type ValidatorRegistry interface {
	// Register a domain validator
	Register(validator DomainValidator[interface{}]) error

	// Unregister a validator by name
	Unregister(name string) error

	// Get validators by domain and category
	GetValidators(domain, category string) []DomainValidator[interface{}]

	// Get all validators for a domain
	GetDomainValidators(domain string) []DomainValidator[interface{}]

	// Validate using all applicable validators
	ValidateAll(ctx context.Context, data interface{}, domain, category string) ValidationResult

	// List all registered validators
	ListValidators() []ValidatorInfo
}

// ValidatorInfo provides metadata about registered validators
type ValidatorInfo struct {
	Name         string   `json:"name"`
	Domain       string   `json:"domain"`
	Category     string   `json:"category"`
	Priority     int      `json:"priority"`
	Dependencies []string `json:"dependencies"`
}

// NewValidatorChain creates a new validator chain
func NewValidatorChain[T any](strategy ChainStrategy) *ValidatorChain[T] {
	return &ValidatorChain[T]{
		validators: make([]Validator[T], 0),
		strategy:   strategy,
	}
}

// Add adds a validator to the chain
func (c *ValidatorChain[T]) Add(validator Validator[T]) *ValidatorChain[T] {
	c.validators = append(c.validators, validator)
	return c
}

// Validate executes the validator chain
func (c *ValidatorChain[T]) Validate(ctx context.Context, value T) ValidationResult {
	result := ValidationResult{
		Valid:    true,
		Errors:   make([]error, 0),
		Warnings: make([]string, 0),
	}

	for _, validator := range c.validators {
		validationResult := validator.Validate(ctx, value)

		// Collect errors and warnings
		result.Errors = append(result.Errors, validationResult.Errors...)
		result.Warnings = append(result.Warnings, validationResult.Warnings...)

		// Apply strategy
		if !validationResult.Valid {
			result.Valid = false
			if c.strategy == StopOnFirstError {
				break
			}
		}

		if len(validationResult.Warnings) > 0 && c.strategy == StopOnFirstWarning {
			break
		}
	}

	return result
}

// Name returns the chain name
func (c *ValidatorChain[T]) Name() string {
	return "ValidatorChain"
}
