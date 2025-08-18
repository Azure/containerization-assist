package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	mcperrors "github.com/Azure/containerization-assist/pkg/common/errors"
	"github.com/rs/zerolog"
)

// HealthChecker provides mechanical Kubernetes health checking operations
type HealthChecker struct {
	kube   KubeRunner
	logger zerolog.Logger
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(kube KubeRunner, logger zerolog.Logger) *HealthChecker {
	return &HealthChecker{
		kube:   kube,
		logger: logger.With().Str("component", "k8s_health_checker").Logger(),
	}
}

// HealthCheckResult contains the result of a health check
type HealthCheckResult struct {
	Success   bool                    `json:"success"`
	Namespace string                  `json:"namespace"`
	Pods      []DetailedPodStatus     `json:"pods"`
	Services  []DetailedServiceStatus `json:"services"`
	Summary   HealthSummary           `json:"summary"`
	Duration  time.Duration           `json:"duration"`
	Context   map[string]interface{}  `json:"context"`
	Error     *HealthCheckError       `json:"error,omitempty"`
}

// DetailedPodStatus provides detailed pod health information
type DetailedPodStatus struct {
	Name       string            `json:"name"`
	Namespace  string            `json:"namespace"`
	Status     string            `json:"status"`
	Phase      string            `json:"phase"`
	Ready      bool              `json:"ready"`
	Restarts   int               `json:"restarts"`
	Age        string            `json:"age"`
	Node       string            `json:"node"`
	IP         string            `json:"ip"`
	Containers []ContainerStatus `json:"containers"`
	Conditions []PodCondition    `json:"conditions"`
	Events     []string          `json:"events,omitempty"`
}

// ContainerStatus provides container health information
type ContainerStatus struct {
	Name         string `json:"name"`
	Ready        bool   `json:"ready"`
	RestartCount int    `json:"restart_count"`
	State        string `json:"state"`
	Image        string `json:"image"`
	Reason       string `json:"reason,omitempty"`
	Message      string `json:"message,omitempty"`
}

// PodCondition represents a pod condition
type PodCondition struct {
	Type               string `json:"type"`
	Status             string `json:"status"`
	LastTransitionTime string `json:"last_transition_time"`
	Reason             string `json:"reason,omitempty"`
	Message            string `json:"message,omitempty"`
}

// DetailedServiceStatus provides detailed service information
type DetailedServiceStatus struct {
	Name       string            `json:"name"`
	Namespace  string            `json:"namespace"`
	Type       string            `json:"type"`
	ClusterIP  string            `json:"cluster_ip"`
	ExternalIP string            `json:"external_ip"`
	Ports      []ServicePort     `json:"ports"`
	Selector   map[string]string `json:"selector"`
	Age        string            `json:"age"`
	Endpoints  []string          `json:"endpoints,omitempty"`
}

// ServicePort represents a service port
type ServicePort struct {
	Name       string `json:"name,omitempty"`
	Port       int    `json:"port"`
	TargetPort string `json:"target_port"`
	Protocol   string `json:"protocol"`
	NodePort   int    `json:"node_port,omitempty"`
}

// HealthSummary provides a summary of health check results
type HealthSummary struct {
	TotalPods     int     `json:"total_pods"`
	ReadyPods     int     `json:"ready_pods"`
	FailedPods    int     `json:"failed_pods"`
	PendingPods   int     `json:"pending_pods"`
	TotalServices int     `json:"total_services"`
	HealthyRatio  float64 `json:"healthy_ratio"`
}

// HealthCheckError provides detailed health check error information
type HealthCheckError struct {
	Type      string                 `json:"type"`
	Message   string                 `json:"message"`
	Namespace string                 `json:"namespace,omitempty"`
	Resource  string                 `json:"resource,omitempty"`
	Context   map[string]interface{} `json:"context"`
}

// HealthCheckOptions contains options for health checking
type HealthCheckOptions struct {
	Namespace       string
	LabelSelector   string
	IncludeEvents   bool
	IncludeServices bool
	Timeout         time.Duration
}

// CheckApplicationHealth performs comprehensive health check of deployed applications
func (hc *HealthChecker) CheckApplicationHealth(ctx context.Context, options HealthCheckOptions) (*HealthCheckResult, error) {
	startTime := time.Now()

	result := &HealthCheckResult{
		Namespace: options.Namespace,
		Pods:      make([]DetailedPodStatus, 0),
		Services:  make([]DetailedServiceStatus, 0),
		Context:   make(map[string]interface{}),
	}

	if result.Namespace == "" {
		result.Namespace = "default"
	}

	hc.logger.Info().
		Str("namespace", result.Namespace).
		Str("label_selector", options.LabelSelector).
		Msg("Starting application health check")

	// Set timeout context if specified
	healthCtx := ctx
	if options.Timeout > 0 {
		var cancel context.CancelFunc
		healthCtx, cancel = context.WithTimeout(ctx, options.Timeout)
		defer cancel()
	}

	// Get detailed pod information
	pods, err := hc.getDetailedPodStatus(healthCtx, result.Namespace, options.LabelSelector)
	if err != nil {
		result.Error = &HealthCheckError{
			Type:      "pod_check_error",
			Message:   fmt.Sprintf("Failed to get pod status: %v", err),
			Namespace: result.Namespace,
		}
		result.Duration = time.Since(startTime)
		return result, nil
	}
	result.Pods = pods

	// Get service information if requested
	if options.IncludeServices {
		services, err := hc.getDetailedServiceStatus(healthCtx, result.Namespace)
		if err != nil {
			hc.logger.Warn().Err(err).Msg("Failed to get service status")
		} else {
			result.Services = services
		}
	}

	// Calculate summary
	result.Summary = hc.calculateHealthSummary(result.Pods, result.Services)

	// Determine overall success
	result.Success = result.Summary.HealthyRatio >= 1.0 && result.Summary.FailedPods == 0

	result.Duration = time.Since(startTime)
	result.Context = map[string]interface{}{
		"health_check_time": result.Duration.Seconds(),
		"namespace":         result.Namespace,
		"label_selector":    options.LabelSelector,
		"include_services":  options.IncludeServices,
	}

	hc.logger.Info().
		Int("total_pods", result.Summary.TotalPods).
		Int("ready_pods", result.Summary.ReadyPods).
		Int("failed_pods", result.Summary.FailedPods).
		Float64("healthy_ratio", result.Summary.HealthyRatio).
		Bool("success", result.Success).
		Dur("duration", result.Duration).
		Msg("Application health check completed")

	return result, nil
}

// WaitForPodReadiness waits for pods to become ready
func (hc *HealthChecker) WaitForPodReadiness(ctx context.Context, namespace string, labelSelector string, timeout time.Duration) (*HealthCheckResult, error) {
	hc.logger.Info().
		Str("namespace", namespace).
		Str("label_selector", labelSelector).
		Dur("timeout", timeout).
		Msg("Waiting for pod readiness")

	// Set up polling
	pollInterval := 5 * time.Second
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCtx.Done():
			// Timeout reached, return current status
			result, err := hc.CheckApplicationHealth(ctx, HealthCheckOptions{
				Namespace:     namespace,
				LabelSelector: labelSelector,
			})
			if err != nil {
				return nil, mcperrors.New(mcperrors.CodeIoError, "core", "timeout waiting for pod readiness: %v", err)
			}
			result.Error = &HealthCheckError{
				Type:      "timeout_error",
				Message:   fmt.Sprintf("Timeout waiting for pods to become ready after %v", timeout),
				Namespace: namespace,
			}
			return result, nil

		case <-ticker.C:
			result, err := hc.CheckApplicationHealth(ctx, HealthCheckOptions{
				Namespace:     namespace,
				LabelSelector: labelSelector,
			})
			if err != nil {
				hc.logger.Warn().Err(err).Msg("Health check failed during readiness wait")
				continue
			}

			if result.Success {
				hc.logger.Info().
					Int("ready_pods", result.Summary.ReadyPods).
					Dur("wait_time", time.Since(timeoutCtx.Value("start_time").(time.Time))).
					Msg("All pods are ready")
				return result, nil
			}

			hc.logger.Debug().
				Int("ready_pods", result.Summary.ReadyPods).
				Int("total_pods", result.Summary.TotalPods).
				Msg("Pods not yet ready, continuing to wait")
		}
	}
}

