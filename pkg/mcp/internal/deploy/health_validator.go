package deploy

import (
	"fmt"
	"strings"

	"github.com/Azure/container-kit/pkg/core/kubernetes"
	"github.com/rs/zerolog"
)

// HealthValidator handles analysis and validation of health check results
type HealthValidator struct {
	logger zerolog.Logger
}

// NewHealthValidator creates a new health validator
func NewHealthValidator(logger zerolog.Logger) *HealthValidator {
	return &HealthValidator{
		logger: logger,
	}
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
