package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
	"github.com/rs/zerolog"
)

// GetServerHealthArgs represents the arguments for getting server health
type GetServerHealthArgs struct {
	types.BaseToolArgs
	IncludeDetails bool `json:"include_details,omitempty" jsonschema:"description=Include detailed metrics"`
}

// SystemResources represents system resource usage
type SystemResources struct {
	CPUCount       int     `json:"cpu_count"`
	MemoryTotal    uint64  `json:"memory_total_bytes"`
	MemoryUsed     uint64  `json:"memory_used_bytes"`
	MemoryPercent  float64 `json:"memory_percent"`
	GoroutineCount int     `json:"goroutine_count"`
	DiskTotal      uint64  `json:"disk_total_bytes"`
	DiskUsed       uint64  `json:"disk_used_bytes"`
	DiskPercent    float64 `json:"disk_percent"`
}

// CircuitBreakerStatus represents the status of a circuit breaker
type CircuitBreakerStatus struct {
	Name        string    `json:"name"`
	State       string    `json:"state"` // "closed", "open", "half-open"
	Failures    int       `json:"failures"`
	Successes   int       `json:"successes"`
	LastFailure time.Time `json:"last_failure,omitempty"`
	NextRetry   time.Time `json:"next_retry,omitempty"`
}

// ServiceHealth represents the health of an external service
type ServiceHealth struct {
	Name      string    `json:"name"`
	Status    string    `json:"status"` // "healthy", "degraded", "unhealthy"
	Message   string    `json:"message,omitempty"`
	LastCheck time.Time `json:"last_check"`
}

// JobQueueStats represents job queue statistics
type JobQueueStats struct {
	QueueDepth      int     `json:"queue_depth"`
	ProcessingRate  float64 `json:"processing_rate_per_minute"`
	ActiveWorkers   int     `json:"active_workers"`
	CompletedJobs   int     `json:"completed_jobs"`
	FailedJobs      int     `json:"failed_jobs"`
	AverageWaitTime string  `json:"average_wait_time"`
}

// GetServerHealthResult represents the server health status
type GetServerHealthResult struct {
	types.BaseToolResponse
	Status          string                          `json:"status"` // "healthy", "degraded", "unhealthy"
	Uptime          string                          `json:"uptime"`
	SystemResources SystemResources                 `json:"system_resources"`
	Sessions        SessionHealthStats              `json:"sessions"`
	CircuitBreakers map[string]CircuitBreakerStatus `json:"circuit_breakers"`
	Services        []ServiceHealth                 `json:"services"`
	JobQueue        JobQueueStats                   `json:"job_queue"`
	RecentErrors    []RecentError                   `json:"recent_errors,omitempty"`
	Warnings        []string                        `json:"warnings,omitempty"`
}

// SessionHealthStats represents session-related health statistics
type SessionHealthStats struct {
	ActiveSessions  int     `json:"active_sessions"`
	TotalSessions   int     `json:"total_sessions"`
	MaxSessions     int     `json:"max_sessions"`
	SessionsPercent float64 `json:"sessions_percent"`
	TotalDiskUsed   int64   `json:"total_disk_used_bytes"`
	DiskQuota       int64   `json:"disk_quota_bytes"`
	DiskUsedPercent float64 `json:"disk_used_percent"`
}

// RecentError represents a recent error
type RecentError struct {
	Timestamp time.Time `json:"timestamp"`
	Tool      string    `json:"tool"`
	Error     string    `json:"error"`
	Count     int       `json:"count"`
}

// HealthChecker interface for checking service health
type HealthChecker interface {
	GetSystemResources() SystemResources
	GetSessionStats() SessionHealthStats
	GetCircuitBreakerStats() map[string]CircuitBreakerStatus
	CheckServiceHealth(ctx context.Context) []ServiceHealth
	GetJobQueueStats() JobQueueStats
	GetRecentErrors(limit int) []RecentError
	GetUptime() time.Duration
}

