package build

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	coredocker "github.com/Azure/container-kit/pkg/core/docker"
	sessiontypes "github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
	"github.com/localrivet/gomcp/server"
	"github.com/rs/zerolog"
)

// BuildExecutorService handles the execution of Docker builds with progress reporting
type BuildExecutorService struct {
	pipelineAdapter mcptypes.PipelineOperations
	sessionManager  mcptypes.ToolSessionManager
	logger          zerolog.Logger
	analyzer        *BuildAnalyzer
	troubleshooter  *BuildTroubleshooter
	securityScanner *BuildSecurityScanner
}

// NewBuildExecutor creates a new build executor
func NewBuildExecutor(adapter mcptypes.PipelineOperations, sessionManager mcptypes.ToolSessionManager, logger zerolog.Logger) *BuildExecutorService {
	executorLogger := logger.With().Str("component", "build_executor").Logger()
	return &BuildExecutorService{
		pipelineAdapter: adapter,
		sessionManager:  sessionManager,
		logger:          executorLogger,
		analyzer:        NewBuildAnalyzer(logger),
		troubleshooter:  NewBuildTroubleshooter(logger),
		securityScanner: NewBuildSecurityScanner(logger),
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
		return nil, mcptypes.NewErrorBuilder("SESSION_ID_REQUIRED", "Session ID is required", "validation_error").
			WithField("session_id", args.SessionID).
			WithOperation("build_image").
			WithStage("input_validation").
			WithImmediateStep(1, "Provide session ID", "Specify a valid session ID for the build operation").
			Build()
	}
	if args.ImageName == "" {
		return nil, mcptypes.NewErrorBuilder("IMAGE_NAME_REQUIRED", "Image name is required", "validation_error").
			WithField("image_name", args.ImageName).
			WithOperation("build_image").
			WithStage("input_validation").
			WithImmediateStep(1, "Provide image name", "Specify a Docker image name like 'myapp' or 'myregistry.com/myapp'").
			Build()
	}
	// Get session and workspace info
	sessionInterface, err := e.sessionManager.GetSession(args.SessionID)
	if err != nil {
		return nil, types.NewSessionError(args.SessionID, "build_image").
			WithStage("session_load").
			WithTool("build_image_atomic").
			WithRootCause("Session ID does not exist or has expired").
			WithCommand(2, "Create new session", "Create a new session if the current one is invalid", "analyze_repository --repo_path /path/to/repo", "New session created").
			Build()
	}
	session := sessionInterface.(*mcptypes.SessionState)
	workspaceDir := e.pipelineAdapter.GetSessionWorkspace(session.SessionID)
	buildContext := e.getBuildContext(args.BuildContext, workspaceDir)
	dockerfilePath := e.getDockerfilePath(args.DockerfilePath, buildContext)
	e.logger.Info().
		Str("session_id", args.SessionID).
		Str("image_name", args.ImageName).
		Str("dockerfile_path", dockerfilePath).
		Str("build_context", buildContext).
		Msg("Starting Docker build with AI-driven fixing")
	// Note: The actual fixing logic would be handled by the fixingMixin
	// This is a simplified version that just falls back to regular execution
	startTime := time.Now()
	result := &AtomicBuildImageResult{
		BaseToolResponse:    types.NewBaseResponse("atomic_build_image", args.SessionID, args.DryRun),
		BaseAIContextResult: mcptypes.NewBaseAIContextResult("build", false, 0), // Duration will be updated later
		SessionID:           args.SessionID,
		ImageName:           args.ImageName,
		ImageTag:            e.getImageTag(args.ImageTag),
		Platform:            e.getPlatform(args.Platform),
		BuildContext_Info:   &BuildContextInfo{},
	}
	return e.executeWithoutProgress(ctx, args, result, startTime)
}

