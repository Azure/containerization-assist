package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/rs/zerolog"
)

// SessionManager manages MCP sessions
type SessionManager struct {
	sessions     map[string]*SessionState
	mutex        sync.RWMutex
	workspaceDir string
	maxSessions  int
	sessionTTL   time.Duration

	store SessionStore

	maxDiskPerSession int64
	totalDiskLimit    int64

	logger zerolog.Logger

	cleanupTicker *time.Ticker
	cleanupDone   chan bool
	stopped       bool

	// Statistics tracking
	startTime time.Time
	diskUsage map[string]int64
}

// SessionManagerConfig represents session manager configuration
type SessionManagerConfig struct {
	WorkspaceDir      string
	MaxSessions       int
	SessionTTL        time.Duration
	MaxDiskPerSession int64
	TotalDiskLimit    int64
	StorePath         string
	Logger            zerolog.Logger
}

// NewSessionManager creates a new SessionManager
func NewSessionManager(config SessionManagerConfig) (*SessionManager, error) {
	if err := os.MkdirAll(config.WorkspaceDir, 0o750); err != nil {
		config.Logger.Error().Err(err).Str("path", config.WorkspaceDir).Msg("Failed to create workspace directory")
		return nil, fmt.Errorf("failed to create workspace directory %s: %w", config.WorkspaceDir, err)
	}

	var store SessionStore
	var err error

	if config.StorePath != "" {
		store, err = NewBoltSessionStore(context.Background(), config.StorePath)
		if err != nil {
			config.Logger.Error().Err(err).Str("store_path", config.StorePath).Msg("Failed to initialize bolt store")
			return nil, fmt.Errorf("failed to initialize bolt store at %s: %w", config.StorePath, err)
		}
	} else {
		store = NewMemorySessionStore()
	}

	sm := &SessionManager{
		sessions:          make(map[string]*SessionState),
		workspaceDir:      config.WorkspaceDir,
		maxSessions:       config.MaxSessions,
		sessionTTL:        config.SessionTTL,
		store:             store,
		maxDiskPerSession: config.MaxDiskPerSession,
		totalDiskLimit:    config.TotalDiskLimit,
		logger:            config.Logger,
		cleanupDone:       make(chan bool),
		startTime:         time.Now(),
		diskUsage:         make(map[string]int64),
	}

	if err := sm.loadExistingSessions(); err != nil {
		sm.logger.Warn().Err(err).Msg("Failed to load existing sessions")
	}

	return sm, nil
}

// getOrCreateSessionConcrete retrieves or creates session
func (sm *SessionManager) getOrCreateSessionConcrete(sessionID string) (*SessionState, error) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if session, exists := sm.sessions[sessionID]; exists {
		session.UpdateLastAccessed()
		return session, nil
	}

	if session, err := sm.store.Load(context.Background(), sessionID); err == nil {
		sm.sessions[sessionID] = session
		session.UpdateLastAccessed()
		sm.logger.Info().Str("session_id", sessionID).Msg("Loaded session from persistence")
		return session, nil
	}

	if sessionID == "" {
		sessionID = generateSessionID()
	}

	if len(sm.sessions) >= sm.maxSessions {
		// Automatically clean up the oldest session to make room
		if err := sm.cleanupOldestSession(); err != nil {
			return nil, fmt.Errorf("maximum number of sessions (%d) reached and failed to cleanup oldest session: %w", sm.maxSessions, err)
		}
	}

	if err := sm.checkGlobalDiskQuota(); err != nil {
		return nil, err
	}

	workspaceDir := filepath.Join(sm.workspaceDir, sessionID)
	if err := os.MkdirAll(workspaceDir, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create session workspace: %w", err)
	}

	session := NewSessionStateWithTTL(sessionID, workspaceDir, sm.sessionTTL)
	session.MaxDiskUsage = sm.maxDiskPerSession

	sm.sessions[sessionID] = session

	if err := sm.store.Save(context.Background(), sessionID, session); err != nil {
		sm.logger.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to persist new session")
	}

	sm.logger.Info().Str("session_id", sessionID).Msg("Created new session")
	return session, nil
}

