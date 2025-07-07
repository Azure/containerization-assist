package checkpoint

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/orchestration/execution"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
	"github.com/google/uuid"
	"go.etcd.io/bbolt"
)

// ListCheckpoints returns all checkpoint IDs for a session
func (cm *BoltCheckpointManager) ListCheckpoints(sessionID string) ([]string, error) {
	var checkpointIDs []string

	err := cm.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(checkpointsBucket))
		if bucket == nil {
			return nil // No checkpoints yet
		}

		prefix := sessionID + ":"
		c := bucket.Cursor()

		for k, _ := c.Seek([]byte(prefix)); k != nil && strings.HasPrefix(string(k), prefix); k, _ = c.Next() {
			// Extract checkpoint ID from key
			parts := strings.Split(string(k), ":")
			if len(parts) >= 2 {
				checkpointIDs = append(checkpointIDs, parts[1])
			}
		}

		return nil
	})

	if err != nil {
		return nil, errors.Wrap(err, "orchestration", "failed to list checkpoints")
	}

	cm.logger.Debug().
		Str("session_id", sessionID).
		Int("count", len(checkpointIDs)).
		Msg("Listed checkpoints")

	return checkpointIDs, nil
}

// GetCheckpoint retrieves a specific checkpoint
func (cm *BoltCheckpointManager) GetCheckpoint(sessionID, checkpointID string) (*execution.WorkflowCheckpoint, error) {
	var checkpoint *execution.WorkflowCheckpoint

	err := cm.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(checkpointsBucket))
		if bucket == nil {
			return errors.NewError().Messagef("checkpoint not found").WithLocation().Build()
		}

		key := fmt.Sprintf("%s:%s", sessionID, checkpointID)
		data := bucket.Get([]byte(key))
		if data == nil {
			return errors.NewError().Messagef("checkpoint not found").WithLocation().Build()
		}

		var envelope CheckpointEnvelope
		if err := json.Unmarshal(data, &envelope); err != nil {
			return errors.Wrap(err, "orchestration", "failed to unmarshal envelope")
		}

		// Verify integrity if enabled
		if cm.enableIntegrity {
			if err := cm.verifyChecksum(envelope.Data, envelope.Checksum); err != nil {
				return err
			}
		}

		// Decompress if needed
		checkpointData, err := cm.decompressData(envelope.Data, envelope.Compressed)
		if err != nil {
			return err
		}

		// Deserialize checkpoint
		checkpoint = &execution.WorkflowCheckpoint{}
		if err := json.Unmarshal(checkpointData, checkpoint); err != nil {
			return errors.Wrap(err, "orchestration", "failed to unmarshal checkpoint")
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	cm.logger.Debug().
		Str("session_id", sessionID).
		Str("checkpoint_id", checkpointID).
		Msg("Retrieved checkpoint")

	return checkpoint, nil
}

// DeleteCheckpoint removes a specific checkpoint
func (cm *BoltCheckpointManager) DeleteCheckpoint(sessionID, checkpointID string) error {
	err := cm.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(checkpointsBucket))
		if bucket != nil {
			key := fmt.Sprintf("%s:%s", sessionID, checkpointID)
			if err := bucket.Delete([]byte(key)); err != nil {
				return err
			}
		}

		metaBucket := tx.Bucket([]byte(metadataBucket))
		if metaBucket != nil {
			metaKey := fmt.Sprintf("%s:%s:meta", sessionID, checkpointID)
			if err := metaBucket.Delete([]byte(metaKey)); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return errors.Wrap(err, "orchestration", "failed to delete checkpoint")
	}

	cm.logger.Info().
		Str("session_id", sessionID).
		Str("checkpoint_id", checkpointID).
		Msg("Deleted checkpoint")

	return nil
}

// CleanupCheckpoints removes all checkpoints for a session
func (cm *BoltCheckpointManager) CleanupCheckpoints(sessionID string) error {
	deletedCount := 0

	err := cm.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(checkpointsBucket))
		if bucket != nil {
			prefix := sessionID + ":"
			c := bucket.Cursor()

			var keysToDelete [][]byte
			for k, _ := c.Seek([]byte(prefix)); k != nil && strings.HasPrefix(string(k), prefix); k, _ = c.Next() {
				keysToDelete = append(keysToDelete, bytes.Clone(k))
			}

			for _, key := range keysToDelete {
				if err := bucket.Delete(key); err != nil {
					return err
				}
				deletedCount++
			}
		}

		metaBucket := tx.Bucket([]byte(metadataBucket))
		if metaBucket != nil {
			prefix := sessionID + ":"
			c := metaBucket.Cursor()

			var keysToDelete [][]byte
			for k, _ := c.Seek([]byte(prefix)); k != nil && strings.HasPrefix(string(k), prefix); k, _ = c.Next() {
				keysToDelete = append(keysToDelete, bytes.Clone(k))
			}

			for _, key := range keysToDelete {
				if err := metaBucket.Delete(key); err != nil {
					return err
				}
			}
		}

		return nil
	})

	if err != nil {
		return errors.Wrap(err, "orchestration", "failed to cleanup checkpoints")
	}

	cm.logger.Info().
		Str("session_id", sessionID).
		Int("deleted_count", deletedCount).
		Msg("Cleaned up checkpoints")

	return nil
}