// ExecuteWithContext executes the tool with GoMCP server context for native progress tracking
func (e *BuildExecutorService) ExecuteWithContext(serverCtx *server.Context, args AtomicBuildImageArgs) (*AtomicBuildImageResult, error) {
	startTime := time.Now()
	// Create result object early for error handling
	result := &AtomicBuildImageResult{
		BaseToolResponse:    types.NewBaseResponse("atomic_build_image", args.SessionID, args.DryRun),
		BaseAIContextResult: mcptypes.NewBaseAIContextResult("build", false, 0), // Duration will be updated later
		SessionID:           args.SessionID,
		ImageName:           args.ImageName,
		ImageTag:            e.getImageTag(args.ImageTag),
		Platform:            e.getPlatform(args.Platform),
		BuildContext_Info:   &BuildContextInfo{},
	}
	// Use centralized build stages for progress tracking
	// TODO: Move progress adapter to avoid import cycles
	// _ = internal.NewGoMCPProgressAdapter(serverCtx, []internal.LocalProgressStage{
	//	{Name: "Initialize", Weight: 0.10, Description: "Loading session and validating inputs"},
	//	{Name: "Build", Weight: 0.70, Description: "Building Docker image"},
	//	{Name: "Verify", Weight: 0.15, Description: "Verifying build results"},
	//	{Name: "Finalize", Weight: 0.05, Description: "Updating session state"},
	// })
	// Execute with progress tracking
	ctx := context.Background()
	err := e.executeWithProgress(ctx, args, result, startTime, nil)
	// Always set total duration
	result.TotalDuration = time.Since(startTime)
	// Complete progress tracking
	if err != nil {
		e.logger.Info().Msg("Build failed")
		result.Success = false
		return result, nil // Return result with error info, not the error itself
	} else {
		e.logger.Info().Msg("Build completed successfully")
	}
	return result, nil
}

