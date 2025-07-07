package deploy

import (
	"context"
	"fmt"
	"time"

	"log/slog"

	"github.com/Azure/container-kit/pkg/core/kubernetes"
	"github.com/Azure/container-kit/pkg/mcp/application/core"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
	"github.com/Azure/container-kit/pkg/mcp/domain/types"
	"github.com/Azure/container-kit/pkg/mcp/services"
)

// HealthChecker handles the core health checking operations
type HealthChecker struct {
	pipelineAdapter core.TypedPipelineOperations
	sessionManager  session.UnifiedSessionManager
	sessionStore    services.SessionStore
	sessionState    services.SessionState
	logger          *slog.Logger
}

// NewHealthChecker creates a new health checker using unified session manager
func NewHealthChecker(adapter core.TypedPipelineOperations, sessionManager session.UnifiedSessionManager, logger *slog.Logger) *HealthChecker {
	return &HealthChecker{
		pipelineAdapter: adapter,
		sessionManager:  sessionManager,
		logger:          logger,
	}
}

// NewHealthCheckerWithServices creates a new health checker using focused services
func NewHealthCheckerWithServices(adapter core.TypedPipelineOperations, sessionStore services.SessionStore, sessionState services.SessionState, logger *slog.Logger) *HealthChecker {
	return &HealthChecker{
		pipelineAdapter: adapter,
		sessionStore:    sessionStore,
		sessionState:    sessionState,
		logger:          logger,
	}
}

// PerformHealthCheck performs the actual health check
func (hc *HealthChecker) PerformHealthCheck(ctx context.Context, args AtomicCheckHealthArgs, reporter interface{}) (*AtomicCheckHealthResult, error) {
	startTime := time.Now()

	// Get session
	sessionState, err := hc.sessionManager.GetSession(ctx, args.SessionID)
	if err != nil {
		// Create result with error for session failure
		result := &AtomicCheckHealthResult{
			BaseToolResponse:    types.BaseToolResponse{Success: false, Timestamp: time.Now()},
			BaseAIContextResult: core.NewBaseAIContextResult("health", false, time.Since(startTime)),
			Namespace:           hc.getNamespace(args.Namespace),
			Context:             &HealthContext{},
		}
		result.Success = false

		hc.logger.Error("Failed to get session",
			"error", err,
			"session_id", args.SessionID)
		return result, nil
	}

	session := sessionState.ToCoreSessionState()

	// Build label selector
	labelSelector := hc.buildLabelSelector(args, session)
	namespace := hc.getNamespace(args.Namespace)

	hc.logger.Info("Starting atomic application health check",
		"session_id", session.SessionID,
		"namespace", namespace,
		"label_selector", labelSelector,
		"wait_for_ready", args.WaitForReady)

	// Create base response
	result := &AtomicCheckHealthResult{
		BaseToolResponse:    types.BaseToolResponse{Success: false, Timestamp: time.Now()},
		BaseAIContextResult: core.NewBaseAIContextResult("health", true, 0), // Duration will be set later
		Namespace:           namespace,
		Context:             &HealthContext{},
	}

	// Handle dry-run
	if args.DryRun {
		result.Recommendations = []string{
			"This is a dry-run - actual health check would be performed",
			fmt.Sprintf("Would check health in namespace: %s", namespace),
			fmt.Sprintf("Using label selector: %s", labelSelector),
		}
		return result, nil
	}

	// Validate prerequisites
	if err := hc.validateHealthCheckPrerequisites(result, args); err != nil {
		hc.logger.Error("Health check prerequisites validation failed",
			"error", err,
			"session_id", session.SessionID,
			"namespace", namespace,
			"label_selector", labelSelector)
		result.Success = false
		return result, nil
	}

	// Perform health check using core operations
	// Convert to typed parameters for CheckHealthTyped
	healthParams := core.HealthCheckParams{
		Namespace:   args.Namespace,
		AppName:     args.AppName,
		Resources:   []string{}, // Default empty
		WaitTimeout: 300,        // 5 minutes default
	}
	healthResult, err := hc.pipelineAdapter.CheckHealthTyped(ctx, session.SessionID, healthParams)

	// Convert from typed result to kubernetes.HealthCheckResult
	var kubernetesHealthResult *kubernetes.HealthCheckResult
	if healthResult != nil {
		kubernetesHealthResult = hc.convertTypedHealthCheckResult(healthResult, namespace)
	}

	if err != nil {
		hc.logger.Error("Health check failed",
			"error", err,
			"session_id", session.SessionID,
			"namespace", namespace,
			"label_selector", labelSelector)

		result.Success = false
		result.HealthStatus = "unknown"
		result.Summary = fmt.Sprintf("Health check failed: %s", err.Error())
		result.Recommendations = []string{
			"Check if the application is deployed",
			"Verify label selector matches deployed resources",
			"Check Kubernetes cluster connectivity",
		}
		return result, nil
	}

	// Wait for readiness if requested
	if args.WaitForReady {
		timeout := hc.getWaitTimeout(args.WaitTimeout)
		waitResult := hc.waitForApplicationReady(ctx, session.SessionID, namespace, labelSelector, timeout)
		if waitResult != nil {
			// Merge wait results with health results
			if kubernetesHealthResult == nil {
				kubernetesHealthResult = waitResult
			} else {
				// Update status based on wait results
				if !waitResult.Success {
					kubernetesHealthResult.Success = false
				}
			}
		}
	}

	// Process results
	if kubernetesHealthResult != nil {
		result.Success = kubernetesHealthResult.Success
		result.HealthStatus = hc.determineHealthStatus(kubernetesHealthResult)
		result.OverallScore = hc.calculateHealthScore(kubernetesHealthResult)
		result.Summary = "Health check completed"
		result.CheckedAt = time.Now().UTC().Format(time.RFC3339)

		// Extract pod and service information
		result.PodSummaries = hc.extractPodSummaries(kubernetesHealthResult)
		result.ServiceSummaries = hc.extractServiceSummaries(kubernetesHealthResult)
	} else {
		result.Success = false
		result.HealthStatus = "unknown"
		result.Summary = "No health data available"
	}

	return result, nil
}

