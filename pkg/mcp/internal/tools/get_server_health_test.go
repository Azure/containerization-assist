package tools

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockHealthChecker for testing
type MockHealthChecker struct {
	startTime               time.Time
	systemResourcesFunc     func() SystemResources
	sessionStatsFunc        func() SessionHealthStats
	circuitBreakerStatsFunc func() map[string]CircuitBreakerStatus
	checkServiceHealthFunc  func(ctx context.Context) []ServiceHealth
	jobQueueStatsFunc       func() JobQueueStats
	recentErrorsFunc        func(limit int) []RecentError
}

func NewMockHealthChecker() *MockHealthChecker {
	return &MockHealthChecker{
		startTime: time.Now(),
	}
}

func (m *MockHealthChecker) GetSystemResources() SystemResources {
	if m.systemResourcesFunc != nil {
		return m.systemResourcesFunc()
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return SystemResources{
		CPUCount:       runtime.NumCPU(),
		MemoryTotal:    memStats.Sys,
		MemoryUsed:     memStats.Alloc,
		MemoryPercent:  float64(memStats.Alloc) / float64(memStats.Sys) * 100,
		GoroutineCount: runtime.NumGoroutine(),
		DiskTotal:      10 * 1024 * 1024 * 1024, // 10GB mock
		DiskUsed:       2 * 1024 * 1024 * 1024,  // 2GB mock
		DiskPercent:    20.0,
	}
}

func (m *MockHealthChecker) GetSessionStats() SessionHealthStats {
	if m.sessionStatsFunc != nil {
		return m.sessionStatsFunc()
	}

	return SessionHealthStats{
		ActiveSessions:  3,
		TotalSessions:   10,
		MaxSessions:     50,
		SessionsPercent: 6.0,
		TotalDiskUsed:   1024 * 1024 * 500,       // 500MB
		DiskQuota:       10 * 1024 * 1024 * 1024, // 10GB
		DiskUsedPercent: 5.0,
	}
}

func (m *MockHealthChecker) GetCircuitBreakerStats() map[string]CircuitBreakerStatus {
	if m.circuitBreakerStatsFunc != nil {
		return m.circuitBreakerStatsFunc()
	}

	return map[string]CircuitBreakerStatus{
		"docker": {
			Name:      "docker",
			State:     "closed",
			Failures:  0,
			Successes: 100,
		},
		"kubernetes": {
			Name:      "kubernetes",
			State:     "closed",
			Failures:  1,
			Successes: 50,
		},
		"registry": {
			Name:      "registry",
			State:     "closed",
			Failures:  0,
			Successes: 25,
		},
	}
}

func (m *MockHealthChecker) CheckServiceHealth(ctx context.Context) []ServiceHealth {
	if m.checkServiceHealthFunc != nil {
		return m.checkServiceHealthFunc(ctx)
	}

	return []ServiceHealth{
		{
			Name:      "docker",
			Status:    "healthy",
			LastCheck: time.Now(),
		},
		{
			Name:      "kubernetes",
			Status:    "healthy",
			LastCheck: time.Now(),
		},
		{
			Name:      "registry",
			Status:    "healthy",
			LastCheck: time.Now(),
		},
	}
}

func (m *MockHealthChecker) GetJobQueueStats() JobQueueStats {
	return JobQueueStats{
		QueueDepth:      5,
		ProcessingRate:  12.5,
		ActiveWorkers:   3,
		CompletedJobs:   150,
		FailedJobs:      2,
		AverageWaitTime: "15s",
	}
}

func (m *MockHealthChecker) GetRecentErrors(limit int) []RecentError {
	return []RecentError{
		{
			Timestamp: time.Now().Add(-5 * time.Minute),
			Tool:      "build_image",
			Error:     "Docker daemon not responding",
			Count:     2,
		},
	}
}

func (m *MockHealthChecker) GetUptime() time.Duration {
	return time.Since(m.startTime)
}

func TestGetServerHealthTool_Execute(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("healthy server status", func(t *testing.T) {
		// Setup
		healthChecker := NewMockHealthChecker()
		tool := NewGetServerHealthTool(logger, healthChecker)

		// Execute
		args := GetServerHealthArgs{
			IncludeDetails: true,
		}
		result, err := tool.Execute(context.Background(), args)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "healthy", result.Status)
		assert.NotEmpty(t, result.Uptime)
		assert.Equal(t, 3, result.Sessions.ActiveSessions)
		assert.Equal(t, 50, result.Sessions.MaxSessions)
		assert.Len(t, result.CircuitBreakers, 3)
		assert.Len(t, result.Services, 3)
		assert.Equal(t, 5, result.JobQueue.QueueDepth)
		assert.Len(t, result.RecentErrors, 1)
		assert.Empty(t, result.Warnings)
	})

	t.Run("degraded server status - high memory", func(t *testing.T) {
		// Setup
		healthChecker := &MockHealthChecker{
			startTime: time.Now(),
			systemResourcesFunc: func() SystemResources {
				var memStats runtime.MemStats
				runtime.ReadMemStats(&memStats)
				return SystemResources{
					CPUCount:       runtime.NumCPU(),
					MemoryTotal:    memStats.Sys,
					MemoryUsed:     memStats.Alloc,
					MemoryPercent:  92.0, // High memory usage
					GoroutineCount: runtime.NumGoroutine(),
					DiskTotal:      10 * 1024 * 1024 * 1024,
					DiskUsed:       2 * 1024 * 1024 * 1024,
					DiskPercent:    20.0,
				}
			},
		}
		tool := NewGetServerHealthTool(logger, healthChecker)

		// Execute
		args := GetServerHealthArgs{}
		result, err := tool.Execute(context.Background(), args)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "degraded", result.Status)
		assert.Contains(t, result.Warnings, "High memory usage: 92.0%")
	})

	t.Run("degraded server status - circuit breaker open", func(t *testing.T) {
		// Setup
		healthChecker := &MockHealthChecker{
			startTime: time.Now(),
			circuitBreakerStatsFunc: func() map[string]CircuitBreakerStatus {
				return map[string]CircuitBreakerStatus{
					"docker": {
						Name:        "docker",
						State:       "open",
						Failures:    10,
						Successes:   0,
						LastFailure: time.Now(),
					},
				}
			},
		}
		tool := NewGetServerHealthTool(logger, healthChecker)

		// Execute
		args := GetServerHealthArgs{}
		result, err := tool.Execute(context.Background(), args)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "degraded", result.Status)
		assert.Contains(t, result.Warnings[0], "Circuit breaker docker is open")
	})

	t.Run("unhealthy server status - multiple issues", func(t *testing.T) {
		// Setup
		healthChecker := &MockHealthChecker{
			startTime: time.Now(),
			checkServiceHealthFunc: func(ctx context.Context) []ServiceHealth {
				return []ServiceHealth{
					{
						Name:      "docker",
						Status:    "unhealthy",
						Message:   "Docker daemon not responding",
						LastCheck: time.Now(),
					},
					{
						Name:      "kubernetes",
						Status:    "unhealthy",
						Message:   "Unable to connect to cluster",
						LastCheck: time.Now(),
					},
				}
			},
		}
		tool := NewGetServerHealthTool(logger, healthChecker)

		// Execute
		args := GetServerHealthArgs{}
		result, err := tool.Execute(context.Background(), args)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "unhealthy", result.Status)
		assert.Len(t, result.Warnings, 2)
	})

	t.Run("without detailed metrics", func(t *testing.T) {
		// Setup
		healthChecker := NewMockHealthChecker()
		tool := NewGetServerHealthTool(logger, healthChecker)

		// Execute
		args := GetServerHealthArgs{
			IncludeDetails: false,
		}
		result, err := tool.Execute(context.Background(), args)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "healthy", result.Status)
		assert.Empty(t, result.RecentErrors) // Should be empty when include_details is false
	})
}
