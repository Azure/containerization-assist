package session

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/retry"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/rs/zerolog/log"
	bolt "go.etcd.io/bbolt"
)

// SessionStore defines the interface for session persistence
type SessionStore interface {
	Save(ctx context.Context, sessionID string, state *SessionState) error
	Load(ctx context.Context, sessionID string) (*SessionState, error)
	Delete(ctx context.Context, sessionID string) error
	List(ctx context.Context) ([]string, error)
	Close() error
}

// BoltSessionStore implements SessionStore using BoltDB
type BoltSessionStore struct {
	db *bolt.DB
}

const (
	sessionsBucket = "sessions"
)

// NewBoltSessionStore creates a new BoltDB-based session store
func NewBoltSessionStore(ctx context.Context, dbPath string) (*BoltSessionStore, error) {
	// Use unified retry coordinator for database connection
	retryCoordinator := retry.NewCoordinator(log.Logger)

	var db *bolt.DB
	err := retryCoordinator.Execute(ctx, "database_open", func(ctx context.Context) error {
		var openErr error
		db, openErr = bolt.Open(dbPath, 0o600, &bolt.Options{
			Timeout:        5 * time.Second,
			NoGrowSync:     false,
			NoFreelistSync: false,
			FreelistType:   bolt.FreelistArrayType,
		})

		// Handle special case for locked database
		if openErr == bolt.ErrTimeout {
			backupPath := fmt.Sprintf("%s.locked.%d", dbPath, time.Now().Unix())
			if renameErr := os.Rename(dbPath, backupPath); renameErr == nil {
				log.Warn().Str("backup_path", backupPath).Msg("Moved locked database file")
				// Try again with the moved file
				db, openErr = bolt.Open(dbPath, 0o600, &bolt.Options{
					Timeout:        5 * time.Second,
					NoGrowSync:     false,
					NoFreelistSync: false,
					FreelistType:   bolt.FreelistArrayType,
				})
			}
		}

		return openErr
	})

	if err != nil {
		return nil, types.NewSessionError("", "open_database").
			WithField("database_path", dbPath).
			WithField("attempts", 3).
			WithRootCause(fmt.Sprintf("BoltDB open failed after 3 attempts: %v", err)).
			WithImmediateStep(1, "Check lock file", "Verify no other container-kit instance is running").
			WithImmediateStep(2, "Check permissions", "Ensure write permissions to database directory").
			WithImmediateStep(3, "Check disk space", "Verify sufficient disk space is available").
			Build()
	}

	// Create the sessions bucket if it doesn't exist
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(sessionsBucket))
		return err
	})
	if err != nil {
		if closeErr := db.Close(); closeErr != nil {
			// Log the close error but return the original error
			log.Warn().Err(closeErr).Msg("Failed to close database after bucket creation error")
		}
		return nil, types.NewSessionError("", "create_bucket").
			WithRootCause(fmt.Sprintf("Sessions bucket creation failed: %v", err)).
			WithImmediateStep(1, "Check database integrity", "Verify database file is not corrupted").
			WithImmediateStep(2, "Restart with clean database", "Delete database file if corruption suspected").
			Build()
	}

	return &BoltSessionStore{db: db}, nil
}

// Save persists a session state to the database
func (s *BoltSessionStore) Save(ctx context.Context, sessionID string, state *SessionState) error {
	// Check context cancellation before starting
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal session state: %w", err)
	}

	// Use channel to make database operation cancellable
	type result struct {
		err error
	}
	resultCh := make(chan result, 1)

	go func() {
		err := s.db.Update(func(tx *bolt.Tx) error {
			bucket := tx.Bucket([]byte(sessionsBucket))
			return bucket.Put([]byte(sessionID), data)
		})
		resultCh <- result{err: err}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case res := <-resultCh:
		return res.err
	}
}

