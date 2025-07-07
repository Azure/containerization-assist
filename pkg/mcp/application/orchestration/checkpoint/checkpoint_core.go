package checkpoint

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/orchestration/execution"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
	"github.com/google/uuid"
	"go.etcd.io/bbolt"
)

// CreateCheckpoint creates a new checkpoint for a workflow session
func (cm *BoltCheckpointManager) CreateCheckpoint(
	session *session.WorkflowSession,
	stageName string,
	message string,
	workflowSpec *session.WorkflowSpec,
) (*execution.WorkflowCheckpoint, error) {
	checkpointID := uuid.New().String()

	checkpoint := &execution.WorkflowCheckpoint{
		ID:         checkpointID,
		WorkflowID: session.WorkflowID,
		SessionID:  session.SessionState.ID,
		StageName:  stageName,
		Timestamp:  time.Now(),
		State: map[string]interface{}{
			"workflow_spec": workflowSpec,
		},
		SessionState: map[string]interface{}{
			"session_id":        session.SessionState.ID,
			"workflow_id":       session.WorkflowID,
			"workflow_name":     session.WorkflowName,
			"status":            session.Status,
			"current_stage":     session.CurrentStage,
			"completed_stages":  session.CompletedStages,
			"failed_stages":     session.FailedStages,
			"skipped_stages":    session.SkippedStages,
			"shared_context":    session.SharedContext,
			"resource_bindings": session.ResourceBindings,
			"start_time":        session.StartTime,
			"last_activity":     session.LastActivity,
		},
		StageResults: session.StageResults,
		Message:      message,
	}

	err := cm.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(checkpointsBucket))
		if err != nil {
			return errors.NewError().Messagef("error").WithLocation().Build()
		}

		checkpointData, err := json.Marshal(checkpoint)
		if err != nil {
			return errors.NewError().Messagef("error").WithLocation().Build()
		}

		compressedData, isCompressed, err := cm.compressData(checkpointData)
		if err != nil {
			return errors.NewError().Messagef("error").WithLocation().Build()
		}

		checksum := cm.calculateChecksum(compressedData)

		envelope := &CheckpointEnvelope{
			Version:    1,
			Compressed: isCompressed,
			Checksum:   checksum,
			DataSize:   len(checkpointData),
			CreatedAt:  time.Now(),
			Data:       compressedData,
			Metadata: map[string]string{
				"session_id":       session.ID,
				"stage_name":       stageName,
				"workflow_name":    session.WorkflowName,
				"compression_mode": fmt.Sprintf("%d", cm.compressionMode),
			},
		}

		envelopeData, err := json.Marshal(envelope)
		if err != nil {
			return errors.NewError().Messagef("error").WithLocation().Build()
		}

		key := fmt.Sprintf("%s_%s", session.ID, checkpointID)
		return bucket.Put([]byte(key), envelopeData)
	})

	if err != nil {
		return nil, errors.NewError().Messagef("error").Build()
	}

	cm.logger.Info().
		Str("checkpoint_id", checkpointID).
		Str("session_id", session.ID).
		Str("stage_name", stageName).
		Str("message", message).
		Msg("Created workflow checkpoint")

	return checkpoint, nil
}

// GetLatestCheckpoint returns the most recent checkpoint for a session
func (cm *BoltCheckpointManager) GetLatestCheckpoint(sessionID string) (*execution.WorkflowCheckpoint, error) {
	checkpointIDs, err := cm.ListCheckpoints(sessionID)
	if err != nil {
		return nil, err
	}

	if len(checkpointIDs) == 0 {
		return nil, errors.NewError().Messagef("error").WithLocation().Build()
	}

	checkpoint, err := cm.GetCheckpoint(sessionID, checkpointIDs[0])
	if err != nil {
		return nil, err
	}

	return checkpoint, nil
}
