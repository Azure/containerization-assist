package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-copilot/pkg/core/kubernetes"
	"github.com/Azure/container-copilot/pkg/mcp/internal/api/contract"
	"github.com/Azure/container-copilot/pkg/mcp/internal/interfaces"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	sessiontypes "github.com/Azure/container-copilot/pkg/mcp/internal/types/session"
	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
	"github.com/localrivet/gomcp/server"
	"github.com/rs/zerolog"
)

// AtomicCheckHealthArgs defines arguments for atomic application health checking
type AtomicCheckHealthArgs struct {
	types.BaseToolArgs

	// Target specification
	Namespace     string `json:"namespace,omitempty" description:"Kubernetes namespace (default: default)"`
	AppName       string `json:"app_name,omitempty" description:"Application name for label selection"`
	LabelSelector string `json:"label_selector,omitempty" description:"Custom label selector (e.g., app=myapp,version=v1)"`

	// Health check configuration
	IncludeServices bool `json:"include_services,omitempty" description:"Include service health checks (default: true)"`
	IncludeEvents   bool `json:"include_events,omitempty" description:"Include pod events in analysis (default: true)"`
	WaitForReady    bool `json:"wait_for_ready,omitempty" description:"Wait for pods to become ready"`
	WaitTimeout     int  `json:"wait_timeout,omitempty" description:"Wait timeout in seconds (default: 300)"`

	// Analysis depth
	DetailedAnalysis bool `json:"detailed_analysis,omitempty" description:"Perform detailed container and condition analysis"`
	IncludeLogs      bool `json:"include_logs,omitempty" description:"Include recent container logs in analysis"`
	LogLines         int  `json:"log_lines,omitempty" description:"Number of log lines to include (default: 50)"`
}

// AtomicCheckHealthResult defines the response from atomic health checking
type AtomicCheckHealthResult struct {
	types.BaseToolResponse
	BaseAIContextResult      // Embed AI context methods
	Success             bool `json:"success"`

	// Session context
	SessionID     string `json:"session_id"`
	Namespace     string `json:"namespace"`
	LabelSelector string `json:"label_selector"`

	// Health check results from core operations
	HealthResult *kubernetes.HealthCheckResult `json:"health_result"`

	// Wait results (if waiting was requested)
	WaitResult *kubernetes.HealthCheckResult `json:"wait_result,omitempty"`

	// Timing information
	HealthCheckDuration time.Duration `json:"health_check_duration"`
	WaitDuration        time.Duration `json:"wait_duration,omitempty"`
	TotalDuration       time.Duration `json:"total_duration"`

	// Rich context for Claude reasoning
	HealthContext *HealthContext `json:"health_context"`

	// Rich error information if operation failed
}

// HealthContext provides rich context for Claude to reason about application health
type HealthContext struct {
	// Overall health summary
	OverallStatus  string  `json:"overall_status"`  // Health status: healthy, degraded, unhealthy, unknown
	HealthScore    float64 `json:"health_score"`    // 0.0 to 1.0
	ReadinessRatio float64 `json:"readiness_ratio"` // Ready pods / Total pods

	// Pod analysis
	PodSummary      PodSummary       `json:"pod_summary"`
	PodIssues       []PodIssue       `json:"pod_issues"`
	ContainerIssues []ContainerIssue `json:"container_issues"`

	// Service analysis
	ServiceSummary ServiceSummary `json:"service_summary"`
	ServiceIssues  []string       `json:"service_issues"`

	// Performance insights
	ResourceUsage     ResourceUsageInfo `json:"resource_usage"`
	PerformanceIssues []string          `json:"performance_issues"`

	// Stability analysis
	RestartAnalysis RestartAnalysis `json:"restart_analysis"`
	StabilityIssues []string        `json:"stability_issues"`

	// Recommendations
	HealthRecommendations []string `json:"health_recommendations"`
	NextStepSuggestions   []string `json:"next_step_suggestions"`
	TroubleshootingTips   []string `json:"troubleshooting_tips,omitempty"`
}

// PodSummary provides summary of pod health
type PodSummary struct {
	TotalPods   int `json:"total_pods"`
	ReadyPods   int `json:"ready_pods"`
	PendingPods int `json:"pending_pods"`
	FailedPods  int `json:"failed_pods"`
	RunningPods int `json:"running_pods"`
}

// PodIssue represents a pod-level health issue
type PodIssue struct {
	PodName     string   `json:"pod_name"`
	IssueType   string   `json:"issue_type"` // Issue type: not_ready, failed, pending, crashloop
	Description string   `json:"description"`
	Severity    string   `json:"severity"` // "low", "medium", "high", "critical"
	Since       string   `json:"since"`
	Suggestions []string `json:"suggestions"`
}

