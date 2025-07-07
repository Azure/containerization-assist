package deploy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"log/slog"

	"github.com/Azure/container-kit/pkg/common/validation-core/core"
	"github.com/Azure/container-kit/pkg/common/validation-core/validators"
	"github.com/Azure/container-kit/pkg/core/kubernetes"
)

// HealthValidator handles analysis and validation of health check results using unified validation framework
type HealthValidator struct {
	logger          *slog.Logger
	healthValidator core.Validator
}

// UnifiedHealthValidator provides a unified validation interface
type UnifiedHealthValidator struct {
	impl *HealthValidator
}

// NewHealthValidator creates a new health validator with unified validation support
func NewHealthValidator(logger *slog.Logger) *HealthValidator {
	return &HealthValidator{
		logger:          logger.With("component", "unified_health_validator"),
		healthValidator: validators.NewHealthValidator(),
	}
}

// NewUnifiedHealthValidator creates a new unified health validator
func NewUnifiedHealthValidator(logger *slog.Logger) *UnifiedHealthValidator {
	return &UnifiedHealthValidator{
		impl: NewHealthValidator(logger),
	}
}

// ValidateHealthUnified performs health validation using unified validation framework
func (hv *HealthValidator) ValidateHealthUnified(ctx context.Context, healthData map[string]interface{}) (*core.DeployResult, error) {
	hv.logger.Info("Starting unified health validation")

	// Create health validation data with timestamp
	validationData := map[string]interface{}{
		"health_data": healthData,
		"timestamp":   time.Now(),
		"type":        "health_check",
	}

	// Use unified health validator
	options := core.NewValidationOptions().WithStrictMode(true)
	nonGenericResult := hv.healthValidator.Validate(ctx, validationData, options)

	// Convert to DeployResult
	result := core.NewDeployResult("unified_health_validator", "1.0.0")
	result.Valid = nonGenericResult.Valid
	result.Errors = nonGenericResult.Errors
	result.Warnings = nonGenericResult.Warnings
	result.Suggestions = nonGenericResult.Suggestions
	result.Duration = nonGenericResult.Duration
	result.Metadata = nonGenericResult.Metadata

	// Add health-specific validation
	if pods, ok := healthData["pods"].([]interface{}); ok {
		healthy, total := calculateHealthMetrics(pods)
		if healthy < total {
			result.AddWarning(core.NewWarning(
				"HEALTH_DEGRADED",
				fmt.Sprintf("Health degraded: %d/%d pods ready", healthy, total),
			))
		}
	}

	// Add deployment-specific metadata
	resource := core.KubernetesResource{
		APIVersion: extractStringFromData(healthData, "api_version", "v1"),
		Kind:       extractStringFromData(healthData, "kind", "Pod"),
		Name:       extractStringFromData(healthData, "name", "unknown"),
		Namespace:  extractStringFromData(healthData, "namespace", "default"),
	}

	result.Data = core.DeployValidationData{
		Namespace: extractStringFromData(healthData, "namespace", "default"),
		Resources: []core.KubernetesResource{resource},
		ClusterInfo: map[string]interface{}{
			"validation_type": "health_check",
			"health_data":     healthData,
		},
	}

	hv.logger.Info("Unified health validation completed",
		"valid", result.Valid,
		"errors", len(result.Errors),
		"warnings", len(result.Warnings))

	return result, nil
}

