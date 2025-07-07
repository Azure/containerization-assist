package build

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/application/core"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
	"github.com/Azure/container-kit/pkg/mcp/services"
	"github.com/rs/zerolog"
)

// TypeSafeBuildImageTool implements the new type-safe api.TypedBuildTool interface
type TypeSafeBuildImageTool struct {
	pipelineAdapter core.TypedPipelineOperations
	sessionManager  session.UnifiedSessionManager // Legacy field for backward compatibility
	sessionStore    services.SessionStore         // Modern service interface
	sessionState    services.SessionState         // Modern service interface
	logger          zerolog.Logger
	timeout         time.Duration
	atomicTool      *AtomicBuildImageTool // If available
}

// NewTypeSafeBuildImageTool creates a new type-safe build image tool (legacy constructor)
func NewTypeSafeBuildImageTool(
	adapter core.TypedPipelineOperations,
	sessionManager session.UnifiedSessionManager,
	logger zerolog.Logger,
) api.TypedBuildTool {
	return &TypeSafeBuildImageTool{
		pipelineAdapter: adapter,
		sessionManager:  sessionManager,
		logger:          logger.With().Str("tool", "typesafe_build_image").Logger(),
		timeout:         30 * time.Minute, // Default build timeout
	}
}

// NewTypeSafeBuildImageToolWithServices creates a new type-safe build image tool using service interfaces
func NewTypeSafeBuildImageToolWithServices(
	adapter core.TypedPipelineOperations,
	serviceContainer services.ServiceContainer,
	logger zerolog.Logger,
) api.TypedBuildTool {
	toolLogger := logger.With().Str("tool", "typesafe_build_image").Logger()

	return &TypeSafeBuildImageTool{
		pipelineAdapter: adapter,
		sessionStore:    serviceContainer.SessionStore(),
		sessionState:    serviceContainer.SessionState(),
		logger:          toolLogger,
		timeout:         30 * time.Minute, // Default build timeout
	}
}

// Name implements api.TypedTool
func (t *TypeSafeBuildImageTool) Name() string {
	return "build_image"
}

// Description implements api.TypedTool
func (t *TypeSafeBuildImageTool) Description() string {
	return "Builds a Docker image from a Dockerfile and build context"
}

// Execute implements api.TypedTool with type-safe input and output
func (t *TypeSafeBuildImageTool) Execute(
	ctx context.Context,
	input api.TypedToolInput[api.TypedBuildInput, api.BuildContext],
) (api.TypedToolOutput[api.TypedBuildOutput, api.BuildDetails], error) {
	// Telemetry execution removed

	return t.executeInternal(ctx, input)
}

