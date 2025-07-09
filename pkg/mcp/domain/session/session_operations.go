package session

import "context"

// Operations handles basic session CRUD operations
type Operations interface {
	// GetSession retrieves a session by ID
	GetSession(sessionID string) (*SessionState, error)

	// GetOrCreateSession gets or creates a session
	GetOrCreateSession(sessionID string) (*SessionState, error)

	// UpdateSession updates session state
	UpdateSession(ctx context.Context, sessionID string, updateFunc func(*SessionState) error) error

	// DeleteSession deletes a session
	DeleteSession(sessionID string) error
}

// Query handles session querying operations
type Query interface {
	// ListSessions lists all sessions
	ListSessions() ([]*SessionState, error)

	// ListSessionSummaries lists session summaries
	ListSessionSummaries() ([]*SessionSummary, error)

	// GetSessionData retrieves session data
	GetSessionData(ctx context.Context, sessionID string) (map[string]interface{}, error)
}

// JobTracker handles job tracking within sessions
type JobTracker interface {
	// StartJob starts a new job in the session
	StartJob(sessionID string, jobType string) (string, error)

	// UpdateJobStatus updates job status
	UpdateJobStatus(sessionID string, jobID string, status JobStatus, result interface{}, err error) error

	// CompleteJob completes a job
	CompleteJob(sessionID string, jobID string, result interface{}) error
}

// ToolTracker handles tool execution tracking
type ToolTracker interface {
	// TrackToolExecution tracks tool execution
	TrackToolExecution(sessionID string, toolName string, args interface{}) error

	// CompleteToolExecution completes tool execution
	CompleteToolExecution(sessionID string, toolName string, success bool, err error, tokensUsed int) error
}

// ErrorTracker handles error tracking
type ErrorTracker interface {
	// TrackError tracks an error
	TrackError(sessionID string, err error, context interface{}) error
}

// Lifecycle handles session lifecycle management
type Lifecycle interface {
	// StartCleanupRoutine starts cleanup routine
	StartCleanupRoutine()

	// Stop stops the session services
	Stop() error
}
