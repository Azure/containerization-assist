package deploy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal"
	"github.com/Azure/container-kit/pkg/mcp/internal/build"
	"github.com/Azure/container-kit/pkg/mcp/internal/retry"

	"github.com/Azure/container-kit/pkg/core/kubernetes"
	sessiontypes "github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
	"github.com/localrivet/gomcp/server"
	"github.com/rs/zerolog"
)

type AtomicCheckHealthArgs struct {
	types.BaseToolArgs

	Namespace     string `json:"namespace,omitempty" description:"Kubernetes namespace (default: default)"`
	AppName       string `json:"app_name,omitempty" description:"Application name for label selection"`
	LabelSelector string `json:"label_selector,omitempty" description:"Custom label selector (e.g., app=myapp,version=v1)"`

	IncludeServices bool `json:"include_services,omitempty" description:"Include service health checks (default: true)"`
	IncludeEvents   bool `json:"include_events,omitempty" description:"Include pod events in analysis (default: true)"`
	WaitForReady    bool `json:"wait_for_ready,omitempty" description:"Wait for pods to become ready"`
	WaitTimeout     int  `json:"wait_timeout,omitempty" description:"Wait timeout in seconds (default: 300)"`

	DetailedAnalysis bool `json:"detailed_analysis,omitempty" description:"Perform detailed container and condition analysis"`
	IncludeLogs      bool `json:"include_logs,omitempty" description:"Include recent container logs in analysis"`
	LogLines         int  `json:"log_lines,omitempty" description:"Number of log lines to include (default: 50)"`
}

type AtomicCheckHealthResult struct {
	types.BaseToolResponse
	internal.BaseAIContextResult
	Success bool `json:"success"`

	SessionID     string `json:"session_id"`
	Namespace     string `json:"namespace"`
	LabelSelector string `json:"label_selector"`

	HealthResult *kubernetes.HealthCheckResult `json:"health_result"`

	WaitResult *kubernetes.HealthCheckResult `json:"wait_result,omitempty"`

	HealthCheckDuration time.Duration `json:"health_check_duration"`
	WaitDuration        time.Duration `json:"wait_duration,omitempty"`
	TotalDuration       time.Duration `json:"total_duration"`

	HealthContext *HealthContext `json:"health_context"`
}

type HealthContext struct {
	OverallStatus  string  `json:"overall_status"`
	HealthScore    float64 `json:"health_score"`
	ReadinessRatio float64 `json:"readiness_ratio"`

	PodSummary      PodSummary       `json:"pod_summary"`
	PodIssues       []PodIssue       `json:"pod_issues"`
	ContainerIssues []ContainerIssue `json:"container_issues"`

	ServiceSummary ServiceSummary `json:"service_summary"`
	ServiceIssues  []string       `json:"service_issues"`

	ResourceUsage     ResourceUsageInfo `json:"resource_usage"`
	PerformanceIssues []string          `json:"performance_issues"`

	RestartAnalysis RestartAnalysis `json:"restart_analysis"`
	StabilityIssues []string        `json:"stability_issues"`

	HealthRecommendations []string `json:"health_recommendations"`
	NextStepSuggestions   []string `json:"next_step_suggestions"`
	TroubleshootingTips   []string `json:"troubleshooting_tips,omitempty"`
}

type PodSummary struct {
	TotalPods   int `json:"total_pods"`
	ReadyPods   int `json:"ready_pods"`
	PendingPods int `json:"pending_pods"`
	FailedPods  int `json:"failed_pods"`
	RunningPods int `json:"running_pods"`
}

type PodIssue struct {
	PodName     string   `json:"pod_name"`
	IssueType   string   `json:"issue_type"` // Issue type: not_ready, failed, pending, crashloop
	Description string   `json:"description"`
	Severity    string   `json:"severity"`
	Since       string   `json:"since"`
	Suggestions []string `json:"suggestions"`
}

type ContainerIssue struct {
	PodName       string `json:"pod_name"`
	ContainerName string `json:"container_name"`
	IssueType     string `json:"issue_type"`
	Description   string `json:"description"`
	Severity      string `json:"severity"`
	RestartCount  int    `json:"restart_count"`
	LastRestart   string `json:"last_restart,omitempty"`
}

type ServiceSummary struct {
	TotalServices   int `json:"total_services"`
	HealthyServices int `json:"healthy_services"`
	EndpointsReady  int `json:"endpoints_ready"`
	EndpointsTotal  int `json:"endpoints_total"`
}

type ResourceUsageInfo struct {
	HighCPUPods      []string `json:"high_cpu_pods"`
	HighMemoryPods   []string `json:"high_memory_pods"`
	ResourceWarnings []string `json:"resource_warnings"`
}

