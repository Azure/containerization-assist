// Package session provides session management adapters
package session

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	domainsession "github.com/Azure/containerization-assist/pkg/domain/session"
	"github.com/Azure/containerization-assist/pkg/infrastructure/persistence/session"
)

// BoltStoreAdapter adapts the BoltStore to implement OptimizedSessionManager
type BoltStoreAdapter struct {
	store       *session.BoltStore
	logger      *slog.Logger
	defaultTTL  time.Duration
	maxSessions int
}

// NewBoltStoreAdapter creates a new adapter for BoltStore
func NewBoltStoreAdapter(dbPath string, logger *slog.Logger, defaultTTL time.Duration, maxSessions int) (*BoltStoreAdapter, error) {
	store, err := session.NewBoltStore(dbPath, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create bolt store: %w", err)
	}

	return &BoltStoreAdapter{
		store:       store,
		defaultTTL:  defaultTTL,
		maxSessions: maxSessions,
	}, nil
}

// Get retrieves a session by ID
func (a *BoltStoreAdapter) Get(ctx context.Context, sessionID string) (*SessionState, error) {
	sess, err := a.store.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	return &SessionState{
		SessionID: sess.ID,
		CreatedAt: sess.CreatedAt,
		UpdatedAt: sess.UpdatedAt,
		ExpiresAt: sess.ExpiresAt,
		Status:    string(sess.Status),
		Stage:     sess.Stage,
		UserID:    sess.UserID,
		Labels:    sess.Labels,
		Metadata:  sess.Metadata,
	}, nil
}

// GetOrCreate gets an existing session or creates a new one
func (a *BoltStoreAdapter) GetOrCreate(ctx context.Context, sessionID string) (*SessionState, error) {
	// Try to get existing session
	existing, err := a.Get(ctx, sessionID)
	if err == nil {
		return existing, nil
	}

	// Create new session
	now := time.Now()
	sess := domainsession.Session{
		ID:        sessionID,
		UserID:    "", // Would need to be provided
		CreatedAt: now,
		UpdatedAt: now,
		ExpiresAt: now.Add(a.defaultTTL),
		Status:    domainsession.StatusActive,
		Stage:     "",
		Labels:    make(map[string]string),
		Metadata:  make(map[string]interface{}),
	}

	if err := a.store.Create(ctx, sess); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return &SessionState{
		SessionID: sessionID,
		CreatedAt: now,
		UpdatedAt: now,
		ExpiresAt: now.Add(a.defaultTTL),
		Status:    string(domainsession.StatusActive),
		Stage:     "",
		UserID:    "",
		Labels:    make(map[string]string),
		Metadata:  make(map[string]interface{}),
	}, nil
}

// Update modifies a session using an update function
func (a *BoltStoreAdapter) Update(ctx context.Context, sessionID string, updateFunc func(*SessionState) error) error {
	// Get current session
	state, err := a.Get(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session for update: %w", err)
	}

	// Apply update function
	if err := updateFunc(state); err != nil {
		return fmt.Errorf("update function failed: %w", err)
	}

	// Convert back to domain session
	sess := domainsession.Session{
		ID:        state.SessionID,
		UserID:    state.UserID,
		CreatedAt: state.CreatedAt,
		UpdatedAt: time.Now(),
		ExpiresAt: state.ExpiresAt,
		Status:    domainsession.Status(state.Status),
		Stage:     state.Stage,
		Labels:    state.Labels,
		Metadata:  state.Metadata,
	}

	// Update in store
	return a.store.Update(ctx, sess)
}

// List returns all active sessions
func (a *BoltStoreAdapter) List(ctx context.Context) ([]*SessionState, error) {
	sessions, err := a.store.List(ctx)
	if err != nil {
		return nil, err
	}

	states := make([]*SessionState, len(sessions))
	for i, sess := range sessions {
		states[i] = &SessionState{
			SessionID: sess.ID,
			CreatedAt: sess.CreatedAt,
			UpdatedAt: sess.UpdatedAt,
			ExpiresAt: sess.ExpiresAt,
			Status:    string(sess.Status),
			Stage:     sess.Stage,
			UserID:    sess.UserID,
			Labels:    sess.Labels,
			Metadata:  sess.Metadata,
		}
	}

	return states, nil
}

// Stats returns session statistics
func (a *BoltStoreAdapter) Stats() *SessionStats {
	stats, err := a.store.Stats(context.Background())
	if err != nil {
		return &SessionStats{
			MaxSessions: a.maxSessions,
		}
	}

	return &SessionStats{
		ActiveSessions:   stats.ActiveSessions,
		TotalSessions:    stats.TotalSessions,
		MaxSessions:      a.maxSessions,
		TotalCreated:     stats.TotalSessions, // BoltStore doesn't track total created separately
		TotalExpired:     0,                   // BoltStore doesn't track expired
		AverageSessionMS: 0,                   // Would need to calculate from sessions
		MemoryUsage:      0,                   // BoltStore doesn't provide memory usage
	}
}

// Stop shuts down the session manager
func (a *BoltStoreAdapter) Stop(ctx context.Context) error {
	return a.store.Close()
}

// Ensure BoltStoreAdapter implements OptimizedSessionManager
var _ OptimizedSessionManager = (*BoltStoreAdapter)(nil)
