package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	coredocker "github.com/Azure/container-copilot/pkg/core/docker"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	sessiontypes "github.com/Azure/container-copilot/pkg/mcp/internal/types/session"
	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
	"github.com/localrivet/gomcp/server"
	"github.com/rs/zerolog"
)

// BuildExecutor handles the execution of Docker builds with progress reporting
type BuildExecutor struct {
	pipelineAdapter mcptypes.PipelineOperations
	sessionManager  mcptypes.ToolSessionManager
	logger          zerolog.Logger
}

// NewBuildExecutor creates a new build executor
func NewBuildExecutor(adapter mcptypes.PipelineOperations, sessionManager mcptypes.ToolSessionManager, logger zerolog.Logger) *BuildExecutor {
	return &BuildExecutor{
		pipelineAdapter: adapter,
		sessionManager:  sessionManager,
		logger:          logger.With().Str("component", "build_executor").Logger(),
	}
}

// ExecuteWithFixes runs the atomic Docker image build with AI-driven fixing capabilities
func (e *BuildExecutor) ExecuteWithFixes(ctx context.Context, args AtomicBuildImageArgs, fixingMixin interface{}) (*AtomicBuildImageResult, error) {

	// Check if fixing is enabled
	if fixingMixin == nil {
		e.logger.Warn().Msg("AI-driven fixing not enabled, falling back to regular execution")
		return e.ExecuteBuild(ctx, args)
	}

	// First validate basic requirements
	if args.SessionID == "" {
		return nil, types.NewValidationErrorBuilder("Session ID is required", "session_id", args.SessionID).
			WithField("session_id", args.SessionID).
			WithOperation("build_image").
			WithStage("input_validation").
			WithImmediateStep(1, "Provide session ID", "Specify a valid session ID for the build operation").
			Build()
	}
	if args.ImageName == "" {
		return nil, types.NewValidationErrorBuilder("Image name is required", "image_name", args.ImageName).
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
	session := sessionInterface.(*sessiontypes.SessionState)

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
	return e.ExecuteBuild(ctx, args)
}

// ExecuteBuild runs the atomic Docker image build (deprecated: use ExecuteWithContext)
func (e *BuildExecutor) ExecuteBuild(ctx context.Context, args AtomicBuildImageArgs) (*AtomicBuildImageResult, error) {
	// Fallback: execute without progress tracking for backward compatibility
	startTime := time.Now()
	result := &AtomicBuildImageResult{
		BaseToolResponse:    types.NewBaseResponse("atomic_build_image", args.SessionID, args.DryRun),
		BaseAIContextResult: NewBaseAIContextResult("build", false, 0), // Duration will be updated later
		SessionID:           args.SessionID,
		ImageName:           args.ImageName,
		ImageTag:            e.getImageTag(args.ImageTag),
		Platform:            e.getPlatform(args.Platform),
		BuildContext_Info:   &BuildContextInfo{},
	}
	return e.executeWithoutProgress(ctx, args, result, startTime)
}

// ExecuteWithContext executes the tool with GoMCP server context for native progress tracking
func (e *BuildExecutor) ExecuteWithContext(serverCtx *server.Context, args AtomicBuildImageArgs) (*AtomicBuildImageResult, error) {
	startTime := time.Now()

	// Create result object early for error handling
	result := &AtomicBuildImageResult{
		BaseToolResponse:    types.NewBaseResponse("atomic_build_image", args.SessionID, args.DryRun),
		BaseAIContextResult: NewBaseAIContextResult("build", false, 0), // Duration will be updated later
		SessionID:           args.SessionID,
		ImageName:           args.ImageName,
		ImageTag:            e.getImageTag(args.ImageTag),
		Platform:            e.getPlatform(args.Platform),
		BuildContext_Info:   &BuildContextInfo{},
	}

	// Use centralized build stages for progress tracking
	// Progress adapter removed

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
func (e *BuildExecutor) executeWithProgress(ctx context.Context, args AtomicBuildImageArgs, result *AtomicBuildImageResult, startTime time.Time, reporter mcptypes.ProgressReporter) error {
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
	session := sessionInterface.(*sessiontypes.SessionState)

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
	if err := e.analyzeBuildContext(result); err != nil {
		e.logger.Error().Err(err).
			Str("dockerfile_path", result.DockerfilePath).
			Str("build_context", result.BuildContext).
			Msg("Build context analysis failed")
		return types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("build context analysis failed: %v", err), "filesystem_error")
	}

	e.logger.Info().Msg("Validating build prerequisites")
	if err := e.validateBuildPrerequisites(result); err != nil {
		e.logger.Error().Err(err).
			Str("dockerfile_path", result.DockerfilePath).
			Str("build_context", result.BuildContext).
			Int64("context_size", result.BuildContext_Info.ContextSize).
			Msg("Build prerequisites validation failed")
		return types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("build prerequisites validation failed: %v", err), "validation_error")
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
		result.BuildFailureAnalysis = e.generateBuildFailureAnalysis(err, result.BuildResult, result)
		e.addTroubleshootingTips(result, err)
		return types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("docker build failed: %v", err), "build_error")
	}

	if result.BuildResult != nil && !result.BuildResult.Success {
		buildErr := types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("build failed: %s", result.BuildResult.Error.Message), "build_error")
		e.logger.Error().Err(buildErr).
			Str("image_ref", result.FullImageRef).
			Str("dockerfile_path", result.DockerfilePath).
			Str("session_id", session.SessionID).
			Msg("Docker build execution failed")
		result.BuildFailureAnalysis = e.generateBuildFailureAnalysis(buildErr, result.BuildResult, result)
		e.addTroubleshootingTips(result, buildErr)
		return types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("docker build execution failed: %v", buildErr), "build_error")
	}

	result.Success = true
	e.logger.Info().Msg("Docker image built successfully")

	// Stage 4: Verify - Running post-build verification
	e.logger.Info().Msg("Running security scan")
	if err := e.runSecurityScan(ctx, session, result); err != nil {
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
			e.addPushTroubleshootingTips(result, result.PushResult, args.RegistryURL, err)
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
	e.generateBuildContext(result)

	if err := e.updateSessionState(session, result); err != nil {
		e.logger.Warn().Err(err).Msg("Failed to update session state")
	}

	e.logger.Info().Msg("Build completed successfully")
	return nil
}

