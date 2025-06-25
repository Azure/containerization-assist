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

	"github.com/rs/zerolog"
)

// SessionManager manages MCP sessions with persistence and quotas
type SessionManager struct {
	sessions     map[string]*SessionState
	mutex        sync.RWMutex
	workspaceDir string
	maxSessions  int
	sessionTTL   time.Duration

	// Persistence layer
	store SessionStore

	// Resource quotas
	maxDiskPerSession int64
	totalDiskLimit    int64

	// Logger
	logger zerolog.Logger

	// Cleanup
	cleanupTicker *time.Ticker
	cleanupDone   chan bool
	stopped       bool // Track if already stopped to prevent double-close
}

// SessionManagerConfig holds configuration for the session manager
type SessionManagerConfig struct {
	WorkspaceDir      string
	MaxSessions       int
	SessionTTL        time.Duration
	MaxDiskPerSession int64
	TotalDiskLimit    int64
	StorePath         string
	Logger            zerolog.Logger
}

// NewSessionManager creates a new session manager with persistence
func NewSessionManager(config SessionManagerConfig) (*SessionManager, error) {
	// Create workspace directory if it doesn't exist
	if err := os.MkdirAll(config.WorkspaceDir, 0o750); err != nil {
		config.Logger.Error().Err(err).Str("path", config.WorkspaceDir).Msg("Failed to create workspace directory")
		return nil, fmt.Errorf("failed to create workspace directory %s: %w", config.WorkspaceDir, err)
	}

	// Initialize persistence store
	var store SessionStore
	var err error

	if config.StorePath != "" {
		store, err = NewBoltSessionStore(config.StorePath)
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
	}

	// Load existing sessions from persistence
	if err := sm.loadExistingSessions(); err != nil {
		sm.logger.Warn().Err(err).Msg("Failed to load existing sessions")
	}

	return sm, nil
}

// getOrCreateSessionConcrete retrieves an existing session or creates a new one
func (sm *SessionManager) getOrCreateSessionConcrete(sessionID string) (*SessionState, error) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// Check if session exists in memory
	if session, exists := sm.sessions[sessionID]; exists {
		session.UpdateLastAccessed()
		return session, nil
	}

	// Try to load from persistence
	if session, err := sm.store.Load(sessionID); err == nil {
		sm.sessions[sessionID] = session
		session.UpdateLastAccessed()
		sm.logger.Info().Str("session_id", sessionID).Msg("Loaded session from persistence")
		return session, nil
	}

	// Create new session if it doesn't exist
	if sessionID == "" {
		sessionID = generateSessionID()
	}

	// Check session limit
	if len(sm.sessions) >= sm.maxSessions {
		return nil, fmt.Errorf("maximum number of sessions (%d) reached", sm.maxSessions)
	}

	// Check total disk usage
	if err := sm.checkGlobalDiskQuota(); err != nil {
		return nil, err
	}

	// Create workspace for the session
	workspaceDir := filepath.Join(sm.workspaceDir, sessionID)
	if err := os.MkdirAll(workspaceDir, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create session workspace: %w", err)
	}

	session := NewSessionStateWithTTL(sessionID, workspaceDir, sm.sessionTTL)
	session.MaxDiskUsage = sm.maxDiskPerSession

	sm.sessions[sessionID] = session

	// Persist the new session
	if err := sm.store.Save(sessionID, session); err != nil {
		sm.logger.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to persist new session")
	}

	sm.logger.Info().Str("session_id", sessionID).Msg("Created new session")
	return session, nil
}

// UpdateSession updates a session and persists the changes (interface-compliant version)
func (sm *SessionManager) UpdateSession(sessionID string, updater func(interface{})) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	updater(session)
	session.UpdateLastAccessed()

	// Persist the changes
	if err := sm.store.Save(sessionID, session); err != nil {
		sm.logger.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to persist session update")
		return err
	}

	return nil
}

