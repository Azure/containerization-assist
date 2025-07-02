package deploy

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/core"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/internal/build"
	"github.com/Azure/container-kit/pkg/mcp/internal/common"
	"github.com/Azure/container-kit/pkg/mcp/internal/observability"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/localrivet/gomcp/server"
	"github.com/rs/zerolog"
)

// AtomicCheckHealthTool handles application health checking with modular components
type AtomicCheckHealthTool struct {
	pipelineAdapter core.PipelineOperations
	sessionManager  core.ToolSessionManager
	logger          zerolog.Logger
	checker         *HealthChecker
	validator       *HealthValidator

	// Optional components for enhanced functionality
	analyzer    common.FailureAnalyzer
	fixingMixin *build.AtomicToolFixingMixin
}

// newAtomicCheckHealthToolImpl creates a new atomic check health tool (internal implementation)
func newAtomicCheckHealthToolImpl(adapter core.PipelineOperations, sessionManager core.ToolSessionManager, logger zerolog.Logger) *AtomicCheckHealthTool {
	toolLogger := logger.With().Str("tool", "atomic_check_health").Logger()

	return &AtomicCheckHealthTool{
		pipelineAdapter: adapter,
		sessionManager:  sessionManager,
		logger:          toolLogger,
		checker:         NewHealthChecker(adapter, sessionManager, toolLogger),
		validator:       NewHealthValidator(toolLogger),
	}
}

// SetAnalyzer sets the tool analyzer for enhanced analysis capabilities
func (t *AtomicCheckHealthTool) SetAnalyzer(analyzer common.FailureAnalyzer) {
	t.analyzer = analyzer
}

// SetFixingMixin sets the fixing mixin for automatic issue resolution
func (t *AtomicCheckHealthTool) SetFixingMixin(mixin *build.AtomicToolFixingMixin) {
	t.fixingMixin = mixin
}

// ExecuteHealthCheck executes health checking without progress reporting
func (t *AtomicCheckHealthTool) ExecuteHealthCheck(ctx context.Context, args AtomicCheckHealthArgs) (*AtomicCheckHealthResult, error) {
	return t.executeWithoutProgress(ctx, args)
}

// ExecuteWithContext executes health checking with progress reporting
func (t *AtomicCheckHealthTool) ExecuteWithContext(serverCtx *server.Context, args AtomicCheckHealthArgs) (*AtomicCheckHealthResult, error) {
	startTime := time.Now()

	progress := observability.NewUnifiedProgressReporter(serverCtx)

	ctx := context.Background()
	result, err := t.executeWithProgress(ctx, args, startTime, progress)

	if err != nil {
		t.logger.Info().Msg("Health check failed")
		if result == nil {
			result = &AtomicCheckHealthResult{
				BaseToolResponse:    types.NewBaseResponse("atomic_check_health", args.SessionID, args.DryRun),
				BaseAIContextResult: mcptypes.NewBaseAIContextResult("health", false, time.Since(startTime)),
				Success:             false,
				HealthStatus:        "unknown",
				Summary:             "Health check failed - please check logs and retry",
				Context:             &HealthContext{},
			}
		}
	}

	return result, err
}

// executeWithProgress executes the health check with progress reporting
func (t *AtomicCheckHealthTool) executeWithProgress(ctx context.Context, args AtomicCheckHealthArgs, startTime time.Time, reporter interface{}) (*AtomicCheckHealthResult, error) {
	if progressReporter, ok := reporter.(interface {
		StartProgress([]core.ProgressStage)
		ReportStage(float64, string)
		CompleteProgress(string)
	}); ok {
		stages := []core.ProgressStage{
			{Name: "Initialize", Weight: 0.1, Description: "Initializing health check"},
			{Name: "Query", Weight: 0.3, Description: "Querying Kubernetes resources"},
			{Name: "Analyze", Weight: 0.4, Description: "Analyzing application health"},
			{Name: "Validate", Weight: 0.15, Description: "Validating health status"},
			{Name: "Finalize", Weight: 0.05, Description: "Generating recommendations"},
		}
		progressReporter.StartProgress(stages)
		defer progressReporter.CompleteProgress("Health check completed")
	}

	// Stage 1: Initialize
	if progressReporter, ok := reporter.(interface{ ReportStage(float64, string) }); ok {
		progressReporter.ReportStage(0.1, "Initializing health check")
	}

	// Perform the actual health check
	result, err := t.checker.PerformHealthCheck(ctx, args, reporter)
	if err != nil {
		return result, err
	}

	// Stage 4: Validate and analyze
	if progressReporter, ok := reporter.(interface{ ReportStage(float64, string) }); ok {
		progressReporter.ReportStage(0.85, "Analyzing health results")
	}

	// Perform validation and analysis - this would need healthResult from checker
	// For now, we'll do basic analysis
	t.validator.AnalyzeApplicationHealth(result, args, nil)

	// Stage 5: Finalize
	if progressReporter, ok := reporter.(interface{ ReportStage(float64, string) }); ok {
		progressReporter.ReportStage(1.0, "Finalizing health assessment")
	}

	// Generate context for AI reasoning
	t.generateHealthContext(result, args)

	// Update session state if available
	if sessionInterface, err := t.sessionManager.GetSession(args.SessionID); err == nil {
		t.updateSessionState(sessionInterface.(*core.SessionState), result)
	}

	t.logger.Info().
		Str("health_status", result.HealthStatus).
		Int("overall_score", result.OverallScore).
		Bool("success", result.Success).
		Msg("Health check completed successfully")

	return result, nil
}

