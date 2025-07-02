package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	bolt "go.etcd.io/bbolt"
)

// WorkflowPersistence provides workflow state persistence and recovery
type WorkflowPersistence struct {
	db     *bolt.DB
	logger zerolog.Logger
}

// NewWorkflowPersistence creates a new workflow persistence manager
func NewWorkflowPersistence(dbPath string, logger zerolog.Logger) (*WorkflowPersistence, error) {
	db, err := bolt.Open(dbPath, 0600, &bolt.Options{
		Timeout: 1 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open persistence database: %w", err)
	}

	// Initialize buckets
	err = db.Update(func(tx *bolt.Tx) error {
		buckets := []string{
			"workflow_sessions",
			"workflow_checkpoints",
			"workflow_specs",
			"workflow_templates",
			"workflow_history",
		}

		for _, bucket := range buckets {
			_, err := tx.CreateBucketIfNotExists([]byte(bucket))
			if err != nil {
				return fmt.Errorf("failed to create bucket %s: %w", bucket, err)
			}
		}

		return nil
	})
	if err != nil {
		db.Close()
		return nil, err
	}

	return &WorkflowPersistence{
		db:     db,
		logger: logger.With().Str("component", "workflow_persistence").Logger(),
	}, nil
}

// SaveSession saves a workflow execution session
func (wp *WorkflowPersistence) SaveSession(session *ExecutionSession) error {
	return wp.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("workflow_sessions"))

		data, err := json.Marshal(session)
		if err != nil {
			return fmt.Errorf("failed to marshal session: %w", err)
		}

		err = bucket.Put([]byte(session.SessionID), data)
		if err != nil {
			return fmt.Errorf("failed to save session: %w", err)
		}

		wp.logger.Debug().
			Str("session_id", session.SessionID).
			Str("workflow_id", session.WorkflowID).
			Str("status", session.Status).
			Msg("Session saved to persistence")

		return nil
	})
}

// LoadSession loads a workflow execution session
func (wp *WorkflowPersistence) LoadSession(sessionID string) (*ExecutionSession, error) {
	var session ExecutionSession

	err := wp.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("workflow_sessions"))

		data := bucket.Get([]byte(sessionID))
		if data == nil {
			return fmt.Errorf("session not found: %s", sessionID)
		}

		err := json.Unmarshal(data, &session)
		if err != nil {
			return fmt.Errorf("failed to unmarshal session: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	wp.logger.Debug().
		Str("session_id", sessionID).
		Str("workflow_id", session.WorkflowID).
		Str("status", session.Status).
		Msg("Session loaded from persistence")

	return &session, nil
}

// SaveCheckpoint saves a workflow checkpoint
func (wp *WorkflowPersistence) SaveCheckpoint(checkpoint *WorkflowCheckpoint) error {
	return wp.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("workflow_checkpoints"))

		data, err := json.Marshal(checkpoint)
		if err != nil {
			return fmt.Errorf("failed to marshal checkpoint: %w", err)
		}

		// Use composite key: sessionID_stageID_timestamp
		key := fmt.Sprintf("%s_%s_%d",
			checkpoint.SessionID,
			checkpoint.StageID,
			checkpoint.Timestamp.UnixNano())

		err = bucket.Put([]byte(key), data)
		if err != nil {
			return fmt.Errorf("failed to save checkpoint: %w", err)
		}

		wp.logger.Debug().
			Str("checkpoint_id", checkpoint.ID).
			Str("session_id", checkpoint.SessionID).
			Str("stage_id", checkpoint.StageID).
			Msg("Checkpoint saved to persistence")

		return nil
	})
}

// LoadCheckpoints loads all checkpoints for a session
func (wp *WorkflowPersistence) LoadCheckpoints(sessionID string) ([]WorkflowCheckpoint, error) {
	var checkpoints []WorkflowCheckpoint

	err := wp.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("workflow_checkpoints"))

		// Iterate through all checkpoints and filter by session ID
		c := bucket.Cursor()
		prefix := []byte(sessionID + "_")

		for k, v := c.Seek(prefix); k != nil && len(k) >= len(prefix) && string(k[:len(prefix)]) == string(prefix); k, v = c.Next() {
			var checkpoint WorkflowCheckpoint

			err := json.Unmarshal(v, &checkpoint)
			if err != nil {
				wp.logger.Warn().
					Err(err).
					Str("key", string(k)).
					Msg("Failed to unmarshal checkpoint")
				continue
			}

			checkpoints = append(checkpoints, checkpoint)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	wp.logger.Debug().
		Str("session_id", sessionID).
		Int("checkpoint_count", len(checkpoints)).
		Msg("Checkpoints loaded from persistence")

	return checkpoints, nil
}

// SaveWorkflowSpec saves a workflow specification
func (wp *WorkflowPersistence) SaveWorkflowSpec(spec *WorkflowSpec) error {
	return wp.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("workflow_specs"))

		data, err := json.Marshal(spec)
		if err != nil {
			return fmt.Errorf("failed to marshal workflow spec: %w", err)
		}

		err = bucket.Put([]byte(spec.ID), data)
		if err != nil {
			return fmt.Errorf("failed to save workflow spec: %w", err)
		}

		wp.logger.Debug().
			Str("workflow_id", spec.ID).
			Str("workflow_name", spec.Name).
			Str("version", spec.Version).
			Msg("Workflow spec saved to persistence")

		return nil
	})
}

