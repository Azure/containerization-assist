package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/errors"
	bolt "go.etcd.io/bbolt"
)

// WorkflowPersistence provides workflow state persistence and recovery
type WorkflowPersistence struct {
	db     *bolt.DB
	logger *slog.Logger
}

// NewWorkflowPersistence creates a new workflow persistence manager
func NewWorkflowPersistence(dbPath string, logger *slog.Logger) (*WorkflowPersistence, error) {
	db, err := bolt.Open(dbPath, 0600, &bolt.Options{
		Timeout: 1 * time.Second,
	})
	if err != nil {
		return nil, errors.NewError().
			Message("failed to open persistence database").
			Code(errors.CodeIOError).
			Type(errors.ErrTypeIO).
			Cause(err).
			Context("db_path", dbPath).
			WithLocation().
			Build()
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
				return errors.NewError().
					Messagef("failed to create bucket %s", bucket).
					Code(errors.CodeIOError).
					Type(errors.ErrTypeIO).
					Cause(err).
					Context("bucket_name", bucket).
					WithLocation().
					Build()
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
		logger: logger.With("component", "workflow_persistence"),
	}, nil
}

// SaveSession saves a workflow execution session
func (wp *WorkflowPersistence) SaveSession(session *ExecutionSession) error {
	return wp.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("workflow_sessions"))

		data, err := json.Marshal(session)
		if err != nil {
			return errors.NewError().
				Message("failed to marshal session").
				Code(errors.CodeInternalError).
				Type(errors.ErrTypeInternal).
				Cause(err).
				Context("session_id", session.ID).
				WithLocation().
				Build()
		}

		err = bucket.Put([]byte(session.ID), data)
		if err != nil {
			return errors.NewError().
				Message("failed to save session").
				Code(errors.CodeIOError).
				Type(errors.ErrTypeIO).
				Cause(err).
				Context("session_id", session.ID).
				WithLocation().
				Build()
		}

		wp.logger.Debug("Session saved to persistence",
			"session_id", session.ID,
			"workflow_id", session.WorkflowID,
			"status", session.Status)

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
			return errors.NewError().
				Messagef("session not found: %s", sessionID).
				Code(errors.CodeResourceNotFound).
				Type(errors.ErrTypeNotFound).
				Context("session_id", sessionID).
				WithLocation().
				Build()
		}

		err := json.Unmarshal(data, &session)
		if err != nil {
			return errors.NewError().
				Message("failed to unmarshal session").
				Code(errors.CodeInternalError).
				Type(errors.ErrTypeInternal).
				Cause(err).
				Context("session_id", sessionID).
				WithLocation().
				Build()
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	wp.logger.Debug("Session loaded from persistence",
		"session_id", sessionID,
		"workflow_id", session.WorkflowID,
		"status", session.Status)

	return &session, nil
}

// SaveCheckpoint saves a workflow checkpoint
func (wp *WorkflowPersistence) SaveCheckpoint(checkpoint *WorkflowCheckpoint) error {
	return wp.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("workflow_checkpoints"))

		data, err := json.Marshal(checkpoint)
		if err != nil {
			return errors.NewError().
				Message("failed to marshal checkpoint").
				Code(errors.CodeInternalError).
				Type(errors.ErrTypeInternal).
				Cause(err).
				Context("checkpoint_id", checkpoint.ID).
				Context("session_id", checkpoint.SessionID).
				WithLocation().
				Build()
		}

		key := fmt.Sprintf("%s_%s_%d",
			checkpoint.SessionID,
			checkpoint.StageID,
			checkpoint.Timestamp.UnixNano())

		err = bucket.Put([]byte(key), data)
		if err != nil {
			return errors.NewError().
				Message("failed to save checkpoint").
				Code(errors.CodeIOError).
				Type(errors.ErrTypeIO).
				Cause(err).
				Context("checkpoint_id", checkpoint.ID).
				Context("session_id", checkpoint.SessionID).
				Context("stage_id", checkpoint.StageID).
				WithLocation().
				Build()
		}

		wp.logger.Debug("Checkpoint saved to persistence",
			"checkpoint_id", checkpoint.ID,
			"session_id", checkpoint.SessionID,
			"stage_id", checkpoint.StageID)

		return nil
	})
}