// ContainerIssue represents a container-level health issue
type ContainerIssue struct {
	PodName       string `json:"pod_name"`
	ContainerName string `json:"container_name"`
	IssueType     string `json:"issue_type"` // Issue type: not_ready, restart_loop, oom_killed, failed
	Description   string `json:"description"`
	Severity      string `json:"severity"`
	RestartCount  int    `json:"restart_count"`
	LastRestart   string `json:"last_restart,omitempty"`
}

// ServiceSummary provides summary of service health
type ServiceSummary struct {
	TotalServices   int `json:"total_services"`
	HealthyServices int `json:"healthy_services"`
	EndpointsReady  int `json:"endpoints_ready"`
	EndpointsTotal  int `json:"endpoints_total"`
}

// ResourceUsageInfo provides resource usage insights
type ResourceUsageInfo struct {
	HighCPUPods      []string `json:"high_cpu_pods"`
	HighMemoryPods   []string `json:"high_memory_pods"`
	ResourceWarnings []string `json:"resource_warnings"`
}

// RestartAnalysis provides pod restart analysis
type RestartAnalysis struct {
	TotalRestarts    int      `json:"total_restarts"`
	PodsWithRestarts int      `json:"pods_with_restarts"`
	HighRestartPods  []string `json:"high_restart_pods"` // Pods with >5 restarts
	RecentRestarts   int      `json:"recent_restarts"`   // Restarts in last hour
}

// AtomicCheckHealthTool implements atomic application health checking using core operations
type AtomicCheckHealthTool struct {
	pipelineAdapter mcptypes.PipelineOperations
	sessionManager  mcptypes.ToolSessionManager
	// errorHandler field removed - using direct error handling
	logger zerolog.Logger
}

// NewAtomicCheckHealthTool creates a new atomic check health tool
func NewAtomicCheckHealthTool(adapter mcptypes.PipelineOperations, sessionManager mcptypes.ToolSessionManager, logger zerolog.Logger) *AtomicCheckHealthTool {
	return &AtomicCheckHealthTool{
		pipelineAdapter: adapter,
		sessionManager:  sessionManager,
		// errorHandler initialization removed - using direct error handling
		logger: logger.With().Str("tool", "atomic_check_health").Logger(),
	}
}

// standardHealthCheckStages provides common stages for health check operations
func standardHealthCheckStages() []interfaces.ProgressStage {
	return []interfaces.ProgressStage{
		{Name: "Initialize", Weight: 0.10, Description: "Loading session and namespace"},
		{Name: "Query", Weight: 0.30, Description: "Querying Kubernetes resources"},
		{Name: "Analyze", Weight: 0.30, Description: "Analyzing pod and service health"},
		{Name: "Wait", Weight: 0.20, Description: "Waiting for ready state (if requested)"},
		{Name: "Report", Weight: 0.10, Description: "Generating health report"},
	}
}

// ExecuteHealthCheck runs the atomic application health check
func (t *AtomicCheckHealthTool) ExecuteHealthCheck(ctx context.Context, args AtomicCheckHealthArgs) (*AtomicCheckHealthResult, error) {
	// Direct execution without progress tracking
	return t.executeWithoutProgress(ctx, args)
}

// ExecuteWithContext runs the atomic health check with GoMCP progress tracking
func (t *AtomicCheckHealthTool) ExecuteWithContext(serverCtx *server.Context, args AtomicCheckHealthArgs) (*AtomicCheckHealthResult, error) {
	// Create progress adapter for GoMCP using centralized health stages
	adapter := NewGoMCPProgressAdapter(serverCtx, interfaces.StandardHealthStages())

	// Execute with progress tracking
	ctx := context.Background()
	result, err := t.performHealthCheck(ctx, args, adapter)

	// Complete progress tracking
	if err != nil {
		adapter.Complete("Health check failed")
	} else {
		adapter.Complete("Health check completed successfully")
	}

	return result, err
}

// executeWithoutProgress executes without progress tracking
func (t *AtomicCheckHealthTool) executeWithoutProgress(ctx context.Context, args AtomicCheckHealthArgs) (*AtomicCheckHealthResult, error) {
	return t.performHealthCheck(ctx, args, nil)
}

