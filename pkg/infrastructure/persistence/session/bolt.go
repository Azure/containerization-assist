// Package session provides infrastructure implementations for session storage
package session

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/containerization-assist/pkg/domain/errors"
	"github.com/Azure/containerization-assist/pkg/domain/session"
	"go.etcd.io/bbolt"
)

const (
	sessionsBucket = "sessions"
)

// BoltStore implements session.Store using BoltDB
type BoltStore struct {
	db *bbolt.DB
}

// NewBoltStore creates a new BoltDB-backed session store
func NewBoltStore(dbPath string, logger *slog.Logger) (*BoltStore, error) {
	// Ensure the parent directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, errors.New(errors.CodeIoError, "persistence", fmt.Sprintf("failed to create directory %s", dir), err)
	}

	db, err := bbolt.Open(dbPath, 0600, &bbolt.Options{
		Timeout: 1 * time.Second,
	})
	if err != nil {
		// Check if error is due to database being locked by another process
		if strings.Contains(err.Error(), "resource temporarily unavailable") || 
		   strings.Contains(err.Error(), "database is locked") ||
		   strings.Contains(err.Error(), "timeout") {
			return nil, errors.New(errors.CodeIoError, "persistence", 
				fmt.Sprintf("database file '%s' is already in use by another MCP server instance. "+
					"Use MCP_STORE_PATH environment variable to specify a different database file", dbPath), err)
		}
		return nil, errors.New(errors.CodeIoError, "persistence", "failed to open bolt db", err)
	}

	// Create the sessions bucket
	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(sessionsBucket))
		return err
	})
	if err != nil {
		_ = db.Close()
		return nil, errors.New(errors.CodeIoError, "persistence", "failed to create sessions bucket", err)
	}

	return &BoltStore{
		db: db,
	}, nil
}

// Close closes the BoltDB connection
func (s *BoltStore) Close() error {
	return s.db.Close()
}

// Create stores a new session
func (s *BoltStore) Create(ctx context.Context, sess session.Session) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(sessionsBucket))

		// Check if session already exists
		if bucket.Get([]byte(sess.ID)) != nil {
			return errors.New(errors.CodeAlreadyExists, "persistence", fmt.Sprintf("session %s already exists", sess.ID), nil)
		}

		data, err := json.Marshal(sess)
		if err != nil {
			return errors.New(errors.CodeInternalError, "persistence", "failed to marshal session", err)
		}

		err = bucket.Put([]byte(sess.ID), data)
		if err != nil {
			return errors.New(errors.CodeIoError, "persistence", "failed to store session", err)
		}

		return nil
	})
}

// Get retrieves a session by ID
func (s *BoltStore) Get(ctx context.Context, id string) (session.Session, error) {
	var sess session.Session

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(sessionsBucket))
		data := bucket.Get([]byte(id))

		if data == nil {
			return errors.New(errors.CodeNotFound, "persistence", fmt.Sprintf("session %s not found", id), nil)
		}

		return json.Unmarshal(data, &sess)
	})

	if err != nil {
		return session.Session{}, err
	}

	return sess, nil
}

// Update modifies an existing session
func (s *BoltStore) Update(ctx context.Context, sess session.Session) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(sessionsBucket))

		// Check if session exists
		if bucket.Get([]byte(sess.ID)) == nil {
			return errors.New(errors.CodeNotFound, "persistence", fmt.Sprintf("session %s not found", sess.ID), nil)
		}

		data, err := json.Marshal(sess)
		if err != nil {
			return errors.New(errors.CodeInternalError, "persistence", "failed to marshal session", err)
		}

		err = bucket.Put([]byte(sess.ID), data)
		if err != nil {
			return errors.New(errors.CodeIoError, "persistence", "failed to update session", err)
		}

		return nil
	})
}

// Delete removes a session
func (s *BoltStore) Delete(ctx context.Context, id string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(sessionsBucket))

		if bucket.Get([]byte(id)) == nil {
			return errors.New(errors.CodeNotFound, "persistence", fmt.Sprintf("session %s not found", id), nil)
		}

		err := bucket.Delete([]byte(id))
		if err != nil {
			return errors.New(errors.CodeIoError, "persistence", "failed to delete session", err)
		}

		return nil
	})
}

// List returns all sessions, optionally filtered
func (s *BoltStore) List(ctx context.Context, filters ...session.Filter) ([]session.Session, error) {
	var sessions []session.Session

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(sessionsBucket))

		return bucket.ForEach(func(k, v []byte) error {
			var sess session.Session
			if err := json.Unmarshal(v, &sess); err != nil {
				return nil // Continue iteration
			}

			// Apply filters
			include := true
			for _, filter := range filters {
				if !filter.Apply(sess) {
					include = false
					break
				}
			}

			if include {
				sessions = append(sessions, sess)
			}

			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	return sessions, nil
}

// Exists checks if a session exists
func (s *BoltStore) Exists(ctx context.Context, id string) (bool, error) {
	var exists bool

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(sessionsBucket))
		exists = bucket.Get([]byte(id)) != nil
		return nil
	})

	return exists, err
}

// Cleanup removes expired sessions
func (s *BoltStore) Cleanup(ctx context.Context) (int, error) {
	var removedCount int

	err := s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(sessionsBucket))

		// Collect expired session IDs
		var expiredIDs []string

		err := bucket.ForEach(func(k, v []byte) error {
			var sess session.Session
			if err := json.Unmarshal(v, &sess); err != nil {
				return nil // Continue iteration
			}

			if sess.IsExpired() {
				expiredIDs = append(expiredIDs, sess.ID)
			}

			return nil
		})
		if err != nil {
			return err
		}

		// Delete expired sessions
		for _, id := range expiredIDs {
			if err := bucket.Delete([]byte(id)); err != nil {
				continue
			}
			removedCount++
		}

		if removedCount > 0 {
		}

		return nil
	})

	return removedCount, err
}

// Stats returns storage statistics
func (s *BoltStore) Stats(ctx context.Context) (session.Stats, error) {
	var stats session.Stats
	var activeSessions, totalSessions int

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(sessionsBucket))

		return bucket.ForEach(func(k, v []byte) error {
			totalSessions++

			var sess session.Session
			if err := json.Unmarshal(v, &sess); err != nil {
				return nil // Continue counting
			}

			if sess.IsActive() {
				activeSessions++
			}

			return nil
		})
	})

	if err != nil {
		return session.Stats{}, err
	}

	stats.ActiveSessions = activeSessions
	stats.TotalSessions = totalSessions
	// MaxSessions would be configured at the service level, not storage level

	return stats, nil
}
