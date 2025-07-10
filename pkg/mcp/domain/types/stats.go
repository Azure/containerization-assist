package domaintypes

import "time"

// WorkspaceStats represents workspace statistics
type WorkspaceStats struct {
	TotalDiskUsage int64 `json:"total_disk_usage"`
	SessionCount   int   `json:"session_count"`
}

// SessionManagerStats represents session manager statistics
type SessionManagerStats struct {
	ActiveSessions int `json:"active_sessions"`
	TotalSessions  int `json:"total_sessions"`
}

// CircuitBreakerStats represents circuit breaker statistics
type CircuitBreakerStats struct {
	State        string     `json:"state"`
	FailureCount int        `json:"failure_count"`
	SuccessCount int64      `json:"success_count"`
	LastFailure  *time.Time `json:"last_failure,omitempty"`
}

// ServerConfig represents server configuration
type ServerConfig struct {
	TotalDiskLimit int64 `json:"total_disk_limit"`
}