// UpdateSession updates and persists session
func (sm *SessionManager) UpdateSession(sessionID string, updater func(interface{})) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	updater(session)
	session.UpdateLastAccessed()

	if err := sm.store.Save(context.Background(), sessionID, session); err != nil {
		sm.logger.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to persist session update")
		return err
	}

	return nil
}

// UpdateSessionTyped updates session with typed function
func (sm *SessionManager) UpdateSessionTyped(sessionID string, updater func(*SessionState)) error {
	return sm.UpdateSession(sessionID, func(s interface{}) {
		if session, ok := s.(*SessionState); ok {
			updater(session)
		}
	})
}

// GetSessionConcrete retrieves session by ID
func (sm *SessionManager) GetSessionConcrete(sessionID string) (*SessionState, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	if session, exists := sm.sessions[sessionID]; exists {
		return session, nil
	}

	return nil, fmt.Errorf("session not found: %s", sessionID)
}

// GetSessionInterface implements ToolSessionManager interface
func (sm *SessionManager) GetSessionInterface(sessionID string) (interface{}, error) {
	session, err := sm.GetSessionConcrete(sessionID)
	if err != nil {
		return nil, err
	}
	return session, nil
}

// GetSession implements ToolSessionManager interface
func (sm *SessionManager) GetSession(sessionID string) (interface{}, error) {
	session, err := sm.GetSessionConcrete(sessionID)
	if err != nil {
		return nil, err
	}
	return session, nil
}

// GetOrCreateSession implements ToolSessionManager interface
func (sm *SessionManager) GetOrCreateSession(sessionID string) (interface{}, error) {
	session, err := sm.getOrCreateSessionConcrete(sessionID)
	if err != nil {
		return nil, err
	}
	return session, nil
}

// CreateSession implements ToolSessionManager interface
func (sm *SessionManager) CreateSession(userID string) (interface{}, error) {
	// Generate a new session ID, optionally incorporating userID for uniqueness
	sessionID := generateSessionID()
	if userID != "" {
		// Store user association in session metadata
		session, err := sm.getOrCreateSessionConcrete(sessionID)
		if err != nil {
			return nil, err
		}
		if session.Metadata == nil {
			session.Metadata = make(map[string]interface{})
		}
		session.Metadata["user_id"] = userID
		session.UpdateLastAccessed()
		// Save the updated session
		if err := sm.store.Save(context.Background(), sessionID, session); err != nil {
			sm.logger.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to save session with user metadata")
		}
		return session, nil
	}
	// Create session without user association
	return sm.getOrCreateSessionConcrete(sessionID)
}

// ListSessionSummaries returns a list of all session summaries
func (sm *SessionManager) ListSessionSummaries() []SessionSummary {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	summaries := make([]SessionSummary, 0, len(sm.sessions))
	for _, session := range sm.sessions {
		summaries = append(summaries, session.GetSummary())
	}

	return summaries
}

// DeleteSession removes a session and cleans up its workspace
func (sm *SessionManager) DeleteSession(ctx context.Context, sessionID string) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Clean up workspace
	if err := os.RemoveAll(session.WorkspaceDir); err != nil {
		sm.logger.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to clean up workspace")
	}

	// Remove from memory
	delete(sm.sessions, sessionID)

	// Remove from persistence
	if err := sm.store.Delete(context.Background(), sessionID); err != nil {
		sm.logger.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to remove session from persistence")
		return err
	}

	sm.logger.Info().Str("session_id", sessionID).Msg("Deleted session")
	return nil
}

// FindSessionByRepo finds a session by repository URL
func (sm *SessionManager) FindSessionByRepo(ctx context.Context, repoURL string) (interface{}, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	for _, session := range sm.sessions {
		// Check if repository URL matches
		if session.RepoURL == repoURL {
			return session, nil
		}
	}

	return nil, fmt.Errorf("no session found for repository URL: %s", repoURL)
}