// executeWithProgress handles the main execution with progress reporting
func (e *BuildExecutorService) executeWithProgress(ctx context.Context, args AtomicBuildImageArgs, result *AtomicBuildImageResult, startTime time.Time, reporter interface{}) error {
	// Stage 1: Initialize - Loading session and validating inputs
	e.logger.Info().Msg("Loading session")
	sessionInterface, err := e.sessionManager.GetSession(args.SessionID)
	if err != nil {
		e.logger.Error().Err(err).Str("session_id", args.SessionID).Msg("Failed to get session")
		return types.NewSessionError(args.SessionID, "build_image").
			WithStage("initialize").
			WithTool("build_image_atomic").
			WithField("image_name", args.ImageName).
			WithRootCause("Session ID does not exist or has expired").
			WithCommand(2, "Create new session", "Create a new session if the current one is invalid", "analyze_repository --repo_path /path/to/repo", "New session created").
			Build()
	}
	session := sessionInterface.(*mcptypes.SessionState)
	// Set session details
	result.SessionID = session.SessionID
	result.WorkspaceDir = e.pipelineAdapter.GetSessionWorkspace(session.SessionID)
	result.FullImageRef = fmt.Sprintf("%s:%s", result.ImageName, result.ImageTag)
	result.BuildContext = e.getBuildContext(args.BuildContext, result.WorkspaceDir)
	result.DockerfilePath = e.getDockerfilePath(args.DockerfilePath, result.BuildContext)
	e.logger.Info().Msg("Session initialized")
	// Handle dry-run
	if args.DryRun {
		result.BuildContext_Info.NextStepSuggestions = []string{
			"This is a dry-run - actual Docker image build would be performed",
			fmt.Sprintf("Would build image: %s", result.FullImageRef),
			fmt.Sprintf("Using Dockerfile: %s", result.DockerfilePath),
			fmt.Sprintf("Build context: %s", result.BuildContext),
		}
		result.Success = true
		e.logger.Info().Msg("Dry-run completed")
		return nil
	}
	// Stage 2: Analyze - Analyzing build context and dependencies
	e.logger.Info().Msg("Analyzing build context")
	if err := e.analyzer.AnalyzeBuildContext(result); err != nil {
		e.logger.Error().Err(err).
			Str("dockerfile_path", result.DockerfilePath).
			Str("build_context", result.BuildContext).
			Msg("Build context analysis failed")
		return mcptypes.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("build context analysis failed: %v", err), "filesystem_error")
	}
	e.logger.Info().Msg("Validating build prerequisites")
	if err := e.analyzer.ValidateBuildPrerequisites(result); err != nil {
		e.logger.Error().Err(err).
			Str("dockerfile_path", result.DockerfilePath).
			Str("build_context", result.BuildContext).
			Int64("context_size", result.BuildContext_Info.ContextSize).
			Msg("Build prerequisites validation failed")
		return mcptypes.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("build prerequisites validation failed: %v", err), "validation_error")
	}
	e.logger.Info().Msg("Analysis completed")
	// Stage 3: Build - Building Docker image
	e.logger.Info().Msg("Building Docker image")
	buildStartTime := time.Now()
	buildResult, err := e.pipelineAdapter.BuildDockerImage(
		session.SessionID, // Use compatibility method
		result.FullImageRef,
		result.DockerfilePath,
	)
	result.BuildDuration = time.Since(buildStartTime)
	// Convert from mcptypes.BuildResult to coredocker.BuildResult
	if buildResult != nil {
		result.BuildResult = &coredocker.BuildResult{
			Success:  buildResult.Success,
			ImageID:  buildResult.ImageID,
			ImageRef: buildResult.ImageRef,
			Duration: result.BuildDuration, // Use the duration we already calculated
		}
		if buildResult.Error != nil {
			result.BuildResult.Error = &coredocker.BuildError{
				Type:    buildResult.Error.Type,
				Message: buildResult.Error.Message,
			}
		}
	}
	if err != nil {
		e.logger.Error().Err(err).
			Str("image_ref", result.FullImageRef).
			Str("dockerfile_path", result.DockerfilePath).
			Str("session_id", session.SessionID).
			Msg("Docker build failed")
		result.BuildFailureAnalysis = e.troubleshooter.GenerateBuildFailureAnalysis(err, result.BuildResult, result)
		e.troubleshooter.AddTroubleshootingTips(result, err)
		return mcptypes.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("docker build failed: %v", err), "build_error")
	}
	if result.BuildResult != nil && !result.BuildResult.Success {
		buildErr := mcptypes.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("build failed: %s", result.BuildResult.Error.Message), "build_error")
		e.logger.Error().Err(buildErr).
			Str("image_ref", result.FullImageRef).
			Str("dockerfile_path", result.DockerfilePath).
			Str("session_id", session.SessionID).
			Msg("Docker build execution failed")
		result.BuildFailureAnalysis = e.troubleshooter.GenerateBuildFailureAnalysis(buildErr, result.BuildResult, result)
		e.troubleshooter.AddTroubleshootingTips(result, buildErr)
		return mcptypes.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("docker build execution failed: %v", buildErr), "build_error")
	}
	result.Success = true
	e.logger.Info().Msg("Docker image built successfully")
	// Stage 4: Verify - Running post-build verification
	e.logger.Info().Msg("Running security scan")
	if err := e.securityScanner.RunSecurityScan(ctx, session, result); err != nil {
		e.logger.Warn().Err(err).Msg("Security scan failed, but build was successful")
		result.BuildContext_Info.TroubleshootingTips = append(
			result.BuildContext_Info.TroubleshootingTips,
			fmt.Sprintf("Security scan failed: %v - consider installing Trivy for vulnerability scanning", err),
		)
	}
	// Push image if requested
	if args.PushAfterBuild && args.RegistryURL != "" {
		e.logger.Info().Msg("Pushing image to registry")
		pushStartTime := time.Now()
		// Construct full image ref with registry
		registryImageRef := result.FullImageRef
		if args.RegistryURL != "" && !strings.Contains(result.FullImageRef, "/") {
			registryImageRef = fmt.Sprintf("%s/%s", args.RegistryURL, result.FullImageRef)
		}
		err := e.pipelineAdapter.PushDockerImage(
			session.SessionID, // Use compatibility method
			registryImageRef,
		)
		result.PushDuration = time.Since(pushStartTime)
		// Create pushResult based on error
		if err != nil {
			// Detect authentication errors from error message
			errorType := "push_error"
			if strings.Contains(strings.ToLower(err.Error()), "authentication") ||
				strings.Contains(strings.ToLower(err.Error()), "unauthorized") ||
				strings.Contains(strings.ToLower(err.Error()), "login") ||
				strings.Contains(strings.ToLower(err.Error()), "auth") {
				errorType = "auth_error"
			}
			result.PushResult = &coredocker.RegistryPushResult{
				Success: false,
				Error: &coredocker.RegistryError{
					Type:    errorType,
					Message: err.Error(),
				},
			}
			e.logger.Warn().Err(err).Msg("Failed to push image, but build was successful")
			e.troubleshooter.AddPushTroubleshootingTips(result, result.PushResult, args.RegistryURL, err)
		} else {
			result.PushResult = &coredocker.RegistryPushResult{
				Success:  true,
				Registry: args.RegistryURL,
				ImageRef: registryImageRef,
			}
		}
	}
	e.logger.Info().Msg("Verification completed")
	// Stage 5: Finalize - Cleaning up and saving results
	e.logger.Info().Msg("Finalizing")
	e.analyzer.GenerateBuildContext(result)
	if err := e.updateSessionState(session, result); err != nil {
		e.logger.Warn().Err(err).Msg("Failed to update session state")
	}
	e.logger.Info().Msg("Build completed successfully")
	return nil
}

