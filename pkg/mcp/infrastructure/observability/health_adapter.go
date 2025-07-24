// Package observability provides unified monitoring, tracing, and health infrastructure
// for the MCP components. It consolidates telemetry, distributed tracing, health checks,
// and logging enrichment into a single coherent package.
package observability

import (
	"context"
	"errors"

	"github.com/Azure/container-kit/pkg/mcp/domain/health"
)

// HealthMonitorAdapter adapts the observability Monitor to implement domain/health.Monitor interface
type HealthMonitorAdapter struct {
	monitor *Monitor
}

// NewHealthMonitorAdapter creates a new adapter that bridges observability.Monitor to health.Monitor
func NewHealthMonitorAdapter(monitor *Monitor) *HealthMonitorAdapter {
	return &HealthMonitorAdapter{
		monitor: monitor,
	}
}

// RegisterChecker adapts and registers a health.Checker with the observability monitor
func (a *HealthMonitorAdapter) RegisterChecker(checker health.Checker) {
	// Create an adapter that converts health.Checker to observability.Checker
	obsChecker := &healthCheckerAdapter{
		domainChecker: checker,
	}
	a.monitor.RegisterChecker(obsChecker)
}

// GetHealth returns the overall health status in domain format
func (a *HealthMonitorAdapter) GetHealth(ctx context.Context) health.HealthReport {
	// Run all checks
	a.monitor.CheckAll(ctx)

	// Get the report from observability monitor
	obsReport := a.monitor.GetReport()

	// Convert to domain format
	components := make(map[string]health.ComponentHealth)
	for name, check := range obsReport.Checks {
		components[name] = health.ComponentHealth{
			Status:  convertStatus(check.Status),
			Message: check.Message,
			Details: check.Details,
		}
	}

	metadata := make(map[string]interface{})
	metadata["uptime"] = obsReport.Uptime.String()
	metadata["version"] = obsReport.Version
	metadata["timestamp"] = obsReport.Timestamp
	metadata["summary"] = obsReport.Summary

	return health.HealthReport{
		Status:     convertStatus(obsReport.Status),
		Components: components,
		Metadata:   metadata,
	}
}

// GetComponentHealth returns health status for a specific component
func (a *HealthMonitorAdapter) GetComponentHealth(ctx context.Context, component string) (health.Status, error) {
	// Run checks first
	a.monitor.CheckAll(ctx)

	// Get the report
	report := a.monitor.GetReport()

	// Find the component
	if check, ok := report.Checks[component]; ok {
		return convertStatus(check.Status), nil
	}

	return health.StatusUnhealthy, errors.New("component not found")
}

// healthCheckerAdapter adapts domain/health.Checker to observability.Checker
type healthCheckerAdapter struct {
	domainChecker health.Checker
}

func (h *healthCheckerAdapter) Name() string {
	return h.domainChecker.Name()
}

func (h *healthCheckerAdapter) Check(ctx context.Context) Check {
	status, message, details := h.domainChecker.Check(ctx)
	return Check{
		Name:    h.domainChecker.Name(),
		Status:  convertDomainStatus(status),
		Message: message,
		Details: details,
	}
}

// convertStatus converts observability.Status to health.Status
func convertStatus(s Status) health.Status {
	switch s {
	case StatusHealthy:
		return health.StatusHealthy
	case StatusDegraded:
		return health.StatusDegraded
	case StatusUnhealthy:
		return health.StatusUnhealthy
	default:
		return health.StatusUnhealthy
	}
}

// convertDomainStatus converts health.Status to observability.Status
func convertDomainStatus(s health.Status) Status {
	switch s {
	case health.StatusHealthy:
		return StatusHealthy
	case health.StatusDegraded:
		return StatusDegraded
	case health.StatusUnhealthy:
		return StatusUnhealthy
	default:
		return StatusUnhealthy
	}
}
