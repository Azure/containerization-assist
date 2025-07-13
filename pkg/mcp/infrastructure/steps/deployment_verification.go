// Package steps provides comprehensive deployment verification with diagnostics
package steps

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/common/errors"
)

// DeploymentDiagnostics contains comprehensive diagnostics for a deployment
type DeploymentDiagnostics struct {
	Timestamp     time.Time              `json:"timestamp"`
	Namespace     string                 `json:"namespace"`
	AppName       string                 `json:"app_name"`
	DeploymentOK  bool                   `json:"deployment_ok"`
	PodsReady     int                    `json:"pods_ready"`
	PodsTotal     int                    `json:"pods_total"`
	PodStatuses   []PodStatus            `json:"pod_statuses"`
	Services      []ServiceStatus        `json:"services"`
	Events        []string               `json:"recent_events"`
	Logs          map[string]string      `json:"pod_logs"`
	ResourceUsage map[string]interface{} `json:"resource_usage"`
	Errors        []string               `json:"errors"`
	Warnings      []string               `json:"warnings"`
}

// PodStatus represents the status of a single pod
type PodStatus struct {
	Name            string   `json:"name"`
	Ready           bool     `json:"ready"`
	Status          string   `json:"status"`
	Restarts        int      `json:"restarts"`
	Age             string   `json:"age"`
	Node            string   `json:"node"`
	IP              string   `json:"ip"`
	ContainerStates []string `json:"container_states"`
	LastError       string   `json:"last_error,omitempty"`
}

// ServiceStatus represents the status of a service
type ServiceStatus struct {
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	ClusterIP  string   `json:"cluster_ip"`
	ExternalIP string   `json:"external_ip,omitempty"`
	Ports      []string `json:"ports"`
	Endpoints  int      `json:"endpoints"`
}

// VerifyDeploymentWithDiagnostics performs comprehensive deployment verification
func VerifyDeploymentWithDiagnostics(ctx context.Context, k8sResult *K8sResult, logger *slog.Logger) (*DeploymentDiagnostics, error) {
	if k8sResult == nil {
		return nil, errors.New(errors.CodeInvalidParameter, "deployment-verification", "k8s result is required", nil)
	}

	logger.Info("Starting comprehensive deployment verification",
		"app_name", k8sResult.AppName,
		"namespace", k8sResult.Namespace)

	diagnostics := &DeploymentDiagnostics{
		Timestamp:   time.Now(),
		Namespace:   k8sResult.Namespace,
		AppName:     k8sResult.AppName,
		PodStatuses: []PodStatus{},
		Services:    []ServiceStatus{},
		Events:      []string{},
		Logs:        make(map[string]string),
		Errors:      []string{},
		Warnings:    []string{},
	}

	// 1. Check deployment status
	if err := checkDeploymentStatus(ctx, k8sResult, diagnostics, logger); err != nil {
		diagnostics.Errors = append(diagnostics.Errors, fmt.Sprintf("Deployment check failed: %v", err))
	}

	// 2. Get pod statuses
	if err := getPodStatuses(ctx, k8sResult, diagnostics, logger); err != nil {
		diagnostics.Errors = append(diagnostics.Errors, fmt.Sprintf("Pod status check failed: %v", err))
	}

	// 3. Get service information
	if err := getServiceInfo(ctx, k8sResult, diagnostics, logger); err != nil {
		diagnostics.Warnings = append(diagnostics.Warnings, fmt.Sprintf("Service check failed: %v", err))
	}

	// 4. Collect recent events
	if err := collectEvents(ctx, k8sResult, diagnostics, logger); err != nil {
		diagnostics.Warnings = append(diagnostics.Warnings, fmt.Sprintf("Event collection failed: %v", err))
	}

	// 5. Collect pod logs (only if pods are not ready)
	if diagnostics.PodsReady < diagnostics.PodsTotal {
		if err := collectPodLogs(ctx, k8sResult, diagnostics, logger); err != nil {
			diagnostics.Warnings = append(diagnostics.Warnings, fmt.Sprintf("Log collection failed: %v", err))
		}
	}

	// 6. Check resource usage
	if err := checkResourceUsage(ctx, k8sResult, diagnostics, logger); err != nil {
		diagnostics.Warnings = append(diagnostics.Warnings, fmt.Sprintf("Resource check failed: %v", err))
	}

	// Determine overall health
	diagnostics.DeploymentOK = diagnostics.PodsReady == diagnostics.PodsTotal &&
		diagnostics.PodsTotal > 0 &&
		len(diagnostics.Errors) == 0

	logger.Info("Deployment verification completed",
		"deployment_ok", diagnostics.DeploymentOK,
		"pods_ready", diagnostics.PodsReady,
		"pods_total", diagnostics.PodsTotal,
		"errors", len(diagnostics.Errors),
		"warnings", len(diagnostics.Warnings))

	return diagnostics, nil
}