// executeWithoutProgress executes the health check without progress reporting
func (t *AtomicCheckHealthTool) executeWithoutProgress(ctx context.Context, args AtomicCheckHealthArgs) (*AtomicCheckHealthResult, error) {
	// Perform the actual health check
	result, err := t.checker.PerformHealthCheck(ctx, args, nil)
	if err != nil {
		return result, err
	}

	// Perform validation and analysis
	t.validator.AnalyzeApplicationHealth(result, args, nil)

	// Generate context for AI reasoning
	t.generateHealthContext(result, args)

	// Update session state if available
	if sessionInterface, err := t.sessionManager.GetSession(args.SessionID); err == nil {
		t.updateSessionState(sessionInterface.(*core.SessionState), result)
	}

	t.logger.Info().
		Str("health_status", result.HealthStatus).
		Int("overall_score", result.OverallScore).
		Bool("success", result.Success).
		Msg("Health check completed successfully")

	return result, nil
}

// generateHealthContext generates context for AI reasoning
func (t *AtomicCheckHealthTool) generateHealthContext(result *AtomicCheckHealthResult, args AtomicCheckHealthArgs) {
	if result.Context == nil {
		result.Context = &HealthContext{}
	}

	ctx := result.Context

	// Set basic context
	ctx.AppName = args.AppName
	ctx.Namespace = result.Namespace

	// Calculate stability score based on issues
	stabilityScore := 100
	if len(result.PodIssues) > 0 {
		stabilityScore -= len(result.PodIssues) * 10
	}
	if len(result.ContainerIssues) > 0 {
		stabilityScore -= len(result.ContainerIssues) * 5
	}
	if stabilityScore < 0 {
		stabilityScore = 0
	}
	ctx.StabilityScore = stabilityScore

	// Set deployment type based on analysis
	if len(result.PodSummaries) > 1 {
		ctx.DeploymentType = "Deployment"
	} else if len(result.PodSummaries) == 1 {
		ctx.DeploymentType = "Single Pod"
	} else {
		ctx.DeploymentType = "Unknown"
	}

	// Add operational context
	result.CheckedAt = time.Now().UTC().Format(time.RFC3339)

	// Set health indicators
	if result.RestartAnalysis != nil {
		ctx.HistoricalRestarts = result.RestartAnalysis.TotalRestarts
		ctx.PersistentIssues = result.RestartAnalysis.RestartPattern == "frequent" || result.RestartAnalysis.RestartPattern == "continuous"
	}
}

// updateSessionState updates the session state with health check results
func (t *AtomicCheckHealthTool) updateSessionState(session *core.SessionState, result *AtomicCheckHealthResult) error {
	// Update session metadata with health check results
	if session.Metadata == nil {
		session.Metadata = make(map[string]interface{})
	}
	session.Metadata["last_health_check"] = map[string]interface{}{
		"timestamp": time.Now(),
		"status":    result.HealthStatus,
		"score":     result.OverallScore,
		"issues":    len(result.PodIssues) + len(result.ContainerIssues),
		"namespace": result.Namespace,
		"summary":   result.Summary,
	}

	t.logger.Debug().
		Str("session_id", session.SessionID).
		Str("health_status", result.HealthStatus).
		Msg("Updated session state with health check results")

	return nil
}

