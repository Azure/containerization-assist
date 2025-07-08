// Package pipeline provides operations for container build, deployment, and management
package pipeline

import (
	"log/slog"

	mcptypes "github.com/Azure/container-kit/pkg/mcp/core"
	sessionsvc "github.com/Azure/container-kit/pkg/mcp/session"
)

// NewOperations creates a new pipeline operations implementation
func NewOperations(
	sessionManager *sessionsvc.SessionManager,
	clients *mcptypes.MCPClients,
	logger *slog.Logger,
) *Operations {
	return createOperations(sessionManager, clients, logger)
}

// createOperations is the common creation logic that initializes the Operations struct
func createOperations(
	sessionManager *sessionsvc.SessionManager,
	clients *mcptypes.MCPClients,
	logger *slog.Logger,
) *Operations {
	ops := &Operations{
		sessionManager: sessionManager,
		clients:        clients,
		logger:         logger.With("component", "pipeline_operations"),
	}

	if clients != nil {
		ops.dockerClient = clients.Docker
	}

	return ops
}