// executeWithoutProgress handles execution without progress tracking (fallback)
func (e *BuildExecutorService) executeWithoutProgress(ctx context.Context, args AtomicBuildImageArgs, result *AtomicBuildImageResult, startTime time.Time) (*AtomicBuildImageResult, error) {
	// Get session
	sessionInterface, err := e.sessionManager.GetSession(args.SessionID)
	if err != nil {
		e.logger.Error().Err(err).Str("session_id", args.SessionID).Msg("Failed to get session")
		result.Success = false
		result.TotalDuration = time.Since(startTime)
		return result, types.NewSessionError(args.SessionID, "build_image").
			WithStage("initialize").
			WithTool("build_image_atomic").
			WithField("image_name", args.ImageName).
			WithField("image_tag", args.ImageTag).
			WithRootCause("Session ID does not exist or has expired").
			WithCommand(2, "Create new session", "Create a new session if the current one is invalid", "analyze_repository --repo_path /path/to/repo", "New session created").
			Build()
	}
	session := sessionInterface.(*mcptypes.SessionState)
	// Set session details
	result.SessionID = session.SessionID
	result.WorkspaceDir = e.pipelineAdapter.GetSessionWorkspace(session.SessionID)
	result.FullImageRef = fmt.Sprintf("%s:%s", result.ImageName, result.ImageTag)
	result.BuildContext = e.getBuildContext(args.BuildContext, result.WorkspaceDir)
	result.DockerfilePath = e.getDockerfilePath(args.DockerfilePath, result.BuildContext)
	// Analyze build context
	if err := e.analyzer.AnalyzeBuildContext(result); err != nil {
		e.logger.Error().Err(err).
			Str("dockerfile_path", result.DockerfilePath).
			Str("build_context", result.BuildContext).
			Msg("Build context analysis failed")
		result.Success = false
		result.TotalDuration = time.Since(startTime)
		return result, types.NewBuildError("Build context analysis failed", args.SessionID, args.ImageName).
			WithStage("analysis").
			WithRelatedFiles(result.DockerfilePath, result.BuildContext).
			WithRootCause(err.Error()).
			WithImmediateStep(1, "Check Dockerfile exists", "Verify the Dockerfile exists at the specified path").
			WithImmediateStep(2, "Validate build context", "Ensure build context directory contains necessary files").
			WithPrevention("Always verify Dockerfile and build context paths before building").
			Build()
	}
	if err := e.analyzer.ValidateBuildPrerequisites(result); err != nil {
		e.logger.Error().Err(err).
			Str("dockerfile_path", result.DockerfilePath).
			Str("build_context", result.BuildContext).
			Int64("context_size", result.BuildContext_Info.ContextSize).
			Msg("Build prerequisites validation failed")
		result.Success = false
		result.TotalDuration = time.Since(startTime)
		return result, types.NewBuildError("Build prerequisites validation failed", args.SessionID, args.ImageName).
			WithStage("validation").
			WithRelatedFiles(result.DockerfilePath).
			WithRootCause(err.Error()).
			WithField("context_size_mb", result.BuildContext_Info.ContextSize/1024/1024).
			WithDiagnosticCheck("dockerfile_exists", result.BuildContext_Info.DockerfileExists, "Dockerfile presence check").
			WithDiagnosticCheck("context_size", result.BuildContext_Info.ContextSize < 5*1024*1024*1024, "Build context size check").
			WithImmediateStep(1, "Check Docker daemon", "Ensure Docker daemon is running").
			WithCommand(2, "Test Docker", "Test Docker connectivity", "docker version", "Docker version information displayed").
			Build()
	}
	// Build image
	buildStartTime := time.Now()
	buildResult, err := e.pipelineAdapter.BuildDockerImage(session.SessionID, result.FullImageRef, result.DockerfilePath)
	result.BuildDuration = time.Since(buildStartTime)
	// Convert from mcptypes.BuildResult to coredocker.BuildResult
	if buildResult != nil {
		result.BuildResult = &coredocker.BuildResult{
			Success:  buildResult.Success,
			ImageID:  buildResult.ImageID,
			ImageRef: buildResult.ImageRef,
			Duration: result.BuildDuration, // Use the duration we already calculated
		}
		if buildResult.Error != nil {
			result.BuildResult.Error = &coredocker.BuildError{
				Type:    buildResult.Error.Type,
				Message: buildResult.Error.Message,
			}
		}
		e.logger.Error().Err(err).Msg("Docker build failed")
		result.BuildFailureAnalysis = e.troubleshooter.GenerateBuildFailureAnalysis(err, result.BuildResult, result)
		e.troubleshooter.AddTroubleshootingTips(result, err)
		result.Success = false
		result.TotalDuration = time.Since(startTime)
		return result, mcptypes.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("docker build failed: %v", err), "build_error")
	}
	result.Success = true
	// Run security scan
	if err := e.securityScanner.RunSecurityScan(ctx, session, result); err != nil {
		e.logger.Warn().Err(err).Msg("Security scan failed, but build was successful")
	}
	// Push if requested
	if args.PushAfterBuild && args.RegistryURL != "" {
		pushStartTime := time.Now()
		// Construct full image ref with registry
		registryImageRef := result.FullImageRef
		if args.RegistryURL != "" && !strings.Contains(result.FullImageRef, "/") {
			registryImageRef = fmt.Sprintf("%s/%s", args.RegistryURL, result.FullImageRef)
		}
		err := e.pipelineAdapter.PushDockerImage(session.SessionID, registryImageRef)
		result.PushDuration = time.Since(pushStartTime)
		if err != nil {
			// Detect authentication errors from error message
			errorType := "push_error"
			if strings.Contains(strings.ToLower(err.Error()), "authentication") ||
				strings.Contains(strings.ToLower(err.Error()), "unauthorized") ||
				strings.Contains(strings.ToLower(err.Error()), "login") ||
				strings.Contains(strings.ToLower(err.Error()), "auth") {
				errorType = "auth_error"
			}
			result.PushResult = &coredocker.RegistryPushResult{
				Success: false,
				Error: &coredocker.RegistryError{
					Type:    errorType,
					Message: err.Error(),
				},
			}
			e.logger.Warn().Err(err).Msg("Failed to push image, but build was successful")
			e.troubleshooter.AddPushTroubleshootingTips(result, result.PushResult, args.RegistryURL, err)
		} else {
			result.PushResult = &coredocker.RegistryPushResult{
				Success:  true,
				Registry: args.RegistryURL,
				ImageRef: registryImageRef,
			}
		}
	}
	// Finalize
	e.analyzer.GenerateBuildContext(result)
	if err := e.updateSessionState(session, result); err != nil {
		e.logger.Warn().Err(err).Msg("Failed to update session state")
	}
	result.TotalDuration = time.Since(startTime)
	return result, nil
}

