package observability

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/errors"
	"github.com/Azure/container-kit/pkg/mcp/internal/utils"
	"github.com/rs/zerolog"
)

// Collector gathers diagnostic information for rich error contexts
type Collector struct {
	logger zerolog.Logger
}

// NewCollector creates a new diagnostics collector
func NewCollector(logger zerolog.Logger) *Collector {
	return &Collector{
		logger: logger.With().Str("component", "diagnostics").Logger(),
	}
}

// CollectSystemState gathers current system state information
func (c *Collector) CollectSystemState(ctx context.Context) errors.SystemState {
	state := errors.SystemState{
		DockerAvailable: c.checkDockerAvailable(),
		K8sConnected:    c.checkK8sConnection(),
		DiskSpaceMB:     c.getAvailableDiskSpace(),
		MemoryMB:        c.getMemoryUsage(),
		LoadAverage:     0.0, // TODO: implement load average collection
	}

	c.logger.Debug().
		Bool("docker", state.DockerAvailable).
		Bool("k8s", state.K8sConnected).
		Int64("disk_mb", state.DiskSpaceMB).
		Msg("Collected system state")

	return state
}

// CollectResourceUsage gathers current resource usage
func (c *Collector) CollectResourceUsage() errors.ResourceUsage {
	usage := errors.ResourceUsage{
		CPUPercent:     c.getCPUUsage(),
		MemoryMB:       c.getMemoryUsage(),
		DiskUsageMB:    c.getDiskUsage(),
		NetworkBytesTx: 0, // TODO: implement network TX collection
		NetworkBytesRx: 0, // TODO: implement network RX collection
	}

	c.logger.Debug().
		Float64("cpu_percent", usage.CPUPercent).
		Int64("memory_mb", usage.MemoryMB).
		Msg("Collected resource usage")

	return usage
}

// CollectBuildDiagnostics gathers diagnostics specific to build errors
func (c *Collector) CollectBuildDiagnostics(ctx context.Context, buildContext string) map[string]interface{} {
	diag := make(map[string]interface{})

	// Check Docker version
	if version, err := c.getDockerVersion(); err == nil {
		diag["docker_version"] = version
	}

	// Check Docker daemon info
	if info, err := c.getDockerInfo(); err == nil {
		diag["docker_info"] = info
	}

	// Check build context size
	if size, err := c.getDirectorySize(buildContext); err == nil {
		diag["build_context_size_mb"] = size / (1024 * 1024)
	}

	// Check available Docker images
	if images, err := c.getDockerImages(); err == nil {
		diag["available_images"] = len(images)
	}

	return diag
}

// CollectDeploymentDiagnostics gathers diagnostics specific to deployment errors
func (c *Collector) CollectDeploymentDiagnostics(ctx context.Context, namespace string) map[string]interface{} {
	diag := make(map[string]interface{})

	// Check kubectl version
	if version, err := c.getKubectlVersion(); err == nil {
		diag["kubectl_version"] = version
	}

	// Check current context
	if context, err := c.getKubeContext(); err == nil {
		diag["kube_context"] = context
	}

	// Check namespace exists
	if exists, err := c.checkNamespaceExists(namespace); err == nil {
		diag["namespace_exists"] = exists
	}

	// Get namespace quota if available
	if quota, err := c.getNamespaceQuota(namespace); err == nil {
		diag["namespace_quota"] = quota
	}

	return diag
}

// Helper methods

func (c *Collector) checkDockerAvailable() bool {
	cmd := exec.Command("docker", "version")
	err := cmd.Run()
	return err == nil
}

func (c *Collector) checkK8sConnection() bool {
	cmd := exec.Command("kubectl", "cluster-info")
	err := cmd.Run()
	return err == nil
}

func (c *Collector) getAvailableDiskSpace() int64 {
	var stat syscall.Statfs_t
	wd, err := os.Getwd()
	if err != nil {
		return 0
	}

	if err := syscall.Statfs(wd, &stat); err != nil {
		return 0
	}

	// Available blocks * block size / 1MB
	return int64(stat.Bavail) * int64(stat.Bsize) / (1024 * 1024)
}

func (c *Collector) getWorkspaceQuota() int64 {
	// Default workspace quota in MB
	return 1024 // 1GB default
}

func (c *Collector) checkNetworkStatus() string {
	// Simple network check
	cmd := exec.Command("ping", "-c", "1", "-W", "2", "8.8.8.8")
	if err := cmd.Run(); err != nil {
		return "offline"
	}
	return "online"
}

