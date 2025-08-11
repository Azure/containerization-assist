// Package steps provides comprehensive deployment verification with diagnostics
package steps

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os/exec"
	"strconv"
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

// PortForwardConfig contains configuration for port forwarding
type PortForwardConfig struct {
	Timeout    time.Duration // Default: 30 minutes
	LocalPort  int           // Auto-assigned if 0
	TargetPort int           // From service discovery
	Background bool          // Run as background process
}

// PortForwardResult contains the result of port forwarding attempt
type PortForwardResult struct {
	Success   bool
	LocalPort int
	ProcessID int
	AccessURL string
	Error     error
	Timeout   time.Time
}

// HealthCheckConfig contains configuration for health check operations
type HealthCheckConfig struct {
	URL            string        // Constructed from port forward
	Timeout        time.Duration // Default: 30 seconds
	RetryAttempts  int           // Default: 3
	ExpectedStatus []int         // Default: [200, 201, 204]
}

// HealthCheckResult contains the result of health check
type HealthCheckResult struct {
	Success      bool
	ResponseCode int
	ResponseTime time.Duration
	Error        error
}

// VerificationResult contains comprehensive verification results
type VerificationResult struct {
	DeploymentSuccess bool               `json:"deployment_success"`
	PortForwardResult *PortForwardResult `json:"port_forward,omitempty"`
	HealthCheckResult *HealthCheckResult `json:"health_check,omitempty"`
	AccessURL         string             `json:"access_url,omitempty"`
	Messages          []StatusMessage    `json:"messages"`
	UserMessage       string             `json:"user_message,omitempty"` // Formatted message for end user
	NextSteps         string             `json:"next_steps,omitempty"`   // Suggested next actions
}

