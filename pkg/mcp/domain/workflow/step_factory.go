// Package workflow provides step factory for creating workflow steps.
package workflow

import (
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/infrastructure/ml"
)

// StepFactory creates workflow steps with optional optimizations
type StepFactory struct {
	optimizedBuild *ml.OptimizedBuildStep
	logger         *slog.Logger
}

// NewStepFactory creates a new step factory
func NewStepFactory(optimizedBuild *ml.OptimizedBuildStep, logger *slog.Logger) *StepFactory {
	return &StepFactory{
		optimizedBuild: optimizedBuild,
		logger:         logger,
	}
}

// CreateBuildStep creates either an optimized or standard build step
func (f *StepFactory) CreateBuildStep() Step {
	if f.optimizedBuild != nil {
		f.logger.Info("Creating optimized build step with AI-powered resource prediction")
		return NewOptimizedBuildStep(f.optimizedBuild)
	}
	f.logger.Info("Creating standard build step")
	return NewBuildStep()
}

// CreateAllSteps creates all workflow steps with appropriate optimizations
func (f *StepFactory) CreateAllSteps() []Step {
	return []Step{
		NewAnalyzeStep(),
		NewDockerfileStep(),
		f.CreateBuildStep(), // Use factory to create appropriate build step
		NewScanStep(),
		NewTagStep(),
		NewPushStep(),
		NewManifestStep(),
		NewClusterStep(),
		NewDeployStep(),
		NewVerifyStep(),
	}
}
