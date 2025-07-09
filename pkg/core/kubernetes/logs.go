package kubernetes

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	mcperrors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/rs/zerolog"
)

// LogCollector collects logs from Kubernetes pods
type LogCollector struct {
	logger     zerolog.Logger
	kubectlCmd string
}

// NewLogCollector creates a new log collector
func NewLogCollector(logger zerolog.Logger) *LogCollector {
	return &LogCollector{
		logger:     logger.With().Str("component", "log_collector").Logger(),
		kubectlCmd: "kubectl",
	}
}

// PodLogs represents logs from a pod
type PodLogs struct {
	PodName      string    `json:"pod_name"`
	Namespace    string    `json:"namespace"`
	Container    string    `json:"container,omitempty"`
	Status       string    `json:"status"`
	RestartCount int       `json:"restart_count"`
	Logs         []string  `json:"logs"`
	Error        string    `json:"error,omitempty"`
	CollectedAt  time.Time `json:"collected_at"`
}

// CollectLogsResult contains collected logs from multiple pods
type CollectLogsResult struct {
	Success     bool      `json:"success"`
	PodLogs     []PodLogs `json:"pod_logs"`
	TotalPods   int       `json:"total_pods"`
	FailedPods  int       `json:"failed_pods"`
	CollectedAt time.Time `json:"collected_at"`
}

// CollectPodLogs collects logs from pods matching the label selector
func (lc *LogCollector) CollectPodLogs(ctx context.Context, namespace, labelSelector string, tailLines int) (*CollectLogsResult, error) {
	result := &CollectLogsResult{
		Success:     true,
		CollectedAt: time.Now(),
		PodLogs:     make([]PodLogs, 0),
	}

	// Get pods matching the label selector
	pods, err := lc.getPodsWithStatus(ctx, namespace, labelSelector)
	if err != nil {
		return result, mcperrors.NewError().Messagef("failed to get pods: %w", err).WithLocation().Build()
	}

	result.TotalPods = len(pods)
	if result.TotalPods == 0 {
		return result, nil
	}

	// Collect logs from each pod
	for _, podInfo := range pods {
		podLog := PodLogs{
			PodName:      podInfo.Name,
			Namespace:    namespace,
			Status:       podInfo.Status,
			RestartCount: podInfo.RestartCount,
			CollectedAt:  time.Now(),
		}

		// Only collect logs from pods that have containers
		if podInfo.Status != "Pending" {
			logs, err := lc.getPodLogs(ctx, namespace, podInfo.Name, tailLines)
			if err != nil {
				podLog.Error = err.Error()
				result.FailedPods++
				lc.logger.Warn().
					Err(err).
					Str("pod", podInfo.Name).
					Msg("Failed to collect pod logs")
			} else {
				podLog.Logs = logs
			}
		} else {
			// For pending pods, get events instead
			events, err := lc.getPodEvents(ctx, namespace, podInfo.Name)
			if err == nil && len(events) > 0 {
				podLog.Logs = events
			}
		}

		result.PodLogs = append(result.PodLogs, podLog)
	}

	return result, nil
}

// CollectCrashingPodLogs specifically collects logs from pods that are crashing or in error state
func (lc *LogCollector) CollectCrashingPodLogs(ctx context.Context, namespace, labelSelector string) (*CollectLogsResult, error) {
	result := &CollectLogsResult{
		Success:     true,
		CollectedAt: time.Now(),
		PodLogs:     make([]PodLogs, 0),
	}

	// Get pods matching the label selector
	pods, err := lc.getPodsWithStatus(ctx, namespace, labelSelector)
	if err != nil {
		return result, mcperrors.NewError().Messagef("failed to get pods: %w", err).WithLocation(

		// Filter for crashing/error pods
		).Build()
	}

	for _, podInfo := range pods {
		if lc.isPodUnhealthy(podInfo) {
			podLog := PodLogs{
				PodName:      podInfo.Name,
				Namespace:    namespace,
				Status:       podInfo.Status,
				RestartCount: podInfo.RestartCount,
				CollectedAt:  time.Now(),
			}

			// Get logs (including previous container logs if crashed)
			if podInfo.RestartCount > 0 {
				// Get previous container logs
				prevLogs, err := lc.getPreviousPodLogs(ctx, namespace, podInfo.Name, 30)
				if err == nil {
					podLog.Logs = append(podLog.Logs, "=== Previous Container Logs ===")
					podLog.Logs = append(podLog.Logs, prevLogs...)
					podLog.Logs = append(podLog.Logs, "")
				}
			}

			// Get current logs
			currentLogs, err := lc.getPodLogs(ctx, namespace, podInfo.Name, 30)
			if err != nil {
				podLog.Error = err.Error()
				// Try to get pod events if logs fail
				events, _ := lc.getPodEvents(ctx, namespace, podInfo.Name)
				if len(events) > 0 {
					podLog.Logs = append(podLog.Logs, "=== Pod Events ===")
					podLog.Logs = append(podLog.Logs, events...)
				}
			} else {
				podLog.Logs = append(podLog.Logs, "=== Current Container Logs ===")
				podLog.Logs = append(podLog.Logs, currentLogs...)
			}

			result.PodLogs = append(result.PodLogs, podLog)
			result.TotalPods++
			if podLog.Error != "" {
				result.FailedPods++
			}
		}
	}

	return result, nil
}

