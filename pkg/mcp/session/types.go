package session

import (
	"time"
)

// Additional session types to complement existing definitions

// SessionType defines the type of session
type SessionType string

const (
	SessionTypeInteractive SessionType = "interactive"
	SessionTypeWorkflow    SessionType = "workflow"
	SessionTypeBatch       SessionType = "batch"
	SessionTypeAPI         SessionType = "api"
)

// Extended session status to complement existing ones
const (
	SessionStatusSuspended SessionStatus = "suspended"
	SessionStatusDeleted   SessionStatus = "deleted"
)

// SessionStatus type alias for consistency
type SessionStatus = string

// SessionMetadata contains session metadata
type SessionMetadata struct {
	Version     string                 `json:"version"`
	Environment string                 `json:"environment,omitempty"`
	Platform    string                 `json:"platform,omitempty"`
	Resources   SessionResources       `json:"resources,omitempty"`
	Permissions []string               `json:"permissions,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Custom      map[string]interface{} `json:"custom,omitempty"`
}

// SessionResources defines resource limits for a session
type SessionResources struct {
	MaxMemory     string        `json:"max_memory,omitempty"`
	MaxCPU        string        `json:"max_cpu,omitempty"`
	MaxStorage    string        `json:"max_storage,omitempty"`
	MaxExecutions int           `json:"max_executions,omitempty"`
	Timeout       time.Duration `json:"timeout,omitempty"`
}

// HistoryEntry represents an action in session history
type HistoryEntry struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Action    string                 `json:"action"`
	Tool      string                 `json:"tool,omitempty"`
	Input     map[string]interface{} `json:"input,omitempty"`
	Output    map[string]interface{} `json:"output,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Duration  time.Duration          `json:"duration,omitempty"`
}

// SessionStats contains session statistics
type SessionStats struct {
	TotalExecutions      int           `json:"total_executions"`
	SuccessfulExecutions int           `json:"successful_executions"`
	FailedExecutions     int           `json:"failed_executions"`
	TotalDuration        time.Duration `json:"total_duration"`
	LastExecutionTime    *time.Time    `json:"last_execution_time,omitempty"`
	ResourceUsage        ResourceUsage `json:"resource_usage"`
}

// ResourceUsage tracks resource consumption
type ResourceUsage struct {
	CPUSeconds float64 `json:"cpu_seconds"`
	MemoryMB   float64 `json:"memory_mb"`
	StorageMB  float64 `json:"storage_mb"`
	NetworkMB  float64 `json:"network_mb"`
}

// CreateSessionRequest for creating new sessions
type CreateSessionRequest struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Type        SessionType            `json:"type,omitempty"`
	Labels      map[string]string      `json:"labels,omitempty"`
	TTL         time.Duration          `json:"ttl,omitempty"`
	Resources   *SessionResources      `json:"resources,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// UpdateSessionRequest for updating sessions
type UpdateSessionRequest struct {
	Name        *string                `json:"name,omitempty"`
	Description *string                `json:"description,omitempty"`
	Labels      map[string]string      `json:"labels,omitempty"`
	TTL         *time.Duration         `json:"ttl,omitempty"`
	Resources   *SessionResources      `json:"resources,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// SessionFilter for querying sessions
type SessionFilter struct {
	IDs           []string          `json:"ids,omitempty"`
	Names         []string          `json:"names,omitempty"`
	Types         []SessionType     `json:"types,omitempty"`
	Statuses      []SessionStatus   `json:"statuses,omitempty"`
	Owners        []string          `json:"owners,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
	CreatedAfter  *time.Time        `json:"created_after,omitempty"`
	CreatedBefore *time.Time        `json:"created_before,omitempty"`
	ActiveAfter   *time.Time        `json:"active_after,omitempty"`
	ActiveBefore  *time.Time        `json:"active_before,omitempty"`
	Limit         int               `json:"limit,omitempty"`
	Offset        int               `json:"offset,omitempty"`
}
