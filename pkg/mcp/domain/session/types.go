package session

import (
	"context"

	"github.com/Azure/container-kit/pkg/mcp/domain"
)

// State re-exports domain.SessionState for backward compatibility
type State = domain.SessionState

// Info re-exports domain.SessionInfo for backward compatibility
type Info = domain.SessionInfo

// Summary represents a session summary for backward compatibility
type Summary = SessionSummary

// JobStatus represents the status of a job/session
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

// Manager interface defines session management operations
type Manager interface {
	// GetSession retrieves a session by ID
	GetSession(sessionID string) (*State, error)

	// GetSessionTyped retrieves a session with type safety
	GetSessionTyped(sessionID string) (*State, error)

	// GetSessionConcrete retrieves a concrete session
	GetSessionConcrete(sessionID string) (*State, error)

	// GetSessionData retrieves session data
	GetSessionData(ctx context.Context, sessionID string) (map[string]interface{}, error)

	// GetOrCreateSession gets or creates a session
	GetOrCreateSession(sessionID string) (*State, error)

	// GetOrCreateSessionTyped gets or creates a session with type safety
	GetOrCreateSessionTyped(sessionID string) (*State, error)

	// UpdateSession updates session state
	UpdateSession(ctx context.Context, sessionID string, updateFunc func(*State) error) error

	// DeleteSession deletes a session
	DeleteSession(sessionID string) error

	// ListSessionsTyped lists sessions with type safety
	ListSessionsTyped() ([]*State, error)

	// ListSessionSummaries lists session summaries
	ListSessionSummaries() ([]*Summary, error)

	// StartJob starts a new job in the session
	StartJob(sessionID string, jobType string) (string, error)

	// UpdateJobStatus updates job status
	UpdateJobStatus(sessionID string, jobID string, status JobStatus, result interface{}, err error) error

	// CompleteJob completes a job
	CompleteJob(sessionID string, jobID string, result interface{}) error

	// TrackToolExecution tracks tool execution
	TrackToolExecution(sessionID string, toolName string, args interface{}) error

	// CompleteToolExecution completes tool execution
	CompleteToolExecution(sessionID string, toolName string, success bool, err error, tokensUsed int) error

	// TrackError tracks an error
	TrackError(sessionID string, err error, context interface{}) error

	// StartCleanupRoutine starts cleanup routine
	StartCleanupRoutine()

	// Stop stops the session manager
	Stop() error
}

// Type aliases for backward compatibility
//
//nolint:revive // These aliases are needed for backward compatibility
type SessionState = State

//nolint:revive // These aliases are needed for backward compatibility
type SessionInfo = Info

//nolint:revive // These aliases are needed for backward compatibility
type SessionManager = Manager

// UnifiedSessionManager combines SessionManager with additional unified capabilities
type UnifiedSessionManager interface {
	SessionManager
	// Additional unified methods can be added here if needed
}
