package mcp

import (
	"reflect"

	"github.com/Azure/container-kit/pkg/mcp/core"
)

// FactoryBuilder helps build strongly-typed factories with less boilerplate
type FactoryBuilder[T core.Tool] struct {
	factoryFunc TypedFactoryFunc[T]
	toolType    string
	metadata    core.ToolMetadata
}

// TypedFactoryFunc represents typed factory function
type TypedFactoryFunc[T core.Tool] func() T

// StronglyTypedToolFactory creates typed tool instances
type StronglyTypedToolFactory[T core.Tool] interface {
	Create() T
	GetType() string
	GetMetadata() core.ToolMetadata
}

// NewFactoryBuilder creates a new factory builder for a specific tool type
func NewFactoryBuilder[T core.Tool](factoryFunc TypedFactoryFunc[T]) *FactoryBuilder[T] {
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
func (b *FactoryBuilder[T]) WithMetadata(metadata core.ToolMetadata) *FactoryBuilder[T] {
	b.metadata = metadata
	return b
}

// WithBasicMetadata sets basic metadata fields
func (b *FactoryBuilder[T]) WithBasicMetadata(name, description, version, category string) *FactoryBuilder[T] {
	b.metadata = core.ToolMetadata{
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

// NewStronglyTypedFactory creates typed factory
func NewStronglyTypedFactory[T core.Tool](factoryFunc TypedFactoryFunc[T], toolType string, metadata core.ToolMetadata) StronglyTypedToolFactory[T] {
	return &stronglyTypedFactory[T]{
		factoryFunc: factoryFunc,
		toolType:    toolType,
		metadata:    metadata,
	}
}

// stronglyTypedFactory implements StronglyTypedToolFactory
type stronglyTypedFactory[T core.Tool] struct {
	factoryFunc TypedFactoryFunc[T]
	toolType    string
	metadata    core.ToolMetadata
}

func (f *stronglyTypedFactory[T]) Create() T {
	return f.factoryFunc()
}

func (f *stronglyTypedFactory[T]) GetType() string {
	return f.toolType
}

func (f *stronglyTypedFactory[T]) GetMetadata() core.ToolMetadata {
	return f.metadata
}
