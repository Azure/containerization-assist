// Package observability provides unified monitoring, tracing, and health infrastructure
// for the MCP components. It consolidates telemetry, distributed tracing, health checks,
// and logging enrichment into a single coherent package.
package observability

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func createTestMonitor() *Monitor {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewMonitor(logger)
}

func TestNewMonitor(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	monitor := NewMonitor(logger)

	assert.NotNil(t, monitor)
	assert.NotNil(t, monitor.logger)
	assert.NotNil(t, monitor.checkers)
	assert.NotNil(t, monitor.results)
	assert.Equal(t, 0, len(monitor.checkers))
	assert.Equal(t, 0, len(monitor.results))
	assert.Equal(t, "0.0.6", monitor.version)
	assert.WithinDuration(t, time.Now(), monitor.startTime, time.Second)
}

func TestMonitor_SetVersion(t *testing.T) {
	monitor := createTestMonitor()

	monitor.SetVersion("1.2.3")
	assert.Equal(t, "1.2.3", monitor.version)

	report := monitor.GetReport()
	assert.Equal(t, "1.2.3", report.Version)
}

func TestMonitor_RegisterChecker(t *testing.T) {
	monitor := createTestMonitor()

	checker := NewBasicChecker("test-check", func(ctx context.Context) (Status, string, map[string]string) {
		return StatusHealthy, "OK", nil
	})

	monitor.RegisterChecker(checker)

	assert.Equal(t, 1, len(monitor.checkers))
	assert.Contains(t, monitor.checkers, "test-check")
	assert.Equal(t, checker, monitor.checkers["test-check"])
}

func TestMonitor_RegisterChecker_Multiple(t *testing.T) {
	monitor := createTestMonitor()

	checker1 := NewBasicChecker("check1", func(ctx context.Context) (Status, string, map[string]string) {
		return StatusHealthy, "OK", nil
	})
	checker2 := NewBasicChecker("check2", func(ctx context.Context) (Status, string, map[string]string) {
		return StatusDegraded, "Warning", nil
	})

	monitor.RegisterChecker(checker1)
	monitor.RegisterChecker(checker2)

	assert.Equal(t, 2, len(monitor.checkers))
	assert.Contains(t, monitor.checkers, "check1")
	assert.Contains(t, monitor.checkers, "check2")
}

func TestMonitor_UnregisterChecker(t *testing.T) {
	monitor := createTestMonitor()

	checker := NewBasicChecker("test-check", func(ctx context.Context) (Status, string, map[string]string) {
		return StatusHealthy, "OK", nil
	})

	monitor.RegisterChecker(checker)
	assert.Equal(t, 1, len(monitor.checkers))

	// Add a result to verify it gets cleaned up
	monitor.results["test-check"] = Check{Name: "test-check", Status: StatusHealthy}

	monitor.UnregisterChecker("test-check")
	assert.Equal(t, 0, len(monitor.checkers))
	assert.Equal(t, 0, len(monitor.results))
}

func TestMonitor_UnregisterChecker_NonExistent(t *testing.T) {
	monitor := createTestMonitor()

	// Should not panic or error
	monitor.UnregisterChecker("non-existent")
	assert.Equal(t, 0, len(monitor.checkers))
}

func TestMonitor_CheckAll_NoCheckers(t *testing.T) {
	monitor := createTestMonitor()

	monitor.CheckAll(context.Background())

	report := monitor.GetReport()
	assert.Equal(t, StatusHealthy, report.Status)
	assert.Equal(t, 0, len(report.Checks))
	assert.Equal(t, 0, report.Summary["total"])
}

func TestMonitor_CheckAll_SingleHealthyChecker(t *testing.T) {
	monitor := createTestMonitor()

	checker := NewBasicChecker("healthy-check", func(ctx context.Context) (Status, string, map[string]string) {
		return StatusHealthy, "All systems operational", map[string]string{"component": "database"}
	})

	monitor.RegisterChecker(checker)
	monitor.CheckAll(context.Background())

	report := monitor.GetReport()
	assert.Equal(t, StatusHealthy, report.Status)
	assert.Equal(t, 1, len(report.Checks))
	assert.Contains(t, report.Checks, "healthy-check")

	check := report.Checks["healthy-check"]
	assert.Equal(t, "healthy-check", check.Name)
	assert.Equal(t, StatusHealthy, check.Status)
	assert.Equal(t, "All systems operational", check.Message)
	assert.Equal(t, "database", check.Details["component"])
	assert.WithinDuration(t, time.Now(), check.LastChecked, time.Second)
	assert.Greater(t, check.ResponseTime.Nanoseconds(), int64(0))
}

