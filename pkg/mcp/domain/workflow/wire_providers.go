// Package workflow provides Wire providers for workflow components.
package workflow

import (
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/domain/events"
	"github.com/Azure/container-kit/pkg/mcp/domain/saga"
)

// ProvideStepFactory creates a StepFactory with optional ML optimization
func ProvideStepFactory(stepProvider StepProvider, optimizer BuildOptimizer, optimizedBuildStep Step, logger *slog.Logger) *StepFactory {
	return NewStepFactory(stepProvider, optimizer, optimizedBuildStep, logger)
}

// ProvideOrchestrator creates a concrete Orchestrator
func ProvideOrchestrator(factory *StepFactory, progressFactory ProgressTrackerFactory, tracer Tracer, logger *slog.Logger) *Orchestrator {
	return NewOrchestratorWithFactory(factory, progressFactory, tracer, logger)
}

// ProvideEventOrchestrator creates an EventOrchestrator using decorators
func ProvideEventOrchestrator(orchestrator *Orchestrator, publisher *events.Publisher) EventAwareOrchestrator {
	// Use the decorator pattern
	return WithEvents(orchestrator, publisher)
}

// ProvideSagaOrchestrator creates a SagaOrchestrator using decorators
func ProvideSagaOrchestrator(eventOrchestrator EventAwareOrchestrator, sagaCoordinator *saga.SagaCoordinator, logger *slog.Logger) SagaAwareOrchestrator {
	// Use the decorator pattern
	return WithSaga(eventOrchestrator, sagaCoordinator, logger)
}

// ProvideWorkflowOrchestrator provides the base workflow orchestrator as an interface
func ProvideWorkflowOrchestrator(orchestrator *Orchestrator) WorkflowOrchestrator {
	return orchestrator
}
