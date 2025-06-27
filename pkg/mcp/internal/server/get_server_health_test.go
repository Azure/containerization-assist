package server

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
	"github.com/rs/zerolog"
)

// MockHealthChecker for testing
type MockHealthChecker struct{}

func (m *MockHealthChecker) GetSystemResources() mcptypes.SystemResources {
	return mcptypes.SystemResources{
		CPUUsage:    25.5,
		MemoryUsage: 60.0,
		DiskUsage:   45.0,
		OpenFiles:   150,
		GoRoutines:  25,
		HeapSize:    1024000,
		LastUpdated: time.Now(),
	}
}

func (m *MockHealthChecker) GetSessionStats() mcptypes.SessionHealthStats {
	return mcptypes.SessionHealthStats{
		ActiveSessions:    5,
		TotalSessions:     25,
		FailedSessions:    2,
		AverageSessionAge: 45.0,
		SessionErrors:     1,
	}
}

func (m *MockHealthChecker) GetCircuitBreakerStats() map[string]mcptypes.CircuitBreakerStatus {
	return map[string]mcptypes.CircuitBreakerStatus{
		"registry": {
			State:         "closed",
			FailureCount:  0,
			LastFailure:   time.Time{},
			NextRetry:     time.Time{},
			TotalRequests: 100,
			SuccessCount:  100,
		},
		"database": {
			State:         "open",
			FailureCount:  5,
			LastFailure:   time.Now().Add(-5 * time.Minute),
			NextRetry:     time.Now().Add(30 * time.Second),
			TotalRequests: 50,
			SuccessCount:  45,
		},
	}
}

func (m *MockHealthChecker) CheckServiceHealth(ctx context.Context) []mcptypes.ServiceHealth {
	return []mcptypes.ServiceHealth{
		{
			Name:         "docker",
			Status:       "healthy",
			ResponseTime: 50 * time.Millisecond,
			LastCheck:    time.Now(),
		},
		{
			Name:         "kubernetes",
			Status:       "degraded",
			ResponseTime: 200 * time.Millisecond,
			LastCheck:    time.Now(),
		},
	}
}

func (m *MockHealthChecker) GetJobQueueStats() mcptypes.JobQueueStats {
	return mcptypes.JobQueueStats{
		QueuedJobs:      3,
		RunningJobs:     2,
		CompletedJobs:   15,
		FailedJobs:      1,
		AverageWaitTime: 2.5,
	}
}

func (m *MockHealthChecker) GetRecentErrors(limit int) []mcptypes.RecentError {
	return []mcptypes.RecentError{
		{
			Timestamp: time.Now().Add(-5 * time.Minute),
			Message:   "Connection timeout to registry",
			Component: "registry",
			Severity:  "error",
		},
		{
			Timestamp: time.Now().Add(-10 * time.Minute),
			Message:   "Slow response from kubernetes API",
			Component: "kubernetes",
			Severity:  "warning",
		},
	}
}

func (m *MockHealthChecker) GetUptime() time.Duration {
	return 24*time.Hour + 30*time.Minute
}

// Test GetServerHealthArgs type
func TestGetServerHealthArgs(t *testing.T) {
	args := GetServerHealthArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: "health-session-123",
			DryRun:    false,
		},
		IncludeDetails: true,
	}

	if args.SessionID != "health-session-123" {
		t.Errorf("Expected SessionID to be 'health-session-123', got '%s'", args.SessionID)
	}
	if args.DryRun {
		t.Error("Expected DryRun to be false")
	}
	if !args.IncludeDetails {
		t.Error("Expected IncludeDetails to be true")
	}
}