// ListSessions (interface compatible) returns sessions with optional filtering
func (sm *SessionManager) ListSessions(ctx context.Context, filter map[string]interface{}) ([]interface{}, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	// Convert sessions to interface{} slice for compatibility
	var results []interface{}
	for _, session := range sm.sessions {
		// Apply basic filtering if provided
		if filter != nil {
			// Simple filter implementation - could be expanded
			if status, ok := filter["status"]; ok && status != "active" {
				continue
			}
		}
		results = append(results, session)
	}

	return results, nil
}

// GetOrCreateSession (interface compatible) for ToolSessionManager interface
func (sm *SessionManager) GetOrCreateSessionFromRepo(repoURL string) (interface{}, error) {
	// First try to find an existing session for this repo
	if session, err := sm.FindSessionByRepo(context.Background(), repoURL); err == nil {
		return session, nil
	}

	// If not found, create a new session with a random ID
	sessionID := fmt.Sprintf("session-%d", time.Now().Unix())
	session, err := sm.getOrCreateSessionConcrete(sessionID)
	if err != nil {
		return nil, err
	}

	// Update the session with repo URL
	err = sm.UpdateSession(session.SessionID, func(s interface{}) {
		if state, ok := s.(*SessionState); ok {
			state.RepoURL = repoURL
		}
	})
	if err != nil {
		return nil, err
	}

	return session, nil
}

// GarbageCollect removes expired sessions and cleans up resources
func (sm *SessionManager) GarbageCollect() error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	return sm.garbageCollectUnsafe()
}

// garbageCollectUnsafe removes expired sessions without acquiring mutex (caller must hold mutex)
func (sm *SessionManager) garbageCollectUnsafe() error {
	var expiredSessions []string

	// Identify expired sessions
	for sessionID, session := range sm.sessions {
		if session.IsExpired() {
			expiredSessions = append(expiredSessions, sessionID)
		}
	}

	// Remove expired sessions
	for _, sessionID := range expiredSessions {
		if err := sm.deleteSessionUnsafe(sessionID); err != nil {
			sm.logger.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to delete expired session")
		}
	}

	// Clean up orphaned workspaces
	if err := sm.cleanupOrphanedWorkspaces(); err != nil {
		sm.logger.Warn().Err(err).Msg("Failed to clean up orphaned workspaces")
	}

	// Clean up expired sessions from persistence (only for BoltSessionStore)
	if boltStore, ok := sm.store.(*BoltSessionStore); ok {
		if err := boltStore.CleanupExpired(context.Background(), sm.sessionTTL); err != nil {
			sm.logger.Warn().Err(err).Msg("Failed to clean up expired sessions from persistence")
		}
	}

	sm.logger.Info().Int("cleaned_count", len(expiredSessions)).Msg("Garbage collection completed")
	return nil
}

// CheckDiskQuota checks if a session can allocate additional disk space
func (sm *SessionManager) CheckDiskQuota(sessionID string, additionalBytes int64) error {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Check per-session quota only if a limit is set
	if session.MaxDiskUsage > 0 && session.DiskUsage+additionalBytes > session.MaxDiskUsage {
		return fmt.Errorf("session disk quota exceeded: %d + %d > %d",
			session.DiskUsage, additionalBytes, session.MaxDiskUsage)
	}

	// Check global quota only if a limit is set
	if sm.totalDiskLimit > 0 {
		totalUsage := sm.getTotalDiskUsage()
		if totalUsage+additionalBytes > sm.totalDiskLimit {
			return fmt.Errorf("global disk quota exceeded: %d + %d > %d",
				totalUsage, additionalBytes, sm.totalDiskLimit)
		}
	}

	return nil
}

// StartCleanupRoutine starts a background cleanup routine
func (sm *SessionManager) StartCleanupRoutine() {
	sm.cleanupTicker = time.NewTicker(1 * time.Hour)

	go func() {
		for {
			select {
			case <-sm.cleanupTicker.C:
				if err := sm.GarbageCollect(); err != nil {
					sm.logger.Error().Err(err).Msg("Garbage collection failed")
				}
			case <-sm.cleanupDone:
				return
			}
		}
	}()

	sm.logger.Info().Msg("Started session cleanup routine")
}