// executeWithoutProgress handles execution without progress tracking (fallback)
func (e *BuildExecutor) executeWithoutProgress(ctx context.Context, args AtomicBuildImageArgs, result *AtomicBuildImageResult, startTime time.Time) (*AtomicBuildImageResult, error) {
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
	session := sessionInterface.(*sessiontypes.SessionState)

	// Set session details
	result.SessionID = session.SessionID
	result.WorkspaceDir = e.pipelineAdapter.GetSessionWorkspace(session.SessionID)
	result.FullImageRef = fmt.Sprintf("%s:%s", result.ImageName, result.ImageTag)
	result.BuildContext = e.getBuildContext(args.BuildContext, result.WorkspaceDir)
	result.DockerfilePath = e.getDockerfilePath(args.DockerfilePath, result.BuildContext)

	// Handle dry-run
	if args.DryRun {
		result.BuildContext_Info.NextStepSuggestions = []string{
			"This is a dry-run - actual Docker image build would be performed",
			fmt.Sprintf("Would build image: %s", result.FullImageRef),
			fmt.Sprintf("Using Dockerfile: %s", result.DockerfilePath),
			fmt.Sprintf("Build context: %s", result.BuildContext),
		}
		result.Success = true
		result.TotalDuration = time.Since(startTime)
		return result, nil
	}

	// Analyze and validate
	if err := e.analyzeBuildContext(result); err != nil {
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

	if err := e.validateBuildPrerequisites(result); err != nil {
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
	}

	if err != nil || (result.BuildResult != nil && !result.BuildResult.Success) {
		if err == nil && result.BuildResult != nil && result.BuildResult.Error != nil {
			err = types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("build failed: %s", result.BuildResult.Error.Message), "build_error")
		}
		e.logger.Error().Err(err).Msg("Docker build failed")
		result.BuildFailureAnalysis = e.generateBuildFailureAnalysis(err, result.BuildResult, result)
		e.addTroubleshootingTips(result, err)
		result.Success = false
		result.TotalDuration = time.Since(startTime)
		return result, types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("docker build failed: %v", err), "build_error")
	}

	result.Success = true

	// Run security scan
	if err := e.runSecurityScan(ctx, session, result); err != nil {
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
			e.addPushTroubleshootingTips(result, result.PushResult, args.RegistryURL, err)
		} else {
			result.PushResult = &coredocker.RegistryPushResult{
				Success:  true,
				Registry: args.RegistryURL,
				ImageRef: registryImageRef,
			}
		}
	}

	// Finalize
	e.generateBuildContext(result)
	if err := e.updateSessionState(session, result); err != nil {
		e.logger.Warn().Err(err).Msg("Failed to update session state")
	}

	result.TotalDuration = time.Since(startTime)
	return result, nil
}

// updateSessionState updates the session with build results
func (e *BuildExecutor) updateSessionState(session *sessiontypes.SessionState, result *AtomicBuildImageResult) error {
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
		session.AddToolExecution(execution)

		session.Metadata["build_success"] = true
		session.Metadata["image_id"] = result.BuildResult.ImageID
	} else {
		session.Metadata["build_success"] = false
	}

	if result.PushResult != nil && result.PushResult.Success {
		session.Metadata["push_success"] = true
		session.Metadata["registry_url"] = result.PushResult.Registry
	}

	session.UpdateLastAccessed()

	return e.sessionManager.UpdateSession(session.SessionID, func(s *sessiontypes.SessionState) {
		*s = *session
	})
}

// Helper methods

