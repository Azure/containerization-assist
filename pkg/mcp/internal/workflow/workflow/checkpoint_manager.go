package workflow

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// DefaultCheckpointManager implements CheckpointManager interface
type DefaultCheckpointManager struct {
	logger         zerolog.Logger
	sessionManager WorkflowSessionManager
	checkpoints    map[string]*WorkflowCheckpoint // checkpointID -> checkpoint
	sessionIndex   map[string][]string            // sessionID -> checkpointIDs
	mu             sync.RWMutex
}

// NewCheckpointManager creates a new checkpoint manager
func NewCheckpointManager(logger zerolog.Logger, sessionManager WorkflowSessionManager) CheckpointManager {
	return &DefaultCheckpointManager{
		logger:         logger.With().Str("component", "checkpoint_manager").Logger(),
		sessionManager: sessionManager,
		checkpoints:    make(map[string]*WorkflowCheckpoint),
		sessionIndex:   make(map[string][]string),
	}
}

// CreateCheckpoint creates a new checkpoint for the workflow session
func (cm *DefaultCheckpointManager) CreateCheckpoint(
	session *WorkflowSession,
	stageName string,
	message string,
	workflowSpec *WorkflowSpec,
) (*WorkflowCheckpoint, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	checkpointID := uuid.New().String()
	checkpoint := &WorkflowCheckpoint{
		ID:        checkpointID,
		StageName: stageName,
		Timestamp: time.Now(),
		Message:   message,
		SessionState: map[string]interface{}{
			"id":                session.ID,
			"workflow_id":       session.WorkflowID,
			"workflow_name":     session.WorkflowName,
			"workflow_version":  session.WorkflowVersion,
			"status":            session.Status,
			"current_stage":     session.CurrentStage,
			"completed_stages":  session.CompletedStages,
			"failed_stages":     session.FailedStages,
			"skipped_stages":    session.SkippedStages,
			"shared_context":    session.SharedContext,
			"resource_bindings": session.ResourceBindings,
			"start_time":        session.StartTime,
			"last_activity":     session.LastActivity,
			"execution_options": session.ExecutionOptions,
		},
		StageResults: session.StageResults,
		WorkflowSpec: workflowSpec,
	}

	// Store checkpoint
	cm.checkpoints[checkpointID] = checkpoint

	// Update session index
	if _, exists := cm.sessionIndex[session.ID]; !exists {
		cm.sessionIndex[session.ID] = []string{}
	}
	cm.sessionIndex[session.ID] = append(cm.sessionIndex[session.ID], checkpointID)

	// Add checkpoint to session
	session.Checkpoints = append(session.Checkpoints, *checkpoint)

	// Persist updated session
	if err := cm.sessionManager.UpdateSession(session); err != nil {
		// Rollback checkpoint creation
		delete(cm.checkpoints, checkpointID)
		cm.removeFromIndex(session.ID, checkpointID)
		return nil, fmt.Errorf("failed to persist checkpoint: %w", err)
	}

	cm.logger.Info().
		Str("checkpoint_id", checkpointID).
		Str("session_id", session.ID).
		Str("stage_name", stageName).
		Msg("Created workflow checkpoint")

	return checkpoint, nil
}

// RestoreFromCheckpoint restores a workflow session from a checkpoint
func (cm *DefaultCheckpointManager) RestoreFromCheckpoint(sessionID, checkpointID string) (*WorkflowSession, error) {
	cm.mu.RLock()
	checkpoint, exists := cm.checkpoints[checkpointID]
	cm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("checkpoint %s not found", checkpointID)
	}

	// Verify checkpoint belongs to the session
	if !cm.checkpointBelongsToSession(sessionID, checkpointID) {
		return nil, fmt.Errorf("checkpoint %s does not belong to session %s", checkpointID, sessionID)
	}

	// Restore session from checkpoint state
	session := &WorkflowSession{
		ID:               checkpoint.SessionState["id"].(string),
		WorkflowID:       checkpoint.SessionState["workflow_id"].(string),
		WorkflowName:     checkpoint.SessionState["workflow_name"].(string),
		WorkflowVersion:  checkpoint.SessionState["workflow_version"].(string),
		Status:           checkpoint.SessionState["status"].(WorkflowStatus),
		CurrentStage:     checkpoint.SessionState["current_stage"].(string),
		CompletedStages:  checkpoint.SessionState["completed_stages"].([]string),
		FailedStages:     checkpoint.SessionState["failed_stages"].([]string),
		SkippedStages:    checkpoint.SessionState["skipped_stages"].([]string),
		SharedContext:    checkpoint.SessionState["shared_context"].(map[string]interface{}),
		ResourceBindings: checkpoint.SessionState["resource_bindings"].(map[string]string),
		StartTime:        checkpoint.SessionState["start_time"].(time.Time),
		LastActivity:     checkpoint.SessionState["last_activity"].(time.Time),
		StageResults:     checkpoint.StageResults,
		Checkpoints:      []WorkflowCheckpoint{*checkpoint},
	}

	// Restore execution options if available
	if execOpts, ok := checkpoint.SessionState["execution_options"]; ok && execOpts != nil {
		session.ExecutionOptions = execOpts.(*ExecutionOptions)
	}

	// Update timestamps
	session.LastActivity = time.Now()
	session.UpdatedAt = time.Now()

	// Reset status to running for resume
	session.Status = WorkflowStatusPaused

	cm.logger.Info().
		Str("checkpoint_id", checkpointID).
		Str("session_id", sessionID).
		Str("stage_name", checkpoint.StageName).
		Msg("Restored session from checkpoint")

	return session, nil
}

