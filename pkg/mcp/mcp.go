// Package mcp provides a minimal public API surface for the MCP server.
// Only essential types and functions are exposed publicly.
//
// This package exposes:
//   - Server: The main MCP server type
//   - ServerConfig: Server configuration
//   - ConversationConfig: Conversation mode configuration
//   - NewServer: Server constructor
//   - DefaultServerConfig: Default configuration factory
//
// All other types and implementation details are internal.
package mcp

import (
	"github.com/Azure/container-copilot/pkg/mcp/internal/core"
)

// Essential Public API Types

// Server represents the MCP server.
// Use NewServer() to create a new instance.
type Server = core.Server

// ServerConfig holds configuration for the MCP server.
// Use DefaultServerConfig() to get default values.
type ServerConfig = core.ServerConfig

// ConversationConfig holds configuration for conversation mode.
// Used with Server.EnableConversationMode().
type ConversationConfig = core.ConversationConfig

// Essential Public API Functions

// NewServer creates a new MCP server with the given configuration.
// This is the primary entry point for creating MCP servers.
func NewServer(config ServerConfig) (*Server, error) {
	return core.NewServer(config)
}

// DefaultServerConfig returns a default server configuration.
// Modify the returned config as needed before passing to NewServer().
func DefaultServerConfig() ServerConfig {
	return core.DefaultServerConfig()
}