// performHealthCheck performs the actual health check
func (t *AtomicCheckHealthTool) performHealthCheck(ctx context.Context, args AtomicCheckHealthArgs, reporter interfaces.ProgressReporter) (*AtomicCheckHealthResult, error) {
	startTime := time.Now()

	// Get session
	sessionInterface, err := t.sessionManager.GetSession(args.SessionID)
	if err != nil {
		// Create result with error for session failure
		result := &AtomicCheckHealthResult{
			BaseToolResponse:    types.NewBaseResponse("atomic_check_health", args.SessionID, args.DryRun),
			BaseAIContextResult: NewBaseAIContextResult("health", false, time.Since(startTime)),
			SessionID:           args.SessionID,
			Namespace:           t.getNamespace(args.Namespace),
			TotalDuration:       time.Since(startTime),
			HealthContext:       &HealthContext{},
		}
		result.Success = false

		t.logger.Error().Err(err).
			Str("session_id", args.SessionID).
			Msg("Failed to get session")
		// Session retrieval error is returned directly
		return result, nil
	}
	session := sessionInterface.(*sessiontypes.SessionState)

	// Build label selector
	labelSelector := t.buildLabelSelector(args, session)
	namespace := t.getNamespace(args.Namespace)

	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("namespace", namespace).
		Str("label_selector", labelSelector).
		Bool("wait_for_ready", args.WaitForReady).
		Msg("Starting atomic application health check")

	// Stage 1: Initialize
	if reporter != nil {
		reporter.ReportStage(0.0, "Initializing health check")
	}

	// Create base response
	result := &AtomicCheckHealthResult{
		BaseToolResponse:    types.NewBaseResponse("atomic_check_health", session.SessionID, args.DryRun),
		BaseAIContextResult: NewBaseAIContextResult("health", true, 0), // Duration will be set later
		SessionID:           session.SessionID,
		Namespace:           namespace,
		LabelSelector:       labelSelector,
		HealthContext:       &HealthContext{},
	}

	if reporter != nil {
		reporter.ReportStage(0.5, "Session and namespace loaded")
	}

	// Handle dry-run
	if args.DryRun {
		result.HealthContext.NextStepSuggestions = []string{
			"This is a dry-run - actual health check would be performed",
			fmt.Sprintf("Would check health in namespace: %s", namespace),
			fmt.Sprintf("Using label selector: %s", labelSelector),
		}
		result.TotalDuration = time.Since(startTime)
		return result, nil
	}

	// Validate prerequisites
	if reporter != nil {
		reporter.ReportStage(0.8, "Validating prerequisites")
	}

	if err := t.validateHealthCheckPrerequisites(result, args); err != nil {
		t.logger.Error().Err(err).
			Str("session_id", session.SessionID).
			Str("namespace", namespace).
			Str("label_selector", labelSelector).
			Msg("Health check prerequisites validation failed")
		// Prerequisites validation error is returned directly
		result.Success = false
		result.TotalDuration = time.Since(startTime)
		return result, nil
	}

	// Stage 2: Query Kubernetes resources
	if reporter != nil {
		reporter.NextStage("Querying Kubernetes resources")
		reporter.ReportStage(0.1, "Fetching pod and service status")
	}

	// Perform health check using core operations
	healthStartTime := time.Now()
	healthResult, err := t.pipelineAdapter.CheckApplicationHealth(
		session.SessionID,
		namespace,
		labelSelector,
		30*time.Second, // Default timeout for health checks
	)
	result.HealthCheckDuration = time.Since(healthStartTime)
	
	// Convert from mcptypes.HealthCheckResult to kubernetes.HealthCheckResult
	if healthResult != nil {
		result.HealthResult = &kubernetes.HealthCheckResult{
			Success:   healthResult.Healthy,
			Namespace: namespace,
			Duration:  result.HealthCheckDuration,
		}
		if healthResult.Error != nil {
			result.HealthResult.Error = &kubernetes.HealthCheckError{
				Type:    healthResult.Error.Type,
				Message: healthResult.Error.Message,
			}
		}
		// Convert pod statuses
		for _, ps := range healthResult.PodStatuses {
			podStatus := kubernetes.DetailedPodStatus{
				Name:      ps.Name,
				Namespace: namespace,
				Status:    ps.Status,
				Ready:     ps.Ready,
			}
			result.HealthResult.Pods = append(result.HealthResult.Pods, podStatus)
		}
		// Update summary
		result.HealthResult.Summary = kubernetes.HealthSummary{
			TotalPods: len(result.HealthResult.Pods),
			ReadyPods: 0,
			FailedPods: 0,
			PendingPods: 0,
		}
		for _, pod := range result.HealthResult.Pods {
			if pod.Ready {
				result.HealthResult.Summary.ReadyPods++
			} else if pod.Status == "Failed" || pod.Phase == "Failed" {
				result.HealthResult.Summary.FailedPods++
			} else if pod.Status == "Pending" || pod.Phase == "Pending" {
				result.HealthResult.Summary.PendingPods++
			}
		}
		if result.HealthResult.Summary.TotalPods > 0 {
			result.HealthResult.Summary.HealthyRatio = float64(result.HealthResult.Summary.ReadyPods) / float64(result.HealthResult.Summary.TotalPods)
		}
	}

	if reporter != nil {
		reporter.ReportStage(0.9, "Resource query complete")
	}

	if err != nil {
		t.logger.Error().Err(err).
			Str("session_id", session.SessionID).
			Str("namespace", namespace).
			Str("label_selector", labelSelector).
			Msg("Health check failed")
		t.addTroubleshootingTips(result, "health_check", err)
		// Health check error is returned directly
		result.Success = false
		result.TotalDuration = time.Since(startTime)
		return result, nil
	}

	t.logger.Info().
		Str("session_id", session.SessionID).
		Bool("healthy", result.HealthResult != nil && result.HealthResult.Success).
		Int("pods_ready", result.HealthResult.Summary.ReadyPods).
		Int("pods_total", result.HealthResult.Summary.TotalPods).
		Dur("health_check_duration", result.HealthCheckDuration).
		Msg("Application health check completed")

	// Stage 3: Analyze pod and service health
	if reporter != nil {
		reporter.NextStage("Analyzing pod and service health")
		reporter.ReportStage(0.1, "Processing health data")
	}

	// Analyze the health results to populate the context
	result.Success = result.HealthResult != nil && result.HealthResult.Success

	if reporter != nil {
		reporter.ReportStage(0.5, "Health data analyzed")
	}

	// Stage 4: Wait for readiness if requested
	if args.WaitForReady && (result.HealthResult == nil || !result.HealthResult.Success) {
		if reporter != nil {
			reporter.NextStage("Waiting for ready state")
			reporter.ReportStage(0.1, "Starting readiness wait")
		}

		waitStartTime := time.Now()
		timeout := t.getWaitTimeout(args.WaitTimeout)

		t.logger.Info().
			Str("session_id", session.SessionID).
			Dur("timeout", timeout).
			Msg("Waiting for application to become ready")

		// Simple polling loop for readiness (core operations don't have wait functionality)
		waitResult := t.waitForApplicationReady(ctx, session.SessionID, namespace, labelSelector, timeout)
		result.WaitDuration = time.Since(waitStartTime)
		result.WaitResult = waitResult

		if waitResult != nil && waitResult.Success {
			t.logger.Info().
				Str("session_id", session.SessionID).
				Dur("wait_duration", result.WaitDuration).
				Msg("Application became ready")
		} else {
			t.logger.Warn().
				Str("session_id", session.SessionID).
				Dur("wait_duration", result.WaitDuration).
				Msg("Application did not become ready within timeout")
		}

		if reporter != nil {
			reporter.ReportStage(0.9, "Readiness wait complete")
		}
	}

	// Stage 5: Generate health report
	if reporter != nil {
		reporter.NextStage("Generating health report")
		reporter.ReportStage(0.2, "Analyzing results in detail")
	}

	// Analyze health results in detail
	t.analyzeApplicationHealth(result, args)

	if reporter != nil {
		reporter.ReportStage(0.8, "Health report generated")
	}

	result.TotalDuration = time.Since(startTime)
	// Update BaseAIContextResult fields
	result.BaseAIContextResult.Duration = result.TotalDuration
	result.BaseAIContextResult.IsSuccessful = result.Success
	if result.HealthContext != nil {
		result.BaseAIContextResult.ErrorCount = len(result.HealthContext.PodIssues) + len(result.HealthContext.ContainerIssues)
		result.BaseAIContextResult.WarningCount = len(result.HealthContext.ServiceIssues) + len(result.HealthContext.PerformanceIssues)
	}

	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("overall_status", result.HealthContext.OverallStatus).
		Float64("health_score", result.HealthContext.HealthScore).
		Dur("total_duration", result.TotalDuration).
		Msg("Atomic application health check completed successfully")

	if reporter != nil {
		reporter.ReportStage(1.0, "Health check complete")
	}

	return result, nil
}

