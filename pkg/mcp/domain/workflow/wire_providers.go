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

// ProvideBaseOrchestrator creates a concrete BaseOrchestrator
func ProvideBaseOrchestrator(factory *StepFactory, progressFactory ProgressTrackerFactory, logger *slog.Logger, tracer Tracer) *BaseOrchestrator {
	// Create base orchestrator with common middleware
	middlewares := []StepMiddleware{
		RetryMiddleware(),
		ProgressMiddleware(),
	}

	// Add tracing middleware if tracer is available
	if tracer != nil {
		middlewares = append([]StepMiddleware{TracingMiddleware(tracer)}, middlewares...)
	}

	return NewBaseOrchestrator(factory, progressFactory, logger, middlewares...)
}

// ProvideEventOrchestrator creates an EventOrchestrator using decorators
func ProvideEventOrchestrator(orchestrator *BaseOrchestrator, publisher *events.Publisher) EventAwareOrchestrator {
	// Use the decorator pattern to add event awareness
	return WithEvents(orchestrator, publisher)
}

// ProvideSagaOrchestrator creates a SagaOrchestrator using decorators
func ProvideSagaOrchestrator(eventOrchestrator EventAwareOrchestrator, sagaCoordinator *saga.SagaCoordinator, containerManager ContainerManager, deploymentManager DeploymentManager, logger *slog.Logger) SagaAwareOrchestrator {
	// Use the decorator pattern to add saga support
	return WithSagaAndDependencies(eventOrchestrator, sagaCoordinator, containerManager, deploymentManager, logger)
}

// ProvideWorkflowOrchestrator provides the base workflow orchestrator as an interface
func ProvideWorkflowOrchestrator(orchestrator *BaseOrchestrator) WorkflowOrchestrator {
	return orchestrator
}