func TestMonitor_CheckAll_MultipleCheckers(t *testing.T) {
	monitor := createTestMonitor()

	healthyChecker := NewBasicChecker("healthy", func(ctx context.Context) (Status, string, map[string]string) {
		return StatusHealthy, "OK", nil
	})
	degradedChecker := NewBasicChecker("degraded", func(ctx context.Context) (Status, string, map[string]string) {
		return StatusDegraded, "Slow response", nil
	})
	unhealthyChecker := NewBasicChecker("unhealthy", func(ctx context.Context) (Status, string, map[string]string) {
		return StatusUnhealthy, "Service unavailable", nil
	})

	monitor.RegisterChecker(healthyChecker)
	monitor.RegisterChecker(degradedChecker)
	monitor.RegisterChecker(unhealthyChecker)

	monitor.CheckAll(context.Background())

	report := monitor.GetReport()
	assert.Equal(t, StatusUnhealthy, report.Status) // Overall status should be unhealthy
	assert.Equal(t, 3, len(report.Checks))
	assert.Equal(t, 3, report.Summary["total"])
	assert.Equal(t, 1, report.Summary["healthy"])
	assert.Equal(t, 1, report.Summary["degraded"])
	assert.Equal(t, 1, report.Summary["unhealthy"])
}

func TestMonitor_CheckAll_WithTimeout(t *testing.T) {
	monitor := createTestMonitor()

	// Checker that takes longer than the default timeout
	slowChecker := NewBasicChecker("slow", func(ctx context.Context) (Status, string, map[string]string) {
		select {
		case <-ctx.Done():
			return StatusUnhealthy, "Timed out", nil
		case <-time.After(15 * time.Second): // Longer than 10s timeout
			return StatusHealthy, "Finally done", nil
		}
	})

	monitor.RegisterChecker(slowChecker)

	start := time.Now()
	monitor.CheckAll(context.Background())
	elapsed := time.Since(start)

	// Should complete in reasonable time due to timeout
	assert.Less(t, elapsed, 12*time.Second)

	report := monitor.GetReport()
	check := report.Checks["slow"]
	assert.Equal(t, StatusUnhealthy, check.Status)
	assert.Equal(t, "Timed out", check.Message)
}

func TestMonitor_CheckAll_ConcurrentExecution(t *testing.T) {
	monitor := createTestMonitor()

	// Create multiple checkers that track execution order
	var executionOrder []string
	var orderMutex sync.Mutex

	for i := 0; i < 5; i++ {
		name := string(rune('A' + i))
		checker := NewBasicChecker(name, func(ctx context.Context) (Status, string, map[string]string) {
			// Small delay to increase chance of concurrent execution
			time.Sleep(10 * time.Millisecond)

			orderMutex.Lock()
			executionOrder = append(executionOrder, name)
			orderMutex.Unlock()

			return StatusHealthy, "OK", nil
		})
		monitor.RegisterChecker(checker)
	}

	monitor.CheckAll(context.Background())

	// Verify all checkers executed
	assert.Equal(t, 5, len(executionOrder))

	// Verify all results were recorded
	report := monitor.GetReport()
	assert.Equal(t, 5, len(report.Checks))
}

func TestMonitor_GetReport_NoChecks(t *testing.T) {
	monitor := createTestMonitor()

	report := monitor.GetReport()

	assert.Equal(t, StatusHealthy, report.Status)
	assert.WithinDuration(t, time.Now(), report.Timestamp, time.Second)
	assert.Greater(t, report.Uptime.Nanoseconds(), int64(0))
	assert.Equal(t, "0.0.6", report.Version)
	assert.Equal(t, 0, len(report.Checks))
	assert.Equal(t, 0, report.Summary["total"])
	assert.Equal(t, 0, report.Summary["healthy"])
	assert.Equal(t, 0, report.Summary["degraded"])
	assert.Equal(t, 0, report.Summary["unhealthy"])
	assert.Equal(t, "container-kit-mcp", report.Details["service"])
}

func TestMonitor_GetReport_StatusPriority(t *testing.T) {
	tests := []struct {
		name           string
		checkStatuses  []Status
		expectedStatus Status
	}{
		{"all healthy", []Status{StatusHealthy, StatusHealthy}, StatusHealthy},
		{"healthy and degraded", []Status{StatusHealthy, StatusDegraded}, StatusDegraded},
		{"all degraded", []Status{StatusDegraded, StatusDegraded}, StatusDegraded},
		{"degraded and unhealthy", []Status{StatusDegraded, StatusUnhealthy}, StatusUnhealthy},
		{"all unhealthy", []Status{StatusUnhealthy, StatusUnhealthy}, StatusUnhealthy},
		{"mixed all", []Status{StatusHealthy, StatusDegraded, StatusUnhealthy}, StatusUnhealthy},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			monitor := createTestMonitor()

			for i, status := range test.checkStatuses {
				name := string(rune('A' + i))
				checker := NewBasicChecker(name, func(s Status) func(ctx context.Context) (Status, string, map[string]string) {
					return func(ctx context.Context) (Status, string, map[string]string) {
						return s, "test", nil
					}
				}(status))
				monitor.RegisterChecker(checker)
			}

			monitor.CheckAll(context.Background())
			report := monitor.GetReport()

			assert.Equal(t, test.expectedStatus, report.Status)
		})
	}
}

