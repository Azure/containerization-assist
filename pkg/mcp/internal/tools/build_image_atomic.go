package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	coredocker "github.com/Azure/container-copilot/pkg/core/docker"
	"github.com/Azure/container-copilot/pkg/mcp/internal/api/contract"
	"github.com/Azure/container-copilot/pkg/mcp/internal/constants"
	"github.com/Azure/container-copilot/pkg/mcp/internal/fixing"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
	"github.com/localrivet/gomcp/server"
	"github.com/rs/zerolog"
)

// Import the unified interface types
// These will be available from "github.com/Azure/container-copilot/pkg/mcp" after interface migration

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
	BaseAIContextResult      // Embedded for AI context methods
	Success             bool `json:"success"`

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
}

// AtomicBuildImageTool is the main tool for atomic Docker image building
type AtomicBuildImageTool struct {
	pipelineAdapter mcptypes.PipelineOperations
	sessionManager  mcptypes.ToolSessionManager
	logger          zerolog.Logger

	// Module components
	contextAnalyzer *BuildContextAnalyzer
	validator       *BuildValidator
	executor        *BuildExecutor
	fixingMixin     *fixing.AtomicToolFixingMixin
}

// NewAtomicBuildImageTool creates a new atomic build image tool
func NewAtomicBuildImageTool(adapter mcptypes.PipelineOperations, sessionManager mcptypes.ToolSessionManager, logger zerolog.Logger) *AtomicBuildImageTool {
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
		fixingMixin:     nil, // Will be set via SetAnalyzer if fixing is enabled
	}
}

// SetAnalyzer enables AI-driven fixing capabilities by providing an analyzer
func (t *AtomicBuildImageTool) SetAnalyzer(analyzer mcptypes.AIAnalyzer) {
	if analyzer != nil {
		t.fixingMixin = fixing.NewAtomicToolFixingMixin(analyzer, "atomic_build_image", t.logger)
	}
}

// ExecuteWithFixes runs the atomic Docker image build with AI-driven fixing capabilities
func (t *AtomicBuildImageTool) ExecuteWithFixes(ctx context.Context, args AtomicBuildImageArgs) (*AtomicBuildImageResult, error) {
	// Delegate to executor with fixing mixin
	return t.executor.ExecuteWithFixes(ctx, args, t.fixingMixin)
}

// ExecuteBuild runs the atomic Docker image build (deprecated: use ExecuteWithContext)
func (t *AtomicBuildImageTool) ExecuteBuild(ctx context.Context, args AtomicBuildImageArgs) (*AtomicBuildImageResult, error) {
	// Use executor for backward compatibility
	return t.executor.ExecuteBuild(ctx, args)
}

// ExecuteWithContext executes the tool with GoMCP server context for native progress tracking
func (t *AtomicBuildImageTool) ExecuteWithContext(serverCtx *server.Context, args AtomicBuildImageArgs) (*AtomicBuildImageResult, error) {
	startTime := time.Now()

	// Create result object early for error handling
	result := &AtomicBuildImageResult{
		BaseToolResponse:    types.NewBaseResponse("atomic_build_image", args.SessionID, args.DryRun),
		BaseAIContextResult: NewBaseAIContextResult("build", false, 0), // Duration will be updated later
		SessionID:           args.SessionID,
		ImageName:           args.ImageName,
		ImageTag:            t.getImageTag(args.ImageTag),
		Platform:            t.getPlatform(args.Platform),
		BuildContext_Info:   &BuildContextInfo{},
	}

	// Use centralized build stages for progress tracking
	// Progress adapter removed

	// Delegate to executor with progress tracking
	ctx := context.Background()
	err := t.executor.executeWithProgress(ctx, args, result, startTime, nil)

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
func (t *AtomicBuildImageTool) GetMetadata() mcptypes.ToolMetadata {
	return mcptypes.ToolMetadata{
		Name:         "atomic_build_image",
		Description:  "Builds Docker images atomically with multi-stage support, caching optimization, and security scanning",
		Version:      constants.AtomicToolVersion,
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
		return types.NewValidationErrorBuilder("Invalid argument type for atomic_build_image", "args", args).
			WithField("expected", "AtomicBuildImageArgs").
			WithField("received", fmt.Sprintf("%T", args)).
			Build()
	}

	if buildArgs.ImageName == "" {
		return types.NewValidationErrorBuilder("ImageName is required", "image_name", buildArgs.ImageName).
			WithField("field", "image_name").
			Build()
	}

	if buildArgs.SessionID == "" {
		return types.NewValidationErrorBuilder("SessionID is required", "session_id", buildArgs.SessionID).
			WithField("field", "session_id").
			Build()
	}

	return nil
}

// Execute implements unified Tool interface
func (t *AtomicBuildImageTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	buildArgs, ok := args.(AtomicBuildImageArgs)
	if !ok {
		return nil, types.NewValidationErrorBuilder("Invalid argument type for atomic_build_image", "args", args).
			WithField("expected", "AtomicBuildImageArgs").
			WithField("received", fmt.Sprintf("%T", args)).
			Build()
	}

	// Call the typed Execute method
	return t.ExecuteTyped(ctx, buildArgs)
}

// Legacy interface methods for backward compatibility

// GetName returns the tool name (legacy SimpleTool compatibility)
func (t *AtomicBuildImageTool) GetName() string {
	return t.GetMetadata().Name
}

// GetDescription returns the tool description (legacy SimpleTool compatibility)
func (t *AtomicBuildImageTool) GetDescription() string {
	return t.GetMetadata().Description
}

// GetVersion returns the tool version (legacy SimpleTool compatibility)
func (t *AtomicBuildImageTool) GetVersion() string {
	return t.GetMetadata().Version
}

// GetCapabilities returns the tool capabilities (legacy SimpleTool compatibility)
func (t *AtomicBuildImageTool) GetCapabilities() contract.ToolCapabilities {
	return contract.ToolCapabilities{
		SupportsDryRun:    true,
		SupportsStreaming: true,
		IsLongRunning:     true,
		RequiresAuth:      false,
	}
}

// ExecuteTyped provides the original typed execute method
func (t *AtomicBuildImageTool) ExecuteTyped(ctx context.Context, args AtomicBuildImageArgs) (*AtomicBuildImageResult, error) {
	return t.ExecuteBuild(ctx, args)
}

// Helper methods

func (t *AtomicBuildImageTool) getImageTag(tag string) string {
	if tag == "" {
		return constants.DefaultImageTag
	}
	return tag
}

func (t *AtomicBuildImageTool) getPlatform(platform string) string {
	if platform == "" {
		return constants.DefaultPlatform
	}
	return platform
}

func (t *AtomicBuildImageTool) getBuildContext(context, workspaceDir string) string {
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

func (t *AtomicBuildImageTool) getDockerfilePath(dockerfilePath, buildContext string) string {
	if dockerfilePath == "" {
		return filepath.Join(buildContext, "Dockerfile")
	}

	// If relative path, make it relative to build context
	if !filepath.IsAbs(dockerfilePath) {
		return filepath.Join(buildContext, dockerfilePath)
	}

	return dockerfilePath
}
