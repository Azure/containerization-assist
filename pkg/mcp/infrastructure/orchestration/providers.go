// Package orchestration provides unified dependency injection for container orchestration services
package orchestration

import (
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/domain/saga"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/orchestration/container"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/orchestration/kubernetes"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/orchestration/steps"
	"github.com/google/wire"
)

// Providers provides all orchestration domain dependencies
var Providers = wire.NewSet(
	// Container management
	container.NewDockerContainerManager,

	// Kubernetes deployment
	kubernetes.NewKubernetesDeploymentManager,

	// Workflow steps
	steps.NewRegistryStepProvider,
	ProvideStepFactory,

	// Orchestrators
	ProvideBaseOrchestrator,
	wire.Bind(new(workflow.WorkflowOrchestrator), new(*workflow.BaseOrchestrator)),

	// Saga coordination
	saga.NewSagaCoordinator,

	// Interface bindings would go here if needed
)

// ProvideStepFactory creates a workflow step factory
func ProvideStepFactory(stepProvider workflow.StepProvider, optimizer workflow.BuildOptimizer, logger *slog.Logger) *workflow.StepFactory {
	// For now, pass nil for optimized build step since it doesn't implement the interface
	return workflow.NewStepFactory(stepProvider, optimizer, nil, logger)
}

// ProvideBaseOrchestrator creates the base orchestrator
func ProvideBaseOrchestrator(factory *workflow.StepFactory, emitterFactory workflow.ProgressEmitterFactory, logger *slog.Logger) *workflow.BaseOrchestrator {
	return workflow.NewBaseOrchestrator(factory, emitterFactory, logger)
}
