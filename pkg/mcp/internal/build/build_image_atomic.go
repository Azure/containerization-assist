package build

import (
	"context"
	"fmt"
	"time"

	// mcp import removed - using mcptypes

	coredocker "github.com/Azure/container-kit/pkg/core/docker"
	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/internal/observability"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"

	mcptypes "github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/localrivet/gomcp/server"
	"github.com/rs/zerolog"
)

// AtomicBuildImageArgs defines arguments for atomic Docker image building
type AtomicBuildImageArgs struct {
	types.BaseToolArgs
	ImageName      string            `json:"image_name" jsonschema:"required,pattern=^[a-zA-Z0-9][a-zA-Z0-9._/-]*$" description:"Docker image name (e.g., my-app)"`
	ImageTag       string            `json:"image_tag,omitempty" jsonschema:"pattern=^[a-zA-Z0-9][a-zA-Z0-9._-]*$" description:"Image tag (default: latest)"`
	DockerfilePath string            `json:"dockerfile_path,omitempty" description:"Path to Dockerfile (default: ./Dockerfile)"`
	BuildContext   string            `json:"build_context,omitempty" description:"Build context directory (default: session workspace)"`
	Platform       string            `json:"platform,omitempty" jsonschema:"enum=linux/amd64,linux/arm64,linux/arm/v7" description:"Target platform (default: linux/amd64)"`
	NoCache        bool              `json:"no_cache,omitempty" description:"Build without cache"`
	BuildArgs      map[string]string `json:"build_args,omitempty" description:"Docker build arguments"`
	PushAfterBuild bool              `json:"push_after_build,omitempty" description:"Push image after successful build"`
	RegistryURL    string            `json:"registry_url,omitempty" jsonschema:"pattern=^[a-zA-Z0-9][a-zA-Z0-9.-]*[a-zA-Z0-9](:[0-9]+)?$" description:"Registry URL for pushing (if push_after_build=true)"`
}

// AtomicBuildImageResult defines the response from atomic Docker image building
type AtomicBuildImageResult struct {
	types.BaseToolResponse
	mcptypes.BaseAIContextResult      // Embedded for AI context methods
	Success                      bool `json:"success"`
	// Session context
	SessionID    string `json:"session_id"`
	WorkspaceDir string `json:"workspace_dir"`
	// Build configuration
	ImageName      string `json:"image_name"`
	ImageTag       string `json:"image_tag"`
	FullImageRef   string `json:"full_image_ref"`
	DockerfilePath string `json:"dockerfile_path"`
	BuildContext   string `json:"build_context"`
	Platform       string `json:"platform"`
	// Build results from core operations
	BuildResult  *coredocker.BuildResult        `json:"build_result"`
	PushResult   *coredocker.RegistryPushResult `json:"push_result,omitempty"`
	SecurityScan *coredocker.ScanResult         `json:"security_scan,omitempty"`
	// Timing information
	BuildDuration time.Duration `json:"build_duration"`
	PushDuration  time.Duration `json:"push_duration,omitempty"`
	ScanDuration  time.Duration `json:"scan_duration,omitempty"`
	TotalDuration time.Duration `json:"total_duration"`
	// Rich context for Claude reasoning
	BuildContext_Info *BuildContextInfo `json:"build_context_info"`
	// AI context for decision-making
	BuildFailureAnalysis *BuildFailureAnalysis `json:"build_failure_analysis,omitempty"`
	// Optimization results
	OptimizationResult *OptimizationResult `json:"optimization_result,omitempty"`
	// Performance metrics
	PerformanceReport *BuildPerformanceReport `json:"performance_report,omitempty"`
}

// AtomicBuildImageTool is the main tool for atomic Docker image building
type AtomicBuildImageTool struct {
	pipelineAdapter mcptypes.PipelineOperations
	sessionManager  core.ToolSessionManager
	logger          zerolog.Logger
	// Module components
	contextAnalyzer *BuildContextAnalyzer
	validator       *BuildValidatorImpl
	executor        *BuildExecutorService
	fixingMixin     *AtomicToolFixingMixin
}

