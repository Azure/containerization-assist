package mcp

import (
	"reflect"
)

// FactoryBuilder helps build strongly-typed factories with less boilerplate
type FactoryBuilder[T Tool] struct {
	factoryFunc TypedFactoryFunc[T]
	toolType    string
	metadata    ToolMetadata
}

// NewFactoryBuilder creates a new factory builder for a specific tool type
func NewFactoryBuilder[T Tool](factoryFunc TypedFactoryFunc[T]) *FactoryBuilder[T] {
	return &FactoryBuilder[T]{
		factoryFunc: factoryFunc,
	}
}

// WithType sets the tool type name
func (b *FactoryBuilder[T]) WithType(toolType string) *FactoryBuilder[T] {
	b.toolType = toolType
	return b
}

// WithMetadata sets the tool metadata
func (b *FactoryBuilder[T]) WithMetadata(metadata ToolMetadata) *FactoryBuilder[T] {
	b.metadata = metadata
	return b
}

// WithBasicMetadata sets basic metadata fields
func (b *FactoryBuilder[T]) WithBasicMetadata(name, description, version, category string) *FactoryBuilder[T] {
	b.metadata = ToolMetadata{
		Name:        name,
		Description: description,
		Version:     version,
		Category:    category,
	}
	return b
}

// Build creates the strongly-typed factory
func (b *FactoryBuilder[T]) Build() StronglyTypedToolFactory[T] {
	if b.toolType == "" {
		// Use reflection to get the type name if not provided
		var zero T
		b.toolType = reflect.TypeOf(zero).Elem().Name()
	}

	if b.metadata.Name == "" {
		b.metadata.Name = b.toolType
	}

	return NewStronglyTypedFactory(b.factoryFunc, b.toolType, b.metadata)
}

// RegisterBuilder is a convenience function that builds and registers a factory
func RegisterBuilder[T Tool](registry StronglyTypedToolRegistry, name string, builder *FactoryBuilder[T]) error {
	factory := builder.Build()
	return RegisterGeneric(registry, name, factory)
}

// SimpleFactory creates a basic factory with minimal configuration
func SimpleFactory[T Tool](factoryFunc TypedFactoryFunc[T], name string) StronglyTypedToolFactory[T] {
	return NewFactoryBuilder(factoryFunc).
		WithType(name).
		WithBasicMetadata(name, "Auto-generated tool", "1.0.0", "general").
		Build()
}

// MustRegisterSimple registers a simple factory and panics on error (for init functions)
func MustRegisterSimple[T Tool](registry StronglyTypedToolRegistry, name string, factoryFunc TypedFactoryFunc[T]) {
	factory := SimpleFactory(factoryFunc, name)
	if err := RegisterGeneric(registry, name, factory); err != nil {
		panic(err)
	}
}