// LoadWorkflowSpec loads a workflow specification
func (wp *WorkflowPersistence) LoadWorkflowSpec(workflowID string) (*WorkflowSpec, error) {
	var spec WorkflowSpec

	err := wp.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("workflow_specs"))

		data := bucket.Get([]byte(workflowID))
		if data == nil {
			return fmt.Errorf("workflow spec not found: %s", workflowID)
		}

		err := json.Unmarshal(data, &spec)
		if err != nil {
			return fmt.Errorf("failed to unmarshal workflow spec: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	wp.logger.Debug().
		Str("workflow_id", workflowID).
		Str("workflow_name", spec.Name).
		Msg("Workflow spec loaded from persistence")

	return &spec, nil
}

// ListSessions lists all workflow sessions with optional filtering
func (wp *WorkflowPersistence) ListSessions(filter SessionFilter) ([]*ExecutionSession, error) {
	var sessions []*ExecutionSession

	err := wp.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("workflow_sessions"))

		c := bucket.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var session ExecutionSession

			err := json.Unmarshal(v, &session)
			if err != nil {
				wp.logger.Warn().
					Err(err).
					Str("session_id", string(k)).
					Msg("Failed to unmarshal session")
				continue
			}

			// Apply filters
			if !wp.matchesFilter(&session, filter) {
				continue
			}

			sessions = append(sessions, &session)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	wp.logger.Debug().
		Int("session_count", len(sessions)).
		Msg("Sessions loaded from persistence")

	return sessions, nil
}

// matchesFilter checks if a session matches the given filter
func (wp *WorkflowPersistence) matchesFilter(session *ExecutionSession, filter SessionFilter) bool {
	// Filter by status
	if filter.Status != "" && session.Status != filter.Status {
		return false
	}

	// Filter by workflow ID
	if filter.WorkflowID != "" && session.WorkflowID != filter.WorkflowID {
		return false
	}

	// Filter by workflow name
	if filter.WorkflowName != "" && session.WorkflowName != filter.WorkflowName {
		return false
	}

	// Filter by start time
	if filter.StartTime != nil {
		if filter.StartAfter.After(session.StartTime) {
			return false
		}
		if filter.EndTime != nil && filter.EndTime.Before(session.StartTime) {
			return false
		}
	}

	// Filter by labels
	if len(filter.Labels) > 0 {
		for key, value := range filter.Labels {
			if sessionValue, exists := session.Labels[key]; !exists || sessionValue != value {
				return false
			}
		}
	}

	return true
}

// SaveWorkflowHistory saves workflow execution history
func (wp *WorkflowPersistence) SaveWorkflowHistory(sessionID string, event WorkflowHistoryEvent) error {
	return wp.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("workflow_history"))

		// Create sub-bucket for session if it doesn't exist
		sessionBucket, err := bucket.CreateBucketIfNotExists([]byte(sessionID))
		if err != nil {
			return fmt.Errorf("failed to create session bucket: %w", err)
		}

		data, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("failed to marshal history event: %w", err)
		}

		// Use timestamp as key for ordering
		key := fmt.Sprintf("%d_%s", event.Timestamp.UnixNano(), event.ID)

		err = sessionBucket.Put([]byte(key), data)
		if err != nil {
			return fmt.Errorf("failed to save history event: %w", err)
		}

		return nil
	})
}

// LoadWorkflowHistory loads workflow execution history
func (wp *WorkflowPersistence) LoadWorkflowHistory(sessionID string) ([]WorkflowHistoryEvent, error) {
	var events []WorkflowHistoryEvent

	err := wp.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("workflow_history"))

		sessionBucket := bucket.Bucket([]byte(sessionID))
		if sessionBucket == nil {
			return nil // No history for this session
		}

		c := sessionBucket.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var event WorkflowHistoryEvent

			err := json.Unmarshal(v, &event)
			if err != nil {
				wp.logger.Warn().
					Err(err).
					Str("key", string(k)).
					Msg("Failed to unmarshal history event")
				continue
			}

			events = append(events, event)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return events, nil
}

