package validators

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/common/validation-core/core"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// HealthValidator validates health check results and pod/container states
type HealthValidator struct {
	*BaseValidatorImpl
	thresholds HealthThresholds
}

// HealthThresholds defines thresholds for health validation
type HealthThresholds struct {
	MinHealthyRatio  float64       // Minimum ratio of healthy pods (default 0.8)
	MaxRestartCount  int           // Maximum allowed restarts per container (default 5)
	MaxPendingTime   time.Duration // Maximum time a pod can be pending (default 5m)
	CriticalRestarts int           // Restart count considered critical (default 10)
}

// NewHealthValidator creates a new health validator
func NewHealthValidator() *HealthValidator {
	return &HealthValidator{
		BaseValidatorImpl: NewBaseValidator("health", "1.0.0", []string{"health", "pod", "container", "kubernetes"}),
		thresholds: HealthThresholds{
			MinHealthyRatio:  0.8,
			MaxRestartCount:  5,
			MaxPendingTime:   5 * time.Minute,
			CriticalRestarts: 10,
		},
	}
}

// WithThresholds sets custom thresholds
func (h *HealthValidator) WithThresholds(thresholds HealthThresholds) *HealthValidator {
	h.thresholds = thresholds
	return h
}

// Validate validates health check results
func (h *HealthValidator) Validate(ctx context.Context, data interface{}, options *core.ValidationOptions) *core.NonGenericResult {
	startTime := time.Now()
	result := h.BaseValidatorImpl.Validate(ctx, data, options)

	// Type assertion for health data
	switch v := data.(type) {
	case map[string]interface{}:
		h.validateHealthData(v, result, options)
	case HealthCheckData:
		h.validateHealthCheckData(v, result, options)
	default:
		result.AddError(&core.Error{
			Code:     "INVALID_HEALTH_DATA",
			Message:  fmt.Sprintf("Expected health data, got %T", data),
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityHigh,
		})
	}

	result.Duration = time.Since(startTime)
	return result
}

