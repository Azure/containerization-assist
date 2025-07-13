// Package workflow provides Wire providers for workflow components.
package workflow

import (
	"log/slog"

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
