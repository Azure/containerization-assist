package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/orchestration/execution"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
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
		return nil, errors.NewError().Message("failed to open persistence database").Cause(err).WithLocation().Build()
	}

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
				return errors.NewError().Message(fmt.Sprintf("failed to create bucket %s", bucket)).Cause(err).Build()
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
func (wp *WorkflowPersistence) SaveSession(session *execution.ExecutionSession) error {
	return wp.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("workflow_sessions"))

		data, err := json.Marshal(session)
		if err != nil {
			return errors.NewError().Message("failed to marshal session").Cause(err).Build()
		}

		err = bucket.Put([]byte(session.SessionID), data)
		if err != nil {
			return errors.NewError().Message("failed to save session").Cause(err).Build()
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
func (wp *WorkflowPersistence) LoadSession(sessionID string) (*execution.ExecutionSession, error) {
	var session execution.ExecutionSession

	err := wp.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("workflow_sessions"))

		data := bucket.Get([]byte(sessionID))
		if data == nil {
			return errors.NewError().Messagef("session not found: %s", sessionID).Build()
		}

		err := json.Unmarshal(data, &session)
		if err != nil {
			return errors.NewError().Message("failed to unmarshal session").Cause(err).Build()
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
func (wp *WorkflowPersistence) SaveCheckpoint(checkpoint *execution.WorkflowCheckpoint) error {
	return wp.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("workflow_checkpoints"))

		data, err := json.Marshal(checkpoint)
		if err != nil {
			return errors.NewError().Message("failed to marshal checkpoint").Cause(err).WithLocation().Build()
		}

		key := fmt.Sprintf("%s_%s_%d",
			checkpoint.SessionID,
			checkpoint.StageID,
			checkpoint.Timestamp.UnixNano())

		err = bucket.Put([]byte(key), data)
		if err != nil {
			return errors.NewError().Message("failed to save checkpoint").Cause(err).Build()
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
func (wp *WorkflowPersistence) LoadCheckpoints(sessionID string) ([]execution.WorkflowCheckpoint, error) {
	var checkpoints []execution.WorkflowCheckpoint

	err := wp.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("workflow_checkpoints"))

		c := bucket.Cursor()
		prefix := []byte(sessionID + "_")

		for k, v := c.Seek(prefix); k != nil && len(k) >= len(prefix) && string(k[:len(prefix)]) == string(prefix); k, v = c.Next() {
			var checkpoint execution.WorkflowCheckpoint

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
func (wp *WorkflowPersistence) SaveWorkflowSpec(spec *execution.WorkflowSpec) error {
	return wp.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("workflow_specs"))

		data, err := json.Marshal(spec)
		if err != nil {
			return errors.NewError().Message("failed to marshal workflow spec").Cause(err).Build()
		}

		err = bucket.Put([]byte(spec.ID), data)
		if err != nil {
			return errors.NewError().Message("failed to save workflow spec").Cause(err).Build()
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
func (wp *WorkflowPersistence) LoadWorkflowSpec(workflowID string) (*execution.WorkflowSpec, error) {
	var spec execution.WorkflowSpec

	err := wp.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("workflow_specs"))

		data := bucket.Get([]byte(workflowID))
		if data == nil {
			return errors.NewError().Messagef("workflow spec not found: %s", workflowID).Build()
		}

		err := json.Unmarshal(data, &spec)
		if err != nil {
			return errors.NewError().Message("failed to unmarshal workflow spec").Cause(err).Build()
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
func (wp *WorkflowPersistence) ListSessions(filter execution.SessionFilter) ([]*execution.ExecutionSession, error) {
	var sessions []*execution.ExecutionSession

	err := wp.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("workflow_sessions"))

		c := bucket.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var session execution.ExecutionSession

			err := json.Unmarshal(v, &session)
			if err != nil {
				wp.logger.Warn().
					Err(err).
					Str("session_id", string(k)).
					Msg("Failed to unmarshal session")
				continue
			}

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
func (wp *WorkflowPersistence) matchesFilter(session *execution.ExecutionSession, filter execution.SessionFilter) bool {
	if filter.Status != "" && session.Status != filter.Status {
		return false
	}

	if filter.WorkflowID != "" && session.WorkflowID != filter.WorkflowID {
		return false
	}

	if filter.WorkflowName != "" && session.WorkflowName != filter.WorkflowName {
		return false
	}

	if filter.StartTime != nil {
		if filter.StartAfter.After(session.StartTime) {
			return false
		}
		if filter.EndTime != nil && filter.EndTime.Before(session.StartTime) {
			return false
		}
	}

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

		sessionBucket, err := bucket.CreateBucketIfNotExists([]byte(sessionID))
		if err != nil {
			return errors.NewError().Message("failed to create session bucket").Cause(err).Build()
		}

		data, err := json.Marshal(event)
		if err != nil {
			return errors.NewError().Message("failed to marshal history event").Cause(err).WithLocation().Build()
		}

		key := fmt.Sprintf("%d_%s", event.Timestamp.UnixNano(), event.ID)

		err = sessionBucket.Put([]byte(key), data)
		if err != nil {
			return errors.NewError().Message("failed to save history event").Cause(err).Build()
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
			return nil
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
	session, err := wp.LoadSession(sessionID)
	if err != nil {
		return nil, errors.NewError().Message("failed to load session").Cause(err).WithLocation().Build()
	}

	checkpoints, err := wp.LoadCheckpoints(sessionID)
	if err != nil {
		return nil, errors.NewError().Message("failed to load checkpoints").Cause(err).WithLocation().Build()
	}

	var latestCheckpoint *execution.WorkflowCheckpoint
	for i := len(checkpoints) - 1; i >= 0; i-- {
		checkpoint := &checkpoints[i]
		if checkpoint.SessionID == sessionID {
			latestCheckpoint = checkpoint
			break
		}
	}

	var spec *execution.WorkflowSpec
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
func (wp *WorkflowPersistence) determineRecoveryStrategy(session *execution.ExecutionSession, checkpoint *execution.WorkflowCheckpoint) string {
	if checkpoint == nil {
		return "restart"
	}

	switch session.Status {
	case execution.WorkflowStatusPaused:
		return "resume"
	case execution.WorkflowStatusFailed:
		if len(session.FailedStages) > 0 {
			return "retry_failed"
		}
		return "resume"
	case execution.WorkflowStatusRunning:
		if time.Since(session.LastActivity) > 10*time.Minute {
			return "resume_stale"
		}
		return "wait"
	case execution.WorkflowStatusCompleted:
		return "completed"
	default:
		return "restart"
	}
}

// DeleteSession deletes a workflow session and its associated data
func (wp *WorkflowPersistence) DeleteSession(sessionID string) error {
	return wp.db.Update(func(tx *bolt.Tx) error {
		sessionBucket := tx.Bucket([]byte("workflow_sessions"))
		if err := sessionBucket.Delete([]byte(sessionID)); err != nil {
			return errors.NewError().Message("failed to delete session").Cause(err).WithLocation().Build()
		}

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
	Session          *execution.ExecutionSession   `json:"session"`
	LastCheckpoint   *execution.WorkflowCheckpoint `json:"last_checkpoint,omitempty"`
	WorkflowSpec     *execution.WorkflowSpec       `json:"workflow_spec,omitempty"`
	History          []WorkflowHistoryEvent        `json:"history,omitempty"`
	RecoveryTime     time.Time                     `json:"recovery_time"`
	RecoveryStrategy string                        `json:"recovery_strategy"`
}
