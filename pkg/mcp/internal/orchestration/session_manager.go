package orchestration

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/orchestration/workflow"
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
func (sm *BoltWorkflowSessionManager) CreateSession(workflowSpec *workflow.WorkflowSpec) (*workflow.WorkflowSession, error) {
	sessionID := uuid.New().String()
	workflowID := fmt.Sprintf("%s_%s_%d", workflowSpec.Metadata.Name, workflowSpec.Metadata.Version, time.Now().Unix())

	session := &workflow.WorkflowSession{
		ID:               sessionID,
		WorkflowID:       workflowID,
		WorkflowName:     workflowSpec.Metadata.Name,
		WorkflowVersion:  workflowSpec.Metadata.Version,
		Labels:           make(map[string]string),
		Status:           workflow.WorkflowStatusPending,
		CurrentStage:     "",
		CompletedStages:  []string{},
		FailedStages:     []string{},
		SkippedStages:    []string{},
		StageResults:     make(map[string]interface{}),
		SharedContext:    make(map[string]interface{}),
		Checkpoints:      []workflow.WorkflowCheckpoint{},
		ResourceBindings: make(map[string]string),
		StartTime:        time.Now(),
		LastActivity:     time.Now(),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	// Initialize labels from workflow metadata
	if workflowSpec.Metadata.Labels != nil {
		for key, value := range workflowSpec.Metadata.Labels {
			session.Labels[key] = value
		}
	}

	// Initialize shared context with workflow variables
	if workflowSpec.Spec.Variables != nil {
		for key, value := range workflowSpec.Spec.Variables {
			session.SharedContext[key] = value
		}
	}

	// Store session in database
	err := sm.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(workflowSessionsBucket))
		if err != nil {
			return fmt.Errorf("failed to create sessions bucket: %w", err)
		}

		sessionData, err := json.Marshal(session)
		if err != nil {
			return fmt.Errorf("failed to marshal session: %w", err)
		}

		return bucket.Put([]byte(sessionID), sessionData)
	})

	if err != nil {
		return nil, fmt.Errorf("failed to store session: %w", err)
	}

	sm.logger.Info().
		Str("session_id", sessionID).
		Str("workflow_id", workflowID).
		Str("workflow_name", workflowSpec.Metadata.Name).
		Msg("Created new workflow session")

	return session, nil
}

// GetSession retrieves a workflow session by ID
func (sm *BoltWorkflowSessionManager) GetSession(sessionID string) (*workflow.WorkflowSession, error) {
	var session *workflow.WorkflowSession

	err := sm.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(workflowSessionsBucket))
		if bucket == nil {
			return fmt.Errorf("sessions bucket not found")
		}

		sessionData := bucket.Get([]byte(sessionID))
		if sessionData == nil {
			return fmt.Errorf("session not found: %s", sessionID)
		}

		session = &workflow.WorkflowSession{}
		return json.Unmarshal(sessionData, session)
	})

	if err != nil {
		return nil, err
	}

	return session, nil
}

// UpdateSession updates an existing workflow session
func (sm *BoltWorkflowSessionManager) UpdateSession(session *workflow.WorkflowSession) error {
	session.UpdatedAt = time.Now()

	err := sm.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(workflowSessionsBucket))
		if bucket == nil {
			return fmt.Errorf("sessions bucket not found")
		}

		sessionData, err := json.Marshal(session)
		if err != nil {
			return fmt.Errorf("failed to marshal session: %w", err)
		}

		return bucket.Put([]byte(session.ID), sessionData)
	})

	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
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
			return fmt.Errorf("sessions bucket not found")
		}

		return bucket.Delete([]byte(sessionID))
	})

	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	sm.logger.Info().
		Str("session_id", sessionID).
		Msg("Deleted workflow session")

	return nil
}