// validateHealthCheckPrerequisites validates health check prerequisites
func (t *AtomicCheckHealthTool) validateHealthCheckPrerequisites(result *AtomicCheckHealthResult, args AtomicCheckHealthArgs) error {
	if args.AppName == "" && args.LabelSelector == "" {
		return types.NewValidationErrorBuilder("Application identifier is required for health checking", "app_identifier", "").
			WithField("app_name", args.AppName).
			WithField("label_selector", args.LabelSelector).
			WithOperation("check_health").
			WithStage("input_validation").
			WithRootCause("No application identifier provided - cannot determine which resources to check").
			WithImmediateStep(1, "Provide app name", "Specify the application name using the app_name parameter").
			WithImmediateStep(2, "Provide label selector", "Specify a Kubernetes label selector using the label_selector parameter").
			Build()
	}

	return nil
}

// waitForApplicationReady implements a simple polling wait for application readiness
func (t *AtomicCheckHealthTool) waitForApplicationReady(ctx context.Context, sessionID, namespace, labelSelector string, timeout time.Duration) *kubernetes.HealthCheckResult {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(10 * time.Second) // Poll every 10 seconds
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCtx.Done():
			// Timeout reached, return final status
			result, err := t.pipelineAdapter.CheckApplicationHealth(sessionID, namespace, labelSelector, 30*time.Second)
			if err != nil || result == nil {
				return nil
			}
			// Convert from mcptypes.HealthCheckResult to kubernetes.HealthCheckResult
			return t.convertHealthCheckResult(result, namespace)

		case <-ticker.C:
			result, err := t.pipelineAdapter.CheckApplicationHealth(sessionID, namespace, labelSelector, 30*time.Second)
			if err != nil || result == nil {
				continue // Continue polling on error
			}

			if result.Healthy {
				// Convert from mcptypes.HealthCheckResult to kubernetes.HealthCheckResult
				return t.convertHealthCheckResult(result, namespace) // Application is ready
			}
		}
	}
}

