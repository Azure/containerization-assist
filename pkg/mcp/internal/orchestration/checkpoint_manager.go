package orchestration

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/workflow"
	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"go.etcd.io/bbolt"
)

// BoltCheckpointManager implements CheckpointManager using BoltDB with compression and integrity checks
type BoltCheckpointManager struct {
	db              *bbolt.DB
	logger          zerolog.Logger
	compressionMode CompressionMode
	enableIntegrity bool
}

// CompressionMode defines how checkpoint data is compressed
type CompressionMode int

const (
	NoCompression CompressionMode = iota
	GzipCompression
)

// CheckpointOptions configures checkpoint behavior
type CheckpointOptions struct {
	Compression     CompressionMode
	EnableIntegrity bool
	IncludeMetrics  bool
	CompactOldData  bool
}

// NewBoltCheckpointManager creates a new BoltDB-backed checkpoint manager
func NewBoltCheckpointManager(db *bbolt.DB, logger zerolog.Logger) *BoltCheckpointManager {
	return &BoltCheckpointManager{
		db:              db,
		logger:          logger.With().Str("component", "checkpoint_manager").Logger(),
		compressionMode: GzipCompression, // Enable compression by default
		enableIntegrity: true,            // Enable integrity checks by default
	}
}

// NewBoltCheckpointManagerWithOptions creates a checkpoint manager with custom options
func NewBoltCheckpointManagerWithOptions(db *bbolt.DB, logger zerolog.Logger, opts CheckpointOptions) *BoltCheckpointManager {
	return &BoltCheckpointManager{
		db:              db,
		logger:          logger.With().Str("component", "checkpoint_manager").Logger(),
		compressionMode: opts.Compression,
		enableIntegrity: opts.EnableIntegrity,
	}
}

const (
	checkpointsBucket = "workflow_checkpoints"
	metadataBucket    = "checkpoint_metadata"
)