// HealthCheckData represents health check data structure
type HealthCheckData struct {
	Namespace    string                 `json:"namespace"`
	AppName      string                 `json:"app_name"`
	Pods         []PodHealth            `json:"pods"`
	Services     []ServiceHealth        `json:"services"`
	HealthStatus string                 `json:"health_status"`
	CheckedAt    time.Time              `json:"checked_at"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// PodHealth represents pod health information
type PodHealth struct {
	Name       string            `json:"name"`
	Status     string            `json:"status"`
	Ready      bool              `json:"ready"`
	Phase      string            `json:"phase"`
	Node       string            `json:"node"`
	Age        time.Duration     `json:"age"`
	Containers []ContainerHealth `json:"containers"`
	Conditions []PodCondition    `json:"conditions"`
	Labels     map[string]string `json:"labels"`
}

// ContainerHealth represents container health information
type ContainerHealth struct {
	Name         string    `json:"name"`
	Ready        bool      `json:"ready"`
	RestartCount int       `json:"restart_count"`
	State        string    `json:"state"`
	LastStart    time.Time `json:"last_start"`
	ExitCode     int       `json:"exit_code"`
}

// ServiceHealth represents service health information
type ServiceHealth struct {
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	ClusterIP string   `json:"cluster_ip"`
	Endpoints int      `json:"endpoints"`
	Ports     []string `json:"ports"`
}

// PodCondition represents a pod condition
type PodCondition struct {
	Type   string `json:"type"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

// validateHealthData validates health data from a map
func (h *HealthValidator) validateHealthData(data map[string]interface{}, result *core.NonGenericResult, options *core.ValidationOptions) {
	// Validate namespace
	if namespace, ok := data["namespace"].(string); !ok || namespace == "" {
		result.AddFieldError("namespace", "Namespace is required")
	}

	// Validate pods
	if pods, ok := data["pods"].([]interface{}); ok {
		h.validatePods(pods, result, options)
	}

	// Validate services
	if services, ok := data["services"].([]interface{}); ok {
		h.validateServices(services, result, options)
	}

	// Validate health status
	if status, ok := data["health_status"].(string); ok {
		h.validateHealthStatus(status, result)
	}
}

// validateHealthCheckData validates structured health check data
func (h *HealthValidator) validateHealthCheckData(data HealthCheckData, result *core.NonGenericResult, options *core.ValidationOptions) {
	// Validate namespace
	if data.Namespace == "" {
		result.AddFieldError("namespace", "Namespace is required")
	}

	// Validate pods
	h.validatePodHealth(data.Pods, result, options)

	// Validate services
	h.validateServiceHealth(data.Services, result, options)

	// Validate overall health status
	h.validateHealthStatus(data.HealthStatus, result)

	// Calculate health metrics
	h.calculateHealthMetrics(data, result)
}

// validatePods validates pod data from interface slice
func (h *HealthValidator) validatePods(pods []interface{}, result *core.NonGenericResult, options *core.ValidationOptions) {
	var healthyPods, totalPods int

	for i, pod := range pods {
		if podMap, ok := pod.(map[string]interface{}); ok {
			totalPods++

			// Check pod status
			if status, ok := podMap["status"].(string); ok {
				if err := h.validatePodStatus(status, i, result); err == nil {
					if ready, ok := podMap["ready"].(bool); ok && ready {
						healthyPods++
					}
				}
			}

			// Check containers
			if containers, ok := podMap["containers"].([]interface{}); ok {
				h.validateContainers(containers, i, result)
			}
		}
	}

	// Check healthy ratio
	if totalPods > 0 {
		ratio := float64(healthyPods) / float64(totalPods)
		if ratio < h.thresholds.MinHealthyRatio {
			result.AddError(&core.Error{
				Code:     "UNHEALTHY_POD_RATIO",
				Message:  fmt.Sprintf("Only %.1f%% of pods are healthy (threshold: %.1f%%)", ratio*100, h.thresholds.MinHealthyRatio*100),
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityHigh,
			})
		}
	}
}

// validatePodHealth validates structured pod health data
func (h *HealthValidator) validatePodHealth(pods []PodHealth, result *core.NonGenericResult, options *core.ValidationOptions) {
	var healthyPods int
	totalRestarts := 0
	pendingPods := 0

	for i, pod := range pods {
		// Validate pod status
		if err := h.validatePodStatus(pod.Status, i, result); err == nil && pod.Ready {
			healthyPods++
		}

		// Check pod phase
		switch strings.ToLower(pod.Phase) {
		case "pending":
			pendingPods++
			if pod.Age > h.thresholds.MaxPendingTime {
				pendingError := &core.Error{
					Code:     "POD_PENDING_TOO_LONG",
					Message:  fmt.Sprintf("Pod %s has been pending for %v", pod.Name, pod.Age),
					Type:     core.ErrTypeValidation,
					Severity: core.SeverityHigh,
					Field:    fmt.Sprintf("pods[%d]", i),
				}
				pendingError.Suggestions = append(pendingError.Suggestions, "Check node resources and scheduling constraints")
				result.AddError(pendingError)
			}
		case "failed":
			result.AddError(&core.Error{
				Code:     "POD_FAILED",
				Message:  fmt.Sprintf("Pod %s is in failed state", pod.Name),
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityCritical,
				Field:    fmt.Sprintf("pods[%d]", i),
			})
		}

		// Validate containers
		for j, container := range pod.Containers {
			totalRestarts += container.RestartCount
			h.validateContainerHealth(container, i, j, result)
		}

		// Check conditions
		h.validatePodConditions(pod.Conditions, pod.Name, result)
	}

	// Overall health checks
	if len(pods) > 0 {
		healthRatio := float64(healthyPods) / float64(len(pods))
		if healthRatio < h.thresholds.MinHealthyRatio {
			healthError := &core.Error{
				Code:     "LOW_HEALTH_RATIO",
				Message:  fmt.Sprintf("Only %.1f%% of pods are healthy", healthRatio*100),
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityHigh,
			}
			healthError.Suggestions = append(healthError.Suggestions, "Investigate pod failures and resource constraints")
			result.AddError(healthError)
		}

		// Add warning for high restart count
		if totalRestarts > len(pods)*h.thresholds.MaxRestartCount {
			warning := core.NewWarning(
				"HIGH_RESTART_COUNT",
				fmt.Sprintf("Total restart count (%d) exceeds threshold", totalRestarts),
			)
			result.AddWarning(warning)
		}
	}
}

// validateContainers validates container data
func (h *HealthValidator) validateContainers(containers []interface{}, podIndex int, result *core.NonGenericResult) {
	for j, container := range containers {
		if containerMap, ok := container.(map[string]interface{}); ok {
			// Check restart count
			if restarts, ok := containerMap["restart_count"].(int); ok && restarts > h.thresholds.MaxRestartCount {
				severity := core.SeverityMedium
				if restarts > h.thresholds.CriticalRestarts {
					severity = core.SeverityHigh
				}

				restartError := &core.Error{
					Code:     "HIGH_CONTAINER_RESTARTS",
					Message:  fmt.Sprintf("Container has restarted %d times", restarts),
					Type:     core.ErrTypeValidation,
					Severity: severity,
					Field:    fmt.Sprintf("pods[%d].containers[%d]", podIndex, j),
				}
				restartError.Suggestions = append(restartError.Suggestions, "Check container logs and resource limits")
				result.AddError(restartError)
			}
		}
	}
}

// validateContainerHealth validates structured container health
func (h *HealthValidator) validateContainerHealth(container ContainerHealth, podIndex, containerIndex int, result *core.NonGenericResult) {
	field := fmt.Sprintf("pods[%d].containers[%d]", podIndex, containerIndex)

	// Check restart count
	if container.RestartCount > h.thresholds.MaxRestartCount {
		severity := core.SeverityMedium
		if container.RestartCount > h.thresholds.CriticalRestarts {
			severity = core.SeverityHigh
		}

		containerError := &core.Error{
			Code:     "CONTAINER_RESTART_THRESHOLD",
			Message:  fmt.Sprintf("Container %s has %d restarts", container.Name, container.RestartCount),
			Type:     core.ErrTypeValidation,
			Severity: severity,
			Field:    field,
		}
		containerError.Suggestions = append(containerError.Suggestions, "Review container logs for crash reasons")
		result.AddError(containerError)
	}

	// Check container state
	switch strings.ToLower(container.State) {
	case "crashloopbackoff":
		crashError := &core.Error{
			Code:     "CONTAINER_CRASH_LOOP",
			Message:  fmt.Sprintf("Container %s is in CrashLoopBackOff", container.Name),
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityCritical,
			Field:    field,
		}
		crashError.Suggestions = append(crashError.Suggestions, "Application is crashing repeatedly - check logs and configuration")
		result.AddError(crashError)
	case "imagepullbackoff":
		imageError := &core.Error{
			Code:     "IMAGE_PULL_ERROR",
			Message:  fmt.Sprintf("Container %s cannot pull image", container.Name),
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityHigh,
			Field:    field,
		}
		imageError.Suggestions = append(imageError.Suggestions, "Verify image name and registry credentials")
		result.AddError(imageError)
	case "error":
		result.AddError(&core.Error{
			Code:     "CONTAINER_ERROR",
			Message:  fmt.Sprintf("Container %s is in error state", container.Name),
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityHigh,
			Field:    field,
		})
	}

	// Check exit code
	if container.ExitCode != 0 && container.State != "running" {
		result.AddWarning(&core.Warning{
			Error: &core.Error{
				Code:     "NON_ZERO_EXIT",
				Message:  fmt.Sprintf("Container %s exited with code %d", container.Name, container.ExitCode),
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityMedium,
				Field:    field,
			},
		})
	}
}

// validateServices validates service data
func (h *HealthValidator) validateServices(services []interface{}, result *core.NonGenericResult, options *core.ValidationOptions) {
	for i, service := range services {
		if serviceMap, ok := service.(map[string]interface{}); ok {
			// Check endpoints
			if endpoints, ok := serviceMap["endpoints"].(int); ok && endpoints == 0 {
				result.AddWarning(&core.Warning{
					Error: &core.Error{
						Code:     "NO_SERVICE_ENDPOINTS",
						Message:  "Service has no endpoints",
						Type:     core.ErrTypeValidation,
						Severity: core.SeverityMedium,
						Field:    fmt.Sprintf("services[%d]", i),
					},
				})
			}
		}
	}
}

// validateServiceHealth validates structured service health
func (h *HealthValidator) validateServiceHealth(services []ServiceHealth, result *core.NonGenericResult, options *core.ValidationOptions) {
	for i, service := range services {
		// Check endpoints
		if service.Endpoints == 0 {
			result.AddWarning(&core.Warning{
				Error: &core.Error{
					Code:     "SERVICE_NO_ENDPOINTS",
					Message:  fmt.Sprintf("Service %s has no endpoints", service.Name),
					Type:     core.ErrTypeValidation,
					Severity: core.SeverityMedium,
					Field:    fmt.Sprintf("services[%d]", i),
				},
			})
		}

		// Validate service type
		validTypes := map[string]bool{
			"ClusterIP":    true,
			"NodePort":     true,
			"LoadBalancer": true,
			"ExternalName": true,
		}
		if !validTypes[service.Type] {
			result.AddWarning(&core.Warning{
				Error: &core.Error{
					Code:     "UNKNOWN_SERVICE_TYPE",
					Message:  fmt.Sprintf("Unknown service type: %s", service.Type),
					Type:     core.ErrTypeValidation,
					Severity: core.SeverityLow,
					Field:    fmt.Sprintf("services[%d].type", i),
				},
			})
		}
	}
}

// validatePodStatus validates pod status string
func (h *HealthValidator) validatePodStatus(status string, index int, result *core.NonGenericResult) error {
	validStatuses := map[string]bool{
		"running":   true,
		"pending":   true,
		"succeeded": true,
		"failed":    true,
		"unknown":   true,
	}

	statusLower := strings.ToLower(status)
	if !validStatuses[statusLower] {
		err := &core.Error{
			Code:     "INVALID_POD_STATUS",
			Message:  fmt.Sprintf("Invalid pod status: %s", status),
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityMedium,
			Field:    fmt.Sprintf("pods[%d].status", index),
		}
		result.AddError(err)
		return err
	}

	return nil
}

// validateHealthStatus validates overall health status
func (h *HealthValidator) validateHealthStatus(status string, result *core.NonGenericResult) {
	validStatuses := map[string]bool{
		"healthy":   true,
		"degraded":  true,
		"unhealthy": true,
		"unknown":   true,
	}

	if !validStatuses[strings.ToLower(status)] {
		result.AddFieldError("health_status", fmt.Sprintf("Invalid health status: %s", status))
	}
}

// validatePodConditions validates pod conditions
func (h *HealthValidator) validatePodConditions(conditions []PodCondition, podName string, result *core.NonGenericResult) {
	for _, condition := range conditions {
		// Check for problematic conditions
		if condition.Type == "Ready" && condition.Status != "True" {
			result.AddWarning(&core.Warning{
				Error: &core.Error{
					Code:     "POD_NOT_READY",
					Message:  fmt.Sprintf("Pod %s is not ready: %s", podName, condition.Reason),
					Type:     core.ErrTypeValidation,
					Severity: core.SeverityMedium,
				},
			})
		}

		// Check for scheduling issues
		if condition.Type == "PodScheduled" && condition.Status != "True" {
			scheduleError := &core.Error{
				Code:     "POD_SCHEDULING_FAILED",
				Message:  fmt.Sprintf("Pod %s could not be scheduled: %s", podName, condition.Reason),
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityHigh,
			}
			scheduleError.Suggestions = append(scheduleError.Suggestions, "Check node resources and affinity rules")
			result.AddError(scheduleError)
		}
	}
}

// calculateHealthMetrics calculates and adds health metrics to the result
func (h *HealthValidator) calculateHealthMetrics(data HealthCheckData, result *core.NonGenericResult) {
	metrics := make(map[string]interface{})

	// Calculate pod metrics
	if len(data.Pods) > 0 {
		healthyPods := 0
		totalRestarts := 0

		for _, pod := range data.Pods {
			if pod.Ready {
				healthyPods++
			}
			for _, container := range pod.Containers {
				totalRestarts += container.RestartCount
			}
		}

		metrics["pod_count"] = len(data.Pods)
		metrics["healthy_pods"] = healthyPods
		metrics["health_ratio"] = float64(healthyPods) / float64(len(data.Pods))
		metrics["total_restarts"] = totalRestarts
		metrics["avg_restarts_per_pod"] = float64(totalRestarts) / float64(len(data.Pods))
	}

	// Calculate service metrics
	if len(data.Services) > 0 {
		servicesWithEndpoints := 0
		for _, service := range data.Services {
			if service.Endpoints > 0 {
				servicesWithEndpoints++
			}
		}

		metrics["service_count"] = len(data.Services)
		metrics["services_with_endpoints"] = servicesWithEndpoints
	}

	// Add metrics to result
	result.Metadata.Context["health_metrics"] = metrics

	// Calculate risk level based on metrics
	riskLevel := h.calculateRiskLevel(metrics)
	result.RiskLevel = riskLevel
}

// calculateRiskLevel calculates risk level based on health metrics
func (h *HealthValidator) calculateRiskLevel(metrics map[string]interface{}) string {
	healthRatio, ok := metrics["health_ratio"].(float64)
	if !ok {
		healthRatio = 0.0 // Default to worst case if not available
	}

	totalRestarts, ok := metrics["total_restarts"].(int)
	if !ok {
		totalRestarts = 0 // Default to 0 if not available
	}

	if healthRatio < 0.5 || totalRestarts > 50 {
		return "critical"
	} else if healthRatio < 0.8 || totalRestarts > 20 {
		return "high"
	} else if healthRatio < 0.95 || totalRestarts > 10 {
		return "medium"
	}
	return "low"
}

// ============================================================================
// Type-Safe Validation Methods (applying Kubernetes validator pattern)
// ============================================================================

// ValidateTyped validates health data with type safety
func (h *HealthValidator) ValidateTyped(ctx context.Context, healthData HealthCheckData, options *core.ValidationOptions) *core.NonGenericResult {
	startTime := time.Now()
	result := h.BaseValidatorImpl.Validate(ctx, healthData, options)

	// Use the typed data for validation
	h.validateHealthCheckData(healthData, result, options)

	result.Duration = time.Since(startTime)
	return result
}

// ConvertToHealthCheckData converts interface{} to typed HealthCheckData
// HealthDataConverter handles conversion of various data types to HealthCheckData
type HealthDataConverter struct{}

// NewHealthDataConverter creates a new health data converter
func NewHealthDataConverter() *HealthDataConverter {
	return &HealthDataConverter{}
}

// convertSinglePod converts a single pod map to PodHealth
func (c *HealthDataConverter) convertSinglePod(podMap map[string]interface{}) PodHealth {
	podHealth := PodHealth{
		Labels: make(map[string]string),
	}

	// Set basic pod fields
	if name, ok := podMap["name"].(string); ok {
		podHealth.Name = name
	}
	if status, ok := podMap["status"].(string); ok {
		podHealth.Status = status
	}
	if ready, ok := podMap["ready"].(bool); ok {
		podHealth.Ready = ready
	}
	if phase, ok := podMap["phase"].(string); ok {
		podHealth.Phase = phase
	}
	if node, ok := podMap["node"].(string); ok {
		podHealth.Node = node
	}

	// Convert containers
	c.convertContainers(&podHealth, podMap)

	// Convert conditions
	c.convertConditions(&podHealth, podMap)

	return podHealth
}

// convertContainers converts containers from interface{} to []ContainerHealth
func (c *HealthDataConverter) convertContainers(podHealth *PodHealth, podMap map[string]interface{}) {
	containers, ok := podMap["containers"].([]interface{})
	if !ok {
		return
	}

	for _, container := range containers {
		containerMap, ok := container.(map[string]interface{})
		if !ok {
			continue
		}

		containerHealth := ContainerHealth{
			LastStart: time.Now(),
		}
		if name, ok := containerMap["name"].(string); ok {
			containerHealth.Name = name
		}
		if ready, ok := containerMap["ready"].(bool); ok {
			containerHealth.Ready = ready
		}
		if restarts, ok := containerMap["restart_count"].(int); ok {
			containerHealth.RestartCount = restarts
		}
		if state, ok := containerMap["state"].(string); ok {
			containerHealth.State = state
		}
		if exitCode, ok := containerMap["exit_code"].(int); ok {
			containerHealth.ExitCode = exitCode
		}
		podHealth.Containers = append(podHealth.Containers, containerHealth)
	}
}

// convertConditions converts conditions from interface{} to []PodCondition
func (c *HealthDataConverter) convertConditions(podHealth *PodHealth, podMap map[string]interface{}) {
	conditions, ok := podMap["conditions"].([]interface{})
	if !ok {
		return
	}

	for _, condition := range conditions {
		conditionMap, ok := condition.(map[string]interface{})
		if !ok {
			continue
		}

		podCondition := PodCondition{}
		if condType, ok := conditionMap["type"].(string); ok {
			podCondition.Type = condType
		}
		if status, ok := conditionMap["status"].(string); ok {
			podCondition.Status = status
		}
		if reason, ok := conditionMap["reason"].(string); ok {
			podCondition.Reason = reason
		}
		podHealth.Conditions = append(podHealth.Conditions, podCondition)
	}
}

// convertSingleService converts a single service map to ServiceHealth
func (c *HealthDataConverter) convertSingleService(serviceMap map[string]interface{}) ServiceHealth {
	serviceHealth := ServiceHealth{}

	if name, ok := serviceMap["name"].(string); ok {
		serviceHealth.Name = name
	}
	if svcType, ok := serviceMap["type"].(string); ok {
		serviceHealth.Type = svcType
	}
	if clusterIP, ok := serviceMap["cluster_ip"].(string); ok {
		serviceHealth.ClusterIP = clusterIP
	}
	if endpoints, ok := serviceMap["endpoints"].(int); ok {
		serviceHealth.Endpoints = endpoints
	}
	if ports, ok := serviceMap["ports"].([]string); ok {
		serviceHealth.Ports = ports
	}

	return serviceHealth
}

func ConvertToHealthCheckData(data interface{}) (HealthCheckData, error) {
	converter := NewHealthDataConverter()
	return converter.Convert(data)
}

// Convert handles the main conversion logic using factory pattern
func (c *HealthDataConverter) Convert(data interface{}) (HealthCheckData, error) {
	healthData := HealthCheckData{
		Metadata:  make(map[string]interface{}),
		CheckedAt: time.Now(),
	}

	switch v := data.(type) {
	case HealthCheckData:
		return v, nil
	case map[string]interface{}:
		return c.convertFromMap(v, healthData)
	default:
		return healthData, errors.NewError().Messagef("unsupported data type for health validation: %T", data).Build()
	}
}

// convertFromMap converts from map[string]interface{} to HealthCheckData
func (c *HealthDataConverter) convertFromMap(v map[string]interface{}, healthData HealthCheckData) (HealthCheckData, error) {
	// Convert basic fields
	if namespace, ok := v["namespace"].(string); ok {
		healthData.Namespace = namespace
	}
	if appName, ok := v["app_name"].(string); ok {
		healthData.AppName = appName
	}
	if status, ok := v["health_status"].(string); ok {
		healthData.HealthStatus = status
	}

	// Convert pods using factory methods
	if pods, ok := v["pods"].([]interface{}); ok {
		for _, pod := range pods {
			if podMap, ok := pod.(map[string]interface{}); ok {
				podHealth := c.convertSinglePod(podMap)
				healthData.Pods = append(healthData.Pods, podHealth)
			}
		}
	}

	// Convert services using factory methods
	if services, ok := v["services"].([]interface{}); ok {
		for _, service := range services {
			if serviceMap, ok := service.(map[string]interface{}); ok {
				serviceHealth := c.convertSingleService(serviceMap)
				healthData.Services = append(healthData.Services, serviceHealth)
			}
		}
	}

	return healthData, nil
}