// LoadCheckpoints loads all checkpoints for a session
func (wp *WorkflowPersistence) LoadCheckpoints(sessionID string) ([]WorkflowCheckpoint, error) {
	var checkpoints []WorkflowCheckpoint

	err := wp.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("workflow_checkpoints"))

		c := bucket.Cursor()
		prefix := []byte(sessionID + "_")

		for k, v := c.Seek(prefix); k != nil && len(k) >= len(prefix) && string(k[:len(prefix)]) == string(prefix); k, v = c.Next() {
			var checkpoint WorkflowCheckpoint

			err := json.Unmarshal(v, &checkpoint)
			if err != nil {
				wp.logger.Warn("Failed to unmarshal checkpoint",
					"error", err,
					"key", string(k))
				continue
			}

			checkpoints = append(checkpoints, checkpoint)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	wp.logger.Debug("Checkpoints loaded from persistence",
		"session_id", sessionID,
		"checkpoint_count", len(checkpoints))

	return checkpoints, nil
}

// SaveWorkflowSpec saves a workflow specification
func (wp *WorkflowPersistence) SaveWorkflowSpec(spec *WorkflowSpec) error {
	return wp.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("workflow_specs"))

		data, err := json.Marshal(spec)
		if err != nil {
			return errors.NewError().
				Message("failed to marshal workflow spec").
				Code(errors.CodeInternalError).
				Type(errors.ErrTypeInternal).
				Cause(err).
				Context("workflow_id", spec.ID).
				Context("workflow_name", spec.Name).
				WithLocation().
				Build()
		}

		err = bucket.Put([]byte(spec.ID), data)
		if err != nil {
			return errors.NewError().
				Message("failed to save workflow spec").
				Code(errors.CodeIOError).
				Type(errors.ErrTypeIO).
				Cause(err).
				Context("workflow_id", spec.ID).
				Context("workflow_name", spec.Name).
				WithLocation().
				Build()
		}

		wp.logger.Debug("Workflow spec saved to persistence",
			"workflow_id", spec.ID,
			"workflow_name", spec.Name,
			"version", spec.Version)

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
			return errors.NewError().
				Messagef("workflow spec not found: %s", workflowID).
				Code(errors.CodeResourceNotFound).
				Type(errors.ErrTypeNotFound).
				Context("workflow_id", workflowID).
				WithLocation().
				Build()
		}

		err := json.Unmarshal(data, &spec)
		if err != nil {
			return errors.NewError().
				Message("failed to unmarshal workflow spec").
				Code(errors.CodeInternalError).
				Type(errors.ErrTypeInternal).
				Cause(err).
				Context("workflow_id", workflowID).
				WithLocation().
				Build()
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	wp.logger.Debug("Workflow spec loaded from persistence",
		"workflow_id", workflowID,
		"workflow_name", spec.Name)

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
				wp.logger.Warn("Failed to unmarshal session",
					"error", err,
					"session_id", string(k))
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

	wp.logger.Debug("Sessions loaded from persistence",
		"session_count", len(sessions))

	return sessions, nil
}

// matchesFilter checks if a session matches the given filter
func (wp *WorkflowPersistence) matchesFilter(session *ExecutionSession, filter SessionFilter) bool {
	if filter.Status != "" && session.Status != filter.Status {
		return false
	}

	if filter.WorkflowID != "" && session.WorkflowID != filter.WorkflowID {
		return false
	}

	// Skip WorkflowName filtering as ExecutionSession doesn't have this field
	// if filter.WorkflowName != "" && session.WorkflowName != filter.WorkflowName {
	// 	return false
	// }

	if filter.StartTime != nil {
		if filter.StartAfter.After(session.StartTime) {
			return false
		}
		if filter.EndTime != nil && filter.EndTime.Before(session.StartTime) {
			return false
		}
	}

	// Skip Labels filtering as ExecutionSession doesn't have this field
	// if len(filter.Labels) > 0 {
	//	for key, value := range filter.Labels {
	//		if sessionValue, exists := session.Labels[key]; !exists || sessionValue != value {
	//			return false
	//		}
	//	}
	// }

	return true
}

