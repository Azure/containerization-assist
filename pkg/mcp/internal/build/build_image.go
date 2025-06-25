package build

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Azure/container-copilot/pkg/core/analysis"
	mcptypes "github.com/Azure/container-copilot/pkg/mcp/internal/types"
	types "github.com/Azure/container-copilot/pkg/mcp/types"
	"github.com/Azure/container-copilot/pkg/pipeline"
	"github.com/Azure/container-copilot/pkg/pipeline/dockerstage"
	"github.com/rs/zerolog"
)

// BuildImageArgs defines the arguments for building a Docker image
type BuildImageArgs struct {
	mcptypes.BaseToolArgs
	ImageName    string            `json:"image_name,omitempty" description:"Image name"`
	Registry     string            `json:"registry,omitempty" description:"Registry URL"`
	BuildArgs    map[string]string `json:"build_args,omitempty" description:"Docker build arguments"`
	NoCache      bool              `json:"no_cache,omitempty" description:"Build without cache"`
	Platform     string            `json:"platform,omitempty" description:"Target platform (e.g., linux/amd64)"`
	BuildTimeout time.Duration     `json:"build_timeout,omitempty" description:"Build timeout (default: 10m)"`
	AsyncBuild   bool              `json:"async_build,omitempty" description:"Run build asynchronously"`
}

// BuildImageResult represents the result of a Docker image build
type BuildImageResult struct {
	mcptypes.BaseToolResponse
	Success       bool                `json:"success"`
	JobID         string              `json:"job_id,omitempty"` // For async builds
	ImageID       string              `json:"image_id,omitempty"`
	ImageRef      string              `json:"image_ref"`
	Size          int64               `json:"size_bytes,omitempty"`
	LayerCount    int                 `json:"layer_count"`
	Logs          []string            `json:"logs"`
	Duration      time.Duration       `json:"duration"`
	CacheHitRatio float64             `json:"cache_hit_ratio"`
	Error         *mcptypes.ToolError `json:"error,omitempty"`

	// Enhanced context for the external AI
	DockerfileUsed string                   `json:"dockerfile_used,omitempty"`
	BuildStrategy  string                   `json:"build_strategy,omitempty"`
	BuildErrors    string                   `json:"build_errors,omitempty"`
	RepositoryInfo *analysis.AnalysisResult `json:"repository_info,omitempty"`
}

// BuildImageTool handles Docker image building operations by integrating with existing pipeline
type BuildImageTool struct {
	sessionManager  BuildImageSessionManager
	pipelineAdapter BuildImagePipelineAdapter
	clients         interface{}
	logger          zerolog.Logger
}

// BuildImageSessionManager interface for managing session state
type BuildImageSessionManager interface {
	GetSession(sessionID string) (*BuildImageSession, error)
	SaveSession(session *BuildImageSession) error
	GetBaseDir() string
}

// BuildImagePipelineAdapter interface for converting between MCP and pipeline state
type BuildImagePipelineAdapter interface {
	ConvertToDockerState(sessionID, imageName, registryURL string) (*pipeline.PipelineState, error)
	UpdateSessionFromDockerResults(sessionID string, pipelineState *pipeline.PipelineState) error
	GetSessionWorkspace(sessionID string) string
}

// BuildImageSession represents the current session state
type BuildImageSession struct {
	ID        string                  `json:"id"`
	State     *BuildImageSessionState `json:"state"`
	CreatedAt time.Time               `json:"created_at"`
	UpdatedAt time.Time               `json:"updated_at"`
}

// BuildImageSessionState holds the current state of containerization progress
type BuildImageSessionState struct {
	RepositoryAnalysis   *analysis.AnalysisResult `json:"repository_analysis,omitempty"`
	DockerfileGeneration *DockerfileGeneration    `json:"dockerfile_generation,omitempty"`
	BuildAttempts        []BuildAttempt           `json:"build_attempts,omitempty"`
	CurrentStage         string                   `json:"current_stage"`
}

// DockerfileGeneration tracks Dockerfile generation state
type DockerfileGeneration struct {
	Content     string    `json:"content"`
	Template    string    `json:"template,omitempty"`
	GeneratedAt time.Time `json:"generated_at"`
}

// BuildAttempt tracks each build attempt
type BuildAttempt struct {
	ImageReference string        `json:"image_reference"`
	Success        bool          `json:"success"`
	ErrorMessage   string        `json:"error_message,omitempty"`
	BuildLogs      string        `json:"build_logs,omitempty"`
	Duration       time.Duration `json:"duration"`
	Timestamp      time.Time     `json:"timestamp"`
}