// AnalyzeApplicationHealth performs comprehensive analysis of application health
func (hv *HealthValidator) AnalyzeApplicationHealth(result *AtomicCheckHealthResult, args AtomicCheckHealthArgs, healthResult *kubernetes.HealthCheckResult) {
	if healthResult == nil {
		return
	}

	// Initialize context if needed
	if result.Context == nil {
		result.Context = &HealthContext{}
	}

	ctx := result.Context

	// Calculate overall health metrics
	readinessRatio := 0.0
	if len(healthResult.Pods) > 0 {
		readyCount := 0
		for _, pod := range healthResult.Pods {
			if pod.Ready {
				readyCount++
			}
		}
		readinessRatio = float64(readyCount) / float64(len(healthResult.Pods))
	}

	// Set context information
	ctx.Namespace = result.Namespace
	ctx.AppName = args.AppName

	// Determine overall status
	if readinessRatio >= 1.0 {
		result.HealthStatus = "healthy"
	} else if readinessRatio >= 0.7 {
		result.HealthStatus = "degraded"
	} else if readinessRatio > 0.0 {
		result.HealthStatus = "unhealthy"
	} else {
		result.HealthStatus = "unknown"
	}

	// Analyze individual pods for issues
	hv.analyzePodIssues(result, healthResult)

	// Analyze restart patterns
	hv.analyzeRestartPatterns(result, healthResult)

	// Generate recommendations
	hv.generateRecommendations(result, args, healthResult)

	// Generate troubleshooting context
	hv.generateTroubleshootingContext(result, healthResult)
}

// analyzePodIssues analyzes individual pod health issues
func (hv *HealthValidator) analyzePodIssues(result *AtomicCheckHealthResult, healthResult *kubernetes.HealthCheckResult) {
	var totalRestarts int
	var podsWithRestarts int

	for _, pod := range healthResult.Pods {
		// Analyze pod-level issues
		if !pod.Ready {
			issue := PodIssue{
				PodName:     pod.Name,
				Issue:       "Pod is not ready",
				Description: fmt.Sprintf("Pod status: %s", pod.Status),
				Severity:    hv.determinePodIssueSeverity(pod.Status),
				Category:    hv.determinePodIssueCategory(pod.Status),
				Suggestion:  hv.generatePodSuggestion(pod.Status),
			}

			result.PodIssues = append(result.PodIssues, issue)
		}

		// Analyze container-level issues if detailed analysis is enabled
		// This would require more detailed pod information from Kubernetes API
		if len(pod.Containers) > 0 {
			for _, container := range pod.Containers {
				if container.RestartCount > 0 {
					totalRestarts += container.RestartCount
					podsWithRestarts++

					containerIssue := ContainerIssue{
						PodName:       pod.Name,
						ContainerName: container.Name,
						Issue:         "Container restarts detected",
						Description:   fmt.Sprintf("Container has restarted %d times", container.RestartCount),
						Severity:      hv.determineRestartSeverity(container.RestartCount),
						Category:      "stability",
						Suggestion:    hv.generateRestartSuggestion(container.RestartCount),
					}

					result.ContainerIssues = append(result.ContainerIssues, containerIssue)
				}
			}
		}
	}

	// Update restart analysis
	if result.RestartAnalysis == nil {
		result.RestartAnalysis = &RestartAnalysis{}
	}
	result.RestartAnalysis.TotalRestarts = totalRestarts
	result.RestartAnalysis.RestartPattern = hv.determineRestartPattern(totalRestarts, podsWithRestarts, len(healthResult.Pods))
}

// analyzeRestartPatterns analyzes pod restart patterns
func (hv *HealthValidator) analyzeRestartPatterns(result *AtomicCheckHealthResult, healthResult *kubernetes.HealthCheckResult) {
	if result.RestartAnalysis == nil {
		result.RestartAnalysis = &RestartAnalysis{}
	}

	restartAnalysis := result.RestartAnalysis
	restartReasons := make(map[string]int)
	var affectedPods []string

	for _, pod := range healthResult.Pods {
		if len(pod.Containers) > 0 {
			for _, container := range pod.Containers {
				if container.RestartCount > 0 {
					affectedPods = append(affectedPods, pod.Name)

					// Categorize restart reasons (this would be more detailed with real Kubernetes data)
					if container.RestartCount > 10 {
						restartReasons["frequent_restarts"]++
					} else if container.RestartCount > 3 {
						restartReasons["moderate_restarts"]++
					} else {
						restartReasons["occasional_restarts"]++
					}
				}
			}
		}
	}

	restartAnalysis.RestartReasons = restartReasons
	restartAnalysis.AffectedPods = affectedPods
	restartAnalysis.RecommendedAction = hv.generateRestartRecommendation(restartAnalysis.TotalRestarts, len(affectedPods))
}

