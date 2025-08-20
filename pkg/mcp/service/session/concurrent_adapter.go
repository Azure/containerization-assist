// Package session provides concurrent-safe session management adapters
package session

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	domainsession "github.com/Azure/containerization-assist/pkg/mcp/domain/session"
)

// ConcurrentBoltAdapter wraps BoltStoreAdapter with workflow state locking
type ConcurrentBoltAdapter struct {
	*BoltStoreAdapter
	workflowLocks sync.Map // map[sessionID]*sync.Mutex for workflow state updates
}

// NewConcurrentBoltAdapter creates a new concurrent-safe adapter
func NewConcurrentBoltAdapter(dbPath string, logger *slog.Logger, defaultTTL time.Duration, maxSessions int) (*ConcurrentBoltAdapter, error) {
	baseAdapter, err := NewBoltStoreAdapter(dbPath, logger, defaultTTL, maxSessions)
	if err != nil {
		return nil, err
	}

	return &ConcurrentBoltAdapter{
		BoltStoreAdapter: baseAdapter,
	}, nil
}

// getWorkflowLock returns a mutex for workflow state updates on a specific session
func (a *ConcurrentBoltAdapter) getWorkflowLock(sessionID string) *sync.Mutex {
	lock, _ := a.workflowLocks.LoadOrStore(sessionID, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

// UpdateWorkflowState atomically updates workflow state in session metadata
func (a *ConcurrentBoltAdapter) UpdateWorkflowState(ctx context.Context, sessionID string, updateFunc func(metadata map[string]interface{}) error) error {
	// Get workflow lock for this session
	lock := a.getWorkflowLock(sessionID)
	lock.Lock()
	defer lock.Unlock()

	// Use the base Update method with our locked update function
	return a.Update(ctx, sessionID, func(state *SessionState) error {
		if state.Metadata == nil {
			state.Metadata = make(map[string]interface{})
		}

		// Let the caller update the metadata
		if err := updateFunc(state.Metadata); err != nil {
			return err
		}

		// Update timestamp
		state.UpdatedAt = time.Now()
		return nil
	})
}

// GetWorkflowState retrieves workflow state from session metadata with read consistency
func (a *ConcurrentBoltAdapter) GetWorkflowState(ctx context.Context, sessionID string) (map[string]interface{}, error) {
	state, err := a.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	if state.Metadata == nil {
		return make(map[string]interface{}), nil
	}

	// Return a copy to prevent external modifications
	metadata := make(map[string]interface{})
	for k, v := range state.Metadata {
		metadata[k] = v
	}

	return metadata, nil
}

// AcquireWorkflowLock acquires an exclusive lock for workflow operations
// Caller MUST call the returned unlock function when done
func (a *ConcurrentBoltAdapter) AcquireWorkflowLock(sessionID string) func() {
	lock := a.getWorkflowLock(sessionID)
	lock.Lock()
	return lock.Unlock
}

// UpdateWithVersion performs optimistic locking update using version checking
func (a *ConcurrentBoltAdapter) UpdateWithVersion(ctx context.Context, sessionID string, expectedVersion time.Time, updateFunc func(*SessionState) error) error {
	// Get current session
	state, err := a.Get(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session for versioned update: %w", err)
	}

	// Check version (using UpdatedAt as version)
	if !state.UpdatedAt.Equal(expectedVersion) {
		return fmt.Errorf("session %s was modified by another process (expected: %v, current: %v)",
			sessionID, expectedVersion, state.UpdatedAt)
	}

	// Apply update
	return a.Update(ctx, sessionID, updateFunc)
}

// BatchUpdateWorkflowStates updates multiple session workflow states atomically
func (a *ConcurrentBoltAdapter) BatchUpdateWorkflowStates(ctx context.Context, updates map[string]func(map[string]interface{}) error) error {
	// Sort session IDs to prevent deadlocks
	var sessionIDs []string
	for id := range updates {
		sessionIDs = append(sessionIDs, id)
	}

	// Simple sort
	for i := 0; i < len(sessionIDs); i++ {
		for j := i + 1; j < len(sessionIDs); j++ {
			if sessionIDs[i] > sessionIDs[j] {
				sessionIDs[i], sessionIDs[j] = sessionIDs[j], sessionIDs[i]
			}
		}
	}

	// Acquire locks in order
	unlocks := make([]func(), 0, len(sessionIDs))
	for _, id := range sessionIDs {
		unlock := a.AcquireWorkflowLock(id)
		unlocks = append(unlocks, unlock)
	}

	// Ensure all locks are released
	defer func() {
		for _, unlock := range unlocks {
			unlock()
		}
	}()

	// Perform updates
	for sessionID, updateFunc := range updates {
		state, err := a.Get(ctx, sessionID)
		if err != nil {
			a.logger.Warn("Skipping missing session in batch update", "session_id", sessionID, "error", err)
			continue
		}

		if state.Metadata == nil {
			state.Metadata = make(map[string]interface{})
		}

		if err := updateFunc(state.Metadata); err != nil {
			return fmt.Errorf("failed to update session %s: %w", sessionID, err)
		}

		// Convert back to domain session for update
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

		if err := a.store.Update(ctx, sess); err != nil {
			return fmt.Errorf("failed to persist session %s: %w", sessionID, err)
		}
	}

	return nil
}

// CleanupLocks removes locks for expired sessions to prevent memory leaks
func (a *ConcurrentBoltAdapter) CleanupLocks(ctx context.Context) {
	// List all sessions
	sessions, err := a.List(ctx)
	if err != nil {
		a.logger.Warn("Failed to list sessions for lock cleanup", "error", err)
		return
	}

	// Track active session IDs
	activeIDs := make(map[string]bool)
	for _, sess := range sessions {
		if sess.ExpiresAt.After(time.Now()) {
			activeIDs[sess.SessionID] = true
		}
	}

	// Remove locks for non-active sessions
	removed := 0
	a.workflowLocks.Range(func(key, value interface{}) bool {
		sessionID := key.(string)
		if !activeIDs[sessionID] {
			a.workflowLocks.Delete(sessionID)
			removed++
		}
		return true
	})

	if removed > 0 {
		a.logger.Info("Cleaned up workflow locks", "removed", removed)
	}
}

// StartCleanupRoutine starts a background routine to clean up expired locks
func (a *ConcurrentBoltAdapter) StartCleanupRoutine(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				a.CleanupLocks(ctx)
			}
		}
	}()
}

// Ensure ConcurrentBoltAdapter implements OptimizedSessionManager
var _ OptimizedSessionManager = (*ConcurrentBoltAdapter)(nil)