// RecoverSession recovers a workflow session from the last checkpoint
func (wp *WorkflowPersistence) RecoverSession(ctx context.Context, sessionID string) (*RecoveredSession, error) {
	// Load the session
	session, err := wp.LoadSession(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load session: %w", err)
	}

	// Load checkpoints
	checkpoints, err := wp.LoadCheckpoints(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load checkpoints: %w", err)
	}

	// Find the latest valid checkpoint
	var latestCheckpoint *WorkflowCheckpoint
	for i := len(checkpoints) - 1; i >= 0; i-- {
		checkpoint := &checkpoints[i]
		// Validate checkpoint (could add more validation here)
		if checkpoint.SessionID == sessionID {
			latestCheckpoint = checkpoint
			break
		}
	}

	// Load workflow spec
	var spec *WorkflowSpec
	if latestCheckpoint != nil && latestCheckpoint.WorkflowSpec != nil {
		spec = latestCheckpoint.WorkflowSpec
	} else {
		spec, err = wp.LoadWorkflowSpec(session.WorkflowID)
		if err != nil {
			wp.logger.Warn().
				Err(err).
				Str("workflow_id", session.WorkflowID).
				Msg("Failed to load workflow spec for recovery")
		}
	}

	// Load workflow history
	history, err := wp.LoadWorkflowHistory(sessionID)
	if err != nil {
		wp.logger.Warn().
			Err(err).
			Str("session_id", sessionID).
			Msg("Failed to load workflow history")
	}

	recovered := &RecoveredSession{
		Session:          session,
		LastCheckpoint:   latestCheckpoint,
		WorkflowSpec:     spec,
		History:          history,
		RecoveryTime:     time.Now(),
		RecoveryStrategy: wp.determineRecoveryStrategy(session, latestCheckpoint),
	}

	wp.logger.Info().
		Str("session_id", sessionID).
		Str("status", session.Status).
		Str("recovery_strategy", recovered.RecoveryStrategy).
		Bool("has_checkpoint", latestCheckpoint != nil).
		Msg("Session recovered from persistence")

	return recovered, nil
}

// determineRecoveryStrategy determines the best recovery strategy
func (wp *WorkflowPersistence) determineRecoveryStrategy(session *ExecutionSession, checkpoint *WorkflowCheckpoint) string {
	if checkpoint == nil {
		return "restart" // No checkpoint, must restart
	}

	// Check session status
	switch session.Status {
	case WorkflowStatusPaused:
		return "resume" // Can resume from checkpoint
	case WorkflowStatusFailed:
		if len(session.FailedStages) > 0 {
			return "retry_failed" // Retry only failed stages
		}
		return "resume" // Resume from checkpoint
	case WorkflowStatusRunning:
		// Check if session is stale (no activity for 10 minutes)
		if time.Since(session.LastActivity) > 10*time.Minute {
			return "resume_stale" // Resume from checkpoint (likely crashed)
		}
		return "wait" // Still running, wait for completion
	case WorkflowStatusCompleted:
		return "completed" // Nothing to recover
	default:
		return "restart" // Unknown status, restart
	}
}

// DeleteSession deletes a workflow session and its associated data
func (wp *WorkflowPersistence) DeleteSession(sessionID string) error {
	return wp.db.Update(func(tx *bolt.Tx) error {
		// Delete from sessions bucket
		sessionBucket := tx.Bucket([]byte("workflow_sessions"))
		if err := sessionBucket.Delete([]byte(sessionID)); err != nil {
			return fmt.Errorf("failed to delete session: %w", err)
		}

		// Delete checkpoints
		checkpointBucket := tx.Bucket([]byte("workflow_checkpoints"))
		c := checkpointBucket.Cursor()
		prefix := []byte(sessionID + "_")

		for k, _ := c.Seek(prefix); k != nil && len(k) >= len(prefix) && string(k[:len(prefix)]) == string(prefix); k, _ = c.Next() {
			if err := checkpointBucket.Delete(k); err != nil {
				wp.logger.Warn().
					Err(err).
					Str("key", string(k)).
					Msg("Failed to delete checkpoint")
			}
		}

		// Delete history
		historyBucket := tx.Bucket([]byte("workflow_history"))
		if err := historyBucket.DeleteBucket([]byte(sessionID)); err != nil && err != bolt.ErrBucketNotFound {
			wp.logger.Warn().
				Err(err).
				Str("session_id", sessionID).
				Msg("Failed to delete history bucket")
		}

		wp.logger.Info().
			Str("session_id", sessionID).
			Msg("Session and associated data deleted from persistence")

		return nil
	})
}

// Close closes the persistence database
func (wp *WorkflowPersistence) Close() error {
	return wp.db.Close()
}

// WorkflowHistoryEvent represents an event in workflow execution history
type WorkflowHistoryEvent struct {
	ID        string                 `json:"id"`
	SessionID string                 `json:"session_id"`
	EventType string                 `json:"event_type"`
	StageID   string                 `json:"stage_id,omitempty"`
	ToolName  string                 `json:"tool_name,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Duration  time.Duration          `json:"duration,omitempty"`
	Success   bool                   `json:"success"`
	Error     string                 `json:"error,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// RecoveredSession represents a recovered workflow session
type RecoveredSession struct {
	Session          *ExecutionSession      `json:"session"`
	LastCheckpoint   *WorkflowCheckpoint    `json:"last_checkpoint,omitempty"`
	WorkflowSpec     *WorkflowSpec          `json:"workflow_spec,omitempty"`
	History          []WorkflowHistoryEvent `json:"history,omitempty"`
	RecoveryTime     time.Time              `json:"recovery_time"`
	RecoveryStrategy string                 `json:"recovery_strategy"`
}
