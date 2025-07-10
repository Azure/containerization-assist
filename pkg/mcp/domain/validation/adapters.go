package validation

import (
	"context"
	"fmt"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// LegacyValidatorAdapter adapts the new unified validation system to legacy interfaces
// This enables gradual migration from existing validation functions
type LegacyValidatorAdapter struct {
	validator DomainValidator[interface{}]
}

// NewLegacyValidatorAdapter creates an adapter for a domain validator
func NewLegacyValidatorAdapter(validator DomainValidator[interface{}]) *LegacyValidatorAdapter {
	return &LegacyValidatorAdapter{
		validator: validator,
	}
}

// ValidateOldInterface provides backward compatibility with legacy validation interfaces
// that return just an error instead of ValidationResult
func (a *LegacyValidatorAdapter) ValidateOldInterface(data interface{}) error {
	result := a.validator.Validate(context.Background(), data)
	if !result.Valid && len(result.Errors) > 0 {
		return result.Errors[0] // Return first error for compatibility
	}
	return nil
}

// ValidateWithContext provides validation with context for legacy interfaces
func (a *LegacyValidatorAdapter) ValidateWithContext(ctx context.Context, data interface{}) error {
	result := a.validator.Validate(ctx, data)
	if !result.Valid && len(result.Errors) > 0 {
		return result.Errors[0]
	}
	return nil
}

// SimpleFunctionAdapter adapts simple validation functions to the unified system
type SimpleFunctionAdapter struct {
	name         string
	domain       string
	category     string
	priority     int
	validateFunc func(interface{}) error
}

// NewSimpleFunctionAdapter creates an adapter for simple validation functions
func NewSimpleFunctionAdapter(name, domain, category string, priority int, validateFunc func(interface{}) error) *SimpleFunctionAdapter {
	return &SimpleFunctionAdapter{
		name:         name,
		domain:       domain,
		category:     category,
		priority:     priority,
		validateFunc: validateFunc,
	}
}

func (a *SimpleFunctionAdapter) Validate(ctx context.Context, value interface{}) ValidationResult {
	err := a.validateFunc(value)
	if err != nil {
		return ValidationResult{
			Valid:  false,
			Errors: []error{err},
		}
	}
	return ValidationResult{
		Valid:  true,
		Errors: make([]error, 0),
	}
}

func (a *SimpleFunctionAdapter) Name() string {
	return a.name
}

func (a *SimpleFunctionAdapter) Domain() string {
	return a.domain
}

func (a *SimpleFunctionAdapter) Category() string {
	return a.category
}

func (a *SimpleFunctionAdapter) Priority() int {
	return a.priority
}

func (a *SimpleFunctionAdapter) Dependencies() []string {
	return []string{} // Simple functions typically have no dependencies
}

// RegistryAdapter provides a backward-compatible interface to the unified registry
type RegistryAdapter struct {
	registry ValidatorRegistry
}

// NewRegistryAdapter creates an adapter for the unified registry
func NewRegistryAdapter(registry ValidatorRegistry) *RegistryAdapter {
	return &RegistryAdapter{
		registry: registry,
	}
}

// ValidateKubernetesManifest provides legacy function signature for Kubernetes validation
func (r *RegistryAdapter) ValidateKubernetesManifest(manifest map[string]interface{}) error {
	result := r.registry.ValidateAll(context.Background(), manifest, "kubernetes", "manifest")
	if !result.Valid && len(result.Errors) > 0 {
		return result.Errors[0]
	}
	return nil
}

// ValidateDockerConfig provides legacy function signature for Docker validation
func (r *RegistryAdapter) ValidateDockerConfig(config map[string]interface{}) error {
	result := r.registry.ValidateAll(context.Background(), config, "docker", "config")
	if !result.Valid && len(result.Errors) > 0 {
		return result.Errors[0]
	}
	return nil
}

// ValidateSecurityPolicy provides legacy function signature for security validation
func (r *RegistryAdapter) ValidateSecurityPolicy(policy map[string]interface{}) error {
	result := r.registry.ValidateAll(context.Background(), policy, "security", "policy")
	if !result.Valid && len(result.Errors) > 0 {
		return result.Errors[0]
	}
	return nil
}

// ValidateWithDomain provides a generic validation function for any domain
func (r *RegistryAdapter) ValidateWithDomain(data interface{}, domain, category string) error {
	result := r.registry.ValidateAll(context.Background(), data, domain, category)
	if !result.Valid && len(result.Errors) > 0 {
		return result.Errors[0]
	}
	return nil
}

// StringValidationAdapter adapts string validation functions
type StringValidationAdapter struct {
	name         string
	validateFunc func(string) error
}

// NewStringValidationAdapter creates an adapter for string validation functions
func NewStringValidationAdapter(name string, validateFunc func(string) error) *StringValidationAdapter {
	return &StringValidationAdapter{
		name:         name,
		validateFunc: validateFunc,
	}
}

func (a *StringValidationAdapter) Validate(ctx context.Context, value interface{}) ValidationResult {
	str, ok := value.(string)
	if !ok {
		return ValidationResult{
			Valid:  false,
			Errors: []error{errors.NewValidationFailed("input", "expected string input")},
		}
	}

	err := a.validateFunc(str)
	if err != nil {
		return ValidationResult{
			Valid:  false,
			Errors: []error{err},
		}
	}

	return ValidationResult{
		Valid:  true,
		Errors: make([]error, 0),
	}
}

func (a *StringValidationAdapter) Name() string {
	return a.name
}

func (a *StringValidationAdapter) Domain() string {
	return "common"
}

func (a *StringValidationAdapter) Category() string {
	return "string"
}

func (a *StringValidationAdapter) Priority() int {
	return 50 // Default priority for basic validators
}

func (a *StringValidationAdapter) Dependencies() []string {
	return []string{}
}

// NetworkValidationAdapter adapts network validation functions
type NetworkValidationAdapter struct {
	name         string
	validateFunc func(string) error
}

// NewNetworkValidationAdapter creates an adapter for network validation functions
func NewNetworkValidationAdapter(name string, validateFunc func(string) error) *NetworkValidationAdapter {
	return &NetworkValidationAdapter{
		name:         name,
		validateFunc: validateFunc,
	}
}

func (a *NetworkValidationAdapter) Validate(ctx context.Context, value interface{}) ValidationResult {
	str, ok := value.(string)
	if !ok {
		return ValidationResult{
			Valid:  false,
			Errors: []error{errors.NewValidationFailed("input", "expected string input for network validation")},
		}
	}

	err := a.validateFunc(str)
	if err != nil {
		return ValidationResult{
			Valid:  false,
			Errors: []error{err},
		}
	}

	return ValidationResult{
		Valid:  true,
		Errors: make([]error, 0),
	}
}

func (a *NetworkValidationAdapter) Name() string {
	return a.name
}

func (a *NetworkValidationAdapter) Domain() string {
	return "network"
}

func (a *NetworkValidationAdapter) Category() string {
	return "basic"
}

func (a *NetworkValidationAdapter) Priority() int {
	return 75 // Higher priority for network validation
}

func (a *NetworkValidationAdapter) Dependencies() []string {
	return []string{}
}

// ValidationResultAdapter converts ValidationResult to different formats
type ValidationResultAdapter struct {
	result ValidationResult
}

// NewValidationResultAdapter creates an adapter for ValidationResult
func NewValidationResultAdapter(result ValidationResult) *ValidationResultAdapter {
	return &ValidationResultAdapter{result: result}
}

// AsError returns the first error if validation failed, nil if valid
func (a *ValidationResultAdapter) AsError() error {
	if !a.result.Valid && len(a.result.Errors) > 0 {
		return a.result.Errors[0]
	}
	return nil
}

// AsMultiError returns all errors as a custom multi-error type
func (a *ValidationResultAdapter) AsMultiError() error {
	if !a.result.Valid && len(a.result.Errors) > 0 {
		if len(a.result.Errors) == 1 {
			return a.result.Errors[0]
		}
		return &MultiValidationError{Errors: a.result.Errors}
	}
	return nil
}

// AsBoolError returns validation status and error separately
func (a *ValidationResultAdapter) AsBoolError() (bool, error) {
	return a.result.Valid, a.AsError()
}

// MultiValidationError groups multiple validation errors
type MultiValidationError struct {
	Errors []error
}

func (m *MultiValidationError) Error() string {
	if len(m.Errors) == 1 {
		return m.Errors[0].Error()
	}
	return fmt.Sprintf("multiple validation errors: %d errors", len(m.Errors))
}

func (m *MultiValidationError) Unwrap() []error {
	return m.Errors
}

// IsMultiValidationError checks if an error is a MultiValidationError
func IsMultiValidationError(err error) (*MultiValidationError, bool) {
	if multiErr, ok := err.(*MultiValidationError); ok {
		return multiErr, true
	}
	return nil, false
}

// MigrationHelper provides utilities for migrating to the unified system
type MigrationHelper struct {
	registry ValidatorRegistry
}

// NewMigrationHelper creates a helper for validation migration
func NewMigrationHelper(registry ValidatorRegistry) *MigrationHelper {
	return &MigrationHelper{registry: registry}
}

// RegisterLegacyFunction registers a legacy validation function in the unified system
func (m *MigrationHelper) RegisterLegacyFunction(name, domain, category string, priority int, validateFunc func(interface{}) error) error {
	adapter := NewSimpleFunctionAdapter(name, domain, category, priority, validateFunc)
	return m.registry.Register(adapter)
}

// RegisterStringValidator registers a string validation function
func (m *MigrationHelper) RegisterStringValidator(name string, validateFunc func(string) error) error {
	adapter := NewStringValidationAdapter(name, validateFunc)
	return m.registry.Register(adapter)
}

// RegisterNetworkValidator registers a network validation function
func (m *MigrationHelper) RegisterNetworkValidator(name string, validateFunc func(string) error) error {
	adapter := NewNetworkValidationAdapter(name, validateFunc)
	return m.registry.Register(adapter)
}

// ValidateAndAdapt validates data and returns result in the specified format
func (m *MigrationHelper) ValidateAndAdapt(data interface{}, domain, category string, resultFormat string) interface{} {
	result := m.registry.ValidateAll(context.Background(), data, domain, category)
	adapter := NewValidationResultAdapter(result)

	switch resultFormat {
	case "error":
		return adapter.AsError()
	case "multi-error":
		return adapter.AsMultiError()
	case "bool-error":
		valid, err := adapter.AsBoolError()
		return map[string]interface{}{"valid": valid, "error": err}
	default:
		return result
	}
}
