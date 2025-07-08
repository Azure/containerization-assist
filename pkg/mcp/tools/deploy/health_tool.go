package deploy

import (
	"context"
	"time"

	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/core"
	errors "github.com/Azure/container-kit/pkg/mcp/errors"
	internalcommon "github.com/Azure/container-kit/pkg/mcp/internal/common"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	validation "github.com/Azure/container-kit/pkg/mcp/security"
	"github.com/Azure/container-kit/pkg/mcp/services"
	"github.com/Azure/container-kit/pkg/mcp/session"
	"github.com/Azure/container-kit/pkg/mcp/tools/build"
	"github.com/localrivet/gomcp/server"
)

// AtomicCheckHealthTool handles application health checking with modular components
type AtomicCheckHealthTool struct {
	pipelineAdapter core.TypedPipelineOperations
	sessionManager  session.UnifiedSessionManager
	logger          *slog.Logger
	checker         *HealthChecker
	validator       *HealthValidator

	// Optional components for enhanced functionality
	analyzer    internalcommon.FailureAnalyzer
	fixingMixin *build.AtomicToolFixingMixin
}

// newAtomicCheckHealthToolImplUnified creates a new atomic check health tool using unified session manager (internal implementation)
func newAtomicCheckHealthToolImplUnified(adapter core.TypedPipelineOperations, sessionManager session.UnifiedSessionManager, logger *slog.Logger) *AtomicCheckHealthTool {
	toolLogger := logger.With("tool", "atomic_check_health")

	return &AtomicCheckHealthTool{
		pipelineAdapter: adapter,
		sessionManager:  sessionManager,
		logger:          toolLogger,
		checker:         NewHealthChecker(adapter, sessionManager, toolLogger),
		validator:       NewHealthValidator(toolLogger),
	}
}

// newAtomicCheckHealthToolImplServices creates a new atomic check health tool using focused services (internal implementation)
func newAtomicCheckHealthToolImplServices(adapter core.TypedPipelineOperations, sessionStore services.SessionStore, sessionState services.SessionState, logger *slog.Logger) *AtomicCheckHealthTool {
	toolLogger := logger.With("tool", "atomic_check_health")

	return &AtomicCheckHealthTool{
		pipelineAdapter: adapter,
		sessionManager:  nil, // Not needed when using services
		logger:          toolLogger,
		checker:         NewHealthCheckerWithServices(adapter, sessionStore, sessionState, toolLogger),
		validator:       NewHealthValidator(toolLogger),
	}
}

// SetAnalyzer sets the tool analyzer for enhanced analysis capabilities
func (t *AtomicCheckHealthTool) SetAnalyzer(analyzer internalcommon.FailureAnalyzer) {
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

	// Progress tracking infrastructure removed
	var progress interface{} = nil

	ctx := context.Background()
	result, err := t.executeWithProgress(ctx, args, startTime, progress)

	if err != nil {
		t.logger.Info("Health check failed")
		if result == nil {
			result = &AtomicCheckHealthResult{
				BaseToolResponse:    types.BaseToolResponse{Success: false, Timestamp: time.Now()},
				BaseAIContextResult: core.NewBaseAIContextResult("health", false, time.Since(startTime)),
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
	// Progress tracking infrastructure removed

	// Stage 1: Initialize - progress reporting removed

	// Perform the actual health check
	result, err := t.checker.PerformHealthCheck(ctx, args, reporter)
	if err != nil {
		return result, err
	}

	// Stage 4: Validate and analyze - progress reporting removed

	// Perform validation and analysis - this would need healthResult from checker
	// For now, we'll do basic analysis
	t.validator.AnalyzeApplicationHealth(result, args, nil)

	// Stage 5: Finalize - progress reporting removed

	// Generate context for AI reasoning
	t.generateHealthContext(result, args)

	// Update session state if available
	if session, err := t.sessionManager.GetSession(ctx, args.SessionID); err == nil {
		t.updateSessionState(session.ToCoreSessionState(), result)
	}

	t.logger.Info("Health check completed successfully",
		"health_status", result.HealthStatus,
		"overall_score", result.OverallScore,
		"success", result.Success)

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
	if session, err := t.sessionManager.GetSession(ctx, args.SessionID); err == nil {
		t.updateSessionState(session.ToCoreSessionState(), result)
	}

	t.logger.Info("Health check completed successfully",
		"health_status", result.HealthStatus,
		"overall_score", result.OverallScore,
		"success", result.Success)

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

	t.logger.Debug("Updated session state with health check results",
		"session_id", session.SessionID,
		"health_status", result.HealthStatus)

	return nil
}

// Tool interface methods

// GetName returns the tool name

// GetMetadata returns comprehensive tool metadata
func (t *AtomicCheckHealthTool) GetMetadata() api.ToolMetadata {
	return api.ToolMetadata{
		Name:        "atomic_check_health",
		Description: "Comprehensive Kubernetes application health checking with detailed analysis and recommendations",
		Version:     "1.0.0",
		Category:    api.ToolCategory("operations"),
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
		Tags:         []string{"health", "kubernetes", "atomic"},
		Status:       api.ToolStatus("active"),
		RegisteredAt: time.Now(),
		LastModified: time.Now(),
	}
}

// Validate validates the tool arguments
func (t *AtomicCheckHealthTool) Validate(ctx context.Context, args interface{}) error {
	// Validate using tag-based validation
	return validation.ValidateTaggedStruct(args)
}

func (t *AtomicCheckHealthTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	typedArgs, ok := args.(AtomicCheckHealthArgs)
	if !ok {
		return nil, errors.NewError().Messagef("invalid argument type: expected AtomicCheckHealthArgs").Build()
	}

	return t.ExecuteHealthCheck(ctx, typedArgs)
}