// Stop gracefully stops the session manager
func (sm *SessionManager) Stop() error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// Check if already stopped to prevent double-close race condition
	if sm.stopped {
		sm.logger.Debug().Msg("SessionManager already stopped")
		return nil
	}

	sm.stopped = true

	if sm.cleanupTicker != nil {
		sm.cleanupTicker.Stop()
		close(sm.cleanupDone)
	}

	// Final garbage collection (unsafe version since we already hold the mutex)
	if err := sm.garbageCollectUnsafe(); err != nil {
		sm.logger.Warn().Err(err).Msg("Final garbage collection failed")
	}

	// Close persistence store
	if err := sm.store.Close(); err != nil {
		return fmt.Errorf("failed to close session store: %w", err)
	}

	sm.logger.Info().Msg("Session manager stopped")
	return nil
}

// AddSessionLabel adds a label to a session
func (sm *SessionManager) AddSessionLabel(sessionID, label string) error {
	return sm.UpdateSession(sessionID, func(s interface{}) {
		if session, ok := s.(*SessionState); ok {
			session.AddLabel(label)
		}
	})
}

// RemoveSessionLabel removes a label from a session
func (sm *SessionManager) RemoveSessionLabel(sessionID, label string) error {
	return sm.UpdateSession(sessionID, func(s interface{}) {
		if session, ok := s.(*SessionState); ok {
			session.RemoveLabel(label)
		}
	})
}

// SetSessionLabels replaces all labels for a session
func (sm *SessionManager) SetSessionLabels(sessionID string, labels []string) error {
	return sm.UpdateSession(sessionID, func(s interface{}) {
		if session, ok := s.(*SessionState); ok {
			session.SetLabels(labels)
		}
	})
}

// GetSessionsByLabel returns sessions that have the specified label
func (sm *SessionManager) GetSessionsByLabel(label string) []SessionSummary {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	var results []SessionSummary
	for _, session := range sm.sessions {
		if session.HasLabel(label) {
			results = append(results, session.GetSummary())
		}
	}
	return results
}

// GetAllLabels returns all unique labels across all sessions
func (sm *SessionManager) GetAllLabels() []string {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	labelSet := make(map[string]bool)
	for _, session := range sm.sessions {
		for _, label := range session.Labels {
			labelSet[label] = true
		}
	}

	labels := make([]string, 0, len(labelSet))
	for label := range labelSet {
		labels = append(labels, label)
	}
	return labels
}

// ListSessionsFiltered returns sessions filtered by multiple criteria including labels
func (sm *SessionManager) ListSessionsFiltered(filters SessionFilters) []SessionSummary {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	var results []SessionSummary
	for _, session := range sm.sessions {
		if sm.matchesFilters(session, filters) {
			results = append(results, session.GetSummary())
		}
	}
	return results
}

// GetStats returns statistics about the session manager
func (sm *SessionManager) GetStats() *core.SessionManagerStats {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	totalSessions := len(sm.sessions)
	activeSessions := 0
	failedSessions := 0
	totalAge := 0.0

	for _, session := range sm.sessions {
		if !session.IsExpired() {
			activeSessions++
			age := time.Since(session.CreatedAt).Minutes()
			totalAge += age
		} else {
			failedSessions++
		}
	}

	averageAge := 0.0
	if activeSessions > 0 {
		averageAge = totalAge / float64(activeSessions)
	}

	stats := &core.SessionManagerStats{
		ActiveSessions:    activeSessions,
		TotalSessions:     totalSessions,
		FailedSessions:    failedSessions,
		AverageSessionAge: averageAge,
		SessionErrors:     0, // TODO: implement error tracking
	}

	return stats
}