// Test GetServerHealthResult type
func TestGetServerHealthResult(t *testing.T) {
	sysRes := mcptypes.SystemResources{
		CPUUsage:    30.0,
		MemoryUsage: 70.0,
		OpenFiles:   100,
		GoRoutines:  20,
	}

	sessionStats := mcptypes.SessionHealthStats{
		ActiveSessions: 10,
		TotalSessions:  50,
	}

	result := GetServerHealthResult{
		BaseToolResponse: types.BaseToolResponse{
			SessionID: "health-result-456",
			Tool:      "get_server_health",
		},
		Status:          "healthy",
		Uptime:          "24h30m",
		SystemResources: sysRes,
		Sessions:        sessionStats,
		CircuitBreakers: map[string]mcptypes.CircuitBreakerStatus{
			"test": {State: "closed", FailureCount: 0},
		},
		Services: []mcptypes.ServiceHealth{
			{Name: "test-service", Status: "healthy"},
		},
		JobQueue: mcptypes.JobQueueStats{
			QueuedJobs:  2,
			RunningJobs: 1,
		},
		Warnings: []string{"Low disk space"},
	}

	if result.SessionID != "health-result-456" {
		t.Errorf("Expected SessionID to be 'health-result-456', got '%s'", result.SessionID)
	}
	if result.Tool != "get_server_health" {
		t.Errorf("Expected Tool to be 'get_server_health', got '%s'", result.Tool)
	}
	if result.Status != "healthy" {
		t.Errorf("Expected Status to be 'healthy', got '%s'", result.Status)
	}
	if result.Uptime != "24h30m" {
		t.Errorf("Expected Uptime to be '24h30m', got '%s'", result.Uptime)
	}
	if result.SystemResources.CPUUsage != 30.0 {
		t.Errorf("Expected CPU usage to be 30.0, got %f", result.SystemResources.CPUUsage)
	}
	if result.Sessions.ActiveSessions != 10 {
		t.Errorf("Expected 10 active sessions, got %d", result.Sessions.ActiveSessions)
	}
	if len(result.CircuitBreakers) != 1 {
		t.Errorf("Expected 1 circuit breaker, got %d", len(result.CircuitBreakers))
	}
	if len(result.Services) != 1 {
		t.Errorf("Expected 1 service, got %d", len(result.Services))
	}
	if result.JobQueue.QueuedJobs != 2 {
		t.Errorf("Expected 2 queued jobs, got %d", result.JobQueue.QueuedJobs)
	}
	if len(result.Warnings) != 1 {
		t.Errorf("Expected 1 warning, got %d", len(result.Warnings))
	}
}

// Test MockHealthChecker methods
func TestMockHealthChecker_GetSystemResources(t *testing.T) {
	checker := &MockHealthChecker{}
	resources := checker.GetSystemResources()

	if resources.CPUUsage != 25.5 {
		t.Errorf("Expected CPU usage to be 25.5, got %f", resources.CPUUsage)
	}
	if resources.MemoryUsage != 60.0 {
		t.Errorf("Expected memory usage to be 60.0, got %f", resources.MemoryUsage)
	}
	if resources.OpenFiles != 150 {
		t.Errorf("Expected open files to be 150, got %d", resources.OpenFiles)
	}
}

func TestMockHealthChecker_GetSessionStats(t *testing.T) {
	checker := &MockHealthChecker{}
	stats := checker.GetSessionStats()

	if stats.ActiveSessions != 5 {
		t.Errorf("Expected 5 active sessions, got %d", stats.ActiveSessions)
	}
	if stats.TotalSessions != 25 {
		t.Errorf("Expected 25 total sessions, got %d", stats.TotalSessions)
	}
	if stats.SessionErrors != 1 {
		t.Errorf("Expected session errors to be 1, got %d", stats.SessionErrors)
	}
}

func TestMockHealthChecker_GetCircuitBreakerStats(t *testing.T) {
	checker := &MockHealthChecker{}
	breakers := checker.GetCircuitBreakerStats()

	if len(breakers) != 2 {
		t.Errorf("Expected 2 circuit breakers, got %d", len(breakers))
	}

	registry, exists := breakers["registry"]
	if !exists {
		t.Error("Expected registry circuit breaker to exist")
	}
	if registry.State != "closed" {
		t.Errorf("Expected registry state to be 'closed', got '%s'", registry.State)
	}

	database, exists := breakers["database"]
	if !exists {
		t.Error("Expected database circuit breaker to exist")
	}
	if database.State != "open" {
		t.Errorf("Expected database state to be 'open', got '%s'", database.State)
	}
	if database.FailureCount != 5 {
		t.Errorf("Expected 5 failures, got %d", database.FailureCount)
	}
}

func TestMockHealthChecker_CheckServiceHealth(t *testing.T) {
	checker := &MockHealthChecker{}
	services := checker.CheckServiceHealth(context.Background())

	if len(services) != 2 {
		t.Errorf("Expected 2 services, got %d", len(services))
	}

	if services[0].Name != "docker" {
		t.Errorf("Expected first service to be 'docker', got '%s'", services[0].Name)
	}
	if services[0].Status != "healthy" {
		t.Errorf("Expected docker status to be 'healthy', got '%s'", services[0].Status)
	}

	if services[1].Name != "kubernetes" {
		t.Errorf("Expected second service to be 'kubernetes', got '%s'", services[1].Name)
	}
	if services[1].Status != "degraded" {
		t.Errorf("Expected kubernetes status to be 'degraded', got '%s'", services[1].Status)
	}
}

