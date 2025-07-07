package core

import (
	"context"
)

// Validator defines the core interface for all validators
// Deprecated: Use GenericValidator[T] for type-safe validation
type Validator interface {
	// Validate performs validation on the provided data
	Validate(ctx context.Context, data interface{}, options *ValidationOptions) *NonGenericResult

	// GetName returns the validator name
	GetName() string

	// GetVersion returns the validator version
	GetVersion() string

	// GetSupportedTypes returns the data types this validator can handle
	GetSupportedTypes() []string
}

// GenericValidator defines the type-safe validator interface using generics
type GenericValidator[T any] interface {
	// Validate performs type-safe validation on the provided data
	Validate(ctx context.Context, data T, options *ValidationOptions) *Result[T]

	// GetName returns the validator name
	GetName() string

	// GetVersion returns the validator version
	GetVersion() string

	// GetSupportedTypes returns the data types this validator can handle
	GetSupportedTypes() []string
}

// Domain-specific validator interfaces
type BuildValidator = GenericValidator[BuildValidationData]
type DeployValidator = GenericValidator[DeployValidationData]
type SecurityValidator = GenericValidator[SecurityValidationData]
type SessionValidator = GenericValidator[SessionValidationData]
type RuntimeValidator = GenericValidator[RuntimeValidationData]

// ValidatorRegistry defines the interface for validator management
// Deprecated: Use TypedValidatorRegistry for type safety.
// NOTE: This is different from api.ValidatorRegistry which has a different signature.
// This interface is for internal use only.
type ValidatorRegistry interface {
	// Register registers a validator with the given name
	Register(name string, validator Validator) error

	// Unregister removes a validator
	Unregister(name string) error

	// Get retrieves a validator by name
	Get(name string) (Validator, bool)

	// List returns all registered validators
	List() map[string]Validator

	// GetByType returns validators that support the given type
	GetByType(dataType string) []Validator

	// Clear removes all validators
	Clear()
}

// TypedValidatorRegistry defines a type-safe interface for validator management
type TypedValidatorRegistry[T any] interface {
	// Register registers a typed validator with the given name
	Register(name string, validator GenericValidator[T]) error

	// Unregister removes a validator
	Unregister(name string) error

	// Get retrieves a typed validator by name
	Get(name string) (GenericValidator[T], bool)

	// List returns all registered typed validators
	List() map[string]GenericValidator[T]

	// GetByType returns typed validators that support the given type
	GetByType(dataType string) []GenericValidator[T]

	// Clear removes all validators
	Clear()
}

// ValidatorChain defines an interface for chaining multiple validators
type ValidatorChain interface {
	Validator

	// AddValidator adds a validator to the chain
	AddValidator(validator Validator) ValidatorChain

	// RemoveValidator removes a validator from the chain
	RemoveValidator(name string) bool

	// GetValidators returns all validators in the chain
	GetValidators() []Validator

	// Clear removes all validators from the chain
	Clear()
}

// ConditionalValidator defines an interface for conditional validation
// Deprecated: Use TypedConditionalValidator[T] for type safety
type ConditionalValidator interface {
	Validator

	// ShouldValidate determines if validation should be performed based on data and context
	ShouldValidate(ctx context.Context, data interface{}, options *ValidationOptions) bool

	// GetCondition returns the condition function
	GetCondition() func(interface{}) bool

	// SetCondition sets the condition function
	SetCondition(condition func(interface{}) bool)
}

// TypedConditionalValidator defines a type-safe interface for conditional validation
type TypedConditionalValidator[T any] interface {
	GenericValidator[T]

	// ShouldValidate determines if validation should be performed based on typed data and context
	ShouldValidate(ctx context.Context, data T, options *ValidationOptions) bool

	// GetCondition returns the type-safe condition function
	GetCondition() func(T) bool

	// SetCondition sets the type-safe condition function
	SetCondition(condition func(T) bool)
}

// ValidationContext provides context information for validation operations
type ValidationContext struct {
	SessionID string                 `json:"session_id"`
	Tool      string                 `json:"tool"`
	Operation string                 `json:"operation"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// TypedValidationContext provides type-safe context information for validation operations
type TypedValidationContext struct {
	SessionID     string             `json:"session_id"`
	Tool          string             `json:"tool"`
	Operation     string             `json:"operation"`
	StringFields  map[string]string  `json:"string_fields,omitempty"`
	NumberFields  map[string]float64 `json:"number_fields,omitempty"`
	BooleanFields map[string]bool    `json:"boolean_fields,omitempty"`
}

// ToLegacy converts TypedValidationContext to the legacy ValidationContext for backward compatibility
func (tvc *TypedValidationContext) ToLegacy() *ValidationContext {
	legacyMetadata := make(map[string]interface{})

	for k, v := range tvc.StringFields {
		legacyMetadata[k] = v
	}
	for k, v := range tvc.NumberFields {
		legacyMetadata[k] = v
	}
	for k, v := range tvc.BooleanFields {
		legacyMetadata[k] = v
	}

	return &ValidationContext{
		SessionID: tvc.SessionID,
		Tool:      tvc.Tool,
		Operation: tvc.Operation,
		Metadata:  legacyMetadata,
	}
}

// FromLegacy creates TypedValidationContext from legacy ValidationContext
func NewTypedValidationContextFromLegacy(vc *ValidationContext) *TypedValidationContext {
	tvc := &TypedValidationContext{
		SessionID:     vc.SessionID,
		Tool:          vc.Tool,
		Operation:     vc.Operation,
		StringFields:  make(map[string]string),
		NumberFields:  make(map[string]float64),
		BooleanFields: make(map[string]bool),
	}

	for k, v := range vc.Metadata {
		switch val := v.(type) {
		case string:
			tvc.StringFields[k] = val
		case float64:
			tvc.NumberFields[k] = val
		case int:
			tvc.NumberFields[k] = float64(val)
		case bool:
			tvc.BooleanFields[k] = val
		}
	}

	return tvc
}
