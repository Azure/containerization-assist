package build

import (
	"context"
	"time"

	errors "github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// BuildExecutorImpl implements the BuildExecutor interface
type BuildExecutorImpl struct {
	strategyManager *StrategyManager
	validator       BuildValidator
	activeBuilds    map[string]*activeBuild
	logger          zerolog.Logger
}

// activeBuild represents an active build process
type activeBuild struct {
	ID        string
	Context   BuildContext
	Strategy  BuildStrategy
	Status    *BuildStatus
	Cancel    context.CancelFunc
	StartTime time.Time
}

// NewBuildExecutor creates a new build executor
func NewBuildExecutorImpl(strategyManager *StrategyManager, validator BuildValidator, logger zerolog.Logger) *BuildExecutorImpl {
	return &BuildExecutorImpl{
		strategyManager: strategyManager,
		validator:       validator,
		activeBuilds:    make(map[string]*activeBuild),
		logger:          logger.With().Str("component", "build_executor").Logger(),
	}
}

// Execute runs a build with the selected strategy
func (e *BuildExecutorImpl) Execute(ctx context.Context, buildCtx BuildContext, strategy BuildStrategy) (*ExecutionResult, error) {
	startTime := time.Now()
	buildID := uuid.New().String()
	e.logger.Info().
		Str("build_id", buildID).
		Str("image", buildCtx.ImageName).
		Str("strategy", strategy.Name()).
		Msg("Starting build execution")
	// Initialize result
	result := &ExecutionResult{
		Performance: &PerformanceMetrics{
			TotalDuration: 0,
		},
	}
	// Phase 1: Validation
	if err := e.runValidation(buildCtx, result); err != nil {
		return nil, errors.NewError().Message("validation failed").Cause(err).WithLocation(

		// Update total duration
		).Build()
	}

	result.Performance.TotalDuration = time.Since(startTime)

	return result, nil
}

// runValidation performs validation steps for the build
func (e *BuildExecutorImpl) runValidation(buildCtx BuildContext, result *ExecutionResult) error {
	// Basic validation logic - can be expanded
	if buildCtx.DockerfilePath == "" {
		return errors.NewError().Messagef("dockerfile path is required").WithLocation().Build()
	}
	if buildCtx.ImageName == "" {
		return errors.NewError().Messagef("image name is required").WithLocation().Build(

		// Helper to create a simple progress reporter for testing
		)
	}
	return nil
}

type SimpleProgressReporter struct {
	logger zerolog.Logger
}

func NewSimpleProgressReporter(logger zerolog.Logger) *SimpleProgressReporter {
	return &SimpleProgressReporter{logger: logger}
}
func (r *SimpleProgressReporter) ReportProgress(progress float64, stage string, message string) {
	r.logger.Info().
		Float64("progress", progress).
		Str("stage", stage).
		Msg(message)
}
func (r *SimpleProgressReporter) ReportError(err error) {
	r.logger.Error().Err(err).Msg("Build error")
}
func (r *SimpleProgressReporter) ReportWarning(message string) {
	r.logger.Warn().Msg(message)
}
func (r *SimpleProgressReporter) ReportInfo(message string) {
	r.logger.Info().Msg(message)
}
