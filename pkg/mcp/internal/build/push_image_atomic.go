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

	"github.com/rs/zerolog"
)

// Note: Using centralized stage definitions from core.StandardPushStages()
// AtomicPushImageArgs defines arguments for atomic Docker image push
type AtomicPushImageArgs struct {
	types.BaseToolArgs
	// Image information
	ImageRef    string `json:"image_ref" jsonschema:"required,pattern=^[a-zA-Z0-9][a-zA-Z0-9._/-]*:[a-zA-Z0-9][a-zA-Z0-9._-]*$" description:"Full image reference to push (e.g., myregistry.azurecr.io/myapp:latest)"`
	RegistryURL string `json:"registry_url,omitempty" jsonschema:"pattern=^[a-zA-Z0-9][a-zA-Z0-9.-]*[a-zA-Z0-9](:[0-9]+)?$" description:"Override registry URL (optional - extracted from image_ref if not provided)"`
	// Push configuration
	Timeout    int  `json:"timeout,omitempty" jsonschema:"minimum=30,maximum=3600" description:"Push timeout in seconds (default: 600)"`
	RetryCount int  `json:"retry_count,omitempty" jsonschema:"minimum=0,maximum=10" description:"Number of retry attempts (default: 3)"`
	Force      bool `json:"force,omitempty" description:"Force push even if image already exists"`
}

// AtomicPushImageResult defines the response from atomic Docker image push
type AtomicPushImageResult struct {
	types.BaseToolResponse
	mcptypes.BaseAIContextResult      // Embed AI context methods
	Success                      bool `json:"success"`
	// Session context
	SessionID    string `json:"session_id"`
	WorkspaceDir string `json:"workspace_dir"`
	// Push configuration
	ImageRef    string `json:"image_ref"`
	RegistryURL string `json:"registry_url"`
	// Push results from core operations
	PushResult *coredocker.RegistryPushResult `json:"push_result"`
	// Timing information
	PushDuration  time.Duration `json:"push_duration"`
	TotalDuration time.Duration `json:"total_duration"`
	// Rich context for Claude reasoning
	PushContext *PushContext `json:"push_context"`
}

// PushContext provides rich context for Claude to reason about
type PushContext struct {
	// Push analysis
	PushStatus    string  `json:"push_status"`
	LayersPushed  int     `json:"layers_pushed"`
	LayersCached  int     `json:"layers_cached"`
	PushSizeMB    float64 `json:"push_size_mb"`
	CacheHitRatio float64 `json:"cache_hit_ratio"`
	// Registry information
	RegistryType     string `json:"registry_type"`
	RegistryEndpoint string `json:"registry_endpoint"`
	AuthMethod       string `json:"auth_method,omitempty"`
	// Error analysis
	ErrorType     string `json:"error_type,omitempty"`
	ErrorCategory string `json:"error_category,omitempty"`
	IsRetryable   bool   `json:"is_retryable"`
	// Next step suggestions
	NextStepSuggestions []string `json:"next_step_suggestions"`
	TroubleshootingTips []string `json:"troubleshooting_tips,omitempty"`
	AuthenticationGuide []string `json:"authentication_guide,omitempty"`
}

// AtomicPushImageTool implements atomic Docker image push using core operations
type AtomicPushImageTool struct {
	pipelineAdapter mcptypes.PipelineOperations
	sessionManager  core.ToolSessionManager
	logger          zerolog.Logger
	analyzer        ToolAnalyzer
	fixingMixin     *AtomicToolFixingMixin
}

// NewAtomicPushImageTool creates a new atomic push image tool
func NewAtomicPushImageTool(adapter mcptypes.PipelineOperations, sessionManager core.ToolSessionManager, logger zerolog.Logger) *AtomicPushImageTool {
	return &AtomicPushImageTool{
		pipelineAdapter: adapter,
		sessionManager:  sessionManager,
		logger:          logger.With().Str("tool", "atomic_push_image").Logger(),
	}
}

// SetAnalyzer sets the analyzer for failure analysis
func (t *AtomicPushImageTool) SetAnalyzer(analyzer ToolAnalyzer) {
	t.analyzer = analyzer
}

// SetFixingMixin sets the fixing mixin for automatic error recovery
func (t *AtomicPushImageTool) SetFixingMixin(mixin *AtomicToolFixingMixin) {
	t.fixingMixin = mixin
}

// ExecuteWithFixes runs the atomic Docker image push with automatic fixes
func (t *AtomicPushImageTool) ExecuteWithFixes(ctx context.Context, args AtomicPushImageArgs) (*AtomicPushImageResult, error) {
	if t.fixingMixin != nil && !args.DryRun {
		// Create wrapper operation for push process
		var result *AtomicPushImageResult
		progress := observability.NewUnifiedProgressReporter(nil) // No server context in ExecuteWithFixes
		operation := NewDockerOperation(DockerOperationConfig{
			Type:          OperationPush,
			Name:          args.ImageRef,
			RetryAttempts: 3,
			Timeout:       10 * time.Minute, // Push operations typically take longer
			ExecuteFunc: func(ctx context.Context) error {
				var err error
				// TODO: Fix method call - executeWithoutProgress method not found
				// result, err = t.executeWithoutProgress(ctx, args, nil, time.Now())
				result = &AtomicPushImageResult{Success: false}
				err = fmt.Errorf("push operation not implemented")
				if err != nil {
					return err
				}
				if !result.Success {
					return fmt.Errorf("operation failed")
				}
				return nil
			},
		}, progress)
		// TODO: Fix method call - ExecuteWithProgress method not found
		// if err := operation.ExecuteWithProgress(ctx, progress); err != nil {
		if err := operation.ExecuteOnce(ctx); err != nil {
			return nil, err
		}
		return result, nil
	}
	// TODO: Fix method call and variables - executeWithoutProgress method not found, result undefined
	// return t.executeWithoutProgress(ctx, args, result, time.Now())
	return &AtomicPushImageResult{Success: false}, fmt.Errorf("push operation not implemented")
}
