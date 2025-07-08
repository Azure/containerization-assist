package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	errors "github.com/Azure/container-kit/pkg/mcp/errors"
)

// StoreFactory is a function that creates a SessionStore
type StoreFactory func(context.Context, string) (SessionStore, error)

// DefaultStoreFactory is deprecated - use SessionService instead
// var DefaultStoreFactory StoreFactory // REMOVED: Global state eliminated

// NewSessionManager creates a new SessionManager
func NewSessionManager(config SessionManagerConfig) (*SessionManager, error) {
	if err := os.MkdirAll(config.WorkspaceDir, 0o750); err != nil {
		return nil, errors.NewError().Message("failed to create workspace directory").Cause(err).Build()
	}

	var store SessionStore
	if config.StorePath != "" {
		// Note: Use SessionService for store factory management instead of global state
		return nil, errors.NewError().Message("use SessionService.CreateSessionManager() instead of global DefaultStoreFactory").Build()
	} else {
		// Create a simple in-memory store
		store = NewMemoryStore()
	}

	sm := &SessionManager{
		sessions:     make(map[string]*SessionState),
		workspaceDir: config.WorkspaceDir,
		maxSessions:  config.MaxSessions,
		sessionTTL:   config.SessionTTL,
		store:        store,
		logger:       config.Logger.With("component", "session_manager"),
	}

	// Load existing sessions from persistent store
	if err := sm.loadExistingSessions(); err != nil {
		sm.logger.Warn("Failed to load existing sessions", "error", err)
	}

	// Clean up orphaned workspaces
	if err := sm.cleanupOrphanedWorkspaces(); err != nil {
		sm.logger.Warn("Failed to cleanup orphaned workspaces", "error", err)
	}

	return sm, nil
}

