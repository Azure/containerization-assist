package session

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"

	bolt "go.etcd.io/bbolt"
)

// BoltSessionStore implements SessionStore interface using BoltDB
type BoltSessionStore struct {
	db             *bolt.DB
	sessionsBucket []byte
}

// NewBoltSessionStore creates a new BoltDB-backed session store
func NewBoltSessionStore(dbPath string) (*BoltSessionStore, error) {
	db, err := bolt.Open(dbPath, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, errors.NewError().
			Code(errors.CodeIOError).
			Type(errors.ErrTypeIO).
			Message("failed to open bolt database").
			Context("db_path", dbPath).
			Cause(err).Build()
	}

	store := &BoltSessionStore{
		db:             db,
		sessionsBucket: []byte("sessions"),
	}

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(store.sessionsBucket)
		return err
	})
	if err != nil {
		return nil, errors.NewError().
			Code(errors.CodeInternalError).
			Type(errors.ErrTypeInternal).
			Message("failed to create sessions bucket").
			Cause(err).Build()
	}

	return store, nil
}

// Create implements SessionStore.Create
func (s *BoltSessionStore) Create(ctx context.Context, metadata map[string]interface{}) (string, error) {
	sessionID := generateSessionID()
	session := &api.Session{
		ID:        sessionID,
		Metadata:  metadata,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	data, err := json.Marshal(session)
	if err != nil {
		return "", errors.NewError().
			Code(errors.CodeTypeConversionFailed).
			Type(errors.ErrTypeValidation).
			Message("failed to marshal session").
			Context("session_id", sessionID).
			Cause(err).Build()
	}

	err = s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(s.sessionsBucket)
		return b.Put([]byte(sessionID), data)
	})
	if err != nil {
		return "", errors.NewError().
			Code(errors.CodeIOError).
			Type(errors.ErrTypeIO).
			Message("failed to store session").
			Context("session_id", sessionID).
			Cause(err).Build()
	}

	return sessionID, nil
}

// Get implements SessionStore.Get
func (s *BoltSessionStore) Get(ctx context.Context, sessionID string) (*api.Session, error) {
	var session *api.Session

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(s.sessionsBucket)
		data := b.Get([]byte(sessionID))
		if data == nil {
			return errors.NewError().
				Code(errors.CodeResourceNotFound).
				Type(errors.ErrTypeNotFound).
				Message("session not found").
				Context("session_id", sessionID).Build()
		}

		session = &api.Session{}
		return json.Unmarshal(data, session)
	})

	if err != nil {
		if richErr, ok := err.(*errors.RichError); ok && richErr.Code == errors.CodeResourceNotFound {
			return nil, err
		}
		return nil, errors.NewError().
			Code(errors.CodeTypeConversionFailed).
			Type(errors.ErrTypeIO).
			Message("failed to retrieve session").
			Context("session_id", sessionID).
			Cause(err).Build()
	}

	return session, nil
}

// Update implements SessionStore.Update
func (s *BoltSessionStore) Update(ctx context.Context, sessionID string, data map[string]interface{}) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(s.sessionsBucket)
		existing := b.Get([]byte(sessionID))
		if existing == nil {
			return errors.NewError().
				Code(errors.CodeResourceNotFound).
				Type(errors.ErrTypeNotFound).
				Message("session not found for update").
				Context("session_id", sessionID).Build()
		}

		var session api.Session
		if err := json.Unmarshal(existing, &session); err != nil {
			return errors.NewError().
				Code(errors.CodeTypeConversionFailed).
				Type(errors.ErrTypeIO).
				Message("failed to unmarshal existing session").
				Context("session_id", sessionID).
				Cause(err).Build()
		}

		for key, value := range data {
			session.Metadata[key] = value
		}
		session.UpdatedAt = time.Now()

		updatedData, err := json.Marshal(session)
		if err != nil {
			return errors.NewError().
				Code(errors.CodeTypeConversionFailed).
				Type(errors.ErrTypeValidation).
				Message("failed to marshal updated session").
				Context("session_id", sessionID).
				Cause(err).Build()
		}

		return b.Put([]byte(sessionID), updatedData)
	})
}

// Delete implements SessionStore.Delete
func (s *BoltSessionStore) Delete(ctx context.Context, sessionID string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(s.sessionsBucket)
		existing := b.Get([]byte(sessionID))
		if existing == nil {
			return errors.NewError().
				Code(errors.CodeResourceNotFound).
				Type(errors.ErrTypeNotFound).
				Message("session not found for deletion").
				Context("session_id", sessionID).Build()
		}

		return b.Delete([]byte(sessionID))
	})
}

// Close closes the BoltDB database
func (s *BoltSessionStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// generateSessionID generates a unique session identifier
func generateSessionID() string {
	return fmt.Sprintf("session_%d", time.Now().UnixNano())
}