// checkDeploymentStatus checks the deployment resource status
func checkDeploymentStatus(ctx context.Context, k8sResult *K8sResult, diag *DeploymentDiagnostics, logger *slog.Logger) error {
	cmd := exec.CommandContext(ctx, "kubectl", "get", "deployment", k8sResult.AppName,
		"-n", k8sResult.Namespace,
		"-o", "jsonpath={.status.readyReplicas}/{.status.replicas}")

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Try to get more info about why it failed
		statusCmd := exec.CommandContext(ctx, "kubectl", "get", "deployment", k8sResult.AppName,
			"-n", k8sResult.Namespace, "-o", "wide")
		statusOutput, _ := statusCmd.CombinedOutput()

		return fmt.Errorf("deployment not found or error: %v, status: %s", err, string(statusOutput))
	}

	parts := strings.Split(string(output), "/")
	if len(parts) == 2 {
		fmt.Sscanf(parts[0], "%d", &diag.PodsReady)
		fmt.Sscanf(parts[1], "%d", &diag.PodsTotal)
	}

	return nil
}

// getPodStatuses gets detailed status for all pods
func getPodStatuses(ctx context.Context, k8sResult *K8sResult, diag *DeploymentDiagnostics, logger *slog.Logger) error {
	// Get pods in JSON format for detailed parsing
	cmd := exec.CommandContext(ctx, "kubectl", "get", "pods",
		"-n", k8sResult.Namespace,
		"-l", fmt.Sprintf("app=%s", k8sResult.AppName),
		"-o", "json")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get pods: %v", err)
	}

	var podList struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Status struct {
				Phase             string `json:"phase"`
				PodIP             string `json:"podIP"`
				HostIP            string `json:"hostIP"`
				ContainerStatuses []struct {
					Name         string `json:"name"`
					Ready        bool   `json:"ready"`
					RestartCount int    `json:"restartCount"`
					State        struct {
						Running *struct{} `json:"running,omitempty"`
						Waiting *struct {
							Reason  string `json:"reason"`
							Message string `json:"message"`
						} `json:"waiting,omitempty"`
						Terminated *struct {
							Reason  string `json:"reason"`
							Message string `json:"message"`
						} `json:"terminated,omitempty"`
					} `json:"state"`
				} `json:"containerStatuses"`
			} `json:"status"`
			Spec struct {
				NodeName string `json:"nodeName"`
			} `json:"spec"`
		} `json:"items"`
	}

	if err := json.Unmarshal(output, &podList); err != nil {
		return fmt.Errorf("failed to parse pod list: %v", err)
	}

	for _, pod := range podList.Items {
		podStatus := PodStatus{
			Name:            pod.Metadata.Name,
			Status:          pod.Status.Phase,
			Node:            pod.Spec.NodeName,
			IP:              pod.Status.PodIP,
			ContainerStates: []string{},
		}

		// Check container statuses
		allReady := true
		for _, container := range pod.Status.ContainerStatuses {
			podStatus.Restarts += container.RestartCount

			if !container.Ready {
				allReady = false
			}

			// Determine container state
			var state string
			if container.State.Running != nil {
				state = "Running"
			} else if container.State.Waiting != nil {
				state = fmt.Sprintf("Waiting: %s", container.State.Waiting.Reason)
				if container.State.Waiting.Message != "" {
					podStatus.LastError = container.State.Waiting.Message
				}
			} else if container.State.Terminated != nil {
				state = fmt.Sprintf("Terminated: %s", container.State.Terminated.Reason)
				if container.State.Terminated.Message != "" {
					podStatus.LastError = container.State.Terminated.Message
				}
			}

			podStatus.ContainerStates = append(podStatus.ContainerStates,
				fmt.Sprintf("%s: %s", container.Name, state))
		}

		podStatus.Ready = allReady
		diag.PodStatuses = append(diag.PodStatuses, podStatus)
	}

	return nil
}

