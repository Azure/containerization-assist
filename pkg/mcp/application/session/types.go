package session

import (
	"context"
	"time"
)

// SessionManager interface defines session management operations
type SessionManager interface {
	GetSession(ctx context.Context, sessionID string) (*SessionState, error)
	GetSessionTyped(ctx context.Context, sessionID string) (*SessionState, error)
	GetSessionConcrete(ctx context.Context, sessionID string) (*SessionState, error)
	GetOrCreateSession(ctx context.Context, sessionID string) (*SessionState, error)
	GetOrCreateSessionTyped(ctx context.Context, sessionID string) (*SessionState, error)
	UpdateSession(ctx context.Context, sessionID string, updateFunc func(*SessionState) error) error
	ListSessionsTyped(ctx context.Context) ([]*SessionState, error)
	ListSessionSummaries(ctx context.Context) ([]*SessionSummary, error)
	UpdateJobStatus(ctx context.Context, sessionID, jobID string, status JobStatus, result interface{}, err error) error
	StartCleanupRoutine(ctx context.Context) error
	Stop(ctx context.Context) error
	GetStats() (*SessionStats, error)
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
	ActiveSessions int
	TotalSessions  int
	MaxSessions    int
}