// GetAllSessions implements ListSessionsManager interface
func (sm *SessionManager) GetAllSessions() ([]*SessionData, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	var sessions []*SessionData
	for _, session := range sm.sessions {
		sessionData := &SessionData{
			ID:             session.SessionID,
			State:          session,
			CreatedAt:      session.CreatedAt,
			UpdatedAt:      session.LastAccessed,
			ExpiresAt:      session.ExpiresAt,
			WorkspacePath:  session.WorkspaceDir,
			DiskUsage:      sm.diskUsage[session.SessionID],
			ActiveJobs:     []string{}, // TODO: implement actual job tracking
			CompletedTools: []string{}, // TODO: implement actual tool tracking
			LastError:      "",         // TODO: implement error tracking
			Labels:         []string{}, // TODO: implement label support
			RepoURL:        session.RepoURL,
			Metadata:       make(map[string]string), // TODO: convert from interface{} metadata
		}
		sessions = append(sessions, sessionData)
	}
	return sessions, nil
}

// GetSessionData implements ListSessionsManager interface (SessionData version)
func (sm *SessionManager) GetSessionData(sessionID string) (*SessionData, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	sessionData := &SessionData{
		ID:             session.SessionID,
		State:          session,
		CreatedAt:      session.CreatedAt,
		UpdatedAt:      session.LastAccessed,
		ExpiresAt:      session.ExpiresAt,
		WorkspacePath:  session.WorkspaceDir,
		DiskUsage:      sm.diskUsage[session.SessionID],
		ActiveJobs:     []string{}, // TODO: implement actual job tracking
		CompletedTools: []string{}, // TODO: implement actual tool tracking
		LastError:      "",         // TODO: implement error tracking
		Labels:         []string{}, // TODO: implement label support
		RepoURL:        session.RepoURL,
		Metadata:       make(map[string]string), // TODO: convert from interface{} metadata
	}
	return sessionData, nil
}

// SessionFilters defines criteria for filtering sessions
type SessionFilters struct {
	Labels        []string   `json:"labels,omitempty"`         // Sessions must have ALL these labels
	AnyLabel      []string   `json:"any_label,omitempty"`      // Sessions must have ANY of these labels
	Status        string     `json:"status,omitempty"`         // active, expired, quota_exceeded
	RepoURL       string     `json:"repo_url,omitempty"`       // Filter by repository URL
	CreatedAfter  *time.Time `json:"created_after,omitempty"`  // Created after this time
	CreatedBefore *time.Time `json:"created_before,omitempty"` // Created before this time
}

// Helper methods

// matchesFilters checks if a session matches the given filters
func (sm *SessionManager) matchesFilters(session *SessionState, filters SessionFilters) bool {
	// Check ALL labels requirement
	if len(filters.Labels) > 0 {
		for _, requiredLabel := range filters.Labels {
			if !session.HasLabel(requiredLabel) {
				return false
			}
		}
	}

	// Check ANY label requirement
	if len(filters.AnyLabel) > 0 {
		hasAnyLabel := false
		for _, anyLabel := range filters.AnyLabel {
			if session.HasLabel(anyLabel) {
				hasAnyLabel = true
				break
			}
		}
		if !hasAnyLabel {
			return false
		}
	}

	// Check status
	if filters.Status != "" {
		sessionStatus := "active"
		if session.IsExpired() {
			sessionStatus = "expired"
		} else if session.HasExceededDiskQuota() {
			sessionStatus = "quota_exceeded"
		}
		if sessionStatus != filters.Status {
			return false
		}
	}

	// Check repository URL
	if filters.RepoURL != "" && session.RepoURL != filters.RepoURL {
		return false
	}

	// Check created after
	if filters.CreatedAfter != nil && session.CreatedAt.Before(*filters.CreatedAfter) {
		return false
	}

	// Check created before
	if filters.CreatedBefore != nil && session.CreatedAt.After(*filters.CreatedBefore) {
		return false
	}

	return true
}

func (sm *SessionManager) loadExistingSessions() error {
	sessionIDs, err := sm.store.List(context.Background())
	if err != nil {
		return err
	}

	for _, sessionID := range sessionIDs {
		session, err := sm.store.Load(context.Background(), sessionID)
		if err != nil {
			sm.logger.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to load session")
			continue
		}

		// Only load non-expired sessions
		if !session.IsExpired() {
			sm.sessions[sessionID] = session
		}
	}

	sm.logger.Info().Int("loaded_count", len(sm.sessions)).Msg("Loaded existing sessions")
	return nil
}

