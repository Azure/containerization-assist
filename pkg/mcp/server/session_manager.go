package server

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/errors"
)

// memorySessionManager is a production-ready in-memory session manager MVP
// that can be swapped out for Redis or other storage later
type memorySessionManager struct {
	logger *slog.Logger
	mu     sync.RWMutex
	
	// Core storage
	sessions map[string]*sessionEntry
	
	// Configuration
	defaultTTL      time.Duration
	cleanupInterval time.Duration
	maxSessions     int
	
	// Background cleanup
	cleanupDone chan struct{}
	cleanupStop chan struct{}
}

// sessionEntry wraps SessionState with additional metadata for efficient management
type sessionEntry struct {
	*SessionState
	expiresAt time.Time
	labels    map[string]string
	lastAccess time.Time
}

// newMemorySessionManager creates a new production-ready in-memory session manager
func newMemorySessionManager(logger *slog.Logger, defaultTTL time.Duration, maxSessions int) SessionManager {
	manager := &memorySessionManager{
		logger:          logger.With("component", "session_manager"),
		sessions:        make(map[string]*sessionEntry),
		defaultTTL:      defaultTTL,
		cleanupInterval: defaultTTL / 4, // Clean up 4x more frequently than TTL
		maxSessions:     maxSessions,
		cleanupDone:     make(chan struct{}),
		cleanupStop:     make(chan struct{}),
	}
	
	return manager
}

// GetSession retrieves a session, updating its last access time
func (m *memorySessionManager) GetSession(ctx context.Context, sessionID string) (*SessionState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	entry, exists := m.sessions[sessionID]
	if !exists {
		return nil, errors.New(errors.CodeNotFound, "session", "session not found", nil)
	}
	
	// Check if expired
	if time.Now().After(entry.expiresAt) {
		delete(m.sessions, sessionID)
		m.logger.Debug("Session expired and removed", "session_id", sessionID)
		return nil, errors.New(errors.CodeNotFound, "session", "session expired", nil)
	}
	
	// Update last access time
	entry.lastAccess = time.Now()
	entry.UpdatedAt = time.Now()
	
	return entry.SessionState, nil
}

