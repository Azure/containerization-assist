// Package pipeline provides operations for container build, deployment, and management
package pipeline

import (
	"log/slog"

	sessionsvc "github.com/Azure/container-kit/pkg/mcp/domain/session"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/domain/types"
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