// analyzeApplicationHealth performs detailed analysis of health results
func (t *AtomicCheckHealthTool) analyzeApplicationHealth(result *AtomicCheckHealthResult, args AtomicCheckHealthArgs) {
	ctx := result.HealthContext
	healthResult := result.HealthResult

	// Calculate overall health metrics
	ctx.ReadinessRatio = 0.0
	if healthResult.Summary.TotalPods > 0 {
		ctx.ReadinessRatio = float64(healthResult.Summary.ReadyPods) / float64(healthResult.Summary.TotalPods)
	}

	ctx.HealthScore = ctx.ReadinessRatio // Simple health score based on readiness

	// Determine overall status
	if ctx.ReadinessRatio >= 1.0 {
		ctx.OverallStatus = types.HealthStatusHealthy
	} else if ctx.ReadinessRatio >= 0.7 {
		ctx.OverallStatus = types.HealthStatusDegraded
	} else if ctx.ReadinessRatio > 0.0 {
		ctx.OverallStatus = types.HealthStatusUnhealthy
	} else {
		ctx.OverallStatus = "unknown"
	}

	// Analyze pod summary
	ctx.PodSummary = PodSummary{
		TotalPods:   healthResult.Summary.TotalPods,
		ReadyPods:   healthResult.Summary.ReadyPods,
		FailedPods:  healthResult.Summary.FailedPods,
		PendingPods: healthResult.Summary.PendingPods,
		RunningPods: healthResult.Summary.ReadyPods, // Simplification
	}

	// Analyze service summary
	ctx.ServiceSummary = ServiceSummary{
		TotalServices:   len(healthResult.Services),
		HealthyServices: len(healthResult.Services), // Assume all discovered services are healthy
		EndpointsReady:  healthResult.Summary.ReadyPods,
		EndpointsTotal:  healthResult.Summary.TotalPods,
	}

	// Analyze individual pods for issues
	t.analyzePodIssues(ctx, healthResult)

	// Analyze restart patterns
	t.analyzeRestartPatterns(ctx, healthResult)
}

