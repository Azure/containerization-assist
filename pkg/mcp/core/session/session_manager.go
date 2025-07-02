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
	workspaceDir string
	maxSessions  int
	sessionTTL   time.Duration
	store        SessionStore
	logger       zerolog.Logger
	mu           sync.RWMutex
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
		return nil, fmt.Errorf("failed to create workspace directory: %w", err)
	}

	var store SessionStore
	if config.StorePath != "" {
		var err error
		store, err = NewBoltSessionStore(context.Background(), config.StorePath)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize session store: %w", err)
		}
	} else {
		store = NewMemorySessionStore()
	}

	sm := &SessionManager{
		sessions:     make(map[string]*SessionState),
		workspaceDir: config.WorkspaceDir,
		maxSessions:  config.MaxSessions,
		sessionTTL:   config.SessionTTL,
		store:        store,
		logger:       config.Logger,
	}

	if err := sm.loadExistingSessions(); err != nil {
		sm.logger.Warn().Err(err).Msg("Failed to load existing sessions")
	}

	config.Logger.Info().Int("loaded_count", len(sm.sessions)).Msg("Loaded existing sessions")
	return sm, nil
}

// getOrCreateSessionConcrete retrieves or creates session
func (sm *SessionManager) getOrCreateSessionConcrete(sessionID string) (*SessionState, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

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
		if err := sm.cleanupOldestSession(); err != nil {
			return nil, fmt.Errorf("session limit exceeded: %w", err)
		}
	}

	workspaceDir := filepath.Join(sm.workspaceDir, sessionID)
	if err := os.MkdirAll(workspaceDir, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create session workspace: %w", err)
	}

	session := NewSessionStateWithTTL(sessionID, workspaceDir, sm.sessionTTL)

	sm.sessions[sessionID] = session

	if err := sm.store.Save(context.Background(), sessionID, session); err != nil {
		sm.logger.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to persist new session")
	}

	sm.logger.Info().Str("session_id", sessionID).Msg("Created new session")
	return session, nil
}

// UpdateSession updates and persists session
func (sm *SessionManager) UpdateSession(sessionID string, updater func(interface{})) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	updater(session)
	session.UpdateLastAccessed()

	if err := sm.store.Save(context.Background(), sessionID, session); err != nil {
		sm.logger.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to persist session update")
		return fmt.Errorf("failed to save session: %w", err)
	}

	return nil
}

// GetSessionConcrete retrieves session by ID
func (sm *SessionManager) GetSessionConcrete(sessionID string) (*SessionState, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if session, exists := sm.sessions[sessionID]; exists {
		return session, nil
	}

	return nil, fmt.Errorf("session not found: %s", sessionID)
}

// GetSessionInterface implements ToolSessionManager interface
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
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	summaries := make([]SessionSummary, 0, len(sm.sessions))
	for _, session := range sm.sessions {
		summaries = append(summaries, session.GetSummary())
	}

	return summaries
}

// DeleteSession removes a session and cleans up its workspace
func (sm *SessionManager) DeleteSession(ctx context.Context, sessionID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

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
		return fmt.Errorf("failed to delete session from store: %w", err)
	}

	sm.logger.Info().Str("session_id", sessionID).Msg("Deleted session")
	return nil
}

// GarbageCollect removes expired sessions and cleans up resources
func (sm *SessionManager) GarbageCollect() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
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
		if err := boltStore.CleanupExpired(context.Background(), time.Hour*24); err != nil {
			sm.logger.Warn().Err(err).Msg("Failed to clean up expired sessions from persistence")
		}
	}

	sm.logger.Info().Int("cleaned_count", len(expiredSessions)).Msg("Garbage collection completed")
	return nil
}

// Stop gracefully stops the session manager
func (sm *SessionManager) Stop() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Close persistence store
	if err := sm.store.Close(); err != nil {
		return fmt.Errorf("failed to close session store: %w", err)
	}

	sm.logger.Info().Msg("Session manager stopped")
	return nil
}

