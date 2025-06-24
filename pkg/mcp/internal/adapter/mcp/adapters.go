package mcp

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/adapter"
	"github.com/Azure/container-copilot/pkg/mcp/internal/store"
	"github.com/Azure/container-copilot/pkg/mcp/internal/tools"
)

// WorkspaceManagerAdapter adapts WorkspaceManager to tools interfaces
type WorkspaceManagerAdapter struct {
	*store.WorkspaceManager
}

// GetWorkspacePath returns the workspace path for a session
func (a *WorkspaceManagerAdapter) GetWorkspacePath(sessionID string) string {
	return filepath.Join(a.GetBaseDir(), sessionID)
}

// DeleteWorkspace deletes a session's workspace
func (a *WorkspaceManagerAdapter) DeleteWorkspace(sessionID string) error {
	return a.CleanupWorkspace(sessionID)
}

// GetWorkspaceSize returns the size of a session's workspace
func (a *WorkspaceManagerAdapter) GetWorkspaceSize(sessionID string) (int64, error) {
	path := a.GetWorkspacePath(sessionID)

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

// ServerHealthAdapter makes Server implement tools.HealthChecker
type ServerHealthAdapter struct {
	server adapter.ServerInterface
}

// NewServerHealthAdapter creates a new ServerHealthAdapter
func NewServerHealthAdapter(server adapter.ServerInterface) *ServerHealthAdapter {
	return &ServerHealthAdapter{
		server: server,
	}
}

// GetSystemResources returns current system resource usage
func (a *ServerHealthAdapter) GetSystemResources() tools.SystemResources {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Get workspace stats for disk usage
	wsStats := a.server.GetWorkspaceStats()
	config := a.server.GetConfig()

	return tools.SystemResources{
		CPUCount:       runtime.NumCPU(),
		MemoryTotal:    memStats.Sys,
		MemoryUsed:     memStats.Alloc,
		MemoryPercent:  float64(memStats.Alloc) / float64(memStats.Sys) * 100,
		GoroutineCount: runtime.NumGoroutine(),
		DiskTotal:      uint64(config.TotalDiskLimit),
		DiskUsed:       uint64(wsStats.TotalDiskUsage),
		DiskPercent:    float64(wsStats.TotalDiskUsage) / float64(config.TotalDiskLimit) * 100,
	}
}

// GetSessionStats returns session-related statistics
func (a *ServerHealthAdapter) GetSessionStats() tools.SessionHealthStats {
	stats := a.server.GetSessionManagerStats()
	wsStats := a.server.GetWorkspaceStats()
	config := a.server.GetConfig()

	maxSessions := 10

	sessionPercent := float64(stats.ActiveSessions) / float64(maxSessions) * 100
	diskPercent := float64(wsStats.TotalDiskUsage) / float64(config.TotalDiskLimit) * 100

	return tools.SessionHealthStats{
		ActiveSessions:  stats.ActiveSessions,
		TotalSessions:   stats.TotalSessions,
		MaxSessions:     maxSessions,
		SessionsPercent: sessionPercent,
		TotalDiskUsed:   wsStats.TotalDiskUsage,
		DiskQuota:       config.TotalDiskLimit,
		DiskUsedPercent: diskPercent,
	}
}

// GetCircuitBreakerStats returns circuit breaker statistics
func (a *ServerHealthAdapter) GetCircuitBreakerStats() map[string]tools.CircuitBreakerStatus {
	cbStats := a.server.GetCircuitBreakerStats()

	result := make(map[string]tools.CircuitBreakerStatus)
	for name, stats := range cbStats {
		cbStatus := tools.CircuitBreakerStatus{
			Name:      name,
			State:     stats.State,
			Failures:  stats.FailureCount,
			Successes: stats.SuccessCount,
		}
		if stats.LastFailure != nil {
			cbStatus.LastFailure = *stats.LastFailure
		}
		result[name] = cbStatus
	}

	return result
}

// CheckServiceHealth checks the health of external services
func (a *ServerHealthAdapter) CheckServiceHealth(ctx context.Context) []tools.ServiceHealth {
	services := []tools.ServiceHealth{}

	// Check Docker
	dockerHealth := tools.ServiceHealth{
		Name:      "docker",
		Status:    "healthy",
		LastCheck: time.Now(),
	}
	// In real implementation, would check Docker daemon
	services = append(services, dockerHealth)

	// Check Kubernetes (Kind)
	k8sHealth := tools.ServiceHealth{
		Name:      "kubernetes",
		Status:    "healthy",
		LastCheck: time.Now(),
	}
	// In real implementation, would check Kind cluster
	services = append(services, k8sHealth)

	// Check Registry
	registryHealth := tools.ServiceHealth{
		Name:      "registry",
		Status:    "healthy",
		LastCheck: time.Now(),
	}
	// In real implementation, would check registry connectivity
	services = append(services, registryHealth)

	return services
}

// GetJobQueueStats returns job queue statistics
func (a *ServerHealthAdapter) GetJobQueueStats() tools.JobQueueStats {
	// In real implementation, would get from job manager
	return tools.JobQueueStats{
		QueueDepth:      0,
		ProcessingRate:  0,
		ActiveWorkers:   5, // Default max workers
		CompletedJobs:   0,
		FailedJobs:      0,
		AverageWaitTime: "0s",
	}
}

// GetRecentErrors returns recent errors
func (a *ServerHealthAdapter) GetRecentErrors(limit int) []tools.RecentError {
	// In real implementation, would track errors
	return []tools.RecentError{}
}

// GetUptime returns server uptime
func (a *ServerHealthAdapter) GetUptime() time.Duration {
	return time.Since(a.server.GetStartTime())
}
