package build

import (
	"context"
	"time"

	// mcp import removed - using mcptypes

	coredocker "github.com/Azure/container-kit/pkg/core/docker"
	"github.com/Azure/container-kit/pkg/mcp/application/api"

	"github.com/Azure/container-kit/pkg/mcp/application/core"
	"github.com/Azure/container-kit/pkg/mcp/domain/types"
	toolstypes "github.com/Azure/container-kit/pkg/mcp/domain/types/tools"
	"github.com/Azure/container-kit/pkg/mcp/domain/validation"
	"github.com/Azure/container-kit/pkg/mcp/services"

	mcptypes "github.com/Azure/container-kit/pkg/mcp/application/core"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/localrivet/gomcp/server"
	"log/slog"
)

// AtomicBuildImageArgs defines arguments for atomic Docker image building
type AtomicBuildImageArgs struct {
	types.BaseToolArgs
	ImageName      string            `json:"image_name" validate:"required,docker_image" description:"Docker image name (e.g., my-app)"`
	ImageTag       string            `json:"image_tag,omitempty" validate:"omitempty,docker_tag" description:"Image tag (default: latest)"`
	DockerfilePath string            `json:"dockerfile_path,omitempty" validate:"omitempty,secure_path" description:"Path to Dockerfile (default: ./Dockerfile)"`
	BuildContext   string            `json:"build_context,omitempty" validate:"omitempty,secure_path" description:"Build context directory (default: session workspace)"`
	Platform       string            `json:"platform,omitempty" validate:"omitempty,platform" description:"Target platform (default: linux/amd64)"`
	NoCache        bool              `json:"no_cache,omitempty" description:"Build without cache"`
	BuildArgs      map[string]string `json:"build_args,omitempty" validate:"omitempty" description:"Docker build arguments"`
	PushAfterBuild bool              `json:"push_after_build,omitempty" description:"Push image after successful build"`
	RegistryURL    string            `json:"registry_url,omitempty" validate:"omitempty,registry_url" description:"Registry URL for pushing (if push_after_build=true)"`
	DryRun         bool              `json:"dry_run,omitempty" description:"Preview changes without executing"`
}

// AtomicBuildImageResult defines the response from atomic Docker image building
type AtomicBuildImageResult struct {
	types.BaseToolResponse
	core.BaseAIContextResult      // Embedded for AI context methods
	Success                  bool `json:"success"`
	// Session context
	SessionID    string `json:"session_id"`
	WorkspaceDir string `json:"workspace_dir"`
	// Build configuration
	ImageName            string                         `json:"image_name"`
	ImageTag             string                         `json:"image_tag"`
	FullImageRef         string                         `json:"full_image_ref"`
	DockerfilePath       string                         `json:"dockerfile_path"`
	BuildContext         string                         `json:"build_context"`
	Platform             string                         `json:"platform"`
	BuildResult          *coredocker.BuildResult        `json:"build_result"`
	PushResult           *coredocker.RegistryPushResult `json:"push_result,omitempty"`
	SecurityScan         *coredocker.ScanResult         `json:"security_scan,omitempty"`
	BuildDuration        time.Duration                  `json:"build_duration"`
	PushDuration         time.Duration                  `json:"push_duration,omitempty"`
	ScanDuration         time.Duration                  `json:"scan_duration,omitempty"`
	TotalDuration        time.Duration                  `json:"total_duration"`
	BuildContext_Info    *BuildContextInfo              `json:"build_context_info"`
	BuildFailureAnalysis *BuildFailureAnalysis          `json:"build_failure_analysis,omitempty"`
	OptimizationResult   *OptimizationResult            `json:"optimization_result,omitempty"`
}

// AtomicBuildImageTool is the main tool for atomic Docker image building
type AtomicBuildImageTool struct {
	pipelineAdapter mcptypes.TypedPipelineOperations
	sessionStore    services.SessionStore
	sessionState    services.SessionState
	logger          *slog.Logger
	contextAnalyzer *BuildContextAnalyzer
	validator       *BuildValidatorImpl
	executor        *BuildExecutorService
	fixingMixin     *AtomicToolFixingMixin
}