// analyzePodIssues analyzes individual pod health issues
func (t *AtomicCheckHealthTool) analyzePodIssues(ctx *HealthContext, healthResult *kubernetes.HealthCheckResult) {
	var totalRestarts int
	var podsWithRestarts int

	for _, pod := range healthResult.Pods {
		// Analyze pod-level issues
		if !pod.Ready {
			issue := PodIssue{
				PodName:     pod.Name,
				Description: fmt.Sprintf("Pod is not ready: %s", pod.Status),
				Severity:    t.determinePodIssueSeverity(pod.Status),
				Since:       pod.Age,
			}

			switch strings.ToLower(pod.Status) {
			case "pending":
				issue.IssueType = types.HealthStatusPending
				issue.Suggestions = []string{
					"Check if the node has sufficient resources",
					"Verify image can be pulled from registry",
					"Check for scheduling constraints",
				}
			case "failed", "error":
				issue.IssueType = types.HealthStatusFailed
				issue.Suggestions = []string{
					"Check container logs for error details",
					"Verify container image and entry point",
					"Check resource limits and requests",
				}
			case "crashloopbackoff":
				issue.IssueType = "crashloop"
				issue.Suggestions = []string{
					"Container is repeatedly crashing - check logs",
					"Verify application startup configuration",
					"Check for missing dependencies or config",
				}
			default:
				issue.IssueType = "not_ready"
				issue.Suggestions = []string{
					"Check pod conditions and events",
					"Verify readiness probe configuration",
				}
			}

			ctx.PodIssues = append(ctx.PodIssues, issue)
		}

		// Analyze container-level issues
		for _, container := range pod.Containers {
			if container.RestartCount > 0 {
				totalRestarts += container.RestartCount
				if container.RestartCount > 0 {
					podsWithRestarts++
				}
			}

			if !container.Ready || container.RestartCount > 3 {
				containerIssue := ContainerIssue{
					PodName:       pod.Name,
					ContainerName: container.Name,
					RestartCount:  container.RestartCount,
					Description:   fmt.Sprintf("Container state: %s", container.State),
				}

				if container.RestartCount > 10 {
					containerIssue.IssueType = "restart_loop"
					containerIssue.Severity = "high"
				} else if container.RestartCount > 3 {
					containerIssue.IssueType = "frequent_restarts"
					containerIssue.Severity = "medium"
				} else if !container.Ready {
					containerIssue.IssueType = "not_ready"
					containerIssue.Severity = "medium"
				}

				if container.Reason != "" {
					containerIssue.Description = fmt.Sprintf("%s: %s", container.Reason, container.Message)

					if strings.Contains(strings.ToLower(container.Reason), "oom") {
						containerIssue.IssueType = "oom_killed"
						containerIssue.Severity = "high"
					}
				}

				ctx.ContainerIssues = append(ctx.ContainerIssues, containerIssue)
			}
		}
	}

	// Update restart analysis
	ctx.RestartAnalysis = RestartAnalysis{
		TotalRestarts:    totalRestarts,
		PodsWithRestarts: podsWithRestarts,
	}

	// Identify high restart pods
	for _, pod := range healthResult.Pods {
		for _, container := range pod.Containers {
			if container.RestartCount > 5 {
				ctx.RestartAnalysis.HighRestartPods = append(
					ctx.RestartAnalysis.HighRestartPods,
					fmt.Sprintf("%s (%d restarts)", pod.Name, container.RestartCount),
				)
			}
		}
	}
}

// analyzeRestartPatterns analyzes pod restart patterns for stability issues
func (t *AtomicCheckHealthTool) analyzeRestartPatterns(ctx *HealthContext, healthResult *kubernetes.HealthCheckResult) {
	if ctx.RestartAnalysis.TotalRestarts > 0 {
		if ctx.RestartAnalysis.TotalRestarts > 20 {
			ctx.StabilityIssues = append(ctx.StabilityIssues,
				fmt.Sprintf("High number of total restarts (%d) indicates stability issues",
					ctx.RestartAnalysis.TotalRestarts))
		}

		if len(ctx.RestartAnalysis.HighRestartPods) > 0 {
			ctx.StabilityIssues = append(ctx.StabilityIssues,
				"Some pods have excessive restart counts - investigate underlying causes")
		}
	}
}

// generateHealthContext generates rich context for Claude reasoning
func (t *AtomicCheckHealthTool) generateHealthContext(result *AtomicCheckHealthResult, args AtomicCheckHealthArgs) {
	ctx := result.HealthContext

	// Generate health recommendations
	if ctx.OverallStatus == types.HealthStatusHealthy {
		ctx.HealthRecommendations = append(ctx.HealthRecommendations,
			"Application is healthy - monitor for continued stability")
		ctx.NextStepSuggestions = append(ctx.NextStepSuggestions,
			"Application is running well - consider setting up monitoring and alerting")
	} else {
		ctx.HealthRecommendations = append(ctx.HealthRecommendations,
			"Application has health issues - investigate and resolve pod problems")
		ctx.NextStepSuggestions = append(ctx.NextStepSuggestions,
			"Check pod logs and events to diagnose issues")
	}

	// Add specific recommendations based on issues
	if len(ctx.PodIssues) > 0 {
		ctx.HealthRecommendations = append(ctx.HealthRecommendations,
			"Address pod-level issues to improve application health")
	}

	if len(ctx.ContainerIssues) > 0 {
		ctx.HealthRecommendations = append(ctx.HealthRecommendations,
			"Investigate container restart issues to improve stability")
	}

	if ctx.RestartAnalysis.TotalRestarts > 5 {
		ctx.HealthRecommendations = append(ctx.HealthRecommendations,
			"Review application configuration and resource limits to reduce restarts")
	}

	// Add monitoring recommendations
	ctx.HealthRecommendations = append(ctx.HealthRecommendations,
		"Set up health check endpoints and monitoring dashboards")
	ctx.HealthRecommendations = append(ctx.HealthRecommendations,
		"Configure alerting for pod failures and high restart rates")

	// Generate next steps
	if result.WaitResult != nil && result.WaitResult.Success {
		ctx.NextStepSuggestions = append(ctx.NextStepSuggestions,
			"Application became ready after waiting - monitor for stability")
	} else if args.WaitForReady && result.WaitResult != nil {
		ctx.NextStepSuggestions = append(ctx.NextStepSuggestions,
			"Application did not become ready - investigate deployment issues")
	}

	ctx.NextStepSuggestions = append(ctx.NextStepSuggestions,
		"Use this health check regularly to monitor application status")
}

