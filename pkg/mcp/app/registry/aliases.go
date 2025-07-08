package registry

import (
	"github.com/Azure/container-kit/pkg/mcp/application/api"
)

// Compatibility aliases for backward compatibility during migration
// TODO: Remove these after full migration to unified registry

// Legacy registry type aliases
type TypedToolRegistry = Registry
type FederatedRegistry = Registry
type ToolRegistry = Registry
type MemoryRegistry = Registry
type MemoryToolRegistry = Registry

// Legacy constructor aliases
func NewTypedToolRegistry(opts ...Option) *Registry { return New(opts...) }
func NewFederatedRegistry(opts ...Option) *Registry { return New(opts...) }
func NewToolRegistry(opts ...Option) *Registry { return New(opts...) }
func NewMemoryRegistry(opts ...Option) *Registry { return New(opts...) }
func NewMemoryToolRegistry(opts ...Option) *Registry { return New(opts...) }

// Service interface compatibility
// RegisterTool adapts the api.Tool interface to the services.ToolRegistry interface
func (r *Registry) RegisterTool(tool api.Tool, opts ...api.RegistryOption) error {
	return r.Register(tool, opts...)
}

// UnregisterTool adapts the Unregister method to the services.ToolRegistry interface
func (r *Registry) UnregisterTool(name string) error {
	return r.Unregister(name)
}

// GetTool adapts the Get method to the services.ToolRegistry interface
func (r *Registry) GetTool(name string) (api.Tool, error) {
	return r.Get(name)
}

// ListTools adapts the List method to the services.ToolRegistry interface
func (r *Registry) ListTools() []string {
	return r.List()
}

// GetMetadata is already compatible with services.ToolRegistry interface
// No adapter needed