// NewBuildImageTool creates a new build image tool
func NewBuildImageTool(
	sessionManager BuildImageSessionManager,
	pipelineAdapter BuildImagePipelineAdapter,
	clients interface{},
	logger zerolog.Logger,
) *BuildImageTool {
	return &BuildImageTool{
		sessionManager:  sessionManager,
		pipelineAdapter: pipelineAdapter,
		clients:         clients,
		logger:          logger.With().Str("component", "build_image_tool").Logger(),
	}
}

// ExecuteTyped builds a Docker image using the existing pipeline logic
func (t *BuildImageTool) ExecuteTyped(ctx context.Context, args BuildImageArgs) (*BuildImageResult, error) {
	startTime := time.Now()

	// Create base response
	response := &BuildImageResult{
		BaseToolResponse: mcptypes.NewBaseResponse("build_image", args.SessionID, args.DryRun),
		ImageRef:         t.normalizeImageRef(args),
		Logs:             make([]string, 0),
	}

	// Handle dry-run
	if args.DryRun {
		response.Success = true
		response.Logs = append(response.Logs, "DRY-RUN: Would build Docker image")
		response.Logs = append(response.Logs, fmt.Sprintf("DRY-RUN: Image reference: %s", response.ImageRef))
		response.Logs = append(response.Logs, "DRY-RUN: Would check for Dockerfile in workspace")
		response.Logs = append(response.Logs, "DRY-RUN: Would validate build context")

		if args.AsyncBuild {
			response.JobID = fmt.Sprintf("build_job_%d", time.Now().UnixNano())
			response.Logs = append(response.Logs, fmt.Sprintf("DRY-RUN: Would create async job: %s", response.JobID))
		}

		response.Duration = time.Since(startTime)
		return response, nil
	}

	t.logger.Info().Str("session_id", args.SessionID).Str("image_name", args.ImageName).Msg("Starting Docker build")

	// Convert MCP arguments to pipeline state
	pipelineState, err := t.pipelineAdapter.ConvertToDockerState(args.SessionID, args.ImageName, args.Registry)
	if err != nil {
		t.logger.Error().Err(err).Msg("Failed to convert to pipeline state")
		response.Error = &mcptypes.ToolError{
			Type:    "validation_error",
			Message: fmt.Sprintf("Failed to prepare build context: %v", err),
		}
		response.Success = false
		response.Duration = time.Since(startTime)
		return response, nil
	}

	// Check if Dockerfile exists in session state
	if pipelineState.Dockerfile.Content == "" {
		response.Error = &mcptypes.ToolError{
			Type:    "validation_error",
			Message: "Dockerfile not found in session. Run generate_dockerfile first.",
		}
		response.Success = false
		response.Duration = time.Since(startTime)
		return response, nil
	}

	response.DockerfileUsed = pipelineState.Dockerfile.Content
	// Get repository info from metadata if available
	if repoAnalysis, ok := pipelineState.Metadata[pipeline.RepoAnalysisResultKey]; ok {
		if analysis, ok := repoAnalysis.(*analysis.AnalysisResult); ok {
			response.RepositoryInfo = analysis
		}
	}
	response.Logs = append(response.Logs, "Found Dockerfile in session context")
	response.Logs = append(response.Logs, fmt.Sprintf("Building image: %s", response.ImageRef))

	// Get workspace directory for this session
	workspaceDir := t.pipelineAdapter.GetSessionWorkspace(args.SessionID)

	// Set build options on pipeline state
	if pipelineState.Metadata == nil {
		pipelineState.Metadata = make(map[pipeline.MetadataKey]any)
	}
	pipelineState.Metadata[pipeline.MetadataKey("no_cache")] = args.NoCache
	pipelineState.Metadata[pipeline.MetadataKey("platform")] = args.Platform
	pipelineState.Metadata[pipeline.MetadataKey("build_args")] = args.BuildArgs

	// Create Docker stage with nil AI client (MCP mode doesn't use external AI)
	// The hosting LLM provides all reasoning; pipeline should work without AI client
	dockerStage := &dockerstage.DockerStage{
		AIClient:         nil, // No external AI in MCP - hosting LLM handles reasoning
		UseDraftTemplate: true,
		Parser:           &pipeline.DefaultParser{},
	}

	// Set up runner options for the pipeline stage
	runnerOptions := pipeline.RunnerOptions{
		TargetDirectory: workspaceDir,
	}

	// Check if this should be async based on timeout
	buildTimeout := args.BuildTimeout
	if buildTimeout == 0 {
		buildTimeout = 10 * time.Minute
	}

	if args.AsyncBuild || buildTimeout > 2*time.Minute {
		// Start async build
		jobID := fmt.Sprintf("build_job_%d", time.Now().UnixNano())
		response.JobID = jobID
		response.Success = true
		response.Logs = append(response.Logs, fmt.Sprintf("Starting async build with job ID: %s", jobID))
		
		// Start async build in goroutine
		go func() {
			t.logger.Info().Str("job_id", jobID).Msg("Starting async build process")
			asyncErr := t.executeAsyncBuild(ctx, args, pipelineState, dockerStage, runnerOptions, jobID)
			if asyncErr != nil {
				t.logger.Error().Err(asyncErr).Str("job_id", jobID).Msg("Async build failed")
			} else {
				t.logger.Info().Str("job_id", jobID).Msg("Async build completed successfully")
			}
		}()
		
		response.Duration = time.Since(startTime)
		return response, nil
	}

	response.Logs = append(response.Logs, "Starting Docker build using existing pipeline...")

	// Execute the Docker stage using existing pipeline logic
	err = dockerStage.Run(ctx, pipelineState, t.clients, runnerOptions)
	if err != nil {
		t.logger.Error().Err(err).Msg("Docker stage execution failed")

		// Extract error details for the external AI to reason about
		buildErrors := dockerStage.GetErrors(pipelineState)

		response.Success = false
		response.BuildErrors = buildErrors
		response.Logs = append(response.Logs, "Build failed with errors:")
		response.Logs = append(response.Logs, buildErrors)

		response.Error = &mcptypes.ToolError{
			Type:    "execution_error",
			Message: fmt.Sprintf("Docker build failed: %v", err),
		}

		// Still update session with partial results for next attempt
		if updateErr := t.pipelineAdapter.UpdateSessionFromDockerResults(args.SessionID, pipelineState); updateErr != nil {
			t.logger.Warn().Err(updateErr).Msg("Failed to update session with partial results")
		}

		response.Duration = time.Since(startTime)
		return response, nil
	}

	// Build succeeded!
	response.Success = true
	response.ImageID = pipelineState.ImageName // The pipeline sets this to the actual image ID
	response.ImageRef = fmt.Sprintf("%s/%s:latest", args.Registry, args.ImageName)
	response.BuildStrategy = "AI-powered iterative build with error fixing"
	response.Logs = append(response.Logs, "Docker build completed successfully")
	response.Logs = append(response.Logs, fmt.Sprintf("Image ID: %s", response.ImageID))

	// Update session with successful build results
	if err := t.pipelineAdapter.UpdateSessionFromDockerResults(args.SessionID, pipelineState); err != nil {
		t.logger.Error().Err(err).Msg("Failed to update session with build results")
		response.Error = &mcptypes.ToolError{
			Type:    "execution_error",
			Message: fmt.Sprintf("Failed to save build results: %v", err),
		}
		response.Success = false
		response.Duration = time.Since(startTime)
		return response, nil
	}

	response.Duration = time.Since(startTime)

	t.logger.Info().
		Str("session_id", args.SessionID).
		Str("image_ref", response.ImageRef).
		Dur("duration", response.Duration).
		Msg("Docker build completed successfully")

	return response, nil
}