// generateRecommendations generates actionable recommendations
func (hv *HealthValidator) generateRecommendations(result *AtomicCheckHealthResult, args AtomicCheckHealthArgs, healthResult *kubernetes.HealthCheckResult) {
	var recommendations []string

	// Health status based recommendations
	switch result.HealthStatus {
	case "healthy":
		recommendations = append(recommendations, "✅ Application is healthy and all pods are ready")
		recommendations = append(recommendations, "Consider setting up monitoring and alerting for continued health")
	case "degraded":
		recommendations = append(recommendations, "⚠️ Application is partially healthy - some pods are not ready")
		recommendations = append(recommendations, "Investigate non-ready pods and their logs")
	case "unhealthy":
		recommendations = append(recommendations, "🔴 Application is unhealthy - significant issues detected")
		recommendations = append(recommendations, "Immediate attention required - check pod logs and events")
	case "unknown":
		recommendations = append(recommendations, "❓ Unable to determine application health")
		recommendations = append(recommendations, "Verify deployment exists and label selector is correct")
	}

	// Issue-specific recommendations
	if len(result.PodIssues) > 0 {
		recommendations = append(recommendations, "🔍 Pod issues detected:")
		for _, issue := range result.PodIssues {
			if issue.Suggestion != "" {
				recommendations = append(recommendations, fmt.Sprintf("  • %s: %s", issue.PodName, issue.Suggestion))
			}
		}
	}

	// Restart-specific recommendations
	if result.RestartAnalysis != nil && result.RestartAnalysis.TotalRestarts > 0 {
		recommendations = append(recommendations, "🔄 Container restarts detected:")
		recommendations = append(recommendations, result.RestartAnalysis.RecommendedAction)
	}

	// Resource-specific recommendations
	if len(healthResult.Pods) == 0 {
		recommendations = append(recommendations, "📋 No pods found - check if application is deployed")
		recommendations = append(recommendations, "Verify namespace and label selector are correct")
	}

	result.Recommendations = recommendations
}

// generateTroubleshootingContext generates context for troubleshooting
func (hv *HealthValidator) generateTroubleshootingContext(result *AtomicCheckHealthResult, healthResult *kubernetes.HealthCheckResult) {
	context := make(map[string]interface{})

	// Pod distribution analysis
	podsByPhase := make(map[string]int)
	nodeDistribution := make(map[string]int)

	for _, pod := range healthResult.Pods {
		podsByPhase[pod.Phase]++
		if pod.Node != "" {
			nodeDistribution[pod.Node]++
		}
	}

	context["pod_phase_distribution"] = podsByPhase
	context["node_distribution"] = nodeDistribution
	context["total_pods"] = len(healthResult.Pods)
	context["total_services"] = len(healthResult.Services)

	// Issue analysis
	if len(result.PodIssues) > 0 {
		issuesByCategory := make(map[string]int)
		issuesBySeverity := make(map[string]int)

		for _, issue := range result.PodIssues {
			issuesByCategory[issue.Category]++
			issuesBySeverity[issue.Severity]++
		}

		context["issues_by_category"] = issuesByCategory
		context["issues_by_severity"] = issuesBySeverity
	}

	result.AnalysisDetails = context
}

// Helper methods

func (hv *HealthValidator) determinePodIssueSeverity(status string) string {
	switch strings.ToLower(status) {
	case "failed", "error", "crashloopbackoff":
		return "critical"
	case "pending":
		return "major"
	case "running":
		return "minor"
	default:
		return "warning"
	}
}

