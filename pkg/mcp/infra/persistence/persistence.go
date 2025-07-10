package persistence

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
	"github.com/Azure/container-kit/pkg/mcp/infra/retry"
	bolt "go.etcd.io/bbolt"
)

// BoltSessionStore implements SessionStore using BoltDB
type BoltSessionStore struct {
	db     *bolt.DB
	logger *slog.Logger
}

const (
	sessionsBucket = "sessions"
)

// NewBoltSessionStore creates a new BoltDB-based session store
func NewBoltSessionStore(ctx context.Context, dbPath string, logger *slog.Logger) (*BoltSessionStore, error) {
	retryCoordinator := retry.New()

	var db *bolt.DB
	err := retryCoordinator.Execute(ctx, "database_open", func(ctx context.Context) error {
		var openErr error
		db, openErr = bolt.Open(dbPath, 0o600, &bolt.Options{
			Timeout:        5 * time.Second,
			NoGrowSync:     false,
			NoFreelistSync: false,
			FreelistType:   bolt.FreelistArrayType,
		})

		if openErr == bolt.ErrTimeout {
			backupPath := fmt.Sprintf("%s.locked.%d", dbPath, time.Now().Unix())
			if renameErr := os.Rename(dbPath, backupPath); renameErr == nil {
				logger.Warn("Moved locked database file", "backup_path", backupPath)
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
		return nil, errors.Wrapf(err, "persistence", "Failed to open BoltDB database at %s after 3 attempts", dbPath)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(sessionsBucket))
		return err
	})
	if err != nil {
		if closeErr := db.Close(); closeErr != nil {
			logger.Warn("Failed to close database after bucket creation error", "error", closeErr)
		}
		return nil, errors.Wrapf(err, "persistence", "Failed to create sessions bucket in database")
	}

	return &BoltSessionStore{db: db, logger: logger}, nil
}

// Save persists a session state to the database
func (s *BoltSessionStore) Save(ctx context.Context, sessionID string, state *session.SessionState) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	data, err := json.Marshal(state)
	if err != nil {
		return errors.NewError().Message("failed to marshal session state").Cause(err).WithLocation().Build()
	}

	type result struct {
		err error
	}
	resultCh := make(chan result, 1)

	go func() {
		defer close(resultCh)
		err := s.db.Update(func(tx *bolt.Tx) error {
			bucket := tx.Bucket([]byte(sessionsBucket))
			return bucket.Put([]byte(sessionID), data)
		})
		select {
		case resultCh <- result{err: err}:
		case <-ctx.Done():
		}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case res := <-resultCh:
		return res.err
	}
}

// Load retrieves a session state from the database
func (s *BoltSessionStore) Load(ctx context.Context, sessionID string) (*session.SessionState, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	type result struct {
		state *session.SessionState
		err   error
	}
	resultCh := make(chan result, 1)

	go func() {
		defer close(resultCh)
		var localState *session.SessionState
		err := s.db.View(func(tx *bolt.Tx) error {
			bucket := tx.Bucket([]byte(sessionsBucket))
			data := bucket.Get([]byte(sessionID))
			if data == nil {
				return errors.NewError().Messagef("session not found: %s", sessionID).Build()
			}

			localState = &session.SessionState{}
			return json.Unmarshal(data, localState)
		})
		select {
		case resultCh <- result{state: localState, err: err}:
		case <-ctx.Done():
		}
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
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	type result struct {
		err error
	}
	resultCh := make(chan result, 1)

	go func() {
		defer close(resultCh)
		err := s.db.Update(func(tx *bolt.Tx) error {
			bucket := tx.Bucket([]byte(sessionsBucket))
			return bucket.Delete([]byte(sessionID))
		})
		select {
		case resultCh <- result{err: err}:
		case <-ctx.Done():
		}
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
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	type result struct {
		sessionIDs []string
		err        error
	}
	resultCh := make(chan result, 1)

	go func() {
		defer close(resultCh)
		var sessionIDs []string
		err := s.db.View(func(tx *bolt.Tx) error {
			bucket := tx.Bucket([]byte(sessionsBucket))
			return bucket.ForEach(func(k, v []byte) error {
				sessionIDs = append(sessionIDs, string(k))
				return nil
			})
		})
		select {
		case resultCh <- result{sessionIDs: sessionIDs, err: err}:
		case <-ctx.Done():
		}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-resultCh:
		return res.sessionIDs, res.err
	}
}

// Close closes the database connection
func (s *BoltSessionStore) Close(ctx context.Context) error {
	// BoltDB doesn't support context-aware close, but we can check context first
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.db.Close()
}

// CleanupExpired removes expired sessions from the database
func (s *BoltSessionStore) CleanupExpired(ctx context.Context, ttl time.Duration) error {
	expiredSessions := make([]string, 0)

	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(sessionsBucket))
		return bucket.ForEach(func(k, v []byte) error {
			var state session.SessionState
			if err := json.Unmarshal(v, &state); err != nil {
				return err
			}

			// TODO: Implement IsExpired method on SessionState
			// if state.IsExpired() {
			//     expiredSessions = append(expiredSessions, string(k))
			// }

			return nil
		})
	})
	if err != nil {
		return errors.NewError().Message("failed to identify expired sessions").Cause(err).WithLocation().Build()
	}

	for _, sessionID := range expiredSessions {
		if err := s.Delete(ctx, sessionID); err != nil {
			return errors.NewError().Message(fmt.Sprintf("failed to delete expired session %s", sessionID)).Cause(err).Build()
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

			var state session.SessionState
			if err := json.Unmarshal(v, &state); err != nil {
				return err
			}

			// TODO: Implement DiskUsage field on SessionState
			// stats.TotalDiskUsage += state.DiskUsage

			// TODO: Implement IsExpired method on SessionState
			// if state.IsExpired() {
			//     stats.ExpiredSessions++
			// } else {
			//     stats.ActiveSessions++
			// }

			// TODO: Implement GetActiveJobCount method on SessionState
			// if state.GetActiveJobCount() > 0 {
			//     stats.SessionsWithJobs++
			// }

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
	sessions map[string]*session.SessionState
}

// NewMemorySessionStore creates a new in-memory session store
func NewMemorySessionStore() *MemorySessionStore {
	return &MemorySessionStore{
		sessions: make(map[string]*session.SessionState),
	}
}

// Save stores a session in memory
func (s *MemorySessionStore) Save(ctx context.Context, sessionID string, state *session.SessionState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}

	var copy session.SessionState
	if err := json.Unmarshal(data, &copy); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sessionID] = &copy
	return nil
}

// Load retrieves a session from memory
func (s *MemorySessionStore) Load(ctx context.Context, sessionID string) (*session.SessionState, error) {
	s.mu.RLock()
	state, exists := s.sessions[sessionID]
	s.mu.RUnlock()

	if !exists {
		return nil, errors.NewError().Messagef("session not found: %s", sessionID).WithLocation().Build()
	}

	data, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}

	var copy session.SessionState
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
func (s *MemorySessionStore) Close(ctx context.Context) error {
	// Check context for consistency
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}
