// Package session provides optimized session management with a simplified API.
package session

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/common/errors"
)

// OptimizedSessionManager provides a streamlined session management interface
type OptimizedSessionManager interface {
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

// optimizedSessionManager implements the streamlined interface
type optimizedSessionManager struct {
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
	sweeper     *sessionSweeper
}

// sessionSweeper handles background cleanup in a separate component
type sessionSweeper struct {
	manager  *optimizedSessionManager
	interval time.Duration
	stopCh   chan struct{}
	doneCh   chan struct{}
}

// sessionEntry represents a session with its metadata
type sessionEntry struct {
	*SessionState
	expiresAt  time.Time
	labels     map[string]string
	lastAccess time.Time
}

// NewOptimizedSessionManager creates a new streamlined session manager
func NewOptimizedSessionManager(logger *slog.Logger, defaultTTL time.Duration, maxSessions int) OptimizedSessionManager {
	manager := &optimizedSessionManager{
		logger:          logger.With("component", "session_manager"),
		sessions:        make(map[string]*sessionEntry),
		defaultTTL:      defaultTTL,
		cleanupInterval: defaultTTL / 4,
		maxSessions:     maxSessions,
		cleanupDone:     make(chan struct{}),
		cleanupStop:     make(chan struct{}),
	}

	// Create and start the sweeper
	manager.sweeper = &sessionSweeper{
		manager:  manager,
		interval: manager.cleanupInterval,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
	go manager.sweeper.run()

	return manager
}

// Get retrieves a session, updating its last access time
func (m *optimizedSessionManager) Get(ctx context.Context, sessionID string) (*SessionState, error) {
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

// GetOrCreate gets an existing session or creates a new one
func (m *optimizedSessionManager) GetOrCreate(ctx context.Context, sessionID string) (*SessionState, error) {
	// Try to get existing session first
	if session, err := m.Get(ctx, sessionID); err == nil {
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
	session := &SessionState{
		SessionID: sessionID,
		CreatedAt: now,
		UpdatedAt: now,
		ExpiresAt: now.Add(m.defaultTTL),
		Status:    "created",
		Stage:     "init",
		Labels:    make(map[string]string),
		Metadata:  make(map[string]interface{}),
	}

	entry := &sessionEntry{
		SessionState: session,
		expiresAt:    session.ExpiresAt,
		labels:       session.Labels,
		lastAccess:   now,
	}

	m.sessions[sessionID] = entry
	m.logger.Info("Session created", "session_id", sessionID, "expires_at", session.ExpiresAt)

	return session, nil
}

// Update modifies a session using an update function
func (m *optimizedSessionManager) Update(ctx context.Context, sessionID string, updateFunc func(*SessionState) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.sessions[sessionID]
	if !exists {
		return errors.New(errors.CodeNotFound, "session", "session not found", nil)
	}

	// Check if expired
	if time.Now().After(entry.expiresAt) {
		delete(m.sessions, sessionID)
		m.logger.Debug("Session expired and removed", "session_id", sessionID)
		return errors.New(errors.CodeNotFound, "session", "session expired", nil)
	}

	// Apply the update function
	if err := updateFunc(entry.SessionState); err != nil {
		return err
	}

	// Update timestamps
	entry.lastAccess = time.Now()
	entry.UpdatedAt = time.Now()

	m.logger.Debug("Session updated", "session_id", sessionID)
	return nil
}

// List returns all active (non-expired) sessions
func (m *optimizedSessionManager) List(ctx context.Context) ([]*SessionState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	var sessions []*SessionState

	for sessionID, entry := range m.sessions {
		if now.Before(entry.expiresAt) {
			sessions = append(sessions, entry.SessionState)
		} else {
			// Note: We don't delete expired sessions here to avoid modifying
			// the map during a read lock. The sweeper will clean them up.
			m.logger.Debug("Found expired session during list", "session_id", sessionID)
		}
	}

	return sessions, nil
}

// Stats returns current session statistics
func (m *optimizedSessionManager) Stats() *SessionStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	activeSessions := 0

	for _, entry := range m.sessions {
		if now.Before(entry.expiresAt) {
			activeSessions++
		}
	}

	return &SessionStats{
		ActiveSessions: activeSessions,
		TotalSessions:  len(m.sessions),
		MaxSessions:    m.maxSessions,
	}
}

// Stop shuts down the session manager and its background processes
func (m *optimizedSessionManager) Stop(ctx context.Context) error {
	m.logger.Info("Stopping session manager")

	// Stop the sweeper
	close(m.sweeper.stopCh)

	// Wait for sweeper to finish or context to cancel
	select {
	case <-m.sweeper.doneCh:
		m.logger.Info("Session sweeper stopped gracefully")
	case <-ctx.Done():
		m.logger.Warn("Session sweeper stop timed out")
	}

	// Clear all sessions
	m.mu.Lock()
	m.sessions = make(map[string]*sessionEntry)
	m.mu.Unlock()

	m.logger.Info("Session manager stopped")
	return nil
}

// evictOldestSession removes the session with the oldest last access time
func (m *optimizedSessionManager) evictOldestSession() {
	if len(m.sessions) == 0 {
		return
	}

	var oldestSessionID string
	var oldestTime time.Time

	for sessionID, entry := range m.sessions {
		if oldestSessionID == "" || entry.lastAccess.Before(oldestTime) {
			oldestSessionID = sessionID
			oldestTime = entry.lastAccess
		}
	}

	if oldestSessionID != "" {
		delete(m.sessions, oldestSessionID)
		m.logger.Info("Evicted oldest session", "session_id", oldestSessionID, "last_access", oldestTime)
	}
}

// sessionSweeper methods

// run starts the background cleanup routine
func (s *sessionSweeper) run() {
	defer close(s.doneCh)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.cleanup()
		case <-s.stopCh:
			s.manager.logger.Debug("Session sweeper received stop signal")
			return
		}
	}
}

// cleanup removes expired sessions
func (s *sessionSweeper) cleanup() {
	s.manager.mu.Lock()
	defer s.manager.mu.Unlock()

	now := time.Now()
	var expiredSessions []string

	for sessionID, entry := range s.manager.sessions {
		if now.After(entry.expiresAt) {
			expiredSessions = append(expiredSessions, sessionID)
		}
	}

	for _, sessionID := range expiredSessions {
		delete(s.manager.sessions, sessionID)
	}

	if len(expiredSessions) > 0 {
		s.manager.logger.Debug("Cleaned up expired sessions", "count", len(expiredSessions))
	}
}