// CreateSession implements UnifiedSessionManager interface
func (sm *SessionManager) CreateSession(ctx context.Context, id string) (*SessionState, error) {
	if id == "" {
		id = generateSessionID()
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Check if session already exists
	if _, exists := sm.sessions[id]; exists {
		return nil, errors.NewError().Messagef("session already exists: %s", id).Build()
	}

	// Check session limit
	if len(sm.sessions) >= sm.maxSessions {
		// Try to cleanup expired sessions first
		if err := sm.garbageCollectUnsafe(); err != nil {
			sm.logger.Warn("Failed to garbage collect sessions", "error", err)
		}

		// If still at limit, cleanup oldest session
		if len(sm.sessions) >= sm.maxSessions {
			if err := sm.cleanupOldestSession(); err != nil {
				return nil, errors.NewError().Message("failed to cleanup oldest session").Cause(err).Build()
			}
		}
	}

	// Create new session
	sessionDir := filepath.Join(sm.workspaceDir, id)
	if err := os.MkdirAll(sessionDir, 0o750); err != nil {
		return nil, errors.NewError().Message("failed to create session directory").Cause(err).Build()
	}

	now := time.Now()
	session := &SessionState{
		ID:           id,
		SessionID:    id,
		CreatedAt:    now,
		UpdatedAt:    now,
		ExpiresAt:    now.Add(sm.sessionTTL),
		WorkspaceDir: sessionDir,
		Status:       "active",
		Metadata:     make(map[string]interface{}),
	}

	sm.sessions[id] = session

	// Save to persistent store
	if err := sm.store.Save(ctx, id, session); err != nil {
		// Remove from memory if save fails
		delete(sm.sessions, id)
		return nil, errors.NewError().Message("failed to save session").Cause(err).Build()
	}

	sm.logger.Info("Created new session", "session_id", id)
	return session, nil
}

// GetOrCreateSession implements UnifiedSessionManager interface
func (sm *SessionManager) GetOrCreateSession(ctx context.Context, sessionID string) (*SessionState, error) {
	return sm.getOrCreateSessionConcrete(sessionID)
}

// getOrCreateSessionConcrete gets or creates a session
func (sm *SessionManager) getOrCreateSessionConcrete(sessionID string) (*SessionState, error) {
	if sessionID == "" {
		sessionID = generateSessionID()
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Check if session exists
	if session, exists := sm.sessions[sessionID]; exists {
		session.UpdatedAt = time.Now()
		return session, nil
	}

	// Check session limit
	if len(sm.sessions) >= sm.maxSessions {
		// Try to cleanup expired sessions first
		if err := sm.garbageCollectUnsafe(); err != nil {
			sm.logger.Warn("Failed to garbage collect sessions", "error", err)
		}

		// If still at limit, cleanup oldest session
		if len(sm.sessions) >= sm.maxSessions {
			if err := sm.cleanupOldestSession(); err != nil {
				return nil, errors.NewError().Message("failed to cleanup oldest session").Cause(err).Build()
			}
		}
	}

	// Create new session
	sessionDir := filepath.Join(sm.workspaceDir, sessionID)
	if err := os.MkdirAll(sessionDir, 0o750); err != nil {
		return nil, errors.NewError().Message("failed to create session directory").Cause(err).Build()
	}

	now := time.Now()
	session := &SessionState{
		ID:           sessionID,
		SessionID:    sessionID,
		CreatedAt:    now,
		UpdatedAt:    now,
		WorkspaceDir: sessionDir,
		Metadata:     make(map[string]interface{}),
		Status:       SessionStatusActive,
		Labels:       []string{},
	}

	sm.sessions[sessionID] = session

	// Save to persistent store
	if err := sm.store.Save(context.Background(), sessionID, session); err != nil {
		sm.logger.Warn("Failed to save session to persistent store", "error", err, "session_id", sessionID)
	}

	sm.logger.Info("Created new session", "session_id", sessionID)
	return session, nil
}

// GetSession implements UnifiedSessionManager interface
func (sm *SessionManager) GetSession(ctx context.Context, sessionID string) (*SessionState, error) {
	return sm.GetSessionConcrete(sessionID)
}

// GetSessionConcrete gets a session by ID
func (sm *SessionManager) GetSessionConcrete(sessionID string) (*SessionState, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		// Try to load from persistent store
		loadedSession, err := sm.store.Load(context.Background(), sessionID)
		if err != nil {
			return nil, errors.NewError().Messagef("session not found: %s", sessionID).Build()
		}
		return loadedSession, nil
	}

	return session, nil
}

// UpdateSession implements UnifiedSessionManager interface
func (sm *SessionManager) UpdateSession(ctx context.Context, sessionID string, updater func(*SessionState) error) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return errors.NewError().Messagef("session not found: %s", sessionID).Build()
	}

	// Apply the updater function
	if err := updater(session); err != nil {
		return errors.NewError().Message("failed to update session").Cause(err).Build()
	}

	session.UpdatedAt = time.Now()

	// Save to persistent store
	if err := sm.store.Save(ctx, sessionID, session); err != nil {
		sm.logger.Warn("Failed to save updated session", "error", err, "session_id", sessionID)
	}

	return nil
}

// UpdateSessionLegacy updates a session (legacy interface)
func (sm *SessionManager) UpdateSessionLegacy(sessionID string, updater func(interface{})) error {
	return sm.UpdateSession(context.Background(), sessionID, func(session *SessionState) error {
		updater(session)
		return nil
	})
}

// DeleteSession implements UnifiedSessionManager interface
func (sm *SessionManager) DeleteSession(ctx context.Context, sessionID string) error {
	return sm.DeleteSessionTyped(ctx, sessionID)
}

// DeleteSessionTyped deletes a session with context
func (sm *SessionManager) DeleteSessionTyped(ctx context.Context, sessionID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return errors.NewError().Messagef("session not found: %s", sessionID).Build()
	}

	// Remove from memory
	delete(sm.sessions, sessionID)

	// Clean up workspace directory
	if err := os.RemoveAll(session.WorkspaceDir); err != nil {
		sm.logger.Warn("Failed to clean up session workspace", "error", err, "session_id", sessionID)
	}

	// Remove from persistent store
	if err := sm.store.Delete(ctx, sessionID); err != nil {
		sm.logger.Warn("Failed to remove session from persistent store", "error", err, "session_id", sessionID)
	}

	sm.logger.Info("Deleted session", "session_id", sessionID)
	return nil
}