// Tool interface methods

// GetName returns the tool name
func (t *AtomicCheckHealthTool) GetName() string {
	return "atomic_check_health"
}

// GetDescription returns the tool description
func (t *AtomicCheckHealthTool) GetDescription() string {
	return "Comprehensive Kubernetes application health checking with detailed analysis and recommendations"
}

// GetVersion returns the tool version
func (t *AtomicCheckHealthTool) GetVersion() string {
	return "1.0.0"
}

// GetCapabilities returns the tool capabilities
func (t *AtomicCheckHealthTool) GetCapabilities() types.ToolCapabilities {
	return types.ToolCapabilities{
		SupportsDryRun:    true,
		SupportsStreaming: true,
		IsLongRunning:     true,
		RequiresAuth:      false,
	}
}

// GetMetadata returns comprehensive tool metadata
func (t *AtomicCheckHealthTool) GetMetadata() core.ToolMetadata {
	return core.ToolMetadata{
		Name:        "atomic_check_health",
		Description: "Comprehensive Kubernetes application health checking with detailed analysis and recommendations",
		Version:     "1.0.0",
		Category:    "operations",
		Dependencies: []string{
			"session_manager",
			"kubernetes_client",
			"pipeline_operations",
		},
		Capabilities: []string{
			"health_checking",
			"pod_analysis",
			"service_validation",
			"restart_pattern_analysis",
			"troubleshooting_recommendations",
			"readiness_waiting",
			"detailed_diagnostics",
		},
		Requirements: []string{
			"valid_session_id",
			"kubernetes_access",
			"namespace_permissions",
		},
		Parameters: map[string]string{
			"session_id":        "string - Session ID for context",
			"namespace":         "string - Kubernetes namespace (default: default)",
			"app_name":          "string - Application name for label selection",
			"label_selector":    "string - Custom label selector",
			"include_services":  "boolean - Include service health checks",
			"include_events":    "boolean - Include pod events in analysis",
			"wait_for_ready":    "boolean - Wait for pods to become ready",
			"wait_timeout":      "integer - Wait timeout in seconds",
			"detailed_analysis": "boolean - Perform detailed analysis",
			"include_logs":      "boolean - Include container logs",
			"log_lines":         "integer - Number of log lines to include",
		},
		Examples: []core.ToolExample{
			{
				Name:        "basic_health_check",
				Description: "Basic health check for an application",
				Input: map[string]interface{}{
					"session_id": "session-123",
					"app_name":   "my-app",
				},
				Output: map[string]interface{}{
					"health_status": "healthy",
					"overall_score": 100,
				},
			},
			{
				Name:        "detailed_health_analysis",
				Description: "Comprehensive health analysis with waiting",
				Input: map[string]interface{}{
					"session_id":        "session-123",
					"namespace":         "production",
					"app_name":          "my-app",
					"wait_for_ready":    true,
					"detailed_analysis": true,
					"include_logs":      true,
				},
				Output: map[string]interface{}{
					"health_status":   "degraded",
					"overall_score":   75,
					"pod_issues":      2,
					"recommendations": []string{"Check pod logs", "Verify resources"},
				},
			},
		},
	}
}

// Validate validates the tool arguments
func (t *AtomicCheckHealthTool) Validate(ctx context.Context, args interface{}) error {
	typedArgs, ok := args.(AtomicCheckHealthArgs)
	if !ok {
		return fmt.Errorf("invalid argument type: expected AtomicCheckHealthArgs")
	}

	if typedArgs.SessionID == "" {
		return fmt.Errorf("session_id is required")
	}

	if typedArgs.WaitTimeout < 0 {
		return fmt.Errorf("wait_timeout must be non-negative")
	}

	if typedArgs.LogLines < 0 {
		return fmt.Errorf("log_lines must be non-negative")
	}

	return nil
}

// Execute executes the tool with generic arguments
func (t *AtomicCheckHealthTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	typedArgs, ok := args.(AtomicCheckHealthArgs)
	if !ok {
		return nil, fmt.Errorf("invalid argument type: expected AtomicCheckHealthArgs")
	}

	return t.ExecuteTyped(ctx, typedArgs)
}

// ExecuteTyped executes the tool with typed arguments
func (t *AtomicCheckHealthTool) ExecuteTyped(ctx context.Context, args AtomicCheckHealthArgs) (*AtomicCheckHealthResult, error) {
	return t.ExecuteHealthCheck(ctx, args)
}
