// Package workflow provides step factory for creating workflow steps.
package workflow

import (
	"log/slog"
)

// StepFactory creates workflow steps with optional optimizations
type StepFactory struct {
	stepProvider       StepProvider
	optimizer          BuildOptimizer
	optimizedBuildStep Step // Pre-created optimized build step from infrastructure
	logger             *slog.Logger
}

// NewStepFactory creates a new step factory
func NewStepFactory(stepProvider StepProvider, optimizer BuildOptimizer, optimizedBuildStep Step, logger *slog.Logger) *StepFactory {
	return &StepFactory{
		stepProvider:       stepProvider,
		optimizer:          optimizer,
		optimizedBuildStep: optimizedBuildStep,
		logger:             logger,
	}
}

// CreateBuildStep creates either an optimized or standard build step
func (f *StepFactory) CreateBuildStep() Step {
	if f.optimizedBuildStep != nil {
		f.logger.Info("Creating optimized build step with AI-powered resource prediction")
		return f.optimizedBuildStep
	}
	f.logger.Info("Creating standard build step")
	return f.stepProvider.GetBuildStep()
}

// CreateAllSteps creates all workflow steps with appropriate optimizations
func (f *StepFactory) CreateAllSteps() []Step {
	if f.stepProvider == nil {
		f.logger.Warn("Step provider is nil, creating empty step list")
		return []Step{}
	}
	return []Step{
		f.stepProvider.GetAnalyzeStep(),
		f.stepProvider.GetDockerfileStep(),
		f.CreateBuildStep(), // Use factory to create appropriate build step
		f.stepProvider.GetScanStep(),
		f.stepProvider.GetTagStep(),
		f.stepProvider.GetPushStep(),
		f.stepProvider.GetManifestStep(),
		f.stepProvider.GetClusterStep(),
		f.stepProvider.GetDeployStep(),
		f.stepProvider.GetVerifyStep(),
	}
}
