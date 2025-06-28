package build

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// BuildExecutorImpl implements the BuildExecutor interface
type BuildExecutorImpl struct {
	strategyManager *StrategyManager
	validator       BuildValidator
	activeBuilds    map[string]*activeBuild
	mu              sync.RWMutex
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
	validationStart := time.Now()
	if err := e.runValidation(buildCtx, result); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}
	result.Performance.ValidationTime = time.Since(validationStart)
	// Phase 2: Build
	buildStart := time.Now()
	buildResult, err := e.runBuild(ctx, buildCtx, strategy, buildID)
	if err != nil {
		return nil, fmt.Errorf("build failed: %w", err)
	}
	result.BuildResult = buildResult
	result.Performance.BuildTime = time.Since(buildStart)
	// Phase 3: Post-build analysis
	e.analyzePerformance(result)
	// Total duration
	result.Performance.TotalDuration = time.Since(startTime)
	e.logger.Info().
		Str("build_id", buildID).
		Bool("success", result.BuildResult.Success).
		Dur("duration", result.Performance.TotalDuration).
		Msg("Build execution completed")
	return result, nil
}

// ExecuteWithProgress runs a build with progress reporting
func (e *BuildExecutorImpl) ExecuteWithProgress(ctx context.Context, buildCtx BuildContext, strategy BuildStrategy, reporter ExtendedBuildReporter) (*ExecutionResult, error) {
	buildID := uuid.New().String()
	// Create cancellable context
	buildCtx2, cancel := context.WithCancel(ctx)
	defer cancel()
	// Register active build
	activeBuild := &activeBuild{
		ID:        buildID,
		Context:   buildCtx,
		Strategy:  strategy,
		Cancel:    cancel,
		StartTime: time.Now(),
		Status: &BuildStatus{
			BuildID:      buildID,
			State:        "starting",
			Progress:     0,
			CurrentStage: StageValidation,
			StartTime:    time.Now(),
		},
	}
	e.mu.Lock()
	e.activeBuilds[buildID] = activeBuild
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		delete(e.activeBuilds, buildID)
		e.mu.Unlock()
	}()
	// Execute with progress tracking
	reporter.ReportOverall(0, "Starting validation")
	result, err := e.executeWithProgressInternal(buildCtx2, buildCtx, strategy, activeBuild, reporter)
	if err != nil {
		reporter.ReportError(err)
		return nil, err
	}
	reporter.ReportOverall(100, "Build completed successfully")
	return result, nil
}

// Monitor monitors a running build
func (e *BuildExecutorImpl) Monitor(buildID string) (*BuildStatus, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	activeBuild, exists := e.activeBuilds[buildID]
	if !exists {
		return nil, fmt.Errorf("build %s not found", buildID)
	}
	// Return a copy of the status
	status := *activeBuild.Status
	return &status, nil
}

// Cancel cancels a running build
func (e *BuildExecutorImpl) Cancel(buildID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	activeBuild, exists := e.activeBuilds[buildID]
	if !exists {
		return fmt.Errorf("build %s not found", buildID)
	}
	e.logger.Info().Str("build_id", buildID).Msg("Cancelling build")
	// Cancel the build context
	activeBuild.Cancel()
	activeBuild.Status.State = "cancelled"
	return nil
}

