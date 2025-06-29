package deploy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/core/kubernetes"
	"github.com/Azure/container-kit/pkg/mcp/core"

	// mcp import removed - using mcptypes
	"github.com/Azure/container-kit/pkg/mcp/internal/build"
	"github.com/Azure/container-kit/pkg/mcp/internal/observability"
	"github.com/Azure/container-kit/pkg/mcp/internal/retry"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"

	mcptypes "github.com/Azure/container-kit/pkg/mcp/core"
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
	mcptypes.BaseAIContextResult      // Embed AI context methods
	Success                      bool `json:"success"`

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
	pipelineAdapter  mcptypes.PipelineOperations
	sessionManager   core.ToolSessionManager
	retryCoordinator *retry.Coordinator
	// errorHandler field removed - using direct error handling
	logger      zerolog.Logger
	analyzer    build.ToolAnalyzer
	fixingMixin *build.AtomicToolFixingMixin
}

// NewAtomicCheckHealthTool creates a new atomic check health tool
func NewAtomicCheckHealthTool(adapter mcptypes.PipelineOperations, sessionManager core.ToolSessionManager, logger zerolog.Logger) *AtomicCheckHealthTool {
	coordinator := retry.New()

	// Set up health check specific retry policy
	coordinator.SetPolicy("health_check", &retry.Policy{
		MaxAttempts:     10, // More attempts for health checks
		InitialDelay:    5 * time.Second,
		MaxDelay:        30 * time.Second,
		BackoffStrategy: retry.BackoffExponential,
		Multiplier:      1.5, // Gentler escalation for health checks
		Jitter:          true,
		ErrorPatterns: []string{
			"timeout", "connection refused", "network unreachable",
			"service unavailable", "not ready", "pending",
		},
	})

	return &AtomicCheckHealthTool{
		pipelineAdapter:  adapter,
		sessionManager:   sessionManager,
		retryCoordinator: coordinator,
		// errorHandler initialization removed - using direct error handling
		logger: logger.With().Str("tool", "atomic_check_health").Logger(),
	}
}

// SetAnalyzer sets the analyzer for failure analysis
func (t *AtomicCheckHealthTool) SetAnalyzer(analyzer build.ToolAnalyzer) {
	t.analyzer = analyzer
}

// SetFixingMixin sets the fixing mixin for automatic error recovery
func (t *AtomicCheckHealthTool) SetFixingMixin(mixin *build.AtomicToolFixingMixin) {
	t.fixingMixin = mixin
}

// standardHealthCheckStages provides common stages for health check operations
// standardHealthCheckStages is no longer used with unified progress implementation
// func standardHealthCheckStages() []internal.LocalProgressStage {
// 	return []internal.LocalProgressStage{
// 		{Name: "Initialize", Weight: 0.10, Description: "Loading session and namespace"},
// 		{Name: "Query", Weight: 0.30, Description: "Querying Kubernetes resources"},
// 		{Name: "Analyze", Weight: 0.30, Description: "Analyzing pod and service health"},
// 		{Name: "Wait", Weight: 0.20, Description: "Waiting for ready state (if requested)"},
// 		{Name: "Report", Weight: 0.10, Description: "Generating health report"},
// 	}
// }

// ExecuteHealthCheck runs the atomic application health check
func (t *AtomicCheckHealthTool) ExecuteHealthCheck(ctx context.Context, args AtomicCheckHealthArgs) (*AtomicCheckHealthResult, error) {
	// Direct execution without progress tracking
	return t.executeWithoutProgress(ctx, args)
}

// ExecuteWithContext runs the atomic health check with GoMCP progress tracking
func (t *AtomicCheckHealthTool) ExecuteWithContext(serverCtx *server.Context, args AtomicCheckHealthArgs) (*AtomicCheckHealthResult, error) {
	// Create progress adapter for GoMCP using centralized health stages
	progress := observability.NewUnifiedProgressReporter(serverCtx)

	// Execute with progress tracking
	ctx := context.Background()
	result, err := t.performHealthCheck(ctx, args, progress)

	// Complete progress tracking
	if err != nil {
		t.logger.Info().Msg("Health check failed")
	} else {
		t.logger.Info().Msg("Health check completed successfully")
	}

	return result, err
}