func TestMockHealthChecker_GetJobQueueStats(t *testing.T) {
	checker := &MockHealthChecker{}
	stats := checker.GetJobQueueStats()

	if stats.QueuedJobs != 3 {
		t.Errorf("Expected 3 queued jobs, got %d", stats.QueuedJobs)
	}
	if stats.RunningJobs != 2 {
		t.Errorf("Expected 2 running jobs, got %d", stats.RunningJobs)
	}
	if stats.CompletedJobs != 15 {
		t.Errorf("Expected 15 completed jobs, got %d", stats.CompletedJobs)
	}
	if stats.FailedJobs != 1 {
		t.Errorf("Expected 1 failed job, got %d", stats.FailedJobs)
	}
	if stats.AverageWaitTime != 2.5 {
		t.Errorf("Expected average wait time to be 2.5, got %f", stats.AverageWaitTime)
	}
}

func TestMockHealthChecker_GetRecentErrors(t *testing.T) {
	checker := &MockHealthChecker{}
	errors := checker.GetRecentErrors(5)

	if len(errors) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(errors))
	}

	if errors[0].Component != "registry" {
		t.Errorf("Expected first error component to be 'registry', got '%s'", errors[0].Component)
	}
	if errors[0].Severity != "error" {
		t.Errorf("Expected first error severity to be 'error', got '%s'", errors[0].Severity)
	}

	if errors[1].Component != "kubernetes" {
		t.Errorf("Expected second error component to be 'kubernetes', got '%s'", errors[1].Component)
	}
	if errors[1].Severity != "warning" {
		t.Errorf("Expected second error severity to be 'warning', got '%s'", errors[1].Severity)
	}
}

func TestMockHealthChecker_GetUptime(t *testing.T) {
	checker := &MockHealthChecker{}
	uptime := checker.GetUptime()

	expected := 24*time.Hour + 30*time.Minute
	if uptime != expected {
		t.Errorf("Expected uptime to be %v, got %v", expected, uptime)
	}
}

// Test NewGetServerHealthTool constructor
func TestNewGetServerHealthTool(t *testing.T) {
	logger := zerolog.Nop()
	checker := &MockHealthChecker{}

	tool := NewGetServerHealthTool(logger, checker)

	if tool == nil {
		t.Error("NewGetServerHealthTool should not return nil")
		return
	}
	if tool.healthChecker != checker {
		t.Error("Expected healthChecker to be set correctly")
	}
}

// Test GetServerHealthTool Execute with valid args
func TestGetServerHealthTool_Execute_ValidArgs(t *testing.T) {
	logger := zerolog.Nop()
	checker := &MockHealthChecker{}
	tool := NewGetServerHealthTool(logger, checker)

	args := GetServerHealthArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: "test-health-session",
		},
		IncludeDetails: true,
	}

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Errorf("Execute should not return error, got %v", err)
	}
	if result == nil {
		t.Error("Execute should return result")
	}
}

// Test GetServerHealthTool Execute with invalid args
func TestGetServerHealthTool_Execute_InvalidArgs(t *testing.T) {
	logger := zerolog.Nop()
	checker := &MockHealthChecker{}
	tool := NewGetServerHealthTool(logger, checker)

	// Invalid args type
	result, err := tool.Execute(context.Background(), "invalid")
	if err == nil {
		t.Error("Execute should return error for invalid args type")
	}
	if result != nil {
		t.Error("Execute should not return result for invalid args")
	}
}

// Test GetServerHealthTool struct initialization
func TestGetServerHealthToolStruct(t *testing.T) {
	logger := zerolog.Nop()
	checker := &MockHealthChecker{}

	tool := GetServerHealthTool{
		logger:        logger,
		healthChecker: checker,
	}

	if tool.healthChecker == nil {
		t.Error("Expected healthChecker to be set")
	}

	// Test that we can call methods on the health checker
	resources := tool.healthChecker.GetSystemResources()
	if resources.CPUUsage != 25.5 {
		t.Errorf("Expected CPU usage to be 25.5, got %f", resources.CPUUsage)
	}
}

// Test GetServerHealthArgs variations
func TestGetServerHealthArgsVariations(t *testing.T) {
	// Test minimal args
	minimalArgs := GetServerHealthArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: "minimal-health",
		},
		IncludeDetails: false,
	}

	if minimalArgs.SessionID != "minimal-health" {
		t.Errorf("Expected SessionID to be 'minimal-health', got '%s'", minimalArgs.SessionID)
	}
	if minimalArgs.IncludeDetails {
		t.Error("Expected IncludeDetails to be false")
	}

	// Test full args
	fullArgs := GetServerHealthArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: "full-health",
			DryRun:    true,
		},
		IncludeDetails: true,
	}

	if fullArgs.SessionID != "full-health" {
		t.Errorf("Expected SessionID to be 'full-health', got '%s'", fullArgs.SessionID)
	}
	if !fullArgs.DryRun {
		t.Error("Expected DryRun to be true")
	}
	if !fullArgs.IncludeDetails {
		t.Error("Expected IncludeDetails to be true")
	}
}