// Internal execution methods
func (e *BuildExecutorImpl) runValidation(buildCtx BuildContext, result *ExecutionResult) error {
	e.logger.Debug().Msg("Running build validation")
	// Validate Dockerfile
	dockerfileResult, err := e.validator.ValidateDockerfile(buildCtx.DockerfilePath)
	if err != nil {
		return fmt.Errorf("failed to validate Dockerfile: %w", err)
	}
	result.ValidationResult = dockerfileResult
	if !dockerfileResult.Valid {
		return fmt.Errorf("Dockerfile validation failed with %d errors", len(dockerfileResult.Errors))
	}
	// Validate build context
	contextResult, err := e.validator.ValidateBuildContext(buildCtx)
	if err != nil {
		return fmt.Errorf("failed to validate build context: %w", err)
	}
	// Merge validation results
	result.ValidationResult.Warnings = append(result.ValidationResult.Warnings, contextResult.Warnings...)
	result.ValidationResult.Info = append(result.ValidationResult.Info, contextResult.Info...)
	// Security validation
	securityResult, err := e.validator.ValidateSecurityRequirements(buildCtx.DockerfilePath)
	if err != nil {
		return fmt.Errorf("failed to validate security: %w", err)
	}
	result.SecurityResult = securityResult
	return nil
}
func (e *BuildExecutorImpl) runBuild(ctx context.Context, buildCtx BuildContext, strategy BuildStrategy, buildID string) (*BuildResult, error) {
	e.logger.Info().
		Str("build_id", buildID).
		Str("strategy", strategy.Name()).
		Msg("Running build with strategy")
	// Execute the build
	buildResult, err := strategy.Build(buildCtx)
	if err != nil {
		return nil, err
	}
	return buildResult, nil
}
func (e *BuildExecutorImpl) executeWithProgressInternal(ctx context.Context, buildCtx BuildContext, strategy BuildStrategy, activeBuild *activeBuild, reporter ExtendedBuildReporter) (*ExecutionResult, error) {
	result := &ExecutionResult{
		Performance: &PerformanceMetrics{},
	}
	stages := []struct {
		name     string
		weight   float64
		executor func() error
	}{
		{
			name:   StageValidation,
			weight: 0.1,
			executor: func() error {
				return e.runValidation(buildCtx, result)
			},
		},
		{
			name:   StagePreBuild,
			weight: 0.1,
			executor: func() error {
				// Pre-build tasks
				reporter.ReportInfo("Preparing build environment")
				return nil
			},
		},
		{
			name:   StageBuild,
			weight: 0.7,
			executor: func() error {
				buildResult, err := e.runBuild(ctx, buildCtx, strategy, activeBuild.ID)
				if err != nil {
					return err
				}
				result.BuildResult = buildResult
				return nil
			},
		},
		{
			name:   StagePostBuild,
			weight: 0.1,
			executor: func() error {
				// Post-build tasks
				reporter.ReportInfo("Finalizing build artifacts")
				e.analyzePerformance(result)
				return nil
			},
		},
	}
	// Execute stages
	var completedWeight float64
	for _, stage := range stages {
		// Update status
		activeBuild.Status.CurrentStage = stage.name
		activeBuild.Status.State = "running"
		activeBuild.Status.Message = fmt.Sprintf("Executing %s", stage.name)
		// Report progress
		progress := completedWeight * 100
		reporter.ReportStage(progress, fmt.Sprintf("Starting %s", stage.name))
		// Execute stage
		stageStart := time.Now()
		if err := stage.executor(); err != nil {
			activeBuild.Status.State = "failed"
			activeBuild.Status.Message = err.Error()
			return nil, fmt.Errorf("%s failed: %w", stage.name, err)
		}
		// Update metrics
		switch stage.name {
		case StageValidation:
			result.Performance.ValidationTime = time.Since(stageStart)
		case StageBuild:
			result.Performance.BuildTime = time.Since(stageStart)
		}
		completedWeight += stage.weight
		// Check for cancellation
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("build cancelled")
		default:
		}
	}
	// Final status update
	activeBuild.Status.State = "completed"
	activeBuild.Status.Progress = 100
	activeBuild.Status.Message = "Build completed successfully"
	return result, nil
}
func (e *BuildExecutorImpl) analyzePerformance(result *ExecutionResult) {
	if result.BuildResult == nil {
		return
	}
	// Calculate cache efficiency
	totalOps := float64(result.BuildResult.CacheHits + result.BuildResult.CacheMisses)
	if totalOps > 0 {
		result.Performance.CacheUtilization = float64(result.BuildResult.CacheHits) / totalOps
	}
	// Estimate network and disk usage
	if result.BuildResult.ImageSizeBytes > 0 {
		result.Performance.DiskUsageMB = float64(result.BuildResult.ImageSizeBytes) / (1024 * 1024)
		// Rough estimate: network transfer is about 80% of final image size
		result.Performance.NetworkTransferMB = result.Performance.DiskUsageMB * 0.8
	}
	// Add artifacts
	if result.BuildResult.Success {
		result.Artifacts = append(result.Artifacts, BuildArtifact{
			Type: "docker-image",
			Name: result.BuildResult.FullImageRef,
			Size: result.BuildResult.ImageSizeBytes,
		})
	}
}

// Helper to create a simple progress reporter for testing
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
