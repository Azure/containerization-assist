// Package workflow provides state persistence for workflow execution
package workflow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"log/slog"
)

// WorkflowCheckpoint represents a saved state of workflow execution
type WorkflowCheckpoint struct {
	WorkflowID   string                 `json:"workflow_id"`
	Timestamp    time.Time              `json:"timestamp"`
	CurrentStep  int                    `json:"current_step"`
	TotalSteps   int                    `json:"total_steps"`
	Args         ContainerizeAndDeployArgs `json:"args"`
	State        map[string]interface{} `json:"state"`
	Errors       []string               `json:"errors"`
	Warnings     []string               `json:"warnings"`
	CompletedSteps []string             `json:"completed_steps"`
}

// StatePersistence handles workflow state persistence
type StatePersistence struct {
	workspaceDir string
	logger       *slog.Logger
}

// NewStatePersistence creates a new state persistence handler
func NewStatePersistence(workspaceDir string, logger *slog.Logger) *StatePersistence {
	return &StatePersistence{
		workspaceDir: workspaceDir,
		logger:       logger.With("component", "state-persistence"),
	}
}

// SaveCheckpoint saves the current workflow state
func (sp *StatePersistence) SaveCheckpoint(checkpoint *WorkflowCheckpoint) error {
	// Ensure checkpoint directory exists
	checkpointDir := filepath.Join(sp.workspaceDir, "checkpoints", checkpoint.WorkflowID)
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
		sp.logger.Warn("Failed to update latest checkpoint", "error", err)
	}

	sp.logger.Info("Checkpoint saved",
		"workflow_id", checkpoint.WorkflowID,
		"step", checkpoint.CurrentStep,
		"path", checkpointPath)

	return nil
}

// LoadLatestCheckpoint loads the most recent checkpoint for a workflow
func (sp *StatePersistence) LoadLatestCheckpoint(workflowID string) (*WorkflowCheckpoint, error) {
	checkpointPath := filepath.Join(sp.workspaceDir, "checkpoints", workflowID, "latest.json")
	
	data, err := os.ReadFile(checkpointPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No checkpoint exists
		}
		return nil, fmt.Errorf("failed to read checkpoint: %v", err)
	}

	var checkpoint WorkflowCheckpoint
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		return nil, fmt.Errorf("failed to unmarshal checkpoint: %v", err)
	}

	sp.logger.Info("Checkpoint loaded",
		"workflow_id", checkpoint.WorkflowID,
		"step", checkpoint.CurrentStep,
		"timestamp", checkpoint.Timestamp)

	return &checkpoint, nil
}

// CleanupOldCheckpoints removes checkpoints older than the specified duration
func (sp *StatePersistence) CleanupOldCheckpoints(maxAge time.Duration) error {
	checkpointsDir := filepath.Join(sp.workspaceDir, "checkpoints")
	
	// Check if checkpoints directory exists
	if _, err := os.Stat(checkpointsDir); os.IsNotExist(err) {
		return nil // Nothing to cleanup
	}

	cutoffTime := time.Now().Add(-maxAge)
	var cleaned int

	err := filepath.Walk(checkpointsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Skip directories and latest.json files
		if info.IsDir() || info.Name() == "latest.json" {
			return nil
		}

		// Check if file is older than cutoff
		if info.ModTime().Before(cutoffTime) {
			if err := os.Remove(path); err != nil {
				sp.logger.Warn("Failed to remove old checkpoint", "path", path, "error", err)
			} else {
				cleaned++
			}
		}

		return nil
	})

	if err != nil {
		sp.logger.Warn("Error during checkpoint cleanup", "error", err)
	}

	if cleaned > 0 {
		sp.logger.Info("Cleaned up old checkpoints", "count", cleaned)
	}

	return nil
}

// WorkflowStateManager manages workflow state during execution
type WorkflowStateManager struct {
	persistence  *StatePersistence
	workflowID   string
	args         ContainerizeAndDeployArgs
	state        map[string]interface{}
	errors       []string
	warnings     []string
	completed    []string
	currentStep  int
	totalSteps   int
	logger       *slog.Logger
}

// NewWorkflowStateManager creates a new workflow state manager
func NewWorkflowStateManager(workflowID string, args ContainerizeAndDeployArgs, totalSteps int, persistence *StatePersistence, logger *slog.Logger) *WorkflowStateManager {
	return &WorkflowStateManager{
		persistence: persistence,
		workflowID:  workflowID,
		args:        args,
		state:       make(map[string]interface{}),
		errors:      []string{},
		warnings:    []string{},
		completed:   []string{},
		currentStep: 0,
		totalSteps:  totalSteps,
		logger:      logger.With("component", "state-manager", "workflow_id", workflowID),
	}
}

// SaveState saves the current workflow state
func (sm *WorkflowStateManager) SaveState(stepName string) error {
	checkpoint := &WorkflowCheckpoint{
		WorkflowID:     sm.workflowID,
		Timestamp:      time.Now(),
		CurrentStep:    sm.currentStep,
		TotalSteps:     sm.totalSteps,
		Args:           sm.args,
		State:          sm.state,
		Errors:         sm.errors,
		Warnings:       sm.warnings,
		CompletedSteps: sm.completed,
	}

	return sm.persistence.SaveCheckpoint(checkpoint)
}

// SetStepCompleted marks a step as completed
func (sm *WorkflowStateManager) SetStepCompleted(stepName string) {
	sm.completed = append(sm.completed, stepName)
	sm.currentStep++
}

// AddError adds an error to the state
func (sm *WorkflowStateManager) AddError(err string) {
	sm.errors = append(sm.errors, err)
}

// AddWarning adds a warning to the state
func (sm *WorkflowStateManager) AddWarning(warning string) {
	sm.warnings = append(sm.warnings, warning)
}

// SetState sets a state value
func (sm *WorkflowStateManager) SetState(key string, value interface{}) {
	sm.state[key] = value
}

// GetState retrieves a state value
func (sm *WorkflowStateManager) GetState(key string) (interface{}, bool) {
	val, ok := sm.state[key]
	return val, ok
}

// CanResumeFrom checks if workflow can be resumed from a checkpoint
func (sm *WorkflowStateManager) CanResumeFrom(checkpoint *WorkflowCheckpoint) bool {
	// Check if args match (excluding test mode flag)
	if checkpoint.Args.RepoURL != sm.args.RepoURL ||
		checkpoint.Args.Branch != sm.args.Branch {
		return false
	}

	// Check if workflow is incomplete
	return checkpoint.CurrentStep < checkpoint.TotalSteps
}