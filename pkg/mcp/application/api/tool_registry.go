// Package api provides the single source of truth for all MCP interfaces.
// This file contains the unified ToolRegistry interface.
package api

import (
	"context"
)

// ToolRegistry defines unified tool registration and discovery.
// This interface replaces all existing registry implementations with a unified approach.
type ToolRegistry interface {
	// Register registers a tool factory function.
	// The factory function creates instances of the tool when needed.
	Register(name string, factory interface{}) error

	// Discover finds tools by name and returns the factory result.
	// Returns an error if the tool is not found.
	Discover(name string) (interface{}, error)

	// List returns all registered tool names.
	List() []string

	// Metadata returns tool metadata.
	// Returns an error if the tool is not found.
	Metadata(name string) (ToolMetadata, error)

	// SetMetadata updates tool metadata.
	// Returns an error if the tool is not found.
	SetMetadata(name string, metadata ToolMetadata) error

	// Unregister removes a tool from the registry.
	// Returns an error if the tool is not found.
	Unregister(name string) error

	// Execute runs a tool by name with the given input.
	// This method provides compatibility with the existing Registry interface.
	Execute(ctx context.Context, name string, input ToolInput) (ToolOutput, error)

	// Close releases all resources used by the registry.
	Close() error
}