func TestMonitor_IsHealthy(t *testing.T) {
	monitor := createTestMonitor()

	// Initially healthy (no checks)
	assert.True(t, monitor.IsHealthy())

	// Add unhealthy check
	unhealthyChecker := NewBasicChecker("unhealthy", func(ctx context.Context) (Status, string, map[string]string) {
		return StatusUnhealthy, "Down", nil
	})
	monitor.RegisterChecker(unhealthyChecker)
	monitor.CheckAll(context.Background())

	assert.False(t, monitor.IsHealthy())

	// Replace with degraded check
	monitor.UnregisterChecker("unhealthy")
	degradedChecker := NewBasicChecker("degraded", func(ctx context.Context) (Status, string, map[string]string) {
		return StatusDegraded, "Slow", nil
	})
	monitor.RegisterChecker(degradedChecker)
	monitor.CheckAll(context.Background())

	assert.True(t, monitor.IsHealthy()) // Degraded is considered healthy for IsHealthy()
}

func TestMonitor_IsReady(t *testing.T) {
	monitor := createTestMonitor()

	// Initially ready (no checks)
	assert.True(t, monitor.IsReady())

	// Add degraded check
	degradedChecker := NewBasicChecker("degraded", func(ctx context.Context) (Status, string, map[string]string) {
		return StatusDegraded, "Slow", nil
	})
	monitor.RegisterChecker(degradedChecker)
	monitor.CheckAll(context.Background())

	assert.False(t, monitor.IsReady()) // Degraded is not ready

	// Replace with healthy check
	monitor.UnregisterChecker("degraded")
	healthyChecker := NewBasicChecker("healthy", func(ctx context.Context) (Status, string, map[string]string) {
		return StatusHealthy, "OK", nil
	})
	monitor.RegisterChecker(healthyChecker)
	monitor.CheckAll(context.Background())

	assert.True(t, monitor.IsReady())
}

func TestBasicChecker_Creation(t *testing.T) {
	checkFn := func(ctx context.Context) (Status, string, map[string]string) {
		return StatusHealthy, "test message", map[string]string{"key": "value"}
	}

	checker := NewBasicChecker("test-checker", checkFn)

	assert.NotNil(t, checker)
	assert.Equal(t, "test-checker", checker.Name())
	assert.NotNil(t, checker.checkFn)
}

func TestBasicChecker_Check(t *testing.T) {
	expectedStatus := StatusDegraded
	expectedMessage := "System is slow"
	expectedDetails := map[string]string{"cpu": "80%", "memory": "60%"}

	checkFn := func(ctx context.Context) (Status, string, map[string]string) {
		return expectedStatus, expectedMessage, expectedDetails
	}

	checker := NewBasicChecker("performance-check", checkFn)
	result := checker.Check(context.Background())

	assert.Equal(t, "performance-check", result.Name)
	assert.Equal(t, expectedStatus, result.Status)
	assert.Equal(t, expectedMessage, result.Message)
	assert.Equal(t, expectedDetails, result.Details)
}

func TestBasicChecker_CheckWithContext(t *testing.T) {
	checker := NewBasicChecker("context-check", func(ctx context.Context) (Status, string, map[string]string) {
		select {
		case <-ctx.Done():
			return StatusUnhealthy, "Context cancelled", nil
		default:
			return StatusHealthy, "OK", nil
		}
	})

	// Test with normal context
	result := checker.Check(context.Background())
	assert.Equal(t, StatusHealthy, result.Status)

	// Test with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result = checker.Check(ctx)
	assert.Equal(t, StatusUnhealthy, result.Status)
	assert.Equal(t, "Context cancelled", result.Message)
}

func TestMonitor_ThreadSafety(t *testing.T) {
	monitor := createTestMonitor()

	// Test concurrent operations
	const numGoroutines = 10
	var wg sync.WaitGroup

	// Concurrent registration/unregistration
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			name := string(rune('A' + id))
			checker := NewBasicChecker(name, func(ctx context.Context) (Status, string, map[string]string) {
				return StatusHealthy, "OK", nil
			})

			monitor.RegisterChecker(checker)
			monitor.CheckAll(context.Background())
			_ = monitor.GetReport()
			_ = monitor.IsHealthy()
			_ = monitor.IsReady()
			monitor.UnregisterChecker(name)
		}(i)
	}

	wg.Wait()

	// Should not have panicked and should be in clean state
	assert.Equal(t, 0, len(monitor.checkers))
	assert.True(t, monitor.IsHealthy())
}