// GetServerHealthTool implements the get_server_health MCP tool
type GetServerHealthTool struct {
	logger        zerolog.Logger
	healthChecker HealthChecker
}

// NewGetServerHealthTool creates a new server health tool
func NewGetServerHealthTool(logger zerolog.Logger, healthChecker HealthChecker) *GetServerHealthTool {
	return &GetServerHealthTool{
		logger:        logger,
		healthChecker: healthChecker,
	}
}

// Execute implements the unified Tool interface
func (t *GetServerHealthTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	// Type assertion to get proper args
	healthArgs, ok := args.(GetServerHealthArgs)
	if !ok {
		return nil, fmt.Errorf("invalid arguments type: expected GetServerHealthArgs, got %T", args)
	}

	return t.ExecuteTyped(ctx, healthArgs)
}

// ExecuteTyped provides typed execution for backward compatibility
func (t *GetServerHealthTool) ExecuteTyped(ctx context.Context, args GetServerHealthArgs) (*GetServerHealthResult, error) {
	t.logger.Info().
		Bool("include_details", args.IncludeDetails).
		Msg("Checking server health")

	// Get system resources
	sysResources := t.healthChecker.GetSystemResources()

	// Get session statistics
	sessionStats := t.healthChecker.GetSessionStats()

	// Get circuit breaker states
	circuitBreakers := t.healthChecker.GetCircuitBreakerStats()

	// Check external services
	services := t.healthChecker.CheckServiceHealth(ctx)

	// Get job queue stats
	jobQueue := t.healthChecker.GetJobQueueStats()

	// Get recent errors if requested
	var recentErrors []RecentError
	if args.IncludeDetails {
		recentErrors = t.healthChecker.GetRecentErrors(10)
	}

	// Calculate overall status
	status, warnings := t.calculateOverallStatus(sysResources, sessionStats, circuitBreakers, services, jobQueue)

	// Get uptime
	uptime := t.healthChecker.GetUptime()

	result := &GetServerHealthResult{
		BaseToolResponse: types.NewBaseResponse("get_server_health", args.SessionID, args.DryRun),
		Status:           status,
		Uptime:           uptime.String(),
		SystemResources:  sysResources,
		Sessions:         sessionStats,
		CircuitBreakers:  circuitBreakers,
		Services:         services,
		JobQueue:         jobQueue,
		RecentErrors:     recentErrors,
		Warnings:         warnings,
	}

	t.logger.Info().
		Str("status", status).
		Str("uptime", uptime.String()).
		Int("warnings", len(warnings)).
		Msg("Server health check completed")

	return result, nil
}