func (hv *HealthValidator) determinePodIssueCategory(status string) string {
	switch strings.ToLower(status) {
	case "pending":
		return "resource"
	case "failed", "error":
		return "configuration"
	case "crashloopbackoff":
		return "stability"
	case "imagepullbackoff":
		return "registry"
	default:
		return "general"
	}
}

func (hv *HealthValidator) generatePodSuggestion(status string) string {
	switch strings.ToLower(status) {
	case "pending":
		return "Check node resources and scheduling constraints"
	case "failed", "error":
		return "Check container logs and configuration"
	case "crashloopbackoff":
		return "Application is crashing repeatedly - check logs and startup configuration"
	case "imagepullbackoff":
		return "Cannot pull container image - check image name and registry access"
	default:
		return "Check pod events and conditions for more details"
	}
}

func (hv *HealthValidator) determineRestartSeverity(restartCount int) string {
	if restartCount > 10 {
		return "critical"
	} else if restartCount > 5 {
		return "major"
	} else if restartCount > 1 {
		return "minor"
	}
	return "warning"
}

func (hv *HealthValidator) generateRestartSuggestion(restartCount int) string {
	if restartCount > 10 {
		return "Frequent restarts indicate a serious stability issue - investigate immediately"
	} else if restartCount > 5 {
		return "Multiple restarts detected - check for resource constraints or configuration issues"
	}
	return "Some restarts detected - monitor for patterns and check logs"
}

func (hv *HealthValidator) determineRestartPattern(totalRestarts, podsWithRestarts, totalPods int) string {
	if totalRestarts == 0 {
		return "none"
	}

	restartRatio := float64(podsWithRestarts) / float64(totalPods)
	avgRestartsPerPod := float64(totalRestarts) / float64(podsWithRestarts)

	if restartRatio > 0.8 && avgRestartsPerPod > 5 {
		return "continuous"
	} else if restartRatio > 0.5 || avgRestartsPerPod > 3 {
		return "frequent"
	} else if restartRatio > 0.2 || avgRestartsPerPod > 1 {
		return "occasional"
	}

	return "minimal"
}

func (hv *HealthValidator) generateRestartRecommendation(totalRestarts, affectedPods int) string {
	if totalRestarts > 20 {
		return "Critical: High restart count indicates serious stability issues - investigate application configuration and resource constraints"
	} else if totalRestarts > 10 {
		return "Warning: Moderate restart activity detected - monitor application logs and resource usage"
	} else if totalRestarts > 3 {
		return "Info: Some restart activity detected - verify application startup behavior"
	}
	return "Minimal restart activity - application appears stable"
}

// ============================================================================
// UNIFIED VALIDATION INTERFACE METHODS
// ============================================================================

// Validate implements the GenericValidator interface
func (uhv *UnifiedHealthValidator) Validate(ctx context.Context, data core.DeployValidationData, options *core.ValidationOptions) *core.DeployResult {
	// Convert DeployValidationData to the format expected by ValidateHealthUnified
	healthData := map[string]interface{}{
		"namespace":       data.Namespace,
		"resources":       data.Resources,
		"validation_type": getValidationType(data.ClusterInfo),
		"manifest_type":   getManifestType(data.ClusterInfo),
	}

	result, err := uhv.impl.ValidateHealthUnified(ctx, healthData)
	if err != nil {
		if result == nil {
			result = core.NewDeployResult("unified_health_validator", "1.0.0")
		}
		result.AddError(core.NewDeployError("VALIDATION_ERROR", err.Error(), "validation"))
	}
	return result
}

// GetName returns the validator name
func (uhv *UnifiedHealthValidator) GetName() string {
	return "unified_health_validator"
}

// GetVersion returns the validator version
func (uhv *UnifiedHealthValidator) GetVersion() string {
	return "1.0.0"
}

