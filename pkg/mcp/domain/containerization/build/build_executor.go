package build

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"log/slog"

	coredocker "github.com/Azure/container-kit/pkg/core/docker"
	"github.com/Azure/container-kit/pkg/mcp/application/core"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/application/core"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
	sessiontypes "github.com/Azure/container-kit/pkg/mcp/domain/session"
	"github.com/Azure/container-kit/pkg/mcp/domain/types"
	"github.com/Azure/container-kit/pkg/mcp/services"
)

// BuildExecutorService handles the execution of Docker builds with progress reporting
type BuildExecutorService struct {
	pipelineAdapter mcptypes.TypedPipelineOperations
	sessionManager  sessiontypes.UnifiedSessionManager // DEPRECATED: use sessionStore and sessionState
	sessionStore    services.SessionStore
	sessionState    services.SessionState
	logger          *slog.Logger
	analyzer        *BuildAnalyzer
	validator       *BuildValidatorImpl
	troubleshooter  *BuildTroubleshooter
	securityScanner *BuildSecurityScanner
	optimizer       *BuildOptimizer
	perfMonitor     *PerformanceMonitor
}

// NewBuildExecutor creates a new build executor - DEPRECATED: use NewBuildExecutorWithServices
func NewBuildExecutor(adapter mcptypes.TypedPipelineOperations, sessionManager sessiontypes.UnifiedSessionManager, logger *slog.Logger) *BuildExecutorService {
	executorLogger := logger.With("component", "build_executor")
	return &BuildExecutorService{
		pipelineAdapter: adapter,
		sessionManager:  sessionManager,
		logger:          executorLogger,
		analyzer:        NewBuildAnalyzer(logger),
		validator:       NewBuildValidator(logger),
		troubleshooter:  NewBuildTroubleshooter(logger),
		securityScanner: NewBuildSecurityScanner(logger),
		optimizer:       NewBuildOptimizer(logger),
		perfMonitor:     NewPerformanceMonitor(logger),
	}
}

// NewBuildExecutorWithServices creates a new build executor using focused services
func NewBuildExecutorWithServices(adapter mcptypes.TypedPipelineOperations, sessionStore services.SessionStore, sessionState services.SessionState, logger *slog.Logger) *BuildExecutorService {
	executorLogger := logger.With("component", "build_executor")
	return &BuildExecutorService{
		pipelineAdapter: adapter,
		sessionStore:    sessionStore,
		sessionState:    sessionState,
		logger:          executorLogger,
		analyzer:        NewBuildAnalyzer(logger),
		validator:       NewBuildValidator(logger),
		troubleshooter:  NewBuildTroubleshooter(logger),
		securityScanner: NewBuildSecurityScanner(logger),
		optimizer:       NewBuildOptimizer(logger),
		perfMonitor:     NewPerformanceMonitor(logger),
	}
}

