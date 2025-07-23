// Package orchestration provides unified dependency injection for container orchestration services
package orchestration

import (
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/domain/ml"
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

	// Step enhancer adapter
	ProvideStepEnhancerAdapter,

	// Orchestrators
	ProvideDAGOrchestrator,
	wire.Bind(new(workflow.WorkflowOrchestrator), new(*workflow.DAGOrchestrator)),

	// Interface bindings would go here if needed
)

// ProvideStepFactory creates a workflow step factory
func ProvideStepFactory(stepProvider workflow.StepProvider, optimizer workflow.BuildOptimizer, logger *slog.Logger) *workflow.StepFactory {
	// For now, pass nil for optimized build step since it doesn't implement the interface
	return workflow.NewStepFactory(stepProvider, optimizer, nil, logger)
}

// ProvideDAGOrchestrator creates the DAG-based orchestrator
func ProvideDAGOrchestrator(
	stepProvider workflow.StepProvider,
	emitterFactory workflow.ProgressEmitterFactory,
	logger *slog.Logger,
) (*workflow.DAGOrchestrator, error) {
	return workflow.NewDAGOrchestrator(stepProvider, emitterFactory, logger)
}

// ProvideBaseOrchestrator creates the base orchestrator
func ProvideBaseOrchestrator(factory *workflow.StepFactory, emitterFactory workflow.ProgressEmitterFactory, logger *slog.Logger) *workflow.BaseOrchestrator {
	return workflow.NewBaseOrchestrator(factory, emitterFactory, logger)
}

// ProvideEnhancedOrchestrator creates the orchestrator with AI enhancement middleware
func ProvideEnhancedOrchestrator(
	factory *workflow.StepFactory,
	emitterFactory workflow.ProgressEmitterFactory,
	stepEnhancerAdapter workflow.StepEnhancer,
	logger *slog.Logger,
) *workflow.BaseOrchestrator {
	// Create orchestrator with AI enhancement middleware
	return workflow.NewBaseOrchestrator(
		factory,
		emitterFactory,
		logger,
		workflow.WithMiddleware(
			workflow.CombinedEnhancementMiddleware(stepEnhancerAdapter, logger),
			workflow.RetryMiddleware(),
			workflow.ProgressMiddleware(),
		),
	)
}

// ProvideAdaptiveOrchestrator creates the adaptive orchestrator with advanced AI capabilities
func ProvideAdaptiveOrchestrator(
	baseOrchestrator *workflow.BaseOrchestrator,
	patternRecognizer ml.ErrorPatternRecognizer,
	stepEnhancer ml.StepEnhancer,
	logger *slog.Logger,
) *AdaptiveOrchestratorAdapter {
	return NewAdaptiveOrchestratorAdapter(
		baseOrchestrator,
		patternRecognizer,
		stepEnhancer,
		logger,
	)
}