// NewAtomicBuildImageToolWithServices creates a new atomic build image tool using service interfaces
func NewAtomicBuildImageToolWithServices(adapter mcptypes.TypedPipelineOperations, serviceContainer services.ServiceContainer, logger *slog.Logger) *AtomicBuildImageTool {
	toolLogger := logger.With("tool", "atomic_build_image")

	// Initialize all modules
	contextAnalyzer := NewBuildContextAnalyzer(toolLogger)
	validator := NewBuildValidator(toolLogger)
	executor := NewBuildExecutorWithServices(adapter, serviceContainer.SessionStore(), serviceContainer.SessionState(), toolLogger)

	return &AtomicBuildImageTool{
		pipelineAdapter: adapter,
		sessionStore:    serviceContainer.SessionStore(),
		sessionState:    serviceContainer.SessionState(),
		logger:          toolLogger,
		contextAnalyzer: contextAnalyzer,
		validator:       validator,
		executor:        executor,
		fixingMixin:     nil, // Will be set via SetAnalyzer
	}
}

// SetAnalyzer enables AI-driven fixing capabilities by providing an analyzer
func (t *AtomicBuildImageTool) SetAnalyzer(analyzer core.AIAnalyzer) {
	if analyzer != nil {
		t.fixingMixin = NewAtomicToolFixingMixin(analyzer, "atomic_build_image", t.logger)
	}
}

// Name implements the api.Tool interface
func (t *AtomicBuildImageTool) Name() string {
	return "atomic_build_image"
}

// Description implements the api.Tool interface
func (t *AtomicBuildImageTool) Description() string {
	return "Builds Docker images atomically with session management and AI-driven error fixing capabilities"
}

// ExecuteWithFixes runs the atomic Docker image build with AI-driven fixing capabilities
func (t *AtomicBuildImageTool) ExecuteWithFixes(ctx context.Context, args AtomicBuildImageArgs) (*AtomicBuildImageResult, error) {
	// Delegate to executor with fixing mixin
	if t.fixingMixin != nil {
		return t.executor.ExecuteWithFixes(ctx, args, t.fixingMixin)
	}
	return t.executor.ExecuteWithFixes(ctx, args, nil)
}

// ExecuteWithContext executes the tool with GoMCP server context for native progress tracking
func (t *AtomicBuildImageTool) ExecuteWithContext(serverCtx *server.Context, args *AtomicBuildImageArgs) (*AtomicBuildImageResult, error) {
	startTime := time.Now()

	t.logger.Info("Starting atomic Docker image build",
		"image_name", args.ImageName,
		"session_id", args.SessionID)

	// Step 1: Handle session management using direct services
	var sessionID string
	var err error
	if args.SessionID != "" {
		// Try to get existing session
		_, err = t.sessionStore.Get(context.Background(), args.SessionID)
		if err != nil {
			// Create new session if not found
			sessionID, err = t.sessionStore.Create(context.Background(), map[string]interface{}{
				"tool": "atomic_build_image",
				"args": args,
			})
			if err != nil {
				return nil, errors.NewError().Message("failed to create session").Cause(err).Build()
			}
		} else {
			sessionID = args.SessionID
		}
	} else {
		// Create new session
		sessionID, err = t.sessionStore.Create(context.Background(), map[string]interface{}{
			"tool": "atomic_build_image",
			"args": args,
		})
		if err != nil {
			return nil, errors.NewError().Message("failed to create session").Cause(err).Build()
		}
	}

	t.logger.Info("Created or retrieved session for build",
		"session_id", sessionID)

	// Create result object early for error handling
	result := &AtomicBuildImageResult{
		BaseToolResponse:    types.BaseToolResponse{Success: false, Timestamp: time.Now()},
		BaseAIContextResult: core.NewBaseAIContextResult("build", false, 0), // Duration will be updated later
		SessionID:           sessionID,
		WorkspaceDir:        "/tmp/workspace", // Default workspace
		ImageName:           args.ImageName,
		ImageTag:            t.getImageTag(args.ImageTag),
		Platform:            t.getPlatform(args.Platform),
		BuildContext_Info:   &BuildContextInfo{},
	}

	// Progress tracking infrastructure removed

	// Step 2: Delegate to executor with progress tracking
	ctx := context.Background()
	// Progress parameter removed from executor call
	err = t.executor.executeWithProgress(ctx, *args, result, startTime, nil)

	// Step 3: Update session metadata with execution result
	if err := t.sessionStore.Update(context.Background(), sessionID, map[string]interface{}{
		"last_execution": time.Now(),
		"result":         result,
	}); err != nil {
		t.logger.Warn("Failed to update session metadata", "error", err)
	}

	// Always set total duration
	result.TotalDuration = time.Since(startTime)
	result.Success = (err == nil)

	// Complete progress tracking
	if err != nil {
		t.logger.Error("Build failed",
			"error", err,
			"session_id", sessionID,
			"duration", result.TotalDuration)
		return result, err
	} else {
		t.logger.Info("Build completed successfully",
			"session_id", sessionID,
			"image_ref", result.FullImageRef,
			"duration", result.TotalDuration)
	}
	return result, nil
}