// SaveWorkflowHistory saves workflow execution history
func (wp *WorkflowPersistence) SaveWorkflowHistory(sessionID string, event WorkflowHistoryEvent) error {
	return wp.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("workflow_history"))

		sessionBucket, err := bucket.CreateBucketIfNotExists([]byte(sessionID))
		if err != nil {
			return errors.NewError().
				Message("failed to create session bucket").
				Code(errors.CodeIOError).
				Type(errors.ErrTypeIO).
				Cause(err).
				Context("session_id", sessionID).
				WithLocation().
				Build()
		}

		data, err := json.Marshal(event)
		if err != nil {
			return errors.NewError().
				Message("failed to marshal history event").
				Code(errors.CodeInternalError).
				Type(errors.ErrTypeInternal).
				Cause(err).
				Context("event_id", event.ID).
				Context("session_id", sessionID).
				Context("event_type", event.EventType).
				WithLocation().
				Build()
		}

		key := fmt.Sprintf("%d_%s", event.Timestamp.UnixNano(), event.ID)

		err = sessionBucket.Put([]byte(key), data)
		if err != nil {
			return errors.NewError().
				Message("failed to save history event").
				Code(errors.CodeIOError).
				Type(errors.ErrTypeIO).
				Cause(err).
				Context("event_id", event.ID).
				Context("session_id", sessionID).
				WithLocation().
				Build()
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
				wp.logger.Warn("Failed to unmarshal history event",
					"error", err,
					"key", string(k))
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
		return nil, errors.NewError().
			Message("failed to load session for recovery").
			Code(errors.CodeResourceNotFound).
			Type(errors.ErrTypeNotFound).
			Cause(err).
			Context("session_id", sessionID).
			Context("operation", "RecoverSession").
			WithLocation().
			Build()
	}

	checkpoints, err := wp.LoadCheckpoints(sessionID)
	if err != nil {
		return nil, errors.NewError().
			Message("failed to load checkpoints for recovery").
			Code(errors.CodeIOError).
			Type(errors.ErrTypeIO).
			Cause(err).
			Context("session_id", sessionID).
			Context("operation", "RecoverSession").
			WithLocation().
			Build()
	}

	var latestCheckpoint *WorkflowCheckpoint
	for i := len(checkpoints) - 1; i >= 0; i-- {
		checkpoint := &checkpoints[i]
		if checkpoint.SessionID == sessionID {
			latestCheckpoint = checkpoint
			break
		}
	}

	var spec *WorkflowSpec
	// Note: WorkflowSpec field not available in WorkflowCheckpoint
	// Always load from spec storage
	if false { // Disabled condition
		// spec = latestCheckpoint.WorkflowSpec
	} else {
		spec, err = wp.LoadWorkflowSpec(session.WorkflowID)
		if err != nil {
			wp.logger.Warn("Failed to load workflow spec for recovery",
				"error", err,
				"workflow_id", session.WorkflowID)
		}
	}

	history, err := wp.LoadWorkflowHistory(sessionID)
	if err != nil {
		wp.logger.Warn("Failed to load workflow history",
			"error", err,
			"session_id", sessionID)
	}

	recovered := &RecoveredSession{
		Session:          session,
		LastCheckpoint:   latestCheckpoint,
		WorkflowSpec:     spec,
		History:          history,
		RecoveryTime:     time.Now(),
		RecoveryStrategy: wp.determineRecoveryStrategy(session, latestCheckpoint),
	}

	wp.logger.Info("Session recovered from persistence",
		"session_id", sessionID,
		"status", session.Status,
		"recovery_strategy", recovered.RecoveryStrategy,
		"has_checkpoint", latestCheckpoint != nil)

	return recovered, nil
}

// determineRecoveryStrategy determines the best recovery strategy
func (wp *WorkflowPersistence) determineRecoveryStrategy(session *ExecutionSession, checkpoint *WorkflowCheckpoint) string {
	if checkpoint == nil {
		return "restart"
	}

	switch session.Status {
	case WorkflowStatusPaused:
		return "resume"
	case WorkflowStatusFailed:
		// Note: FailedStages field not available in ExecutionSession
		return "retry_failed"
	case WorkflowStatusRunning:
		// Note: LastActivity field not available in ExecutionSession
		// Use a simple time check based on start time
		if time.Since(session.StartTime) > 10*time.Minute {
			return "resume_stale"
		}
		return "wait"
	case WorkflowStatusCompleted:
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
			return errors.NewError().
				Message("failed to delete session").
				Code(errors.CodeIOError).
				Type(errors.ErrTypeIO).
				Cause(err).
				Context("session_id", sessionID).
				Context("operation", "DeleteSession").
				WithLocation().
				Build()
		}

		checkpointBucket := tx.Bucket([]byte("workflow_checkpoints"))
		c := checkpointBucket.Cursor()
		prefix := []byte(sessionID + "_")

		for k, _ := c.Seek(prefix); k != nil && len(k) >= len(prefix) && string(k[:len(prefix)]) == string(prefix); k, _ = c.Next() {
			if err := checkpointBucket.Delete(k); err != nil {
				wp.logger.Warn("Failed to delete checkpoint",
					"error", err,
					"key", string(k))
			}
		}

		historyBucket := tx.Bucket([]byte("workflow_history"))
		if err := historyBucket.DeleteBucket([]byte(sessionID)); err != nil && err != bolt.ErrBucketNotFound {
			wp.logger.Warn("Failed to delete history bucket",
				"error", err,
				"session_id", sessionID)
		}

		wp.logger.Info("Session and associated data deleted from persistence",
			"session_id", sessionID)

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