// updateSessionState updates the session with build results
func (e *BuildExecutorService) updateSessionState(session *mcptypes.SessionState, result *AtomicBuildImageResult) error {
	// Update session with build results
	if session.Metadata == nil {
		session.Metadata = make(map[string]interface{})
	}
	session.Metadata["last_built_image"] = result.FullImageRef
	session.Metadata["build_duration"] = result.BuildDuration.Seconds()
	session.Metadata["dockerfile_path"] = result.DockerfilePath
	session.Metadata["build_context"] = result.BuildContext
	if result.BuildResult != nil && result.BuildResult.Success {
		// Add to StageHistory for stage tracking
		now := time.Now()
		startTime := now.Add(-result.BuildDuration) // Calculate start time from duration
		execution := sessiontypes.ToolExecution{
			Tool:       "build_image",
			StartTime:  startTime,
			EndTime:    &now,
			Duration:   &result.BuildDuration,
			Success:    true,
			DryRun:     false,
			TokensUsed: 0, // Could be tracked if needed
		}
		// Track tool execution in metadata
		if session.Metadata == nil {
			session.Metadata = make(map[string]interface{})
		}
		session.Metadata["last_tool_execution"] = execution
		session.Metadata["build_success"] = true
		session.Metadata["image_id"] = result.BuildResult.ImageID
	} else {
		session.Metadata["build_success"] = false
	}
	if result.PushResult != nil && result.PushResult.Success {
		session.Metadata["push_success"] = true
		session.Metadata["registry_url"] = result.PushResult.Registry
	}
	// Update session timestamp
	session.UpdatedAt = time.Now()
	return e.sessionManager.UpdateSession(session.SessionID, func(s interface{}) {
		if sess, ok := s.(*mcptypes.SessionState); ok {
			*sess = *session
		}
	})
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
