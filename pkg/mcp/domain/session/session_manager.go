package session

import (
	"context"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/core"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	internaltypes "github.com/Azure/container-kit/pkg/mcp/domain/types"
)

// Legacy compatibility methods for tools that still use the old interface.
// These methods delegate to the new component-based implementation.

// GetStatsLegacy returns basic statistics about the session manager
func (sm *SessionManager) GetStatsLegacy() (*core.SessionManagerStats, error) {
	return sm.GetStats(context.Background())
}

// GetSessionDataLegacy returns session data by ID (legacy interface)
func (sm *SessionManager) GetSessionDataLegacy(sessionID string) (*SessionData, error) {
	return sm.GetSessionData(context.Background(), sessionID)
}

// GetAllSessions implements ListSessionsManager interface
func (sm *SessionManager) GetAllSessions() (map[string]*SessionState, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sessions := make(map[string]*SessionState)
	for id, session := range sm.sessions {
		sessions[id] = session
	}
	return sessions, nil
}

// TrackError tracks an error for a session (simplified)
func (sm *SessionManager) TrackError(sessionID string, err error, context map[string]interface{}) error {
	sm.logger.Warn("Session error tracked", "error", err, "session_id", sessionID)
	return nil
}

func (sm *SessionManager) SetSessionLabels(sessionID string, labels []string) error {
	return sm.UpdateSessionLabels(sessionID, labels)
}

// Job and tool execution tracking (simplified but functional)
func (sm *SessionManager) StartJob(sessionID, jobType string) (string, error) {
	jobID := "job-" + generateSessionID()
	err := sm.UpdateSession(context.Background(), sessionID, func(session *SessionState) error {
		if session.ActiveJobs == nil {
			session.ActiveJobs = make(map[string]JobInfo)
		}
		session.ActiveJobs[jobID] = JobInfo{
			JobID:     jobID,
			Tool:      jobType,
			Status:    "running",
			StartTime: time.Now(),
		}
		return nil
	})
	return jobID, err
}

func (sm *SessionManager) UpdateJobStatus(sessionID, jobID string, status interface{}, result interface{}, err error) error {
	return sm.UpdateSession(context.Background(), sessionID, func(session *SessionState) error {
		if job, exists := session.ActiveJobs[jobID]; exists {
			if statusStr, ok := status.(string); ok {
				job.Status = JobStatus(statusStr)
			}
			job.Result = result
			session.ActiveJobs[jobID] = job
		}
		return nil
	})
}

func (sm *SessionManager) CompleteJob(sessionID, jobID string, result interface{}) error {
	return sm.UpdateSession(context.Background(), sessionID, func(session *SessionState) error {
		if job, exists := session.ActiveJobs[jobID]; exists {
			now := time.Now()
			job.EndTime = &now
			duration := now.Sub(job.StartTime)
			job.Duration = &duration
			job.Status = "completed"
			job.Result = result
			session.ActiveJobs[jobID] = job
		}
		return nil
	})
}

func (sm *SessionManager) TrackToolExecution(sessionID, toolName string, args interface{}) error {
	return sm.UpdateSession(context.Background(), sessionID, func(session *SessionState) error {
		execution := ToolExecution{
			Tool:      toolName,
			StartTime: time.Now(),
			Success:   false, // Will be updated on completion
		}
		session.StageHistory = append(session.StageHistory, execution)
		return nil
	})
}

func (sm *SessionManager) CompleteToolExecution(sessionID, toolName string, success bool, err error, tokensUsed int) error {
	return sm.UpdateSession(context.Background(), sessionID, func(session *SessionState) error {
		if len(session.StageHistory) > 0 {
			// Update the last entry for this tool
			for i := len(session.StageHistory) - 1; i >= 0; i-- {
				if session.StageHistory[i].Tool == toolName && session.StageHistory[i].EndTime == nil {
					now := time.Now()
					duration := now.Sub(session.StageHistory[i].StartTime)
					session.StageHistory[i].EndTime = &now
					session.StageHistory[i].Duration = &duration
					session.StageHistory[i].Success = success
					session.StageHistory[i].TokensUsed = tokensUsed
					break
				}
			}
		}
		return nil
	})
}

// TypedToolSessionManager interface implementation
// These methods provide type-safe session operations

// GetSessionLegacy retrieves a session and returns interface{} for compatibility with core.SessionManager
func (sm *SessionManager) GetSessionLegacy(sessionID string) (interface{}, error) {
	session, err := sm.GetSession(context.Background(), sessionID)
	if err != nil {
		return nil, err
	}
	return session, nil
}

func (sm *SessionManager) GetSessionTyped(sessionID string) (*core.SessionState, error) {
	session, err := sm.GetSession(context.Background(), sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, errors.NewError().Messagef("session not found: %s", sessionID).Build()
	}
	return session.ToCoreSessionState(), nil
}

