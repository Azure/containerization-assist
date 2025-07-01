package deploy

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/core/kubernetes"
	"github.com/Azure/container-kit/pkg/mcp/core"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/core"
	sessiontypes "github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
)

// HealthChecker handles the core health checking operations
type HealthChecker struct {
	pipelineAdapter core.PipelineOperations
	sessionManager  core.ToolSessionManager
	logger          zerolog.Logger
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(adapter core.PipelineOperations, sessionManager core.ToolSessionManager, logger zerolog.Logger) *HealthChecker {
	return &HealthChecker{
		pipelineAdapter: adapter,
		sessionManager:  sessionManager,
		logger:          logger,
	}
}

// PerformHealthCheck performs the actual health check
func (hc *HealthChecker) PerformHealthCheck(ctx context.Context, args AtomicCheckHealthArgs, reporter interface{}) (*AtomicCheckHealthResult, error) {
	startTime := time.Now()

	// Get session
	sessionInterface, err := hc.sessionManager.GetSession(args.SessionID)
	if err != nil {
		// Create result with error for session failure
		result := &AtomicCheckHealthResult{
			BaseToolResponse:    types.NewBaseResponse("atomic_check_health", args.SessionID, args.DryRun),
			BaseAIContextResult: mcptypes.NewBaseAIContextResult("health", false, time.Since(startTime)),
			Namespace:           hc.getNamespace(args.Namespace),
			Context:             &HealthContext{},
		}
		result.Success = false

		hc.logger.Error().Err(err).
			Str("session_id", args.SessionID).
			Msg("Failed to get session")
		return result, nil
	}
	sessionState := sessionInterface.(*sessiontypes.SessionState)
	session := sessionState.ToCoreSessionState()

	// Build label selector
	labelSelector := hc.buildLabelSelector(args, session)
	namespace := hc.getNamespace(args.Namespace)

	hc.logger.Info().
		Str("session_id", session.SessionID).
		Str("namespace", namespace).
		Str("label_selector", labelSelector).
		Bool("wait_for_ready", args.WaitForReady).
		Msg("Starting atomic application health check")

	// Create base response
	result := &AtomicCheckHealthResult{
		BaseToolResponse:    types.NewBaseResponse("atomic_check_health", session.SessionID, args.DryRun),
		BaseAIContextResult: mcptypes.NewBaseAIContextResult("health", true, 0), // Duration will be set later
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
		hc.logger.Error().Err(err).
			Str("session_id", session.SessionID).
			Str("namespace", namespace).
			Str("label_selector", labelSelector).
			Msg("Health check prerequisites validation failed")
		result.Success = false
		return result, nil
	}

	// Perform health check using core operations
	healthArgs := map[string]interface{}{
		"namespace":     namespace,
		"labelSelector": labelSelector,
		"timeout":       30 * time.Second,
	}
	healthResult, err := hc.pipelineAdapter.CheckHealth(ctx, session.SessionID, healthArgs)

	// Convert from interface{} to kubernetes.HealthCheckResult
	var kubernetesHealthResult *kubernetes.HealthCheckResult
	if healthResult != nil {
		kubernetesHealthResult = hc.convertHealthCheckResult(healthResult.(map[string]interface{}), namespace)
	}

	if err != nil {
		hc.logger.Error().Err(err).
			Str("session_id", session.SessionID).
			Str("namespace", namespace).
			Str("label_selector", labelSelector).
			Msg("Health check failed")

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
		hc.logger.Debug().Str("namespace", args.Namespace).Msg("Validating namespace")
	}

	return nil
}

// waitForApplicationReady waits for the application to become ready
func (hc *HealthChecker) waitForApplicationReady(ctx context.Context, sessionID, namespace, labelSelector string, timeout time.Duration) *kubernetes.HealthCheckResult {
	hc.logger.Info().
		Str("session_id", sessionID).
		Str("namespace", namespace).
		Str("label_selector", labelSelector).
		Dur("timeout", timeout).
		Msg("Waiting for application to become ready")

	// Create context with timeout
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Poll for readiness
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-waitCtx.Done():
			hc.logger.Warn().
				Str("session_id", sessionID).
				Dur("timeout", timeout).
				Msg("Timeout waiting for application readiness")
			return &kubernetes.HealthCheckResult{
				Success:   false,
				Namespace: namespace,
			}

		case <-ticker.C:
			// Check current health status
			healthArgs := map[string]interface{}{
				"namespace":     namespace,
				"labelSelector": labelSelector,
				"timeout":       10 * time.Second,
			}

			healthResult, err := hc.pipelineAdapter.CheckHealth(waitCtx, sessionID, healthArgs)
			if err != nil {
				hc.logger.Debug().Err(err).Msg("Health check during wait failed")
				continue
			}

			if healthResult != nil {
				kubernetesResult := hc.convertHealthCheckResult(healthResult.(map[string]interface{}), namespace)
				if kubernetesResult != nil && kubernetesResult.Success {
					hc.logger.Info().
						Str("session_id", sessionID).
						Msg("Application became ready")
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

func (hc *HealthChecker) convertHealthCheckResult(result map[string]interface{}, namespace string) *kubernetes.HealthCheckResult {
	// Convert the map[string]interface{} result to kubernetes.HealthCheckResult
	// This is a simplified conversion - in practice this would be more comprehensive

	healthResult := &kubernetes.HealthCheckResult{
		Namespace: namespace,
	}

	if overall, ok := result["success"].(bool); ok {
		healthResult.Success = overall
	}

	if summaryData, ok := result["summary"].(map[string]interface{}); ok {
		// Convert summary map to HealthSummary struct
		if totalPods, ok := summaryData["total_pods"].(float64); ok {
			healthResult.Summary.TotalPods = int(totalPods)
		}
		if readyPods, ok := summaryData["ready_pods"].(float64); ok {
			healthResult.Summary.ReadyPods = int(readyPods)
		}
		if failedPods, ok := summaryData["failed_pods"].(float64); ok {
			healthResult.Summary.FailedPods = int(failedPods)
		}
		if healthyRatio, ok := summaryData["healthy_ratio"].(float64); ok {
			healthResult.Summary.HealthyRatio = healthyRatio
		}
	}

	// Convert pod statuses
	if pods, ok := result["pod_statuses"].([]interface{}); ok {
		for _, podInterface := range pods {
			if podMap, ok := podInterface.(map[string]interface{}); ok {
				pod := kubernetes.DetailedPodStatus{
					Name:   getStringFromMap(podMap, "name", ""),
					Phase:  getStringFromMap(podMap, "phase", "Unknown"),
					Ready:  getBoolFromMap(podMap, "ready", false),
					Node:   getStringFromMap(podMap, "node_name", ""),
					Status: getStringFromMap(podMap, "phase", "Unknown"),
				}
				healthResult.Pods = append(healthResult.Pods, pod)
			}
		}
	}

	// Convert service statuses
	if services, ok := result["service_statuses"].([]interface{}); ok {
		for _, serviceInterface := range services {
			if serviceMap, ok := serviceInterface.(map[string]interface{}); ok {
				service := kubernetes.DetailedServiceStatus{
					Name:      getStringFromMap(serviceMap, "name", ""),
					Type:      getStringFromMap(serviceMap, "type", "ClusterIP"),
					ClusterIP: getStringFromMap(serviceMap, "cluster_ip", ""),
				}
				healthResult.Services = append(healthResult.Services, service)
			}
		}
	}

	return healthResult
}

// Utility functions
func getStringFromMap(m map[string]interface{}, key, defaultValue string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return defaultValue
}

func getBoolFromMap(m map[string]interface{}, key string, defaultValue bool) bool {
	if val, ok := m[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return defaultValue
}
