// Package handlers provides a unified set of direct handlers for Containerization Assist MCP,
// replacing the complex CQRS pattern with simple, direct request handlers.
package handlers

import (
	"log/slog"

	"github.com/Azure/containerization-assist/pkg/mcp/domain/events"
	"github.com/Azure/containerization-assist/pkg/mcp/domain/workflow"
	"github.com/Azure/containerization-assist/pkg/mcp/infrastructure/core/resources"
	"github.com/Azure/containerization-assist/pkg/mcp/service/session"
)

// Handlers provides a unified interface to all application handlers
type Handlers struct {
	Workflow *WorkflowHandler
	Status   *StatusHandler
}

// NewHandlers creates a new set of handlers with all dependencies
func NewHandlers(
	orchestrator workflow.EventAwareOrchestrator,
	sessionManager session.OptimizedSessionManager,
	resourceStore *resources.Store,
	eventPublisher *events.Publisher,
	logger *slog.Logger,
) *Handlers {
	return &Handlers{
		Workflow: NewWorkflowHandler(orchestrator, sessionManager, eventPublisher, logger),
		Status:   NewStatusHandler(sessionManager, resourceStore, logger),
	}
}

// Dependencies represents the required dependencies for handlers
type Dependencies struct {
	Orchestrator   workflow.EventAwareOrchestrator
	SessionManager session.OptimizedSessionManager
	ResourceStore  *resources.Store
	EventPublisher *events.Publisher
	Logger         *slog.Logger
}

// NewHandlersFromDeps creates handlers from a dependencies struct
func NewHandlersFromDeps(deps Dependencies) *Handlers {
	return NewHandlers(
		deps.Orchestrator,
		deps.SessionManager,
		deps.ResourceStore,
		deps.EventPublisher,
		deps.Logger,
	)
}
