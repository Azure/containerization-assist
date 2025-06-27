package mcp

import (
	"fmt"
	"sync"
)

// StandardToolRegistry implements both ToolRegistry and StronglyTypedToolRegistry
// with unified registration patterns
type StandardToolRegistry struct {
	mu                sync.RWMutex
	legacyFactories   map[string]ToolFactory
	typedFactories    map[string]StronglyTypedToolFactory[Tool]
	registrationOrder []string
}

// NewStandardToolRegistry creates a new standardized tool registry
func NewStandardToolRegistry() *StandardToolRegistry {
	return &StandardToolRegistry{
		legacyFactories: make(map[string]ToolFactory),
		typedFactories:  make(map[string]StronglyTypedToolFactory[Tool]),
	}
}

// =============================================================================
// Legacy ToolRegistry Interface Implementation
// =============================================================================

// Register implements ToolRegistry interface for legacy compatibility
func (r *StandardToolRegistry) Register(name string, factory ToolFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.legacyFactories[name]; exists {
		return fmt.Errorf("tool factory '%s' is already registered", name)
	}

	r.legacyFactories[name] = factory
	r.registrationOrder = append(r.registrationOrder, name)
	return nil
}

// Get implements ToolRegistry interface for legacy compatibility
func (r *StandardToolRegistry) Get(name string) (ToolFactory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	factory, exists := r.legacyFactories[name]
	if !exists {
		return nil, fmt.Errorf("tool factory '%s' not found", name)
	}

	return factory, nil
}

// List implements ToolRegistry interface
func (r *StandardToolRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Return names in registration order for consistency
	return append([]string(nil), r.registrationOrder...)
}

// GetMetadata implements ToolRegistry interface
func (r *StandardToolRegistry) GetMetadata() map[string]ToolMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()

	metadata := make(map[string]ToolMetadata)

	// Get metadata from legacy factories by creating instances
	for name, factory := range r.legacyFactories {
		tool := factory()
		if metadataProvider, ok := tool.(interface{ GetMetadata() ToolMetadata }); ok {
			metadata[name] = metadataProvider.GetMetadata()
		}
	}

	// Get metadata from typed factories
	for name, factory := range r.typedFactories {
		metadata[name] = factory.GetMetadata()
	}

	return metadata
}

// =============================================================================
// StronglyTypedToolRegistry Interface Implementation
// =============================================================================

// RegisterTyped implements StronglyTypedToolRegistry interface
func (r *StandardToolRegistry) RegisterTyped(name string, factory StronglyTypedToolFactory[Tool]) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.typedFactories[name]; exists {
		return fmt.Errorf("typed tool factory '%s' is already registered", name)
	}

	r.typedFactories[name] = factory
	r.registrationOrder = append(r.registrationOrder, name)
	return nil
}

// GetTyped implements StronglyTypedToolRegistry interface
func (r *StandardToolRegistry) GetTyped(name string) (StronglyTypedToolFactory[Tool], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	factory, exists := r.typedFactories[name]
	if !exists {
		return nil, fmt.Errorf("typed tool factory '%s' not found", name)
	}

	return factory, nil
}

// Note: Generic methods removed due to Go interface limitation
// Use helper functions RegisterGeneric and GetGeneric instead

// =============================================================================
// Enhanced Methods for Standardized Registration
// =============================================================================

// RegisterStandard is the preferred method for registering tools with full metadata
func (r *StandardToolRegistry) RegisterStandard(name string, tool Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check for conflicts
	if _, exists := r.legacyFactories[name]; exists {
		return fmt.Errorf("tool '%s' is already registered as legacy factory", name)
	}
	if _, exists := r.typedFactories[name]; exists {
		return fmt.Errorf("tool '%s' is already registered as typed factory", name)
	}

	// Create factory that returns the tool instance
	factory := func() Tool { return tool }

	// Get metadata from tool
	metadata := tool.GetMetadata()

	// Create typed factory
	typedFactory := NewStronglyTypedFactory(factory, metadata.Name, metadata)

	r.typedFactories[name] = typedFactory
	r.registrationOrder = append(r.registrationOrder, name)

	return nil
}

// Note: RegisterWithBuilder moved to helper function due to Go generic limitation

// GetTool retrieves a tool instance (creates it if needed)
func (r *StandardToolRegistry) GetTool(name string) (Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Try typed factories first (preferred)
	if factory, exists := r.typedFactories[name]; exists {
		return factory.Create(), nil
	}

	// Fallback to legacy factories
	if factory, exists := r.legacyFactories[name]; exists {
		return factory(), nil
	}

	return nil, fmt.Errorf("tool '%s' not found", name)
}

// GetToolInfo returns detailed information about a registered tool
func (r *StandardToolRegistry) GetToolInfo(name string) (*StandardToolInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var metadata ToolMetadata
	var toolType string

	// Try typed factories first
	if factory, exists := r.typedFactories[name]; exists {
		metadata = factory.GetMetadata()
		toolType = factory.GetType()
	} else if factory, exists := r.legacyFactories[name]; exists {
		// Create instance to get metadata
		tool := factory()
		if metadataProvider, ok := tool.(interface{ GetMetadata() ToolMetadata }); ok {
			metadata = metadataProvider.GetMetadata()
			toolType = "legacy"
		} else {
			return nil, fmt.Errorf("tool '%s' does not provide metadata", name)
		}
	} else {
		return nil, fmt.Errorf("tool '%s' not found", name)
	}

	return &StandardToolInfo{
		Name:         metadata.Name,
		Type:         toolType,
		Category:     metadata.Category,
		Description:  metadata.Description,
		Version:      metadata.Version,
		Dependencies: metadata.Dependencies,
		Capabilities: metadata.Capabilities,
	}, nil
}

// Clear removes all registered tools (useful for testing)
func (r *StandardToolRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.legacyFactories = make(map[string]ToolFactory)
	r.typedFactories = make(map[string]StronglyTypedToolFactory[Tool])
	r.registrationOrder = nil
}

// Count returns the total number of registered tools
func (r *StandardToolRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.legacyFactories) + len(r.typedFactories)
}

// IsRegistered checks if a tool is registered
func (r *StandardToolRegistry) IsRegistered(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, legacyExists := r.legacyFactories[name]
	_, typedExists := r.typedFactories[name]

	return legacyExists || typedExists
}