// ExecuteWithFixes runs the atomic Docker image build with AI-driven fixing capabilities
func (e *BuildExecutorService) ExecuteWithFixes(ctx context.Context, args AtomicBuildImageArgs, fixingMixin interface{}) (*AtomicBuildImageResult, error) {
	// Check if fixing is enabled
	if fixingMixin == nil {
		e.logger.Warn("AI-driven fixing not enabled, falling back to regular execution")
		startTime := time.Now()
		result := &AtomicBuildImageResult{
			BaseToolResponse:    types.BaseToolResponse{Success: false, Timestamp: time.Now()},
			BaseAIContextResult: core.NewBaseAIContextResult("build", false, 0),
			SessionID:           args.SessionID,
			ImageName:           args.ImageName,
			ImageTag:            e.getImageTag(args.ImageTag),
			Platform:            e.getPlatform(args.Platform),
			BuildContext_Info:   &BuildContextInfo{},
		}
		return e.executeWithoutProgress(ctx, args, result, startTime)
	}
	// First validate basic requirements
	if args.SessionID == "" {
		return nil, errors.NewError().Messagef("session ID is required").Build()
	}
	if args.ImageName == "" {
		return nil, errors.NewError().Messagef("image name is required").WithLocation(

		// Get session and workspace info
		).Build()
	}

	startTime := time.Now()
	sessionState, err := e.sessionManager.GetSession(ctx, args.SessionID)
	if err != nil {
		return nil, errors.NewError().Message("failed to get session").Cause(err).WithLocation(

		// Prepare workspace paths
		).Build()
	}

	workspaceDir := sessionState.WorkspaceDir

	// Initialize result structure
	result := &AtomicBuildImageResult{
		SessionID:           args.SessionID,
		ImageName:           args.ImageName,
		ImageTag:            e.getImageTag(args.ImageTag),
		Platform:            e.getPlatform(args.Platform),
		BuildContext_Info:   &BuildContextInfo{},
		Success:             false,
		BaseToolResponse:    types.BaseToolResponse{Success: false, Timestamp: time.Now()},
		BaseAIContextResult: core.NewBaseAIContextResult("build", false, 0),
		WorkspaceDir:        workspaceDir,
	}

	// Set paths
	result.BuildContext = e.getBuildContext(args.BuildContext, workspaceDir)
	result.DockerfilePath = e.getDockerfilePath(args.DockerfilePath, result.BuildContext)
	result.FullImageRef = fmt.Sprintf("%s:%s", args.ImageName, result.ImageTag)

	// Execute with progress tracking
	if err := e.executeWithProgress(ctx, args, result, startTime, nil); err != nil {
		return nil, err
	}
	return result, nil
}

// Helper methods
func (e *BuildExecutorService) getImageTag(tag string) string {
	if tag == "" {
		return "latest"
	}
	return tag
}
func (e *BuildExecutorService) getPlatform(platform string) string {
	if platform == "" {
		return "linux/amd64"
	}
	return platform
}
func (e *BuildExecutorService) getBuildContext(context, workspaceDir string) string {
	if context == "" {
		// Default to repo directory in workspace
		return filepath.Join(workspaceDir, "repo")
	}
	// If relative path, make it relative to workspace
	if !filepath.IsAbs(context) {
		return filepath.Join(workspaceDir, context)
	}
	return context
}
func (e *BuildExecutorService) getDockerfilePath(dockerfilePath, buildContext string) string {
	if dockerfilePath == "" {
		// Default to Dockerfile in build context
		return filepath.Join(buildContext, "Dockerfile")
	}
	// If relative path, make it relative to build context
	if !filepath.IsAbs(dockerfilePath) {
		return filepath.Join(buildContext, dockerfilePath)
	}
	return dockerfilePath
}

// buildExecutionContext contains the build execution state
type buildExecutionContext struct {
	sessionState       *sessiontypes.SessionState
	optimizationResult *OptimizationResult
	buildMonitor       *BuildMonitor
	args               AtomicBuildImageArgs
	result             *AtomicBuildImageResult
	startTime          time.Time
}

// executeWithoutProgress executes the build without progress tracking
func (e *BuildExecutorService) executeWithoutProgress(ctx context.Context, args AtomicBuildImageArgs, result *AtomicBuildImageResult, startTime time.Time) (*AtomicBuildImageResult, error) {
	buildCtx := &buildExecutionContext{
		args:      args,
		result:    result,
		startTime: startTime,
	}

	steps := []func(context.Context, *buildExecutionContext) error{
		e.initializeBuildContext,
		e.analyzeBuildContext,
		e.validateBuildPrerequisites,
		e.optimizeBuild,
		e.setupPerformanceMonitoring,
		e.executeBuild,
		e.handlePushIfRequested,
	}

	for _, step := range steps {
		if err := step(ctx, buildCtx); err != nil {
			return e.handleBuildError(buildCtx, err)
		}
	}

	return e.finalizeBuild(buildCtx), nil
}