// Helper methods

func (hc *HealthChecker) getDetailedPodStatus(ctx context.Context, namespace string, labelSelector string) ([]DetailedPodStatus, error) {
	// Get pods in JSON format for detailed information
	output, err := hc.kube.GetPodsJSON(ctx, namespace, labelSelector)
	if err != nil {
		return nil, fmt.Errorf("failed to get pods JSON: %v", err)
	}

	// Parse JSON response
	var podList struct {
		Items []map[string]interface{} `json:"items"`
	}

	if err := json.Unmarshal([]byte(output), &podList); err != nil {
		return nil, fmt.Errorf("failed to parse pods JSON: %v", err)
	}

	pods := make([]DetailedPodStatus, 0, len(podList.Items))
	for _, item := range podList.Items {
		pod := hc.parsePodFromJSON(item)
		pods = append(pods, pod)
	}

	return pods, nil
}

func (hc *HealthChecker) getDetailedServiceStatus(ctx context.Context, namespace string) ([]DetailedServiceStatus, error) {
	// For now, return empty slice since we don't have detailed service JSON parsing
	// This could be implemented by adding a GetServicesJSON method to the KubeRunner interface
	return make([]DetailedServiceStatus, 0), nil
}

func (hc *HealthChecker) parsePodFromJSON(podData map[string]interface{}) DetailedPodStatus {
	pod := DetailedPodStatus{
		Containers: make([]ContainerStatus, 0),
		Conditions: make([]PodCondition, 0),
		Events:     make([]string, 0),
	}

	// Parse metadata
	if metadata, ok := podData["metadata"].(map[string]interface{}); ok {
		if name, ok := metadata["name"].(string); ok {
			pod.Name = name
		}
		if namespace, ok := metadata["namespace"].(string); ok {
			pod.Namespace = namespace
		}
	}

	// Parse spec
	if spec, ok := podData["spec"].(map[string]interface{}); ok {
		if nodeName, ok := spec["nodeName"].(string); ok {
			pod.Node = nodeName
		}
	}

	// Parse status
	if status, ok := podData["status"].(map[string]interface{}); ok {
		if phase, ok := status["phase"].(string); ok {
			pod.Phase = phase
			pod.Status = phase
		}
		if podIP, ok := status["podIP"].(string); ok {
			pod.IP = podIP
		}

		// Parse container statuses
		if containerStatuses, ok := status["containerStatuses"].([]interface{}); ok {
			for _, cs := range containerStatuses {
				if container, ok := cs.(map[string]interface{}); ok {
					containerStatus := hc.parseContainerStatus(container)
					pod.Containers = append(pod.Containers, containerStatus)

					// Update pod ready status based on containers
					if !containerStatus.Ready {
						pod.Ready = false
					}
				}
			}
			// If all containers are ready, pod is ready
			pod.Ready = len(pod.Containers) > 0
			for _, container := range pod.Containers {
				if !container.Ready {
					pod.Ready = false
					break
				}
			}
		}

		// Parse conditions
		if conditions, ok := status["conditions"].([]interface{}); ok {
			for _, c := range conditions {
				if condition, ok := c.(map[string]interface{}); ok {
					podCondition := hc.parsePodCondition(condition)
					pod.Conditions = append(pod.Conditions, podCondition)
				}
			}
		}
	}

	return pod
}