// Load retrieves a session state from the database
func (s *BoltSessionStore) Load(ctx context.Context, sessionID string) (*SessionState, error) {
	// Check context cancellation before starting
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	var state *SessionState

	// Use channel to make database operation cancellable
	type result struct {
		state *SessionState
		err   error
	}
	resultCh := make(chan result, 1)

	go func() {
		var localState *SessionState
		err := s.db.View(func(tx *bolt.Tx) error {
			bucket := tx.Bucket([]byte(sessionsBucket))
			data := bucket.Get([]byte(sessionID))
			if data == nil {
				return fmt.Errorf("session not found: %s", sessionID)
			}

			localState = &SessionState{}
			return json.Unmarshal(data, localState)
		})
		resultCh <- result{state: localState, err: err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-resultCh:
		return res.state, res.err
	}
}

// Delete removes a session from the database
func (s *BoltSessionStore) Delete(ctx context.Context, sessionID string) error {
	// Check context cancellation before starting
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Use channel to make database operation cancellable
	type result struct {
		err error
	}
	resultCh := make(chan result, 1)

	go func() {
		err := s.db.Update(func(tx *bolt.Tx) error {
			bucket := tx.Bucket([]byte(sessionsBucket))
			return bucket.Delete([]byte(sessionID))
		})
		resultCh <- result{err: err}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case res := <-resultCh:
		return res.err
	}
}

// List returns all session IDs in the database
func (s *BoltSessionStore) List(ctx context.Context) ([]string, error) {
	// Check context cancellation before starting
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Use channel to make database operation cancellable
	type result struct {
		sessionIDs []string
		err        error
	}
	resultCh := make(chan result, 1)

	go func() {
		var sessionIDs []string
		err := s.db.View(func(tx *bolt.Tx) error {
			bucket := tx.Bucket([]byte(sessionsBucket))
			return bucket.ForEach(func(k, v []byte) error {
				sessionIDs = append(sessionIDs, string(k))
				return nil
			})
		})
		resultCh <- result{sessionIDs: sessionIDs, err: err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-resultCh:
		return res.sessionIDs, res.err
	}
}

// Close closes the database connection
func (s *BoltSessionStore) Close() error {
	return s.db.Close()
}

// CleanupExpired removes expired sessions from the database
func (s *BoltSessionStore) CleanupExpired(ctx context.Context, ttl time.Duration) error {
	expiredSessions := make([]string, 0)

	// First, identify expired sessions
	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(sessionsBucket))
		return bucket.ForEach(func(k, v []byte) error {
			var state SessionState
			if err := json.Unmarshal(v, &state); err != nil {
				return err
			}

			if state.IsExpired() {
				expiredSessions = append(expiredSessions, string(k))
			}

			return nil
		})
	})
	if err != nil {
		return fmt.Errorf("failed to identify expired sessions: %w", err)
	}

	// Then delete them
	for _, sessionID := range expiredSessions {
		if err := s.Delete(ctx, sessionID); err != nil {
			return fmt.Errorf("failed to delete expired session %s: %w", sessionID, err)
		}
	}

	return nil
}

// GetStats returns statistics about the session store
func (s *BoltSessionStore) GetStats(ctx context.Context) (*SessionStoreStats, error) {
	stats := &SessionStoreStats{}

	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(sessionsBucket))

		return bucket.ForEach(func(k, v []byte) error {
			stats.TotalSessions++

			var state SessionState
			if err := json.Unmarshal(v, &state); err != nil {
				return err
			}

			stats.TotalDiskUsage += state.DiskUsage

			if state.IsExpired() {
				stats.ExpiredSessions++
			} else {
				stats.ActiveSessions++
			}

			if state.GetActiveJobCount() > 0 {
				stats.SessionsWithJobs++
			}

			return nil
		})
	})

	return stats, err
}

// SessionStoreStats provides statistics about the session store
type SessionStoreStats struct {
	TotalSessions    int   `json:"total_sessions"`
	ActiveSessions   int   `json:"active_sessions"`
	ExpiredSessions  int   `json:"expired_sessions"`
	SessionsWithJobs int   `json:"sessions_with_jobs"`
	TotalDiskUsage   int64 `json:"total_disk_usage_bytes"`
}

// MemorySessionStore implements SessionStore using in-memory storage (for testing)
type MemorySessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*SessionState
}

// NewMemorySessionStore creates a new in-memory session store
func NewMemorySessionStore() *MemorySessionStore {
	return &MemorySessionStore{
		sessions: make(map[string]*SessionState),
	}
}

// Save stores a session in memory
func (s *MemorySessionStore) Save(ctx context.Context, sessionID string, state *SessionState) error {
	// Deep copy to prevent external modifications
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}

	var copy SessionState
	if err := json.Unmarshal(data, &copy); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sessionID] = &copy
	return nil
}

// Load retrieves a session from memory
func (s *MemorySessionStore) Load(ctx context.Context, sessionID string) (*SessionState, error) {
	s.mu.RLock()
	state, exists := s.sessions[sessionID]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Deep copy to prevent external modifications
	data, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}

	var copy SessionState
	if err := json.Unmarshal(data, &copy); err != nil {
		return nil, err
	}

	return &copy, nil
}

// Delete removes a session from memory
func (s *MemorySessionStore) Delete(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
	return nil
}

// List returns all session IDs in memory
func (s *MemorySessionStore) List(ctx context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessionIDs := make([]string, 0, len(s.sessions))
	for id := range s.sessions {
		sessionIDs = append(sessionIDs, id)
	}
	return sessionIDs, nil
}

// Close is a no-op for memory store
func (s *MemorySessionStore) Close() error {
	return nil
}