func (e *BuildExecutor) getImageTag(tag string) string {
	if tag == "" {
		return "latest"
	}
	return tag
}

func (e *BuildExecutor) getPlatform(platform string) string {
	if platform == "" {
		return "linux/amd64"
	}
	return platform
}

func (e *BuildExecutor) getBuildContext(context, workspaceDir string) string {
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

func (e *BuildExecutor) getDockerfilePath(dockerfilePath, buildContext string) string {
	if dockerfilePath == "" {
		return filepath.Join(buildContext, "Dockerfile")
	}

	// If relative path, make it relative to build context
	if !filepath.IsAbs(dockerfilePath) {
		return filepath.Join(buildContext, dockerfilePath)
	}

	return dockerfilePath
}

// analyzeBuildContext analyzes the build context and Dockerfile
func (e *BuildExecutor) analyzeBuildContext(result *AtomicBuildImageResult) error {
	ctx := result.BuildContext_Info

	// Check if Dockerfile exists
	if _, err := os.Stat(result.DockerfilePath); err != nil {
		ctx.DockerfileExists = false
		return types.NewRichError("INVALID_ARGUMENTS", fmt.Sprintf("Dockerfile not found at %s", result.DockerfilePath), "file_not_found")
	}
	ctx.DockerfileExists = true

	// Analyze Dockerfile content
	dockerfileContent, err := os.ReadFile(result.DockerfilePath)
	if err != nil {
		return types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("failed to read Dockerfile: %v", err), "file_error")
	}

	lines := strings.Split(string(dockerfileContent), "\n")
	ctx.DockerfileLines = len(lines)

	// Extract basic Dockerfile information
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(line), "FROM ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				if ctx.BaseImage == "" { // First FROM is the base image
					ctx.BaseImage = parts[1]
				}
				ctx.BuildStages++
			}
		}
		if strings.HasPrefix(strings.ToUpper(line), "EXPOSE ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				ctx.ExposedPorts = append(ctx.ExposedPorts, parts[1])
			}
		}
	}

	// Analyze build context directory
	if err := e.analyzeBuildContextDirectory(result); err != nil {
		e.logger.Warn().Err(err).Msg("Failed to analyze build context directory")
	}

	return nil
}