// DeleteSessionLegacy deletes a session (legacy interface)
func (sm *SessionManager) DeleteSessionLegacy(sessionID string) error {
	return sm.DeleteSession(context.Background(), sessionID)
}

// loadExistingSessions loads sessions from persistent store
func (sm *SessionManager) loadExistingSessions() error {
	sessionIDs, err := sm.store.List(context.Background())
	if err != nil {
		return err
	}

	// Load each session by ID
	for _, sessionID := range sessionIDs {
		session, err := sm.store.Load(context.Background(), sessionID)
		if err != nil {
			sm.logger.Warn("Failed to load session", "error", err, "session_id", sessionID)
			continue
		}
		sm.sessions[sessionID] = session
	}

	sm.logger.Info("Loaded existing sessions", "count", len(sm.sessions))
	return nil
}

// cleanupOrphanedWorkspaces removes workspace directories without sessions
func (sm *SessionManager) cleanupOrphanedWorkspaces() error {
	entries, err := os.ReadDir(sm.workspaceDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			sessionID := entry.Name()
			if _, exists := sm.sessions[sessionID]; !exists {
				workspaceDir := filepath.Join(sm.workspaceDir, sessionID)
				if err := os.RemoveAll(workspaceDir); err != nil {
					sm.logger.Warn("Failed to clean up orphaned workspace", "error", err, "workspace", workspaceDir)
				} else {
					sm.logger.Info("Cleaned up orphaned workspace", "workspace", workspaceDir)
				}
			}
		}
	}

	return nil
}

// cleanupOldestSession removes the oldest session to make room
func (sm *SessionManager) cleanupOldestSession() error {
	var oldestSessionID string
	var oldestTime time.Time

	for id, session := range sm.sessions {
		if oldestSessionID == "" || session.UpdatedAt.Before(oldestTime) {
			oldestSessionID = id
			oldestTime = session.UpdatedAt
		}
	}

	if oldestSessionID != "" {
		return sm.deleteSessionInternal(oldestSessionID)
	}
	return nil
}

// deleteSessionInternal removes a session without acquiring the mutex
func (sm *SessionManager) deleteSessionInternal(sessionID string) error {
	session, exists := sm.sessions[sessionID]
	if !exists {
		return errors.NewError().Messagef("session not found: %s", sessionID).Build()
	}

	// Remove from memory
	delete(sm.sessions, sessionID)

	// Clean up workspace directory
	if err := os.RemoveAll(session.WorkspaceDir); err != nil {
		sm.logger.Warn("Failed to clean up session workspace", "error", err, "session_id", sessionID)
	}

	// Remove from persistent store
	if err := sm.store.Delete(context.Background(), sessionID); err != nil {
		sm.logger.Warn("Failed to remove session from persistent store", "error", err, "session_id", sessionID)
	}

	sm.logger.Info("Session cleaned up successfully", "session_id", sessionID)
	return nil
}

// generateSessionID generates a unique session ID
func generateSessionID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		// Fallback to timestamp-based ID
		return fmt.Sprintf("session_%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// SessionFromContext extracts session ID from context
func SessionFromContext(ctx context.Context) string {
	if sessionID, ok := ctx.Value("session_id").(string); ok {
		return sessionID
	}
	return ""
}

// Close implements UnifiedSessionManager interface
func (sm *SessionManager) Close() error {
	return sm.Stop()
}

// Stop closes the session manager and cleans up resources
func (sm *SessionManager) Stop() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Save all sessions to persistent store
	for id, session := range sm.sessions {
		if err := sm.store.Save(context.Background(), id, session); err != nil {
			sm.logger.Warn("Failed to save session during shutdown", "error", err, "session_id", id)
		}
	}

	// Close the store
	if err := sm.store.Close(); err != nil {
		return errors.NewError().Message("failed to close session store").Cause(err).Build()
	}

	sm.logger.Info("Session manager stopped")
	return nil
}
