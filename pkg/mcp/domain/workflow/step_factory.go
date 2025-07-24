package workflow

import (
	"fmt"
	"log/slog"
)

type StepFactory struct {
	stepProvider       StepProvider
	optimizedBuildStep Step // Pre-created optimized build step from infrastructure
	logger             *slog.Logger
}

func NewStepFactory(stepProvider StepProvider, optimizedBuildStep Step, logger *slog.Logger) *StepFactory {
	return &StepFactory{
		stepProvider:       stepProvider,
		optimizedBuildStep: optimizedBuildStep,
		logger:             logger,
	}
}

func (f *StepFactory) CreateBuildStep() (Step, error) {
	if f.optimizedBuildStep != nil {
		f.logger.Info("Creating optimized build step with AI-powered resource prediction")
		return f.optimizedBuildStep, nil
	}
	f.logger.Info("Creating standard build step")
	return f.getStep(StepBuildImage)
}

func (f *StepFactory) getStep(name string) (Step, error) {
	step, err := f.stepProvider.GetStep(name)
	if err != nil {
		return nil, fmt.Errorf("step %s not found: %w", name, err)
	}
	return step, nil
}

func (f *StepFactory) CreateAllSteps() ([]Step, error) {
	if f.stepProvider == nil {
		f.logger.Warn("Step provider is nil, creating empty step list")
		return []Step{}, nil
	}

	stepNames := []string{
		StepAnalyzeRepository,
		StepGenerateDockerfile,
		StepSecurityScan,
		StepTagImage,
		StepPushImage,
		StepGenerateManifests,
		StepSetupCluster,
		StepDeployApplication,
		StepVerifyDeployment,
	}

	steps := make([]Step, 0, len(stepNames)+1) // +1 for build step

	// Add regular steps
	for _, stepName := range stepNames {
		step, err := f.getStep(stepName)
		if err != nil {
			return nil, fmt.Errorf("failed to create step %s: %w", stepName, err)
		}

		// Insert build step at position 2 (after dockerfile, before security_scan)
		if stepName == StepGenerateDockerfile {
			buildStep, err := f.CreateBuildStep()
			if err != nil {
				return nil, fmt.Errorf("failed to create build step: %w", err)
			}
			steps = append(steps, step, buildStep)
		} else {
			steps = append(steps, step)
		}
	}

	return steps, nil
}
