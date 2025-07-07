package session

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"

	bolt "go.etcd.io/bbolt"
)

// BoltSessionState implements SessionState interface using BoltDB
type BoltSessionState struct {
	db                *bolt.DB
	statesBucket      []byte
	checkpointsBucket []byte
}

// NewBoltSessionState creates a new BoltDB-backed session state manager
func NewBoltSessionState(dbPath string) (*BoltSessionState, error) {
	db, err := bolt.Open(dbPath, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, errors.NewError().
			Code(errors.CodeIOError).
			Type(errors.ErrTypeIO).
			Message("failed to open bolt database for session state").
			Context("db_path", dbPath).
			Cause(err).Build()
	}

	store := &BoltSessionState{
		db:                db,
		statesBucket:      []byte("session_states"),
		checkpointsBucket: []byte("session_checkpoints"),
	}

	err = db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(store.statesBucket); err != nil {
			return err
		}
		_, err := tx.CreateBucketIfNotExists(store.checkpointsBucket)
		return err
	})
	if err != nil {
		return nil, errors.NewError().
			Code(errors.CodeInternalError).
			Type(errors.ErrTypeInternal).
			Message("failed to create session state buckets").
			Cause(err).Build()
	}

	return store, nil
}

// SaveState implements SessionState.SaveState
func (s *BoltSessionState) SaveState(ctx context.Context, sessionID string, state map[string]interface{}) error {
	stateData := map[string]interface{}{
		"data":       state,
		"updated_at": time.Now(),
	}

	data, err := json.Marshal(stateData)
	if err != nil {
		return errors.NewError().
			Code(errors.CodeTypeConversionFailed).
			Type(errors.ErrTypeValidation).
			Message("failed to marshal session state").
			Context("session_id", sessionID).
			Cause(err).Build()
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(s.statesBucket)
		err := b.Put([]byte(sessionID), data)
		if err != nil {
			return errors.NewError().
				Code(errors.CodeIOError).
				Type(errors.ErrTypeIO).
				Message("failed to save session state").
				Context("session_id", sessionID).
				Cause(err).Build()
		}
		return nil
	})
}

// LoadState implements SessionState.LoadState
func (s *BoltSessionState) LoadState(ctx context.Context, sessionID string) (map[string]interface{}, error) {
	var state map[string]interface{}

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(s.statesBucket)
		data := b.Get([]byte(sessionID))
		if data == nil {
			return errors.NewError().
				Code(errors.CodeResourceNotFound).
				Type(errors.ErrTypeNotFound).
				Message("session state not found").
				Context("session_id", sessionID).Build()
		}

		var stateData map[string]interface{}
		if err := json.Unmarshal(data, &stateData); err != nil {
			return errors.NewError().
				Code(errors.CodeTypeConversionFailed).
				Type(errors.ErrTypeIO).
				Message("failed to unmarshal session state").
				Context("session_id", sessionID).
				Cause(err).Build()
		}

		if data, ok := stateData["data"].(map[string]interface{}); ok {
			state = data
		} else {
			return errors.NewError().
				Code(errors.CodeInvalidParameter).
				Type(errors.ErrTypeValidation).
				Message("invalid session state format").
				Context("session_id", sessionID).Build()
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return state, nil
}

// SaveCheckpoint implements SessionState.SaveCheckpoint
func (s *BoltSessionState) SaveCheckpoint(ctx context.Context, sessionID string, data interface{}) error {
	checkpointData := map[string]interface{}{
		"data":       data,
		"created_at": time.Now(),
	}

	jsonData, err := json.Marshal(checkpointData)
	if err != nil {
		return errors.NewError().
			Code(errors.CodeTypeConversionFailed).
			Type(errors.ErrTypeValidation).
			Message("failed to marshal checkpoint data").
			Context("session_id", sessionID).
			Cause(err).Build()
	}

	checkpointKey := fmt.Sprintf("%s_checkpoint", sessionID)

	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(s.checkpointsBucket)
		err := b.Put([]byte(checkpointKey), jsonData)
		if err != nil {
			return errors.NewError().
				Code(errors.CodeIOError).
				Type(errors.ErrTypeIO).
				Message("failed to save checkpoint").
				Context("session_id", sessionID).
				Context("checkpoint_key", checkpointKey).
				Cause(err).Build()
		}
		return nil
	})
}

// LoadCheckpoint implements SessionState.LoadCheckpoint
func (s *BoltSessionState) LoadCheckpoint(ctx context.Context, sessionID string) (interface{}, error) {
	checkpointKey := fmt.Sprintf("%s_checkpoint", sessionID)
	var checkpointData interface{}

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(s.checkpointsBucket)
		data := b.Get([]byte(checkpointKey))
		if data == nil {
			return errors.NewError().
				Code(errors.CodeResourceNotFound).
				Type(errors.ErrTypeNotFound).
				Message("checkpoint not found").
				Context("session_id", sessionID).
				Context("checkpoint_key", checkpointKey).Build()
		}

		var checkpoint map[string]interface{}
		if err := json.Unmarshal(data, &checkpoint); err != nil {
			return errors.NewError().
				Code(errors.CodeTypeConversionFailed).
				Type(errors.ErrTypeIO).
				Message("failed to unmarshal checkpoint").
				Context("session_id", sessionID).
				Context("checkpoint_key", checkpointKey).
				Cause(err).Build()
		}

		if data, ok := checkpoint["data"]; ok {
			checkpointData = data
		} else {
			return errors.NewError().
				Code(errors.CodeInvalidParameter).
				Type(errors.ErrTypeValidation).
				Message("invalid checkpoint format").
				Context("session_id", sessionID).
				Context("checkpoint_key", checkpointKey).Build()
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return checkpointData, nil
}

// Close closes the BoltDB database
func (s *BoltSessionState) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