// GetOrCreateSession gets an existing session or creates a new one
func (m *memorySessionManager) GetOrCreateSession(ctx context.Context, sessionID string) (*SessionState, error) {
	// Try to get existing session first
	if session, err := m.GetSession(ctx, sessionID); err == nil {
		return session, nil
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Double-check after acquiring lock
	if entry, exists := m.sessions[sessionID]; exists && time.Now().Before(entry.expiresAt) {
		entry.lastAccess = time.Now()
		entry.UpdatedAt = time.Now()
		return entry.SessionState, nil
	}
	
	// Check session limit
	if len(m.sessions) >= m.maxSessions {
		// Remove oldest session to make room
		m.evictOldestSession()
	}
	
	// Create new session
	now := time.Now()
	sessionState := &SessionState{
		SessionID: sessionID,
		CreatedAt: now,
		UpdatedAt: now,
		ExpiresAt: now.Add(m.defaultTTL),
		Status:    "active",
		Stage:     "initialized",
		Labels:    make(map[string]string),
		Metadata:  make(map[string]interface{}),
	}
	
	entry := &sessionEntry{
		SessionState: sessionState,
		expiresAt:    now.Add(m.defaultTTL),
		labels:       make(map[string]string),
		lastAccess:   now,
	}
	
	m.sessions[sessionID] = entry
	m.logger.Debug("Created new session", 
		"session_id", sessionID, 
		"total_sessions", len(m.sessions),
		"expires_at", entry.expiresAt)
	
	return sessionState, nil
}

// GetSessionTyped implements SessionManager interface
func (m *memorySessionManager) GetSessionTyped(ctx context.Context, sessionID string) (*SessionState, error) {
	return m.GetSession(ctx, sessionID)
}

// GetSessionConcrete implements SessionManager interface  
func (m *memorySessionManager) GetSessionConcrete(ctx context.Context, sessionID string) (*SessionState, error) {
	return m.GetSession(ctx, sessionID)
}

// GetOrCreateSessionTyped implements SessionManager interface
func (m *memorySessionManager) GetOrCreateSessionTyped(ctx context.Context, sessionID string) (*SessionState, error) {
	return m.GetOrCreateSession(ctx, sessionID)
}

// UpdateSession updates a session using a function
func (m *memorySessionManager) UpdateSession(ctx context.Context, sessionID string, updateFunc func(*SessionState) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	entry, exists := m.sessions[sessionID]
	if !exists {
		return errors.New(errors.CodeNotFound, "session", "session not found", nil)
	}
	
	// Check if expired
	if time.Now().After(entry.expiresAt) {
		delete(m.sessions, sessionID)
		return errors.New(errors.CodeNotFound, "session", "session expired", nil)
	}
	
	// Apply update function
	if err := updateFunc(entry.SessionState); err != nil {
		return err
	}
	
	// Update timestamps
	entry.lastAccess = time.Now()
	entry.SessionState.UpdatedAt = time.Now()
	
	m.logger.Debug("Updated session", "session_id", sessionID)
	return nil
}

// ListSessionsTyped returns all active sessions
func (m *memorySessionManager) ListSessionsTyped(ctx context.Context) ([]*SessionState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	now := time.Now()
	sessions := make([]*SessionState, 0, len(m.sessions))
	
	for sessionID, entry := range m.sessions {
		if now.Before(entry.expiresAt) {
			sessions = append(sessions, entry.SessionState)
		} else {
			// Mark for cleanup
			m.logger.Debug("Found expired session during list", "session_id", sessionID)
		}
	}
	
	return sessions, nil
}

// ListSessionSummaries returns session summaries
func (m *memorySessionManager) ListSessionSummaries(ctx context.Context) ([]*SessionSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	now := time.Now()
	summaries := make([]*SessionSummary, 0, len(m.sessions))
	
	for sessionID, entry := range m.sessions {
		if now.Before(entry.expiresAt) {
			summaries = append(summaries, &SessionSummary{
				ID:     sessionID,
				Labels: entry.labels,
			})
		}
	}
	
	return summaries, nil
}

// UpdateJobStatus tracks job status for a session
func (m *memorySessionManager) UpdateJobStatus(ctx context.Context, sessionID, jobID string, status JobStatus, result interface{}, err error) error {
	return m.UpdateSession(ctx, sessionID, func(session *SessionState) error {
		if session.Metadata == nil {
			session.Metadata = make(map[string]interface{})
		}
		
		jobs, exists := session.Metadata["jobs"]
		if !exists {
			jobs = make(map[string]interface{})
			session.Metadata["jobs"] = jobs
		}
		
		jobsMap := jobs.(map[string]interface{})
		jobsMap[jobID] = map[string]interface{}{
			"status": status,
			"result": result,
			"error":  err,
			"updated_at": time.Now(),
		}
		
		m.logger.Debug("Updated job status", 
			"session_id", sessionID, 
			"job_id", jobID, 
			"status", status)
		
		return nil
	})
}

// StartCleanupRoutine starts the background cleanup goroutine
func (m *memorySessionManager) StartCleanupRoutine(ctx context.Context) error {
	go m.runCleanup()
	m.logger.Info("Started session cleanup routine", 
		"interval", m.cleanupInterval,
		"default_ttl", m.defaultTTL)
	return nil
}

// Stop gracefully shuts down the session manager
func (m *memorySessionManager) Stop(ctx context.Context) error {
	close(m.cleanupStop)
	
	// Wait for cleanup to finish or context timeout
	select {
	case <-m.cleanupDone:
		m.logger.Info("Session manager stopped gracefully")
	case <-ctx.Done():
		m.logger.Warn("Session manager stop timed out")
	}
	
	return nil
}

// runCleanup runs the background cleanup routine
func (m *memorySessionManager) runCleanup() {
	defer close(m.cleanupDone)
	
	ticker := time.NewTicker(m.cleanupInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			m.cleanupExpiredSessions()
		case <-m.cleanupStop:
			m.logger.Debug("Cleanup routine stopping")
			return
		}
	}
}

// cleanupExpiredSessions removes expired sessions
func (m *memorySessionManager) cleanupExpiredSessions() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	now := time.Now()
	expiredCount := 0
	
	for sessionID, entry := range m.sessions {
		if now.After(entry.expiresAt) {
			delete(m.sessions, sessionID)
			expiredCount++
		}
	}
	
	if expiredCount > 0 {
		m.logger.Debug("Cleaned up expired sessions", 
			"expired_count", expiredCount,
			"remaining_sessions", len(m.sessions))
	}
}

// evictOldestSession removes the oldest session to make room for new ones
func (m *memorySessionManager) evictOldestSession() {
	if len(m.sessions) == 0 {
		return
	}
	
	var oldestID string
	var oldestTime time.Time
	
	for sessionID, entry := range m.sessions {
		if oldestID == "" || entry.lastAccess.Before(oldestTime) {
			oldestID = sessionID
			oldestTime = entry.lastAccess
		}
	}
	
	if oldestID != "" {
		delete(m.sessions, oldestID)
		m.logger.Debug("Evicted oldest session to make room", 
			"evicted_session_id", oldestID,
			"last_access", oldestTime)
	}
}