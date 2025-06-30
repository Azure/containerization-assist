package build

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/core"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
)

// BuildExecutorService handles the execution of Docker builds with progress reporting
type BuildExecutorService struct {
	pipelineAdapter mcptypes.PipelineOperations
	sessionManager  core.ToolSessionManager
	logger          zerolog.Logger
	analyzer        *BuildAnalyzer
	validator       *BuildValidatorImpl
	troubleshooter  *BuildTroubleshooter
	securityScanner *BuildSecurityScanner
	optimizer       *BuildOptimizer
	perfMonitor     *PerformanceMonitor
}

// NewBuildExecutor creates a new build executor
func NewBuildExecutor(adapter mcptypes.PipelineOperations, sessionManager core.ToolSessionManager, logger zerolog.Logger) *BuildExecutorService {
	executorLogger := logger.With().Str("component", "build_executor").Logger()
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

// ExecuteWithFixes runs the atomic Docker image build with AI-driven fixing capabilities
func (e *BuildExecutorService) ExecuteWithFixes(ctx context.Context, args AtomicBuildImageArgs, fixingMixin interface{}) (*AtomicBuildImageResult, error) {
	// Check if fixing is enabled
	if fixingMixin == nil {
		e.logger.Warn().Msg("AI-driven fixing not enabled, falling back to regular execution")
		startTime := time.Now()
		result := &AtomicBuildImageResult{
			BaseToolResponse:    types.NewBaseResponse("atomic_build_image", args.SessionID, args.DryRun),
			BaseAIContextResult: mcptypes.NewBaseAIContextResult("build", false, 0),
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
		return nil, fmt.Errorf("session ID is required")
	}
	if args.ImageName == "" {
		return nil, fmt.Errorf("image name is required")
	}
	// Get session and workspace info
	startTime := time.Now()
	sessionInterface, err := e.sessionManager.GetSession(args.SessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	session, ok := sessionInterface.(*session.SessionState)
	if !ok {
		return nil, fmt.Errorf("invalid session type")
	}
	// Prepare workspace paths
	workspaceDir := session.WorkspaceDir

	// Initialize result structure
	result := &AtomicBuildImageResult{
		SessionID:           args.SessionID,
		ImageName:           args.ImageName,
		ImageTag:            e.getImageTag(args.ImageTag),
		Platform:            e.getPlatform(args.Platform),
		BuildContext_Info:   &BuildContextInfo{},
		Success:             false,
		BaseToolResponse:    types.NewBaseResponse("atomic_build_image", args.SessionID, args.DryRun),
		BaseAIContextResult: mcptypes.NewBaseAIContextResult("build", false, 0),
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

// executeWithoutProgress executes the build without progress tracking
func (e *BuildExecutorService) executeWithoutProgress(ctx context.Context, args AtomicBuildImageArgs, result *AtomicBuildImageResult, startTime time.Time) (*AtomicBuildImageResult, error) {
	// Get session
	sessionInterface, err := e.sessionManager.GetSession(args.SessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	session, ok := sessionInterface.(*session.SessionState)
	if !ok {
		return nil, fmt.Errorf("invalid session type")
	}

	// Set workspace and paths
	workspaceDir := session.WorkspaceDir
	result.WorkspaceDir = workspaceDir
	result.BuildContext = e.getBuildContext(args.BuildContext, workspaceDir)
	result.DockerfilePath = e.getDockerfilePath(args.DockerfilePath, result.BuildContext)
	result.FullImageRef = fmt.Sprintf("%s:%s", args.ImageName, result.ImageTag)

	// Analyze build context
	if e.analyzer != nil {
		if err := e.analyzer.AnalyzeBuildContext(result); err != nil {
			return nil, fmt.Errorf("build context analysis failed: %w", err)
		}
	}

	// Validate build prerequisites
	if e.validator != nil {
		if err := e.validator.ValidateBuildPrerequisites(result.DockerfilePath, result.BuildContext); err != nil {
			result.Success = false
			result.TotalDuration = time.Since(startTime)
			return result, fmt.Errorf("build prerequisites validation failed: %w", err)
		}
	}

	// Optimize build if optimizer is available
	optimizationResult := &OptimizationResult{}
	if e.optimizer != nil && !args.DryRun {
		e.logger.Info().Msg("Running build optimization analysis")
		optResult, err := e.optimizer.OptimizeBuild(ctx, result.DockerfilePath, result.BuildContext)
		if err != nil {
			e.logger.Warn().Err(err).Msg("Build optimization analysis failed, continuing with standard build")
		} else {
			optimizationResult = optResult
			// Log optimization recommendations
			for _, rec := range optResult.Recommendations {
				e.logger.Info().
					Str("type", rec.Type).
					Str("priority", rec.Priority).
					Str("title", rec.Title).
					Msg("Build optimization recommendation")
			}
		}
	}

	// Build the image
	buildStartTime := time.Now()

	// Start performance monitoring if available
	var buildMonitor *BuildMonitor
	if e.perfMonitor != nil {
		buildOp := &BuildOperation{
			Name:        fmt.Sprintf("build-%s:%s", args.ImageName, result.ImageTag),
			Tool:        "atomic_build_image",
			Type:        "docker",
			Strategy:    "standard",
			SessionID:   args.SessionID,
			ContextSize: result.BuildContext_Info.ContextSize,
		}
		buildMonitor = e.perfMonitor.StartBuildMonitoring(ctx, buildOp)
		defer func() {
			if buildMonitor != nil {
				imageInfo := &BuildImageInfo{
					Name:       args.ImageName,
					Tag:        result.ImageTag,
					Size:       0, // Would be populated from build result
					LayerCount: 0, // Would be populated from build result
				}
				buildMonitor.Complete(result.Success, "", imageInfo)
			}
		}()
	}

	buildArgs := map[string]interface{}{
		"dockerfilePath": result.DockerfilePath,
		"buildContext":   result.BuildContext,
		"imageName":      args.ImageName,
		"imageTag":       result.ImageTag,
	}

	// Apply optimization cache settings if available
	if len(optimizationResult.CacheStrategy.CacheFrom) > 0 {
		buildArgs["cacheFrom"] = optimizationResult.CacheStrategy.CacheFrom
	}
	if len(optimizationResult.CacheStrategy.CacheTo) > 0 {
		buildArgs["cacheTo"] = optimizationResult.CacheStrategy.CacheTo
	}

	_, err = e.pipelineAdapter.BuildImage(ctx, session.SessionID, buildArgs)
	result.BuildDuration = time.Since(buildStartTime)

	if err != nil {
		result.Success = false
		e.logger.Error().Err(err).Str("image_name", args.ImageName).Msg("Failed to build image")
		// Add troubleshooting tips
		if e.troubleshooter != nil {
			e.troubleshooter.AddTroubleshootingTips(result, err)
		}
		return result, nil
	}

	result.Success = true
	result.TotalDuration = time.Since(startTime)
	result.BaseAIContextResult = mcptypes.NewBaseAIContextResult("build", result.Success, result.TotalDuration)

	// Add optimization result to response
	if optimizationResult != nil && len(optimizationResult.Recommendations) > 0 {
		result.OptimizationResult = optimizationResult
	}

	// Get performance report if available
	if buildMonitor != nil {
		result.PerformanceReport = buildMonitor.GetReport()
	}

	// Push if requested
	if args.PushAfterBuild && args.RegistryURL != "" {
		pushStartTime := time.Now()
		pushArgs := map[string]interface{}{
			"imageRef":    result.FullImageRef,
			"registryURL": args.RegistryURL,
		}
		_, err = e.pipelineAdapter.PushImage(ctx, session.SessionID, pushArgs)
		result.PushDuration = time.Since(pushStartTime)
		if err != nil {
			e.logger.Error().Err(err).Msg("Failed to push image after build")
			// Build succeeded but push failed
			if e.troubleshooter != nil {
				e.troubleshooter.AddPushTroubleshootingTips(result, nil, args.RegistryURL, err)
			}
		}
	}

	return result, nil
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
