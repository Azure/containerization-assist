// Package handlers provides a unified set of direct handlers for Container Kit MCP,
// replacing the complex CQRS pattern with simple, direct request handlers.
package handlers

import (
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/domain/events"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/core/resources"
	"github.com/Azure/container-kit/pkg/mcp/service/session"
)

// Handlers provides a unified interface to all application handlers
type Handlers struct {
	Workflow *WorkflowHandler
	Status   *StatusHandler
}

// NewHandlers creates a new set of handlers with all dependencies
func NewHandlers(
	orchestrator workflow.EventAwareOrchestrator,
	sessionManager session.SessionManager,
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
	SessionManager session.SessionManager
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
