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

// NOTE: Using mcptypes.SystemResources, mcptypes.CircuitBreakerStatus, and mcptypes.ServiceHealth

// NOTE: Using mcptypes.JobQueueStats

// GetServerHealthResult represents the server health status
type GetServerHealthResult struct {
	types.BaseToolResponse
	Status          string                                   `json:"status"` // "healthy", "degraded", "unhealthy"
	Uptime          string                                   `json:"uptime"`
	SystemResources mcptypes.SystemResources                 `json:"system_resources"`
	Sessions        mcptypes.SessionHealthStats              `json:"sessions"`
	CircuitBreakers map[string]mcptypes.CircuitBreakerStatus `json:"circuit_breakers"`
	Services        []mcptypes.ServiceHealth                 `json:"services"`
	JobQueue        mcptypes.JobQueueStats                   `json:"job_queue"`
	RecentErrors    []mcptypes.RecentError                   `json:"recent_errors,omitempty"`
	Warnings        []string                                 `json:"warnings,omitempty"`
}

// NOTE: Using mcptypes.SessionHealthStats and mcptypes.RecentError

// HealthChecker interface for checking service health
// LocalHealthChecker defines the interface for health checking operations
// This extends the core health checking functionality
type LocalHealthChecker interface {
	GetSystemResources() mcptypes.SystemResources
	GetSessionStats() mcptypes.SessionHealthStats
	GetCircuitBreakerStats() map[string]mcptypes.CircuitBreakerStatus
	CheckServiceHealth(ctx context.Context) []mcptypes.ServiceHealth
	GetJobQueueStats() mcptypes.JobQueueStats
	GetRecentErrors(limit int) []mcptypes.RecentError
	GetUptime() time.Duration
}

// GetServerHealthTool implements the get_server_health MCP tool
type GetServerHealthTool struct {
	logger        zerolog.Logger
	healthChecker LocalHealthChecker
}

// NewGetServerHealthTool creates a new server health tool
func NewGetServerHealthTool(logger zerolog.Logger, healthChecker LocalHealthChecker) *GetServerHealthTool {
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
	var recentErrors []mcptypes.RecentError
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
	sysResources mcptypes.SystemResources,
	sessionStats mcptypes.SessionHealthStats,
	circuitBreakers map[string]mcptypes.CircuitBreakerStatus,
	services []mcptypes.ServiceHealth,
	jobQueue mcptypes.JobQueueStats,
) (string, []string) {
	warnings := []string{}
	status := "healthy"

	// Check system resources
	if sysResources.MemoryUsage > 90 {
		warnings = append(warnings, fmt.Sprintf("High memory usage: %.1f%%", sysResources.MemoryUsage))
		status = "degraded"
	}

	if sysResources.DiskUsage > 90 {
		warnings = append(warnings, fmt.Sprintf("High disk usage: %.1f%%", sysResources.DiskUsage))
		status = "degraded"
	}

	// Check session limits
	if sessionStats.FailedSessions > 0 {
		warnings = append(warnings, fmt.Sprintf("Failed sessions detected: %d", sessionStats.FailedSessions))
		if sessionStats.FailedSessions > 10 {
			status = "degraded"
		}
	}

	if sessionStats.SessionErrors > 50 {
		warnings = append(warnings, fmt.Sprintf("High session error rate: %d errors in last hour", sessionStats.SessionErrors))
		status = "degraded"
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
			warnings = append(warnings, fmt.Sprintf("Service %s is unhealthy: %s", svc.Name, svc.ErrorMessage))
			unhealthyServices++
		case "degraded":
			warnings = append(warnings, fmt.Sprintf("Service %s is degraded: %s", svc.Name, svc.ErrorMessage))
		}
	}
	if unhealthyServices > 0 {
		status = "degraded"
	}
	if unhealthyServices > 1 {
		status = "unhealthy"
	}

	// Check job queue
	if jobQueue.QueuedJobs > 100 {
		warnings = append(warnings, fmt.Sprintf("High job queue depth: %d", jobQueue.QueuedJobs))
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
