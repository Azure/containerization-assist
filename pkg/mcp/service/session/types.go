package session

import (
	"context"
	"time"
)

// SessionManager provides a streamlined session management interface
type SessionManager interface {
	// Get retrieves a session by ID
	Get(ctx context.Context, sessionID string) (*SessionState, error)

	// GetOrCreate gets an existing session or creates a new one
	GetOrCreate(ctx context.Context, sessionID string) (*SessionState, error)

	// Update modifies a session using an update function
	Update(ctx context.Context, sessionID string, updateFunc func(*SessionState) error) error

	// List returns all active sessions
	List(ctx context.Context) ([]*SessionState, error)

	// Stats returns session statistics
	Stats() *SessionStats

	// Stop shuts down the session manager
	Stop(ctx context.Context) error
}

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
	ActiveSessions   int
	TotalSessions    int
	MaxSessions      int
	TotalCreated     int
	TotalExpired     int
	AverageSessionMS int64
	MemoryUsage      int64
}
