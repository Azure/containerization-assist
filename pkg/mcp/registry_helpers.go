package mcp

import (
	"fmt"
)

// =============================================================================
// Generic Registry Interface (for type safety)
// =============================================================================

// GenericRegistry defines the common interface for registries that support generic operations
type GenericRegistry interface {
	// Basic registry operations
	List() []string
	GetMetadata() map[string]ToolMetadata

	// These methods are implemented by the helper functions below due to Go's generic limitation
	// registerGeneric[T Tool](name string, factory StronglyTypedToolFactory[T]) error
	// getGeneric[T Tool](name string) (StronglyTypedToolFactory[T], error)
}

// =============================================================================
// Generic Helper Functions (workaround for Go interface limitation)
// =============================================================================

// RegisterGeneric registers a generic strongly-typed factory with a registry
// Note: Uses interface{} due to Go's limitation with generic methods in interfaces
func RegisterGeneric[T Tool](registry interface{}, name string, factory StronglyTypedToolFactory[T]) error {
	// Ensure the registry implements GenericRegistry for type safety
	if _, ok := registry.(GenericRegistry); !ok {
		return fmt.Errorf("registry must implement GenericRegistry interface, got: %T", registry)
	}

	switch r := registry.(type) {
	case *StandardToolRegistry:
		return registerGenericStandard(r, name, factory)
	case *stronglyTypedRegistry:
		return registerGenericTyped(r, name, factory)
	default:
		return fmt.Errorf("unsupported registry type: %T", registry)
	}
}

// GetGeneric retrieves a generic strongly-typed factory from a registry
// Note: Uses interface{} due to Go's limitation with generic methods in interfaces
func GetGeneric[T Tool](registry interface{}, name string) (StronglyTypedToolFactory[T], error) {
	// Ensure the registry implements GenericRegistry for type safety
	if _, ok := registry.(GenericRegistry); !ok {
		return nil, fmt.Errorf("registry must implement GenericRegistry interface, got: %T", registry)
	}

	switch r := registry.(type) {
	case *StandardToolRegistry:
		return getGenericStandard[T](r, name)
	case *stronglyTypedRegistry:
		return getGenericTyped[T](r, name)
	default:
		return nil, fmt.Errorf("unsupported registry type: %T", registry)
	}
}

// RegisterWithBuilder registers a tool using the factory builder pattern
func RegisterWithBuilder[T Tool](registry interface{}, name string, builder *FactoryBuilder[T]) error {
	factory := builder.Build()
	return RegisterGeneric(registry, name, factory)
}

// RegisterInjectable registers an injectable tool with a registry
func RegisterInjectable[T InjectableTool](registry interface{}, name string, factory InjectableToolFactory[T]) error {
	// Convert to standard factory
	standardFactory := NewStronglyTypedFactory(
		func() Tool { return factory.Create(nil) }, // ClientFactory will be injected later
		name,
		factory.GetMetadata(),
	)
	return RegisterGeneric(registry, name, standardFactory)
}

// =============================================================================
// Internal Implementation Functions
// =============================================================================

// registerGenericStandard implements RegisterGeneric for StandardToolRegistry
func registerGenericStandard[T Tool](r *StandardToolRegistry, name string, factory StronglyTypedToolFactory[T]) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.typedFactories[name]; exists {
		return fmt.Errorf("tool factory '%s' is already registered", name)
	}

	// Type-erase to Tool interface for storage
	adaptedFactory := &typeErasedFactory[T]{
		original: factory,
	}

	r.typedFactories[name] = adaptedFactory
	r.registrationOrder = append(r.registrationOrder, name)
	return nil
}

// getGenericStandard implements GetGeneric for StandardToolRegistry
func getGenericStandard[T Tool](r *StandardToolRegistry, name string) (StronglyTypedToolFactory[T], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	factory, exists := r.typedFactories[name]
	if !exists {
		return nil, fmt.Errorf("typed tool factory '%s' not found", name)
	}

	// Try to convert back to the specific type
	if typedFactory, ok := factory.(*typeErasedFactory[T]); ok {
		return typedFactory.original, nil
	}

	return nil, fmt.Errorf("tool factory '%s' does not match expected type", name)
}

// registerGenericTyped implements RegisterGeneric for stronglyTypedRegistry
func registerGenericTyped[T Tool](r *stronglyTypedRegistry, name string, factory StronglyTypedToolFactory[T]) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.factories[name]; exists {
		return fmt.Errorf("tool factory '%s' is already registered", name)
	}

	// Type-erase to Tool interface for storage
	adaptedFactory := &typeErasedFactory[T]{
		original: factory,
	}

	r.factories[name] = adaptedFactory
	return nil
}

// getGenericTyped implements GetGeneric for stronglyTypedRegistry
func getGenericTyped[T Tool](r *stronglyTypedRegistry, name string) (StronglyTypedToolFactory[T], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	factory, exists := r.factories[name]
	if !exists {
		return nil, fmt.Errorf("tool factory '%s' not found", name)
	}

	// Try to convert back to the specific type
	if typedFactory, ok := factory.(*typeErasedFactory[T]); ok {
		return typedFactory.original, nil
	}

	return nil, fmt.Errorf("tool factory '%s' does not match expected type", name)
}

// =============================================================================
// Convenience Functions
// =============================================================================

// CreateStandardRegistry creates a new standard registry with helper support
func CreateStandardRegistry() *StandardToolRegistry {
	return NewStandardToolRegistry()
}

// CreateStronglyTypedRegistry creates a new strongly-typed registry with helper support
func CreateStronglyTypedRegistry() *stronglyTypedRegistry {
	return &stronglyTypedRegistry{
		factories: make(map[string]StronglyTypedToolFactory[Tool]),
	}
}

// MustRegisterGeneric registers a generic factory and panics on error (for init functions)
func MustRegisterGeneric[T Tool](registry interface{}, name string, factory StronglyTypedToolFactory[T]) {
	if err := RegisterGeneric(registry, name, factory); err != nil {
		panic(fmt.Sprintf("failed to register tool '%s': %v", name, err))
	}
}

// MustRegisterWithBuilder registers with builder and panics on error (for init functions)
func MustRegisterWithBuilder[T Tool](registry interface{}, name string, builder *FactoryBuilder[T]) {
	if err := RegisterWithBuilder(registry, name, builder); err != nil {
		panic(fmt.Sprintf("failed to register tool '%s': %v", name, err))
	}
}