// UpdateSessionTyped updates a session with a typed function (for backward compatibility)
func (sm *SessionManager) UpdateSessionTyped(sessionID string, updater func(*SessionState)) error {
	return sm.UpdateSession(sessionID, func(s interface{}) {
		if session, ok := s.(*SessionState); ok {
			updater(session)
		}
	})
}

// GetSessionConcrete retrieves a session by ID with concrete return type
func (sm *SessionManager) GetSessionConcrete(sessionID string) (*SessionState, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	if session, exists := sm.sessions[sessionID]; exists {
		return session, nil
	}

	return nil, fmt.Errorf("session not found: %s", sessionID)
}

// GetSessionInterface (interface compatible) for ToolSessionManager interface
func (sm *SessionManager) GetSessionInterface(sessionID string) (interface{}, error) {
	session, err := sm.GetSessionConcrete(sessionID)
	if err != nil {
		return nil, err
	}
	return session, nil
}

// GetSession (interface override) for ToolSessionManager interface compatibility
func (sm *SessionManager) GetSession(sessionID string) (interface{}, error) {
	session, err := sm.GetSessionConcrete(sessionID)
	if err != nil {
		return nil, err
	}
	return session, nil
}

// GetOrCreateSession (interface override) for ToolSessionManager interface compatibility
func (sm *SessionManager) GetOrCreateSession(sessionID string) (interface{}, error) {
	session, err := sm.getOrCreateSessionConcrete(sessionID)
	if err != nil {
		return nil, err
	}
	return session, nil
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
	if err := sm.store.Delete(sessionID); err != nil {
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
		if err := boltStore.CleanupExpired(sm.sessionTTL); err != nil {
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

	if session.DiskUsage+additionalBytes > session.MaxDiskUsage {
		return fmt.Errorf("session disk quota exceeded: %d + %d > %d",
			session.DiskUsage, additionalBytes, session.MaxDiskUsage)
	}

	// Check global quota
	totalUsage := sm.getTotalDiskUsage()
	if totalUsage+additionalBytes > sm.totalDiskLimit {
		return fmt.Errorf("global disk quota exceeded: %d + %d > %d",
			totalUsage, additionalBytes, sm.totalDiskLimit)
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
func (sm *SessionManager) GetStats() *SessionManagerStats {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	stats := &SessionManagerStats{
		TotalSessions:  len(sm.sessions),
		TotalDiskUsage: sm.getTotalDiskUsage(),
		MaxSessions:    sm.maxSessions,
		TotalDiskLimit: sm.totalDiskLimit,
	}

	for _, session := range sm.sessions {
		if session.GetActiveJobCount() > 0 {
			stats.SessionsWithJobs++
		}
		if session.IsExpired() {
			stats.ExpiredSessions++
		} else {
			stats.ActiveSessions++
		}
	}

	return stats
}

// SessionManagerStats provides statistics about the session manager
type SessionManagerStats struct {
	TotalSessions    int       `json:"total_sessions"`
	ActiveSessions   int       `json:"active_sessions"`
	ExpiredSessions  int       `json:"expired_sessions"`
	SessionsWithJobs int       `json:"sessions_with_jobs"`
	TotalDiskUsage   int64     `json:"total_disk_usage_bytes"`
	MaxSessions      int       `json:"max_sessions"`
	TotalDiskLimit   int64     `json:"total_disk_limit_bytes"`
	ServerStartTime  time.Time `json:"server_start_time"`
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
	sessionIDs, err := sm.store.List()
	if err != nil {
		return err
	}

	for _, sessionID := range sessionIDs {
		session, err := sm.store.Load(sessionID)
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
	return sm.store.Delete(sessionID)
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

// SessionFromContext extracts session ID from context
func SessionFromContext(ctx context.Context) string {
	if sessionID, ok := ctx.Value("session_id").(string); ok {
		return sessionID
	}
	return ""
}