// executeInternal contains the core execution logic
func (t *TypeSafeBuildImageTool) executeInternal(
	ctx context.Context,
	input api.TypedToolInput[api.TypedBuildInput, api.BuildContext],
) (api.TypedToolOutput[api.TypedBuildOutput, api.BuildDetails], error) {
	startTime := time.Now()

	// Validate input
	if err := t.validateInput(input); err != nil {
		return api.TypedToolOutput[api.TypedBuildOutput, api.BuildDetails]{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	t.logger.Info().
		Str("session_id", input.SessionID).
		Str("image", input.Data.Image).
		Str("dockerfile", input.Data.Dockerfile).
		Str("context", input.Data.ContextPath).
		Bool("no_cache", input.Data.NoCache).
		Msg("Starting Docker image build")

	// Create or get session using appropriate service interface
	sess, err := t.getOrCreateSession(ctx, input.SessionID)
	if err != nil {
		return t.errorOutput(input.SessionID, "Failed to get or create session", err), err
	}

	// Update session state
	sess.AddLabel("building")
	sess.UpdateLastAccessed()

	// Prepare build context
	buildContext := t.prepareBuildContext(input)

	// Execute build via pipeline adapter
	buildResult, err := t.executeBuild(ctx, buildContext)
	if err != nil {
		return t.errorOutput(input.SessionID, "Build failed", err), err
	}

	// Store build results in session
	sess.RemoveLabel("building")
	sess.AddLabel("build_completed")

	// Add execution record
	endTime := time.Now()
	sess.AddToolExecution(session.ToolExecution{
		Tool:      "build_image",
		StartTime: startTime,
		EndTime:   &endTime,
		Success:   err == nil,
	})

	// Build output
	output := api.TypedBuildOutput{
		Success:   true,
		SessionID: input.SessionID,
		ImageID:   buildResult.ImageID,
		Digest:    buildResult.ImageID, // Using ImageID as digest for now
		Tags:      input.Data.Tags,
		BuildMetrics: api.BuildMetrics{
			BuildTime:  time.Since(startTime),
			ImageSize:  buildResult.Size,
			LayerCount: buildResult.LayerCount,
			BaseImage:  "unknown", // Not available in this BuildResult
			CacheUsed:  !input.Data.NoCache && buildResult.CacheHit,
		},
	}

	// Build details
	details := api.BuildDetails{
		ExecutionDetails: api.ExecutionDetails{
			Duration:  time.Since(startTime),
			StartTime: startTime,
			EndTime:   time.Now(),
			ResourcesUsed: api.ResourceUsage{
				CPUTime:    int64(time.Since(startTime).Milliseconds()),
				MemoryPeak: 0, // Not available in this BuildResult
				NetworkIO:  0, // Not available in this BuildResult
				DiskIO:     0, // Not available in this BuildResult
			},
		},
		ImageSize:  buildResult.Size,
		LayerCount: buildResult.LayerCount,
		CacheHit:   buildResult.CacheHit,
		BuildSteps: t.convertBuildSteps(buildResult.Steps),
	}

	t.logger.Info().
		Str("session_id", input.SessionID).
		Dur("duration", time.Since(startTime)).
		Str("image_id", buildResult.ImageID).
		Int64("image_size", buildResult.Size).
		Msg("Docker image build completed")

	return api.TypedToolOutput[api.TypedBuildOutput, api.BuildDetails]{
		Success: true,
		Data:    output,
		Details: details,
	}, nil
}

// Schema implements api.TypedTool
func (t *TypeSafeBuildImageTool) Schema() api.TypedToolSchema[api.TypedBuildInput, api.BuildContext, api.TypedBuildOutput, api.BuildDetails] {
	return api.TypedToolSchema[api.TypedBuildInput, api.BuildContext, api.TypedBuildOutput, api.BuildDetails]{
		Name:        t.Name(),
		Description: t.Description(),
		Version:     "2.0.0",
		InputExample: api.TypedToolInput[api.TypedBuildInput, api.BuildContext]{
			SessionID: "example-session-123",
			Data: api.TypedBuildInput{
				SessionID:   "example-session-123",
				Image:       "myapp:latest",
				Dockerfile:  "Dockerfile",
				ContextPath: ".",
				BuildArgs: map[string]string{
					"VERSION": "1.0.0",
				},
				Tags:     []string{"myapp:latest", "myapp:1.0.0"},
				NoCache:  false,
				Platform: "linux/amd64",
			},
			Context: api.BuildContext{
				ExecutionContext: api.ExecutionContext{
					RequestID: "req-123",
					TraceID:   "trace-456",
					Timeout:   30 * time.Minute,
				},
				Registry: "docker.io",
				Labels: map[string]string{
					"version": "1.0.0",
					"app":     "myapp",
				},
			},
		},
		OutputExample: api.TypedToolOutput[api.TypedBuildOutput, api.BuildDetails]{
			Success: true,
			Data: api.TypedBuildOutput{
				Success:   true,
				SessionID: "example-session-123",
				ImageID:   "sha256:abc123...",
				Digest:    "sha256:def456...",
				Tags:      []string{"myapp:latest", "myapp:1.0.0"},
			},
		},
		Tags:     []string{"build", "docker", "container"},
		Category: api.CategoryBuild,
	}
}

// validateInput validates the typed input
func (t *TypeSafeBuildImageTool) validateInput(input api.TypedToolInput[api.TypedBuildInput, api.BuildContext]) error {
	if input.SessionID == "" {
		return errors.NewError().
			Code(errors.CodeInvalidParameter).
			Message("Session ID is required").
			Type(errors.ErrTypeValidation).
			Severity(errors.SeverityMedium).
			Build()
	}

	if input.Data.Image == "" {
		return errors.NewError().
			Code(errors.CodeInvalidParameter).
			Message("Image name is required").
			Type(errors.ErrTypeValidation).
			Severity(errors.SeverityMedium).
			Build()
	}

	if input.Data.Dockerfile == "" && input.Data.ContextPath == "" {
		return errors.NewError().
			Code(errors.CodeInvalidParameter).
			Message("Either Dockerfile or context path is required").
			Type(errors.ErrTypeValidation).
			Severity(errors.SeverityMedium).
			Build()
	}

	return nil
}

// prepareBuildContext prepares the build context from typed input
func (t *TypeSafeBuildImageTool) prepareBuildContext(input api.TypedToolInput[api.TypedBuildInput, api.BuildContext]) TypeSafeBuildContext {
	return TypeSafeBuildContext{
		DockerfilePath: input.Data.Dockerfile,
		ContextPath:    input.Data.ContextPath,
		ImageName:      input.Data.Image,
		Tags:           input.Data.Tags,
		BuildArgs:      input.Data.BuildArgs,
		NoCache:        input.Data.NoCache,
		Platform:       input.Data.Platform,
		Registry:       input.Context.Registry,
		Labels:         input.Context.Labels,
		CacheFrom:      input.Context.CacheFrom,
	}
}

// executeBuild performs the actual build operation
func (t *TypeSafeBuildImageTool) executeBuild(ctx context.Context, buildCtx TypeSafeBuildContext) (*TypeSafeBuildResult, error) {
	// If we have an atomic tool, use it
	if t.atomicTool != nil {
		// TODO: Implement proper atomic tool conversion
		return nil, errors.NewError().Messagef("atomic tool execution not yet implemented for typed build tool").WithLocation(

		// Otherwise use the pipeline adapter - simplified implementation for now
		// TODO: Use actual pipeline adapter when available
		).Build()
	}

	return nil, errors.NewError().Messagef("pipeline adapter BuildImage not yet implemented").WithLocation(

	// convertBuildSteps converts internal build steps to API format
	).Build()
}

func (t *TypeSafeBuildImageTool) convertBuildSteps(steps []interface{}) []api.BuildStep {
	result := make([]api.BuildStep, 0, len(steps))
	for i := range steps {
		// Convert based on actual step structure
		result = append(result, api.BuildStep{
			Name:     fmt.Sprintf("Step %d", i+1),
			Duration: 0, // TODO: Extract from step data
			Success:  true,
			Error:    "",
		})
	}
	return result
}

// errorOutput creates an error output
func (t *TypeSafeBuildImageTool) errorOutput(sessionID, message string, err error) api.TypedToolOutput[api.TypedBuildOutput, api.BuildDetails] {
	return api.TypedToolOutput[api.TypedBuildOutput, api.BuildDetails]{
		Success: false,
		Data: api.TypedBuildOutput{
			Success:   false,
			SessionID: sessionID,
			ErrorMsg:  fmt.Sprintf("%s: %v", message, err),
		},
		Error: err.Error(),
	}
}

// TypeSafeBuildContext represents internal build context for this tool
type TypeSafeBuildContext struct {
	SessionID      string
	DockerfilePath string
	ContextPath    string
	ImageName      string
	Tags           []string
	BuildArgs      map[string]string
	NoCache        bool
	Platform       string
	Registry       string
	Labels         map[string]string
	CacheFrom      []string
}

// TypeSafeBuildResult represents internal build result for this tool
type TypeSafeBuildResult struct {
	ImageID    string
	Digest     string
	Size       int64
	LayerCount int
	BaseImage  string
	CacheHit   bool
	Steps      []interface{}
	MemoryUsed int64
	NetworkIO  int64
	DiskIO     int64
}

// getOrCreateSession gets or creates a session using appropriate interface (service or legacy)
func (t *TypeSafeBuildImageTool) getOrCreateSession(ctx context.Context, sessionID string) (*session.SessionState, error) {
	// If service interfaces are available, use them (modern pattern)
	if t.sessionStore != nil && t.sessionState != nil {
		// Try to get existing session first
		sessionData, err := t.sessionStore.Get(ctx, sessionID)
		if err != nil {
			// Create new session if it doesn't exist
			newSessionID, err := t.sessionStore.Create(ctx, map[string]interface{}{
				"tool": "typesafe_build_image",
				"type": "build",
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create session: %w", err)
			}
			sessionData, err = t.sessionStore.Get(ctx, newSessionID)
			if err != nil {
				return nil, fmt.Errorf("failed to get created session: %w", err)
			}
		}

		// Convert to session.SessionState for compatibility
		return &session.SessionState{
			SessionID: sessionData.ID,
		}, nil
	}

	// Fall back to legacy unified session manager
	if t.sessionManager != nil {
		return t.sessionManager.GetOrCreateSession(ctx, sessionID)
	}

	return nil, fmt.Errorf("no session management interface available")
}
