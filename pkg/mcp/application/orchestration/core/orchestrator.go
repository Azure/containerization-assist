package core

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/orchestration/execution"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"go.etcd.io/bbolt"
)

// BoltWorkflowSessionManager implements WorkflowSessionManager using BoltDB
type BoltWorkflowSessionManager struct {
	db     *bbolt.DB
	logger zerolog.Logger
}

// NewBoltWorkflowSessionManager creates a new BoltDB-backed workflow session manager
func NewBoltWorkflowSessionManager(db *bbolt.DB, logger zerolog.Logger) *BoltWorkflowSessionManager {
	return &BoltWorkflowSessionManager{
		db:     db,
		logger: logger.With().Str("component", "workflow_session_manager").Logger(),
	}
}

const (
	workflowSessionsBucket = "workflow_sessions"
)

// CreateSession creates a new workflow session
func (sm *BoltWorkflowSessionManager) CreateSession(workflowSpec *session.WorkflowSpec) (*session.WorkflowSession, error) {
	sessionID := uuid.New().String()
	workflowID := fmt.Sprintf("%s_%s_%d", workflowSpec.Metadata.Name, workflowSpec.Metadata.Version, time.Now().Unix())

	// Create the embedded SessionState first
	sessionState := &session.SessionState{
		ID:           sessionID,
		SessionID:    sessionID,
		WorkspaceDir: "/tmp/workflows/" + sessionID,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		LastAccessed: time.Now(),
		Status:       "active",
		Labels:       []string{},
		K8sLabels:    make(map[string]string),
	}

	workflowSession := &session.WorkflowSession{
		SessionState:    sessionState,
		WorkflowID:      workflowID,
		WorkflowName:    workflowSpec.Metadata.Name,
		WorkflowVersion: workflowSpec.Metadata.Version,
		Status:          session.WorkflowStatusPending,
		CurrentStage:    "",
		CompletedStages: []string{},
		FailedStages:    []string{},
		SkippedStages:   []string{},
		StageResults:    make(map[string]interface{}),
		SharedContext:   make(map[string]interface{}),
		Checkpoints:     []session.WorkflowCheckpoint{},
		// Initialize typed alternatives
		TypedVariables: &session.WorkflowVariables{
			BuildArgs:      make(map[string]string),
			Environment:    make(map[string]string),
			ConfigValues:   make(map[string]string),
			UserParameters: make(map[string]string),
		},
		TypedContext: &session.WorkflowContext{
			SessionID:  sessionID,
			Properties: make(map[string]string),
		},
		TypedSharedContext: &session.SharedWorkflowData{
			CustomData: make(map[string]string),
		},
		TypedResourceBindings: &session.ResourceBindings{
			ImageBindings:     make(map[string]string),
			NamespaceBindings: make(map[string]string),
			ConfigMapBindings: make(map[string]string),
			SecretBindings:    make(map[string]string),
			VolumeBindings:    make(map[string]string),
		},
		TypedStageResults: &session.StageResults{
			Results: make(map[string]*session.TypedStageResult),
		},
	}

	// Initialize K8s labels from workflow metadata
	if workflowSpec.Metadata.Labels != nil {
		for key, value := range workflowSpec.Metadata.Labels {
			sessionState.K8sLabels[key] = value
		}
	}

	// Initialize shared context with workflow variables
	if workflowSpec.Spec.Variables != nil {
		for key, value := range workflowSpec.Spec.Variables {
			workflowSession.SharedContext[key] = value
		}
	}

	// Store session in database
	err := sm.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(workflowSessionsBucket))
		if err != nil {
			return errors.NewError().Message("failed to create sessions bucket").Cause(err).Build()
		}

		sessionData, err := json.Marshal(workflowSession)
		if err != nil {
			return errors.NewError().Message("failed to marshal session").Cause(err).Build()
		}

		return bucket.Put([]byte(sessionID), sessionData)
	})

	if err != nil {
		return nil, errors.NewError().Message("failed to store session").Cause(err).Build()
	}

	sm.logger.Info().
		Str("session_id", sessionID).
		Str("workflow_id", workflowID).
		Str("workflow_name", workflowSpec.Metadata.Name).
		Msg("Created new workflow session")

	return workflowSession, nil
}

// GetSession retrieves a workflow session by ID
func (sm *BoltWorkflowSessionManager) GetSession(sessionID string) (*session.WorkflowSession, error) {
	var workflowSession *session.WorkflowSession

	err := sm.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(workflowSessionsBucket))
		if bucket == nil {
			return errors.NewError().Messagef("sessions bucket not found").Build()
		}

		sessionData := bucket.Get([]byte(sessionID))
		if sessionData == nil {
			return errors.NewError().Messagef("session not found: %s", sessionID).Build()
		}

		workflowSession = &session.WorkflowSession{}
		return json.Unmarshal(sessionData, workflowSession)
	})

	if err != nil {
		return nil, err
	}

	return workflowSession, nil
}

