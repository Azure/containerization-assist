package session

import (
	"context"
	"sort"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/core"
	errors "github.com/Azure/container-kit/pkg/mcp/errors"
)

// ListSessions implements UnifiedSessionManager interface
func (sm *SessionManager) ListSessions(ctx context.Context) ([]*SessionData, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sessions := make([]*SessionData, 0, len(sm.sessions))
	for _, session := range sm.sessions {
		sessions = append(sessions, &SessionData{
			ID:           session.ID,
			CreatedAt:    session.CreatedAt,
			UpdatedAt:    session.UpdatedAt,
			WorkspaceDir: session.WorkspaceDir,
			Metadata:     session.Metadata,
			Status:       session.Status,
			Labels:       session.Labels,
			DiskUsage:    session.DiskUsage,
		})
	}

	// Sort by creation time, newest first
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].CreatedAt.After(sessions[j].CreatedAt)
	})

	return sessions, nil
}

// ListSessionSummaries implements UnifiedSessionManager interface
func (sm *SessionManager) ListSessionSummaries(ctx context.Context) ([]SessionSummary, error) {
	return sm.ListSessionSummariesInternal(), nil
}

// ListSessionSummariesInternal returns summaries of all sessions
func (sm *SessionManager) ListSessionSummariesInternal() []SessionSummary {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	summaries := make([]SessionSummary, 0, len(sm.sessions))
	for _, session := range sm.sessions {
		summaries = append(summaries, SessionSummary{
			ID:        session.ID,
			CreatedAt: session.CreatedAt,
			UpdatedAt: session.UpdatedAt,
			Status:    session.Status,
			Labels:    session.Labels,
		})
	}

	// Sort by creation time, newest first
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].CreatedAt.After(summaries[j].CreatedAt)
	})

	return summaries
}

// GetSessionData implements UnifiedSessionManager interface
func (sm *SessionManager) GetSessionData(ctx context.Context, sessionID string) (*SessionData, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		// Try to load from persistent store
		loadedSession, err := sm.store.Load(ctx, sessionID)
		if err != nil {
			return nil, errors.NewError().Messagef("session not found: %s", sessionID).Build()
		}
		session = loadedSession
	}

	return &SessionData{
		ID:           session.ID,
		CreatedAt:    session.CreatedAt,
		UpdatedAt:    session.UpdatedAt,
		WorkspaceDir: session.WorkspaceDir,
		Metadata:     session.Metadata,
		Status:       session.Status,
		Labels:       session.Labels,
		DiskUsage:    session.DiskUsage,
	}, nil
}

// GetStats implements UnifiedSessionManager interface
func (sm *SessionManager) GetStats(ctx context.Context) (*core.SessionManagerStats, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	stats := &core.SessionManagerStats{
		TotalSessions:   len(sm.sessions),
		ActiveSessions:  0,
		ExpiredSessions: 0,
		TotalDiskUsage:  0,
		// SessionsByLabel: make(map[string]int), // Field doesn't exist in core.SessionManagerStats
	}

	now := time.Now()
	for _, session := range sm.sessions {
		// Calculate disk usage
		stats.TotalDiskUsage += session.DiskUsage

		// Count by status
		switch session.Status {
		case SessionStatusActive:
			stats.ActiveSessions++
		case SessionStatusExpired:
			stats.ExpiredSessions++
		}

		// Check if session is expired by TTL
		if sm.sessionTTL > 0 && now.Sub(session.UpdatedAt) > sm.sessionTTL {
			stats.ExpiredSessions++
		}

		// Count by labels
		// for _, label := range session.Labels {
		// 	stats.SessionsByLabel[label]++
		// }
	}

	return stats, nil
}

// HasSession checks if a session exists
func (sm *SessionManager) HasSession(sessionID string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	_, exists := sm.sessions[sessionID]
	return exists
}

// GetAllSessionsLegacy returns all sessions (internal use)
func (sm *SessionManager) GetAllSessionsLegacy() map[string]*SessionState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// Create a copy to avoid external modifications
	sessionsCopy := make(map[string]*SessionState, len(sm.sessions))
	for id, session := range sm.sessions {
		sessionsCopy[id] = session
	}
	return sessionsCopy
}