// GetCheckpointMetadata retrieves metadata about checkpoints for a session
func (cm *BoltCheckpointManager) GetCheckpointMetadata(sessionID string) (map[string]interface{}, error) {
	metadata := make(map[string]interface{})

	err := cm.db.View(func(tx *bbolt.Tx) error {
		checkpointCount := 0
		var lastCheckpointTime time.Time
		totalSize := int64(0)

		bucket := tx.Bucket([]byte(checkpointsBucket))
		if bucket != nil {
			prefix := sessionID + ":"
			c := bucket.Cursor()

			for k, v := c.Seek([]byte(prefix)); k != nil && strings.HasPrefix(string(k), prefix); k, v = c.Next() {
				checkpointCount++
				totalSize += int64(len(v))

				var envelope CheckpointEnvelope
				if err := json.Unmarshal(v, &envelope); err == nil {
					if envelope.CreatedAt.After(lastCheckpointTime) {
						lastCheckpointTime = envelope.CreatedAt
					}
				}
			}
		}

		metadata["checkpoint_count"] = checkpointCount
		metadata["total_size_bytes"] = totalSize
		if !lastCheckpointTime.IsZero() {
			metadata["last_checkpoint_time"] = lastCheckpointTime
		}

		metaBucket := tx.Bucket([]byte(metadataBucket))
		if metaBucket != nil {
			latestKey := fmt.Sprintf("%s:latest", sessionID)
			if data := metaBucket.Get([]byte(latestKey)); data != nil {
				metadata["latest_checkpoint_id"] = string(data)
			}
		}

		return nil
	})

	if err != nil {
		return nil, errors.Wrap(err, "orchestration", "failed to get checkpoint metadata")
	}

	return metadata, nil
}

// ValidateCheckpoint verifies the integrity of a checkpoint
func (cm *BoltCheckpointManager) ValidateCheckpoint(sessionID, checkpointID string) error {
	return cm.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(checkpointsBucket))
		if bucket == nil {
			return errors.NewError().Messagef("checkpoint not found").WithLocation().Build()
		}

		key := fmt.Sprintf("%s:%s", sessionID, checkpointID)
		data := bucket.Get([]byte(key))
		if data == nil {
			return errors.NewError().Messagef("checkpoint not found").WithLocation().Build()
		}

		var envelope CheckpointEnvelope
		if err := json.Unmarshal(data, &envelope); err != nil {
			return errors.Wrap(err, "orchestration", "failed to unmarshal envelope")
		}

		if err := cm.verifyChecksum(envelope.Data, envelope.Checksum); err != nil {
			return err
		}

		checkpointData, err := cm.decompressData(envelope.Data, envelope.Compressed)
		if err != nil {
			return err
		}

		var checkpoint session.WorkflowCheckpoint
		if err := json.Unmarshal(checkpointData, &checkpoint); err != nil {
			return errors.Wrap(err, "orchestration", "checkpoint data is corrupted")
		}

		return nil
	})
}

// storeMetadata stores checkpoint metadata
func (cm *BoltCheckpointManager) storeMetadata(tx *bbolt.Tx, sessionID, checkpointID string, meta map[string]interface{}) error {
	bucket, err := tx.CreateBucketIfNotExists([]byte(metadataBucket))
	if err != nil {
		return err
	}

	metaKey := fmt.Sprintf("%s:%s:meta", sessionID, checkpointID)
	metaData, err := json.Marshal(meta)
	if err != nil {
		return err
	}

	if err := bucket.Put([]byte(metaKey), metaData); err != nil {
		return err
	}

	latestKey := fmt.Sprintf("%s:latest", sessionID)
	if err := bucket.Put([]byte(latestKey), []byte(checkpointID)); err != nil {
		return err
	}

	return nil
}

// generateCheckpointID creates a unique checkpoint ID
func (cm *BoltCheckpointManager) generateCheckpointID() string {
	return uuid.New().String()
}
