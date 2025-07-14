// Package wiring initializes the Wire-generated factories for the MCP server
package wiring

import (
	"github.com/Azure/container-kit/pkg/mcp/application"
)

// init connects the Wire-generated factories to the application layer
func init() {
	// Set server factories for basic and custom config initialization
	application.SetServerFactories(
		InitializeDefaultServer,
		InitializeServerWithConfig,
	)
}

// InitializeServerWithConfig is already defined in wire.go
// This ensures the factory signature matches application.ServerFactoryWithConfig
var _ application.ServerFactoryWithConfig = InitializeServerWithConfig
var _ application.ServerFactory = InitializeDefaultServer