// podInfo holds basic pod information
type podInfo struct {
	Name         string
	Status       string
	RestartCount int
}

// getPodsWithStatus gets pods with their status
func (lc *LogCollector) getPodsWithStatus(ctx context.Context, namespace, labelSelector string) ([]podInfo, error) {
	args := []string{
		"get", "pods",
		"-n", namespace,
		"-l", labelSelector,
		"--no-headers",
		"-o", "custom-columns=NAME:.metadata.name,STATUS:.status.phase,RESTARTS:.status.containerStatuses[0].restartCount",
	}

	cmd := exec.CommandContext(ctx, lc.kubectlCmd, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("kubectl get pods failed: %w", err)
	}

	var pods []podInfo
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 2 {
			restarts := 0
			if len(fields) >= 3 && fields[2] != "<none>" {
				fmt.Sscanf(fields[2], "%d", &restarts)
			}

			pods = append(pods, podInfo{
				Name:         fields[0],
				Status:       fields[1],
				RestartCount: restarts,
			})
		}
	}

	return pods, nil
}

// getPodLogs gets logs from a specific pod
func (lc *LogCollector) getPodLogs(ctx context.Context, namespace, podName string, tailLines int) ([]string, error) {
	args := []string{
		"logs",
		"-n", namespace,
		podName,
		fmt.Sprintf("--tail=%d", tailLines),
	}

	cmd := exec.CommandContext(ctx, lc.kubectlCmd, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("kubectl logs failed: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	return lines, nil
}

// getPreviousPodLogs gets logs from previous container instance
func (lc *LogCollector) getPreviousPodLogs(ctx context.Context, namespace, podName string, tailLines int) ([]string, error) {
	args := []string{
		"logs",
		"-n", namespace,
		podName,
		"--previous",
		fmt.Sprintf("--tail=%d", tailLines),
	}

	cmd := exec.CommandContext(ctx, lc.kubectlCmd, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("kubectl logs --previous failed: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	return lines, nil
}

// getPodEvents gets events for a specific pod
func (lc *LogCollector) getPodEvents(ctx context.Context, namespace, podName string) ([]string, error) {
	args := []string{
		"describe", "pod",
		"-n", namespace,
		podName,
	}

	cmd := exec.CommandContext(ctx, lc.kubectlCmd, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("kubectl describe pod failed: %w", err)
	}

	// Extract events section from describe output
	lines := strings.Split(string(output), "\n")
	var events []string
	inEvents := false

	for _, line := range lines {
		if strings.HasPrefix(line, "Events:") {
			inEvents = true
			continue
		}
		if inEvents {
			if strings.TrimSpace(line) != "" && !strings.HasPrefix(line, " ") {
				// End of events section
				break
			}
			if strings.TrimSpace(line) != "" {
				events = append(events, strings.TrimSpace(line))
			}
		}
	}

	// Limit to last 10 events
	if len(events) > 10 {
		events = events[len(events)-10:]
	}

	return events, nil
}

// isPodUnhealthy checks if a pod is in an unhealthy state
func (lc *LogCollector) isPodUnhealthy(pod podInfo) bool {
	unhealthyStatuses := []string{"Error", "CrashLoopBackOff", "ImagePullBackOff", "ErrImagePull", "Failed"}

	for _, status := range unhealthyStatuses {
		if strings.Contains(pod.Status, status) {
			return true
		}
	}

	// Also consider pods with high restart counts
	return pod.RestartCount > 2
}

// FormatPodLogs formats pod logs for display
func FormatPodLogs(result *CollectLogsResult) string {
	if result == nil || len(result.PodLogs) == 0 {
		return "No pod logs collected."
	}

	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf("=== Pod Logs (Collected at %s) ===\n", result.CollectedAt.Format(time.RFC3339)))
	buf.WriteString(fmt.Sprintf("Total pods: %d, Failed to collect: %d\n\n", result.TotalPods, result.FailedPods))

	for _, podLog := range result.PodLogs {
		buf.WriteString(fmt.Sprintf("Pod: %s (Status: %s, Restarts: %d)\n",
			podLog.PodName, podLog.Status, podLog.RestartCount))

		if podLog.Error != "" {
			buf.WriteString(fmt.Sprintf("Error collecting logs: %s\n", podLog.Error))
		} else if len(podLog.Logs) > 0 {
			buf.WriteString("Logs:\n")
			for _, line := range podLog.Logs {
				buf.WriteString(fmt.Sprintf("  %s\n", line))
			}
		} else {
			buf.WriteString("No logs available\n")
		}
		buf.WriteString("\n")
	}

	return buf.String()
}