// GetStats returns basic statistics about the session manager
func (sm *SessionManager) GetStats() *core.SessionManagerStats {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	totalSessions := len(sm.sessions)
	activeSessions := 0
	for _, session := range sm.sessions {
		if !session.IsExpired() {
			activeSessions++
		}
	}

	return &core.SessionManagerStats{
		ActiveSessions: activeSessions,
		TotalSessions:  totalSessions,
	}
}

// convertToSessionData converts SessionState to SessionData
func (sm *SessionManager) convertToSessionData(session *SessionState) *SessionData {
	// Convert metadata from interface{} to string
	metadata := make(map[string]string)
	for key, value := range session.Metadata {
		if strValue, ok := value.(string); ok {
			metadata[key] = strValue
		} else {
			metadata[key] = fmt.Sprintf("%v", value)
		}
	}

	return &SessionData{
		ID:            session.SessionID,
		State:         session,
		CreatedAt:     session.CreatedAt,
		UpdatedAt:     session.LastAccessed,
		ExpiresAt:     session.ExpiresAt,
		WorkspacePath: session.WorkspaceDir,
		Labels:        session.Labels,
		RepoURL:       session.RepoURL,
		Metadata:      metadata,
	}
}

// GetAllSessions implements ListSessionsManager interface
func (sm *SessionManager) GetAllSessions() ([]*SessionData, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var sessions []*SessionData
	for _, session := range sm.sessions {
		sessions = append(sessions, sm.convertToSessionData(session))
	}
	return sessions, nil
}

// GetSessionData implements ListSessionsManager interface
func (sm *SessionManager) GetSessionData(sessionID string) (*SessionData, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	return sm.convertToSessionData(session), nil
}

func (sm *SessionManager) loadExistingSessions() error {
	sessionIDs, err := sm.store.List(context.Background())
	if err != nil {
		return err
	}

	for _, sessionID := range sessionIDs {
		if session, err := sm.store.Load(context.Background(), sessionID); err == nil && !session.IsExpired() {
			sm.sessions[sessionID] = session
		}
	}

	sm.logger.Info().Int("loaded_count", len(sm.sessions)).Msg("Loaded existing sessions")
	return nil
}

func (sm *SessionManager) deleteSessionUnsafe(sessionID string) error {
	return sm.deleteSessionInternal(sessionID)
}