// initializeBuildContext sets up the build context and paths
func (e *BuildExecutorService) initializeBuildContext(ctx context.Context, buildCtx *buildExecutionContext) error {
	sessionState, err := e.sessionManager.GetSession(ctx, buildCtx.args.SessionID)
	if err != nil {
		return errors.NewError().Message("failed to get session").Cause(err).WithLocation().Build()
	}

	buildCtx.sessionState = sessionState
	workspaceDir := sessionState.WorkspaceDir

	buildCtx.result.WorkspaceDir = workspaceDir
	buildCtx.result.BuildContext = e.getBuildContext(buildCtx.args.BuildContext, workspaceDir)
	buildCtx.result.DockerfilePath = e.getDockerfilePath(buildCtx.args.DockerfilePath, buildCtx.result.BuildContext)
	buildCtx.result.FullImageRef = fmt.Sprintf("%s:%s", buildCtx.args.ImageName, buildCtx.result.ImageTag)

	return nil
}

// analyzeBuildContext performs build context analysis
func (e *BuildExecutorService) analyzeBuildContext(_ context.Context, buildCtx *buildExecutionContext) error {
	if e.analyzer != nil {
		if err := e.analyzer.AnalyzeBuildContext(buildCtx.result); err != nil {
			return errors.NewError().Message("build context analysis failed").Cause(err).WithLocation().Build()
		}
	}
	return nil
}

// validateBuildPrerequisites validates build prerequisites
func (e *BuildExecutorService) validateBuildPrerequisites(_ context.Context, buildCtx *buildExecutionContext) error {
	if e.validator != nil {
		if err := e.validator.ValidateBuildPrerequisites(buildCtx.result.DockerfilePath, buildCtx.result.BuildContext); err != nil {
			return errors.NewError().Message("build prerequisites validation failed").Cause(err).WithLocation().Build()
		}
	}
	return nil
}

// optimizeBuild performs build optimization analysis
func (e *BuildExecutorService) optimizeBuild(ctx context.Context, buildCtx *buildExecutionContext) error {
	buildCtx.optimizationResult = &OptimizationResult{}

	if e.optimizer != nil && !buildCtx.args.DryRun {
		e.logger.Info("Running build optimization analysis")
		optResult, err := e.optimizer.OptimizeBuild(ctx, buildCtx.result.DockerfilePath, buildCtx.result.BuildContext)
		if err != nil {
			e.logger.Warn("Build optimization analysis failed, continuing with standard build", "error", err)
		} else {
			buildCtx.optimizationResult = optResult
			e.logOptimizationRecommendations(optResult)
		}
	}
	return nil
}

// setupPerformanceMonitoring initializes performance monitoring
func (e *BuildExecutorService) setupPerformanceMonitoring(ctx context.Context, buildCtx *buildExecutionContext) error {
	if e.perfMonitor != nil {
		buildOp := &BuildOperation{
			Name:        fmt.Sprintf("build-%s:%s", buildCtx.args.ImageName, buildCtx.result.ImageTag),
			Tool:        "atomic_build_image",
			Type:        "docker",
			Strategy:    "standard",
			SessionID:   buildCtx.args.SessionID,
			ContextSize: buildCtx.result.BuildContext_Info.ContextSize,
		}
		buildCtx.buildMonitor = e.perfMonitor.StartBuildMonitoring(ctx, buildOp)
	}
	return nil
}

// executeBuild performs the actual Docker build
func (e *BuildExecutorService) executeBuild(ctx context.Context, buildCtx *buildExecutionContext) error {
	buildStartTime := time.Now()
	defer func() {
		buildCtx.result.BuildDuration = time.Since(buildStartTime)
	}()

	buildParams := e.createBuildParams(buildCtx)
	_, err := e.pipelineAdapter.BuildImageTyped(ctx, buildCtx.sessionState.SessionID, buildParams)

	if err != nil {
		e.logger.Error("Failed to build image", "error", err, "image_name", buildCtx.args.ImageName)
		return err
	}

	return nil
}