// StatusMessage represents a status message with icon and level
type StatusMessage struct {
	Level   string `json:"level"` // success, warning, error, info
	Message string `json:"message"`
	Icon    string `json:"icon"` // ✅, ⚠️, ❌, ℹ️
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

	// 5. Collect pod logs (only for failing/not ready pods)
	if err := collectPodLogs(ctx, k8sResult, diagnostics, logger); err != nil {
		diagnostics.Warnings = append(diagnostics.Warnings, fmt.Sprintf("Log collection failed: %v", err))
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
	logger.Info("Checking deployment status", "app_name", k8sResult.AppName, "namespace", k8sResult.Namespace)

	// First, check if any deployments exist in the namespace
	allDeploymentsCmd := exec.CommandContext(ctx, "kubectl", "get", "deployments", "-n", k8sResult.Namespace)
	allDeploymentsOutput, err := allDeploymentsCmd.CombinedOutput()
	if err != nil {
		logger.Error("Failed to list deployments", "error", err, "output", string(allDeploymentsOutput))
		return fmt.Errorf("failed to list deployments: %v", err)
	}
	
	logger.Info("All deployments in namespace", "namespace", k8sResult.Namespace, "output", string(allDeploymentsOutput))

	cmd := exec.CommandContext(ctx, "kubectl", "get", "deployment", k8sResult.AppName,
		"-n", k8sResult.Namespace,
		"-o", "jsonpath={.status.readyReplicas}/{.status.replicas}")

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Try to get more info about why it failed
		statusCmd := exec.CommandContext(ctx, "kubectl", "get", "deployment", k8sResult.AppName,
			"-n", k8sResult.Namespace, "-o", "wide")
		statusOutput, _ := statusCmd.CombinedOutput()

		logger.Error("Deployment not found", "app_name", k8sResult.AppName, "namespace", k8sResult.Namespace,
			"error", err, "kubectl_output", string(output), "status_output", string(statusOutput))
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
	// First, get all pods in the namespace to see if there are any pods at all
	allPodsCmd := exec.CommandContext(ctx, "kubectl", "get", "pods", "-n", k8sResult.Namespace)
	allPodsOutput, _ := allPodsCmd.CombinedOutput()
	logger.Info("All pods in namespace", "namespace", k8sResult.Namespace, "output", string(allPodsOutput))

	// Get pods in JSON format for detailed parsing
	selector := fmt.Sprintf("app=%s", k8sResult.AppName)
	logger.Info("Getting pods with selector", "namespace", k8sResult.Namespace, "selector", selector)

	cmd := exec.CommandContext(ctx, "kubectl", "get", "pods",
		"-n", k8sResult.Namespace,
		"-l", selector,
		"-o", "json")

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("Failed to get pods", "error", err, "output", string(output), "selector", selector, "namespace", k8sResult.Namespace)
		return fmt.Errorf("failed to get pods: %v", err)
	}

	outputStr := string(output)
	maxLen := 500
	if len(outputStr) < maxLen {
		maxLen = len(outputStr)
	}
	logger.Info("Pod query result", "output_length", len(output), "raw_output", outputStr[:maxLen])

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

// collectPodLogs collects logs from failing/not ready pods for troubleshooting
func collectPodLogs(ctx context.Context, k8sResult *K8sResult, diag *DeploymentDiagnostics, logger *slog.Logger) error {
	for _, pod := range diag.PodStatuses {
		// Only collect logs for failing or not ready pods
		if pod.Ready && pod.Restarts == 0 {
			continue // Skip healthy pods
		}

		logger.Info("Collecting logs for problematic pod", "pod", pod.Name, "ready", pod.Ready, "restarts", pod.Restarts)

		tailLimit := "100" // More logs for problematic pods

		// Get logs with tail limit
		cmd := exec.CommandContext(ctx, "kubectl", "logs",
			pod.Name,
			"-n", k8sResult.Namespace,
			"--tail="+tailLimit,
			"--all-containers=true")

		output, err := cmd.CombinedOutput()
		if err != nil {
			// Try previous logs if current fails
			prevCmd := exec.CommandContext(ctx, "kubectl", "logs",
				pod.Name,
				"-n", k8sResult.Namespace,
				"--tail="+tailLimit,
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

// startPortForwardWithTimeout starts kubectl port-forward with timeout
func startPortForwardWithTimeout(ctx context.Context, config PortForwardConfig, serviceName, namespace string) (*PortForwardResult, error) {
	// Auto-assign local port if not specified
	if config.LocalPort == 0 {
		config.LocalPort = findAvailablePort()
	}

	// Start kubectl port-forward process
	cmd := exec.CommandContext(ctx, "kubectl", "port-forward",
		fmt.Sprintf("svc/%s", serviceName),
		fmt.Sprintf("%d:%d", config.LocalPort, config.TargetPort),
		"-n", namespace)

	// Capture both stdout and stderr to detect success/failure
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Start the process
	err := cmd.Start()
	if err != nil {
		return &PortForwardResult{
			Success: false,
			Error:   fmt.Errorf("failed to start kubectl port-forward: %w", err),
		}, err
	}

	// Wait for port forward to establish or fail (check every 500ms for up to 5 seconds)
	established := false
	var lastError error

	for i := 0; i < 10; i++ { // 10 * 500ms = 5 seconds max wait
		time.Sleep(500 * time.Millisecond)

		// Check if process is still running
		if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
			// Process exited, check for error
			stderrOutput := stderr.String()
			if stderrOutput != "" {
				lastError = fmt.Errorf("kubectl port-forward failed: %s", stderrOutput)
			} else {
				lastError = fmt.Errorf("kubectl port-forward process exited unexpectedly")
			}
			break
		}

		// Check stdout for success patterns (kubectl prints "Forwarding from..." on success)
		stdoutOutput := stdout.String()
		if strings.Contains(stdoutOutput, "Forwarding from") {
			established = true
			break
		}

		// Also check if port is responding (secondary check)
		if isPortInUse(config.LocalPort) {
			established = true
			break
		}

		// Check stderr for common error patterns
		stderrOutput := stderr.String()
		if strings.Contains(stderrOutput, "not found") ||
			strings.Contains(stderrOutput, "error") ||
			strings.Contains(stderrOutput, "failed") {
			lastError = fmt.Errorf("kubectl port-forward error: %s", stderrOutput)
			break
		}
	}

	if !established {
		// Kill the process if it's still running
		if cmd.Process != nil {
			cmd.Process.Kill()
		}

		if lastError == nil {
			lastError = fmt.Errorf("port forwarding failed to establish after 5 seconds, stdout: %s, stderr: %s",
				stdout.String(), stderr.String())
		}

		return &PortForwardResult{
			Success: false,
			Error:   lastError,
		}, lastError
	}

	accessURL := fmt.Sprintf("http://localhost:%d", config.LocalPort)
	result := &PortForwardResult{
		Success:   true,
		LocalPort: config.LocalPort,
		ProcessID: cmd.Process.Pid,
		AccessURL: accessURL,
		Timeout:   time.Now().Add(config.Timeout),
	}

	// Start cleanup goroutine that will kill the process after the timeout
	go func() {
		// Wait for the timeout duration, then kill the process
		time.Sleep(config.Timeout)
		if cmd.Process != nil {
			slog.Info("Killing port forwarding process")
			cmd.Process.Kill()
		}
	}()

	return result, nil
}

// performHealthCheck performs HTTP health check with retries
func performHealthCheck(ctx context.Context, config HealthCheckConfig) (*HealthCheckResult, error) {
	client := &http.Client{Timeout: config.Timeout}

	for attempt := 1; attempt <= config.RetryAttempts; attempt++ {
		start := time.Now()
		resp, err := client.Get(config.URL)
		responseTime := time.Since(start)

		if err == nil && contains(config.ExpectedStatus, resp.StatusCode) {
			resp.Body.Close()
			return &HealthCheckResult{
				Success:      true,
				ResponseCode: resp.StatusCode,
				ResponseTime: responseTime,
			}, nil
		}

		if resp != nil {
			resp.Body.Close()
		}

		// Wait before retry
		if attempt < config.RetryAttempts {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}

	return &HealthCheckResult{
		Success: false,
		Error:   fmt.Errorf("health check failed after %d attempts", config.RetryAttempts),
	}, nil
}

// findAvailablePort finds an available port by trying common development ports
func findAvailablePort() int {
	// Try common development ports in order
	for _, port := range []int{8080, 8081, 8082, 8083, 8084, 8085} {
		if isPortAvailable(port) {
			return port
		}
	}
	// If all fail, just return 8080 and let kubectl fail if needed
	return 8080
}

// isPortAvailable checks if a specific port is available for binding
func isPortAvailable(port int) bool {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false // Port is not available (already in use)
	}
	defer listener.Close()
	return true // Port is available
}

// isPortInUse checks if a specific port has something listening (for port forwarding detection)
func isPortInUse(port int) bool {
	// Try connecting to the port to see if kubectl port-forward is listening
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 200*time.Millisecond)
	if err != nil {
		// Nothing listening on the port yet
		return false
	}
	// Something is listening on the port (port forwarding established)
	conn.Close()
	return true
}

// contains checks if a slice contains a value
func contains(slice []int, value int) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

// VerifyDeploymentWithPortForward performs enhanced deployment verification with port forwarding
func VerifyDeploymentWithPortForward(ctx context.Context, k8sResult *K8sResult, logger *slog.Logger) (*VerificationResult, error) {
	result := &VerificationResult{
		Messages: []StatusMessage{},
	}

	// 1. First perform standard deployment verification
	diagnostics, err := VerifyDeploymentWithDiagnostics(ctx, k8sResult, logger)
	if err != nil {
		result.DeploymentSuccess = false
		result.Messages = append(result.Messages, StatusMessage{
			Level:   "error",
			Message: fmt.Sprintf("Deployment verification failed: %v", err),
			Icon:    "❌",
		})
		return result, err
	}

	result.DeploymentSuccess = diagnostics.DeploymentOK

	if !diagnostics.DeploymentOK {
		result.Messages = append(result.Messages, StatusMessage{
			Level:   "error",
			Message: "Deployment verification failed",
			Icon:    "❌",
		})
		result.Messages = append(result.Messages, StatusMessage{
			Level:   "warning",
			Message: "Port forwarding skipped (deployment not ready)",
			Icon:    "⚠️",
		})
		result.Messages = append(result.Messages, StatusMessage{
			Level:   "error",
			Message: "Application not accessible",
			Icon:    "❌",
		})
		return result, nil
	}

	result.Messages = append(result.Messages, StatusMessage{
		Level:   "success",
		Message: "Deployment verified successfully",
		Icon:    "✅",
	})

	// 2. Attempt port forwarding
	targetPort := extractTargetPortFromService(ctx, k8sResult, logger)
	if targetPort == 0 {
		targetPort = 8080 // Default fallback
	}

	portForwardConfig := PortForwardConfig{
		Timeout:    30 * time.Minute, // Default 30 minutes
		LocalPort:  0,                // Auto-assign
		TargetPort: targetPort,
		Background: true,
	}

	// Try port forwarding to service first
	portForwardResult, err := startPortForwardWithTimeout(ctx, portForwardConfig, k8sResult.AppName, k8sResult.Namespace)
	if err != nil || !portForwardResult.Success {
		errorMsg := "Port forwarding failed"
		if err != nil {
			errorMsg = fmt.Sprintf("Port forwarding failed: %v", err)
		}

		result.Messages = append(result.Messages, StatusMessage{
			Level:   "warning",
			Message: errorMsg,
			Icon:    "⚠️",
		})
		result.Messages = append(result.Messages, StatusMessage{
			Level:   "info",
			Message: "Application health unknown",
			Icon:    "❓",
		})
		result.Messages = append(result.Messages, StatusMessage{
			Level:   "info",
			Message: "App may be accessible via cluster networking",
			Icon:    "ℹ️",
		})
		return result, nil
	}

	result.Messages = append(result.Messages, StatusMessage{
		Level:   "success",
		Message: fmt.Sprintf("Port forwarding active (timeout: 30min)"),
		Icon:    "✅",
	})
	result.AccessURL = portForwardResult.AccessURL

	// 3. Attempt health check
	healthConfig := HealthCheckConfig{
		URL:            portForwardResult.AccessURL,
		Timeout:        30 * time.Second,
		RetryAttempts:  3,
		ExpectedStatus: []int{200, 201, 204},
	}

	healthResult, err := performHealthCheck(ctx, healthConfig)
	result.HealthCheckResult = healthResult

	if err != nil || !healthResult.Success {
		result.Messages = append(result.Messages, StatusMessage{
			Level:   "warning",
			Message: "Application health check failed",
			Icon:    "⚠️",
		})
	} else {
		result.Messages = append(result.Messages, StatusMessage{
			Level:   "success",
			Message: fmt.Sprintf("Application responding (%d OK, %dms)", healthResult.ResponseCode, healthResult.ResponseTime.Milliseconds()),
			Icon:    "✅",
		})
	}

	// 4. Generate user-friendly message and next steps
	if result.DeploymentSuccess && result.AccessURL != "" {
		// Access URL available case
		result.UserMessage = fmt.Sprintf("✅ Deployment verified successfully\n✅ Port forwarding active (timeout: 30min)\n🔗 Access your app: %s", result.AccessURL)
		result.NextSteps = "Test your application or run additional verification tools"
	} else if result.DeploymentSuccess {
		// Deployment successful but no access URL
		result.UserMessage = "✅ Deployment verified successfully"
		result.NextSteps = "Check cluster networking for app"
	} else {
		// Deployment failed
		result.UserMessage = "❌ Deployment verification failed"
		result.NextSteps = "Check deployment logs with 'kubectl describe' or retry deployment"
	}

	logger.Info("Enhanced deployment verification completed",
		"deployment_success", result.DeploymentSuccess,
		"port_forward_success", result.PortForwardResult != nil && result.PortForwardResult.Success,
		"health_check_success", result.HealthCheckResult != nil && result.HealthCheckResult.Success,
		"access_url", result.AccessURL)

	return result, nil
}

// extractTargetPortFromService extracts the target port from the service
func extractTargetPortFromService(ctx context.Context, k8sResult *K8sResult, logger *slog.Logger) int {
	cmd := exec.CommandContext(ctx, "kubectl", "get", "service", k8sResult.AppName,
		"-n", k8sResult.Namespace,
		"-o", "jsonpath={.spec.ports[0].port}")

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Warn("Failed to get service port, using default", "error", err)
		return 8080
	}

	port, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		logger.Warn("Failed to parse service port, using default", "error", err, "output", string(output))
		return 8080
	}

	return port
}
