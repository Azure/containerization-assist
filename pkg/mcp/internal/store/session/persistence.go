package session

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	sessiontypes "github.com/Azure/container-copilot/pkg/mcp/internal/types/session"
	"github.com/rs/zerolog/log"
	bolt "go.etcd.io/bbolt"
)

// SessionStore defines the interface for session persistence
type SessionStore interface {
	Save(sessionID string, state *sessiontypes.SessionState) error
	Load(sessionID string) (*sessiontypes.SessionState, error)
	Delete(sessionID string) error
	List() ([]string, error)
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
func NewBoltSessionStore(dbPath string) (*BoltSessionStore, error) {
	// Try to open with a longer timeout and retry logic
	var db *bolt.DB
	var err error

	for i := 0; i < 3; i++ {
		db, err = bolt.Open(dbPath, 0o600, &bolt.Options{
			Timeout:        5 * time.Second,
			NoGrowSync:     false,
			NoFreelistSync: false,
			FreelistType:   bolt.FreelistArrayType,
		})
		if err == nil {
			break
		}

		// If it's a timeout error and this is our last attempt,
		// try to move the database file and create a new one
		if i == 2 && err == bolt.ErrTimeout {
			backupPath := fmt.Sprintf("%s.locked.%d", dbPath, time.Now().Unix())
			if renameErr := os.Rename(dbPath, backupPath); renameErr == nil {
				// Try one more time with the moved file
				db, err = bolt.Open(dbPath, 0o600, &bolt.Options{
					Timeout:        5 * time.Second,
					NoGrowSync:     false,
					NoFreelistSync: false,
					FreelistType:   bolt.FreelistArrayType,
				})
				if err == nil {
					// Log that we had to move the old database
					log.Warn().Str("backup_path", backupPath).Msg("Moved locked database file")
					break
				}
			}
		}

		// If it's a timeout error, wait a bit and retry
		if i < 2 {
			time.Sleep(time.Duration(i+1) * time.Second)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to open BoltDB at %s after %d attempts: %w (hint: check if another instance is running)", dbPath, 3, err)
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
		return nil, fmt.Errorf("failed to create sessions bucket: %w", err)
	}

	return &BoltSessionStore{db: db}, nil
}

// Save persists a session state to the database
func (s *BoltSessionStore) Save(sessionID string, state *sessiontypes.SessionState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal session state: %w", err)
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(sessionsBucket))
		return bucket.Put([]byte(sessionID), data)
	})
}

// Load retrieves a session state from the database
func (s *BoltSessionStore) Load(sessionID string) (*sessiontypes.SessionState, error) {
	var state *sessiontypes.SessionState

	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(sessionsBucket))
		data := bucket.Get([]byte(sessionID))
		if data == nil {
			return fmt.Errorf("session not found: %s", sessionID)
		}

		state = &sessiontypes.SessionState{}
		return json.Unmarshal(data, state)
	})
	if err != nil {
		return nil, err
	}

	return state, nil
}

// Delete removes a session from the database
func (s *BoltSessionStore) Delete(sessionID string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(sessionsBucket))
		return bucket.Delete([]byte(sessionID))
	})
}

// List returns all session IDs in the database
func (s *BoltSessionStore) List() ([]string, error) {
	var sessionIDs []string

	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(sessionsBucket))
		return bucket.ForEach(func(k, v []byte) error {
			sessionIDs = append(sessionIDs, string(k))
			return nil
		})
	})

	return sessionIDs, err
}

// Close closes the database connection
func (s *BoltSessionStore) Close() error {
	return s.db.Close()
}

// CleanupExpired removes expired sessions from the database
func (s *BoltSessionStore) CleanupExpired(ttl time.Duration) error {
	expiredSessions := make([]string, 0)

	// First, identify expired sessions
	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(sessionsBucket))
		return bucket.ForEach(func(k, v []byte) error {
			var state sessiontypes.SessionState
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
		if err := s.Delete(sessionID); err != nil {
			return fmt.Errorf("failed to delete expired session %s: %w", sessionID, err)
		}
	}

	return nil
}

// GetStats returns statistics about the session store
func (s *BoltSessionStore) GetStats() (*SessionStoreStats, error) {
	stats := &SessionStoreStats{}

	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(sessionsBucket))

		return bucket.ForEach(func(k, v []byte) error {
			stats.TotalSessions++

			var state sessiontypes.SessionState
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
	sessions map[string]*sessiontypes.SessionState
}

// NewMemorySessionStore creates a new in-memory session store
func NewMemorySessionStore() *MemorySessionStore {
	return &MemorySessionStore{
		sessions: make(map[string]*sessiontypes.SessionState),
	}
}

// Save stores a session in memory
func (s *MemorySessionStore) Save(sessionID string, state *sessiontypes.SessionState) error {
	// Deep copy to prevent external modifications
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}

	var copy sessiontypes.SessionState
	if err := json.Unmarshal(data, &copy); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sessionID] = &copy
	return nil
}

// Load retrieves a session from memory
func (s *MemorySessionStore) Load(sessionID string) (*sessiontypes.SessionState, error) {
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

	var copy sessiontypes.SessionState
	if err := json.Unmarshal(data, &copy); err != nil {
		return nil, err
	}

	return &copy, nil
}

// Delete removes a session from memory
func (s *MemorySessionStore) Delete(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
	return nil
}

// List returns all session IDs in memory
func (s *MemorySessionStore) List() ([]string, error) {
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
