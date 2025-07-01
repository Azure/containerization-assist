package deploy

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/core/kubernetes"
	"github.com/Azure/container-kit/pkg/mcp/validation/core"
	"github.com/Azure/container-kit/pkg/mcp/validation/validators"
	"github.com/rs/zerolog"
)

// UnifiedHealthValidator wraps the new health validator with the old interface
type UnifiedHealthValidator struct {
	logger          zerolog.Logger
	healthValidator *validators.HealthValidator
}

// NewUnifiedHealthValidator creates a new unified health validator
func NewUnifiedHealthValidator(logger zerolog.Logger) *HealthValidator {
	// Return the original interface to maintain compatibility
	return &HealthValidator{
		logger: logger,
	}
}

// AnalyzeApplicationHealth performs comprehensive analysis of application health using unified validation
func (hv *HealthValidator) AnalyzeApplicationHealthUnified(result *AtomicCheckHealthResult, args AtomicCheckHealthArgs, healthResult *kubernetes.HealthCheckResult) *core.ValidationResult {
	// Create unified validator
	unifiedValidator := validators.NewHealthValidator()

	// Convert Kubernetes health result to validation format
	healthData := hv.convertToHealthData(healthResult, args)

	// Perform validation
	ctx := context.Background()
	options := core.NewValidationOptions()
	validationResult := unifiedValidator.Validate(ctx, healthData, options)

	// Convert validation result back to original format
	hv.applyValidationResult(result, validationResult, healthResult)

	// Still perform original analysis for additional context
	hv.AnalyzeApplicationHealth(result, args, healthResult)

	return validationResult
}