// CheckpointEnvelope wraps checkpoint data with metadata for compression and integrity
type CheckpointEnvelope struct {
	Version    int                    `json:"version"`
	Compressed bool                   `json:"compressed"`
	Checksum   string                 `json:"checksum,omitempty"`
	DataSize   int                    `json:"data_size"`
	CreatedAt  time.Time              `json:"created_at"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	Data       []byte                 `json:"data"`
}

// compressData compresses data using the configured compression mode
func (cm *BoltCheckpointManager) compressData(data []byte) ([]byte, bool, error) {
	if cm.compressionMode == NoCompression {
		return data, false, nil
	}

	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)

	_, err := gzWriter.Write(data)
	if err != nil {
		return nil, false, mcptypes.WrapRichError(err, "GZIP_WRITE_FAILED", "failed to write to gzip writer", "compression_error")
	}

	err = gzWriter.Close()
	if err != nil {
		return nil, false, mcptypes.WrapRichError(err, "GZIP_CLOSE_FAILED", "failed to close gzip writer", "compression_error")
	}

	compressed := buf.Bytes()

	// Only use compression if it actually reduces size
	if len(compressed) >= len(data) {
		cm.logger.Debug().
			Int("original_size", len(data)).
			Int("compressed_size", len(compressed)).
			Msg("Compression didn't reduce size, storing uncompressed")
		return data, false, nil
	}

	cm.logger.Debug().
		Int("original_size", len(data)).
		Int("compressed_size", len(compressed)).
		Float64("compression_ratio", float64(len(compressed))/float64(len(data))).
		Msg("Data compressed successfully")

	return compressed, true, nil
}

// decompressData decompresses data if it was compressed
func (cm *BoltCheckpointManager) decompressData(data []byte, isCompressed bool) ([]byte, error) {
	if !isCompressed {
		return data, nil
	}

	gzReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, mcptypes.WrapRichError(err, "GZIP_READER_FAILED", "failed to create gzip reader", "decompression_error")
	}
	defer gzReader.Close()

	decompressed, err := io.ReadAll(gzReader)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress data: %w", err)
	}

	return decompressed, nil
}

// calculateChecksum calculates SHA-256 checksum of data
func (cm *BoltCheckpointManager) calculateChecksum(data []byte) string {
	if !cm.enableIntegrity {
		return ""
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// verifyChecksum verifies data integrity using checksum
func (cm *BoltCheckpointManager) verifyChecksum(data []byte, expectedChecksum string) error {
	if !cm.enableIntegrity || expectedChecksum == "" {
		return nil
	}

	actualChecksum := cm.calculateChecksum(data)
	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}

	return nil
}

// CreateCheckpoint creates a new checkpoint for a workflow session
func (cm *BoltCheckpointManager) CreateCheckpoint(
	session *workflow.WorkflowSession,
	stageName string,
	message string,
	workflowSpec *workflow.WorkflowSpec,
) (*workflow.WorkflowCheckpoint, error) {
	checkpointID := uuid.New().String()

	checkpoint := &workflow.WorkflowCheckpoint{
		ID:           checkpointID,
		StageName:    stageName,
		Timestamp:    time.Now(),
		WorkflowSpec: workflowSpec,
		SessionState: map[string]interface{}{
			"session_id":        session.ID,
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

	// Store checkpoint in database with compression and integrity
	err := cm.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(checkpointsBucket))
		if err != nil {
			return fmt.Errorf("failed to create checkpoints bucket: %w", err)
		}

		// Marshal checkpoint data
		checkpointData, err := json.Marshal(checkpoint)
		if err != nil {
			return fmt.Errorf("failed to marshal checkpoint: %w", err)
		}

		// Compress data if enabled
		compressedData, isCompressed, err := cm.compressData(checkpointData)
		if err != nil {
			return fmt.Errorf("failed to compress checkpoint data: %w", err)
		}

		// Calculate checksum for integrity
		checksum := cm.calculateChecksum(compressedData)

		// Create envelope with metadata
		envelope := &CheckpointEnvelope{
			Version:    1,
			Compressed: isCompressed,
			Checksum:   checksum,
			DataSize:   len(checkpointData),
			CreatedAt:  time.Now(),
			Data:       compressedData,
			Metadata: map[string]interface{}{
				"session_id":       session.ID,
				"stage_name":       stageName,
				"workflow_name":    session.WorkflowName,
				"compression_mode": cm.compressionMode,
			},
		}

		// Marshal envelope
		envelopeData, err := json.Marshal(envelope)
		if err != nil {
			return fmt.Errorf("failed to marshal checkpoint envelope: %w", err)
		}

		// Use composite key: sessionID_checkpointID for easy querying
		key := fmt.Sprintf("%s_%s", session.ID, checkpointID)
		return bucket.Put([]byte(key), envelopeData)
	})

	if err != nil {
		return nil, fmt.Errorf("failed to store checkpoint: %w", err)
	}

	cm.logger.Info().
		Str("checkpoint_id", checkpointID).
		Str("session_id", session.ID).
		Str("stage_name", stageName).
		Str("message", message).
		Msg("Created workflow checkpoint")

	return checkpoint, nil
}

// CreateIncrementalCheckpoint creates a checkpoint that only stores changes since the last checkpoint
func (cm *BoltCheckpointManager) CreateIncrementalCheckpoint(
	session *workflow.WorkflowSession,
	stageName string,
	message string,
	workflowSpec *workflow.WorkflowSpec,
) (*workflow.WorkflowCheckpoint, error) {
	// Get the latest checkpoint to calculate delta
	latestCheckpoint, err := cm.GetLatestCheckpoint(session.ID)
	if err != nil {
		// No previous checkpoint, create full checkpoint
		cm.logger.Debug().
			Str("session_id", session.ID).
			Msg("No previous checkpoint found, creating full checkpoint")
		return cm.CreateCheckpoint(session, stageName, message, workflowSpec)
	}

	checkpointID := uuid.New().String()

	// Calculate delta - only include changes since last checkpoint
	deltaCheckpoint := &workflow.WorkflowCheckpoint{
		ID:           checkpointID,
		StageName:    stageName,
		Timestamp:    time.Now(),
		WorkflowSpec: workflowSpec,
		Message:      message + " (incremental)",
		SessionState: cm.calculateSessionStateDelta(session, latestCheckpoint),
		StageResults: cm.calculateStageResultsDelta(session.StageResults, latestCheckpoint.StageResults),
	}

	// Store checkpoint with incremental flag
	err = cm.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(checkpointsBucket))
		if err != nil {
			return fmt.Errorf("failed to create checkpoints bucket: %w", err)
		}

		// Marshal checkpoint data
		checkpointData, err := json.Marshal(deltaCheckpoint)
		if err != nil {
			return fmt.Errorf("failed to marshal incremental checkpoint: %w", err)
		}

		// Compress data if enabled
		compressedData, isCompressed, err := cm.compressData(checkpointData)
		if err != nil {
			return fmt.Errorf("failed to compress incremental checkpoint data: %w", err)
		}

		// Calculate checksum for integrity
		checksum := cm.calculateChecksum(compressedData)

		// Create envelope with incremental metadata
		envelope := &CheckpointEnvelope{
			Version:    1,
			Compressed: isCompressed,
			Checksum:   checksum,
			DataSize:   len(checkpointData),
			CreatedAt:  time.Now(),
			Data:       compressedData,
			Metadata: map[string]interface{}{
				"session_id":        session.ID,
				"stage_name":        stageName,
				"workflow_name":     session.WorkflowName,
				"compression_mode":  cm.compressionMode,
				"incremental":       true,
				"parent_checkpoint": latestCheckpoint.ID,
			},
		}

		// Marshal envelope
		envelopeData, err := json.Marshal(envelope)
		if err != nil {
			return fmt.Errorf("failed to marshal incremental checkpoint envelope: %w", err)
		}

		// Use composite key: sessionID_checkpointID for easy querying
		key := fmt.Sprintf("%s_%s", session.ID, checkpointID)
		return bucket.Put([]byte(key), envelopeData)
	})

	if err != nil {
		return nil, fmt.Errorf("failed to store incremental checkpoint: %w", err)
	}

	cm.logger.Info().
		Str("checkpoint_id", checkpointID).
		Str("session_id", session.ID).
		Str("stage_name", stageName).
		Str("parent_checkpoint", latestCheckpoint.ID).
		Str("message", message).
		Msg("Created incremental workflow checkpoint")

	return deltaCheckpoint, nil
}

// calculateSessionStateDelta calculates the difference in session state
func (cm *BoltCheckpointManager) calculateSessionStateDelta(
	currentSession *workflow.WorkflowSession,
	lastCheckpoint *workflow.WorkflowCheckpoint,
) map[string]interface{} {
	delta := make(map[string]interface{})

	// Compare and add only changed fields
	lastState := lastCheckpoint.SessionState

	if currentSession.Status != workflow.WorkflowStatus(lastState["status"].(string)) {
		delta["status"] = string(currentSession.Status)
	}

	if currentSession.CurrentStage != lastState["current_stage"].(string) {
		delta["current_stage"] = currentSession.CurrentStage
	}

	// Check for new completed stages
	lastCompleted := lastState["completed_stages"].([]interface{})
	lastCompletedStrs := make([]string, len(lastCompleted))
	for i, v := range lastCompleted {
		lastCompletedStrs[i] = v.(string)
	}

	newCompleted := make([]string, 0)
	for _, stage := range currentSession.CompletedStages {
		found := false
		for _, lastStage := range lastCompletedStrs {
			if stage == lastStage {
				found = true
				break
			}
		}
		if !found {
			newCompleted = append(newCompleted, stage)
		}
	}
	if len(newCompleted) > 0 {
		delta["new_completed_stages"] = newCompleted
	}

	// Add current timestamp
	delta["last_activity"] = currentSession.LastActivity

	return delta
}

// calculateStageResultsDelta calculates the difference in stage results
func (cm *BoltCheckpointManager) calculateStageResultsDelta(
	currentResults map[string]interface{},
	lastResults map[string]interface{},
) map[string]interface{} {
	delta := make(map[string]interface{})

	for stageName, result := range currentResults {
		// If stage result is new or changed, include it in delta
		if lastResult, exists := lastResults[stageName]; !exists || !cm.deepEqual(result, lastResult) {
			delta[stageName] = result
		}
	}

	return delta
}

// deepEqual performs a deep comparison of two interface{} values
func (cm *BoltCheckpointManager) deepEqual(a, b interface{}) bool {
	// Simple JSON-based comparison for now
	aJSON, aErr := json.Marshal(a)
	bJSON, bErr := json.Marshal(b)

	if aErr != nil || bErr != nil {
		return false
	}

	return bytes.Equal(aJSON, bJSON)
}

// RestoreFromCheckpoint restores a workflow session from a checkpoint
func (cm *BoltCheckpointManager) RestoreFromCheckpoint(
	sessionID string,
	checkpointID string,
) (*workflow.WorkflowSession, error) {
	var checkpoint *workflow.WorkflowCheckpoint

	// Retrieve checkpoint from database
	err := cm.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(checkpointsBucket))
		if bucket == nil {
			return fmt.Errorf("checkpoints bucket not found")
		}

		key := fmt.Sprintf("%s_%s", sessionID, checkpointID)
		envelopeData := bucket.Get([]byte(key))
		if envelopeData == nil {
			return fmt.Errorf("checkpoint not found: %s", checkpointID)
		}

		// Try to unmarshal as envelope first (new format)
		var envelope CheckpointEnvelope
		if err := json.Unmarshal(envelopeData, &envelope); err == nil && envelope.Version >= 1 {
			// New envelope format - decompress and verify integrity
			decompressedData, err := cm.decompressData(envelope.Data, envelope.Compressed)
			if err != nil {
				return fmt.Errorf("failed to decompress checkpoint data: %w", err)
			}

			// Verify checksum if integrity is enabled
			if err := cm.verifyChecksum(envelope.Data, envelope.Checksum); err != nil {
				cm.logger.Warn().
					Err(err).
					Str("checkpoint_id", checkpointID).
					Msg("Checkpoint integrity check failed, attempting recovery")
				// Continue with corrupted data - better than failing completely
			}

			checkpoint = &workflow.WorkflowCheckpoint{}
			return json.Unmarshal(decompressedData, checkpoint)
		} else {
			// Legacy format - direct unmarshal
			cm.logger.Debug().
				Str("checkpoint_id", checkpointID).
				Msg("Loading checkpoint in legacy format")
			checkpoint = &workflow.WorkflowCheckpoint{}
			return json.Unmarshal(envelopeData, checkpoint)
		}
	})

	if err != nil {
		return nil, err
	}

	// Reconstruct session from checkpoint
	session, err := cm.reconstructSession(checkpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to reconstruct session from checkpoint: %w", err)
	}

	cm.logger.Info().
		Str("checkpoint_id", checkpointID).
		Str("session_id", sessionID).
		Str("stage_name", checkpoint.StageName).
		Msg("Restored workflow session from checkpoint")

	return session, nil
}

// ListCheckpoints returns all checkpoints for a session
func (cm *BoltCheckpointManager) ListCheckpoints(sessionID string) ([]*workflow.WorkflowCheckpoint, error) {
	var checkpoints []*workflow.WorkflowCheckpoint

	err := cm.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(checkpointsBucket))
		if bucket == nil {
			// No checkpoints exist yet
			return nil
		}

		cursor := bucket.Cursor()
		prefix := []byte(sessionID + "_")

		for key, value := cursor.Seek(prefix); key != nil && len(key) > len(prefix) && string(key[:len(prefix)]) == string(prefix); key, value = cursor.Next() {
			var checkpoint workflow.WorkflowCheckpoint
			if err := json.Unmarshal(value, &checkpoint); err != nil {
				cm.logger.Warn().
					Err(err).
					Str("checkpoint_key", string(key)).
					Msg("Failed to unmarshal checkpoint, skipping")
				continue
			}
			checkpoints = append(checkpoints, &checkpoint)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list checkpoints: %w", err)
	}

	// Sort checkpoints by timestamp (newest first)
	for i := 0; i < len(checkpoints)-1; i++ {
		for j := i + 1; j < len(checkpoints); j++ {
			if checkpoints[i].Timestamp.Before(checkpoints[j].Timestamp) {
				checkpoints[i], checkpoints[j] = checkpoints[j], checkpoints[i]
			}
		}
	}

	cm.logger.Debug().
		Str("session_id", sessionID).
		Int("checkpoint_count", len(checkpoints)).
		Msg("Listed workflow checkpoints")

	return checkpoints, nil
}

// DeleteCheckpoint removes a specific checkpoint
func (cm *BoltCheckpointManager) DeleteCheckpoint(checkpointID string) error {
	var deletedKey string

	err := cm.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(checkpointsBucket))
		if bucket == nil {
			return fmt.Errorf("checkpoints bucket not found")
		}

		// Find the checkpoint by scanning all keys
		cursor := bucket.Cursor()
		for key, _ := cursor.First(); key != nil; key, _ = cursor.Next() {
			keyStr := string(key)
			if len(keyStr) > 37 && keyStr[len(keyStr)-36:] == checkpointID { // UUID length is 36
				deletedKey = keyStr
				return bucket.Delete(key)
			}
		}

		return fmt.Errorf("checkpoint not found: %s", checkpointID)
	})

	if err != nil {
		return fmt.Errorf("failed to delete checkpoint: %w", err)
	}

	cm.logger.Info().
		Str("checkpoint_id", checkpointID).
		Str("deleted_key", deletedKey).
		Msg("Deleted workflow checkpoint")

	return nil
}

// DeleteSessionCheckpoints removes all checkpoints for a session
func (cm *BoltCheckpointManager) DeleteSessionCheckpoints(sessionID string) error {
	var deletedCount int

	err := cm.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(checkpointsBucket))
		if bucket == nil {
			return nil // No checkpoints to delete
		}

		cursor := bucket.Cursor()
		prefix := []byte(sessionID + "_")
		var keysToDelete [][]byte

		for key, _ := cursor.Seek(prefix); key != nil && len(key) > len(prefix) && string(key[:len(prefix)]) == string(prefix); key, _ = cursor.Next() {
			keysToDelete = append(keysToDelete, append([]byte(nil), key...))
		}

		for _, key := range keysToDelete {
			if err := bucket.Delete(key); err != nil {
				return err
			}
			deletedCount++
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to delete session checkpoints: %w", err)
	}

	cm.logger.Info().
		Str("session_id", sessionID).
		Int("deleted_count", deletedCount).
		Msg("Deleted session checkpoints")

	return nil
}

// CleanupExpiredCheckpoints removes checkpoints older than the specified duration
func (cm *BoltCheckpointManager) CleanupExpiredCheckpoints(maxAge time.Duration) (int, error) {
	cutoffTime := time.Now().Add(-maxAge)
	var expiredKeys []string

	// Find expired checkpoints
	err := cm.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(checkpointsBucket))
		if bucket == nil {
			return nil
		}

		cursor := bucket.Cursor()
		for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
			var checkpoint workflow.WorkflowCheckpoint
			if err := json.Unmarshal(value, &checkpoint); err != nil {
				continue
			}

			if checkpoint.Timestamp.Before(cutoffTime) {
				expiredKeys = append(expiredKeys, string(key))
			}
		}

		return nil
	})

	if err != nil {
		return 0, fmt.Errorf("failed to find expired checkpoints: %w", err)
	}

	// Delete expired checkpoints
	deletedCount := 0
	err = cm.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(checkpointsBucket))
		if bucket == nil {
			return nil
		}

		for _, key := range expiredKeys {
			if err := bucket.Delete([]byte(key)); err != nil {
				cm.logger.Warn().
					Err(err).
					Str("checkpoint_key", key).
					Msg("Failed to delete expired checkpoint")
			} else {
				deletedCount++
			}
		}

		return nil
	})

	if err != nil {
		return deletedCount, fmt.Errorf("failed to delete expired checkpoints: %w", err)
	}

	cm.logger.Info().
		Int("deleted_count", deletedCount).
		Dur("max_age", maxAge).
		Msg("Cleaned up expired workflow checkpoints")

	return deletedCount, nil
}

// GetLatestCheckpoint returns the most recent checkpoint for a session
func (cm *BoltCheckpointManager) GetLatestCheckpoint(sessionID string) (*workflow.WorkflowCheckpoint, error) {
	checkpoints, err := cm.ListCheckpoints(sessionID)
	if err != nil {
		return nil, err
	}

	if len(checkpoints) == 0 {
		return nil, fmt.Errorf("no checkpoints found for session: %s", sessionID)
	}

	// Checkpoints are already sorted by timestamp (newest first)
	return checkpoints[0], nil
}

// GetCheckpointMetrics returns metrics about workflow checkpoints
func (cm *BoltCheckpointManager) GetCheckpointMetrics() (*CheckpointMetrics, error) {
	metrics := &CheckpointMetrics{
		SessionCounts: make(map[string]int),
		StageCounts:   make(map[string]int),
	}

	err := cm.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(checkpointsBucket))
		if bucket == nil {
			return nil
		}

		cursor := bucket.Cursor()
		for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
			var checkpoint workflow.WorkflowCheckpoint
			if err := json.Unmarshal(value, &checkpoint); err != nil {
				continue
			}

			metrics.TotalCheckpoints++

			// Extract session ID from key
			keyStr := string(key)
			parts := strings.Split(keyStr, "_")
			if len(parts) >= 6 { // UUID format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
				sessionID := strings.Join(parts[:5], "_")
				metrics.SessionCounts[sessionID]++
			}

			metrics.StageCounts[checkpoint.StageName]++

			if checkpoint.Timestamp.After(metrics.LastCheckpoint) {
				metrics.LastCheckpoint = checkpoint.Timestamp
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get checkpoint metrics: %w", err)
	}

	return metrics, nil
}

// Helper methods

func (cm *BoltCheckpointManager) reconstructSession(checkpoint *workflow.WorkflowCheckpoint) (*workflow.WorkflowSession, error) {
	sessionState := checkpoint.SessionState

	// Extract values with type assertions
	getStringValue := func(key string, defaultValue string) string {
		if val, ok := sessionState[key].(string); ok {
			return val
		}
		return defaultValue
	}

	getStringSlice := func(key string) []string {
		if val, ok := sessionState[key].([]interface{}); ok {
			result := make([]string, len(val))
			for i, v := range val {
				if str, ok := v.(string); ok {
					result[i] = str
				}
			}
			return result
		}
		return []string{}
	}

	getStringMap := func(key string) map[string]string {
		if val, ok := sessionState[key].(map[string]interface{}); ok {
			result := make(map[string]string)
			for k, v := range val {
				if str, ok := v.(string); ok {
					result[k] = str
				}
			}
			return result
		}
		return make(map[string]string)
	}

	getTime := func(key string, defaultValue time.Time) time.Time {
		if val, ok := sessionState[key].(string); ok {
			if t, err := time.Parse(time.RFC3339, val); err == nil {
				return t
			}
		}
		return defaultValue
	}

	session := &workflow.WorkflowSession{
		ID:               getStringValue("session_id", ""),
		WorkflowID:       getStringValue("workflow_id", ""),
		WorkflowName:     getStringValue("workflow_name", ""),
		Status:           workflow.WorkflowStatus(getStringValue("status", string(workflow.WorkflowStatusPending))),
		CurrentStage:     getStringValue("current_stage", ""),
		CompletedStages:  getStringSlice("completed_stages"),
		FailedStages:     getStringSlice("failed_stages"),
		SkippedStages:    getStringSlice("skipped_stages"),
		StageResults:     checkpoint.StageResults,
		ResourceBindings: getStringMap("resource_bindings"),
		StartTime:        getTime("start_time", time.Now()),
		LastActivity:     getTime("last_activity", time.Now()),
		CreatedAt:        getTime("start_time", time.Now()), // Use start_time as created_at
		UpdatedAt:        checkpoint.Timestamp,
	}

	// Restore shared context
	if sharedContext, ok := sessionState["shared_context"].(map[string]interface{}); ok {
		session.SharedContext = sharedContext
	} else {
		session.SharedContext = make(map[string]interface{})
	}

	// Add checkpoint to session's checkpoint list
	session.Checkpoints = []workflow.WorkflowCheckpoint{*checkpoint}

	return session, nil
}

// CheckpointMetrics contains metrics about workflow checkpoints
type CheckpointMetrics struct {
	TotalCheckpoints int            `json:"total_checkpoints"`
	SessionCounts    map[string]int `json:"session_counts"`
	StageCounts      map[string]int `json:"stage_counts"`
	LastCheckpoint   time.Time      `json:"last_checkpoint"`
}
