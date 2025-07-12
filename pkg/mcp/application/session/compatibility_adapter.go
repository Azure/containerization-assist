// Package session provides compatibility between old and new session manager interfaces.
package session

import (
	"context"
	"log/slog"
	"time"
)

// compatibilityAdapter wraps the OptimizedSessionManager to provide the old SessionManager interface
type compatibilityAdapter struct {
	optimized OptimizedSessionManager
	logger    *slog.Logger
}

// NewMemorySessionManager creates a session manager using the optimized implementation
// but exposing the legacy interface for backward compatibility
func NewMemorySessionManager(logger *slog.Logger, defaultTTL time.Duration, maxSessions int) SessionManager {
	optimized := NewOptimizedSessionManager(logger, defaultTTL, maxSessions)

	return &compatibilityAdapter{
		optimized: optimized,
		logger:    logger.With("component", "session_adapter"),
	}
}

// Core methods - delegate to optimized manager

func (a *compatibilityAdapter) GetSession(ctx context.Context, sessionID string) (*SessionState, error) {
	return a.optimized.Get(ctx, sessionID)
}

func (a *compatibilityAdapter) GetOrCreateSession(ctx context.Context, sessionID string) (*SessionState, error) {
	return a.optimized.GetOrCreate(ctx, sessionID)
}

func (a *compatibilityAdapter) UpdateSession(ctx context.Context, sessionID string, updateFunc func(*SessionState) error) error {
	return a.optimized.Update(ctx, sessionID, updateFunc)
}

func (a *compatibilityAdapter) Stop(ctx context.Context) error {
	return a.optimized.Stop(ctx)
}

func (a *compatibilityAdapter) GetStats() (*SessionStats, error) {
	stats := a.optimized.Stats()
	return stats, nil
}

// Legacy methods - provide compatibility implementations

func (a *compatibilityAdapter) GetSessionTyped(ctx context.Context, sessionID string) (*SessionState, error) {
	// GetSessionTyped was identical to GetSession - just delegate
	a.logger.Debug("Using deprecated GetSessionTyped, consider migrating to optimized interface")
	return a.optimized.Get(ctx, sessionID)
}

func (a *compatibilityAdapter) GetSessionConcrete(ctx context.Context, sessionID string) (*SessionState, error) {
	// GetSessionConcrete was identical to GetSession - just delegate
	a.logger.Debug("Using deprecated GetSessionConcrete, consider migrating to optimized interface")
	return a.optimized.Get(ctx, sessionID)
}

func (a *compatibilityAdapter) GetOrCreateSessionTyped(ctx context.Context, sessionID string) (*SessionState, error) {
	// GetOrCreateSessionTyped was identical to GetOrCreateSession - just delegate
	a.logger.Debug("Using deprecated GetOrCreateSessionTyped, consider migrating to optimized interface")
	return a.optimized.GetOrCreate(ctx, sessionID)
}

func (a *compatibilityAdapter) ListSessionsTyped(ctx context.Context) ([]*SessionState, error) {
	// ListSessionsTyped maps to the new List method
	a.logger.Debug("Using deprecated ListSessionsTyped, consider migrating to optimized interface")
	return a.optimized.List(ctx)
}

func (a *compatibilityAdapter) ListSessionSummaries(ctx context.Context) ([]*SessionSummary, error) {
	// ListSessionSummaries - convert full sessions to summaries
	a.logger.Debug("Using deprecated ListSessionSummaries, consider migrating to optimized interface")

	sessions, err := a.optimized.List(ctx)
	if err != nil {
		return nil, err
	}

	summaries := make([]*SessionSummary, len(sessions))
	for i, session := range sessions {
		summaries[i] = &SessionSummary{
			ID:     session.SessionID,
			Labels: session.Labels,
		}
	}

	return summaries, nil
}

func (a *compatibilityAdapter) UpdateJobStatus(ctx context.Context, sessionID, jobID string, status JobStatus, result interface{}, err error) error {
	// UpdateJobStatus - implement using the Update method
	a.logger.Debug("Using deprecated UpdateJobStatus, consider migrating to optimized interface")

	return a.optimized.Update(ctx, sessionID, func(session *SessionState) error {
		// Store job status in metadata
		if session.Metadata == nil {
			session.Metadata = make(map[string]interface{})
		}

		jobData := map[string]interface{}{
			"status": string(status),
		}

		if result != nil {
			jobData["result"] = result
		}

		if err != nil {
			jobData["error"] = err.Error()
		}

		// Store job data under the job ID
		session.Metadata["job_"+jobID] = jobData

		return nil
	})
}

func (a *compatibilityAdapter) StartCleanupRoutine(ctx context.Context) error {
	// StartCleanupRoutine is no longer needed - the optimized manager handles cleanup automatically
	a.logger.Debug("StartCleanupRoutine is deprecated - optimized manager handles cleanup automatically")
	return nil
}