// calculateOverallStatus determines the overall health status
func (t *GetServerHealthTool) calculateOverallStatus(
	sysResources SystemResources,
	sessionStats SessionHealthStats,
	circuitBreakers map[string]CircuitBreakerStatus,
	services []ServiceHealth,
	jobQueue JobQueueStats,
) (string, []string) {
	warnings := []string{}
	status := "healthy"

	// Check system resources
	if sysResources.MemoryPercent > 90 {
		warnings = append(warnings, fmt.Sprintf("High memory usage: %.1f%%", sysResources.MemoryPercent))
		status = "degraded"
	}

	if sysResources.DiskPercent > 90 {
		warnings = append(warnings, fmt.Sprintf("High disk usage: %.1f%%", sysResources.DiskPercent))
		status = "degraded"
	}

	// Check session limits
	if sessionStats.SessionsPercent > 80 {
		warnings = append(warnings, fmt.Sprintf("Approaching session limit: %d/%d", sessionStats.ActiveSessions, sessionStats.MaxSessions))
		if sessionStats.SessionsPercent > 95 {
			status = "degraded"
		}
	}

	if sessionStats.DiskUsedPercent > 80 {
		warnings = append(warnings, fmt.Sprintf("High workspace disk usage: %.1f%%", sessionStats.DiskUsedPercent))
		if sessionStats.DiskUsedPercent > 95 {
			status = "degraded"
		}
	}

	// Check circuit breakers
	openBreakers := 0
	for name, cb := range circuitBreakers {
		if cb.State == "open" {
			warnings = append(warnings, fmt.Sprintf("Circuit breaker %s is open", name))
			openBreakers++
		}
	}
	if openBreakers > 0 {
		status = "degraded"
	}
	if openBreakers > 2 {
		status = "unhealthy"
	}

	// Check services
	unhealthyServices := 0
	for _, svc := range services {
		switch svc.Status {
		case "unhealthy":
			warnings = append(warnings, fmt.Sprintf("Service %s is unhealthy: %s", svc.Name, svc.Message))
			unhealthyServices++
		case "degraded":
			warnings = append(warnings, fmt.Sprintf("Service %s is degraded: %s", svc.Name, svc.Message))
		}
	}
	if unhealthyServices > 0 {
		status = "degraded"
	}
	if unhealthyServices > 1 {
		status = "unhealthy"
	}

	// Check job queue
	if jobQueue.QueueDepth > 100 {
		warnings = append(warnings, fmt.Sprintf("High job queue depth: %d", jobQueue.QueueDepth))
		status = "degraded"
	}

	return status, warnings
}

// GetMetadata returns comprehensive metadata about the server health tool
func (t *GetServerHealthTool) GetMetadata() mcptypes.ToolMetadata {
	return mcptypes.ToolMetadata{
		Name:        "get_server_health",
		Description: "Check comprehensive server health including resources, services, and circuit breakers",
		Version:     "1.0.0",
		Category:    "Monitoring",
		Dependencies: []string{
			"Health Checker",
			"System Monitor",
			"Circuit Breakers",
			"Service Health Checks",
		},
		Capabilities: []string{
			"System resource monitoring",
			"Session health tracking",
			"Circuit breaker status",
			"External service health",
			"Job queue monitoring",
			"Error tracking",
			"Overall health assessment",
		},
		Requirements: []string{
			"Health checker instance",
			"System monitoring access",
		},
		Parameters: map[string]string{
			"include_details": "Optional: Include detailed metrics and recent errors",
		},
		Examples: []mcptypes.ToolExample{
			{
				Name:        "Basic health check",
				Description: "Get basic server health status",
				Input:       map[string]interface{}{},
				Output: map[string]interface{}{
					"status": "healthy",
					"uptime": "24h30m",
					"system_resources": map[string]interface{}{
						"memory_percent": 45.2,
						"disk_percent":   25.8,
						"cpu_count":      8,
					},
					"sessions": map[string]interface{}{
						"active_sessions":  12,
						"total_sessions":   15,
						"sessions_percent": 80.0,
					},
					"warnings": []string{},
				},
			},
			{
				Name:        "Detailed health check with warnings",
				Description: "Get detailed health status including recent errors",
				Input: map[string]interface{}{
					"include_details": true,
				},
				Output: map[string]interface{}{
					"status": "degraded",
					"uptime": "12h15m",
					"warnings": []string{
						"High memory usage: 92.1%",
						"Circuit breaker docker_registry is open",
					},
					"recent_errors": []map[string]interface{}{
						{
							"timestamp": "2024-12-17T10:30:00Z",
							"tool":      "build_image",
							"error":     "Docker daemon connection failed",
							"count":     3,
						},
					},
				},
			},
		},
	}
}

// Validate checks if the provided arguments are valid for the server health tool
func (t *GetServerHealthTool) Validate(ctx context.Context, args interface{}) error {
	_, ok := args.(GetServerHealthArgs)
	if !ok {
		return fmt.Errorf("invalid arguments type: expected GetServerHealthArgs, got %T", args)
	}

	// Validate health checker is available
	if t.healthChecker == nil {
		return fmt.Errorf("health checker is not configured")
	}

	return nil
}
