package orchestration

import (
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// StateMachine manages workflow state transitions and validation
type StateMachine struct {
	logger         zerolog.Logger
	sessionManager WorkflowSessionManager
	mu             sync.RWMutex
	transitions    map[WorkflowStatus][]WorkflowStatus
}

// NewStateMachine creates a new workflow state machine
func NewStateMachine(logger zerolog.Logger, sessionManager WorkflowSessionManager) *StateMachine {
	return &StateMachine{
		logger:         logger.With().Str("component", "workflow_state_machine").Logger(),
		sessionManager: sessionManager,
		transitions: map[WorkflowStatus][]WorkflowStatus{
			WorkflowStatusPending: {
				WorkflowStatusRunning,
				WorkflowStatusCancelled,
			},
			WorkflowStatusRunning: {
				WorkflowStatusCompleted,
				WorkflowStatusFailed,
				WorkflowStatusPaused,
				WorkflowStatusCancelled,
			},
			WorkflowStatusPaused: {
				WorkflowStatusRunning,
				WorkflowStatusCancelled,
			},
			WorkflowStatusCompleted: {},
			WorkflowStatusFailed:    {},
			WorkflowStatusCancelled: {},
		},
	}
}

// TransitionState transitions a workflow session to a new state
func (sm *StateMachine) TransitionState(session *WorkflowSession, newStatus WorkflowStatus) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Validate transition
	if !sm.isValidTransition(session.Status, newStatus) {
		return fmt.Errorf("invalid state transition from %s to %s", session.Status, newStatus)
	}

	oldStatus := session.Status
	session.Status = newStatus
	session.LastActivity = time.Now()
	session.UpdatedAt = time.Now()

	// Update end time for terminal states
	if sm.isTerminalState(newStatus) {
		now := time.Now()
		session.EndTime = &now
		session.Duration = now.Sub(session.StartTime)
	}

	// Persist state change
	if err := sm.sessionManager.UpdateSession(session); err != nil {
		// Rollback state on persistence failure
		session.Status = oldStatus
		return fmt.Errorf("failed to persist state transition: %w", err)
	}

	sm.logger.Info().
		Str("session_id", session.ID).
		Str("old_status", string(oldStatus)).
		Str("new_status", string(newStatus)).
		Msg("Workflow state transitioned")

	return nil
}

// ValidateStateTransition checks if a state transition is valid without applying it
func (sm *StateMachine) ValidateStateTransition(fromStatus, toStatus WorkflowStatus) error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if !sm.isValidTransition(fromStatus, toStatus) {
		return fmt.Errorf("invalid state transition from %s to %s", fromStatus, toStatus)
	}
	return nil
}

// GetAllowedTransitions returns the allowed transitions from a given state
func (sm *StateMachine) GetAllowedTransitions(status WorkflowStatus) []WorkflowStatus {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	transitions, ok := sm.transitions[status]
	if !ok {
		return []WorkflowStatus{}
	}

	// Return a copy to prevent modification
	result := make([]WorkflowStatus, len(transitions))
	copy(result, transitions)
	return result
}

// UpdateStageStatus updates the status of a specific stage in the session
func (sm *StateMachine) UpdateStageStatus(session *WorkflowSession, stageName string, status StageStatus) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session.LastActivity = time.Now()
	session.UpdatedAt = time.Now()

	switch status {
	case StageStatusCompleted:
		// Remove from failed/skipped if present
		session.FailedStages = removeFromSlice(session.FailedStages, stageName)
		session.SkippedStages = removeFromSlice(session.SkippedStages, stageName)

		// Add to completed if not already present
		if !contains(session.CompletedStages, stageName) {
			session.CompletedStages = append(session.CompletedStages, stageName)
		}

	case StageStatusFailed:
		// Remove from completed/skipped if present
		session.CompletedStages = removeFromSlice(session.CompletedStages, stageName)
		session.SkippedStages = removeFromSlice(session.SkippedStages, stageName)

		// Add to failed if not already present
		if !contains(session.FailedStages, stageName) {
			session.FailedStages = append(session.FailedStages, stageName)
		}

	case StageStatusSkipped:
		// Remove from completed/failed if present
		session.CompletedStages = removeFromSlice(session.CompletedStages, stageName)
		session.FailedStages = removeFromSlice(session.FailedStages, stageName)

		// Add to skipped if not already present
		if !contains(session.SkippedStages, stageName) {
			session.SkippedStages = append(session.SkippedStages, stageName)
		}
	}

	// Persist state change
	if err := sm.sessionManager.UpdateSession(session); err != nil {
		return fmt.Errorf("failed to update stage status: %w", err)
	}

	sm.logger.Debug().
		Str("session_id", session.ID).
		Str("stage_name", stageName).
		Str("stage_status", string(status)).
		Msg("Stage status updated")

	return nil
}

// IsTerminalState checks if a workflow status is terminal
func (sm *StateMachine) IsTerminalState(status WorkflowStatus) bool {
	return sm.isTerminalState(status)
}

// Internal helper methods

func (sm *StateMachine) isValidTransition(from, to WorkflowStatus) bool {
	allowed, ok := sm.transitions[from]
	if !ok {
		return false
	}

	for _, status := range allowed {
		if status == to {
			return true
		}
	}
	return false
}

func (sm *StateMachine) isTerminalState(status WorkflowStatus) bool {
	return status == WorkflowStatusCompleted ||
		status == WorkflowStatusFailed ||
		status == WorkflowStatusCancelled
}

// StageStatus represents the status of a workflow stage
type StageStatus string

const (
	StageStatusPending   StageStatus = "pending"
	StageStatusRunning   StageStatus = "running"
	StageStatusCompleted StageStatus = "completed"
	StageStatusFailed    StageStatus = "failed"
	StageStatusSkipped   StageStatus = "skipped"
)

// Helper functions

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func removeFromSlice(slice []string, item string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}