// addTroubleshootingTips adds troubleshooting tips based on errors
func (t *AtomicCheckHealthTool) addTroubleshootingTips(result *AtomicCheckHealthResult, stage string, err error) {
	ctx := result.HealthContext
	errStr := strings.ToLower(err.Error())

	if strings.Contains(errStr, "unauthorized") || strings.Contains(errStr, "forbidden") {
		ctx.TroubleshootingTips = append(ctx.TroubleshootingTips,
			"Check Kubernetes RBAC permissions for health checking")
	}

	if strings.Contains(errStr, "connection") || strings.Contains(errStr, "cluster") {
		ctx.TroubleshootingTips = append(ctx.TroubleshootingTips,
			"Verify Kubernetes cluster connectivity")
	}

	if strings.Contains(errStr, "namespace") {
		ctx.TroubleshootingTips = append(ctx.TroubleshootingTips,
			"Check if the target namespace exists")
	}

	if strings.Contains(errStr, "not found") {
		ctx.TroubleshootingTips = append(ctx.TroubleshootingTips,
			"No resources found matching the label selector - verify deployment")
	}
}

// updateSessionState updates the session with health check results
func (t *AtomicCheckHealthTool) updateSessionState(session *sessiontypes.SessionState, result *AtomicCheckHealthResult) error {
	// Update session with health check results
	if session.Metadata == nil {
		session.Metadata = make(map[string]interface{})
	}

	session.Metadata["last_health_check"] = time.Now().Format(time.RFC3339)
	session.Metadata["health_status"] = result.HealthContext.OverallStatus
	session.Metadata["health_score"] = result.HealthContext.HealthScore
	session.Metadata["pods_ready"] = result.HealthContext.PodSummary.ReadyPods
	session.Metadata["pods_total"] = result.HealthContext.PodSummary.TotalPods
	session.Metadata["health_issues_count"] = len(result.HealthContext.PodIssues)

	if result.HealthResult != nil && result.HealthResult.Success {
		session.Metadata["health_check_success"] = true
	} else {
		session.Metadata["health_check_success"] = false
	}

	session.UpdateLastAccessed()

	return t.sessionManager.UpdateSession(session.SessionID, func(s *sessiontypes.SessionState) { *s = *session })
}

// Helper methods

func (t *AtomicCheckHealthTool) buildLabelSelector(args AtomicCheckHealthArgs, session *sessiontypes.SessionState) string {
	if args.LabelSelector != "" {
		return args.LabelSelector
	}

	if args.AppName != "" {
		return fmt.Sprintf("app=%s", args.AppName)
	}

	// Try to get app name from session metadata
	if session.Metadata != nil {
		if lastDeployedApp, ok := session.Metadata["last_deployed_app"].(string); ok && lastDeployedApp != "" {
			return fmt.Sprintf("app=%s", lastDeployedApp)
		}
	}

	// Default label selector
	return types.AppLabel
}

func (t *AtomicCheckHealthTool) getNamespace(namespace string) string {
	if namespace == "" {
		return "default"
	}
	return namespace
}

func (t *AtomicCheckHealthTool) getWaitTimeout(timeout int) time.Duration {
	if timeout <= 0 {
		return 5 * time.Minute // Default 5 minutes
	}
	return time.Duration(timeout) * time.Second
}

func (t *AtomicCheckHealthTool) determinePodIssueSeverity(status string) string {
	switch strings.ToLower(status) {
	case "failed", "error", "crashloopbackoff":
		return "high"
	case types.HealthStatusPending:
		return "medium"
	default:
		return "low"
	}
}

