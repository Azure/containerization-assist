// Package application provides dependency injection for the application layer
package application

import (
	"context"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/common/runner"
	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/application/session"
	domainevents "github.com/Azure/container-kit/pkg/mcp/domain/events"
	domainml "github.com/Azure/container-kit/pkg/mcp/domain/ml"
	domainprompts "github.com/Azure/container-kit/pkg/mcp/domain/prompts"
	domainresources "github.com/Azure/container-kit/pkg/mcp/domain/resources"
	"github.com/Azure/container-kit/pkg/mcp/domain/saga"
	domainsampling "github.com/Azure/container-kit/pkg/mcp/domain/sampling"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/google/wire"
	"github.com/mark3labs/mcp-go/mcp"
)

// Providers provides all application layer dependencies
var Providers = wire.NewSet(
	// Grouped dependency providers
	ProvideCoreDeps,
	ProvideWorkflowDeps,
	ProvidePersistenceDeps,
	ProvideAIDeps,
	ProvideGroupedDependencies,

	// LLM configuration provider
	ProvideLLMConfig,

	// Legacy dependencies aggregator (for backward compatibility)
	ProvideDependencies,

	// Server implementation
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
	sagaCoordinator *saga.SagaCoordinator,
	workflowOrchestrator workflow.WorkflowOrchestrator,
	errorPatternRecognizer domainml.ErrorPatternRecognizer,
	enhancedErrorHandler domainml.EnhancedErrorHandler,
	stepEnhancer domainml.StepEnhancer,
	samplingClient domainsampling.UnifiedSampler,
	promptManager domainprompts.Manager,
) *Dependencies {
	// Create workflow dependencies to get the decorated orchestrators
	workflowDeps := ProvideWorkflowDeps(workflowOrchestrator, eventPublisher, progressEmitterFactory, sagaCoordinator, logger)

	return &Dependencies{
		Logger:                 logger,
		Config:                 config,
		SessionManager:         sessionManager,
		ResourceStore:          resourceStore,
		ProgressEmitterFactory: progressEmitterFactory,
		EventPublisher:         eventPublisher,
		SagaCoordinator:        sagaCoordinator,
		WorkflowOrchestrator:   workflowOrchestrator,
		EventAwareOrchestrator: workflowDeps.EventAwareOrchestrator,
		SagaAwareOrchestrator:  workflowDeps.SagaAwareOrchestrator,
		ErrorPatternRecognizer: errorPatternRecognizer,
		EnhancedErrorHandler:   enhancedErrorHandler,
		StepEnhancer:           stepEnhancer,
		SamplingClient:         samplingClient,
		PromptManager:          promptManager,
	}
}

// ProvideCoreDeps provides core system dependencies
func ProvideCoreDeps(logger *slog.Logger, config workflow.ServerConfig, runner runner.CommandRunner) CoreDeps {
	return CoreDeps{
		Logger: logger,
		Config: config,
		Runner: runner,
	}
}

// ProvideWorkflowDeps provides workflow orchestration dependencies
func ProvideWorkflowDeps(
	orchestrator workflow.WorkflowOrchestrator,
	eventPublisher domainevents.Publisher,
	progressEmitterFactory workflow.ProgressEmitterFactory,
	sagaCoordinator *saga.SagaCoordinator,
	logger *slog.Logger,
) WorkflowDeps {
	// Create EventAwareOrchestrator by wrapping the base orchestrator with event capabilities
	var eventAwareOrchestrator workflow.EventAwareOrchestrator
	if baseOrch, ok := orchestrator.(*workflow.BaseOrchestrator); ok {
		eventAwareOrchestrator = workflow.WithEvents(baseOrch, eventPublisher)
	} else {
		// If it's not a BaseOrchestrator, we'll need to create a wrapper
		logger.Warn("Could not create EventAwareOrchestrator: base orchestrator is not *BaseOrchestrator")
		eventAwareOrchestrator = &eventOrchestratorAdapter{
			base:      orchestrator,
			publisher: eventPublisher,
			logger:    logger,
		}
	}

	// Create SagaAwareOrchestrator by wrapping the EventAwareOrchestrator with saga capabilities
	sagaAwareOrchestrator := workflow.WithSaga(eventAwareOrchestrator, sagaCoordinator, logger)

	return WorkflowDeps{
		Orchestrator:           orchestrator,
		EventAwareOrchestrator: eventAwareOrchestrator,
		SagaAwareOrchestrator:  sagaAwareOrchestrator,
		EventPublisher:         eventPublisher,
		ProgressEmitterFactory: progressEmitterFactory,
		SagaCoordinator:        sagaCoordinator,
	}
}

// ProvidePersistenceDeps provides data persistence dependencies
func ProvidePersistenceDeps(
	sessionManager session.OptimizedSessionManager,
	resourceStore domainresources.Store,
) PersistenceDeps {
	return PersistenceDeps{
		SessionManager: sessionManager,
		ResourceStore:  resourceStore,
	}
}

// ProvideAIDeps provides AI/ML service dependencies
func ProvideAIDeps(
	samplingClient domainsampling.UnifiedSampler,
	promptManager domainprompts.Manager,
	errorPatternRecognizer domainml.ErrorPatternRecognizer,
	enhancedErrorHandler domainml.EnhancedErrorHandler,
	stepEnhancer domainml.StepEnhancer,
) AIDeps {
	return AIDeps{
		SamplingClient:         samplingClient,
		PromptManager:          promptManager,
		ErrorPatternRecognizer: errorPatternRecognizer,
		EnhancedErrorHandler:   enhancedErrorHandler,
		StepEnhancer:           stepEnhancer,
	}
}

// ProvideGroupedDependencies aggregates all dependency groups
func ProvideGroupedDependencies(
	core CoreDeps,
	workflow WorkflowDeps,
	persistence PersistenceDeps,
	ai AIDeps,
) *GroupedDependencies {
	return &GroupedDependencies{
		Core:        core,
		Workflow:    workflow,
		Persistence: persistence,
		AI:          ai,
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

// ProvideLLMConfig provides LLM configuration for the application
func ProvideLLMConfig() *LLMConfig {
	// Return default LLM configuration
	// This can be overridden by server options
	defaultConfig := DefaultLLMConfig()
	return &defaultConfig
}
