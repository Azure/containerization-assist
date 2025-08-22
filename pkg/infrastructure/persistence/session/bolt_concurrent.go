// Package session provides concurrent-safe session storage improvements
package session

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/Azure/containerization-assist/pkg/domain/errors"
	"github.com/Azure/containerization-assist/pkg/domain/session"
	"go.etcd.io/bbolt"
)

// ConcurrentBoltStore wraps BoltStore with additional concurrency controls
type ConcurrentBoltStore struct {
	*BoltStore
	sessionLocks sync.Map // map[sessionID]*sync.RWMutex
}

// NewConcurrentBoltStore creates a new concurrent-safe BoltDB store
func NewConcurrentBoltStore(store *BoltStore) *ConcurrentBoltStore {
	return &ConcurrentBoltStore{
		BoltStore: store,
	}
}

// getSessionLock returns a lock for a specific session ID
func (s *ConcurrentBoltStore) getSessionLock(sessionID string) *sync.RWMutex {
	lock, _ := s.sessionLocks.LoadOrStore(sessionID, &sync.RWMutex{})
	return lock.(*sync.RWMutex)
}

// UpdateAtomic performs an atomic read-modify-write operation on a session
func (s *ConcurrentBoltStore) UpdateAtomic(ctx context.Context, sessionID string, updateFunc func(*session.Session) error) error {
	lock := s.getSessionLock(sessionID)
	lock.Lock()
	defer lock.Unlock()

	// Perform the entire read-modify-write in a single transaction
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(sessionsBucket))

		// Get current session data
		data := bucket.Get([]byte(sessionID))
		if data == nil {
			return errors.New(errors.CodeNotFound, "persistence", fmt.Sprintf("session %s not found", sessionID), nil)
		}

		// Unmarshal current session
		var sess session.Session
		if err := json.Unmarshal(data, &sess); err != nil {
			return errors.New(errors.CodeInternalError, "persistence", "failed to unmarshal session", err)
		}

		// Apply update function
		if err := updateFunc(&sess); err != nil {
			return err
		}

		// Update timestamp
		sess.UpdatedAt = time.Now()

		// Marshal and save updated session
		updatedData, err := json.Marshal(sess)
		if err != nil {
			return errors.New(errors.CodeInternalError, "persistence", "failed to marshal updated session", err)
		}

		return bucket.Put([]byte(sessionID), updatedData)
	})
}

// CompareAndSwap performs an optimistic locking update using version checking
func (s *ConcurrentBoltStore) CompareAndSwap(ctx context.Context, sessionID string, expectedVersion int64, updateFunc func(*session.Session) error) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(sessionsBucket))

		// Get current session data
		data := bucket.Get([]byte(sessionID))
		if data == nil {
			return errors.New(errors.CodeNotFound, "persistence", fmt.Sprintf("session %s not found", sessionID), nil)
		}

		// Unmarshal current session
		var sess session.Session
		if err := json.Unmarshal(data, &sess); err != nil {
			return errors.New(errors.CodeInternalError, "persistence", "failed to unmarshal session", err)
		}

		// Check version (using UpdatedAt timestamp as version)
		currentVersion := sess.UpdatedAt.UnixNano()
		if currentVersion != expectedVersion {
			return errors.New(errors.CodeVersionMismatch, "persistence",
				fmt.Sprintf("session %s was modified by another process (expected version: %d, current: %d)",
					sessionID, expectedVersion, currentVersion), nil)
		}

		// Apply update function
		if err := updateFunc(&sess); err != nil {
			return err
		}

		// Update timestamp (new version)
		sess.UpdatedAt = time.Now()

		// Marshal and save updated session
		updatedData, err := json.Marshal(sess)
		if err != nil {
			return errors.New(errors.CodeInternalError, "persistence", "failed to marshal updated session", err)
		}

		return bucket.Put([]byte(sessionID), updatedData)
	})
}

// GetWithLock retrieves a session with a read lock
func (s *ConcurrentBoltStore) GetWithLock(ctx context.Context, sessionID string) (session.Session, func(), error) {
	lock := s.getSessionLock(sessionID)
	lock.RLock()

	sess, err := s.Get(ctx, sessionID)
	if err != nil {
		lock.RUnlock()
		return session.Session{}, nil, err
	}

	// Return unlock function for caller to defer
	return sess, lock.RUnlock, nil
}

// BatchUpdate performs multiple session updates in a single transaction
func (s *ConcurrentBoltStore) BatchUpdate(ctx context.Context, updates map[string]func(*session.Session) error) error {
	// Acquire all locks in sorted order to prevent deadlocks
	var sessionIDs []string
	for id := range updates {
		sessionIDs = append(sessionIDs, id)
	}

	// Sort IDs to ensure consistent lock ordering
	for i := 0; i < len(sessionIDs); i++ {
		for j := i + 1; j < len(sessionIDs); j++ {
			if sessionIDs[i] > sessionIDs[j] {
				sessionIDs[i], sessionIDs[j] = sessionIDs[j], sessionIDs[i]
			}
		}
	}

	// Acquire locks in order
	locks := make([]*sync.RWMutex, len(sessionIDs))
	for i, id := range sessionIDs {
		lock := s.getSessionLock(id)
		lock.Lock()
		locks[i] = lock
		defer lock.Unlock()
	}

	// Perform all updates in a single transaction
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(sessionsBucket))

		for sessionID, updateFunc := range updates {
			// Get current session data
			data := bucket.Get([]byte(sessionID))
			if data == nil {
				continue // Skip missing sessions
			}

			// Unmarshal, update, and save
			var sess session.Session
			if err := json.Unmarshal(data, &sess); err != nil {
				return err
			}

			if err := updateFunc(&sess); err != nil {
				return err
			}

			sess.UpdatedAt = time.Now()

			updatedData, err := json.Marshal(sess)
			if err != nil {
				return err
			}

			if err := bucket.Put([]byte(sessionID), updatedData); err != nil {
				return err
			}
		}

		return nil
	})
}