// AI Context Interface Implementations for AtomicCheckHealthResult

// SimpleTool interface implementation

// GetName returns the tool name
func (t *AtomicCheckHealthTool) GetName() string {
	return "atomic_check_health"
}

// GetDescription returns the tool description
func (t *AtomicCheckHealthTool) GetDescription() string {
	return "Performs comprehensive health checks on Kubernetes applications including pod status, service availability, and resource utilization"
}

// GetVersion returns the tool version
func (t *AtomicCheckHealthTool) GetVersion() string {
	return "1.0.0"
}

// GetCapabilities returns the tool capabilities
func (t *AtomicCheckHealthTool) GetCapabilities() contract.ToolCapabilities {
	return contract.ToolCapabilities{
		SupportsDryRun:    true,
		SupportsStreaming: true,
		IsLongRunning:     true,
		RequiresAuth:      false,
	}
}

// Validate validates the tool arguments
func (t *AtomicCheckHealthTool) Validate(ctx context.Context, args interface{}) error {
	healthArgs, ok := args.(AtomicCheckHealthArgs)
	if !ok {
		return types.NewValidationErrorBuilder("Invalid argument type for atomic_check_health", "args", args).
			WithField("expected", "AtomicCheckHealthArgs").
			WithField("received", fmt.Sprintf("%T", args)).
			Build()
	}

	if healthArgs.SessionID == "" {
		return types.NewValidationErrorBuilder("SessionID is required", "session_id", healthArgs.SessionID).
			WithField("field", "session_id").
			Build()
	}

	// Validate either app_name or label_selector is provided
	if healthArgs.AppName == "" && healthArgs.LabelSelector == "" {
		return types.NewValidationErrorBuilder("Either app_name or label_selector must be provided", "selection", "").
			WithField("app_name", healthArgs.AppName).
			WithField("label_selector", healthArgs.LabelSelector).
			Build()
	}

	return nil
}

// Execute implements SimpleTool interface with generic signature
func (t *AtomicCheckHealthTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	healthArgs, ok := args.(AtomicCheckHealthArgs)
	if !ok {
		return nil, types.NewValidationErrorBuilder("Invalid argument type for atomic_check_health", "args", args).
			WithField("expected", "AtomicCheckHealthArgs").
			WithField("received", fmt.Sprintf("%T", args)).
			Build()
	}

	// Call the typed Execute method
	return t.ExecuteTyped(ctx, healthArgs)
}

// ExecuteTyped provides the original typed execute method
func (t *AtomicCheckHealthTool) ExecuteTyped(ctx context.Context, args AtomicCheckHealthArgs) (*AtomicCheckHealthResult, error) {
	return t.ExecuteHealthCheck(ctx, args)
}

// AI Context methods are now provided by embedded BaseAIContextResult

// convertHealthCheckResult converts from mcptypes.HealthCheckResult to kubernetes.HealthCheckResult
func (t *AtomicCheckHealthTool) convertHealthCheckResult(result *mcptypes.HealthCheckResult, namespace string) *kubernetes.HealthCheckResult {
	if result == nil {
		return nil
	}
	
	k8sResult := &kubernetes.HealthCheckResult{
		Success:   result.Healthy,
		Namespace: namespace,
	}
	
	if result.Error != nil {
		k8sResult.Error = &kubernetes.HealthCheckError{
			Type:    result.Error.Type,
			Message: result.Error.Message,
		}
	}
	
	// Convert pod statuses
	for _, ps := range result.PodStatuses {
		podStatus := kubernetes.DetailedPodStatus{
			Name:      ps.Name,
			Namespace: namespace,
			Status:    ps.Status,
			Ready:     ps.Ready,
		}
		k8sResult.Pods = append(k8sResult.Pods, podStatus)
	}
	
	// Update summary
	k8sResult.Summary = kubernetes.HealthSummary{
		TotalPods:   len(k8sResult.Pods),
		ReadyPods:   0,
		FailedPods:  0,
		PendingPods: 0,
	}
	
	for _, pod := range k8sResult.Pods {
		if pod.Ready {
			k8sResult.Summary.ReadyPods++
		} else if pod.Status == "Failed" || pod.Phase == "Failed" {
			k8sResult.Summary.FailedPods++
		} else if pod.Status == "Pending" || pod.Phase == "Pending" {
			k8sResult.Summary.PendingPods++
		}
	}
	
	if k8sResult.Summary.TotalPods > 0 {
		k8sResult.Summary.HealthyRatio = float64(k8sResult.Summary.ReadyPods) / float64(k8sResult.Summary.TotalPods)
	}
	
	return k8sResult
}