// normalizeImageRef creates a normalized image reference string
func (t *BuildImageTool) normalizeImageRef(args BuildImageArgs) string {
	imageName := args.ImageName
	if imageName == "" {
		imageName = "my-app"
	}

	registry := args.Registry
	if registry == "" {
		// Use local registry or default
		return fmt.Sprintf("%s:latest", imageName)
	}

	return fmt.Sprintf("%s/%s:latest", registry, imageName)
}

// executeAsyncBuild runs the build process asynchronously
func (t *BuildImageTool) executeAsyncBuild(ctx context.Context, args BuildImageArgs, pipelineState *pipeline.PipelineState, dockerStage *dockerstage.DockerStage, runnerOptions pipeline.RunnerOptions, jobID string) error {
	
	// Create a new context with timeout for the async build
	buildTimeout := args.BuildTimeout
	if buildTimeout == 0 {
		buildTimeout = 10 * time.Minute
	}
	
	asyncCtx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()
	
	t.logger.Info().
		Str("job_id", jobID).
		Str("session_id", args.SessionID).
		Dur("timeout", buildTimeout).
		Msg("Executing async Docker build")
	
	// Execute the Docker stage using existing pipeline logic
	err := dockerStage.Run(asyncCtx, pipelineState, t.clients, runnerOptions)
	if err != nil {
		t.logger.Error().
			Err(err).
			Str("job_id", jobID).
			Msg("Async Docker stage execution failed")
		
		// Store build failure in session for later retrieval
		if updateErr := t.pipelineAdapter.UpdateSessionFromDockerResults(args.SessionID, pipelineState); updateErr != nil {
			t.logger.Warn().Err(updateErr).Str("job_id", jobID).Msg("Failed to update session with async build failure")
		}
		
		return err
	}
	
	// Build succeeded - update session with results
	t.logger.Info().
		Str("job_id", jobID).
		Str("image_id", pipelineState.ImageName).
		Msg("Async build completed successfully")
	
	if err := t.pipelineAdapter.UpdateSessionFromDockerResults(args.SessionID, pipelineState); err != nil {
		t.logger.Error().
			Err(err).
			Str("job_id", jobID).
			Msg("Failed to update session with async build results")
		return err
	}
	
	return nil
}