// getServiceInfo gets service information
func getServiceInfo(ctx context.Context, k8sResult *K8sResult, diag *DeploymentDiagnostics, logger *slog.Logger) error {
	cmd := exec.CommandContext(ctx, "kubectl", "get", "service", k8sResult.AppName,
		"-n", k8sResult.Namespace,
		"-o", "json")

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Service might not exist, which is not critical
		logger.Debug("No service found", "app", k8sResult.AppName, "error", err)
		return nil
	}

	var service struct {
		Spec struct {
			Type      string `json:"type"`
			ClusterIP string `json:"clusterIP"`
			Ports     []struct {
				Port       int    `json:"port"`
				TargetPort int    `json:"targetPort"`
				NodePort   int    `json:"nodePort,omitempty"`
				Protocol   string `json:"protocol"`
			} `json:"ports"`
		} `json:"spec"`
		Status struct {
			LoadBalancer struct {
				Ingress []struct {
					IP string `json:"ip"`
				} `json:"ingress"`
			} `json:"loadBalancer"`
		} `json:"status"`
	}

	if err := json.Unmarshal(output, &service); err != nil {
		return fmt.Errorf("failed to parse service: %v", err)
	}

	svcStatus := ServiceStatus{
		Name:      k8sResult.AppName,
		Type:      service.Spec.Type,
		ClusterIP: service.Spec.ClusterIP,
		Ports:     []string{},
	}

	// Format ports
	for _, port := range service.Spec.Ports {
		portStr := fmt.Sprintf("%d:%d/%s", port.Port, port.TargetPort, port.Protocol)
		if port.NodePort > 0 {
			portStr = fmt.Sprintf("%d:%d:%d/%s", port.Port, port.TargetPort, port.NodePort, port.Protocol)
		}
		svcStatus.Ports = append(svcStatus.Ports, portStr)
	}

	// Get external IP for LoadBalancer services
	if service.Spec.Type == "LoadBalancer" && len(service.Status.LoadBalancer.Ingress) > 0 {
		svcStatus.ExternalIP = service.Status.LoadBalancer.Ingress[0].IP
	}

	// Count endpoints
	endpointCmd := exec.CommandContext(ctx, "kubectl", "get", "endpoints", k8sResult.AppName,
		"-n", k8sResult.Namespace,
		"-o", "jsonpath={.subsets[*].addresses[*].ip}")
	endpointOutput, _ := endpointCmd.CombinedOutput()
	if len(endpointOutput) > 0 {
		endpoints := strings.Fields(string(endpointOutput))
		svcStatus.Endpoints = len(endpoints)
	}

	diag.Services = append(diag.Services, svcStatus)
	return nil
}

// collectEvents collects recent events related to the deployment
func collectEvents(ctx context.Context, k8sResult *K8sResult, diag *DeploymentDiagnostics, logger *slog.Logger) error {
	// Get events for the namespace, filtered by the app
	cmd := exec.CommandContext(ctx, "kubectl", "get", "events",
		"-n", k8sResult.Namespace,
		"--field-selector", fmt.Sprintf("involvedObject.name=%s", k8sResult.AppName),
		"--sort-by=.lastTimestamp",
		"-o", "custom-columns=TIME:.lastTimestamp,TYPE:.type,REASON:.reason,MESSAGE:.message",
		"--no-headers")

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Also try to get pod events
		for _, pod := range diag.PodStatuses {
			podCmd := exec.CommandContext(ctx, "kubectl", "get", "events",
				"-n", k8sResult.Namespace,
				"--field-selector", fmt.Sprintf("involvedObject.name=%s", pod.Name),
				"--sort-by=.lastTimestamp",
				"-o", "custom-columns=TIME:.lastTimestamp,TYPE:.type,REASON:.reason,MESSAGE:.message",
				"--no-headers")
			podOutput, _ := podCmd.CombinedOutput()
			if len(podOutput) > 0 {
				output = append(output, podOutput...)
			}
		}
	}

	if len(output) > 0 {
		lines := strings.Split(string(output), "\n")
		// Keep last 20 events
		start := 0
		if len(lines) > 20 {
			start = len(lines) - 20
		}
		for _, line := range lines[start:] {
			if line = strings.TrimSpace(line); line != "" {
				diag.Events = append(diag.Events, line)
			}
		}
	}

	return nil
}