func (hc *HealthChecker) parseContainerStatus(containerData map[string]interface{}) ContainerStatus {
	container := ContainerStatus{}

	if name, ok := containerData["name"].(string); ok {
		container.Name = name
	}
	if ready, ok := containerData["ready"].(bool); ok {
		container.Ready = ready
	}
	if restartCount, ok := containerData["restartCount"].(float64); ok {
		container.RestartCount = int(restartCount)
	}
	if image, ok := containerData["image"].(string); ok {
		container.Image = image
	}

	// Parse state
	if state, ok := containerData["state"].(map[string]interface{}); ok {
		if running, ok := state["running"]; ok && running != nil {
			container.State = "running"
		} else if waiting, ok := state["waiting"].(map[string]interface{}); ok {
			container.State = "waiting"
			if reason, ok := waiting["reason"].(string); ok {
				container.Reason = reason
			}
			if message, ok := waiting["message"].(string); ok {
				container.Message = message
			}
		} else if terminated, ok := state["terminated"].(map[string]interface{}); ok {
			container.State = "terminated"
			if reason, ok := terminated["reason"].(string); ok {
				container.Reason = reason
			}
			if message, ok := terminated["message"].(string); ok {
				container.Message = message
			}
		}
	}

	return container
}

func (hc *HealthChecker) parsePodCondition(conditionData map[string]interface{}) PodCondition {
	condition := PodCondition{}

	if conditionType, ok := conditionData["type"].(string); ok {
		condition.Type = conditionType
	}
	if status, ok := conditionData["status"].(string); ok {
		condition.Status = status
	}
	if lastTransitionTime, ok := conditionData["lastTransitionTime"].(string); ok {
		condition.LastTransitionTime = lastTransitionTime
	}
	if reason, ok := conditionData["reason"].(string); ok {
		condition.Reason = reason
	}
	if message, ok := conditionData["message"].(string); ok {
		condition.Message = message
	}

	return condition
}

func (hc *HealthChecker) calculateHealthSummary(pods []DetailedPodStatus, services []DetailedServiceStatus) HealthSummary {
	summary := HealthSummary{
		TotalServices: len(services),
	}

	for _, pod := range pods {
		summary.TotalPods++

		switch strings.ToLower(pod.Status) {
		case "running":
			if pod.Ready {
				summary.ReadyPods++
			}
		case "failed", "crashloopbackoff":
			summary.FailedPods++
		case "pending":
			summary.PendingPods++
		}
	}

	// Calculate healthy ratio
	if summary.TotalPods > 0 {
		summary.HealthyRatio = float64(summary.ReadyPods) / float64(summary.TotalPods)
	}

	return summary
}