// UpdateSession updates an existing workflow session
func (sm *BoltWorkflowSessionManager) UpdateSession(session *session.WorkflowSession) error {
	session.SessionState.UpdatedAt = time.Now()

	err := sm.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(workflowSessionsBucket))
		if bucket == nil {
			return errors.NewError().Messagef("sessions bucket not found").Build()
		}

		sessionData, err := json.Marshal(session)
		if err != nil {
			return errors.NewError().Message("failed to marshal session").Cause(err).Build()
		}

		return bucket.Put([]byte(session.SessionState.ID), sessionData)
	})

	if err != nil {
		return errors.NewError().Message("failed to update session").Cause(err).Build()
	}

	sm.logger.Debug().
		Str("session_id", session.ID).
		Str("status", string(session.Status)).
		Str("current_stage", session.CurrentStage).
		Msg("Updated workflow session")

	return nil
}

// DeleteSession deletes a workflow session
func (sm *BoltWorkflowSessionManager) DeleteSession(sessionID string) error {
	err := sm.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(workflowSessionsBucket))
		if bucket == nil {
			return errors.NewError().Messagef("sessions bucket not found").Build()
		}

		return bucket.Delete([]byte(sessionID))
	})

	if err != nil {
		return errors.NewError().Message("failed to delete session").Cause(err).Build()
	}

	sm.logger.Info().
		Str("session_id", sessionID).
		Msg("Deleted workflow session")

	return nil
}

// ListSessions returns a list of workflow sessions matching the filter
func (sm *BoltWorkflowSessionManager) ListSessions(filter execution.SessionFilter) ([]*session.WorkflowSession, error) {
	var sessions []*session.WorkflowSession

	err := sm.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(workflowSessionsBucket))
		if bucket == nil {
			// No sessions exist yet
			return nil
		}

		cursor := bucket.Cursor()
		count := 0
		skipped := 0

		for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
			// Apply offset
			if filter.Offset > 0 && skipped < filter.Offset {
				skipped++
				continue
			}

			// Apply limit
			if filter.Limit > 0 && count >= filter.Limit {
				break
			}

			var workflowSession session.WorkflowSession
			if err := json.Unmarshal(value, &workflowSession); err != nil {
				sm.logger.Warn().
					Err(err).
					Str("session_id", string(key)).
					Msg("Failed to unmarshal session, skipping")
				continue
			}

			// Apply filters
			if filter.WorkflowName != "" && workflowSession.WorkflowName != filter.WorkflowName {
				continue
			}
			if filter.Status != "" && string(workflowSession.Status) != filter.Status {
				continue
			}
			// TODO: Fix StartTime and EndTime references - these fields don't exist in WorkflowSession
			// Skipping time filters for now as they need to be mapped to SessionState fields

			// Check label filters
			if len(filter.Labels) > 0 {
				// Check if session K8s labels match filter labels
				if !sm.labelsMatch(workflowSession.SessionState.K8sLabels, filter.Labels) {
					continue
				}
			}

			sessions = append(sessions, &workflowSession)
			count++
		}

		return nil
	})

	if err != nil {
		return nil, errors.NewError().Message("failed to list sessions").Cause(err).Build()
	}

	sm.logger.Debug().
		Int("total_sessions", len(sessions)).
		Str("workflow_name", filter.WorkflowName).
		Str("status", filter.Status).
		Msg("Listed workflow sessions")

	return sessions, nil
}

// GetSessionsByWorkflow returns all sessions for a specific workflow
func (sm *BoltWorkflowSessionManager) GetSessionsByWorkflow(workflowName string) ([]*session.WorkflowSession, error) {
	return sm.ListSessions(execution.SessionFilter{
		WorkflowName: workflowName,
	})
}

// GetActiveSession returns active sessions (running, paused)
func (sm *BoltWorkflowSessionManager) GetActiveSessions() ([]*session.WorkflowSession, error) {
	allSessions, err := sm.ListSessions(execution.SessionFilter{})
	if err != nil {
		return nil, err
	}

	var activeSessions []*session.WorkflowSession
	for _, workflowSession := range allSessions {
		if workflowSession.Status == session.WorkflowStatusRunning || workflowSession.Status == session.WorkflowStatusPaused {
			activeSessions = append(activeSessions, workflowSession)
		}
	}

	return activeSessions, nil
}