// NewAtomicBuildImageTool creates a new atomic build image tool
func NewAtomicBuildImageTool(adapter mcptypes.PipelineOperations, sessionManager core.ToolSessionManager, logger zerolog.Logger) *AtomicBuildImageTool {
	toolLogger := logger.With().Str("tool", "atomic_build_image").Logger()
	// Initialize all modules
	contextAnalyzer := NewBuildContextAnalyzer(toolLogger)
	validator := NewBuildValidator(toolLogger)
	executor := NewBuildExecutor(adapter, sessionManager, toolLogger)
	return &AtomicBuildImageTool{
		pipelineAdapter: adapter,
		sessionManager:  sessionManager,
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

// ExecuteWithFixes runs the atomic Docker image build with AI-driven fixing capabilities
func (t *AtomicBuildImageTool) ExecuteWithFixes(ctx context.Context, args AtomicBuildImageArgs) (*AtomicBuildImageResult, error) {
	// Delegate to executor with fixing mixin
	if t.fixingMixin != nil {
		return t.executor.ExecuteWithFixes(ctx, args, t.fixingMixin)
	}
	return t.executor.ExecuteWithFixes(ctx, args, nil)
}

// ExecuteWithContext executes the tool with GoMCP server context for native progress tracking
func (t *AtomicBuildImageTool) ExecuteWithContext(serverCtx *server.Context, args AtomicBuildImageArgs) (*AtomicBuildImageResult, error) {
	startTime := time.Now()
	// Create result object early for error handling
	result := &AtomicBuildImageResult{
		BaseToolResponse:    types.NewBaseResponse("atomic_build_image", args.SessionID, args.DryRun),
		BaseAIContextResult: mcptypes.NewBaseAIContextResult("build", false, 0), // Duration will be updated later
		SessionID:           args.SessionID,
		ImageName:           args.ImageName,
		ImageTag:            t.getImageTag(args.ImageTag),
		Platform:            t.getPlatform(args.Platform),
		BuildContext_Info:   &BuildContextInfo{},
	}
	// Use centralized build stages for progress tracking
	progress := observability.NewUnifiedProgressReporter(serverCtx)

	// Delegate to executor with progress tracking
	ctx := context.Background()
	err := t.executor.executeWithProgress(ctx, args, result, startTime, progress)
	// Always set total duration
	result.TotalDuration = time.Since(startTime)
	// Complete progress tracking
	if err != nil {
		t.logger.Info().Msg("Build failed")
		result.Success = false
		return result, nil // Return result with error info, not the error itself
	} else {
		t.logger.Info().Msg("Build completed successfully")
	}
	return result, nil
}

// Tool interface implementation (unified MCP interface)
// GetMetadata returns comprehensive tool metadata
func (t *AtomicBuildImageTool) GetMetadata() core.ToolMetadata {
	return core.ToolMetadata{
		Name:         "atomic_build_image",
		Description:  "Builds Docker images atomically with multi-stage support, caching optimization, and security scanning",
		Version:      "1.0.0",
		Category:     "docker",
		Dependencies: []string{"docker"},
		Capabilities: []string{
			"supports_dry_run",
			"supports_streaming",
			"long_running",
		},
		Requirements: []string{"docker_daemon", "build_context"},
		Parameters: map[string]string{
			"image_name":       "required - Docker image name",
			"image_tag":        "optional - Image tag (default: latest)",
			"dockerfile_path":  "optional - Path to Dockerfile",
			"build_context":    "optional - Build context directory",
			"platform":         "optional - Target platform (default: linux/amd64)",
			"no_cache":         "optional - Build without cache",
			"build_args":       "optional - Docker build arguments",
			"push_after_build": "optional - Push image after build",
			"registry_url":     "optional - Registry URL for pushing",
		},
		Examples: []mcptypes.ToolExample{
			{
				Name:        "basic_build",
				Description: "Build a basic Docker image",
				Input: map[string]interface{}{
					"session_id": "session-123",
					"image_name": "my-app",
					"image_tag":  "v1.0.0",
				},
				Output: map[string]interface{}{
					"success":        true,
					"full_image_ref": "my-app:v1.0.0",
					"build_duration": "30s",
				},
			},
		},
	}
}

// Validate validates the tool arguments (unified interface)
func (t *AtomicBuildImageTool) Validate(ctx context.Context, args interface{}) error {
	buildArgs, ok := args.(AtomicBuildImageArgs)
	if !ok {
		return fmt.Errorf("invalid argument type for atomic_build_image: expected AtomicBuildImageArgs, got %T", args)
	}
	if buildArgs.ImageName == "" {
		return fmt.Errorf("validation error")
	}
	if buildArgs.SessionID == "" {
		return fmt.Errorf("validation error")
	}
	return nil
}

// Execute implements unified Tool interface
func (t *AtomicBuildImageTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	buildArgs, ok := args.(AtomicBuildImageArgs)
	if !ok {
		return nil, fmt.Errorf("invalid argument type for atomic_build_image: expected AtomicBuildImageArgs, got %T", args)
	}
	// Execute with nil server context (no progress tracking)
	return t.ExecuteWithContext(nil, buildArgs)
}