// GetSupportedTypes returns the data types this validator can handle
func (uhv *UnifiedHealthValidator) GetSupportedTypes() []string {
	return []string{"DeployValidationData", "map[string]interface{}", "HealthCheckResult"}
}

// ValidateApplicationHealth performs comprehensive application health validation
func (uhv *UnifiedHealthValidator) ValidateApplicationHealth(ctx context.Context, healthResult *kubernetes.HealthCheckResult, metadata map[string]interface{}) (*core.DeployResult, error) {
	// Convert health result to validation data
	healthData := map[string]interface{}{
		"pods":     convertPodsToInterface(healthResult.Pods),
		"services": convertServicesToInterface(healthResult.Services),
		"metadata": metadata,
	}

	result, err := uhv.impl.ValidateHealthUnified(ctx, healthData)
	if err != nil && result == nil {
		result = core.NewDeployResult("unified_health_validator", "1.0.0")
		result.AddError(core.NewDeployError("VALIDATION_ERROR", err.Error(), "validation"))
	}

	// Add health-specific analysis
	if healthResult != nil {
		healthy, total := calculateHealthMetricsFromResult(healthResult)
		// Add health data to cluster info instead of resources
		if result.Data.ClusterInfo == nil {
			result.Data.ClusterInfo = make(map[string]interface{})
		}
		result.Data.ClusterInfo["health_status"] = fmt.Sprintf("%d/%d pods ready", healthy, total)
		result.Data.ClusterInfo["total_pods"] = total
		result.Data.ClusterInfo["healthy_pods"] = healthy
	}

	return result, nil
}

// Helper functions for unified validation

func calculateHealthMetrics(pods []interface{}) (healthy, total int) {
	total = len(pods)
	for _, pod := range pods {
		if podMap, ok := pod.(map[string]interface{}); ok {
			if ready, ok := podMap["ready"].(bool); ok && ready {
				healthy++
			}
		}
	}
	return healthy, total
}

func calculateHealthMetricsFromResult(healthResult *kubernetes.HealthCheckResult) (healthy, total int) {
	total = len(healthResult.Pods)
	for _, pod := range healthResult.Pods {
		if pod.Ready {
			healthy++
		}
	}
	return healthy, total
}

func extractStringFromData(data map[string]interface{}, key, defaultValue string) string {
	if value, ok := data[key].(string); ok {
		return value
	}
	return defaultValue
}

func convertPodsToInterface(pods []kubernetes.DetailedPodStatus) []interface{} {
	result := make([]interface{}, len(pods))
	for i, pod := range pods {
		result[i] = map[string]interface{}{
			"name":     pod.Name,
			"ready":    pod.Ready,
			"status":   pod.Status,
			"phase":    pod.Phase,
			"node":     pod.Node,
			"restarts": pod.Restarts,
		}
	}
	return result
}

func convertServicesToInterface(services []kubernetes.DetailedServiceStatus) []interface{} {
	result := make([]interface{}, len(services))
	for i, service := range services {
		result[i] = map[string]interface{}{
			"name":       service.Name,
			"namespace":  service.Namespace,
			"type":       service.Type,
			"cluster_ip": service.ClusterIP,
		}
	}
	return result
}

// Helper functions for data structure conversion
func getValidationType(clusterInfo map[string]interface{}) string {
	if vt, ok := clusterInfo["validation_type"].(string); ok {
		return vt
	}
	return "unknown"
}

func getManifestType(clusterInfo map[string]interface{}) string {
	if mt, ok := clusterInfo["manifest_type"].(string); ok {
		return mt
	}
	return "kubernetes"
}

// Migration helpers for backward compatibility

// MigrateHealthValidatorToUnified provides a drop-in replacement for legacy HealthValidator
func MigrateHealthValidatorToUnified(logger *slog.Logger) *UnifiedHealthValidator {
	return NewUnifiedHealthValidator(logger)
}