// collectPodLogs collects logs from pods that are not ready
func collectPodLogs(ctx context.Context, k8sResult *K8sResult, diag *DeploymentDiagnostics, logger *slog.Logger) error {
	for _, pod := range diag.PodStatuses {
		if !pod.Ready || pod.Restarts > 0 {
			// Get logs with tail limit
			cmd := exec.CommandContext(ctx, "kubectl", "logs",
				pod.Name,
				"-n", k8sResult.Namespace,
				"--tail=50",
				"--all-containers=true")

			output, err := cmd.CombinedOutput()
			if err != nil {
				// Try previous logs if current fails
				prevCmd := exec.CommandContext(ctx, "kubectl", "logs",
					pod.Name,
					"-n", k8sResult.Namespace,
					"--tail=50",
					"--previous",
					"--all-containers=true")
				prevOutput, prevErr := prevCmd.CombinedOutput()

				if prevErr == nil && len(prevOutput) > 0 {
					diag.Logs[pod.Name+"_previous"] = string(prevOutput)
				}

				if len(output) > 0 {
					diag.Logs[pod.Name+"_error"] = fmt.Sprintf("Error getting logs: %v\nPartial output: %s", err, string(output))
				}
			} else if len(output) > 0 {
				diag.Logs[pod.Name] = string(output)
			}
		}
	}

	return nil
}

// checkResourceUsage checks resource utilization
func checkResourceUsage(ctx context.Context, k8sResult *K8sResult, diag *DeploymentDiagnostics, logger *slog.Logger) error {
	// Get resource usage if metrics-server is available
	cmd := exec.CommandContext(ctx, "kubectl", "top", "pods",
		"-n", k8sResult.Namespace,
		"-l", fmt.Sprintf("app=%s", k8sResult.AppName),
		"--no-headers")

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Metrics server might not be installed
		logger.Debug("Unable to get resource metrics", "error", err)
		return nil
	}

	diag.ResourceUsage = make(map[string]interface{})
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 3 {
			podName := fields[0]
			cpu := fields[1]
			memory := fields[2]

			diag.ResourceUsage[podName] = map[string]string{
				"cpu":    cpu,
				"memory": memory,
			}
		}
	}

	return nil
}

// GenerateDiagnosticReport creates a human-readable diagnostic report
func GenerateDiagnosticReport(diag *DeploymentDiagnostics) string {
	var report bytes.Buffer

	report.WriteString(fmt.Sprintf("Deployment Diagnostics Report\n"))
	report.WriteString(fmt.Sprintf("Generated: %s\n", diag.Timestamp.Format(time.RFC3339)))
	report.WriteString(fmt.Sprintf("Namespace: %s, App: %s\n", diag.Namespace, diag.AppName))
	report.WriteString(fmt.Sprintf("Status: %s\n\n", map[bool]string{true: "✓ HEALTHY", false: "✗ UNHEALTHY"}[diag.DeploymentOK]))

	// Pod Summary
	report.WriteString(fmt.Sprintf("Pods: %d/%d Ready\n", diag.PodsReady, diag.PodsTotal))
	for _, pod := range diag.PodStatuses {
		status := "✗"
		if pod.Ready {
			status = "✓"
		}
		report.WriteString(fmt.Sprintf("  %s %s (%s) - Restarts: %d\n", status, pod.Name, pod.Status, pod.Restarts))
		if pod.LastError != "" {
			report.WriteString(fmt.Sprintf("    Error: %s\n", pod.LastError))
		}
	}

	// Services
	if len(diag.Services) > 0 {
		report.WriteString("\nServices:\n")
		for _, svc := range diag.Services {
			report.WriteString(fmt.Sprintf("  - %s (%s) - Endpoints: %d\n", svc.Name, svc.Type, svc.Endpoints))
			if svc.ExternalIP != "" {
				report.WriteString(fmt.Sprintf("    External IP: %s\n", svc.ExternalIP))
			}
		}
	}

	// Recent Events
	if len(diag.Events) > 0 {
		report.WriteString("\nRecent Events:\n")
		for _, event := range diag.Events {
			report.WriteString(fmt.Sprintf("  %s\n", event))
		}
	}

	// Errors and Warnings
	if len(diag.Errors) > 0 {
		report.WriteString("\nErrors:\n")
		for _, err := range diag.Errors {
			report.WriteString(fmt.Sprintf("  ✗ %s\n", err))
		}
	}

	if len(diag.Warnings) > 0 {
		report.WriteString("\nWarnings:\n")
		for _, warn := range diag.Warnings {
			report.WriteString(fmt.Sprintf("  ⚠ %s\n", warn))
		}
	}

	// Pod Logs (if any)
	if len(diag.Logs) > 0 {
		report.WriteString("\nPod Logs (last 50 lines):\n")
		for podName, logs := range diag.Logs {
			report.WriteString(fmt.Sprintf("\n=== %s ===\n%s\n", podName, logs))
		}
	}

	return report.String()
}
