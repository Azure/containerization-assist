// Package application provides dependency injection for the application layer
package application

import (
	"context"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/application/session"
	domainevents "github.com/Azure/container-kit/pkg/mcp/domain/events"
	domainml "github.com/Azure/container-kit/pkg/mcp/domain/ml"
	domainprompts "github.com/Azure/container-kit/pkg/mcp/domain/prompts"
	domainresources "github.com/Azure/container-kit/pkg/mcp/domain/resources"
	domainsampling "github.com/Azure/container-kit/pkg/mcp/domain/sampling"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/google/wire"
	"github.com/mark3labs/mcp-go/mcp"
)

// Providers provides simplified application layer dependencies
// NOTE: Most dependencies are now provided through composition layer
var Providers = wire.NewSet(
	// Core application services only
	ProvideDependencies,
	ProvideServer,
)

// ProvideDependencies aggregates all dependencies needed by the application layer
func ProvideDependencies(
	logger *slog.Logger,
	config workflow.ServerConfig,
	sessionManager session.OptimizedSessionManager,
	resourceStore domainresources.Store,
	progressEmitterFactory workflow.ProgressEmitterFactory,
	eventPublisher domainevents.Publisher,
	workflowOrchestrator workflow.WorkflowOrchestrator,
	errorPatternRecognizer domainml.ErrorPatternRecognizer,
	enhancedErrorHandler domainml.EnhancedErrorHandler,
	stepEnhancer domainml.StepEnhancer,
	samplingClient domainsampling.UnifiedSampler,
	promptManager domainprompts.Manager,
) *Dependencies {
	// Create EventAwareOrchestrator using a simple adapter
	eventAwareOrchestrator := &eventOrchestratorAdapter{
		base:      workflowOrchestrator,
		publisher: eventPublisher,
		logger:    logger,
	}

	return &Dependencies{
		Logger:                 logger,
		Config:                 config,
		SessionManager:         sessionManager,
		ResourceStore:          resourceStore,
		ProgressEmitterFactory: progressEmitterFactory,
		EventPublisher:         eventPublisher,
		WorkflowOrchestrator:   workflowOrchestrator,
		EventAwareOrchestrator: eventAwareOrchestrator,
		ErrorPatternRecognizer: errorPatternRecognizer,
		EnhancedErrorHandler:   enhancedErrorHandler,
		StepEnhancer:           stepEnhancer,
		SamplingClient:         samplingClient,
		PromptManager:          promptManager,
	}
}

// ProvideServer creates the MCP server implementation
func ProvideServer(deps *Dependencies) (api.MCPServer, error) {
	return NewMCPServerFromDeps(deps)
}

// eventOrchestratorAdapter is a fallback adapter when the base orchestrator is not a BaseOrchestrator
type eventOrchestratorAdapter struct {
	base      workflow.WorkflowOrchestrator
	publisher domainevents.Publisher
	logger    *slog.Logger
}

// Execute delegates to the base orchestrator
func (e *eventOrchestratorAdapter) Execute(ctx context.Context, req *mcp.CallToolRequest, args *workflow.ContainerizeAndDeployArgs) (*workflow.ContainerizeAndDeployResult, error) {
	return e.base.Execute(ctx, req, args)
}

// PublishWorkflowEvent publishes a workflow event using the event publisher
func (e *eventOrchestratorAdapter) PublishWorkflowEvent(ctx context.Context, workflowID string, eventType string, payload interface{}) error {
	// Create a simple event for publishing
	event := &adapterWorkflowEvent{
		workflowID: workflowID,
		eventType:  eventType,
		payload:    payload,
	}

	return e.publisher.Publish(ctx, event)
}

// adapterWorkflowEvent is a simple event implementation for the adapter
type adapterWorkflowEvent struct {
	workflowID string
	eventType  string
	payload    interface{}
}

func (e *adapterWorkflowEvent) EventID() string       { return "" }
func (e *adapterWorkflowEvent) EventType() string     { return e.eventType }
func (e *adapterWorkflowEvent) Timestamp() time.Time  { return time.Now() }
func (e *adapterWorkflowEvent) OccurredAt() time.Time { return time.Now() }
func (e *adapterWorkflowEvent) WorkflowID() string    { return e.workflowID }
func (e *adapterWorkflowEvent) Serialize() ([]byte, error) {
	return []byte("{}"), nil
}
