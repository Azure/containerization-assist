// Package workflow provides Wire providers for workflow components.
package workflow

import (
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/domain/events"
	"github.com/Azure/container-kit/pkg/mcp/domain/saga"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/ml"
)

// ProvideStepFactory creates a StepFactory with optional ML optimization
func ProvideStepFactory(optimizedBuild *ml.OptimizedBuildStep, logger *slog.Logger) *StepFactory {
	return NewStepFactory(optimizedBuild, logger)
}

// ProvideOptimizedOrchestrator creates an orchestrator with optimized steps
func ProvideOptimizedOrchestrator(factory *StepFactory, logger *slog.Logger) *Orchestrator {
	return NewOrchestratorWithFactory(factory, logger)
}

// ProvideWorkflowOrchestrator provides the base workflow orchestrator as an interface
func ProvideWorkflowOrchestrator(factory *StepFactory, logger *slog.Logger) WorkflowOrchestrator {
	return NewOrchestratorWithFactory(factory, logger)
}

// ProvideEventAwareOrchestrator provides the event-aware orchestrator as an interface
func ProvideEventAwareOrchestrator(orchestrator *Orchestrator, publisher *events.Publisher, logger *slog.Logger) EventAwareOrchestrator {
	return &EventOrchestrator{
		Orchestrator:   orchestrator,
		eventPublisher: publisher,
		eventUtils:     events.EventUtils{},
	}
}

// ProvideSagaAwareOrchestrator provides the saga-aware orchestrator as an interface
func ProvideSagaAwareOrchestrator(eventOrchestrator *EventOrchestrator, sagaCoordinator *saga.SagaCoordinator) SagaAwareOrchestrator {
	return &SagaOrchestrator{
		EventOrchestrator: eventOrchestrator,
		sagaCoordinator:   sagaCoordinator,
	}
}