// handlePushIfRequested handles image pushing if requested
func (e *BuildExecutorService) handlePushIfRequested(ctx context.Context, buildCtx *buildExecutionContext) error {
	if !buildCtx.args.PushAfterBuild || buildCtx.args.RegistryURL == "" {
		return nil
	}

	pushStartTime := time.Now()
	defer func() {
		buildCtx.result.PushDuration = time.Since(pushStartTime)
	}()

	pushParams := core.PushImageParams{
		ImageRef:   buildCtx.result.FullImageRef,
		Registry:   buildCtx.args.RegistryURL,
		Repository: "",
		Tag:        "",
	}

	_, err := e.pipelineAdapter.PushImageTyped(ctx, buildCtx.sessionState.SessionID, pushParams)
	if err != nil {
		e.logger.Error("Failed to push image after build", "error", err)
		if e.troubleshooter != nil {
			e.troubleshooter.AddPushTroubleshootingTips(buildCtx.result, nil, buildCtx.args.RegistryURL, err)
		}
	}

	return nil
}

// createBuildParams creates build parameters with optimization settings
func (e *BuildExecutorService) createBuildParams(buildCtx *buildExecutionContext) core.BuildImageParams {
	buildParams := core.BuildImageParams{
		SessionID:      buildCtx.sessionState.SessionID,
		DockerfilePath: buildCtx.args.DockerfilePath,
		ContextPath:    buildCtx.args.BuildContext,
		ImageName:      buildCtx.args.ImageName,
		Tags:           []string{buildCtx.args.ImageName},
		BuildArgs:      make(map[string]string),
		NoCache:        false,
		Pull:           true,
	}

	if buildCtx.args.BuildArgs != nil {
		for k, v := range buildCtx.args.BuildArgs {
			buildParams.BuildArgs[k] = fmt.Sprintf("%v", v)
		}
	}

	return buildParams
}

// logOptimizationRecommendations logs optimization recommendations
func (e *BuildExecutorService) logOptimizationRecommendations(optResult *OptimizationResult) {
	for _, rec := range optResult.Recommendations {
		e.logger.Info("Build optimization recommendation", "type", rec.Type, "priority", rec.Priority, "title", rec.Title)
	}
}

// handleBuildError handles build errors and generates failure analysis
func (e *BuildExecutorService) handleBuildError(buildCtx *buildExecutionContext, err error) (*AtomicBuildImageResult, error) {
	buildCtx.result.Success = false
	buildCtx.result.TotalDuration = time.Since(buildCtx.startTime)

	if e.troubleshooter != nil {
		e.troubleshooter.AddTroubleshootingTips(buildCtx.result, err)

		buildResult := &coredocker.BuildResult{
			Success: false,
			Error: &coredocker.BuildError{
				Message: err.Error(),
				Type:    "build_error",
			},
		}
		buildCtx.result.BuildFailureAnalysis = e.troubleshooter.GenerateBuildFailureAnalysis(err, buildResult, buildCtx.result)
	}

	return buildCtx.result, nil
}

// finalizeBuild completes the build process
func (e *BuildExecutorService) finalizeBuild(buildCtx *buildExecutionContext) *AtomicBuildImageResult {
	buildCtx.result.Success = true
	buildCtx.result.TotalDuration = time.Since(buildCtx.startTime)
	buildCtx.result.BaseAIContextResult = core.NewBaseAIContextResult("build", buildCtx.result.Success, buildCtx.result.TotalDuration)

	// Complete performance monitoring
	if buildCtx.buildMonitor != nil {
		imageInfo := &BuiltImageInfo{
			Name:       buildCtx.args.ImageName,
			Tag:        buildCtx.result.ImageTag,
			Size:       0,
			LayerCount: 0,
		}
		buildCtx.buildMonitor.Complete(buildCtx.result.Success, "", imageInfo)
		buildCtx.result.PerformanceReport = buildCtx.buildMonitor.GetReport()
	}

	// Add optimization result to response
	if buildCtx.optimizationResult != nil && len(buildCtx.optimizationResult.Recommendations) > 0 {
		buildCtx.result.OptimizationResult = buildCtx.optimizationResult
	}

	return buildCtx.result
}

// executeWithProgress executes the build with progress tracking
func (e *BuildExecutorService) executeWithProgress(ctx context.Context, args AtomicBuildImageArgs, result *AtomicBuildImageResult, startTime time.Time, progress interface{}) error {
	// For now, just call executeWithoutProgress
	result, err := e.executeWithoutProgress(ctx, args, result, startTime)
	if err != nil {
		return err
	}
	return nil
}