// validateHealthCheckPrerequisites validates that health checking can proceed
func (hc *HealthChecker) validateHealthCheckPrerequisites(result *AtomicCheckHealthResult, args AtomicCheckHealthArgs) error {
	// Basic validation - in a real implementation this would check:
	// - Kubernetes connectivity
	// - Namespace existence
	// - RBAC permissions
	// - Resource availability

	if args.Namespace != "" && args.Namespace != "default" && args.Namespace != "kube-system" {
		// Validate namespace exists (simplified)
		hc.logger.Debug("Validating namespace", "namespace", args.Namespace)
	}

	return nil
}

// waitForApplicationReady waits for the application to become ready
func (hc *HealthChecker) waitForApplicationReady(ctx context.Context, sessionID, namespace, labelSelector string, timeout time.Duration) *kubernetes.HealthCheckResult {
	hc.logger.Info("Waiting for application to become ready",
		"session_id", sessionID,
		"namespace", namespace,
		"label_selector", labelSelector,
		"timeout", timeout)

	// Create context with timeout
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Poll for readiness
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-waitCtx.Done():
			hc.logger.Warn("Timeout waiting for application readiness",
				"session_id", sessionID,
				"timeout", timeout)
			return &kubernetes.HealthCheckResult{
				Success:   false,
				Namespace: namespace,
			}

		case <-ticker.C:
			// Check current health status
			// Convert to typed parameters for CheckHealthTyped
			healthParams := core.HealthCheckParams{
				Namespace:   namespace,
				AppName:     "",         // Extract from labelSelector if needed
				Resources:   []string{}, // Default empty
				WaitTimeout: 10,         // 10 seconds for this check
			}
			healthResult, err := hc.pipelineAdapter.CheckHealthTyped(waitCtx, sessionID, healthParams)
			if err != nil {
				hc.logger.Debug("Health check during wait failed", "error", err)
				continue
			}

			if healthResult != nil {
				kubernetesResult := hc.convertTypedHealthCheckResult(healthResult, namespace)
				if kubernetesResult != nil && kubernetesResult.Success {
					hc.logger.Info("Application became ready",
						"session_id", sessionID)
					return kubernetesResult
				}
			}
		}
	}
}

