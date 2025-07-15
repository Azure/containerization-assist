// Package session provides session management services
package session

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	domainsession "github.com/Azure/container-kit/pkg/mcp/domain/session"
)

// Service implements session management using the domain store interface
type Service struct {
	store           domainsession.Store
	logger          *slog.Logger
	defaultTTL      time.Duration
	cleanupInterval time.Duration
	stopCleanup     chan struct{}
}

// NewService creates a new session service
func NewService(store domainsession.Store, logger *slog.Logger, defaultTTL time.Duration) *Service {
	// Default cleanup interval to 5 minutes
	cleanupInterval := 5 * time.Minute
	if defaultTTL < cleanupInterval && defaultTTL > 0 {
		// If TTL is shorter than 5 minutes, run cleanup more frequently
		cleanupInterval = defaultTTL / 2
	}

	s := &Service{
		store:           store,
		logger:          logger.With("component", "session_service"),
		defaultTTL:      defaultTTL,
		cleanupInterval: cleanupInterval,
		stopCleanup:     make(chan struct{}),
	}

	// Start background cleanup process
	go s.startBackgroundCleanup()

	return s
}

// Get retrieves a session by ID
func (s *Service) Get(ctx context.Context, sessionID string) (*SessionState, error) {
	sess, err := s.store.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	// Convert domain session to application SessionState
	return s.domainToApplicationSession(sess), nil
}

// GetOrCreate gets an existing session or creates a new one
func (s *Service) GetOrCreate(ctx context.Context, sessionID string) (*SessionState, error) {
	// Try to get existing session first
	sess, err := s.store.Get(ctx, sessionID)
	if err == nil {
		return s.domainToApplicationSession(sess), nil
	}

	// Check if error is specifically "not found" vs other errors
	if err != domainsession.ErrSessionNotFound {
		// This is a real error (e.g., I/O error), not just a missing session
		s.logger.Error("Failed to retrieve session",
			slog.String("session_id", sessionID),
			slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to retrieve session: %w", err)
	}

	// Create new session if not found
	newSession := domainsession.NewSession(sessionID, "", s.defaultTTL)

	err = s.store.Create(ctx, newSession)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	s.logger.Info("Created new session",
		slog.String("session_id", sessionID),
		slog.Duration("ttl", s.defaultTTL))
	return s.domainToApplicationSession(newSession), nil
}

// Update modifies a session using an update function
func (s *Service) Update(ctx context.Context, sessionID string, updateFunc func(*SessionState) error) error {
	// Get current session
	sess, err := s.store.Get(ctx, sessionID)
	if err != nil {
		return err
	}

	// Convert to application format
	appSession := s.domainToApplicationSession(sess)

	// Apply update function
	if err := updateFunc(appSession); err != nil {
		return err
	}

	// Convert back to domain format
	updatedSession := s.applicationToDomainSession(*appSession)

	// Store updated session
	return s.store.Update(ctx, updatedSession)
}

// List returns all active sessions
func (s *Service) List(ctx context.Context) ([]*SessionState, error) {
	// Get all active sessions
	sessions, err := s.store.List(ctx, domainsession.StatusFilter{Status: domainsession.StatusActive})
	if err != nil {
		return nil, err
	}

	// Convert to application format
	result := make([]*SessionState, 0, len(sessions))
	for _, sess := range sessions {
		result = append(result, s.domainToApplicationSession(sess))
	}

	return result, nil
}

// Stats returns session statistics
func (s *Service) Stats() *SessionStats {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stats, err := s.store.Stats(ctx)
	if err != nil {
		s.logger.Error("Failed to get session stats", "error", err)
		return &SessionStats{}
	}

	return &SessionStats{
		ActiveSessions: stats.ActiveSessions,
		TotalSessions:  stats.TotalSessions,
		MaxSessions:    stats.MaxSessions,
	}
}

// Stop shuts down the session service
func (s *Service) Stop(ctx context.Context) error {
	// Stop background cleanup
	close(s.stopCleanup)

	// Cleanup expired sessions before shutdown
	removed, err := s.store.Cleanup(ctx)
	if err != nil {
		s.logger.Error("Failed to cleanup sessions during shutdown", "error", err)
	} else if removed > 0 {
		s.logger.Info("Cleaned up sessions during shutdown", "count", removed)
	}

	s.logger.Info("Session service stopped")
	return nil
}

// startBackgroundCleanup runs periodic cleanup of expired sessions
func (s *Service) startBackgroundCleanup() {
	ticker := time.NewTicker(s.cleanupInterval)
	defer ticker.Stop()

	s.logger.Info("Started background session cleanup",
		slog.Duration("interval", s.cleanupInterval))

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			if err := s.Cleanup(ctx); err != nil {
				s.logger.Error("Background session cleanup failed",
					slog.String("error", err.Error()))
			}
			cancel()
		case <-s.stopCleanup:
			s.logger.Info("Stopping background session cleanup")
			return
		}
	}
}

// Cleanup removes expired sessions (can be called periodically)
func (s *Service) Cleanup(ctx context.Context) error {
	removed, err := s.store.Cleanup(ctx)
	if err != nil {
		return err
	}

	if removed > 0 {
		s.logger.Info("Session cleanup completed", "removed_count", removed)
	}

	return nil
}

// domainToApplicationSession converts domain session to application SessionState
func (s *Service) domainToApplicationSession(sess domainsession.Session) *SessionState {
	status := string(sess.Status)
	if sess.IsExpired() {
		status = string(domainsession.StatusExpired)
	}

	return &SessionState{
		SessionID: sess.ID,
		CreatedAt: sess.CreatedAt,
		UpdatedAt: sess.UpdatedAt,
		ExpiresAt: sess.ExpiresAt,
		Status:    status,
		Stage:     sess.Stage,
		UserID:    sess.UserID,
		Labels:    sess.Labels,
		Metadata:  sess.Metadata,
	}
}

// applicationToDomainSession converts application SessionState to domain session
func (s *Service) applicationToDomainSession(state SessionState) domainsession.Session {
	status := domainsession.StatusActive
	switch state.Status {
	case string(domainsession.StatusExpired):
		status = domainsession.StatusExpired
	case string(domainsession.StatusSuspended):
		status = domainsession.StatusSuspended
	}

	return domainsession.Session{
		ID:        state.SessionID,
		UserID:    state.UserID,
		CreatedAt: state.CreatedAt,
		UpdatedAt: state.UpdatedAt,
		ExpiresAt: state.ExpiresAt,
		Status:    status,
		Stage:     state.Stage,
		Labels:    state.Labels,
		Metadata:  state.Metadata,
	}
}

// Ensure Service implements OptimizedSessionManager
var _ OptimizedSessionManager = (*Service)(nil)