func (sm *SessionManager) cleanupOrphanedWorkspaces() error {
	workspaces, err := os.ReadDir(sm.workspaceDir)
	if err != nil {
		return err
	}

	for _, workspace := range workspaces {
		if workspace.IsDir() {
			sessionID := workspace.Name()
			if _, exists := sm.sessions[sessionID]; !exists {
				os.RemoveAll(filepath.Join(sm.workspaceDir, sessionID))
			}
		}
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
func (sm *SessionManager) cleanupOldestSession() error {
	if len(sm.sessions) == 0 {
		return nil
	}

	var oldestSessionID string
	var oldestTime time.Time = time.Now()

	for sessionID, session := range sm.sessions {
		if session.LastAccessed.Before(oldestTime) {
			oldestTime = session.LastAccessed
			oldestSessionID = sessionID
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
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Remove from memory
	delete(sm.sessions, sessionID)

	// Clean up workspace directory
	if err := os.RemoveAll(session.WorkspaceDir); err != nil {
		sm.logger.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to clean up session workspace")
	}

	// Remove from persistent store
	if err := sm.store.Delete(context.Background(), sessionID); err != nil {
		sm.logger.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to remove session from persistent store")
	}

	sm.logger.Info().Str("session_id", sessionID).Msg("Session cleaned up successfully")
	return nil
}

// TrackError tracks an error for a session (simplified)
func (sm *SessionManager) TrackError(sessionID string, err error, context map[string]interface{}) error {
	sm.logger.Warn().Err(err).Str("session_id", sessionID).Msg("Session error tracked")
	return nil
}

// SessionFromContext extracts session ID from context
func SessionFromContext(ctx context.Context) string {
	if sessionID, ok := ctx.Value("session_id").(string); ok {
		return sessionID
	}
	return ""
}

// StartCleanupRoutine starts periodic session cleanup (simplified)
func (sm *SessionManager) StartCleanupRoutine() {
	// Simplified: cleanup on demand only
}

// AddSessionLabel adds a label to a session (simplified)
func (sm *SessionManager) AddSessionLabel(sessionID, label string) error {
	return sm.UpdateSession(sessionID, func(s interface{}) {
		if session, ok := s.(*SessionState); ok {
			session.AddLabel(label)
		}
	})
}

// RemoveSessionLabel removes a label from a session (simplified)
func (sm *SessionManager) RemoveSessionLabel(sessionID, label string) error {
	return sm.UpdateSession(sessionID, func(s interface{}) {
		if session, ok := s.(*SessionState); ok {
			session.RemoveLabel(label)
		}
	})
}

// SetSessionLabels replaces all labels for a session (simplified)
func (sm *SessionManager) SetSessionLabels(sessionID string, labels []string) error {
	return sm.UpdateSession(sessionID, func(s interface{}) {
		if session, ok := s.(*SessionState); ok {
			session.SetLabels(labels)
		}
	})
}

// GetAllLabels returns all unique labels across all sessions (simplified)
func (sm *SessionManager) GetAllLabels() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

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

// ListSessions returns all sessions (simplified implementation)
func (sm *SessionManager) ListSessions(ctx context.Context, filter map[string]interface{}) ([]interface{}, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var results []interface{}
	for _, session := range sm.sessions {
		results = append(results, session)
	}
	return results, nil
}

// Job and tool execution tracking (simplified but functional)
func (sm *SessionManager) StartJob(sessionID, jobType string) (string, error) {
	jobID := "job-" + generateSessionID()
	err := sm.UpdateSession(sessionID, func(s interface{}) {
		if session, ok := s.(*SessionState); ok {
			if session.ActiveJobs == nil {
				session.ActiveJobs = make(map[string]JobInfo)
			}
			session.ActiveJobs[jobID] = JobInfo{
				JobID:     jobID,
				Tool:      jobType,
				Status:    "running",
				StartTime: time.Now(),
			}
		}
	})
	return jobID, err
}

func (sm *SessionManager) UpdateJobStatus(sessionID, jobID string, status interface{}, result interface{}, err error) error {
	return sm.UpdateSession(sessionID, func(s interface{}) {
		if session, ok := s.(*SessionState); ok {
			if job, exists := session.ActiveJobs[jobID]; exists {
				if statusStr, ok := status.(string); ok {
					job.Status = JobStatus(statusStr)
				}
				job.Result = result
				session.ActiveJobs[jobID] = job
			}
		}
	})
}

func (sm *SessionManager) CompleteJob(sessionID, jobID string, result interface{}) error {
	return sm.UpdateSession(sessionID, func(s interface{}) {
		if session, ok := s.(*SessionState); ok {
			if job, exists := session.ActiveJobs[jobID]; exists {
				now := time.Now()
				job.EndTime = &now
				duration := now.Sub(job.StartTime)
				job.Duration = &duration
				job.Status = "completed"
				job.Result = result
				session.ActiveJobs[jobID] = job
			}
		}
	})
}

func (sm *SessionManager) TrackToolExecution(sessionID, toolName string, args interface{}) error {
	return sm.UpdateSession(sessionID, func(s interface{}) {
		if session, ok := s.(*SessionState); ok {
			execution := ToolExecution{
				Tool:      toolName,
				StartTime: time.Now(),
				Success:   false, // Will be updated on completion
			}
			session.StageHistory = append(session.StageHistory, execution)
		}
	})
}

func (sm *SessionManager) CompleteToolExecution(sessionID, toolName string, success bool, err error, tokensUsed int) error {
	return sm.UpdateSession(sessionID, func(s interface{}) {
		if session, ok := s.(*SessionState); ok && len(session.StageHistory) > 0 {
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
	})
}