// GetOrCreateSessionTyped gets or creates a session with type safety
func (sm *SessionManager) GetOrCreateSessionTyped(sessionID string) (*core.SessionState, error) {
	session, err := sm.GetOrCreateSession(context.Background(), sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, errors.NewError().Message("failed to get or create session").Build()
	}
	return session.ToCoreSessionState(), nil
}

// SaveSession saves a session to the store
func (sm *SessionManager) SaveSession(ctx context.Context, sessionID string, session *SessionState) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Update in memory
	sm.sessions[sessionID] = session
	session.UpdatedAt = time.Now()

	// Save to persistent store
	if sm.store != nil {
		return sm.store.Save(ctx, sessionID, session)
	}

	return nil
}

// CancelSessionJobs cancels all active jobs for a session
func (sm *SessionManager) CancelSessionJobs(sessionID string) ([]string, error) {
	cancelledJobs := []string{}
	err := sm.UpdateSession(context.Background(), sessionID, func(session *SessionState) error {
		if session.ActiveJobs == nil {
			return nil
		}

		// Cancel all active jobs
		for jobID, job := range session.ActiveJobs {
			if job.Status == "running" || job.Status == "pending" {
				job.Status = "cancelled"
				now := time.Now()
				job.EndTime = &now
				session.ActiveJobs[jobID] = job
				cancelledJobs = append(cancelledJobs, jobID)
			}
		}
		return nil
	})

	return cancelledJobs, err
}

// CreateSessionLegacy creates a new session and returns interface{} for compatibility with core.SessionManager
func (sm *SessionManager) CreateSessionLegacy(userID string) (interface{}, error) {
	sessionState, err := sm.CreateSessionTyped(userID)
	if err != nil {
		return nil, err
	}
	return sessionState, nil
}

// CreateSessionTyped creates a new session with type safety
func (sm *SessionManager) CreateSessionTyped(userID string) (*core.SessionState, error) {
	// Generate a new session ID, optionally incorporating userID for uniqueness
	sessionID := generateSessionID()
	session, err := sm.GetOrCreateSession(context.Background(), sessionID)
	if err != nil {
		return nil, err
	}

	if userID != "" {
		// Store user association in session metadata
		err = sm.UpdateSession(context.Background(), sessionID, func(s *SessionState) error {
			if s.Metadata == nil {
				s.Metadata = make(map[string]interface{})
			}
			s.Metadata["user_id"] = userID
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	return session.ToCoreSessionState(), nil
}

// ListSessionsTyped lists sessions with type safety
func (sm *SessionManager) ListSessionsTyped(ctx context.Context, filter core.SessionFilter) ([]*core.SessionState, error) {
	sessions, err := sm.ListSessions(ctx)
	if err != nil {
		return nil, err
	}

	var results []*core.SessionState
	for _, session := range sessions {
		// Apply filters
		if filter.UserID != "" {
			if session.Metadata == nil {
				continue
			}
			if userID, ok := session.Metadata["user_id"].(string); !ok || userID != filter.UserID {
				continue
			}
		}

		if filter.Status != "" && session.Status != filter.Status {
			continue
		}

		if filter.CreatedAfter != nil && session.CreatedAt.Before(*filter.CreatedAfter) {
			continue
		}

		results = append(results, session.ToCoreSessionState())
	}

	return results, nil
}

// CreateWorkflowSession implements UnifiedSessionManager.CreateWorkflowSession
func (sm *SessionManager) CreateWorkflowSession(ctx context.Context, spec *WorkflowSpec) (*SessionState, error) {
	sessionID := generateSessionID()
	session, err := sm.GetOrCreateSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	// Add workflow metadata
	err = sm.UpdateSession(ctx, sessionID, func(s *SessionState) error {
		if s.Metadata == nil {
			s.Metadata = make(map[string]interface{})
		}
		s.Metadata["workflow_id"] = spec.Metadata.Name
		s.Metadata["workflow_type"] = "workflow"
		return nil
	})
	if err != nil {
		return nil, err
	}

	return session, nil
}

// Helper method to convert from core.SessionState to SessionState
func (sm *SessionManager) convertFromCoreSessionState(coreState *core.SessionState) *SessionState {
	// Create empty ImageRef since coreState.ImageRef is likely a string
	var imageRef internaltypes.ImageReference
	if coreState.ImageRef != "" {
		imageRef = internaltypes.ImageReference{
			Registry:   "",
			Repository: coreState.ImageRef,
			Tag:        "latest",
		}
	}

	return &SessionState{
		SessionID:    coreState.SessionID,
		CreatedAt:    coreState.CreatedAt,
		LastAccessed: coreState.UpdatedAt,
		WorkspaceDir: coreState.WorkspaceDir,
		RepoURL:      coreState.RepoURL,
		ImageRef:     imageRef,
		Metadata:     coreState.Metadata,
	}
}