// ListCheckpoints returns all checkpoints for a session
func (cm *DefaultCheckpointManager) ListCheckpoints(sessionID string) ([]*WorkflowCheckpoint, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	checkpointIDs, exists := cm.sessionIndex[sessionID]
	if !exists {
		return []*WorkflowCheckpoint{}, nil
	}

	checkpoints := make([]*WorkflowCheckpoint, 0, len(checkpointIDs))
	for _, id := range checkpointIDs {
		if checkpoint, exists := cm.checkpoints[id]; exists {
			checkpoints = append(checkpoints, checkpoint)
		}
	}

	return checkpoints, nil
}

// DeleteCheckpoint removes a checkpoint
func (cm *DefaultCheckpointManager) DeleteCheckpoint(checkpointID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	checkpoint, exists := cm.checkpoints[checkpointID]
	if !exists {
		return fmt.Errorf("checkpoint %s not found", checkpointID)
	}

	// Find session ID from checkpoint
	sessionID := checkpoint.SessionState["id"].(string)

	// Remove from storage
	delete(cm.checkpoints, checkpointID)
	cm.removeFromIndex(sessionID, checkpointID)

	cm.logger.Info().
		Str("checkpoint_id", checkpointID).
		Str("session_id", sessionID).
		Msg("Deleted checkpoint")

	return nil
}

// GetCheckpoint retrieves a specific checkpoint
func (cm *DefaultCheckpointManager) GetCheckpoint(checkpointID string) (*WorkflowCheckpoint, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	checkpoint, exists := cm.checkpoints[checkpointID]
	if !exists {
		return nil, fmt.Errorf("checkpoint %s not found", checkpointID)
	}

	return checkpoint, nil
}

// GetLatestCheckpoint returns the most recent checkpoint for a session
func (cm *DefaultCheckpointManager) GetLatestCheckpoint(sessionID string) (*WorkflowCheckpoint, error) {
	checkpoints, err := cm.ListCheckpoints(sessionID)
	if err != nil {
		return nil, err
	}

	if len(checkpoints) == 0 {
		return nil, fmt.Errorf("no checkpoints found for session %s", sessionID)
	}

	// Find the latest checkpoint
	var latest *WorkflowCheckpoint
	for _, checkpoint := range checkpoints {
		if latest == nil || checkpoint.Timestamp.After(latest.Timestamp) {
			latest = checkpoint
		}
	}

	return latest, nil
}

// CleanupOldCheckpoints removes checkpoints older than the specified duration
func (cm *DefaultCheckpointManager) CleanupOldCheckpoints(maxAge time.Duration) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	var deletedCount int

	for checkpointID, checkpoint := range cm.checkpoints {
		if checkpoint.Timestamp.Before(cutoff) {
			sessionID := checkpoint.SessionState["id"].(string)
			delete(cm.checkpoints, checkpointID)
			cm.removeFromIndex(sessionID, checkpointID)
			deletedCount++
		}
	}

	cm.logger.Info().
		Int("deleted_count", deletedCount).
		Dur("max_age", maxAge).
		Msg("Cleaned up old checkpoints")

	return nil
}

// ResumeFromStage creates a checkpoint that allows resuming from a specific stage
func (cm *DefaultCheckpointManager) ResumeFromStage(
	session *WorkflowSession,
	stageName string,
	workflowSpec *WorkflowSpec,
) (*WorkflowCheckpoint, error) {
	// Validate that the stage exists in the workflow
	var stageExists bool
	for _, stage := range workflowSpec.Spec.Stages {
		if stage.Name == stageName {
			stageExists = true
			break
		}
	}

	if !stageExists {
		return nil, fmt.Errorf("stage %s not found in workflow", stageName)
	}

	// Update session state to resume from the specified stage
	session.CurrentStage = stageName
	session.Status = WorkflowStatusPaused

	// Remove any stages after the resume point from completed list
	var newCompleted []string
	for _, completed := range session.CompletedStages {
		if completed != stageName {
			newCompleted = append(newCompleted, completed)
		} else {
			break
		}
	}
	session.CompletedStages = newCompleted

	return cm.CreateCheckpoint(session, stageName, fmt.Sprintf("Resume from stage: %s", stageName), workflowSpec)
}

// Private helper methods

func (cm *DefaultCheckpointManager) checkpointBelongsToSession(sessionID, checkpointID string) bool {
	checkpointIDs, exists := cm.sessionIndex[sessionID]
	if !exists {
		return false
	}

	for _, id := range checkpointIDs {
		if id == checkpointID {
			return true
		}
	}

	return false
}

func (cm *DefaultCheckpointManager) removeFromIndex(sessionID, checkpointID string) {
	if checkpointIDs, exists := cm.sessionIndex[sessionID]; exists {
		for i, id := range checkpointIDs {
			if id == checkpointID {
				cm.sessionIndex[sessionID] = append(checkpointIDs[:i], checkpointIDs[i+1:]...)
				break
			}
		}

		// Clean up empty index entries
		if len(cm.sessionIndex[sessionID]) == 0 {
			delete(cm.sessionIndex, sessionID)
		}
	}
}
