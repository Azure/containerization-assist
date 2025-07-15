// Package persistence provides infrastructure implementations for state persistence
package persistence

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	infraerrors "github.com/Azure/container-kit/pkg/mcp/infrastructure/core"
)

// FileStateStore implements the workflow.StateStore interface using the file system
type FileStateStore struct {
	workspaceDir string
	logger       *slog.Logger
}

// NewFileStateStore creates a new file-based state store
func NewFileStateStore(workspaceDir string, logger *slog.Logger) workflow.StateStore {
	return &FileStateStore{
		workspaceDir: workspaceDir,
		logger:       logger.With("component", "file-state-store"),
	}
}

// SaveCheckpoint persists a workflow checkpoint to the file system
func (s *FileStateStore) SaveCheckpoint(checkpoint *workflow.WorkflowCheckpoint) error {
	// Ensure checkpoint directory exists
	checkpointDir := filepath.Join(s.workspaceDir, "checkpoints", checkpoint.WorkflowID)
	if err := os.MkdirAll(checkpointDir, 0755); err != nil {
		return fmt.Errorf("failed to create checkpoint directory: %v", err)
	}

	// Generate checkpoint filename with timestamp
	filename := fmt.Sprintf("checkpoint_%d.json", checkpoint.Timestamp.Unix())
	checkpointPath := filepath.Join(checkpointDir, filename)

	// Marshal checkpoint to JSON
	data, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal checkpoint: %v", err)
	}

	// Write checkpoint file
	if err := os.WriteFile(checkpointPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write checkpoint file: %v", err)
	}

	// Also save as latest checkpoint for easy access
	latestPath := filepath.Join(checkpointDir, "latest.json")
	if err := os.WriteFile(latestPath, data, 0644); err != nil {
		s.logger.Warn("Failed to update latest checkpoint", "error", err)
	}

	s.logger.Info("Checkpoint saved",
		"workflow_id", checkpoint.WorkflowID,
		"step", checkpoint.CurrentStep,
		"path", checkpointPath)

	return nil
}

// LoadLatestCheckpoint retrieves the most recent checkpoint for a workflow
func (s *FileStateStore) LoadLatestCheckpoint(workflowID string) (*workflow.WorkflowCheckpoint, error) {
	checkpointPath := filepath.Join(s.workspaceDir, "checkpoints", workflowID, "latest.json")

	data, err := os.ReadFile(checkpointPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No checkpoint exists
		}
		return nil, fmt.Errorf("failed to read checkpoint: %v", err)
	}

	var checkpoint workflow.WorkflowCheckpoint
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		return nil, fmt.Errorf("failed to unmarshal checkpoint: %v", err)
	}

	s.logger.Info("Checkpoint loaded",
		"workflow_id", checkpoint.WorkflowID,
		"step", checkpoint.CurrentStep,
		"timestamp", checkpoint.Timestamp)

	return &checkpoint, nil
}

// CleanupOldCheckpoints removes checkpoints older than the specified duration
func (s *FileStateStore) CleanupOldCheckpoints(maxAge time.Duration) error {
	checkpointsDir := filepath.Join(s.workspaceDir, "checkpoints")

	// Check if checkpoints directory exists
	if _, err := os.Stat(checkpointsDir); os.IsNotExist(err) {
		return nil // Nothing to cleanup
	}

	cutoffTime := time.Now().Add(-maxAge)
	var cleaned int

	err := filepath.Walk(checkpointsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Log error with context but continue walking
			s.logger.Debug("Error accessing file during cleanup",
				"path", path,
				"error", err)
			return nil // Continue walking despite individual file errors
		}

		// Skip directories and latest.json files
		if info.IsDir() || info.Name() == "latest.json" {
			return nil
		}

		// Check if file is older than cutoff
		if info.ModTime().Before(cutoffTime) {
			if err := os.Remove(path); err != nil {
				// Create structured error for file removal
				infraErr := infraerrors.NewInfrastructureError(
					"remove_checkpoint",
					"file_system",
					"Failed to remove old checkpoint file",
					err,
					infraerrors.IsPermissionDenied(err), // Recoverable if permission issue
				).WithContext("path", path).
					WithContext("file_age", time.Since(info.ModTime()).String())

				infraErr.LogWithContext(s.logger)
			} else {
				cleaned++
				s.logger.Debug("Removed old checkpoint", "path", path, "age", time.Since(info.ModTime()))
			}
		}

		return nil
	})

	if err != nil {
		s.logger.Warn("Error during checkpoint cleanup", "error", err)
	}

	if cleaned > 0 {
		s.logger.Info("Cleaned up old checkpoints", "count", cleaned)
	}

	return nil
}