func (sm *SessionManager) deleteSessionUnsafe(sessionID string) error {
	session := sm.sessions[sessionID]

	// Clean up workspace
	if err := os.RemoveAll(session.WorkspaceDir); err != nil {
		sm.logger.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to clean up workspace")
	}

	// Remove from memory
	delete(sm.sessions, sessionID)

	// Remove from persistence
	return sm.store.Delete(context.Background(), sessionID)
}

func (sm *SessionManager) cleanupOrphanedWorkspaces() error {
	workspaces, err := os.ReadDir(sm.workspaceDir)
	if err != nil {
		return err
	}

	for _, workspace := range workspaces {
		if !workspace.IsDir() {
			continue
		}

		sessionID := workspace.Name()
		if _, exists := sm.sessions[sessionID]; !exists {
			// Orphaned workspace
			workspacePath := filepath.Join(sm.workspaceDir, sessionID)
			if err := os.RemoveAll(workspacePath); err != nil {
				sm.logger.Warn().Err(err).Str("workspace", workspacePath).Msg("Failed to clean up orphaned workspace")
			} else {
				sm.logger.Info().Str("workspace", workspacePath).Msg("Cleaned up orphaned workspace")
			}
		}
	}

	return nil
}

func (sm *SessionManager) getTotalDiskUsage() int64 {
	var total int64
	for _, session := range sm.sessions {
		total += session.DiskUsage
	}
	return total
}

func (sm *SessionManager) checkGlobalDiskQuota() error {
	// If totalDiskLimit is 0, it means no limit is set
	if sm.totalDiskLimit <= 0 {
		return nil
	}

	totalUsage := sm.getTotalDiskUsage()
	if totalUsage >= sm.totalDiskLimit {
		return fmt.Errorf("global disk quota exceeded: %d >= %d", totalUsage, sm.totalDiskLimit)
	}
	return nil
}

// generateSessionID creates a new random session ID
func generateSessionID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if random generation fails
		return fmt.Sprintf("session-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

// cleanupOldestSession removes the oldest session to make room for a new one
// IMPORTANT: This method must be called with sm.mutex already held
func (sm *SessionManager) cleanupOldestSession() error {
	if len(sm.sessions) == 0 {
		return nil
	}

	// Find the session with the oldest LastAccessed time
	var oldestSessionID string
	var oldestTime time.Time = time.Now()

	for sessionID, session := range sm.sessions {
		if session.LastAccessed.Before(oldestTime) {
			oldestTime = session.LastAccessed
			oldestSessionID = sessionID
		}
	}

	if oldestSessionID != "" {
		sm.logger.Info().
			Str("session_id", oldestSessionID).
			Time("last_accessed", oldestTime).
			Msg("Automatically cleaning up oldest session to make room for new session")

		return sm.deleteSessionInternal(oldestSessionID)
	}

	return nil
}

// deleteSessionInternal removes a session without acquiring the mutex (for internal use)
// IMPORTANT: This method must be called with sm.mutex already held
func (sm *SessionManager) deleteSessionInternal(sessionID string) error {
	return sm.deleteSessionInternalWithContext(context.Background(), sessionID)
}

// deleteSessionInternalWithContext removes a session with context support
// IMPORTANT: This method must be called with sm.mutex already held
func (sm *SessionManager) deleteSessionInternalWithContext(ctx context.Context, sessionID string) error {
	session, exists := sm.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Remove from memory
	delete(sm.sessions, sessionID)

	// Clean up workspace directory
	if err := os.RemoveAll(session.WorkspaceDir); err != nil {
		sm.logger.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to clean up session workspace")
	}

	// Remove from persistent store
	if err := sm.store.Delete(ctx, sessionID); err != nil {
		sm.logger.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to remove session from persistent store")
	}

	sm.logger.Info().Str("session_id", sessionID).Msg("Session cleaned up successfully")
	return nil
}

// SessionFromContext extracts session ID from context
func SessionFromContext(ctx context.Context) string {
	if sessionID, ok := ctx.Value("session_id").(string); ok {
		return sessionID
	}
	return ""
}