// convertToHealthData converts Kubernetes health result to validator format
func (hv *HealthValidator) convertToHealthData(healthResult *kubernetes.HealthCheckResult, args AtomicCheckHealthArgs) validators.HealthCheckData {
	data := validators.HealthCheckData{
		Namespace: args.Namespace,
		AppName:   args.AppName,
		CheckedAt: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	// Convert pods
	for _, pod := range healthResult.Pods {
		podHealth := validators.PodHealth{
			Name:   pod.Name,
			Status: pod.Status,
			Ready:  pod.Ready,
			Phase:  pod.Phase,
			Node:   pod.Node,
			Labels: make(map[string]string), // TODO: Extract from actual pod if available
		}

		// Parse age string to duration if possible
		if pod.Age != "" {
			// For now, just set a default age - in real implementation, parse the age string
			podHealth.Age = 5 * time.Minute // Default placeholder
		}

		// Convert containers
		for _, container := range pod.Containers {
			containerHealth := validators.ContainerHealth{
				Name:         container.Name,
				Ready:        container.Ready,
				RestartCount: container.RestartCount,
				State:        container.State,
				LastStart:    time.Now(), // TODO: Extract actual start time if available
			}

			podHealth.Containers = append(podHealth.Containers, containerHealth)
		}

		// Convert conditions
		for _, condition := range pod.Conditions {
			podCondition := validators.PodCondition{
				Type:   string(condition.Type),
				Status: string(condition.Status),
				Reason: condition.Reason,
			}
			podHealth.Conditions = append(podHealth.Conditions, podCondition)
		}

		data.Pods = append(data.Pods, podHealth)
	}

	// Convert services
	for _, service := range healthResult.Services {
		serviceHealth := validators.ServiceHealth{
			Name:      service.Name,
			Type:      service.Type,
			ClusterIP: service.ClusterIP,
			Endpoints: len(service.Endpoints),
		}

		// Convert ports
		for _, port := range service.Ports {
			serviceHealth.Ports = append(serviceHealth.Ports, fmt.Sprintf("%s:%d", port.Protocol, port.Port))
		}

		data.Services = append(data.Services, serviceHealth)
	}

	// Determine overall health status
	data.HealthStatus = hv.determineHealthStatus(healthResult)

	return data
}

// applyValidationResult applies unified validation results to the original result structure
func (hv *HealthValidator) applyValidationResult(result *AtomicCheckHealthResult, validationResult *core.ValidationResult, healthResult *kubernetes.HealthCheckResult) {
	// Update validation status
	if !validationResult.Valid {
		result.Success = false
	}

	// Convert errors to issues
	for _, err := range validationResult.Errors {
		issue := hv.convertErrorToIssue(err)

		// Categorize by field
		if err.Field != "" && containsString(err.Field, "pods") {
			result.PodIssues = append(result.PodIssues, issue)
		} else if err.Field != "" && containsString(err.Field, "containers") {
			containerIssue := ContainerIssue{
				Issue:       issue.Issue,
				Severity:    issue.Severity,
				Category:    issue.Category,
				Description: issue.Description,
				Suggestion:  issue.Suggestion,
			}
			result.ContainerIssues = append(result.ContainerIssues, containerIssue)
		}
	}

	// Add warnings to recommendations
	for _, warning := range validationResult.Warnings {
		if warning.Message != "" {
			result.Recommendations = append(result.Recommendations, fmt.Sprintf("⚠️ %s", warning.Message))
		}
	}

	// Apply health metrics
	if metrics, ok := validationResult.Metadata.Context["health_metrics"].(map[string]interface{}); ok {
		if result.AnalysisDetails == nil {
			result.AnalysisDetails = make(map[string]interface{})
		}
		result.AnalysisDetails["validation_metrics"] = metrics
	}

	// Set risk level
	if validationResult.RiskLevel != "" {
		result.AnalysisDetails["risk_level"] = validationResult.RiskLevel
	}
}

// convertErrorToIssue converts a validation error to a pod issue
func (hv *HealthValidator) convertErrorToIssue(err *core.ValidationError) PodIssue {
	issue := PodIssue{
		Issue:       err.Code,
		Description: err.Message,
		Severity:    string(err.Severity),
		Category:    hv.determineIssueCategory(err),
	}

	// Extract pod name from field if available
	if err.Field != "" {
		// Parse pod name from field like "pods[0]" or similar
		issue.PodName = hv.extractPodNameFromField(err.Field)
	}

	// Add suggestions
	if len(err.Suggestions) > 0 {
		issue.Suggestion = err.Suggestions[0]
	}

	return issue
}

// determineIssueCategory determines issue category from error
func (hv *HealthValidator) determineIssueCategory(err *core.ValidationError) string {
	switch err.Code {
	case "POD_PENDING_TOO_LONG", "POD_SCHEDULING_FAILED":
		return "resource"
	case "CONTAINER_CRASH_LOOP", "HIGH_CONTAINER_RESTARTS", "NON_ZERO_EXIT":
		return "stability"
	case "IMAGE_PULL_ERROR":
		return "registry"
	case "NO_SERVICE_ENDPOINTS", "SERVICE_NO_ENDPOINTS":
		return "network"
	default:
		return "configuration"
	}
}

// extractPodNameFromField extracts pod name from field path
func (hv *HealthValidator) extractPodNameFromField(field string) string {
	// This is a simplified extraction - in practice would need more robust parsing
	if containsString(field, "pods[") {
		return fmt.Sprintf("pod-%s", field)
	}
	return ""
}

// determineHealthStatus determines overall health status from Kubernetes result
func (hv *HealthValidator) determineHealthStatus(healthResult *kubernetes.HealthCheckResult) string {
	if len(healthResult.Pods) == 0 {
		return "unknown"
	}

	readyCount := 0
	for _, pod := range healthResult.Pods {
		if pod.Ready {
			readyCount++
		}
	}

	readinessRatio := float64(readyCount) / float64(len(healthResult.Pods))

	if readinessRatio >= 1.0 {
		return "healthy"
	} else if readinessRatio >= 0.7 {
		return "degraded"
	} else if readinessRatio > 0.0 {
		return "unhealthy"
	}

	return "unknown"
}

// containsString checks if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr ||
		len(s) > len(substr) && containsString(s[1:], substr)
}

// ValidateHealthCheck validates health check configuration using unified validation
func ValidateHealthCheck(config map[string]interface{}) *core.ValidationResult {
	validator := validators.NewHealthValidator()
	ctx := context.Background()
	options := core.NewValidationOptions()

	return validator.Validate(ctx, config, options)
}

// ValidateHealthThresholds validates health check thresholds
func ValidateHealthThresholds(thresholds validators.HealthThresholds) *core.ValidationResult {
	result := &core.ValidationResult{
		Valid:    true,
		Errors:   make([]*core.ValidationError, 0),
		Warnings: make([]*core.ValidationWarning, 0),
		Metadata: core.ValidationMetadata{
			ValidatedAt:      time.Now(),
			ValidatorName:    "health-threshold",
			ValidatorVersion: "1.0.0",
			Context:          make(map[string]interface{}),
		},
	}

	// Validate thresholds
	if thresholds.MinHealthyRatio < 0 || thresholds.MinHealthyRatio > 1 {
		result.AddFieldError("min_healthy_ratio", "Must be between 0 and 1")
	}

	if thresholds.MaxRestartCount < 0 {
		result.AddFieldError("max_restart_count", "Must be non-negative")
	}

	if thresholds.CriticalRestarts < thresholds.MaxRestartCount {
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "THRESHOLD_MISMATCH",
				Message:  "Critical restart threshold is less than max restart count",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityLow,
			},
		})
	}

	if thresholds.MaxPendingTime < 0 {
		result.AddFieldError("max_pending_time", "Must be non-negative duration")
	}

	return result
}