// analyzeBuildContextDirectory analyzes the build context directory
func (e *BuildExecutor) analyzeBuildContextDirectory(result *AtomicBuildImageResult) error {
	ctx := result.BuildContext_Info

	// Check for .dockerignore
	dockerignorePath := filepath.Join(result.BuildContext, ".dockerignore")
	if _, err := os.Stat(dockerignorePath); err == nil {
		ctx.HasDockerIgnore = true
	}

	// Calculate context size and file count
	var totalSize int64
	var fileCount int

	err := filepath.WalkDir(result.BuildContext, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if !d.IsDir() {
			fileCount++
			if info, err := d.Info(); err == nil {
				totalSize += info.Size()

				// Flag large files (>50MB)
				if info.Size() > 50*1024*1024 {
					relPath, err := filepath.Rel(result.BuildContext, path)
					if err != nil {
						relPath = path // Use absolute path if relative fails
					}
					ctx.LargeFilesFound = append(ctx.LargeFilesFound, relPath)
				}
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	ctx.ContextSize = totalSize
	ctx.FileCount = fileCount

	return nil
}

// validateBuildPrerequisites validates that everything is ready for building
func (e *BuildExecutor) validateBuildPrerequisites(result *AtomicBuildImageResult) error {
	ctx := result.BuildContext_Info

	if !ctx.DockerfileExists {
		return types.NewRichError("INVALID_ARGUMENTS", "Dockerfile is required for building", "missing_dockerfile")
	}

	// Check build context exists
	if _, err := os.Stat(result.BuildContext); err != nil {
		return types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("build context directory not accessible: %v", err), "filesystem_error")
	}

	// Warn about large build context
	if ctx.ContextSize > 100*1024*1024 { // 100MB
		e.logger.Warn().
			Int64("size_mb", ctx.ContextSize/(1024*1024)).
			Msg("Large build context detected")

		ctx.TroubleshootingTips = append(ctx.TroubleshootingTips,
			fmt.Sprintf("Build context is large (%d MB) - consider adding .dockerignore",
				ctx.ContextSize/(1024*1024)))
	}

	return nil
}

// generateBuildContext generates rich context for Claude reasoning
func (e *BuildExecutor) generateBuildContext(result *AtomicBuildImageResult) {
	ctx := result.BuildContext_Info

	// Generate build optimizations based on analysis
	if ctx.BuildStages > 1 {
		ctx.BuildOptimizations = append(ctx.BuildOptimizations,
			"Multi-stage build detected - good for image size optimization")
	}

	if !ctx.HasDockerIgnore && ctx.FileCount > 100 {
		ctx.BuildOptimizations = append(ctx.BuildOptimizations,
			"Consider adding .dockerignore to reduce build context size")
	}

	if len(ctx.LargeFilesFound) > 0 {
		ctx.BuildOptimizations = append(ctx.BuildOptimizations,
			fmt.Sprintf("Large files detected: %s - consider excluding from build context",
				strings.Join(ctx.LargeFilesFound, ", ")))
	}

	// Generate security recommendations
	if strings.Contains(strings.ToLower(ctx.BaseImage), "latest") {
		ctx.SecurityRecommendations = append(ctx.SecurityRecommendations,
			"Consider using specific image tags instead of 'latest' for reproducible builds")
	}

	if !strings.Contains(strings.ToLower(ctx.BaseImage), "alpine") &&
		!strings.Contains(strings.ToLower(ctx.BaseImage), "distroless") {
		ctx.SecurityRecommendations = append(ctx.SecurityRecommendations,
			"Consider using alpine or distroless base images for smaller attack surface")
	}

	// Generate next step suggestions
	if result.BuildResult != nil && result.BuildResult.Success {
		ctx.NextStepSuggestions = append(ctx.NextStepSuggestions,
			"Docker image built successfully - ready for deployment")

		if result.PushResult == nil || !result.PushResult.Success {
			ctx.NextStepSuggestions = append(ctx.NextStepSuggestions,
				"Use push_image tool to push image to registry")
		}

		ctx.NextStepSuggestions = append(ctx.NextStepSuggestions,
			"Use generate_manifests tool to create Kubernetes deployment files")

		ctx.NextStepSuggestions = append(ctx.NextStepSuggestions,
			"Image is stored in session context for subsequent operations")
	}
}

// addPushTroubleshootingTips adds troubleshooting tips for push failures
func (e *BuildExecutor) addPushTroubleshootingTips(result *AtomicBuildImageResult, pushResult *coredocker.RegistryPushResult, registryURL string, err error) {
	// Check if we have detailed error information in pushResult
	if pushResult != nil && pushResult.Error != nil {
		// Check if this is an authentication error
		if pushResult.Error.Type == "auth_error" {
			// Add authentication guidance
			if authGuidance, ok := pushResult.Error.Context["auth_guidance"].([]string); ok {
				result.BuildContext_Info.TroubleshootingTips = append(
					result.BuildContext_Info.TroubleshootingTips,
					authGuidance...,
				)
			} else {
				// Fallback if type assertion fails
				result.BuildContext_Info.TroubleshootingTips = append(
					result.BuildContext_Info.TroubleshootingTips,
					"Authentication failed - run: docker login "+registryURL,
				)
			}
		} else {
			result.BuildContext_Info.TroubleshootingTips = append(
				result.BuildContext_Info.TroubleshootingTips,
				fmt.Sprintf("Push failed: %s - use separate push_image tool to retry", pushResult.Error.Message),
			)
		}
	} else {
		// Generic error message if no detailed error info
		result.BuildContext_Info.TroubleshootingTips = append(
			result.BuildContext_Info.TroubleshootingTips,
			fmt.Sprintf("Push failed: %v - use separate push_image tool to retry", err),
		)
	}
}

// addTroubleshootingTips adds troubleshooting tips based on build errors
func (e *BuildExecutor) addTroubleshootingTips(result *AtomicBuildImageResult, err error) {
	ctx := result.BuildContext_Info
	errStr := strings.ToLower(err.Error())

	if strings.Contains(errStr, "no such file") {
		ctx.TroubleshootingTips = append(ctx.TroubleshootingTips,
			"Check that all files referenced in Dockerfile exist in build context")
	}

	if strings.Contains(errStr, "permission denied") {
		ctx.TroubleshootingTips = append(ctx.TroubleshootingTips,
			"Check file permissions in build context and Dockerfile")
	}

	if strings.Contains(errStr, "network") || strings.Contains(errStr, "timeout") {
		ctx.TroubleshootingTips = append(ctx.TroubleshootingTips,
			"Network issue detected - check internet connectivity for package downloads")
	}

	if strings.Contains(errStr, "space") || strings.Contains(errStr, "disk") {
		ctx.TroubleshootingTips = append(ctx.TroubleshootingTips,
			"Disk space issue - clean up Docker images and containers")
	}

	if strings.Contains(errStr, "exit status") || strings.Contains(errStr, "returned a non-zero code") {
		ctx.TroubleshootingTips = append(ctx.TroubleshootingTips,
			"Build command failed - check the Dockerfile commands and their syntax")
		ctx.TroubleshootingTips = append(ctx.TroubleshootingTips,
			"Review the build logs to identify which step failed")
	}
}

// runSecurityScan runs Trivy security scan on the built image
func (e *BuildExecutor) runSecurityScan(ctx context.Context, session *sessiontypes.SessionState, result *AtomicBuildImageResult) error {
	// Create Trivy scanner
	scanner := coredocker.NewTrivyScanner(e.logger)

	// Check if Trivy is installed
	if !scanner.CheckTrivyInstalled() {
		e.logger.Info().Msg("Trivy not installed, skipping security scan")
		result.BuildContext_Info.SecurityRecommendations = append(
			result.BuildContext_Info.SecurityRecommendations,
			"Install Trivy for container security scanning: curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b /usr/local/bin",
		)
		return nil
	}

	scanStartTime := time.Now()

	// Run security scan with HIGH severity threshold
	scanResult, err := scanner.ScanImage(ctx, result.FullImageRef, "HIGH,CRITICAL")
	if err != nil {
		return types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("security scan failed: %v", err), "scan_error")
	}

	result.ScanDuration = time.Since(scanStartTime)
	result.SecurityScan = scanResult

	// Log scan summary
	e.logger.Info().
		Str("image", result.FullImageRef).
		Int("total_vulnerabilities", scanResult.Summary.Total).
		Int("critical", scanResult.Summary.Critical).
		Int("high", scanResult.Summary.High).
		Dur("scan_duration", result.ScanDuration).
		Msg("Security scan completed")

	// Update session state with scan results
	session.SecurityScan = &sessiontypes.SecurityScanSummary{
		Success:   scanResult.Success,
		ScannedAt: scanResult.ScanTime,
		ImageRef:  result.FullImageRef,
		Summary: sessiontypes.VulnerabilitySummary{
			Total:    scanResult.Summary.Total,
			Critical: scanResult.Summary.Critical,
			High:     scanResult.Summary.High,
			Medium:   scanResult.Summary.Medium,
			Low:      scanResult.Summary.Low,
			Unknown:  scanResult.Summary.Unknown,
		},
		Fixable: scanResult.Summary.Fixable,
		Scanner: "trivy",
	}

	// Also store in metadata for backward compatibility
	if session.Metadata == nil {
		session.Metadata = make(map[string]interface{})
	}
	session.Metadata["security_scan"] = map[string]interface{}{
		"scanned_at":     scanResult.ScanTime,
		"total_vulns":    scanResult.Summary.Total,
		"critical_vulns": scanResult.Summary.Critical,
		"high_vulns":     scanResult.Summary.High,
		"scan_success":   scanResult.Success,
	}

	// Add security recommendations based on scan results
	if scanResult.Summary.Critical > 0 || scanResult.Summary.High > 0 {
		result.BuildContext_Info.SecurityRecommendations = append(
			result.BuildContext_Info.SecurityRecommendations,
			fmt.Sprintf("⚠️ Found %d CRITICAL and %d HIGH severity vulnerabilities",
				scanResult.Summary.Critical, scanResult.Summary.High),
		)

		// Add remediation steps to build context
		for _, step := range scanResult.Remediation {
			result.BuildContext_Info.SecurityRecommendations = append(
				result.BuildContext_Info.SecurityRecommendations,
				fmt.Sprintf("%d. %s: %s", step.Priority, step.Action, step.Description),
			)
		}

		// Mark as failed if critical vulnerabilities found
		if scanResult.Summary.Critical > 0 {
			e.logger.Error().
				Int("critical_vulns", scanResult.Summary.Critical).
				Int("high_vulns", scanResult.Summary.High).
				Str("image_ref", result.FullImageRef).
				Msg("Critical security vulnerabilities found")
			result.Success = false
			return types.NewRichError("INTERNAL_SERVER_ERROR", "critical vulnerabilities found", "security_error")
		}
	}

	// Update next steps based on scan results
	if scanResult.Success {
		result.BuildContext_Info.NextStepSuggestions = append(
			result.BuildContext_Info.NextStepSuggestions,
			"✅ Security scan passed - image is safe to deploy",
		)
	} else {
		result.BuildContext_Info.NextStepSuggestions = append(
			result.BuildContext_Info.NextStepSuggestions,
			"⚠️ Security vulnerabilities found - review and fix before deployment",
		)
	}

	return nil
}

// generateBuildFailureAnalysis creates AI decision-making context for build failures
func (e *BuildExecutor) generateBuildFailureAnalysis(err error, buildResult *coredocker.BuildResult, result *AtomicBuildImageResult) *BuildFailureAnalysis {
	analysis := &BuildFailureAnalysis{}
	errStr := strings.ToLower(err.Error())

	// Determine failure type and stage
	analysis.FailureType, analysis.FailureStage = e.classifyFailure(errStr, buildResult)

	// Identify common causes
	causes := e.identifyFailureCauses(errStr, buildResult, result)
	analysis.CommonCauses = make([]string, len(causes))
	for i, cause := range causes {
		analysis.CommonCauses[i] = cause.Description
	}

	// Generate suggested fixes
	fixes := e.generateSuggestedFixes(errStr, buildResult, result)
	analysis.SuggestedFixes = make([]string, len(fixes))
	for i, fix := range fixes {
		analysis.SuggestedFixes[i] = fix.Description
	}

	// Provide alternative strategies
	strategies := e.generateAlternativeStrategies(errStr, buildResult, result)
	analysis.AlternativeStrategies = make([]string, len(strategies))
	for i, strategy := range strategies {
		analysis.AlternativeStrategies[i] = strategy.Description
	}

	// Analyze performance impact
	perfAnalysis := e.analyzePerformanceImpact(buildResult, result)
	analysis.PerformanceImpact = fmt.Sprintf("Build time: %v, bottlenecks: %v", perfAnalysis.BuildTime, perfAnalysis.Bottlenecks)

	// Identify security implications
	analysis.SecurityImplications = e.identifySecurityImplications(errStr, buildResult, result)

	return analysis
}

// classifyFailure determines the type and stage of build failure
func (e *BuildExecutor) classifyFailure(errStr string, buildResult *coredocker.BuildResult) (string, string) {
	failureType := types.UnknownString
	failureStage := types.UnknownString

	// Classify failure type
	switch {
	case strings.Contains(errStr, "no such file") || strings.Contains(errStr, "not found"):
		failureType = "file_missing"
	case strings.Contains(errStr, "permission denied") || strings.Contains(errStr, "access denied"):
		failureType = "permission"
	case strings.Contains(errStr, "network") || strings.Contains(errStr, "timeout") || strings.Contains(errStr, "connection"):
		failureType = "network"
	case strings.Contains(errStr, "space") || strings.Contains(errStr, "disk full"):
		failureType = "disk_space"
	case strings.Contains(errStr, "syntax") || strings.Contains(errStr, "invalid"):
		failureType = "dockerfile_syntax"
	case strings.Contains(errStr, "exit status") || strings.Contains(errStr, "returned a non-zero code"):
		failureType = "command_failure"
	case strings.Contains(errStr, "dependency") || strings.Contains(errStr, "package"):
		failureType = "dependency"
	case strings.Contains(errStr, "authentication") || strings.Contains(errStr, "unauthorized"):
		failureType = "authentication"
	}

	// Classify failure stage
	switch {
	case strings.Contains(errStr, "pull") || strings.Contains(errStr, "download"):
		failureStage = "image_pull"
	case strings.Contains(errStr, "copy") || strings.Contains(errStr, "add"):
		failureStage = "file_copy"
	case strings.Contains(errStr, "run") || strings.Contains(errStr, "execute"):
		failureStage = "command_execution"
	case strings.Contains(errStr, "build"):
		failureStage = "build_process"
	case strings.Contains(errStr, "dockerfile"):
		failureStage = "dockerfile_parsing"
	}

	return failureType, failureStage
}

// identifyFailureCauses analyzes the failure to identify likely causes
func (e *BuildExecutor) identifyFailureCauses(errStr string, buildResult *coredocker.BuildResult, result *AtomicBuildImageResult) []FailureCause {
	causes := []FailureCause{}

	switch {
	case strings.Contains(errStr, "no such file"):
		causes = append(causes, FailureCause{
			Category:    "filesystem",
			Description: "Required file or directory is missing from build context",
			Likelihood:  "high",
			Evidence:    []string{"'no such file' error in build output", "COPY or ADD instruction failed"},
		})

	case strings.Contains(errStr, "permission denied"):
		causes = append(causes, FailureCause{
			Category:    "permissions",
			Description: "Insufficient permissions to access files or execute commands",
			Likelihood:  "high",
			Evidence:    []string{"'permission denied' error", "File access or execution failed"},
		})

	case strings.Contains(errStr, "network") || strings.Contains(errStr, "timeout"):
		causes = append(causes, FailureCause{
			Category:    "network",
			Description: "Network connectivity issues preventing package downloads",
			Likelihood:  "medium",
			Evidence:    []string{"Network timeout or connection errors", "Package manager failures"},
		})

	case strings.Contains(errStr, "exit status"):
		causes = append(causes, FailureCause{
			Category:    "command",
			Description: "Command in Dockerfile failed during execution",
			Likelihood:  "high",
			Evidence:    []string{"Non-zero exit code from command", "RUN instruction failed"},
		})

	case strings.Contains(errStr, "space") || strings.Contains(errStr, "disk"):
		causes = append(causes, FailureCause{
			Category:    "resources",
			Description: "Insufficient disk space during build process",
			Likelihood:  "medium",
			Evidence:    []string{"Disk space or storage errors", "Build process halted unexpectedly"},
		})
	}

	// Add context-specific causes
	if result.BuildContext_Info.ContextSize > 500*1024*1024 { // > 500MB
		causes = append(causes, FailureCause{
			Category:    "performance",
			Description: "Large build context may cause timeouts or resource issues",
			Likelihood:  "low",
			Evidence:    []string{fmt.Sprintf("Build context size: %d MB", result.BuildContext_Info.ContextSize/(1024*1024))},
		})
	}

	if !result.BuildContext_Info.HasDockerIgnore && result.BuildContext_Info.FileCount > 1000 {
		causes = append(causes, FailureCause{
			Category:    "optimization",
			Description: "Missing .dockerignore with many files may slow build or cause failures",
			Likelihood:  "low",
			Evidence:    []string{fmt.Sprintf("%d files in context", result.BuildContext_Info.FileCount), "No .dockerignore file"},
		})
	}

	return causes
}

// generateSuggestedFixes provides specific remediation steps
func (e *BuildExecutor) generateSuggestedFixes(errStr string, buildResult *coredocker.BuildResult, result *AtomicBuildImageResult) []BuildFix {
	fixes := []BuildFix{}

	switch {
	case strings.Contains(errStr, "no such file"):
		fixes = append(fixes, BuildFix{
			Priority:    "high",
			Title:       "Verify file paths in Dockerfile",
			Description: "Check that all COPY and ADD instructions reference existing files",
			Commands: []string{
				fmt.Sprintf("ls -la %s", result.BuildContext),
				"grep -n 'COPY\\|ADD' " + result.DockerfilePath,
			},
			Validation:    "All referenced files should exist in build context",
			EstimatedTime: "5 minutes",
		})

	case strings.Contains(errStr, "permission denied"):
		fixes = append(fixes, BuildFix{
			Priority:    "high",
			Title:       "Fix file permissions",
			Description: "Ensure files have correct permissions and ownership",
			Commands: []string{
				fmt.Sprintf("chmod +x %s/scripts/*", result.BuildContext),
				fmt.Sprintf("ls -la %s", result.BuildContext),
			},
			Validation:    "Files should have appropriate execute permissions",
			EstimatedTime: "2 minutes",
		})

	case strings.Contains(errStr, "network") || strings.Contains(errStr, "timeout"):
		fixes = append(fixes, BuildFix{
			Priority:    "medium",
			Title:       "Retry with network troubleshooting",
			Description: "Check network connectivity and retry with longer timeout",
			Commands: []string{
				"docker build --network=host --build-arg HTTP_PROXY=$HTTP_PROXY " + result.BuildContext,
				"ping -c 3 google.com",
			},
			Validation:    "Network should be accessible and packages downloadable",
			EstimatedTime: "10 minutes",
		})

	case strings.Contains(errStr, "exit status"):
		fixes = append(fixes, BuildFix{
			Priority:    "high",
			Title:       "Debug failing command",
			Description: "Identify and fix the specific command that failed",
			Commands: []string{
				"docker build --progress=plain " + result.BuildContext,
				"# Review the full output to identify failing step",
			},
			Validation:    "All RUN commands should complete successfully",
			EstimatedTime: "15 minutes",
		})

	case strings.Contains(errStr, "space") || strings.Contains(errStr, "disk"):
		fixes = append(fixes, BuildFix{
			Priority:    "high",
			Title:       "Free up disk space",
			Description: "Clean up Docker resources and system disk space",
			Commands: []string{
				"docker system prune -a",
				"df -h",
				"docker images --format 'table {{.Repository}}\\t{{.Tag}}\\t{{.Size}}'",
			},
			Validation:    "Sufficient disk space should be available",
			EstimatedTime: "5 minutes",
		})
	}

	// Add general fixes based on context
	if result.BuildContext_Info.ContextSize > 100*1024*1024 { // > 100MB
		fixes = append(fixes, BuildFix{
			Priority:    "low",
			Title:       "Optimize build context",
			Description: "Reduce build context size with .dockerignore",
			Commands: []string{
				fmt.Sprintf("echo 'node_modules\\n.git\\n*.log' > %s/.dockerignore", result.BuildContext),
				fmt.Sprintf("du -sh %s", result.BuildContext),
			},
			Validation:    "Build context should be smaller",
			EstimatedTime: "10 minutes",
		})
	}

	return fixes
}

// generateAlternativeStrategies provides different approaches to building
func (e *BuildExecutor) generateAlternativeStrategies(errStr string, buildResult *coredocker.BuildResult, result *AtomicBuildImageResult) []BuildStrategy {
	strategies := []BuildStrategy{}

	// Base strategy alternatives
	strategies = append(strategies, BuildStrategy{
		Name:        "Multi-stage build optimization",
		Description: "Use multi-stage builds to reduce final image size and complexity",
		Pros:        []string{"Smaller final image", "Better caching", "Cleaner separation"},
		Cons:        []string{"More complex Dockerfile", "Longer initial setup"},
		Complexity:  "moderate",
		Example:     "FROM node:18 AS builder\nCOPY . .\nRUN npm ci\nFROM node:18-slim\nCOPY --from=builder /app/dist ./dist",
	})

	if strings.Contains(strings.ToLower(result.BuildContext_Info.BaseImage), "ubuntu") ||
		strings.Contains(strings.ToLower(result.BuildContext_Info.BaseImage), "debian") {
		strategies = append(strategies, BuildStrategy{
			Name:        "Alpine base image",
			Description: "Switch to Alpine Linux for smaller, more secure base image",
			Pros:        []string{"Much smaller size", "Better security", "Faster builds"},
			Cons:        []string{"Different package manager", "Potential compatibility issues"},
			Complexity:  "simple",
			Example:     "FROM alpine:latest\nRUN apk add --no-cache <packages>",
		})
	}

	// Network-specific strategies
	if strings.Contains(errStr, "network") || strings.Contains(errStr, "timeout") {
		strategies = append(strategies, BuildStrategy{
			Name:        "Offline/cached build",
			Description: "Pre-download dependencies and use local cache",
			Pros:        []string{"No network dependencies", "Faster builds", "More reliable"},
			Cons:        []string{"Requires setup", "May be outdated"},
			Complexity:  "complex",
			Example:     "# Download dependencies locally first\n# Use COPY to add to image instead of network download",
		})
	}

	// Performance-specific strategies
	if result.BuildDuration > 5*time.Minute {
		strategies = append(strategies, BuildStrategy{
			Name:        "Build optimization",
			Description: "Optimize layer caching and reduce rebuild time",
			Pros:        []string{"Faster subsequent builds", "Better resource usage"},
			Cons:        []string{"Requires Dockerfile restructuring"},
			Complexity:  "moderate",
			Example:     "# Copy package files first\nCOPY package*.json ./\nRUN npm ci\n# Then copy source code",
		})
	}

	return strategies
}

// analyzePerformanceImpact assesses the performance implications
func (e *BuildExecutor) analyzePerformanceImpact(buildResult *coredocker.BuildResult, result *AtomicBuildImageResult) PerformanceAnalysis {
	analysis := PerformanceAnalysis{}

	// Analyze build time
	analysis.BuildTime = result.BuildDuration

	// Analyze cache efficiency (estimated based on build time and context)
	if buildResult != nil && buildResult.Success {
		// This is a rough estimate - in real implementation you'd check actual cache hits
		if result.BuildDuration < 2*time.Minute && result.BuildContext_Info.FileCount > 100 {
			analysis.CacheEfficiency = "excellent"
		} else if result.BuildDuration < 5*time.Minute {
			analysis.CacheEfficiency = "good"
		} else {
			analysis.CacheEfficiency = types.QualityPoor
		}
	} else {
		analysis.CacheEfficiency = types.UnknownString
	}

	// Estimate image size category
	contextSize := result.BuildContext_Info.ContextSize
	switch {
	case contextSize < 50*1024*1024: // < 50MB
		analysis.ImageSize = types.SizeSmall
	case contextSize < 200*1024*1024: // < 200MB
		analysis.ImageSize = types.SeverityMedium
	default:
		analysis.ImageSize = types.SizeLarge
	}

	// Generate optimizations
	if analysis.BuildTime > 5*time.Minute {
		analysis.Optimizations = append(analysis.Optimizations,
			"Consider multi-stage builds to improve caching",
			"Optimize Dockerfile layer ordering",
			"Use .dockerignore to reduce context size")
	}

	if analysis.CacheEfficiency == "poor" {
		analysis.Optimizations = append(analysis.Optimizations,
			"Restructure Dockerfile to maximize layer reuse",
			"Separate dependency installation from code copying")
	}

	if analysis.ImageSize == types.SizeLarge {
		analysis.Optimizations = append(analysis.Optimizations,
			"Use distroless or alpine base images",
			"Remove unnecessary packages and files",
			"Implement multi-stage builds")
	}

	return analysis
}

// identifySecurityImplications analyzes security aspects of the build failure
func (e *BuildExecutor) identifySecurityImplications(errStr string, buildResult *coredocker.BuildResult, result *AtomicBuildImageResult) []string {
	implications := []string{}

	// Permission-related security implications
	if strings.Contains(errStr, "permission") {
		implications = append(implications,
			"Permission errors may indicate overly restrictive or permissive file access",
			"Review file ownership and ensure principle of least privilege")
	}

	// Network-related security implications
	if strings.Contains(errStr, "network") || strings.Contains(errStr, "download") {
		implications = append(implications,
			"Network failures during build may expose dependencies on external resources",
			"Consider vendoring dependencies to reduce supply chain risks")
	}

	// Base image security implications
	baseImage := strings.ToLower(result.BuildContext_Info.BaseImage)
	if strings.Contains(baseImage, "latest") {
		implications = append(implications,
			"Using 'latest' tag creates unpredictable builds and potential security vulnerabilities",
			"Pin to specific image versions for reproducible and secure builds")
	}

	if strings.Contains(baseImage, "ubuntu") || strings.Contains(baseImage, "centos") {
		implications = append(implications,
			"Full OS base images have larger attack surface",
			"Consider minimal base images like alpine or distroless")
	}

	// Context-specific implications
	if !result.BuildContext_Info.HasDockerIgnore {
		implications = append(implications,
			"Missing .dockerignore may include sensitive files in image layers",
			"Create .dockerignore to prevent accidental inclusion of secrets")
	}

	if len(result.BuildContext_Info.LargeFilesFound) > 0 {
		implications = append(implications,
			"Large files in build context may contain sensitive data",
			"Review and exclude unnecessary large files from image")
	}

	return implications
}