func (c *Collector) getCPUUsage() float64 {
	// Simplified CPU usage - would need more sophisticated implementation
	return 0.0
}

func (c *Collector) getMemoryUsage() int64 {
	// Get memory usage in MB
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int64(m.Alloc / 1024 / 1024)
}

func (c *Collector) getDiskUsage() int64 {
	// Get disk usage of current directory in MB
	wd, err := os.Getwd()
	if err != nil {
		return 0
	}

	size, err := c.getDirectorySize(wd)
	if err != nil {
		return 0
	}
	return size / (1024 * 1024)
}

func (c *Collector) getNetworkBandwidth() string {
	// Placeholder for network bandwidth
	return "unknown"
}

func (c *Collector) getDockerVersion() (string, error) {
	cmd := exec.Command("docker", "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func (c *Collector) getDockerInfo() (map[string]interface{}, error) {
	info := make(map[string]interface{})

	// Get Docker system info
	cmd := exec.Command("docker", "system", "df")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// Parse output for basic info
	lines := strings.Split(string(output), "\n")
	if len(lines) > 0 {
		info["system_df"] = lines[0]
	}

	return info, nil
}

func (c *Collector) getDirectorySize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

func (c *Collector) getDockerImages() ([]string, error) {
	cmd := exec.Command("docker", "images", "--format", "{{.Repository}}:{{.Tag}}")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	return lines, nil
}

func (c *Collector) getKubectlVersion() (string, error) {
	cmd := exec.Command("kubectl", "version", "--client", "--short")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func (c *Collector) getKubeContext() (string, error) {
	cmd := exec.Command("kubectl", "config", "current-context")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func (c *Collector) checkNamespaceExists(namespace string) (bool, error) {
	cmd := exec.Command("kubectl", "get", "namespace", namespace)
	err := cmd.Run()
	return err == nil, nil
}

func (c *Collector) getNamespaceQuota(namespace string) (map[string]interface{}, error) {
	quota := make(map[string]interface{})

	cmd := exec.Command("kubectl", "get", "resourcequota", "-n", namespace, "-o", "json")
	output, err := cmd.Output()
	if err != nil {
		return quota, err
	}

	// For now, just indicate if quota exists
	quota["has_quota"] = len(output) > 0

	return quota, nil
}

// DiagnosticCheck runs a specific diagnostic check
func (c *Collector) RunDiagnosticCheck(name string, checkFunc func() error) errors.DiagnosticCheck {
	check := errors.DiagnosticCheck{
		Name: name,
	}

	err := checkFunc()
	if err != nil {
		check.Status = "fail"
		check.Details = fmt.Sprintf("Check failed: %v", err)
	} else {
		check.Status = "pass"
		check.Details = "Check passed"
	}

	return check
}

// CollectLogs collects recent relevant logs
func (c *Collector) CollectLogs(component string, lines int) []errors.LogEntry {
	logs := make([]errors.LogEntry, 0)

	// Try to get logs from the global log buffer first
	if globalBuffer := c.getGlobalLogBuffer(); globalBuffer != nil {
		utilsLogs := c.extractRecentLogs(globalBuffer, component, lines)
		// Convert utils.LogEntry to errors.LogEntry
		for _, utilsLog := range utilsLogs {
			logs = append(logs, errors.LogEntry{
				Timestamp: utilsLog.Timestamp,
				Level:     utilsLog.Level,
				Source:    c.extractSource(utilsLog, component),
				Message:   utilsLog.Message,
			})
		}
	}

	// If we got logs from the buffer, return them
	if len(logs) > 0 {
		c.logger.Debug().
			Str("component", component).
			Int("count", len(logs)).
			Msg("Collected logs from global buffer")
		return logs
	}

	// Fallback: try to collect from system logs (docker, kubectl, etc.)
	systemLogs := c.collectSystemLogs(component, lines)
	logs = append(logs, systemLogs...)

	// If still no logs, provide helpful debug information
	if len(logs) == 0 {
		logs = append(logs, errors.LogEntry{
			Timestamp: time.Now(),
			Level:     "info",
			Source:    component,
			Message:   fmt.Sprintf("No recent logs found for component '%s' - this may indicate the component is not actively logging or log capture is not configured", component),
		})
	}

	c.logger.Debug().
		Str("component", component).
		Int("requested_lines", lines).
		Int("collected_count", len(logs)).
		Msg("Log collection completed")

	return logs
}

// getGlobalLogBuffer retrieves the global log buffer if available
func (c *Collector) getGlobalLogBuffer() *utils.RingBuffer {
	return utils.GetGlobalLogBuffer()
}

// extractRecentLogs extracts recent logs from the ring buffer, optionally filtered by component
func (c *Collector) extractRecentLogs(buffer *utils.RingBuffer, component string, lines int) []utils.LogEntry {
	allLogs := buffer.GetEntries()

	// Filter by component if specified
	var filteredLogs []utils.LogEntry
	for _, log := range allLogs {
		if component == "" || c.logMatchesSource(log, component) {
			filteredLogs = append(filteredLogs, log)
		}
	}

	// Sort by timestamp (most recent first) and limit
	if len(filteredLogs) > lines {
		filteredLogs = filteredLogs[len(filteredLogs)-lines:]
	}

	return filteredLogs
}

// logMatchesSource checks if a log entry matches the requested component
func (c *Collector) logMatchesSource(log utils.LogEntry, component string) bool {
	// Check if component name appears in log fields or message
	if log.Fields != nil {
		if comp, exists := log.Fields["component"]; exists {
			if compStr, ok := comp.(string); ok && strings.Contains(compStr, component) {
				return true
			}
		}
	}

	// Check if component name appears in the message
	return strings.Contains(strings.ToLower(log.Message), strings.ToLower(component))
}

// extractSource extracts or derives the component name from a log entry
func (c *Collector) extractSource(log utils.LogEntry, requestedSource string) string {
	// Try to get component from log fields first
	if log.Fields != nil {
		if comp, exists := log.Fields["component"]; exists {
			if compStr, ok := comp.(string); ok {
				return compStr
			}
		}
	}

	// Fallback to requested component
	if requestedSource != "" {
		return requestedSource
	}

	// Default to "unknown"
	return "unknown"
}

// collectSystemLogs attempts to collect logs from system sources
func (c *Collector) collectSystemLogs(component string, lines int) []errors.LogEntry {
	var logs []errors.LogEntry

	// Try docker logs if component suggests it's a container
	if strings.Contains(strings.ToLower(component), "docker") || strings.Contains(strings.ToLower(component), "container") {
		dockerLogs := c.collectDockerLogs(lines)
		logs = append(logs, dockerLogs...)
	}

	// Try kubectl logs if component suggests it's kubernetes-related
	if strings.Contains(strings.ToLower(component), "k8s") || strings.Contains(strings.ToLower(component), "kubernetes") {
		k8sLogs := c.collectK8sLogs(lines)
		logs = append(logs, k8sLogs...)
	}

	return logs
}

// collectDockerLogs collects recent docker system logs
func (c *Collector) collectDockerLogs(lines int) []errors.LogEntry {
	var logs []errors.LogEntry

	// Try to get docker system events
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "system", "events", "--since", "5m", "--format", "{{.Time}}: {{.Type}} {{.Action}} {{.Actor.ID}}")
	output, err := cmd.Output()
	if err != nil {
		c.logger.Debug().Err(err).Msg("Failed to collect docker logs")
		return logs
	}

	// Parse docker events into log entries
	lines_text := strings.Split(strings.TrimSpace(string(output)), "\n")
	for i, line := range lines_text {
		if i >= lines || line == "" {
			break
		}

		logs = append(logs, errors.LogEntry{
			Timestamp: time.Now(),
			Level:     "info",
			Source:    "docker",
			Message:   line,
		})
	}

	return logs
}

// collectK8sLogs collects recent kubernetes-related logs
func (c *Collector) collectK8sLogs(lines int) []errors.LogEntry {
	var logs []errors.LogEntry

	// Try to get kubernetes events
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "kubectl", "get", "events", "--sort-by='.lastTimestamp'", "--no-headers", "-o", "custom-columns=TIME:.lastTimestamp,TYPE:.type,REASON:.reason,MESSAGE:.message")
	output, err := cmd.Output()
	if err != nil {
		c.logger.Debug().Err(err).Msg("Failed to collect kubernetes logs")
		return logs
	}

	// Parse kubectl events into log entries
	lines_text := strings.Split(strings.TrimSpace(string(output)), "\n")
	for i, line := range lines_text {
		if i >= lines || line == "" {
			break
		}

		logs = append(logs, errors.LogEntry{
			Timestamp: time.Now(),
			Level:     "info",
			Source:    "kubernetes",
			Message:   line,
		})
	}

	return logs
}