// ListSessions returns a list of workflow sessions matching the filter
func (sm *BoltWorkflowSessionManager) ListSessions(filter workflow.SessionFilter) ([]*workflow.WorkflowSession, error) {
	var sessions []*workflow.WorkflowSession

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

			var session workflow.WorkflowSession
			if err := json.Unmarshal(value, &session); err != nil {
				sm.logger.Warn().
					Err(err).
					Str("session_id", string(key)).
					Msg("Failed to unmarshal session, skipping")
				continue
			}

			// Apply filters
			if filter.WorkflowName != "" && session.WorkflowName != filter.WorkflowName {
				continue
			}
			if filter.Status != "" && session.Status != filter.Status {
				continue
			}
			if filter.StartTime != nil && session.StartTime.Before(*filter.StartTime) {
				continue
			}
			if filter.EndTime != nil && (session.EndTime == nil || session.EndTime.After(*filter.EndTime)) {
				continue
			}

			// Check label filters
			if len(filter.Labels) > 0 {
				// Check if session labels match filter labels
				if !sm.labelsMatch(session.Labels, filter.Labels) {
					continue
				}
			}

			sessions = append(sessions, &session)
			count++
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	sm.logger.Debug().
		Int("total_sessions", len(sessions)).
		Str("workflow_name", filter.WorkflowName).
		Str("status", string(filter.Status)).
		Msg("Listed workflow sessions")

	return sessions, nil
}

// GetSessionsByWorkflow returns all sessions for a specific workflow
func (sm *BoltWorkflowSessionManager) GetSessionsByWorkflow(workflowName string) ([]*workflow.WorkflowSession, error) {
	return sm.ListSessions(workflow.SessionFilter{
		WorkflowName: workflowName,
	})
}

// GetActiveSession returns active sessions (running, paused)
func (sm *BoltWorkflowSessionManager) GetActiveSessions() ([]*workflow.WorkflowSession, error) {
	allSessions, err := sm.ListSessions(workflow.SessionFilter{})
	if err != nil {
		return nil, err
	}

	var activeSessions []*workflow.WorkflowSession
	for _, session := range allSessions {
		if session.Status == workflow.WorkflowStatusRunning || session.Status == workflow.WorkflowStatusPaused {
			activeSessions = append(activeSessions, session)
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
			var session workflow.WorkflowSession
			if err := json.Unmarshal(value, &session); err != nil {
				continue
			}

			// Check if session is expired (completed or failed and older than maxAge)
			if (session.Status == workflow.WorkflowStatusCompleted || session.Status == workflow.WorkflowStatusFailed || session.Status == workflow.WorkflowStatusCancelled) &&
				session.UpdatedAt.Before(cutoffTime) {
				expiredSessions = append(expiredSessions, session.ID)
			}
		}

		return nil
	})

	if err != nil {
		return 0, fmt.Errorf("failed to find expired sessions: %w", err)
	}

	// Delete expired sessions
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
		StatusCounts:     make(map[workflow.WorkflowStatus]int),
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
			var session workflow.WorkflowSession
			if err := json.Unmarshal(value, &session); err != nil {
				continue
			}

			metrics.TotalSessions++
			metrics.StatusCounts[session.Status]++
			metrics.WorkflowCounts[session.WorkflowName]++

			if session.EndTime != nil {
				duration := session.EndTime.Sub(session.StartTime)
				workflowDurations[session.WorkflowName] = append(workflowDurations[session.WorkflowName], duration)
			}

			if session.StartTime.After(metrics.LastActivity) {
				metrics.LastActivity = session.StartTime
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
		return nil, fmt.Errorf("failed to get session metrics: %w", err)
	}

	return metrics, nil
}

// SessionMetrics contains metrics about workflow sessions
type SessionMetrics struct {
	TotalSessions    int                               `json:"total_sessions"`
	StatusCounts     map[workflow.WorkflowStatus]int   `json:"status_counts"`
	WorkflowCounts   map[string]int                    `json:"workflow_counts"`
	AverageDurations map[string]time.Duration          `json:"average_durations"`
	LastActivity     time.Time                         `json:"last_activity"`
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