// Helper methods

func (hc *HealthChecker) buildLabelSelector(args AtomicCheckHealthArgs, session *core.SessionState) string {
	if args.LabelSelector != "" {
		return args.LabelSelector
	}

	if args.AppName != "" {
		return fmt.Sprintf("app=%s", args.AppName)
	}

	// Try to build from session state metadata
	if appName, ok := session.Metadata["app_name"].(string); ok && appName != "" {
		return fmt.Sprintf("app=%s", appName)
	}

	// Default fallback
	return "app=myapp"
}

func (hc *HealthChecker) getNamespace(namespace string) string {
	if namespace == "" {
		return "default"
	}
	return namespace
}

func (hc *HealthChecker) getWaitTimeout(timeout int) time.Duration {
	if timeout <= 0 {
		return 300 * time.Second // Default 5 minutes
	}
	return time.Duration(timeout) * time.Second
}

func (hc *HealthChecker) determineHealthStatus(result *kubernetes.HealthCheckResult) string {
	if result.Success {
		return "healthy"
	}

	// Analyze the level of issues to determine if degraded vs unhealthy
	if len(result.Pods) > 0 {
		readyCount := 0
		for _, pod := range result.Pods {
			if pod.Ready {
				readyCount++
			}
		}

		if readyCount > 0 && readyCount < len(result.Pods) {
			return "degraded"
		}
	}

	return "unhealthy"
}

func (hc *HealthChecker) calculateHealthScore(result *kubernetes.HealthCheckResult) int {
	if result.Success {
		return 100
	}

	score := 50 // Base score for partially working system

	if len(result.Pods) > 0 {
		readyCount := 0
		for _, pod := range result.Pods {
			if pod.Ready {
				readyCount++
			}
		}

		// Adjust score based on ready pod ratio
		readyRatio := float64(readyCount) / float64(len(result.Pods))
		score = int(readyRatio * 100)
	}

	return score
}

func (hc *HealthChecker) extractPodSummaries(result *kubernetes.HealthCheckResult) []PodSummary {
	var summaries []PodSummary

	for _, pod := range result.Pods {
		summary := PodSummary{
			Name:   pod.Name,
			Status: pod.Status,
			Ready:  fmt.Sprintf("%t", pod.Ready),
			Node:   pod.Node,
			Labels: make(map[string]string), // Labels would need to be extracted from Kubernetes metadata
		}
		summaries = append(summaries, summary)
	}

	return summaries
}

func (hc *HealthChecker) extractServiceSummaries(result *kubernetes.HealthCheckResult) []ServiceSummary {
	var summaries []ServiceSummary

	for _, service := range result.Services {
		summary := ServiceSummary{
			Name:      service.Name,
			Type:      service.Type,
			ClusterIP: service.ClusterIP,
			Endpoints: len(service.Endpoints), // Convert endpoints slice length to int
		}
		summaries = append(summaries, summary)
	}

	return summaries
}

// convertTypedHealthCheckResult converts a typed HealthCheckResult to kubernetes.HealthCheckResult
func (hc *HealthChecker) convertTypedHealthCheckResult(result *core.HealthCheckResult, namespace string) *kubernetes.HealthCheckResult {
	if result == nil {
		return nil
	}

	healthResult := &kubernetes.HealthCheckResult{
		Namespace: namespace,
		Success:   result.OverallHealth == "healthy",
	}

	// For now, we'll create a basic summary based on available information
	// In a real implementation, the core.HealthCheckResult would contain more detailed pod/service information
	healthResult.Summary = kubernetes.HealthSummary{
		HealthyRatio: 1.0, // Default to healthy if overall health is "healthy"
	}

	if result.OverallHealth != "healthy" {
		healthResult.Summary.HealthyRatio = 0.0
	}

	return healthResult
}