// CleanupExpiredSessions removes sessions older than the specified duration
func (sm *BoltWorkflowSessionManager) CleanupExpiredSessions(maxAge time.Duration) (int, error) {
	cutoffTime := time.Now().Add(-maxAge)
	var expiredSessions []string

	// Find expired sessions
	err := sm.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(workflowSessionsBucket))
		if bucket == nil {
			return nil
		}

		cursor := bucket.Cursor()
		for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
			var workflowSession session.WorkflowSession
			if err := json.Unmarshal(value, &workflowSession); err != nil {
				continue
			}

			// Check if session is expired (completed or failed and older than maxAge)
			if (workflowSession.Status == session.WorkflowStatusCompleted || workflowSession.Status == session.WorkflowStatusFailed || workflowSession.Status == session.WorkflowStatusCancelled) &&
				workflowSession.SessionState.UpdatedAt.Before(cutoffTime) {
				expiredSessions = append(expiredSessions, workflowSession.SessionState.ID)
			}
		}

		return nil
	})

	if err != nil {
		return 0, errors.NewError().Message("failed to find expired sessions").Cause(err).WithLocation(

		// Delete expired sessions
		).Build()
	}

	deletedCount := 0
	for _, sessionID := range expiredSessions {
		if err := sm.DeleteSession(sessionID); err != nil {
			sm.logger.Warn().
				Err(err).
				Str("session_id", sessionID).
				Msg("Failed to delete expired session")
		} else {
			deletedCount++
		}
	}

	sm.logger.Info().
		Int("deleted_count", deletedCount).
		Dur("max_age", maxAge).
		Msg("Cleaned up expired workflow sessions")

	return deletedCount, nil
}

// GetSessionMetrics returns metrics about workflow sessions
func (sm *BoltWorkflowSessionManager) GetSessionMetrics() (*SessionMetrics, error) {
	metrics := &SessionMetrics{
		StatusCounts:     make(map[session.WorkflowStatus]int),
		WorkflowCounts:   make(map[string]int),
		AverageDurations: make(map[string]time.Duration),
	}

	err := sm.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(workflowSessionsBucket))
		if bucket == nil {
			return nil
		}

		cursor := bucket.Cursor()
		workflowDurations := make(map[string][]time.Duration)

		for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
			var workflowSession session.WorkflowSession
			if err := json.Unmarshal(value, &workflowSession); err != nil {
				continue
			}

			metrics.TotalSessions++
			metrics.StatusCounts[workflowSession.Status]++
			metrics.WorkflowCounts[workflowSession.WorkflowName]++

			// TODO: Fix timing metrics - EndTime and StartTime fields don't exist in current WorkflowSession
			// Need to map these to SessionState fields or add them to WorkflowSession
			// if workflowSession.EndTime != nil {
			//     duration := workflowSession.EndTime.Sub(workflowSession.StartTime)
			//     workflowDurations[workflowSession.WorkflowName] = append(workflowDurations[workflowSession.WorkflowName], duration)
			// }

			if workflowSession.SessionState.UpdatedAt.After(metrics.LastActivity) {
				metrics.LastActivity = workflowSession.SessionState.UpdatedAt
			}
		}

		// Calculate average durations
		for workflowName, durations := range workflowDurations {
			if len(durations) > 0 {
				var total time.Duration
				for _, d := range durations {
					total += d
				}
				metrics.AverageDurations[workflowName] = total / time.Duration(len(durations))
			}
		}

		return nil
	})

	if err != nil {
		return nil, errors.NewError().Message("failed to get session metrics").Cause(err).Build()
	}

	return metrics, nil
}

// SessionMetrics contains metrics about workflow sessions
type SessionMetrics struct {
	TotalSessions    int                            `json:"total_sessions"`
	StatusCounts     map[session.WorkflowStatus]int `json:"status_counts"`
	WorkflowCounts   map[string]int                 `json:"workflow_counts"`
	AverageDurations map[string]time.Duration       `json:"average_durations"`
	LastActivity     time.Time                      `json:"last_activity"`
}

// labelsMatch checks if session labels match the filter labels
// Returns true if all filter labels are present in session labels with matching values
func (sm *BoltWorkflowSessionManager) labelsMatch(sessionLabels, filterLabels map[string]string) bool {
	if len(filterLabels) == 0 {
		return true
	}

	if len(sessionLabels) == 0 {
		return false
	}

	for key, value := range filterLabels {
		if sessionValue, exists := sessionLabels[key]; !exists || sessionValue != value {
			return false
		}
	}

	return true
}
