package health

import "context"

// Status represents the health status of a component
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusDegraded  Status = "degraded"
	StatusUnhealthy Status = "unhealthy"
)

// Checker defines the interface for health checking.
type Checker interface {
	// Check performs a health check
	Check(ctx context.Context) (Status, string, map[string]string)

	// Name returns the name of the health check
	Name() string
}

// Monitor defines the interface for health monitoring.
// This interface is implemented by infrastructure layer.
type Monitor interface {
	// RegisterChecker registers a health checker
	RegisterChecker(checker Checker)

	// GetHealth returns the overall health status
	GetHealth(ctx context.Context) HealthReport

	// GetComponentHealth returns health status for a specific component
	GetComponentHealth(ctx context.Context, component string) (Status, error)
}

// HealthReport represents a complete health report
type HealthReport struct {
	Status     Status                     `json:"status"`
	Components map[string]ComponentHealth `json:"components"`
	Metadata   map[string]interface{}     `json:"metadata,omitempty"`
}

// ComponentHealth represents health status of a single component
type ComponentHealth struct {
	Status  Status            `json:"status"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}
