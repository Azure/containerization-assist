package adapter

import (
	"time"
)

// ServerInterface defines the interface for server operations needed by adapters
type ServerInterface interface {
	GetWorkspaceStats() WorkspaceStats
	GetSessionManagerStats() SessionManagerStats
	GetCircuitBreakerStats() map[string]CircuitBreakerStats
	GetConfig() ServerConfig
	GetStartTime() time.Time
}

// WorkspaceStats represents workspace statistics
type WorkspaceStats struct {
	TotalDiskUsage int64
	SessionCount   int
}

// SessionManagerStats represents session manager statistics
type SessionManagerStats struct {
	ActiveSessions int
	TotalSessions  int
}

// CircuitBreakerStats represents circuit breaker status
type CircuitBreakerStats struct {
	State        string
	FailureCount int
	SuccessCount int
	LastFailure  *time.Time
}

// ServerConfig represents server configuration needed by adapters
type ServerConfig struct {
	TotalDiskLimit int64
}