// executeWithoutProgress executes without progress tracking
func (t *AtomicCheckHealthTool) executeWithoutProgress(ctx context.Context, args AtomicCheckHealthArgs) (*AtomicCheckHealthResult, error) {
	return t.performHealthCheck(ctx, args, nil)
}

// performHealthCheck performs the actual health check
func (t *AtomicCheckHealthTool) performHealthCheck(ctx context.Context, args AtomicCheckHealthArgs, reporter interface{}) (*AtomicCheckHealthResult, error) {
	startTime := time.Now()

	// Get session
	sessionInterface, err := t.sessionManager.GetSession(args.SessionID)
	if err != nil {
		// Create result with error for session failure
		result := &AtomicCheckHealthResult{
			BaseToolResponse:    types.NewBaseResponse("atomic_check_health", args.SessionID, args.DryRun),
			BaseAIContextResult: mcptypes.NewBaseAIContextResult("health", false, time.Since(startTime)),
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
	session := sessionInterface.(*core.SessionState)

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
	// Progress reporting removed

	// Create base response
	result := &AtomicCheckHealthResult{
		BaseToolResponse:    types.NewBaseResponse("atomic_check_health", session.SessionID, args.DryRun),
		BaseAIContextResult: mcptypes.NewBaseAIContextResult("health", true, 0), // Duration will be set later
		SessionID:           session.SessionID,
		Namespace:           namespace,
		LabelSelector:       labelSelector,
		HealthContext:       &HealthContext{},
	}

	// Progress reporting removed

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
	// Progress reporting removed

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
	// Progress reporting removed

	// Perform health check using core operations
	healthStartTime := time.Now()
	// Use the correct interface method
	healthArgs := map[string]interface{}{
		"namespace":     namespace,
		"labelSelector": labelSelector,
		"timeout":       30 * time.Second,
	}
	healthResult, err := t.pipelineAdapter.CheckHealth(ctx, session.SessionID, healthArgs)
	result.HealthCheckDuration = time.Since(healthStartTime)

	// Convert from interface{} to kubernetes.HealthCheckResult
	if healthResult != nil {
		// Handle the interface{} result - try to convert it to the expected structure
		if healthMap, ok := healthResult.(map[string]interface{}); ok {
			result.HealthResult = &kubernetes.HealthCheckResult{
				Success:   getBoolFromMap(healthMap, "healthy", false),
				Namespace: namespace,
				Duration:  result.HealthCheckDuration,
			}

			// Handle error if present
			if errorData, exists := healthMap["error"]; exists && errorData != nil {
				if errorMap, ok := errorData.(map[string]interface{}); ok {
					result.HealthResult.Error = &kubernetes.HealthCheckError{
						Type:    getStringFromMap(errorMap, "type", "unknown"),
						Message: getStringFromMap(errorMap, "message", "unknown error"),
					}
				}
			}

			// Convert pod statuses if present
			if podStatusData, exists := healthMap["pod_statuses"]; exists {
				if podStatuses, ok := podStatusData.([]interface{}); ok {
					for _, ps := range podStatuses {
						if psMap, ok := ps.(map[string]interface{}); ok {
							podStatus := kubernetes.DetailedPodStatus{
								Name:      getStringFromMap(psMap, "name", "unknown"),
								Namespace: namespace,
								Status:    getStringFromMap(psMap, "status", "unknown"),
								Ready:     getBoolFromMap(psMap, "ready", false),
							}
							result.HealthResult.Pods = append(result.HealthResult.Pods, podStatus)
						}
					}
				}
			}
		}

		// Update summary if we have pods
		if result.HealthResult != nil {
			result.HealthResult.Summary = kubernetes.HealthSummary{
				TotalPods:   len(result.HealthResult.Pods),
				ReadyPods:   0,
				FailedPods:  0,
				PendingPods: 0,
			}

			// Count pod statuses
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
	}

	// Progress reporting removed

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
	// Progress reporting removed

	// Analyze the health results to populate the context
	result.Success = result.HealthResult != nil && result.HealthResult.Success

	// Progress reporting removed

	// Stage 4: Wait for readiness if requested
	if args.WaitForReady && (result.HealthResult == nil || !result.HealthResult.Success) {
		// Progress reporting removed

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

		// Progress reporting removed
	}

	// Stage 5: Generate health report
	// Progress reporting removed

	// Analyze health results in detail
	t.analyzeApplicationHealth(result, args)

	// Progress reporting removed

	result.TotalDuration = time.Since(startTime)
	// Update BaseAIContextResult fields
	result.BaseAIContextResult.Duration = result.TotalDuration
	result.BaseAIContextResult.IsSuccessful = result.Success

	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("overall_status", result.HealthContext.OverallStatus).
		Float64("health_score", result.HealthContext.HealthScore).
		Dur("total_duration", result.TotalDuration).
		Msg("Atomic application health check completed successfully")

	// Progress reporting removed

	return result, nil
}

// validateHealthCheckPrerequisites validates health check prerequisites
func (t *AtomicCheckHealthTool) validateHealthCheckPrerequisites(result *AtomicCheckHealthResult, args AtomicCheckHealthArgs) error {
	if args.AppName == "" && args.LabelSelector == "" {
		return fmt.Errorf("error")
	}

	return nil
}

// waitForApplicationReady implements waiting for application readiness using unified retry coordinator
func (t *AtomicCheckHealthTool) waitForApplicationReady(ctx context.Context, sessionID, namespace, labelSelector string, timeout time.Duration) *kubernetes.HealthCheckResult {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var finalResult *kubernetes.HealthCheckResult

	// Use retry coordinator for structured waiting
	err := t.retryCoordinator.Execute(timeoutCtx, "health_check", func(ctx context.Context) error {
		// Use the correct interface method
		healthArgs := map[string]interface{}{
			"namespace":     namespace,
			"labelSelector": labelSelector,
			"timeout":       30 * time.Second,
		}
		result, err := t.pipelineAdapter.CheckHealth(ctx, sessionID, healthArgs)
		if err != nil {
			t.logger.Debug().Err(err).Msg("Health check attempt failed, will retry")
			return err
		}

		if result == nil {
			return fmt.Errorf("health check returned nil result")
		}

		// Convert result for return
		if resultMap, ok := result.(map[string]interface{}); ok {
			finalResult = t.convertHealthCheckResult(resultMap, namespace)
		}

		if resultMap, ok := result.(map[string]interface{}); ok && getBoolFromMap(resultMap, "healthy", false) {
			// Success! Application is ready
			t.logger.Info().
				Str("session_id", sessionID).
				Str("namespace", namespace).
				Str("label_selector", labelSelector).
				Msg("Application became ready")
			return nil
		}

		// Not ready yet, continue retrying
		return fmt.Errorf("application not ready")
	})

	if err != nil {
		t.logger.Warn().Err(err).
			Str("session_id", sessionID).
			Str("namespace", namespace).
			Str("label_selector", labelSelector).
			Dur("timeout", timeout).
			Msg("Application did not become ready within timeout")
	}

	return finalResult
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
func (t *AtomicCheckHealthTool) updateSessionState(session *core.SessionState, result *AtomicCheckHealthResult) error {
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

	session.UpdatedAt = time.Now()

	// Use the correct interface method for session updates
	return t.pipelineAdapter.UpdateSessionState(session.SessionID, func(s *core.SessionState) {
		*s = *session
	})
}

// Helper methods

func (t *AtomicCheckHealthTool) buildLabelSelector(args AtomicCheckHealthArgs, session *core.SessionState) string {
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
func (t *AtomicCheckHealthTool) GetCapabilities() types.ToolCapabilities {
	return types.ToolCapabilities{
		SupportsDryRun:    true,
		SupportsStreaming: true,
		IsLongRunning:     true,
		RequiresAuth:      false,
	}
}

// GetMetadata returns comprehensive metadata about the tool
func (t *AtomicCheckHealthTool) GetMetadata() core.ToolMetadata {
	return core.ToolMetadata{
		Name:        "atomic_check_health",
		Description: "Performs comprehensive health checks on Kubernetes applications including pod status, service availability, and resource utilization",
		Version:     "1.0.0",
		Category:    "monitoring",
		Dependencies: []string{
			"kubernetes_access",
			"network_access",
		},
		Capabilities: []string{
			"endpoint_monitoring",
			"kubernetes_probes",
			"custom_checks",
			"pod_analysis",
			"service_discovery",
			"health_scoring",
		},
		Requirements: []string{
			"kubernetes_access",
			"network_access",
		},
		Parameters: map[string]string{
			"session_id":        "string - Session ID for session context",
			"namespace":         "string - Kubernetes namespace (default: default)",
			"app_name":          "string - Application name for label selection",
			"label_selector":    "string - Custom label selector",
			"include_services":  "bool - Include service health checks",
			"include_events":    "bool - Include pod events in analysis",
			"wait_for_ready":    "bool - Wait for pods to become ready",
			"wait_timeout":      "int - Wait timeout in seconds",
			"detailed_analysis": "bool - Perform detailed container analysis",
			"include_logs":      "bool - Include recent container logs",
			"log_lines":         "int - Number of log lines to include",
		},
		Examples: []mcptypes.ToolExample{
			{
				Name:        "Basic Health Check",
				Description: "Check health of application with app name",
				Input: map[string]interface{}{
					"session_id": "session-123",
					"app_name":   "my-app",
					"namespace":  "default",
				},
				Output: map[string]interface{}{
					"success":        true,
					"overall_status": "healthy",
					"health_score":   1.0,
					"pods_ready":     3,
					"pods_total":     3,
				},
			},
			{
				Name:        "Health Check with Wait",
				Description: "Check health and wait for application to become ready",
				Input: map[string]interface{}{
					"session_id":     "session-123",
					"label_selector": "app=my-app,version=v1",
					"wait_for_ready": true,
					"wait_timeout":   300,
				},
				Output: map[string]interface{}{
					"success":        true,
					"overall_status": "healthy",
					"wait_duration":  "45s",
				},
			},
		},
	}
}

// Validate validates the tool arguments
func (t *AtomicCheckHealthTool) Validate(ctx context.Context, args interface{}) error {
	healthArgs, ok := args.(AtomicCheckHealthArgs)
	if !ok {
		return fmt.Errorf("error")
	}

	if healthArgs.SessionID == "" {
		return fmt.Errorf("error")
	}

	// Validate either app_name or label_selector is provided
	if healthArgs.AppName == "" && healthArgs.LabelSelector == "" {
		return fmt.Errorf("error")
	}

	return nil
}

// Execute implements SimpleTool interface with generic signature
func (t *AtomicCheckHealthTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	healthArgs, ok := args.(AtomicCheckHealthArgs)
	if !ok {
		return nil, fmt.Errorf("error")
	}

	// Call the typed Execute method
	return t.ExecuteTyped(ctx, healthArgs)
}

// ExecuteTyped provides the original typed execute method
func (t *AtomicCheckHealthTool) ExecuteTyped(ctx context.Context, args AtomicCheckHealthArgs) (*AtomicCheckHealthResult, error) {
	return t.ExecuteHealthCheck(ctx, args)
}

// AI Context methods are now provided by embedded internal.BaseAIContextResult

// convertHealthCheckResult converts from mcptypes.HealthCheckResult to kubernetes.HealthCheckResult
func (t *AtomicCheckHealthTool) convertHealthCheckResult(result map[string]interface{}, namespace string) *kubernetes.HealthCheckResult {
	if result == nil {
		return nil
	}

	k8sResult := &kubernetes.HealthCheckResult{
		Success:   getBoolFromMap(result, "healthy", false),
		Namespace: namespace,
	}

	if errorData, exists := result["error"]; exists && errorData != nil {
		if errorMap, ok := errorData.(map[string]interface{}); ok {
			k8sResult.Error = &kubernetes.HealthCheckError{
				Type:    getStringFromMap(errorMap, "type", "unknown"),
				Message: getStringFromMap(errorMap, "message", "unknown error"),
			}
		}
	}

	// Convert pod statuses
	if podStatusData, exists := result["pod_statuses"]; exists {
		if podStatuses, ok := podStatusData.([]interface{}); ok {
			for _, ps := range podStatuses {
				if psMap, ok := ps.(map[string]interface{}); ok {
					podStatus := kubernetes.DetailedPodStatus{
						Name:      getStringFromMap(psMap, "name", "unknown"),
						Namespace: namespace,
						Status:    getStringFromMap(psMap, "status", "unknown"),
						Ready:     getBoolFromMap(psMap, "ready", false),
					}
					k8sResult.Pods = append(k8sResult.Pods, podStatus)
				}
			}
		}
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

// Helper functions for map access
func getStringFromMap(m map[string]interface{}, key, defaultValue string) string {
	if val, exists := m[key]; exists {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return defaultValue
}

func getBoolFromMap(m map[string]interface{}, key string, defaultValue bool) bool {
	if val, exists := m[key]; exists {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return defaultValue
}