type RestartAnalysis struct {
	TotalRestarts    int      `json:"total_restarts"`
	PodsWithRestarts int      `json:"pods_with_restarts"`
	HighRestartPods  []string `json:"high_restart_pods"`
	RecentRestarts   int      `json:"recent_restarts"`
}

type AtomicCheckHealthTool struct {
	pipelineAdapter  mcptypes.PipelineOperations
	sessionManager   mcptypes.ToolSessionManager
	retryCoordinator *retry.Coordinator
	logger           zerolog.Logger
	analyzer         ToolAnalyzer
	fixingMixin      *build.AtomicToolFixingMixin
}

func NewAtomicCheckHealthTool(adapter mcptypes.PipelineOperations, sessionManager mcptypes.ToolSessionManager, logger zerolog.Logger) *AtomicCheckHealthTool {
	coordinator := retry.New()

	coordinator.SetPolicy("health_check", &retry.Policy{
		MaxAttempts:     10,
		InitialDelay:    5 * time.Second,
		MaxDelay:        30 * time.Second,
		BackoffStrategy: retry.BackoffExponential,
		Multiplier:      1.5,
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
		logger:           logger.With().Str("tool", "atomic_check_health").Logger(),
	}
}

func (t *AtomicCheckHealthTool) SetAnalyzer(analyzer ToolAnalyzer) {
	t.analyzer = analyzer
}

func (t *AtomicCheckHealthTool) SetFixingMixin(mixin *build.AtomicToolFixingMixin) {
	t.fixingMixin = mixin
}

func standardHealthCheckStages() []internal.LocalProgressStage {
	return []internal.LocalProgressStage{
		{Name: "Initialize", Weight: 0.10, Description: "Loading session and namespace"},
		{Name: "Query", Weight: 0.30, Description: "Querying Kubernetes resources"},
		{Name: "Analyze", Weight: 0.30, Description: "Analyzing pod and service health"},
		{Name: "Wait", Weight: 0.20, Description: "Waiting for ready state (if requested)"},
		{Name: "Report", Weight: 0.10, Description: "Generating health report"},
	}
}

func (t *AtomicCheckHealthTool) ExecuteHealthCheck(ctx context.Context, args AtomicCheckHealthArgs) (*AtomicCheckHealthResult, error) {
	return t.executeWithoutProgress(ctx, args)
}

func (t *AtomicCheckHealthTool) ExecuteWithContext(serverCtx *server.Context, args AtomicCheckHealthArgs) (*AtomicCheckHealthResult, error) {
	_ = internal.NewGoMCPProgressAdapter(serverCtx, []internal.LocalProgressStage{{Name: "Initialize", Weight: 0.10, Description: "Loading session"}, {Name: "Health", Weight: 0.80, Description: "Checking health"}, {Name: "Finalize", Weight: 0.10, Description: "Updating state"}})

	ctx := context.Background()
	result, err := t.performHealthCheck(ctx, args, nil)

	if err != nil {
		t.logger.Info().Msg("Health check failed")
	} else {
		t.logger.Info().Msg("Health check completed successfully")
	}

	return result, err
}

func (t *AtomicCheckHealthTool) executeWithoutProgress(ctx context.Context, args AtomicCheckHealthArgs) (*AtomicCheckHealthResult, error) {
	return t.performHealthCheck(ctx, args, nil)
}

func (t *AtomicCheckHealthTool) performHealthCheck(ctx context.Context, args AtomicCheckHealthArgs, reporter interface{}) (*AtomicCheckHealthResult, error) {
	startTime := time.Now()

	sessionInterface, err := t.sessionManager.GetSession(args.SessionID)
	if err != nil {
		result := &AtomicCheckHealthResult{
			BaseToolResponse:    types.NewBaseResponse("atomic_check_health", args.SessionID, args.DryRun),
			BaseAIContextResult: internal.NewBaseAIContextResult("health", false, time.Since(startTime)),
			SessionID:           args.SessionID,
			Namespace:           t.getNamespace(args.Namespace),
			TotalDuration:       time.Since(startTime),
			HealthContext:       &HealthContext{},
		}
		result.Success = false

		t.logger.Error().Err(err).
			Str("session_id", args.SessionID).
			Msg("Failed to get session")
		return result, nil
	}
	session := sessionInterface.(*sessiontypes.SessionState)

	labelSelector := t.buildLabelSelector(args, session)
	namespace := t.getNamespace(args.Namespace)

	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("namespace", namespace).
		Str("label_selector", labelSelector).
		Bool("wait_for_ready", args.WaitForReady).
		Msg("Starting atomic application health check")

	result := &AtomicCheckHealthResult{
		BaseToolResponse:    types.NewBaseResponse("atomic_check_health", session.SessionID, args.DryRun),
		BaseAIContextResult: internal.NewBaseAIContextResult("health", true, 0), // Duration will be set later
		SessionID:           session.SessionID,
		Namespace:           namespace,
		LabelSelector:       labelSelector,
		HealthContext:       &HealthContext{},
	}

	if args.DryRun {
		result.HealthContext.NextStepSuggestions = []string{
			"This is a dry-run - actual health check would be performed",
			fmt.Sprintf("Would check health in namespace: %s", namespace),
			fmt.Sprintf("Using label selector: %s", labelSelector),
		}
		result.TotalDuration = time.Since(startTime)
		return result, nil
	}

	if err := t.validateHealthCheckPrerequisites(result, args); err != nil {
		t.logger.Error().Err(err).
			Str("session_id", session.SessionID).
			Str("namespace", namespace).
			Str("label_selector", labelSelector).
			Msg("Health check prerequisites validation failed")
		result.Success = false
		result.TotalDuration = time.Since(startTime)
		return result, nil
	}

	healthStartTime := time.Now()
	healthResult, err := t.pipelineAdapter.CheckApplicationHealth(
		session.SessionID,
		namespace,
		labelSelector,
		30*time.Second, // Default timeout for health checks
	)
	result.HealthCheckDuration = time.Since(healthStartTime)

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
		for _, ps := range healthResult.PodStatuses {
			podStatus := kubernetes.DetailedPodStatus{
				Name:      ps.Name,
				Namespace: namespace,
				Status:    ps.Status,
				Ready:     ps.Ready,
			}
			result.HealthResult.Pods = append(result.HealthResult.Pods, podStatus)
		}
		result.HealthResult.Summary = kubernetes.HealthSummary{
			TotalPods:   len(result.HealthResult.Pods),
			ReadyPods:   0,
			FailedPods:  0,
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

	if err != nil {
		t.logger.Error().Err(err).
			Str("session_id", session.SessionID).
			Str("namespace", namespace).
			Str("label_selector", labelSelector).
			Msg("Health check failed")
		t.addTroubleshootingTips(result, "health_check", err)
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

	result.Success = result.HealthResult != nil && result.HealthResult.Success

	if args.WaitForReady && (result.HealthResult == nil || !result.HealthResult.Success) {

		waitStartTime := time.Now()
		timeout := t.getWaitTimeout(args.WaitTimeout)

		t.logger.Info().
			Str("session_id", session.SessionID).
			Dur("timeout", timeout).
			Msg("Waiting for application to become ready")

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

	}

	t.analyzeApplicationHealth(result, args)

	result.TotalDuration = time.Since(startTime)
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

	return result, nil
}

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

func (t *AtomicCheckHealthTool) waitForApplicationReady(ctx context.Context, sessionID, namespace, labelSelector string, timeout time.Duration) *kubernetes.HealthCheckResult {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var finalResult *kubernetes.HealthCheckResult

	err := t.retryCoordinator.Execute(timeoutCtx, "health_check", func(ctx context.Context) error {
		result, err := t.pipelineAdapter.CheckApplicationHealth(sessionID, namespace, labelSelector, 30*time.Second)
		if err != nil {
			t.logger.Debug().Err(err).Msg("Health check attempt failed, will retry")
			return err
		}

		if result == nil {
			return fmt.Errorf("health check returned nil result")
		}

		finalResult = t.convertHealthCheckResult(result, namespace)

		if result.Healthy {
			t.logger.Info().
				Str("session_id", sessionID).
				Str("namespace", namespace).
				Str("label_selector", labelSelector).
				Msg("Application became ready")
			return nil
		}

		return fmt.Errorf("application not ready: %d/%d pods ready",
			len(result.PodStatuses), len(result.PodStatuses))
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

func (t *AtomicCheckHealthTool) analyzeApplicationHealth(result *AtomicCheckHealthResult, args AtomicCheckHealthArgs) {
	ctx := result.HealthContext
	healthResult := result.HealthResult

	ctx.ReadinessRatio = 0.0
	if healthResult.Summary.TotalPods > 0 {
		ctx.ReadinessRatio = float64(healthResult.Summary.ReadyPods) / float64(healthResult.Summary.TotalPods)
	}

	ctx.HealthScore = ctx.ReadinessRatio // Simple health score based on readiness

	if ctx.ReadinessRatio >= 1.0 {
		ctx.OverallStatus = types.HealthStatusHealthy
	} else if ctx.ReadinessRatio >= 0.7 {
		ctx.OverallStatus = types.HealthStatusDegraded
	} else if ctx.ReadinessRatio > 0.0 {
		ctx.OverallStatus = types.HealthStatusUnhealthy
	} else {
		ctx.OverallStatus = "unknown"
	}

	ctx.PodSummary = PodSummary{
		TotalPods:   healthResult.Summary.TotalPods,
		ReadyPods:   healthResult.Summary.ReadyPods,
		FailedPods:  healthResult.Summary.FailedPods,
		PendingPods: healthResult.Summary.PendingPods,
		RunningPods: healthResult.Summary.ReadyPods, // Simplification
	}

	ctx.ServiceSummary = ServiceSummary{
		TotalServices:   len(healthResult.Services),
		HealthyServices: len(healthResult.Services), // Assume all discovered services are healthy
		EndpointsReady:  healthResult.Summary.ReadyPods,
		EndpointsTotal:  healthResult.Summary.TotalPods,
	}

	t.analyzePodIssues(ctx, healthResult)

	t.analyzeRestartPatterns(ctx, healthResult)
}

func (t *AtomicCheckHealthTool) analyzePodIssues(ctx *HealthContext, healthResult *kubernetes.HealthCheckResult) {
	var totalRestarts int
	var podsWithRestarts int

	for _, pod := range healthResult.Pods {
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

	ctx.RestartAnalysis = RestartAnalysis{
		TotalRestarts:    totalRestarts,
		PodsWithRestarts: podsWithRestarts,
	}

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

func (t *AtomicCheckHealthTool) generateHealthContext(result *AtomicCheckHealthResult, args AtomicCheckHealthArgs) {
	ctx := result.HealthContext

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

	ctx.HealthRecommendations = append(ctx.HealthRecommendations,
		"Set up health check endpoints and monitoring dashboards")
	ctx.HealthRecommendations = append(ctx.HealthRecommendations,
		"Configure alerting for pod failures and high restart rates")

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

func (t *AtomicCheckHealthTool) updateSessionState(session *sessiontypes.SessionState, result *AtomicCheckHealthResult) error {
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

	return t.sessionManager.UpdateSession(session.SessionID, func(s interface{}) {
		if sess, ok := s.(*sessiontypes.SessionState); ok {
			*sess = *session
		}
	})
}

func (t *AtomicCheckHealthTool) buildLabelSelector(args AtomicCheckHealthArgs, session *sessiontypes.SessionState) string {
	if args.LabelSelector != "" {
		return args.LabelSelector
	}

	if args.AppName != "" {
		return fmt.Sprintf("app=%s", args.AppName)
	}

	if session.Metadata != nil {
		if lastDeployedApp, ok := session.Metadata["last_deployed_app"].(string); ok && lastDeployedApp != "" {
			return fmt.Sprintf("app=%s", lastDeployedApp)
		}
	}

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

func (t *AtomicCheckHealthTool) GetName() string {
	return "atomic_check_health"
}

func (t *AtomicCheckHealthTool) GetDescription() string {
	return "Performs comprehensive health checks on Kubernetes applications including pod status, service availability, and resource utilization"
}

func (t *AtomicCheckHealthTool) GetVersion() string {
	return "1.0.0"
}

func (t *AtomicCheckHealthTool) GetCapabilities() types.ToolCapabilities {
	return types.ToolCapabilities{
		SupportsDryRun:    true,
		SupportsStreaming: true,
		IsLongRunning:     true,
		RequiresAuth:      false,
	}
}

func (t *AtomicCheckHealthTool) GetMetadata() mcptypes.ToolMetadata {
	return mcptypes.ToolMetadata{
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

	if healthArgs.AppName == "" && healthArgs.LabelSelector == "" {
		return types.NewValidationErrorBuilder("Either app_name or label_selector must be provided", "selection", "").
			WithField("app_name", healthArgs.AppName).
			WithField("label_selector", healthArgs.LabelSelector).
			Build()
	}

	return nil
}

func (t *AtomicCheckHealthTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	healthArgs, ok := args.(AtomicCheckHealthArgs)
	if !ok {
		return nil, types.NewValidationErrorBuilder("Invalid argument type for atomic_check_health", "args", args).
			WithField("expected", "AtomicCheckHealthArgs").
			WithField("received", fmt.Sprintf("%T", args)).
			Build()
	}

	return t.ExecuteTyped(ctx, healthArgs)
}

func (t *AtomicCheckHealthTool) ExecuteTyped(ctx context.Context, args AtomicCheckHealthArgs) (*AtomicCheckHealthResult, error) {
	return t.ExecuteHealthCheck(ctx, args)
}

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

	for _, ps := range result.PodStatuses {
		podStatus := kubernetes.DetailedPodStatus{
			Name:      ps.Name,
			Namespace: namespace,
			Status:    ps.Status,
			Ready:     ps.Ready,
		}
		k8sResult.Pods = append(k8sResult.Pods, podStatus)
	}

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