// Tool interface implementation (unified MCP interface)
// GetMetadata returns comprehensive tool metadata
func (t *AtomicBuildImageTool) GetMetadata() api.ToolMetadata {
	return api.ToolMetadata{
		Name:         "atomic_build_image",
		Description:  "Builds Docker images atomically with multi-stage support, caching optimization, and security scanning. Uses session context for build configuration",
		Version:      "1.0.0",
		Category:     api.ToolCategory("docker"),
		Tags:         []string{"docker", "build", "container"},
		Status:       api.ToolStatus("active"),
		Dependencies: []string{"docker"},
		Capabilities: []string{
			"supports_dry_run",
			"supports_streaming",
			"long_running",
		},
		Requirements: []string{"docker_daemon", "build_context"},
		RegisteredAt: time.Now(),
		LastModified: time.Now(),
	}
}

// Validate validates the tool arguments (unified interface)
func (t *AtomicBuildImageTool) Validate(ctx context.Context, args interface{}) error {
	// Validate using tag-based validation
	return validation.ValidateTaggedStruct(args)
}

// Schema returns the JSON schema for this tool
func (t *AtomicBuildImageTool) Schema() interface{} {
	return AtomicBuildImageArgsSchema
}

// Execute implements unified Tool interface
func (t *AtomicBuildImageTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	var buildArgs AtomicBuildImageArgs

	switch v := args.(type) {
	case AtomicBuildImageArgs:
		buildArgs = v
	case *AtomicBuildImageArgs:
		buildArgs = *v
	case toolstypes.AtomicBuildImageParams:
		// Convert from typed parameters package to internal args structure
		buildArgs = AtomicBuildImageArgs{
			BaseToolArgs: types.BaseToolArgs{
				SessionID: v.SessionID,
				DryRun:    false, // Default value
			},
			ImageName:      v.ImageName,
			ImageTag:       v.ImageTag,
			DockerfilePath: v.DockerfilePath,
			BuildContext:   v.BuildContext,
			Platform:       v.Platform,
			NoCache:        v.NoCache,
			BuildArgs:      v.BuildArgs,
			PushAfterBuild: v.PushAfterBuild,
			RegistryURL:    v.RegistryURL,
		}
	case *toolstypes.AtomicBuildImageParams:
		// Convert from pointer to typed parameters
		buildArgs = AtomicBuildImageArgs{
			BaseToolArgs: types.BaseToolArgs{
				SessionID: v.SessionID,
				DryRun:    false, // Default value
			},
			ImageName:      v.ImageName,
			ImageTag:       v.ImageTag,
			DockerfilePath: v.DockerfilePath,
			BuildContext:   v.BuildContext,
			Platform:       v.Platform,
			NoCache:        v.NoCache,
			BuildArgs:      v.BuildArgs,
			PushAfterBuild: v.PushAfterBuild,
			RegistryURL:    v.RegistryURL,
		}
	default:
		return nil, errors.NewError().Messagef("invalid argument type for atomic_build_image: expected AtomicBuildImageArgs or AtomicBuildImageParams, got %T", args).WithLocation(

		// Execute with nil server context (no progress tracking)
		).Build()
	}

	return t.ExecuteWithContext(nil, &buildArgs)
}
