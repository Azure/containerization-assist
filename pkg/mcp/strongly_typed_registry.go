package mcp

import (
	"fmt"
	"sync"
)

// stronglyTypedRegistry implements StronglyTypedToolRegistry
type stronglyTypedRegistry struct {
	mu        sync.RWMutex
	factories map[string]StronglyTypedToolFactory[Tool]
}

// NewStronglyTypedRegistry creates a new strongly-typed tool registry
func NewStronglyTypedRegistry() StronglyTypedToolRegistry {
	return &stronglyTypedRegistry{
		factories: make(map[string]StronglyTypedToolFactory[Tool]),
	}
}

// RegisterTyped registers a strongly-typed factory
func (r *stronglyTypedRegistry) RegisterTyped(name string, factory StronglyTypedToolFactory[Tool]) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.factories[name]; exists {
		return fmt.Errorf("tool factory '%s' is already registered", name)
	}

	r.factories[name] = factory
	return nil
}

// GetTyped retrieves a strongly-typed factory
func (r *stronglyTypedRegistry) GetTyped(name string) (StronglyTypedToolFactory[Tool], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	factory, exists := r.factories[name]
	if !exists {
		return nil, fmt.Errorf("tool factory '%s' not found", name)
	}

	return factory, nil
}

// Note: Generic methods removed due to Go interface limitation
// Use helper functions RegisterGeneric and GetGeneric instead

// List returns all registered tool names
func (r *stronglyTypedRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.factories))
	for name := range r.factories {
		names = append(names, name)
	}
	return names
}

// GetMetadata returns metadata for all registered tools
func (r *stronglyTypedRegistry) GetMetadata() map[string]ToolMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()

	metadata := make(map[string]ToolMetadata)
	for name, factory := range r.factories {
		metadata[name] = factory.GetMetadata()
	}
	return metadata
}

// typeErasedFactory adapts a specific typed factory to the Tool interface
type typeErasedFactory[T Tool] struct {
	original StronglyTypedToolFactory[T]
}

func (f *typeErasedFactory[T]) Create() Tool {
	return f.original.Create()
}

func (f *typeErasedFactory[T]) GetType() string {
	return f.original.GetType()
}

func (f *typeErasedFactory[T]) GetMetadata() ToolMetadata {
	return f.original.GetMetadata()
}