// Execute implements the unified Tool interface
func (t *BuildImageTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	// Convert generic args to typed args
	var buildArgs BuildImageArgs

	switch a := args.(type) {
	case BuildImageArgs:
		buildArgs = a
	case map[string]interface{}:
		// Convert from map to struct using JSON marshaling
		jsonData, err := json.Marshal(a)
		if err != nil {
			return nil, mcptypes.NewRichError("INVALID_ARGUMENTS", "Failed to marshal arguments", "validation_error")
		}
		if err = json.Unmarshal(jsonData, &buildArgs); err != nil {
			return nil, mcptypes.NewRichError("INVALID_ARGUMENTS", "Invalid argument structure for build_image", "validation_error")
		}
	default:
		return nil, mcptypes.NewRichError("INVALID_ARGUMENTS", "Invalid argument type for build_image", "validation_error")
	}

	// Call the typed execute method
	return t.ExecuteTyped(ctx, buildArgs)
}

// Validate implements the unified Tool interface
func (t *BuildImageTool) Validate(ctx context.Context, args interface{}) error {
	var buildArgs BuildImageArgs

	switch a := args.(type) {
	case BuildImageArgs:
		buildArgs = a
	case map[string]interface{}:
		// Convert from map to struct using JSON marshaling
		jsonData, err := json.Marshal(a)
		if err != nil {
			return mcptypes.NewRichError("INVALID_ARGUMENTS", "Failed to marshal arguments", "validation_error")
		}
		if err = json.Unmarshal(jsonData, &buildArgs); err != nil {
			return mcptypes.NewRichError("INVALID_ARGUMENTS", "Invalid argument structure for build_image", "validation_error")
		}
	default:
		return mcptypes.NewRichError("INVALID_ARGUMENTS", "Invalid argument type for build_image", "validation_error")
	}

	// Validate required fields
	if buildArgs.SessionID == "" {
		return mcptypes.NewRichError("INVALID_ARGUMENTS", "session_id is required", "validation_error")
	}

	return nil
}

// GetMetadata implements the unified Tool interface
func (t *BuildImageTool) GetMetadata() types.ToolMetadata {
	return types.ToolMetadata{
		Name:         "build_image",
		Description:  "Builds Docker images with AI-powered error fixing and iterative optimization",
		Version:      "1.0.0",
		Category:     "build",
		Dependencies: []string{"generate_dockerfile"},
		Capabilities: []string{
			"docker_build",
			"ai_error_fixing",
			"iterative_optimization",
			"multi_platform_support",
			"build_caching",
			"async_builds",
			"build_args_support",
		},
		Requirements: []string{
			"docker_daemon",
			"dockerfile_exists",
			"session_workspace",
		},
		Parameters: map[string]string{
			"session_id":    "Required session identifier",
			"image_name":    "Image name (optional, defaults to 'my-app')",
			"registry":      "Registry URL (optional)",
			"build_args":    "Docker build arguments (optional)",
			"no_cache":      "Build without cache (optional)",
			"platform":      "Target platform (e.g., linux/amd64) (optional)",
			"build_timeout": "Build timeout (default: 10m) (optional)",
			"async_build":   "Run build asynchronously (optional)",
		},
		Examples: []types.ToolExample{
			{
				Name:        "Basic Build",
				Description: "Build a Docker image from session workspace",
				Input: map[string]interface{}{
					"session_id": "build-session",
					"image_name": "my-app",
				},
				Output: map[string]interface{}{
					"success":   true,
					"image_ref": "my-app:latest",
					"image_id":  "sha256:abc123...",
				},
			},
			{
				Name:        "Build with Registry",
				Description: "Build and tag for specific registry",
				Input: map[string]interface{}{
					"session_id": "build-session",
					"image_name": "my-app",
					"registry":   "myregistry.azurecr.io",
					"build_args": map[string]string{
						"NODE_VERSION": "18",
					},
				},
				Output: map[string]interface{}{
					"success":   true,
					"image_ref": "myregistry.azurecr.io/my-app:latest",
				},
			},
		},
	}
}
