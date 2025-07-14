package session

import (
	"time"
)

// SessionState represents a session's state
type SessionState struct {
	SessionID string
	CreatedAt time.Time
	UpdatedAt time.Time
	ExpiresAt time.Time
	Status    string
	Stage     string
	UserID    string
	Labels    map[string]string
	Metadata  map[string]interface{}
}

// SessionSummary represents a summary of a session
type SessionSummary struct {
	ID     string
	Labels map[string]string
}

// JobStatus represents the status of a job
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
)

// SessionStats represents session statistics
type SessionStats struct {
	ActiveSessions int
	TotalSessions  int
	MaxSessions    int
}
